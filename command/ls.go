package command

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/nabeken/aaa/agent"
	"github.com/nabeken/aws-go-s3/bucket"
)

type LsCommand struct {
	S3Config *S3Config
	Email    string

	filer agent.Filer
}

func (c *LsCommand) init() {
	s3b := bucket.New(s3.New(session.New()), c.S3Config.Bucket)
	c.filer = agent.NewS3Filer(s3b, c.S3Config.KMSKeyID)
}

func (c *LsCommand) Run() error {
	c.init()

	fmt.Println("accounts and domains:")

	accounts, err := c.ListAccounts()
	if err != nil {
		return err
	}

	for _, a := range accounts {
		fmt.Println("\t", a)
		store, err := agent.NewStore(a, c.filer)
		if err != nil {
			return err
		}

		domains, err := store.ListDomains()
		if err != nil {
			return err
		}

		for _, d := range domains {
			authz, err := store.LoadAuthorization(d)
			if err != nil {
				fmt.Printf("\t\t%s (Error: %s)\n", d, err)
				continue
			}

			fmt.Printf("\t\t%s (Expired at %s)\n", d, authz.Expires)
		}
	}

	return nil
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
