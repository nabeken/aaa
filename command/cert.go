package command

import (
	"crypto/rand"
	"crypto/rsa"
	"log"

	"github.com/go-acme/lego/v3/certificate"
	"github.com/go-acme/lego/v3/providers/dns"
	"github.com/nabeken/aaa/agent"
	"github.com/pkg/errors"
)

type CertCommand struct {
	CommonName string   `long:"cn" description:"CommonName to be issued"`
	Domains    []string `long:"domain" description:"Domains to be issued as Subject Alternative Names"`
	CreateKey  bool     `long:"create-key" description:"Create a new keypair"`
	RSAKeySize int      `long:"rsa-key-size" description:"Size of the RSA key, only used if create-key is specified. (allowed: 2048 / 4096)" default:"4096"`
}

func (c *CertCommand) Execute(args []string) error {
	store, err := NewStore(Options.Email, Options.S3Bucket, Options.S3KMSKeyID)
	if err != nil {
		return errors.Wrap(err, "failed to initialize the store")
	}

	return (&CertService{
		Email:      Options.Email,
		CommonName: c.CommonName,
		Domains:    c.Domains,
		CreateKey:  c.CreateKey,
		RSAKeySize: c.RSAKeySize,
		Store:      store,
	}).Run()
}

type CertService struct {
	Email      string
	CommonName string
	Domains    []string
	CreateKey  bool
	RSAKeySize int
	Store      *agent.Store
}

func (svc *CertService) Run() error {
	log.Print("INFO: now issuing certificate...")

	ri, err := svc.Store.LoadRegistration()
	if err != nil {
		return errors.Wrap(err, "unable to load the registration")
	}

	client, err := agent.NewLegoClient(ri)
	if err != nil {
		return err
	}

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
	if svc.CreateKey {
		if svc.RSAKeySize != 4096 && svc.RSAKeySize != 2048 {
			return errors.New("key size must be 4096 or 2048")
		}

		log.Printf("INFO: creating %d bit new private key...", svc.RSAKeySize)
		certPrivkey, err := rsa.GenerateKey(rand.Reader, svc.RSAKeySize)
		if err != nil {
			return errors.Wrap(err, "failed to generate a keypair")
		}

		// storing private key for certificate
		if err := svc.Store.SaveCertKey(svc.CommonName, certPrivkey); err != nil {
			return errors.Wrap(err, "failed to store the private key for the cert")
		}

		key = certPrivkey
	} else {
		log.Print("INFO: using the existing private key...")
	}

	provider, err := dns.NewDNSChallengeProviderByName("route53")
	if err != nil {
		return errors.Wrap(err, "unable to initialize the challenge provider")
	}

	client.Challenge.SetDNS01Provider(provider)

	request := certificate.ObtainRequest{
		Domains:    append([]string{svc.CommonName}, svc.Domains...),
		PrivateKey: key,
	}

	cert, err := client.Certificate.Obtain(request)
	if err != nil {
		return errors.Wrap(err, "unable to obtain the certificate")
	}

	if err := svc.Store.SaveCert(svc.CommonName, cert.Certificate); err != nil {
		return errors.Wrap(err, "failed to store the certificate")
	}

	log.Print("INFO: certificate is successfully saved")

	return nil
}
