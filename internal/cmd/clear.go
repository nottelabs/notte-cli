package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/config"
)

// legacyCurrentAgentFile is retained only so `notte clear` can clean up state
// written by CLI versions that supported agents.
const legacyCurrentAgentFile = "current_agent"

var clearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear all stored state",
	Long:  "Clear all locally stored state including current session, viewer URL, and function. This does not affect credentials or settings.",
	RunE:  runClear,
}

func init() {
	rootCmd.AddCommand(clearCmd)
}

func runClear(cmd *cobra.Command, args []string) error {
	if err := clearCurrentSession(); err != nil {
		return fmt.Errorf("failed to clear current session: %w", err)
	}
	if err := clearCurrentViewerURL(); err != nil {
		return fmt.Errorf("failed to clear current viewer URL: %w", err)
	}
	if err := clearLegacyCurrentAgent(); err != nil {
		return fmt.Errorf("failed to clear legacy current agent: %w", err)
	}
	if err := clearCurrentFunction(); err != nil {
		return fmt.Errorf("failed to clear current function: %w", err)
	}
	if err := clearCurrentSessionExpiry(); err != nil {
		return fmt.Errorf("failed to clear current session expiry: %w", err)
	}

	return PrintResult("Cleared all stored state (session, viewer URL, function, session expiry).", map[string]any{
		"cleared": []string{"session", "viewer_url", "function", "session_expiry"},
		"success": true,
	})
}

func clearLegacyCurrentAgent() error {
	configDir, err := config.Dir()
	if err != nil {
		return err
	}
	path := filepath.Join(configDir, legacyCurrentAgentFile)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
