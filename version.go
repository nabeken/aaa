package main

const Name string = "aaa"
const Version string = "2.0.0"

// GitCommit describes latest commit hash.
// This value is extracted by git command when building.
// To set this from outside, use go build -ldflags "-X main.GitCommit=\"$(COMMIT)\""
var GitCommit string
