package command

import "gopkg.in/alecthomas/kingpin.v2"

type VersionCommand struct {
	Name     string
	Version  string
	Revision string
}

func (c *VersionCommand) Run(ctx *kingpin.ParseContext) error {
	return nil
}
