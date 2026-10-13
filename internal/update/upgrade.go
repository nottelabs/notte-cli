package update

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// UpgradeMethod represents how the CLI was installed.
type UpgradeMethod int

const (
	// UpgradeHomebrew indicates the CLI was installed via Homebrew.
	UpgradeHomebrew UpgradeMethod = iota
	// UpgradeScript indicates the CLI is upgraded by re-running the install
	// script: a release binary or a go install build on macOS or Linux.
	UpgradeScript
	// UpgradeManual indicates the CLI cannot upgrade itself on this platform.
	UpgradeManual
)

// InstallScriptURL is the install script the CLI re-runs to upgrade itself.
const InstallScriptURL = "https://notte.cc/install-cli.sh"

// installScriptURL can be overridden in tests.
var installScriptURL = InstallScriptURL

// DetectInstallMethod checks how the CLI was installed.
func DetectInstallMethod() UpgradeMethod {
	if installedWithHomebrew() {
		return UpgradeHomebrew
	}
	return methodForOS(runtime.GOOS)
}

func installedWithHomebrew() bool {
	brewPath, err := exec.LookPath("brew")
	if err != nil || brewPath == "" {
		return false
	}

	// Check if notte was installed via Homebrew
	cmd := exec.Command("brew", "list", "notte")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run() == nil
}

// methodForOS picks the upgrade method for a CLI not installed with Homebrew.
// The install script only supports macOS and Linux.
func methodForOS(goos string) UpgradeMethod {
	switch goos {
	case "darwin", "linux":
		return UpgradeScript
	default:
		return UpgradeManual
	}
}

// RunUpgrade performs the upgrade using the detected method.
func RunUpgrade(out io.Writer, method UpgradeMethod) error {
	switch method {
	case UpgradeHomebrew:
		return runHomebrewUpgrade(out)
	case UpgradeScript:
		return runScriptUpgrade(out)
	default:
		return printManualUpgrade(out)
	}
}

func runHomebrewUpgrade(out io.Writer) error {
	cmd := exec.Command("brew", "upgrade", "notte")
	cmd.Stdout = out
	cmd.Stderr = out
	return cmd.Run()
}

// runScriptUpgrade downloads the install script and runs it, installing the
// latest release over the running binary.
func runScriptUpgrade(out io.Writer) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	script, err := fetchInstallScript(ctx, &http.Client{})
	if err != nil {
		return err
	}

	// The script runs from -c rather than from stdin, so stdin stays free for
	// the sudo password prompt.
	cmd := exec.Command("sh", "-c", script)
	cmd.Stdin = os.Stdin
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.Env = os.Environ()
	if dir := installDir(); dir != "" {
		cmd.Env = append(cmd.Env, "NOTTE_INSTALL_DIR="+dir)
	}
	return cmd.Run()
}

// fetchInstallScript downloads the whole script before anything runs, so a
// failed or truncated download never executes.
func fetchInstallScript(ctx context.Context, httpClient *http.Client) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, installScriptURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("downloading the install script: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("downloading the install script: %s", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("downloading the install script: %w", err)
	}
	return string(body), nil
}

// installDir returns the directory of the running binary, so the upgrade
// replaces it instead of installing a second copy elsewhere. It returns "" for
// the script's own default, /usr/local/bin, so the script keeps its sudo
// fallback there.
func installDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return dirOverride(filepath.Dir(exe))
}

func dirOverride(dir string) string {
	if dir == "/usr/local/bin" {
		return ""
	}
	return dir
}

func printManualUpgrade(out io.Writer) error {
	_, err := fmt.Fprintln(out, "To upgrade manually, run one of:")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, "  brew install nottelabs/notte-cli/notte")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, "  go install github.com/nottelabs/notte-cli/cmd/notte@latest")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, "Or download from: https://github.com/nottelabs/notte-cli/releases/latest")
	return err
}
