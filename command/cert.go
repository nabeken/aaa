package command

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"log"

	"github.com/lestrrat/go-jwx/jwk"
	"github.com/nabeken/aaa/agent"
)

type CertCommand struct {
	CommonName string   `long:"cn" description:"CommonName to be issued"`
	Domains    []string `long:"domain" description:"Domains to be issued as Subject Alternative Names"`
	CreateKey  bool     `long:"create-key" description:"Create a new keypair"`
}

func (c *CertCommand) Execute(args []string) error {
	store, err := newStore(Options.Email, Options.S3Bucket, Options.S3KMSKeyID)
	if err != nil {
		return err
	}

	log.Print("INFO: now issuing certificate...")

	var privateKey *rsa.PrivateKey

	// Creating private key for cert
	if c.CreateKey {
		log.Print("INFO: creating new private key...")
		certPrivkey, err := rsa.GenerateKey(rand.Reader, 4096)
		if err != nil {
			return err
		}

		certPrivkeyJWK, err := jwk.NewRsaPrivateKey(certPrivkey)
		if err != nil {
			return err
		}

		// storing private key for certificate
		if err := store.SaveCertKey(c.CommonName, certPrivkeyJWK); err != nil {
			return err
		}

		privateKey = certPrivkey
	} else {
		log.Print("INFO: loading existing private key...")
		key, err := store.LoadCertKey(c.CommonName)
		if err != nil {
			return err
		}

		pkey, err := key.Materialize()
		if err != nil {
			return err
		}

		rsaPrivKey, ok := pkey.(*rsa.PrivateKey)
		if !ok {
			return fmt.Errorf("aaa: key is not *rsa.PrivateKey but %v", pkey)
		}

		privateKey = rsaPrivKey
	}

	// Creating CSR
	der, err := agent.CreateCertificateRequest(privateKey, c.CommonName, c.Domains...)
	if err != nil {
		return err
	}

	// initialize client here
	client := agent.NewClient(DirectoryURL(), store)
	if err := client.Init(); err != nil {
		return err
	}

	// Issue new-cert request
	certURL, err := client.NewCertificate(der)
	if err != nil {
		return err
	}

	log.Printf("INFO: certificate will be available at %s", certURL)

	issuerCert, myCert, err := client.GetCertificate(certURL)
	if err != nil {
		return err
	}

	if err := store.SaveCert(c.CommonName, issuerCert, myCert); err != nil {
		return err
	}

	log.Print("INFO: certificate is successfully saved")

	return nil
}
