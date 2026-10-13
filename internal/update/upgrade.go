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
	"strings"
	"time"

	"github.com/nottelabs/notte-cli/internal/config"
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
	exe, err := runningBinary()
	if err != nil {
		return fmt.Errorf("locating the running binary: %w", err)
	}
	// The script always writes a file named notte, so a binary under another
	// name (e.g. a link to notte-v0.0.54) would not be replaced.
	if filepath.Base(exe) != "notte" {
		return fmt.Errorf("%s is not named notte, so the install script cannot replace it; download the latest release from https://github.com/nottelabs/notte-cli/releases/latest", exe)
	}
	return installLatest(&http.Client{}, out, dirOverride(filepath.Dir(exe)))
}

// installLatest fetches the install script and runs it, installing into dir,
// or into the script's default directory when dir is empty.
func installLatest(httpClient *http.Client, out io.Writer, dir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	script, err := fetchInstallScript(ctx, httpClient)
	if err != nil {
		return err
	}

	// The script runs from -c rather than from stdin, so stdin stays free for
	// the sudo password prompt.
	cmd := exec.Command("sh", "-c", script)
	cmd.Stdin = os.Stdin
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.Env = installerEnv(os.Environ(), dir)
	return cmd.Run()
}

// installerEnv drops NOTTE_VERSION and NOTTE_INSTALL_DIR left over from an
// earlier install, so the upgrade installs the latest release and only sets
// the directory the upgrade chose.
func installerEnv(environ []string, dir string) []string {
	env := make([]string, 0, len(environ)+1)
	for _, kv := range environ {
		if strings.HasPrefix(kv, "NOTTE_VERSION=") || strings.HasPrefix(kv, "NOTTE_INSTALL_DIR=") {
			continue
		}
		env = append(env, kv)
	}
	if dir != "" {
		env = append(env, "NOTTE_INSTALL_DIR="+dir)
	}
	return env
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

// runningBinary returns the path of the running binary with links resolved.
func runningBinary() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return exe, nil
}

// dirOverride returns the directory to pass to the install script, or "" for
// the script's own default, /usr/local/bin, so the script keeps its sudo
// fallback there.
func dirOverride(dir string) string {
	if dir == "/usr/local/bin" {
		return ""
	}
	return dir
}

// VerifyUpgraded checks that the running binary's path now holds version or a
// newer release, since the script installs whatever is latest when it runs.
// The install script can succeed while installing somewhere else, e.g. into
// ~/.local/bin when sudo is denied for /usr/local/bin.
func VerifyUpgraded(version string) error {
	exe, err := runningBinary()
	if err != nil {
		return fmt.Errorf("locating the running binary: %w", err)
	}
	return verifyVersion(exe, version)
}

func verifyVersion(exe, version string) error {
	cmd := exec.Command(exe, "version")
	cmd.Env = append(os.Environ(), config.EnvNoUpdateCheck+"=1")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("checking the version of %s: %w", exe, err)
	}
	fields := strings.Fields(string(output))
	if len(fields) > 0 {
		if cmp, err := CompareVersions(fields[len(fields)-1], version); err == nil && cmp >= 0 {
			return nil
		}
	}
	return fmt.Errorf("%s is still older than %s; the install script may have installed it somewhere else (see its output above)", exe, strings.TrimPrefix(version, "v"))
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
