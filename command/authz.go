package command

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/nabeken/aaa/agent"
	"github.com/nabeken/aws-go-s3/bucket"
)

type AuthzCommand struct {
	S3Config *S3Config
	Email    string

	Domain    string
	Challenge string

	Renewal bool
}

func (c *AuthzCommand) Run() error {
	// initialize S3 bucket and filer
	s3b := bucket.New(s3.New(session.New()), c.S3Config.Bucket)
	filer := agent.NewS3Filer(s3b, c.S3Config.KMSKeyID)
	store, err := agent.NewStore(c.Email, filer)
	if err != nil {
		return err
	}

	// If we have authorized domain, we skip authorization request.
	if authz, err := store.LoadAuthorization(c.Domain); err != nil && err != agent.ErrFileNotFound {
		// something is wrong
		return err
	} else if err == nil && !c.Renewal {
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

	// initialize client here
	client := agent.NewClient(DirectoryURL(), store)
	if err := client.Init(); err != nil {
		return err
	}

	authzResp, err := client.NewAuthorization(newAuthzReq)
	if err != nil {
		return err
	}

	log.Printf("INFO: authorization: %s", authzResp.URL)

	// as of 2015/12/15, DNS-01 challenge is broken on LE's end
	// so we now use HTTP-01 challenge instead....
	var challenge agent.Challenge
	var challengeSolver agent.ChallengeSolver

	switch c.Challenge {
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

	publicKey, err := store.LoadPublicKey()
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

	if err := client.SolveChallenge(challenge, keyAuthz); err != nil {
		return err
	}

	if err := client.WaitChallengeDone(challenge); err != nil {
		log.Print("INFO: challenge has been failed")
		return err
	}

	if err := challengeSolver.CleanupChallenge(keyAuthz); err != nil {
		return err
	}

	// getting the latest authorization status
	currentAuthz, err := client.GetAuthorization(authzResp.URL)
	if err != nil {
		return err
	}

	if err := store.SaveAuthorization(currentAuthz); err != nil {
		return err
	}

	log.Print("INFO: challenge has been solved")

	return nil
}
