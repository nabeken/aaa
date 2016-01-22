package agent

import (
	"bytes"
	"crypto"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"github.com/lestrrat/go-jwx/jwa"
	"github.com/lestrrat/go-jwx/jwk"
	"github.com/lestrrat/go-jwx/jws"
	"github.com/tent/http-link-go"
)

const (
	joseContentType = "application/jose+json"
)

var DefaultDirectoryURL = "https://acme-staging.api.letsencrypt.org/directory"

// ACMEError implements error interface.
// See https://tools.ietf.org/html/draft-ietf-acme-acme-01#section-5.4
type ACMEError struct {
	StatusCode int
	Type       string `json:"type"`
	Detail     string `json:"detail"`
}

func (e *ACMEError) Error() string {
	return fmt.Sprintf(
		"aaa: acme error(%d): type: %s detail: %s",
		e.StatusCode,
		e.Type,
		e.Detail,
	)
}

func NewACMEError(resp *http.Response) error {
	e := &ACMEError{
		StatusCode: resp.StatusCode,
	}
	if err := json.NewDecoder(resp.Body).Decode(e); err != nil {
		return errors.New("aaa: failed to decode acme error")
	}
	return e
}

type directory struct {
	NewAuthz   string `json:"new-authz"`
	NewCert    string `json:"new-cert"`
	NewReg     string `json:"new-reg"`
	RevokeCert string `json:"revoke-cert"`
}

// Account is to hold registration information as JSON.
type Account struct {
	URL       string
	TOS       string
	TOSAgreed bool
}

type Combination []int

type NewRegistrationRequest struct {
	Contact []string `json:"contact"`
}

type UpdateRegistrationRequest struct {
	Key       jwk.Key  `json:"key",omitempty`
	Contact   []string `json:"contact"`
	Agreement string   `json:"agreement,omitempty"`
}

type NewAuthorizationRequest struct {
	Identifier *Identifier `json:"identifier"`
}

type Authorization struct {
	// URL is our original property
	URL string `json:"url"`

	Status       string        `json:"status"`
	Expires      string        `json:"expires"`
	Identifier   Identifier    `json:"identifier"`
	Challenges   []Challenge   `json:"challenges"`
	Combinations []Combination `json:"combinations"`
}

// IsExpired returns true if authorization is expired.
func (a *Authorization) IsExpired(now time.Time) bool {
	// If we fails to parse the time, it will be compaired with zero time.
	return a.GetExpires().Before(now)
}

// GetExpires returns Expires in time.Time by parsing strings.
// If it fails to parse, zero value of time.Time will be returned.
func (a *Authorization) GetExpires() time.Time {
	expires, _ := time.Parse(time.RFC3339, a.Expires)
	return expires
}

type Identifier struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type Client struct {
	httpClient *http.Client

	store  *Store
	signer *jws.MultiSign

	privateKey jwk.Key
	publicKey  jwk.Key

	directoryURL string
	directory    *directory
}

// NewClient initializes ACME client and returns the client.
// If it fails to initialize the client, it will return an error.
func NewClient(dirURL string, store *Store) *Client {
	return &Client{
		httpClient:   http.DefaultClient,
		store:        store,
		directoryURL: dirURL,
	}
}

