package command

import (
	"crypto/rand"
	"crypto/rsa"
	"log"
	"time"

	"github.com/lestrrat/go-jwx/jwk"
	"github.com/nabeken/aaa/agent"
)

type CertCommand struct {
	CommonName string
	Domains    []string
	Renewal    bool

	Client *agent.Client
	Store  *agent.Store
}

func (c *CertCommand) Run() error {
	// Loading certificate unless we set Renewal flag
	if !c.Renewal {
		if cert, err := c.Store.LoadCert(c.CommonName); err != nil && err != agent.ErrFileNotFound {
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

	// Creating private key for cert
	certPrivkey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return err
	}

	certPrivkeyJWK, err := jwk.NewRsaPrivateKey(certPrivkey)
	if err != nil {
		return err
	}

	// storing private key for certificate
	if err := c.Store.SaveCertKey(c.CommonName, certPrivkeyJWK); err != nil {
		return err
	}

	// Creating CSR
	der, err := agent.CreateCertificateRequest(certPrivkey, c.CommonName, c.Domains...)
	if err != nil {
		return err
	}

	// Issue new-cert request
	certURL, err := c.Client.NewCertificate(der)
	if err != nil {
		return err
	}

	log.Printf("INFO: certificate will be available at %s", certURL)

	cert, err := c.Client.GetCertificate(certURL)
	if err != nil {
		return err
	}

	if err := c.Store.SaveCert(c.CommonName, cert); err != nil {
		return err
	}

	log.Print("INFO: certificate is successfully saved")

	return nil
}
