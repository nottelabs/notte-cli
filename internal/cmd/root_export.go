package cmd

import "github.com/spf13/cobra"

// RootCommand exposes the assembled command tree.
//
// Only the coverage checker in scripts/ uses it: enumerating commands by
// walking cobra is the one way to get a list that cannot drift from the CLI,
// which parsing --help output or maintaining a second list both can.
func RootCommand() *cobra.Command {
	return rootCmd
}
