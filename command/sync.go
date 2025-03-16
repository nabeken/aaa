package command

import (
	"context"
	"log"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/nabeken/aaa/v3/agent"
	"github.com/nabeken/aws-go-s3/v2/bucket"
)

type SyncCommand struct {
	Domain string `long:"domain" description:"Domain to be synced" required:"true"`

	s3Filer *agent.S3Filer
	osFiler *agent.OSFiler
}

func (c *SyncCommand) init(ctx context.Context) {
	s3b := bucket.New(s3.NewFromConfig(MustNewAWSConfig(ctx)), Options.S3Bucket)
	c.s3Filer = agent.NewS3Filer(s3b, "")
	c.osFiler = &agent.OSFiler{BaseDir: c.Domain}
}

func (c *SyncCommand) Execute(args []string) error {
	ctx := context.Background()

	c.init(ctx)

	for _, fn := range []string{
		"privkey.pem",
		"cert.pem",
	} {
		key := c.s3Filer.Join("aaa-data", Options.Email, "domain", c.Domain, fn)

		blob, err := c.s3Filer.ReadFile(ctx, key)
		if err != nil {
			log.Printf("aaa: failed to read '%s' data from S3: %s", fn, err)
			return err
		}

		if err := c.osFiler.WriteFile(ctx, fn, blob); err != nil {
			log.Printf("aaa: failed to write '%s' data: %s", fn, err)
			return err
		}

		log.Printf("aaa: %s synced", fn)
	}

	return nil
}
