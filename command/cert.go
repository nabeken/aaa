package command

import (
	"crypto/rand"
	"crypto/rsa"
	"log"
	"os"
	"time"

	"github.com/lestrrat/go-jwx/jwk"
	"github.com/nabeken/aaa/agent"
	"github.com/spf13/afero"
	"gopkg.in/alecthomas/kingpin.v2"
)

type CertCommand struct {
	Email      string
	CommonName string
	Domains    []string
	Renewal    bool
}

func (c *CertCommand) Run(ctx *kingpin.ParseContext) error {
	store, err := agent.NewStore(c.Email, new(afero.OsFs))
	if err != nil {
		return err
	}

	// Loading certificate unless we set Renewal flag
	if !c.Renewal {
		if cert, err := store.LoadCert(c.CommonName); err != nil && !os.IsNotExist(err) {
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

	dirURL := agent.DefaultDirectoryURL
	if url := os.Getenv("AAA_DIRECTORY_URL"); url != "" {
		dirURL = url
	}

	client, err := agent.NewClient(dirURL, store)
	if err != nil {
		return err
	}

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
	if err := store.SaveCertKey(c.CommonName, certPrivkeyJWK); err != nil {
		return err
	}

	// Creating CSR
	der, err := agent.CreateCertificateRequest(certPrivkey, c.CommonName, c.Domains...)
	if err != nil {
		return err
	}

	// Issue new-cert request
	certURL, err := client.NewCertificate(der)
	if err != nil {
		return err
	}

	log.Printf("INFO: certificate will be available at %s", certURL)

	cert, err := client.GetCertificate(certURL)
	if err != nil {
		return err
	}

	if err := store.SaveCert(c.CommonName, cert); err != nil {
		return err
	}

	log.Print("INFO: certificate is successfully saved")

	return nil
}