// init initialize ACME client. It fetch the directory resource and also update
// nonce internally.
func (c *Client) Init() error {
	privateKey, err := c.store.LoadPrivateKey()
	if err != nil {
		return err
	}
	c.privateKey = privateKey

	privkey, err := privateKey.Materialize()
	if err != nil {
		return err
	}

	rsaPrivKey, ok := privkey.(*rsa.PrivateKey)
	if !ok {
		return fmt.Errorf("aaa: key is not *rsa.PrivateKey but %v", privkey)
	}

	publicKey, err := c.store.LoadPublicKey()
	if err != nil {
		return err
	}
	c.publicKey = publicKey

	rsaSigner, err := jws.NewRsaSign(jwa.RS256, rsaPrivKey)
	if err != nil {
		return err
	}

	c.signer = jws.NewSigner(rsaSigner)
	for _, s := range c.signer.Signers {
		if err := s.PublicHeaders().Set("jwk", publicKey); err != nil {
			return nil
		}
	}

	resp, err := c.httpClient.Get(c.directoryURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	c.updateNonce(resp)

	if resp.StatusCode > 299 {
		return NewACMEError(resp)
	}

	dir := &directory{}
	if err := json.NewDecoder(resp.Body).Decode(dir); err != nil {
		return err
	}
	c.directory = dir

	return nil
}

func (c *Client) sign(payload []byte) ([]byte, error) {
	msg, err := c.signer.Sign(payload)
	if err != nil {
		return nil, err
	}

	return jws.JSONSerialize{}.Serialize(msg)
}

// Register do new-registration.
func (c *Client) Register(req *NewRegistrationRequest) (*Account, error) {
	newreg := struct {
		Resource string `json:"resource"`
		*NewRegistrationRequest
	}{
		Resource:               "new-reg",
		NewRegistrationRequest: req,
	}

	payload, err := json.Marshal(newreg)
	if err != nil {
		return nil, err
	}

	signed, err := c.sign(payload)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Post(c.directory.NewReg, joseContentType, bytes.NewReader(signed))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	c.updateNonce(resp)

	if resp.StatusCode > 299 {
		return nil, NewACMEError(resp)
	}

	tosLink, err := FindTOS(resp)
	if err != nil {
		return nil, err
	}

	return &Account{URL: resp.Header.Get("Location"), TOS: tosLink.URI}, nil
}

func (c *Client) UpdateRegistration(url string, req *UpdateRegistrationRequest) error {
	// Updating registration with TOS
	updatereg := struct {
		Resource string `json:"resource"`
		*UpdateRegistrationRequest
	}{
		Resource:                  "reg",
		UpdateRegistrationRequest: req,
	}

	payload, err := json.Marshal(updatereg)
	if err != nil {
		return err
	}

	Debug(string(payload))

	signed, err := c.sign(payload)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Post(url, joseContentType, bytes.NewReader(signed))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	c.updateNonce(resp)

	if resp.StatusCode > 299 {
		return NewACMEError(resp)
	}

	return nil
}

func (c *Client) GetAuthorization(authzURL string) (*Authorization, error) {
	resp, err := c.httpClient.Get(authzURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode > 299 {
		return nil, NewACMEError(resp)
	}

	authz := &Authorization{
		URL: authzURL,
	}

	if err := json.NewDecoder(resp.Body).Decode(authz); err != nil {
		return nil, err
	}

	return authz, nil
}

// NewCertificate requests CA to issue new certificate. It will return an URL of
// certificate
func (c *Client) NewCertificate(der string) (string, error) {
	newcert := struct {
		Resource string `json:"resource"`
		CSR      string `json:csr"`
	}{
		Resource: "new-cert",
		CSR:      der,
	}

	payload, err := json.Marshal(newcert)
	if err != nil {
		return "", err
	}

	signed, err := c.sign(payload)
	if err != nil {
		return "", err
	}

	resp, err := c.httpClient.Post(c.directory.NewCert, "application/jose+json", bytes.NewReader(signed))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	c.updateNonce(resp)

	if resp.StatusCode > 299 {
		return "", NewACMEError(resp)
	}

	return resp.Header.Get("Location"), nil
}

func (c *Client) GetCertificate(uri string) (*x509.Certificate, *x509.Certificate, error) {
	last := time.Duration(3 * time.Minute)
	for begin := time.Now(); time.Since(begin) < last; time.Sleep(5 * time.Second) {
		resp, err := c.httpClient.Get(uri)
		if err != nil {
			return nil, nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode > 299 {
			return nil, nil, NewACMEError(resp)
		}

		if resp.StatusCode == http.StatusAccepted {
			Debug("Creation of certificate is still ongoing...")
			continue
		}

		blob, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, nil, err
		}

		if resp.StatusCode == http.StatusOK {
			myCert, err := x509.ParseCertificate(blob)
			if err != nil {
				return nil, nil, err
			}

			issuerCertLink, err := FindLinkByName(resp, "up")
			if err != nil {
				return nil, nil, err
			}

			Debug("Retrieving issuer's certificate...")

			// FIXME: issuerCertLink.URI is not URI... it's just a relative link.
			// so resolving this here...
			u, err := url.Parse(uri)
			if err != nil {
				return nil, nil, err
			}
			u.Path = issuerCertLink.URI

			issuerResp, err := c.httpClient.Get(u.String())
			if err != nil {
				return nil, nil, err
			}
			defer issuerResp.Body.Close()

			if issuerResp.StatusCode > 299 {
				return nil, nil, errors.New("aaa: failed to retrieve issuer's certificate")
			}

			issuerBlob, err := ioutil.ReadAll(issuerResp.Body)
			if err != nil {
				return nil, nil, err
			}

			issuerCert, err := x509.ParseCertificate(issuerBlob)
			if err != nil {
				return nil, nil, err
			}

			return issuerCert, myCert, nil
		}
	}

	return nil, nil, fmt.Errorf("aaa: certificate is not available within %s", last)
}

func (c *Client) NewAuthorization(req *NewAuthorizationRequest) (*Authorization, error) {
	newauthz := struct {
		Resource string `json:"resource"`
		*NewAuthorizationRequest
	}{
		Resource:                "new-authz",
		NewAuthorizationRequest: req,
	}

	payload, err := json.Marshal(newauthz)
	if err != nil {
		return nil, err
	}

	signed, err := c.sign(payload)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Post(c.directory.NewAuthz, "application/jose+json", bytes.NewReader(signed))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	c.updateNonce(resp)

	if resp.StatusCode > 299 {
		return nil, NewACMEError(resp)
	}

	authzResp := &Authorization{
		URL: resp.Header.Get("Location"),
	}
	if err := json.NewDecoder(resp.Body).Decode(authzResp); err != nil {
		return nil, err
	}

	return authzResp, nil
}

// WaitChallengeDone waits for 5 minutes until status is valid or invalid.
func (c *Client) WaitChallengeDone(challenge Challenge) error {
	var status string
	last := time.Duration(5 * time.Minute)

	for begin := time.Now(); time.Since(begin) < last; time.Sleep(5 * time.Second) {
		ch, err := c.getChallengeStatus(challenge.URI)
		if err != nil {
			return err
		}

		status = ch.Status
		Debug("challenge status is " + status)
		switch status {
		case "pending":
			continue
		case "invalid":
			return fmt.Errorf("aaa: %s challenge becomes %s", challenge.Type, status)
		case "valid":
			return nil
		}
	}

	return fmt.Errorf(
		"aaa: %s challenge has not been completed within %s: %s",
		challenge.Type,
		last,
		status,
	)
}

func (c *Client) getChallengeStatus(uri string) (*Challenge, error) {
	resp, err := c.httpClient.Get(uri)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode > 299 {
		return nil, NewACMEError(resp)
	}

	ch := &Challenge{}
	if err := json.NewDecoder(resp.Body).Decode(ch); err != nil {
		return nil, err
	}

	return ch, nil
}

func (c *Client) SolveChallenge(challenge Challenge, keyAuthz string) error {
	cresponse := struct {
		Resource string `json:"resource"`
		*Challenge
	}{
		Resource: "challenge",
		Challenge: &Challenge{
			Type:             challenge.Type,
			Token:            challenge.Token,
			KeyAuthorization: keyAuthz,
		},
	}

	blob, err := json.Marshal(cresponse)
	if err != nil {
		return err
	}

	// TODO:
	Debug(string(blob))

	signed, err := c.sign(blob)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Post(challenge.URI, "application/jose+json", bytes.NewReader(signed))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	c.updateNonce(resp)

	challengeStatus := &Challenge{}
	if err := json.NewDecoder(resp.Body).Decode(challengeStatus); err != nil {
		return err
	}

	Debug(challengeStatus)

	if resp.StatusCode != http.StatusAccepted {
		return challengeStatus.Error
	}

	return nil
}

func (c *Client) updateNonce(resp *http.Response) {
	for _, signer := range c.signer.Signers {
		signer.ProtectedHeaders().Set("nonce", resp.Header.Get("Replay-Nonce"))
	}
}

func FindLinkByName(resp *http.Response, name string) (link.Link, error) {
	for _, header := range resp.Header[http.CanonicalHeaderKey("Link")] {
		links, err := link.Parse(header)
		if err != nil {
			return link.Link{}, err
		}

		for _, link := range links {
			if link.Rel == name {
				return link, nil
			}
		}
	}
	return link.Link{}, errors.New("aaa: no link found")
}

func FindTOS(resp *http.Response) (link.Link, error) {
	return FindLinkByName(resp, "terms-of-service")
}

func Body(resp *http.Response) string {
	body, _ := ioutil.ReadAll(resp.Body)
	return string(body)
}

// BuildKeyAuthorization builds Key Authorization.
// See https://letsencrypt.github.io/acme-spec/#rfc.section.7
func BuildKeyAuthorization(token string, publicKey jwk.Key) (string, error) {
	thumb, err := publicKey.Thumbprint(crypto.SHA256)
	if err != nil {
		return "", err
	}

	return token + "." + base64.RawURLEncoding.EncodeToString(thumb), nil
}
