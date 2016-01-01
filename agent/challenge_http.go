package agent

import (
	"fmt"
	"strings"

	"github.com/nabeken/aws-go-s3/bucket"
	"github.com/nabeken/aws-go-s3/bucket/option"
	"github.com/nabeken/aws-go-s3/ioutils"
)

var DefaultHTTPPort = "9999"

type S3HTTPChallengeSolver struct {
	bucket    *bucket.Bucket
	challenge Challenge
	domain    string
}

func NewS3HTTPChallengeSolver(bucket *bucket.Bucket, challenge Challenge, domain string) *S3HTTPChallengeSolver {
	return &S3HTTPChallengeSolver{
		bucket:    bucket,
		challenge: challenge,
		domain:    domain,
	}
}

func (s *S3HTTPChallengeSolver) SolveChallenge(keyAuthz string) error {
	content, err := ioutils.NewFileReadSeeker(strings.NewReader(keyAuthz))
	if err != nil {
		return err
	}
	defer content.Close()

	_, err = s.bucket.PutObject(
		acmeWellKnownPath(s.challenge.Token),
		content,
		option.ContentLength(int64(len(keyAuthz))),
		option.ACLPublicRead(),
	)
	return err
}

func (s *S3HTTPChallengeSolver) CleanupChallenge(keyAuthz string) error {
	_, err := s.bucket.DeleteObject(acmeWellKnownPath(s.challenge.Token))
	return err
}

func acmeWellKnownPath(token string) string {
	return fmt.Sprintf("/.well-known/acme-challenge/%s", token)
}
