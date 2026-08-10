package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/config"
)

// Legacy current-resource files are retained only so `notte clear` can clean
// up state written by older CLI versions.
var legacyCurrentResourceFiles = []string{
	"current_session",
	"current_viewer_url",
	"current_agent",
	"current_function",
	"current_session_expiry",
}

var clearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear legacy stored resource pointers",
	Long:  "Clear legacy locally stored resource pointers. This does not affect remote resources, credentials, or settings.",
	RunE:  runClear,
}

func init() {
	rootCmd.AddCommand(clearCmd)
}

func runClear(cmd *cobra.Command, args []string) error {
	for _, name := range legacyCurrentResourceFiles {
		if err := clearLegacyCurrentResource(name); err != nil {
			return fmt.Errorf("failed to clear legacy resource pointer %s: %w", name, err)
		}
	}

	return PrintResult("Cleared legacy stored resource pointers.", map[string]any{
		"cleared": legacyCurrentResourceFiles,
		"success": true,
	})
}

func clearLegacyCurrentResource(name string) error {
	configDir, err := config.Dir()
	if err != nil {
		return err
	}
	path := filepath.Join(configDir, name)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
