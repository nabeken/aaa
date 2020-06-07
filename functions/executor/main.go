package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	golambda "github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/service/acm"
	"github.com/aws/aws-sdk-go/service/s3"
	flags "github.com/jessevdk/go-flags"
	"github.com/nabeken/aaa/agent"
	"github.com/nabeken/aaa/command"
	"github.com/nabeken/aaa/slack"
	"github.com/nabeken/aws-go-s3/bucket"
	"github.com/pkg/errors"
)

const challengeType = "dns-01"

var options struct {
	S3Bucket   string
	S3KMSKeyID string
	Email      string
}

type dispatcher struct {
}

func (d *dispatcher) handleCertCommand(arg string, slcmd *slack.Command) (string, error) {
	store, err := command.NewStore(options.Email, options.S3Bucket, options.S3KMSKeyID)
	if err != nil {
		return "", errors.Wrap(err, "failed to initialize the store")
	}

	// opts is a subset of command.CertCommand.
	var opts struct {
		CreateKey  bool `long:"create-key"`
		RSAKeySize int  `long:"rsa-key-size" default:"4096"`
	}
	domains, err := flags.ParseArgs(&opts, strings.Split(arg, " "))
	if err != nil {
		return "", err
	}

	log.Println("domains:", domains)

	// How to execute in Slack:
	// /letsencrypt [command] [domains...] [optional_arguments]
	// For example: /letsencrypt cert foo.bar.com --create-key --rsa-key-size 2048
	svc := &command.CertService{
		CommonName: domains[0],
		Domains:    domains[1:],
		CreateKey:  opts.CreateKey,
		RSAKeySize: opts.RSAKeySize,
		Store:      store,
	}

	if err := svc.Run(); err != nil {
		return "", err
	}

	return fmt.Sprintf(
		"%s The certificate for %s is now available!\n```\n"+
			"aws s3 sync 's3://%s/aaa-data/v2/%s/domain/%s/' '%s'```",
		slack.FormatUserName(slcmd.UserName),
		domains,
		options.S3Bucket,
		options.Email,
		svc.CommonName,
		svc.CommonName,
	), nil
}

func (d *dispatcher) handleUploadCommand(arg string, slcmd *slack.Command) (string, error) {
	sess := command.NewAWSSession()
	s3b := bucket.New(s3.New(sess), options.S3Bucket)
	svc := &command.UploadService{
		Domain:  arg,
		Email:   options.Email,
		S3Filer: agent.NewS3Filer(s3b, ""),
		ACMconn: acm.New(sess),
	}

	arn, err := svc.Run()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(
		"%s The certificate `%s` has been uploaded to ACM! ARN is `%s`",
		slack.FormatUserName(slcmd.UserName),
		arg,
		arn,
	), nil
}

func realmain(event json.RawMessage) (interface{}, error) {
	slcmd, err := slack.ParseCommand(event)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse the command")
	}
	log.Println("slack command:", slcmd)

	handleError := func(err error) error {
		return slack.PostErrorResponse(err, slcmd)
	}

	command := strings.SplitN(slcmd.Text, " ", 2)
	if len(command) != 2 {
		return "", handleError(errors.New("invalid command"))
	}

	dispatcher := &dispatcher{}

	var handler func(string, *slack.Command) (string, error)
	switch command[0] {
	case "cert":
		handler = dispatcher.handleCertCommand
	case "upload":
		handler = dispatcher.handleUploadCommand
	}

	respStr, err := handler(command[1], slcmd)
	if err != nil {
		return nil, handleError(err)
	}
	resp := &slack.CommandResponse{
		ResponseType: "in_channel",
		Text:         respStr,
	}
	return slack.PostResponse(slcmd.ResponseURL, resp), nil
}

func main() {
	// initialize global command option
	options.S3Bucket = os.Getenv("S3_BUCKET")
	options.S3KMSKeyID = os.Getenv("KMS_KEY_ID")
	options.Email = os.Getenv("EMAIL")

	golambda.Start(realmain)
}
