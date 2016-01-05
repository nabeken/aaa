package command

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/nabeken/aaa/agent"
	"github.com/nabeken/aws-go-s3/bucket"
)

type LsCommand struct {
	S3Config *S3Config
	Email    string
	Format   string

	filer agent.Filer
}

func (c *LsCommand) init() {
	s3b := bucket.New(s3.New(session.New()), c.S3Config.Bucket)
	c.filer = agent.NewS3Filer(s3b, c.S3Config.KMSKeyID)
}

func (c *LsCommand) Run() error {
	c.init()

	output, err := c.FetchData()
	if err != nil {
		return err
	}

	switch c.Format {
	case "json":
		if err := json.NewEncoder(os.Stdout).Encode(output); err != nil {
			return err
		}
	default:
		fmt.Println("NOT IMPLEMENTED")
	}

	return nil
}

func (c *LsCommand) FetchData() ([]domain, error) {
	data := []domain{}

	emails, err := c.ListAccounts()
	if err != nil {
		return nil, err
	}

	for _, email := range emails {
		store, err := agent.NewStore(email, c.filer)
		if err != nil {
			return nil, err
		}

		domains, err := store.ListDomains()
		if err != nil {
			return nil, err
		}

		for _, dom := range domains {
			authz, err := store.LoadAuthorization(dom)
			if err != nil {
				log.Printf("failed to load authorization for %s: %s. skipping...", dom, err)
				continue
			}

			cert, err := store.LoadCert(dom)
			if err != nil {
				log.Printf("failed to load certificate for %s: %s (or new-cert is ongoing or this domain is in SAN in other certificates). skipping...", dom, err)
				continue
			}

			data = append(data, domain{
				Email:  email,
				Domain: dom,
				Authorization: authorization{
					Expires: authz.GetExpires(),
				},
				Certificate: certificate{
					NotBefore: cert.NotBefore,
					NotAfter:  cert.NotAfter,
					SAN:       cert.DNSNames,
				},
			})
		}
	}

	return data, nil
}

func (c *LsCommand) ListAccounts() ([]string, error) {
	dirs, err := c.filer.ListDir(agent.StorePrefix)
	if err != nil {
		return nil, err
	}

	accounts := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		elem := c.filer.Split(dir)

		// account (email) is in 2nd element
		if len(elem) > 1 {
			accounts = append(accounts, elem[1])
		}
	}

	return accounts, nil
}

type domain struct {
	Email         string        `json:"email"`
	Domain        string        `json:"domain"`
	Authorization authorization `json:"authorization"`
	Certificate   certificate   `json:"certificate"`
}

type authorization struct {
	Expires time.Time `json:"expires"`
}

type certificate struct {
	NotBefore time.Time `json:"not_before"`
	NotAfter  time.Time `json:"not_after"`
	SAN       []string  `json:"san"`
}
