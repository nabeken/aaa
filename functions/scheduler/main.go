package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	golambda "github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/nabeken/aaa/v3/agent"
	"github.com/nabeken/aaa/v3/command"
	"github.com/nabeken/aaa/v3/slack"
	"github.com/nabeken/aws-go-s3/v2/bucket"
)

const (
	renewalDaysBefore = 30
)

var (
	lambdaSvc *lambda.Client
	s3b       *bucket.Bucket

	slackURL   = os.Getenv("SLACK_URL")
	slackToken = os.Getenv("SLACK_TOKEN")
	s3Bucket   = os.Getenv("S3_BUCKET")
	s3KMSKeyID = os.Getenv("KMS_KEY_ID")
)

func realmain(event json.RawMessage) (any, error) {
	lsSvc := &command.LsService{
		Filer: agent.NewS3Filer(s3b, s3KMSKeyID),
	}

	executorFuncName := os.Getenv("AAA_EXECUTOR_FUNC_NAME")
	if executorFuncName == "" {
		return nil, errors.New("Please set AAA_EXECUTOR_FUNC_NAME environment variable.")
	}

	ctx := context.Background()

	domains, err := lsSvc.FetchData(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing all the domains: %w", err)
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
			return nil, fmt.Errorf("encoding the payload: %w", err)
		}

		req := &lambda.InvokeInput{
			FunctionName:   aws.String(executorFuncName),
			InvocationType: types.InvocationTypeEvent,
			Payload:        payload,
		}

		if _, err := lambdaSvc.Invoke(context.TODO(), req); err != nil {
			return nil, fmt.Errorf("invoking the executor: %w", err)
		}

		slackReq := &slack.CommandResponse{
			ResponseType: "in_channel",
			Text:         fmt.Sprintf("Invoked `%s` for renewal", cmd),
		}

		if err := slack.PostResponse(slackURL, slackReq); err != nil {
			return nil, fmt.Errorf("sending the response to Slack: %w", err)
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
	cfg := command.MustNewAWSConfig(context.Background())

	lambdaSvc = lambda.NewFromConfig(cfg)
	s3b = bucket.New(s3.NewFromConfig(cfg), s3Bucket)

	golambda.Start(realmain)
}
