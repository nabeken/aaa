package command

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"log"
	"strconv"

	"github.com/lestrrat/go-jwx/jwk"
	"github.com/nabeken/aaa/agent"
	"github.com/pkg/errors"
)

type CertCommand struct {
	CommonName string   `long:"cn" description:"CommonName to be issued"`
	Domains    []string `long:"domain" description:"Domains to be issued as Subject Alternative Names"`
	CreateKey  bool     `long:"create-key" description:"Create a new keypair"`
	RSAKeySize string   `long:"rsa-key-size" description:"Size of the RSA key, only used if create-key is specified. (default: 4096, allowed: 2048 / 4096)"`
}

func (c *CertCommand) Execute(args []string) error {
	store, err := NewStore(Options.Email, Options.S3Bucket, Options.S3KMSKeyID)
	if err != nil {
		return errors.Wrap(err, "failed to initialize the store")
	}
	keyLength := 4096
	if c.RSAKeySize != "" {
		keyLengthInt, err := strconv.Atoi(c.RSAKeySize)
		if err != nil {
			return errors.Wrap(err, "RSA key length is not a number")
		}
		if keyLengthInt == 2048 || keyLengthInt == 4096 {
			keyLength = keyLengthInt
		} else {
			return fmt.Errorf("Specified RSA key length is not 2048 or 4096, but %d", keyLengthInt)
		}
	}
	return (&CertService{
		CommonName: c.CommonName,
		Domains:    c.Domains,
		CreateKey:  c.CreateKey,
		RSAKeySize: keyLength,
		Store:      store,
	}).Run()
}

type CertService struct {
	CommonName string
	Domains    []string
	CreateKey  bool
	RSAKeySize int
	Store      *agent.Store
}

func (svc *CertService) Run() error {
	log.Print("INFO: now issuing certificate...")

	// trying to load the key
	key, err := svc.Store.LoadCertKey(svc.CommonName)
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
		certPrivkey, err := rsa.GenerateKey(rand.Reader, svc.RSAKeySize)
		if err != nil {
			return errors.Wrap(err, "failed to generate a keypair")
		}

		certPrivkeyJWK, err := jwk.NewRsaPrivateKey(certPrivkey)
		if err != nil {
			return errors.Wrap(err, "failed to create a JWK")
		}

		// storing private key for certificate
		if err := svc.Store.SaveCertKey(svc.CommonName, certPrivkeyJWK); err != nil {
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
	client := agent.NewClient(DirectoryURL(), svc.Store)
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

	if err := svc.Store.SaveCert(svc.CommonName, issuerCert, myCert); err != nil {
		return errors.Wrap(err, "failed to store the certificate")
	}

	log.Print("INFO: certificate is successfully saved")

	return nil
}
