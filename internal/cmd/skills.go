package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Manage Notte skills for AI coding assistants",
	Long: `Install and manage Notte skills for AI coding assistants.

Skills provide AI assistants with browser automation capabilities
through natural language commands.`,
}

var skillAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Install the Notte skill for your AI coding assistant",
	Long: `Install the Notte browser automation skill using npx.

This command runs: npx skills add nottelabs/notte-cli

The skill enables AI coding assistants (like Cursor, Claude Code, etc.)
to control browser sessions through natural language commands.`,
	RunE: runSkillAdd,
}

var skillRemoveCmd = &cobra.Command{
	Use:     "remove",
	Aliases: []string{"rm"},
	Short:   "Remove the Notte skill from your AI coding assistant",
	Long: `Remove the Notte browser automation skill using npx.

This command runs: npx skills remove nottelabs/notte-cli`,
	RunE: runSkillRemove,
}

func init() {
	rootCmd.AddCommand(skillCmd)
	skillCmd.AddCommand(skillAddCmd)
	skillCmd.AddCommand(skillRemoveCmd)
}

// checkSkillsInstalled verifies that npx and the skills package are available
func checkSkillsInstalled() error {
	// Check if npx is available
	if _, err := exec.LookPath("npx"); err != nil {
		return errors.New("npx not found. Please install Node.js from https://nodejs.org")
	}

	// Check if skills package is available by running 'npx skills --version'
	cmd := exec.Command("npx", "skills", "--version")
	if err := cmd.Run(); err != nil {
		return errors.New("'skills' package not found. Install it with: npm install -g skills")
	}

	return nil
}

func runSkillAdd(cmd *cobra.Command, args []string) error {
	// Check prerequisites
	if err := checkSkillsInstalled(); err != nil {
		return err
	}

	PrintInfo("Installing Notte skill via npx...")

	// Create the npx command
	npxCmd := exec.CommandContext(cmd.Context(), "npx", "skills", "add", "nottelabs/notte-cli")

	// Connect stdout and stderr to show output in real-time
	npxCmd.Stdout = os.Stdout
	npxCmd.Stderr = os.Stderr
	npxCmd.Stdin = os.Stdin

	// Run the command
	if err := npxCmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("skill installation failed with exit code %d", exitErr.ExitCode())
		}
		return fmt.Errorf("failed to run npx: %w", err)
	}

	return nil
}

func runSkillRemove(cmd *cobra.Command, args []string) error {
	// Check prerequisites
	if err := checkSkillsInstalled(); err != nil {
		return err
	}

	PrintInfo("Removing Notte skill via npx...")

	// Create the npx command
	npxCmd := exec.CommandContext(cmd.Context(), "npx", "skills", "remove", "nottelabs/notte-cli")

	// Connect stdout and stderr to show output in real-time
	npxCmd.Stdout = os.Stdout
	npxCmd.Stderr = os.Stderr
	npxCmd.Stdin = os.Stdin

	// Run the command
	if err := npxCmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("skill removal failed with exit code %d", exitErr.ExitCode())
		}
		return fmt.Errorf("failed to run npx: %w", err)
	}

	return nil
}
