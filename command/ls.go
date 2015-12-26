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
}

func (c *LsCommand) Run() error {
	s3b := bucket.New(s3.New(session.New()), c.S3Config.Bucket)
	filer := agent.NewS3Filer(s3b, c.S3Config.KMSKeyID)
	fmt.Println(filer.ListDir(""))
	return nil
}
