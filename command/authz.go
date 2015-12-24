package command

import (
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/nabeken/aaa/agent"
	"github.com/nabeken/aws-go-s3/bucket"
)

type AuthzCommand struct {
	Domain    string
	Challenge string

	Client *agent.Client
	Store  *agent.Store
	Bucket *bucket.Bucket
}

func (c *AuthzCommand) Run() error {
	// If we have authorized domain, we skip authorization request.
	if authz, err := c.Store.LoadAuthorization(c.Domain); err != nil && err != agent.ErrFileNotFound {
		// something is wrong
		return err
	} else if err == nil {
		agent.Debug("previous authorization will be expired at ", authz.Expires)

		if !authz.IsExpired(time.Now()) {
			log.Printf("INFO: authorization for %s has been done. See %s", c.Domain, authz.URL)
			return nil
		}

		// re-authorization is required.
		log.Printf("INFO: previous authorization is expired. re-authorization is required for %s", c.Domain)
	}

	log.Printf("INFO: start authorization for %s with %s", c.Domain, c.Challenge)

	newAuthzReq := &agent.NewAuthorizationRequest{
		Identifier: &agent.Identifier{
			Type:  "dns",
			Value: c.Domain,
		},
	}

	authzResp, err := c.Client.NewAuthorization(newAuthzReq)
	if err != nil {
		return err
	}

	log.Printf("INFO: authorization: %s", authzResp.URL)

	// as of 2015/12/15, DNS-01 challenge is broken on LE's end
	// so we now use HTTP-01 challenge instead....
	var challenge agent.Challenge
	var challengeSolver agent.ChallengeSolver

	switch c.Challenge {
	case "http-01":
		httpPort := agent.DefaultHTTPPort
		if port := os.Getenv("AAA_HTTP_PORT"); port != "" {
			httpPort = port
		}
		httpChallenge, found := agent.FindHTTPChallenge(authzResp)
		if !found {
			return errors.New("aaa: no HTTP challenge and its combination found")
		}
		challenge = httpChallenge
		challengeSolver = agent.NewHTTPChallengeSolver(httpChallenge, c.Domain, httpPort)

	case "s3-http-01":
		httpChallenge, found := agent.FindHTTPChallenge(authzResp)
		if !found {
			return errors.New("aaa: no HTTP challenge and its combination found")
		}

		challenge = httpChallenge
		challengeSolver = agent.NewS3HTTPChallengeSolver(c.Bucket, httpChallenge, c.Domain)

	case "dns-01":
		dnsChallenge, found := agent.FindDNSChallenge(authzResp)
		if !found {
			return errors.New("aaa: no DNS challenge and its combination found")
		}

		r53 := agent.NewRoute53Provider(route53.New(session.New()))
		challenge = dnsChallenge
		challengeSolver = agent.NewDNSChallengeSolver(r53, dnsChallenge, c.Domain)
	default:
		return fmt.Errorf("aaa: challenge %s is not supported")
	}

	publicKey, err := c.Store.LoadPublicKey()
	if err != nil {
		return err
	}

	keyAuthz, err := agent.BuildKeyAuthorization(challenge.Token, publicKey)
	if err != nil {
		return err
	}

	agent.Debug("KeyAuthorization: ", keyAuthz)

	if err := challengeSolver.SolveChallenge(keyAuthz); err != nil {
		return err
	}

	if err := c.Client.SolveChallenge(challenge, keyAuthz); err != nil {
		return err
	}

	if err := c.Client.WaitChallengeDone(challenge); err != nil {
		log.Print("INFO: challenge has been failed")
		return err
	}

	if err := challengeSolver.CleanupChallenge(keyAuthz); err != nil {
		return err
	}

	// getting the latest authorization status
	currentAuthz, err := c.Client.GetAuthorization(authzResp.URL)
	if err != nil {
		return err
	}

	if err := c.Store.SaveAuthorization(currentAuthz); err != nil {
		return err
	}

	log.Print("INFO: challenge has been solved")

	return nil
}
