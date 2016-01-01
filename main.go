package main

import (
	"log"
	"os"

	"github.com/nabeken/aaa/command"
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
	lsCmd, ls := InstallLsCommand(app)

	s3Config := &command.S3Config{}
	app.Flag("s3-bucket", "S3 Bucket Name").Required().StringVar(&s3Config.Bucket)
	app.Flag("s3-kms-key", "KMS Key ID for S3 SSE-KMS").Required().StringVar(&s3Config.KMSKeyID)

	email := app.Flag("email", "Email Address").String()

	// Parse args
	cmd := kingpin.MustParse(app.Parse(os.Args[1:]))

	switch cmd {
	case regCmd.FullCommand():
		reg.S3Config = s3Config
		reg.Email = *email
		return reg.Run()
	case authzCmd.FullCommand():
		authz.S3Config = s3Config
		authz.Email = *email
		return authz.Run()
	case certCmd.FullCommand():
		cert.S3Config = s3Config
		cert.Email = *email
		return cert.Run()
	case lsCmd.FullCommand():
		ls.S3Config = s3Config
		ls.Email = *email
		return ls.Run()
	}

	return nil
}
