package main

import (
	"github.com/bethropolis/localgo/cmd/localgo/cmd"
)

// These are populated at link time via -ldflags -X main.Version=...
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

func main() {
	cmd.SetVersionInfo(Version, GitCommit, BuildDate)
	cmd.Execute()
}
