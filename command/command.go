package command

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/nabeken/aaa/v3/agent"
	"github.com/nabeken/aws-go-s3/v2/bucket"
)

// Options for the global command.
var Options struct {
	S3Bucket   string `long:"s3-bucket" description:"S3 Bucket Name" required:"true"`
	S3KMSKeyID string `long:"s3-kms-key" description:"KMS Key ID for S3 SSE-KMS"`
	Email      string `long:"email" description:"Email Address"`
}

// NewStore initializes agent.Store for cli apps.
func NewStore(email, s3Bucket, s3KMSKeyID string) (*agent.Store, error) {
	ctx := context.Background()
	s3b := bucket.New(s3.NewFromConfig(MustNewAWSConfig(ctx)), s3Bucket)
	filer := agent.NewS3Filer(s3b, s3KMSKeyID)

	store, err := agent.NewStore(email, filer)
	if err != nil {
		return nil, err
	}

	return store, nil
}
