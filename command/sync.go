package command

import (
	"log"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/nabeken/aaa/agent"
	"github.com/nabeken/aws-go-s3/bucket"
)

type SyncCommand struct {
	Domain string `long:"domain" description:"Domain to be synced" required:"true"`

	s3Filer *agent.S3Filer
	osFiler *agent.OSFiler
}

func (c *SyncCommand) init() {
	s3b := bucket.New(s3.New(session.New()), Options.S3Bucket)
	c.s3Filer = agent.NewS3Filer(s3b, "")
	c.osFiler = &agent.OSFiler{c.Domain}
}

func (c *SyncCommand) Execute(args []string) error {
	c.init()

	for _, fn := range []string{
		"privkey.pem",
		"fullchain.pem",
		"cert.pem",
		"chain.pem",
	} {
		key := c.s3Filer.Join("aaa-data", Options.Email, "domain", c.Domain, fn)
		blob, err := c.s3Filer.ReadFile(key)
		if err != nil {
			log.Printf("aaa: failed to read '%s' data from S3: %s", fn, err)
			return err
		}
		if err := c.osFiler.WriteFile(fn, blob); err != nil {
			log.Printf("aaa: failed to write '%s' data: %s", fn, err)
			return err
		}
		log.Printf("aaa: %s synced", fn)
	}

	return nil
}
