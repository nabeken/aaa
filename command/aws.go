package command

import "github.com/aws/aws-sdk-go/aws/session"

func NewAWSSession() *session.Session {
	return session.Must(session.NewSession())
}
