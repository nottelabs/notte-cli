package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

// Skill source / skill name. The skill files live in the nottelabs/notte-skills
// repository (vendored here as the notte-skills submodule). Pointing the
// npx tool at nottelabs/notte-cli would clone this repo, where the skills
// are an empty submodule and no SKILL.md is found.
const (
	skillSource = "nottelabs/notte-skills"
	skillName   = "notte-browser"
)

var skillAddUpgrade bool

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

This command runs: npx skills add nottelabs/notte-skills

With --upgrade (or -f), it runs: npx skills update notte-browser
to refresh an already-installed skill to the latest version.

npx skills prompts interactively to pick which AI assistants to install
to. Pass --yes (-y) to skip that prompt and install to all detected
assistants — required when running non-interactively (CI, scripts).

The skill enables AI coding assistants (like Cursor, Claude Code, etc.)
to control browser sessions through natural language commands.`,
	RunE: runSkillAdd,
}

var skillRemoveCmd = &cobra.Command{
	Use:     "remove",
	Aliases: []string{"rm"},
	Short:   "Remove the Notte skill from your AI coding assistant",
	Long: `Remove the Notte browser automation skill using npx.

This command runs: npx skills remove --skill notte-browser`,
	RunE: runSkillRemove,
}

func init() {
	rootCmd.AddCommand(skillCmd)
	skillCmd.AddCommand(skillAddCmd)
	skillCmd.AddCommand(skillRemoveCmd)

	skillAddCmd.Flags().BoolVarP(&skillAddUpgrade, "upgrade", "f", false,
		"Force a reinstall by updating an already-installed skill to the latest version")
}

func runSkillAdd(cmd *cobra.Command, args []string) error {
	var npxArgs []string
	if skillAddUpgrade {
		PrintInfo("Upgrading Notte skill via npx...")
		npxArgs = []string{"skills", "update", skillName}
	} else {
		PrintInfo("Installing Notte skill via npx...")
		npxArgs = []string{"skills", "add", skillSource}
	}

	return runNpx(cmd, "skill installation", npxArgs)
}

func runSkillRemove(cmd *cobra.Command, args []string) error {
	PrintInfo("Removing Notte skill via npx...")
	return runNpx(cmd, "skill removal", []string{"skills", "remove", "--skill", skillName})
}

// runNpx executes `npx <args>` wired to the current stdio.
//
// `npx skills` prompts interactively (an agent picker for `add`, a scope
// prompt for `update`/`remove`). With no terminal to answer them those
// prompts silently abort and nothing is installed — which is how a previous
// CI run "succeeded" while installing nothing. Forward the global --yes flag
// as `-y` so non-interactive callers can opt into skipping the prompts.
func runNpx(cmd *cobra.Command, action string, args []string) error {
	if yesFlag {
		args = append(args, "-y")
	}

	npxCmd := exec.CommandContext(cmd.Context(), "npx", args...)

	npxCmd.Stdout = os.Stdout
	npxCmd.Stderr = os.Stderr
	npxCmd.Stdin = os.Stdin

	if err := npxCmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("%s failed with exit code %d", action, exitErr.ExitCode())
		}
		return fmt.Errorf("failed to run npx: %w", err)
	}

	return nil
}
