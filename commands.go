package main

import (
	"github.com/nabeken/aaa/command"
	"gopkg.in/alecthomas/kingpin.v2"
)

func InstallRegCommand(app *kingpin.Application) (*kingpin.CmdClause, *command.RegCommand) {
	cmd := &command.RegCommand{}

	reg := app.Command("reg", "Register account.")
	reg.Flag("agree", "Agree with given TOS").StringVar(&cmd.AgreeTOS)

	return reg, cmd
}

func InstallAuthzCommand(app *kingpin.Application) (*kingpin.CmdClause, *command.AuthzCommand) {
	cmd := &command.AuthzCommand{}
	authz := app.Command("authz", "Authorize domain.")
	authz.Flag("domain", "Domain to be authorized").Required().StringVar(&cmd.Domain)
	authz.Flag("challenge", "Challenge Type").Default("http-01").StringVar(&cmd.Challenge)
	authz.Flag("renewal", "Renew the authorization").BoolVar(&cmd.Renewal)

	return authz, cmd
}

func InstallCertCommand(app *kingpin.Application) (*kingpin.CmdClause, *command.CertCommand) {
	cmd := &command.CertCommand{}
	cert := app.Command("cert", "Issue certificate.")
	cert.Flag("cn", "Common Name to be issued").Required().StringVar(&cmd.CommonName)
	cert.Flag("domain", "Domains to be use as Subject Alternative Name").
		StringsVar(&cmd.Domains)

	cert.Flag("create-key", "Create the key").BoolVar(&cmd.CreateKey)

	return cert, cmd
}

func InstallLsCommand(app *kingpin.Application) (*kingpin.CmdClause, *command.LsCommand) {
	cmd := &command.LsCommand{}
	ls := app.Command("ls", "List.")
	ls.Flag("format", "Format the output").Default("json").StringVar(&cmd.Format)

	return ls, cmd
}

func InstallSyncCommand(app *kingpin.Application) (*kingpin.CmdClause, *command.SyncCommand) {
	cmd := &command.SyncCommand{}
	sync := app.Command("sync", "Sync with the certificate in S3")
	sync.Flag("domain", "Domain to be synced").Required().StringVar(&cmd.Domain)

	return sync, cmd
}

func InstallUploadCommand(app *kingpin.Application) (*kingpin.CmdClause, *command.UploadCommand) {
	cmd := &command.UploadCommand{}
	upload := app.Command("upload", "Upload the certificate to IAM")
	upload.Flag("domain", "Domain to be uploaded").Required().StringVar(&cmd.Domain)

	return upload, cmd
}
