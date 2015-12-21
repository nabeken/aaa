package main

import (
	"os"

	"gopkg.in/alecthomas/kingpin.v2"
)

func main() {
	app := kingpin.New("aaa", "ACME Agent For AWS")

	for _, install := range []func(*kingpin.Application){
		InstallRegCommand,
		InstallAuthzCommand,
		InstallCertCommand,
	} {
		install(app)
	}

	kingpin.MustParse(app.Parse(os.Args[1:]))
}
