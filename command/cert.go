package command

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"log"
	"time"

	"github.com/lestrrat/go-jwx/jwk"
	"github.com/nabeken/aaa/agent"
)

type CertCommand struct {
	S3Config *S3Config
	Email    string

	CommonName string
	Domains    []string
	Renewal    bool
	RenewalKey bool
}

func (c *CertCommand) Run() error {
	store, err := newStore(c.Email, c.S3Config)
	if err != nil {
		return err
	}

	// Loading certificate unless we set Renewal flag
	if !c.Renewal {
		if cert, err := store.LoadCert(c.CommonName); err != nil && err != agent.ErrFileNotFound {
			// something is wrong
			return err
		} else if err == nil {
			// If it is found and expiration date is 1 month later, we stop here.
			monthBefore := cert.NotAfter.AddDate(0, -1, 0)
			if time.Now().Before(monthBefore) {
				// we have still 1 month before the cert is expired
				log.Print("INFO: the certificate is up-to-date")
				return nil
			}
			log.Printf("INFO: the certificate will be expired at %s so renewing now", cert.NotAfter)
		}
	}

	log.Print("INFO: now issuing certificate...")

	var privateKey *rsa.PrivateKey

	// Creating private key for cert
	// when it is not renewal or RenewalKey is specified
	if !c.Renewal || c.RenewalKey {
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
