package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	apex "github.com/apex/go-apex"
	"github.com/nabeken/aaa/command"
	"github.com/nabeken/aaa/slack"
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

func (d *dispatcher) handleAuthzCommand(arg string, slcmd *slack.Command) (string, error) {
	svc := &command.AuthzService{
		Challenge:  challengeType,
		Domain:     arg,
		S3Bucket:   options.S3Bucket,
		S3KMSKeyID: options.S3KMSKeyID,
		Email:      options.Email,
	}

	if err := svc.Run(); err != nil {
		return "", err
	}

	return fmt.Sprintf(
		"@%s The authorization for %s has been renewed.",
		slcmd.UserName,
		arg,
	), nil
}

func (d *dispatcher) handleCertCommand(arg string, slcmd *slack.Command) (string, error) {
	domains := strings.Split(arg, " ")
	log.Println("domains:", domains)

	svc := &command.CertService{
		CommonName: domains[0],
		Domains:    domains[1:],
		S3Bucket:   options.S3Bucket,
		S3KMSKeyID: options.S3KMSKeyID,
		Email:      options.Email,
	}

	if err := svc.Run(); err != nil {
		return "", err
	}

	return fmt.Sprintf(
		"@%s The certificate for %s is now available!\n```\n"+
			"aws s3 sync s3://%s/aaa-data/%s/domain/%s %s```",
		slcmd.UserName,
		domains,
		options.S3Bucket,
		svc.CommonName,
	), nil
}

func (d *dispatcher) handleUploadCommand(arg string, slcmd *slack.Command) (string, error) {
	svc := &command.UploadService{
		Domain:   arg,
		S3Bucket: options.S3Bucket,
		Email:    options.Email,
	}

	arn, err := svc.Run()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(
		"@%s The certificate `%s` has been uploaded to IAM! ARN is `%s`",
		slcmd.UserName,
		arg,
		arn,
	), nil
}

func main() {
	// initialize global command option
	options.S3Bucket = os.Getenv("S3_BUCKET")
	options.S3KMSKeyID = os.Getenv("KMS_KEY_ID")
	options.Email = os.Getenv("EMAIL")

	dispatcher := &dispatcher{}

	apex.HandleFunc(func(event json.RawMessage, ctx *apex.Context) (interface{}, error) {
		slcmd, err := slack.ParseCommand(event)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse the command")
		}
		log.Println("slack command:", slcmd)

		handleError := func(err error) error {
			return slack.PostErrorResponse(err, slcmd)
		}

		command := strings.SplitN(slcmd.Text, " ", 2)
		if len(command) != 2 {
			return nil, handleError(errors.New("invalid command"))
		}

		var handler func(string, *slack.Command) (string, error)
		switch command[0] {
		case "cert":
			handler = dispatcher.handleCertCommand
		case "authz":
			handler = dispatcher.handleAuthzCommand
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
		return nil, slack.PostResponse(slcmd.ResponseURL, resp)
	})
}
