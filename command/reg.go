package command

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"log"

	"github.com/go-acme/lego/v3/registration"
	"github.com/nabeken/aaa/agent"
	"gopkg.in/square/go-jose.v2"
)

type RegCommand struct {
	AgreeTOS bool `long:"agree-tos" description:"Agree with the ToS"`
	Override bool `long:"override the registration if it already exists with a new key"`
}

func (c *RegCommand) Execute(args []string) error {
	var (
		privKey crypto.PrivateKey
		err     error
	)

	// initialize S3 bucket and filer
	store, err := NewStore(Options.Email, Options.S3Bucket, Options.S3KMSKeyID)
	if err != nil {
		return err
	}

	ri, err := store.LoadRegistration()
	if err != nil && err != agent.ErrFileNotFound {
		return err
	}

	if err == nil && !c.Override {
		log.Println("INFO: found the existing registration. Please set --override to register with a new key.")
		return nil
	}

	log.Println("INFO: creating new account key pair...")

	privKey, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}

	ri = &agent.RegistrationInfo{
		Email: Options.Email,
		Key: &jose.JSONWebKey{
			Key: privKey,
		},
	}

	client, err := agent.NewLegoClient(ri)
	if err != nil {
		return err
	}

	if !c.AgreeTOS {
		log.Printf("Please agree with TOS found at %s", client.GetToSURL())
		return nil
	}

	log.Println("INFO: registering account...")

	reg, err := client.Registration.Register(registration.RegisterOptions{
		TermsOfServiceAgreed: c.AgreeTOS,
	})
	if err != nil {
		return err
	}

	log.Printf("DEBUG: RegistrationInfo: %#v\n", reg)

	ri.Registration = reg
	if err := store.SaveRegistration(ri); err != nil {
		log.Println("ERROR: unable to save the registration")
		return err
	}

	log.Printf("INFO: registration has been done")

	return nil
}
