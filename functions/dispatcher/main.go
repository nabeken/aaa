package main

import (
	"encoding/json"
	"fmt"
	"os"

	apex "github.com/apex/go-apex"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/nabeken/aaa/slack"
	"github.com/pkg/errors"
)

func main() {
	sess := session.Must(session.NewSession())
	lambdaSvc := lambda.New(sess)

	token := os.Getenv("SLACK_TOKEN")
	executorFuncName := os.Getenv("AAA_EXECUTOR_FUNC_NAME")
	apex.HandleFunc(func(event json.RawMessage, ctx *apex.Context) (interface{}, error) {
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
			InvocationType: aws.String(lambda.InvocationTypeEvent),
			Payload:        event,
		}

		if _, err := lambdaSvc.Invoke(req); err != nil {
			return nil, errors.Wrap(err, "failed to invoke the executor")
		}

		return &slack.CommandResponse{
			ResponseType: "in_channel",
			Text:         fmt.Sprintf("<@%s> Your request has been accepted.", slcmd.UserName),
		}, nil
	})
}
