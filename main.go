package main

import (
	"os"

	"github.com/jessevdk/go-flags"
	"github.com/nabeken/aaa/v3/command"
)

var (
	parser = flags.NewParser(&command.Options, flags.Default)
)

func main() {
	os.Exit(realmain())
}

func realmain() int {
	if _, err := parser.Parse(); err != nil {
		return 1
	}

	return 0
}

func mustAddCommand(
	command string,
	shortDescription string,
	longDescription string,
	data any,
) {
	if _, err := parser.AddCommand(command, shortDescription, longDescription, data); err != nil {
		panic(err)
	}
}

func init() {
	mustAddCommand(
		"reg",
		"Register an account to Let's Encrypt",
		"The reg command registers an account.",
		&command.RegCommand{},
	)
	mustAddCommand(
		"cert",
		"Issue certificates",
		"The cert command issues certificates.",
		&command.CertCommand{},
	)
	mustAddCommand(
		"ls",
		"List domains",
		"The ls command lists domains.",
		&command.LsCommand{},
	)
	mustAddCommand(
		"sync",
		"Sync the certificates",
		"The sync command synchronizes the certificates from S3.",
		&command.SyncCommand{},
	)
	mustAddCommand(
		"upload",
		"Upload the certificate to IAM",
		"The upload command uploads the certificates to IAM.",
		&command.UploadCommand{},
	)
}
