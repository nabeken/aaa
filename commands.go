package main

import (
	"github.com/nabeken/aaa/command"
	"gopkg.in/alecthomas/kingpin.v2"
)

func InstallRegCommand(app *kingpin.Application) {
	cmd := &command.RegCommand{}
	reg := app.Command("reg", "Register account.").Action(cmd.Run)
	reg.Flag("email", "Email Address for registration").Required().StringVar(&cmd.Email)
	reg.Flag("agree", "Agree with given TOS").StringVar(&cmd.AgreeTOS)
}

func InstallAuthzCommand(app *kingpin.Application) {
	cmd := &command.AuthzCommand{}
	authz := app.Command("authz", "Authorize domain.").Action(cmd.Run)
	authz.Flag("email", "Email Address used for registration").Required().StringVar(&cmd.Email)
	authz.Flag("domain", "Domain to be authorized").Required().StringVar(&cmd.Domain)
	authz.Flag("challenge", "Challenge Type").Default("http-01").StringVar(&cmd.Challenge)
	authz.Flag("s3-bucket", "S3 Bucket Name to be used with s3-http-01").StringVar(&cmd.S3Bucket)
}

func InstallCertCommand(app *kingpin.Application) {
	cmd := &command.CertCommand{}
	cert := app.Command("cert", "Issue certificate.").Action(cmd.Run)
	cert.Flag("email", "Email Address used for registration").Required().StringVar(&cmd.Email)
	cert.Flag("cn", "Common Name to be issued").Required().StringVar(&cmd.CommonName)
	cert.Flag("domain", "Domains to be use as Subject Alternative Name").
		StringsVar(&cmd.Domains)

	cert.Flag("renewal", "Renew the certificate").BoolVar(&cmd.Renewal)
}
