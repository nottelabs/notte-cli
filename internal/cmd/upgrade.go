package cmd

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/update"
)

// upgradeRan is set by `notte upgrade`, so the update notice does not offer
// the same upgrade again once the command finishes.
var upgradeRan bool

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade the CLI to the latest release",
	Long: `Upgrade the CLI to the latest release.

Uses Homebrew when the CLI was installed with Homebrew. Otherwise it re-runs
the install script (` + update.InstallScriptURL + `), replacing the
binary that is running. On Windows it prints the manual steps instead.`,
	Args: cobra.NoArgs,
	RunE: runUpgrade,
}

func init() {
	rootCmd.AddCommand(upgradeCmd)
}

func runUpgrade(cmd *cobra.Command, args []string) error {
	upgradeRan = true

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	// An unknown latest version, or a build without a release version such as
	// go install, still upgrades: only a confirmed up-to-date CLI stops here.
	latest := ""
	if release, _ := update.CheckLatestVersion(ctx, &http.Client{Timeout: 10 * time.Second}); release != nil {
		latest = strings.TrimPrefix(release.TagName, "v")
		if newer, err := update.IsNewer(Version, latest); err == nil && !newer {
			return PrintResult(fmt.Sprintf("notte %s is already the latest version", Version), map[string]any{
				"version":  Version,
				"upgraded": false,
			})
		}
	}

	method := update.DetectInstallMethod()
	if method == update.UpgradeManual {
		if !IsJSONOutput() {
			return update.RunUpgrade(os.Stdout, method)
		}
		if err := update.RunUpgrade(os.Stderr, method); err != nil {
			return err
		}
		return PrintResult("", map[string]any{
			"previous_version": Version,
			"version":          latest,
			"upgraded":         false,
		})
	}

	// Installer output goes to stderr so stdout carries only the result.
	if err := update.RunUpgrade(os.Stderr, method); err != nil {
		return fmt.Errorf("upgrade failed: %w", err)
	}
	if method == update.UpgradeScript && latest != "" {
		if err := update.VerifyUpgraded(latest); err != nil {
			return fmt.Errorf("upgrade failed: %w", err)
		}
	}

	message := "Upgraded notte"
	if latest != "" {
		message = fmt.Sprintf("Upgraded notte from %s to %s", Version, latest)
	}
	return PrintResult(message, map[string]any{
		"previous_version": Version,
		"version":          latest,
		"upgraded":         true,
	})
}
