package agent

import (
	"context"
	"crypto"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"os"
	"strconv"

	"github.com/go-acme/lego/v4/certcrypto"
)

var StorePrefix = "aaa-data/v2"

/*
prefix: {{letsencrypt-base}}/aaa-data/v2

Per Store instance:
{{email}}/info
	- {{email}}.json -- the registration info

{{email}}/domain/{{domain}}/
	- privkey.pem   -- the private key in PEM
	- cert.pem      -- the cert
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
		prefix: StorePrefix,
	}

	return s, nil
}

// LoadRegistration returns the existing registration.
func (s *Store) LoadRegistration(ctx context.Context) (*RegistrationInfo, error) {
	blob, err := s.filer.ReadFile(ctx, s.joinPrefix("info", s.email+".json"))
	if err != nil {
		return nil, err
	}

	ri := &RegistrationInfo{}
	if err := json.Unmarshal(blob, ri); err != nil {
		return nil, err
	}

	return ri, nil
}

func (s *Store) SaveRegistration(ctx context.Context, ri *RegistrationInfo) error {
	blob, err := json.Marshal(ri)
	if err != nil {
		return err
	}

	return s.filer.WriteFile(ctx, s.joinPrefix("info", s.email+".json"), blob)
}

func (s *Store) SaveCertKey(ctx context.Context, domain string, privKey crypto.PrivateKey) error {
	return s.filer.WriteFile(
		ctx,
		s.joinPrefix("domain", domain, "privkey.pem"),
		certcrypto.PEMEncode(privKey),
	)
}

func (s *Store) LoadCertKey(ctx context.Context, domain string) (crypto.PrivateKey, error) {
	blob, err := s.filer.ReadFile(ctx, s.joinPrefix("domain", domain, "privkey.pem"))
	if err != nil {
		return nil, err
	}

	return certcrypto.ParsePEMPrivateKey(blob)
}

func (s *Store) LoadCert(ctx context.Context, domain string) (*x509.Certificate, error) {
	blob, err := s.filer.ReadFile(ctx, s.joinPrefix("domain", domain, "cert.pem"))
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(blob)

	return x509.ParseCertificate(block.Bytes)
}

func (s *Store) SaveCert(ctx context.Context, domain string, cert []byte) error {
	return s.filer.WriteFile(ctx, s.joinPrefix("domain", domain, "cert.pem"), cert)
}

func (s *Store) ListDomains(ctx context.Context) ([]string, error) {
	dirs, err := s.filer.ListDir(ctx, s.joinPrefix("domain"))
	if err != nil {
		return nil, err
	}

	domains := make([]string, 0, len(dirs))

	for _, dir := range dirs {
		elem := s.filer.Split(dir)

		// domain is in 4th element
		if len(elem) > 3 {
			domains = append(domains, elem[3])
		}
	}

	return domains, nil
}

func (s *Store) joinPrefix(fns ...string) string {
	return s.filer.Join(append([]string{s.prefix, s.email}, fns...)...)
}
