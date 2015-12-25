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
	Email    string
	AgreeTOS string

	Client *agent.Client
	Store  *agent.Store
}

func (c *RegCommand) Run() error {
	var publicKey jwk.Key
	if key, err := c.Store.LoadPublicKey(); err != nil && err == agent.ErrFileNotFound {
		log.Println("INFO: account key pair is not found. Creating new account key pair...")

		privkey, err := rsa.GenerateKey(rand.Reader, 4096)
		if err != nil {
			return err
		}

		privateKey, err := jwk.NewRsaPrivateKey(privkey)
		if err != nil {
			return err
		}

		if err := c.Store.SaveKey(privateKey); err != nil {
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
	if err := c.Client.Init(); err != nil {
		return err
	}

	var account *agent.Account

	// try to load account info
	account, err := c.Store.LoadAccount()
	if err != nil {
		if err != agent.ErrFileNotFound {
			return err
		}

		// begin new registration
		newRegReq := &agent.NewRegistrationRequest{
			Contact: []string{"mailto:" + c.Email},
		}

		acc, err := c.Client.Register(newRegReq)
		if err != nil {
			return err
		}

		// save an account before we make agreement
		if err := c.Store.SaveAccount(acc); err != nil {
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
		Contact:   []string{"mailto:" + c.Email},
		Agreement: c.AgreeTOS,
		Key:       publicKey,
	}

	if err := c.Client.UpdateRegistration(account.URL, updateRegReq); err != nil {
		return err
	}

	account.TOSAgreed = true
	if err := c.Store.SaveAccount(account); err != nil {
		return err
	}

	log.Printf("INFO: registration has been done with the agreement found at %s", account.URL)

	return nil
}
