package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	golambda "github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/lambda/types"
	"github.com/nabeken/aaa/command"
	"github.com/nabeken/aaa/slack"
	"github.com/pkg/errors"
)

var lambdaSvc *lambda.Client

func realmain(event json.RawMessage) (*slack.CommandResponse, error) {
	token := os.Getenv("SLACK_TOKEN")
	executorFuncName := os.Getenv("AAA_EXECUTOR_FUNC_NAME")

	if executorFuncName == "" {
		return nil, errors.New("Please set AAA_EXECUTOR_FUNC_NAME environment variable.")
	}

	slcmd, err := slack.ParseCommand(event)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse the command")
	}

	if slcmd.Token != token {
		return nil, errors.New("Who are you? Token does not match.")
	}

	req := &lambda.InvokeInput{
		FunctionName:   aws.String(executorFuncName),
		InvocationType: types.InvocationTypeEvent,
		Payload:        event,
	}

	ctx := context.Background()

	if _, err := lambdaSvc.Invoke(ctx, req); err != nil {
		return nil, errors.Wrap(err, "failed to invoke the executor")
	}

	resp := &slack.CommandResponse{
		ResponseType: "in_channel",
		Text:         fmt.Sprintf("%s Your request has been accepted.", slack.FormatUserName(slcmd.UserName)),
	}

	return resp, nil
}

func main() {
	lambdaSvc = lambda.NewFromConfig(command.MustNewAWSConfig(context.Background()))

	golambda.Start(realmain)
}
