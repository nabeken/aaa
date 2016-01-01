package command

import (
	"os"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/nabeken/aaa/agent"
	"github.com/nabeken/aws-go-s3/bucket"
)

type S3Config struct {
	Bucket   string
	KMSKeyID string
}

func DirectoryURL() string {
	dirURL := agent.DefaultDirectoryURL
	if url := os.Getenv("AAA_DIRECTORY_URL"); url != "" {
		dirURL = url
	}
	return dirURL
}

// newStore initializes agent.Store for cli apps.
func newStore(email string, c *S3Config) (*agent.Store, error) {
	s3b := bucket.New(s3.New(session.New()), c.Bucket)
	filer := agent.NewS3Filer(s3b, c.KMSKeyID)
	store, err := agent.NewStore(email, filer)
	if err != nil {
		return nil, err
	}

	return store, nil
}
