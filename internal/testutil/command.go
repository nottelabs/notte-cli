// internal/testutil/command.go
package testutil

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
)

// CommandResult holds the results of running a command
type CommandResult struct {
	Stdout string
	Stderr string
	Err    error
}

// RunCommand executes a cobra command and captures output
func RunCommand(t *testing.T, cmd *cobra.Command, args []string) CommandResult {
	t.Helper()

	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)

	// Silence usage on error to keep output clean
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	err := cmd.Execute()

	return CommandResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
		Err:    err,
	}
}

// RunCommandWithEnv executes a command with custom environment
func RunCommandWithEnv(t *testing.T, cmd *cobra.Command, args []string, env *TestEnv) CommandResult {
	t.Helper()
	return RunCommand(t, cmd, args)
}
