package command

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/nabeken/aaa/v3/agent"
	"github.com/nabeken/aws-go-s3/v2/bucket"
	"github.com/pkg/errors"
)

type LsCommand struct {
	Format string `long:"format" description:"Format the output" default:"json"`
}

func (c *LsCommand) Execute(args []string) error {
	ctx := context.Background()
	s3b := bucket.New(s3.NewFromConfig(MustNewAWSConfig(ctx)), Options.S3Bucket)

	return (&LsService{
		Filer: agent.NewS3Filer(s3b, Options.S3KMSKeyID),
	}).WriteTo(ctx, c.Format, os.Stdout)
}

type LsService struct {
	Filer agent.Filer
}

func (svc *LsService) WriteTo(ctx context.Context, format string, w io.Writer) error {
	output, err := svc.FetchData(ctx)
	if err != nil {
		return err
	}

	switch format {
	case "json":
		return json.NewEncoder(w).Encode(output)
	default:
		return errors.Errorf("'%s' is not implemented", format)
	}
}

func (svc *LsService) FetchData(ctx context.Context) ([]Domain, error) {
	data := []Domain{}

	emails, err := svc.listAccounts(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list the accounts")
	}

	for _, email := range emails {
		store, err := agent.NewStore(email, svc.Filer)
		if err != nil {
			return nil, errors.Wrap(err, "failed to initialize the store")
		}

		domains, err := store.ListDomains(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed to list the domains")
		}

		for _, dom := range domains {
			cert, err := store.LoadCert(ctx, dom)
			if err != nil {
				log.Printf("failed to load certificate for %s: %s (or new-cert is ongoing or this domain is in SAN in other certificates). skipping...", dom, err)
				continue
			}

			data = append(data, Domain{
				Email:  email,
				Domain: dom,
				Certificate: Certificate{
					NotBefore: cert.NotBefore,
					NotAfter:  cert.NotAfter,
					SAN:       cert.DNSNames,
				},
			})
		}
	}

	return data, nil
}

func (svc *LsService) listAccounts(ctx context.Context) ([]string, error) {
	dirs, err := svc.Filer.ListDir(ctx, agent.StorePrefix)
	if err != nil {
		return nil, err
	}

	accounts := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		elem := svc.Filer.Split(dir)

		// account (email) is in 2nd element
		if len(elem) > 1 {
			accounts = append(accounts, elem[1])
		}
	}

	return accounts, nil
}

type Domain struct {
	Email       string      `json:"email"`
	Domain      string      `json:"domain"`
	Certificate Certificate `json:"certificate"`
}

type Certificate struct {
	NotBefore time.Time `json:"not_before"`
	NotAfter  time.Time `json:"not_after"`
	SAN       []string  `json:"san"`
}
