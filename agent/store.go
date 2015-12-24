package agent

import (
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/lestrrat/go-jwx/jwk"
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
	email  string
	filer  Filer
	prefix string
}

func NewStore(email string, filer Filer) (*Store, error) {
	if email == "" {
		return nil, errors.New("aaa: email must not be empty")
	}

	if debug, _ := strconv.ParseBool(os.Getenv("AAA_DEBUG")); debug {
		filer = new(OSFiler)
	}

	s := &Store{
		email:  email,
		filer:  filer,
		prefix: "aaa-data",
	}

	return s, nil
}

// LoadKey returns RSA private key in JWK.
func (s *Store) LoadPrivateKey() (jwk.Key, error) {
	blob, err := s.filer.ReadFile(s.joinPrefix("info", s.email+".jwk"))
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

	return s.filer.WriteFile(s.joinPrefix("info", s.email+".jwk"), blob)
}

func (s *Store) SaveCertKey(domain string, privateKey jwk.Key) error {
	blob, err := json.Marshal(privateKey)
	if err != nil {
		return err
	}

	return s.filer.WriteFile(s.joinPrefix("domain", domain, "privkey.jwk"), blob)
}

func (s *Store) LoadCert(domain string) (*x509.Certificate, error) {
	blob, err := s.filer.ReadFile(s.joinPrefix("domain", domain, "cert.pem"))
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(blob)
	return x509.ParseCertificate(block.Bytes)
}

func (s *Store) SaveCert(domain string, cert *x509.Certificate) error {
	buf := new(bytes.Buffer)
	if err := pem.Encode(buf, &pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}); err != nil {
		return err
	}

	return s.filer.WriteFile(s.joinPrefix("domain", domain, "cert.pem"), buf.Bytes())
}

func (s *Store) LoadAuthorization(domain string) (*Authorization, error) {
	blob, err := s.filer.ReadFile(s.joinPrefix("domain", domain, "authz.json"))
	if err != nil {
		return nil, err
	}

	authz := &Authorization{}
	if err := json.Unmarshal(blob, authz); err != nil {
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
	return s.filer.WriteFile(s.joinPrefix("domain", domain, "authz.json"), blob)
}

func (s *Store) SaveAccount(account *Account) error {
	blob, err := json.Marshal(account)
	if err != nil {
		return err
	}

	return s.filer.WriteFile(s.joinPrefix("info", s.email+".json"), blob)
}

func (s *Store) LoadAccount() (*Account, error) {
	blob, err := s.filer.ReadFile(s.joinPrefix("info", s.email+".json"))
	if err != nil {
		return nil, err
	}

	account := &Account{}
	if err := json.Unmarshal(blob, account); err != nil {
		return nil, err
	}

	return account, nil
}

func (s *Store) joinPrefix(fns ...string) string {
	return s.filer.Join(append([]string{s.prefix, s.email}, fns...)...)
}
