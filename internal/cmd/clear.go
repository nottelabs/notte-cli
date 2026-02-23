package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var clearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear all stored state",
	Long:  "Clear all locally stored state including current session, viewer URL, agent, and function. This does not affect credentials or settings.",
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
	if err := clearCurrentAgent(); err != nil {
		return fmt.Errorf("failed to clear current agent: %w", err)
	}
	if err := clearCurrentFunction(); err != nil {
		return fmt.Errorf("failed to clear current function: %w", err)
	}
	if err := clearCurrentSessionExpiry(); err != nil {
		return fmt.Errorf("failed to clear current session expiry: %w", err)
	}

	return PrintResult("Cleared all stored state (session, viewer URL, agent, function, session expiry).", map[string]any{
		"cleared": []string{"session", "viewer_url", "agent", "function", "session_expiry"},
		"success": true,
	})
}
