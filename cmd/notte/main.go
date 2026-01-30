package main

import (
	"github.com/nottelabs/notte-cli/internal/cmd"
)

// Set via ldflags
var version = "dev"

func main() {
	cmd.Version = version
	cmd.Execute()
}
