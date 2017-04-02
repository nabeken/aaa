package command

import (
	"errors"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/nabeken/aaa/agent"
	"github.com/nabeken/aws-go-s3/bucket"
)

type AuthzCommand struct {
	Domain    string `long:"domain" description:"Domain to be authorized" required:"true"`
	Challenge string `long:"challenge" description:"Challenge Type" default:"dns-01"`
}

func (c *AuthzCommand) Execute(args []string) error {
	// initialize S3 bucket and filer
	s3b := bucket.New(s3.New(session.New()), Options.S3Bucket)
	filer := agent.NewS3Filer(s3b, Options.S3KMSKeyID)
	store, err := agent.NewStore(Options.Email, filer)
	if err != nil {
		return err
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
