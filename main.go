package main

import (
	"os"

	"github.com/jessevdk/go-flags"
	"github.com/nabeken/aaa/command"
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

func init() {
	parser.AddCommand(
		"reg",
		"Register an account to Let's Encrypt",
		"The reg command registers an account.",
		&command.RegCommand{},
	)
	parser.AddCommand(
		"cert",
		"Issue certificates",
		"The cert command issues certificates.",
		&command.CertCommand{},
	)
	parser.AddCommand(
		"ls",
		"List domains",
		"The ls command lists domains.",
		&command.LsCommand{},
	)
	parser.AddCommand(
		"sync",
		"Sync the certificates",
		"The sync command synchronizes the certificates from S3.",
		&command.SyncCommand{},
	)
	parser.AddCommand(
		"upload",
		"Upload the certificate to IAM",
		"The upload command uploads the certificates to IAM.",
		&command.UploadCommand{},
	)
}
