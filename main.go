package main

import (
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/nabeken/aaa/agent"
	"github.com/nabeken/aws-go-s3/bucket"
	"gopkg.in/alecthomas/kingpin.v2"
)

func main() {
	if err := realMain(); err != nil {
		log.Fatal("ERROR: ", err)
	}
}

func realMain() error {
	app := kingpin.New("aaa", "ACME Agent For AWS environment")

	regCmd, reg := InstallRegCommand(app)
	authzCmd, authz := InstallAuthzCommand(app)
	certCmd, cert := InstallCertCommand(app)

	email := app.Flag("email", "Email Address").Required().String()
	s3Bucket := app.Flag("s3-bucket", "S3 Bucket Name").String()

	cmd := kingpin.MustParse(app.Parse(os.Args[1:]))

	// initialize store and client
	s3b := bucket.New(s3.New(session.New()), *s3Bucket)
	store, err := agent.NewStore(*email, s3b)
	if err != nil {
		return err
	}

	dirURL := agent.DefaultDirectoryURL
	if url := os.Getenv("AAA_DIRECTORY_URL"); url != "" {
		dirURL = url
	}

	client := agent.NewClient(dirURL, store)

	// initialize client except for new-registration
	if cmd != regCmd.FullCommand() {
		if err := client.Init(); err != nil {
			return err
		}
	}

	switch cmd {
	case regCmd.FullCommand():
		reg.Email = *email
		reg.Client = client
		reg.Store = store
		return reg.Run()
	case authzCmd.FullCommand():
		authz.Client = client
		authz.Store = store
		authz.Bucket = s3b
		return authz.Run()
	case certCmd.FullCommand():
		cert.Client = client
		cert.Store = store
		return cert.Run()
	}

	return nil
}
