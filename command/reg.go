package command

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"log"

	"github.com/lestrrat/go-jwx/jwk"
	"github.com/nabeken/aaa/agent"
)

type RegCommand struct {
	AgreeTOS string `long:"agree" description:"Agree with given TOS of URL"`
}

func (c *RegCommand) Execute(args []string) error {
	// initialize S3 bucket and filer
	store, err := newStore(Options.Email, Options.S3Bucket, Options.S3KMSKeyID)
	if err != nil {
		return err
	}

	var publicKey jwk.Key
	if key, err := store.LoadPublicKey(); err != nil && err == agent.ErrFileNotFound {
		log.Println("INFO: account key pair is not found. Creating new account key pair...")

		privkey, err := rsa.GenerateKey(rand.Reader, 4096)
		if err != nil {
			return err
		}

		privateKey, err := jwk.NewRsaPrivateKey(privkey)
		if err != nil {
			return err
		}

		if err := store.SaveKey(privateKey); err != nil {
			return err
		}

		key, err = jwk.NewRsaPublicKey(&privkey.PublicKey)
		if err != nil {
			return err
		}

		publicKey = key
		log.Println("INFO: new account key pair has been created")
	} else if err != nil {
		return err
	} else {
		publicKey = key
		log.Println("INFO: account key pair is found")
	}

	// initialize client here
	client := agent.NewClient(DirectoryURL(), store)
	if err := client.Init(); err != nil {
		return err
	}

	var account *agent.Account

	// try to load account info
	account, err = store.LoadAccount()
	if err != nil {
		if err != agent.ErrFileNotFound {
			return err
		}

		// begin new registration
		newRegReq := &agent.NewRegistrationRequest{
			Contact: []string{"mailto:" + Options.Email},
		}

		acc, err := client.Register(newRegReq)
		if err != nil {
			return err
		}

		// save an account before we make agreement
		if err := store.SaveAccount(acc); err != nil {
			return err
		}

		account = acc
	}

	if c.AgreeTOS != account.TOS {
		fmt.Printf("Please agree with TOS found at %s\n", account.TOS)
		return nil
	}

	// update registration to agree with TOS
	updateRegReq := &agent.UpdateRegistrationRequest{
		Contact:   []string{"mailto:" + Options.Email},
		Agreement: c.AgreeTOS,
		Key:       publicKey,
	}

	if err := client.UpdateRegistration(account.URL, updateRegReq); err != nil {
		return err
	}

	account.TOSAgreed = true
	if err := store.SaveAccount(account); err != nil {
		return err
	}

	log.Printf("INFO: registration has been done with the agreement found at %s", account.URL)

	return nil
}
