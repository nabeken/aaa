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

	return authz, cmd
}

func InstallCertCommand(app *kingpin.Application) (*kingpin.CmdClause, *command.CertCommand) {
	cmd := &command.CertCommand{}
	cert := app.Command("cert", "Issue certificate.")
	cert.Flag("cn", "Common Name to be issued").Required().StringVar(&cmd.CommonName)
	cert.Flag("domain", "Domains to be use as Subject Alternative Name").
		StringsVar(&cmd.Domains)

	cert.Flag("renewal", "Renew the certificate").BoolVar(&cmd.Renewal)
	cert.Flag("renewal-key", "Renew the key").BoolVar(&cmd.RenewalKey)

	return cert, cmd
}

func InstallLsCommand(app *kingpin.Application) (*kingpin.CmdClause, *command.LsCommand) {
	cmd := &command.LsCommand{}
	ls := app.Command("ls", "List.")
	ls.Flag("format", "Format the output").Default("json").StringVar(&cmd.Format)

	return ls, cmd
}
