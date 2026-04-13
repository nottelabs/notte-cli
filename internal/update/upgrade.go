package update

import (
	"fmt"
	"io"
	"os"
	"os/exec"
)

// UpgradeMethod represents how the CLI was installed.
type UpgradeMethod int

const (
	// UpgradeHomebrew indicates the CLI was installed via Homebrew.
	UpgradeHomebrew UpgradeMethod = iota
	// UpgradeManual indicates the install method could not be detected.
	UpgradeManual
)

// DetectInstallMethod checks how the CLI was installed.
func DetectInstallMethod() UpgradeMethod {
	brewPath, err := exec.LookPath("brew")
	if err != nil || brewPath == "" {
		return UpgradeManual
	}

	// Check if notte was installed via Homebrew
	cmd := exec.Command("brew", "list", "notte")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return UpgradeManual
	}

	return UpgradeHomebrew
}

// RunUpgrade performs the upgrade using the detected method.
func RunUpgrade(out io.Writer, method UpgradeMethod) error {
	switch method {
	case UpgradeHomebrew:
		return runHomebrewUpgrade(out)
	default:
		return printManualUpgrade(out)
	}
}

func runHomebrewUpgrade(out io.Writer) error {
	cmd := exec.Command("brew", "upgrade", "notte")
	cmd.Stdout = out
	cmd.Stderr = os.Stderr
	return cmd.Run()
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
