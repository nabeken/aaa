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

type dispatcher struct {
}

func (d *dispatcher) handleAuthzCommand(arg string, slcmd *slack.Command) (string, error) {
	cmd := &command.AuthzCommand{
		Challenge: challengeType,
		Domain:    arg,
	}

	if err := cmd.Execute(nil); err != nil {
		return "", err
	}

	return fmt.Sprintf(
		"@%s The authorization for %s has been renewed.",
		slcmd.UserName,
		arg,
	), nil
}

func (d *dispatcher) handleCertCommand(arg string, slcmd *slack.Command) (string, error) {
	cmd := &command.CertCommand{}

	domains := strings.Split(arg, " ")
	log.Println("domains:", domains)

	cmd.CommonName = domains[0]
	cmd.Domains = domains[1:]

	if err := cmd.Execute(nil); err != nil {
		return "", err
	}

	return fmt.Sprintf(
		"@%s The certificate for %s is now available!\n```\n"+
			"aws s3 sync s3://%s/aaa-data/%s/domain/%s %s```",
		slcmd.UserName,
		domains,
		command.Options.S3Bucket,
		cmd.CommonName,
	), nil
}

func main() {
	// initialize global command option
	command.Options.S3Bucket = os.Getenv("S3_BUCKET")
	command.Options.S3KMSKeyID = os.Getenv("KMS_KEY_ID")
	command.Options.Email = os.Getenv("EMAIL")

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

		var respStr string
		switch command[0] {
		case "cert":
			respStr, err = dispatcher.handleCertCommand(command[1], slcmd)
		case "authz":
			respStr, err = dispatcher.handleAuthzCommand(command[1], slcmd)
		}
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
