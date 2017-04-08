package command

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"log"

	"github.com/lestrrat/go-jwx/jwk"
	"github.com/nabeken/aaa/agent"
	"github.com/pkg/errors"
)

type CertCommand struct {
	CommonName string   `long:"cn" description:"CommonName to be issued"`
	Domains    []string `long:"domain" description:"Domains to be issued as Subject Alternative Names"`
	CreateKey  bool     `long:"create-key" description:"Create a new keypair"`
}

func (c *CertCommand) Execute(args []string) error {
	return (&CertService{
		CommonName: c.CommonName,
		Domains:    c.Domains,
		CreateKey:  c.CreateKey,
		S3Bucket:   Options.S3Bucket,
		S3KMSKeyID: Options.S3KMSKeyID,
		Email:      Options.Email,
	}).Run()
}

type CertService struct {
	CommonName string
	Domains    []string
	CreateKey  bool
	S3Bucket   string
	S3KMSKeyID string
	Email      string
}

func (svc *CertService) Run() error {
	store, err := newStore(svc.Email, svc.S3Bucket, svc.S3KMSKeyID)
	if err != nil {
		return errors.Wrap(err, "failed to initialize the store")
	}

	log.Print("INFO: now issuing certificate...")

	// trying to load the key
	key, err := store.LoadCertKey(svc.CommonName)
	if err != nil {
		if err != agent.ErrFileNotFound {
			return errors.Wrap(err, "failed to load the key")
		}

		// we have to create a new keypair anyway
		svc.CreateKey = true
	}

	// Creating private key for cert
	var privateKey *rsa.PrivateKey
	if svc.CreateKey {
		log.Print("INFO: creating new private key...")
		certPrivkey, err := rsa.GenerateKey(rand.Reader, 4096)
		if err != nil {
			return errors.Wrap(err, "failed to generate a keypair")
		}

		certPrivkeyJWK, err := jwk.NewRsaPrivateKey(certPrivkey)
		if err != nil {
			return errors.Wrap(err, "failed to create a JWK")
		}

		// storing private key for certificate
		if err := store.SaveCertKey(svc.CommonName, certPrivkeyJWK); err != nil {
			return errors.Wrap(err, "failed to store the JWK")
		}

		privateKey = certPrivkey
	} else {
		log.Print("INFO: using the existing private key...")
		pkey, err := key.Materialize()
		if err != nil {
			return errors.Wrap(err, "failed to materialize the key")
		}

		rsaPrivKey, ok := pkey.(*rsa.PrivateKey)
		if !ok {
			return fmt.Errorf("aaa: key is not *rsa.PrivateKey but %v", pkey)
		}

		privateKey = rsaPrivKey
	}

	// Creating CSR
	der, err := agent.CreateCertificateRequest(privateKey, svc.CommonName, svc.Domains...)
	if err != nil {
		return errors.Wrap(err, "failed to create a CSR")
	}

	// initialize client here
	client := agent.NewClient(DirectoryURL(), store)
	if err := client.Init(); err != nil {
		return errors.Wrap(err, "failed to initialize the client")
	}

	// Issue new-cert request
	certURL, err := client.NewCertificate(der)
	if err != nil {
		return errors.Wrap(err, "failed to issue the certificate")
	}

	log.Printf("INFO: certificate will be available at %s", certURL)

	issuerCert, myCert, err := client.GetCertificate(certURL)
	if err != nil {
		return errors.Wrap(err, "failed to get the certificate")
	}

	if err := store.SaveCert(svc.CommonName, issuerCert, myCert); err != nil {
		return errors.Wrap(err, "failed to store the certificate")
	}

	log.Print("INFO: certificate is successfully saved")

	return nil
}
