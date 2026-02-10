package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// TestNoGenericIDFlags ensures no command uses a generic "--id" flag.
// All ID flags should be resource-specific (e.g., --session-id, --function-id).
func TestNoGenericIDFlags(t *testing.T) {
	var violations []string

	// Recursive function to check all commands
	checkCommand := func(path string, flags *pflag.FlagSet, flagType string) {
		flags.VisitAll(func(f *pflag.Flag) {
			if f.Name == "id" {
				violations = append(violations, path+" has "+flagType+" --id flag (should be resource-specific)")
			}
		})
	}

	// Check pageCmd and subcommands
	checkCommand("notte page", pageCmd.PersistentFlags(), "persistent")
	checkCommand("notte page", pageCmd.Flags(), "local")
	for _, sub := range pageCmd.Commands() {
		cmdPath := "notte page " + sub.Name()
		checkCommand(cmdPath, sub.Flags(), "local")
		checkCommand(cmdPath, sub.PersistentFlags(), "persistent")
	}

	// Check filesCmd and subcommands
	checkCommand("notte files", filesCmd.PersistentFlags(), "persistent")
	checkCommand("notte files", filesCmd.Flags(), "local")
	for _, sub := range filesCmd.Commands() {
		cmdPath := "notte files " + sub.Name()
		checkCommand(cmdPath, sub.Flags(), "local")
		checkCommand(cmdPath, sub.PersistentFlags(), "persistent")
	}

	// Check functionsCmd and subcommands
	checkCommand("notte functions", functionsCmd.PersistentFlags(), "persistent")
	checkCommand("notte functions", functionsCmd.Flags(), "local")
	for _, sub := range functionsCmd.Commands() {
		cmdPath := "notte functions " + sub.Name()
		checkCommand(cmdPath, sub.Flags(), "local")
		checkCommand(cmdPath, sub.PersistentFlags(), "persistent")
	}

	// Check vaultsCmd and subcommands
	checkCommand("notte vaults", vaultsCmd.PersistentFlags(), "persistent")
	checkCommand("notte vaults", vaultsCmd.Flags(), "local")
	for _, sub := range vaultsCmd.Commands() {
		cmdPath := "notte vaults " + sub.Name()
		checkCommand(cmdPath, sub.Flags(), "local")
		checkCommand(cmdPath, sub.PersistentFlags(), "persistent")
		// Check nested subcommands (e.g., vaults credentials)
		for _, nested := range sub.Commands() {
			nestedPath := cmdPath + " " + nested.Name()
			checkCommand(nestedPath, nested.Flags(), "local")
			checkCommand(nestedPath, nested.PersistentFlags(), "persistent")
		}
	}

	// Check profilesCmd and subcommands
	checkCommand("notte profiles", profilesCmd.PersistentFlags(), "persistent")
	checkCommand("notte profiles", profilesCmd.Flags(), "local")
	for _, sub := range profilesCmd.Commands() {
		cmdPath := "notte profiles " + sub.Name()
		checkCommand(cmdPath, sub.Flags(), "local")
		checkCommand(cmdPath, sub.PersistentFlags(), "persistent")
	}

	// Check personasCmd and subcommands
	checkCommand("notte personas", personasCmd.PersistentFlags(), "persistent")
	checkCommand("notte personas", personasCmd.Flags(), "local")
	for _, sub := range personasCmd.Commands() {
		cmdPath := "notte personas " + sub.Name()
		checkCommand(cmdPath, sub.Flags(), "local")
		checkCommand(cmdPath, sub.PersistentFlags(), "persistent")
	}

	// Check agentsCmd and subcommands
	checkCommand("notte agents", agentsCmd.PersistentFlags(), "persistent")
	checkCommand("notte agents", agentsCmd.Flags(), "local")
	for _, sub := range agentsCmd.Commands() {
		cmdPath := "notte agents " + sub.Name()
		checkCommand(cmdPath, sub.Flags(), "local")
		checkCommand(cmdPath, sub.PersistentFlags(), "persistent")
	}

	// Check sessionsCmd and subcommands
	checkCommand("notte sessions", sessionsCmd.PersistentFlags(), "persistent")
	checkCommand("notte sessions", sessionsCmd.Flags(), "local")
	for _, sub := range sessionsCmd.Commands() {
		cmdPath := "notte sessions " + sub.Name()
		checkCommand(cmdPath, sub.Flags(), "local")
		checkCommand(cmdPath, sub.PersistentFlags(), "persistent")
	}

	if len(violations) > 0 {
		t.Errorf("Found %d command(s) with generic --id flag:\n", len(violations))
		for _, v := range violations {
			t.Errorf("  - %s", v)
		}
		t.Error("All ID flags should be resource-specific (e.g., --session-id, --function-id, --vault-id)")
	}
}

// TestPaginatedListCommandsHaveFlags ensures all paginated list commands
// expose --page, --page-size, and their resource-specific filter flags.
func TestPaginatedListCommandsHaveFlags(t *testing.T) {
	paginatedCommands := []struct {
		cmd           *cobra.Command
		path          string
		requiredFlags []string
	}{
		{sessionsListCmd, "sessions list", []string{"page", "page-size", "only-active"}},
		{agentsListCmd, "agents list", []string{"page", "page-size", "only-active", "only-saved"}},
		{functionsListCmd, "functions list", []string{"page", "page-size", "only-active"}},
		{functionsRunsCmd, "functions runs", []string{"page", "page-size", "only-active"}},
		{personasListCmd, "personas list", []string{"page", "page-size", "only-active"}},
		{profilesListCmd, "profiles list", []string{"page", "page-size", "name"}},
		{vaultsListCmd, "vaults list", []string{"page", "page-size", "only-active"}},
	}

	for _, tc := range paginatedCommands {
		for _, flag := range tc.requiredFlags {
			if tc.cmd.Flags().Lookup(flag) == nil {
				t.Errorf("%s: missing required flag --%s", tc.path, flag)
			}
		}
	}
}
