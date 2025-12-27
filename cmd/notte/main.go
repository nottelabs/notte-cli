package main

import (
	"github.com/salmonumbrella/notte-cli/internal/cmd"
)

// Set via ldflags
var version = "dev"

func main() {
	cmd.Version = version
	cmd.Execute()
}
