package agent

import (
	"crypto"
	"os"

	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"
	"github.com/go-jose/go-jose/v4"
)

var DefaultDirectoryURL = "https://acme-staging-v02.api.letsencrypt.org/directory"

// RegistrationInfo is a data persisted on the storage.
// A private key for this will be persisted in JWK.
type RegistrationInfo struct {
	Email        string                 `json:"email"`
	Registration *registration.Resource `json:"registration"`
	Key          *jose.JSONWebKey       `json:"key"`
}

func (ri *RegistrationInfo) GetEmail() string {
	return ri.Email
}

func (ri *RegistrationInfo) GetPrivateKey() crypto.PrivateKey {
	return ri.Key.Key
}

func (ri *RegistrationInfo) GetRegistration() *registration.Resource {
	return ri.Registration
}

func DirectoryURL() string {
	if url := os.Getenv("AAA_DIRECTORY_URL"); url != "" {
		return url
	}

	return DefaultDirectoryURL
}

// NewClient2 initializes the lego ACME client and returns the client.
// If it fails to initialize the client, it will return an error.
func NewLegoClient(ri *RegistrationInfo) (*lego.Client, error) {
	config := lego.NewConfig(ri)
	config.CADirURL = DirectoryURL()

	return lego.NewClient(config)
}
