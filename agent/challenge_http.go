package agent

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

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

type HTTPChallengeSolver struct {
	domain     string
	ln         net.Listener
	httpPort   string
	httpServer *HTTPProvider
	challenge  Challenge
}

func NewHTTPChallengeSolver(challenge Challenge, domain, httpPort string) *HTTPChallengeSolver {
	return &HTTPChallengeSolver{
		challenge: challenge,
		domain:    domain,
		httpPort:  httpPort,
	}
}

func (s *HTTPChallengeSolver) SolveChallenge(keyAuthz string) error {
	ln, err := net.Listen("tcp", ":"+s.httpPort)
	if err != nil {
		return err
	}
	s.ln = ln

	go func() {
		log.Print("INFO: starting HTTP server for HTTP challenge on ", ln.Addr())
		httpServer := &HTTPProvider{
			Token:            s.challenge.Token,
			KeyAuthorization: keyAuthz,
		}
		httpServer.Serve(s.ln)
		log.Print("INFO: HTTP server for HTTP challenge is closed")
	}()

	// wait for httpServer launched for 1 second
	time.Sleep(1 * time.Second)

	return nil
}

func (s *HTTPChallengeSolver) CleanupChallenge(_ string) error {
	return s.ln.Close()
}

type HTTPProvider struct {
	Token            string
	KeyAuthorization string
}

func (p *HTTPProvider) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if req.Method != "GET" {
		http.Error(rw, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	if req.URL.Path != acmeWellKnownPath(p.Token) {
		http.NotFound(rw, req)
		return
	}

	fmt.Fprint(rw, p.KeyAuthorization)
}

func (p *HTTPProvider) Serve(l net.Listener) error {
	return (&http.Server{Handler: p}).Serve(l)
}

func acmeWellKnownPath(token string) string {
	return fmt.Sprintf("/.well-known/acme-challenge/%s", token)
}
