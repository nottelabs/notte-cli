package update

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/muesli/termenv"
	"golang.org/x/term"

	"github.com/nottelabs/notte-cli/internal/config"
)

// Result holds the outcome of an update check.
type Result struct {
	CurrentVersion  string
	LatestVersion   string
	ReleaseURL      string
	UpdateAvailable bool
}

// Checker manages the async update check lifecycle.
type Checker struct {
	currentVersion string
	configDir      string
	result         *Result
	done           chan struct{}
}

// NewChecker creates a Checker. Returns nil if version is "dev" or
// NOTTE_NO_UPDATE_CHECK is set.
func NewChecker(currentVersion string) *Checker {
	if currentVersion == "dev" {
		return nil
	}
	if os.Getenv(config.EnvNoUpdateCheck) != "" {
		return nil
	}

	configDir, err := config.Dir()
	if err != nil {
		return nil
	}

	return &Checker{
		currentVersion: currentVersion,
		configDir:      configDir,
		done:           make(chan struct{}),
	}
}

// Run performs the update check. It reads the cache first; if the cache is
// fresh, it uses cached data. Otherwise it queries GitHub and updates the cache.
// This method is designed to be called in a goroutine.
func (c *Checker) Run(ctx context.Context) {
	defer close(c.done)

	// Try loading cached result first
	cache, _ := LoadCache(c.configDir)
	if cache != nil && !cache.IsStale(c.currentVersion) {
		newer, err := IsNewer(c.currentVersion, cache.LatestVersion)
		if err == nil && newer {
			c.result = &Result{
				CurrentVersion:  c.currentVersion,
				LatestVersion:   cache.LatestVersion,
				ReleaseURL:      cache.ReleaseURL,
				UpdateAvailable: true,
			}
		}
		return
	}

	// Query GitHub for the latest release
	httpClient := &http.Client{Timeout: 5 * time.Second}
	release, err := CheckLatestVersion(ctx, httpClient)
	if err != nil || release == nil {
		return // silent failure
	}

	// Save to cache regardless of whether an update is available
	newCache := &UpdateCache{
		LatestVersion:  release.TagName,
		CurrentVersion: c.currentVersion,
		CheckedAt:      time.Now(),
		ReleaseURL:     release.HTMLURL,
	}
	_ = SaveCache(c.configDir, newCache) // best-effort

	// Check if the latest version is newer
	newer, err := IsNewer(c.currentVersion, release.TagName)
	if err != nil || !newer {
		return
	}

	c.result = &Result{
		CurrentVersion:  c.currentVersion,
		LatestVersion:   release.TagName,
		ReleaseURL:      release.HTMLURL,
		UpdateAvailable: true,
	}
}

// GetResult blocks until the check completes and returns the result.
// Returns nil if no update is available or the check failed.
func (c *Checker) GetResult() *Result {
	<-c.done
	return c.result
}

// PrintUpdateNotification displays the update message and optionally
// prompts to upgrade. It writes to stderr to avoid polluting stdout.
func PrintUpdateNotification(result *Result, out io.Writer, in io.Reader, skipConfirm bool, jsonMode bool, noColor bool) {
	if jsonMode {
		return
	}
	if result == nil || !result.UpdateAvailable {
		return
	}

	termOut := termenv.NewOutput(os.Stderr)
	colorize := func(s string, color termenv.ANSIColor) string {
		if noColor {
			return s
		}
		return termOut.String(s).Foreground(color).String()
	}

	// Print a blank line to separate from command output
	_, _ = fmt.Fprintln(out)

	// Update available message (yellow)
	current := formatVersion(result.CurrentVersion)
	latest := formatVersion(result.LatestVersion)
	_, _ = fmt.Fprintln(out, colorize(
		fmt.Sprintf("Update available for Notte CLI (%s \u2192 %s) The latest update may fix any errors that occurred.", current, latest),
		termenv.ANSIYellow,
	))

	// Changelog link (cyan)
	changelogURL := result.ReleaseURL
	if changelogURL == "" {
		changelogURL = fmt.Sprintf("https://github.com/nottelabs/notte-cli/releases/tag/%s", latest)
	}
	_, _ = fmt.Fprintln(out, colorize(
		fmt.Sprintf("Changelog: %s", changelogURL),
		termenv.ANSICyan,
	))

	// Check if stdin is a terminal for interactive prompt
	isTerminal := false
	if f, ok := in.(*os.File); ok {
		isTerminal = term.IsTerminal(int(f.Fd()))
	}

	if !isTerminal && !skipConfirm {
		// Non-interactive: just show the notification, no prompt
		return
	}

	// Prompt for upgrade
	if skipConfirm {
		_, _ = fmt.Fprintln(out, "? Would you like to upgrade now? yes")
	} else {
		_, _ = fmt.Fprint(out, "? Would you like to upgrade now? [Y/n]: ")
		reader := bufio.NewReader(in)
		response, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		response = strings.TrimSpace(strings.ToLower(response))
		if response == "n" || response == "no" {
			return
		}
	}

	// Perform upgrade
	method := DetectInstallMethod()
	_, _ = fmt.Fprintln(out, colorize("> Upgrading Notte CLI...", termenv.ANSIGreen))

	if err := RunUpgrade(out, method); err != nil {
		_, _ = fmt.Fprintln(out, colorize(fmt.Sprintf("> Upgrade failed: %s", err), termenv.ANSIRed))
		return
	}

	if method == UpgradeHomebrew {
		_, _ = fmt.Fprintln(out, colorize("> Success! Notte CLI has been upgraded successfully!", termenv.ANSIGreen))
	}
}
