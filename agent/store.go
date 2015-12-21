package agent

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/lestrrat/go-jwx/jwk"
	"github.com/spf13/afero"
)

/*
prefix: {{letsencrypt-base}}/aaa-agent/

Per Store instance:
{{email}}/info
	- {{email}}.json -- the registration info
	- {{email}}.jwk  -- the account private key in JWK

{{email}}/domain/{{domain}}/
    - authz.json    -- the authorization result
	- privkey.jwk   -- the private key in JWK
	- fullchain.pem -- the cert + intermediates
	- cert.pem      -- the cert only
*/

type Store struct {
	email string
	fs    afero.Fs

	prefix string // FIXME: should be configurable
}

// NewStore initialize fs. It makes directory named email.
func NewStore(email string, fs afero.Fs) (*Store, error) {
	if email == "" {
		return nil, errors.New("aaa: email must not be empty")
	}

	s := &Store{
		email:  email,
		fs:     fs,
		prefix: "aaa-agent",
	}

	for _, dir := range []string{
		s.joinPrefix("info"),
		s.joinPrefix("domain"),
	} {
		if err := s.fs.MkdirAll(dir, 0700); err != nil {
			return nil, err
		}
	}

	return s, nil
}

// LoadKey returns RSA private key in JWK.
func (s *Store) LoadPrivateKey() (jwk.Key, error) {
	f, err := s.fs.Open(s.joinPrefix("info", s.email+".jwk"))
	if err != nil {
		return nil, err
	}

	blob, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	keyset, err := jwk.Parse(blob)
	if err != nil {
		return nil, err
	}
	if len(keyset.Keys) == 0 {
		return nil, errors.New("aaa: no key found")
	}

	return keyset.Keys[0], nil
}

func (s *Store) LoadPublicKey() (jwk.Key, error) {
	// LE currently supports RS256 only
	privkey, err := s.LoadPrivateKey()
	if err != nil {
		return nil, err
	}

	key, err := privkey.Materialize()
	if err != nil {
		return nil, err
	}

	rsaPrivKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("aaa: key is not *rsa.PrivateKey but %v", privkey)
	}

	// restore its public key from the private key
	return jwk.NewRsaPublicKey(&rsaPrivKey.PublicKey)
}

func (s *Store) SaveKey(privateKey jwk.Key) error {
	blob, err := json.Marshal(privateKey)
	if err != nil {
		return err
	}

	return afero.WriteFile(s.fs, s.joinPrefix("info", s.email+".jwk"), blob, 0600)
}

func (s *Store) SaveCertKey(domain string, privateKey jwk.Key) error {
	blob, err := json.Marshal(privateKey)
	if err != nil {
		return err
	}

	if err := s.mkDomainDir(domain); err != nil {
		return err
	}

	return afero.WriteFile(s.fs, s.joinPrefix("domain", domain, "privkey.jwk"), blob, 0600)
}

func (s *Store) LoadCert(domain string) (*x509.Certificate, error) {
	f, err := s.fs.Open(s.joinPrefix("domain", domain, "cert.pem"))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	blob, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(blob)
	return x509.ParseCertificate(block.Bytes)
}

func (s *Store) SaveCert(domain string, cert *x509.Certificate) error {
	if err := s.mkDomainDir(domain); err != nil {
		return err
	}

	f, err := s.fs.OpenFile(
		s.joinPrefix("domain", domain, "cert.pem"),
		os.O_WRONLY|os.O_CREATE|os.O_TRUNC,
		0600,
	)
	if err != nil {
		return err
	}

	return pem.Encode(f, &pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
}

func (s *Store) LoadAuthorization(domain string) (*Authorization, error) {
	if err := s.mkDomainDir(domain); err != nil {
		return nil, err
	}

	f, err := s.fs.Open(s.joinPrefix("domain", domain, "authz.json"))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	authz := &Authorization{}
	if err := json.NewDecoder(f).Decode(authz); err != nil {
		return nil, err
	}

	return authz, nil
}

func (s *Store) SaveAuthorization(authz *Authorization) error {
	blob, err := json.Marshal(authz)
	if err != nil {
		return err
	}

	domain := authz.Identifier.Value

	if err := s.mkDomainDir(domain); err != nil {
		return err
	}

	return afero.WriteFile(
		s.fs,
		s.joinPrefix("domain", domain, "authz.json"),
		blob,
		0600,
	)
}

func (s *Store) SaveAccount(account *Account) error {
	blob, err := json.Marshal(account)
	if err != nil {
		return err
	}

	return afero.WriteFile(s.fs, s.joinPrefix("info", s.email+".json"), blob, 0600)
}

func (s *Store) LoadAccount() (*Account, error) {
	f, err := s.fs.Open(s.joinPrefix("info", s.email+".json"))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	account := &Account{}
	if err := json.NewDecoder(f).Decode(account); err != nil {
		return nil, err
	}

	return account, nil
}

func (s *Store) mkDomainDir(domain string) error {
	return s.fs.MkdirAll(s.joinPrefix("domain", domain), 0700)
}

func (s *Store) joinPrefix(fns ...string) string {
	return joinPrefix(append([]string{s.prefix, s.email}, fns...)...)
}

func joinPrefix(fns ...string) string {
	var path string
	if len(fns) > 0 {
		path = fns[0]
		fns = fns[1:]
	}
	for _, fn := range fns {
		path += afero.FilePathSeparator + fn
	}
	return path
}
