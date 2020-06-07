package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	golambda "github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/nabeken/aaa/agent"
	"github.com/nabeken/aaa/command"
	"github.com/nabeken/aaa/slack"
	"github.com/nabeken/aws-go-s3/bucket"
	"github.com/pkg/errors"
)

const (
	renewalDaysBefore = 30
)

var (
	lambdaSvc *lambda.Lambda
	s3b       *bucket.Bucket

	slackURL   = os.Getenv("SLACK_URL")
	slackToken = os.Getenv("SLACK_TOKEN")
	s3Bucket   = os.Getenv("S3_BUCKET")
	s3KMSKeyID = os.Getenv("KMS_KEY_ID")
)

func realmain(event json.RawMessage) (interface{}, error) {
	lsSvc := &command.LsService{
		Filer: agent.NewS3Filer(s3b, s3KMSKeyID),
	}

	executorFuncName := os.Getenv("AAA_EXECUTOR_FUNC_NAME")
	if executorFuncName == "" {
		return nil, errors.New("Please set AAA_EXECUTOR_FUNC_NAME environment variable.")
	}

	domains, err := lsSvc.FetchData()
	if err != nil {
		return nil, errors.Wrap(err, "failed to list all domains")
	}

	now := time.Now()
	renewalDate := now.AddDate(0, 0, renewalDaysBefore)
	renewCommands := []string{}
	for _, domain := range domains {
		if domain.Certificate.NotAfter.Before(renewalDate) {
			renewCommands = append(renewCommands, "cert "+domain.Domain)
		}
	}

	log.Printf("renewCommands: %s", renewCommands)

	// invoking the executor
	for _, cmd := range renewCommands {
		slcmd := &slack.Command{
			Token:       slackToken,
			UserName:    "here",
			ResponseURL: slackURL,
			Command:     "/letsencrypt",
			Text:        cmd,
		}
		payload, err := json.Marshal(slcmd)
		if err != nil {
			return nil, errors.Wrap(err, "failed to encode the payload")
		}
		req := &lambda.InvokeInput{
			FunctionName:   aws.String(executorFuncName),
			InvocationType: aws.String(lambda.InvocationTypeEvent),
			Payload:        payload,
		}
		if _, err := lambdaSvc.Invoke(req); err != nil {
			return nil, errors.Wrap(err, "failed to invoke the executor")
		}

		slackReq := &slack.CommandResponse{
			ResponseType: "in_channel",
			Text:         fmt.Sprintf("Invoked `%s` for renewal", cmd),
		}
		if err := slack.PostResponse(slackURL, slackReq); err != nil {
			return nil, errors.Wrap(err, "failed to send a response to Slack")
		}
	}

	if len(renewCommands) == 0 {
		resp := &slack.CommandResponse{
			ResponseType: "in_channel",
			Text:         "checking renewal but no authz and cert found to be renewal",
		}
		return slack.PostResponse(slackURL, resp), nil
	}

	return nil, nil
}

func main() {
	sess := command.NewAWSSession()

	lambdaSvc = lambda.New(sess)
	s3b = bucket.New(s3.New(command.NewAWSSession()), s3Bucket)

	golambda.Start(realmain)
}
