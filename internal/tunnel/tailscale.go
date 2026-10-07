// Package tunnel turns this machine into a Tailscale exit node that Notte
// sessions can route their public traffic through.
//
// It drives the user's own Tailscale install through the tailscale CLI rather
// than embedding Tailscale, which would roughly double the size of the notte
// binary for a feature most users never touch.
package tunnel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// macAppCLI is the tailscale CLI bundled inside the macOS app. It is tried
// before PATH because the Homebrew `tailscale` formula installs a CLI that
// only works once its own daemon is running, while most Mac users run the app.
const macAppCLI = "/Applications/Tailscale.app/Contents/MacOS/Tailscale"

// InstallHint tells users how to get a working Tailscale.
const InstallHint = "Install Tailscale and sign in to your tailnet: `brew install --cask tailscale` on macOS, or https://tailscale.com/download"

// ErrTailscaleNotFound means no tailscale CLI could be found.
var ErrTailscaleNotFound = errors.New("tailscale CLI not found")

// ErrTailscaleNotRunning means the CLI was found but Tailscale is not connected.
var ErrTailscaleNotRunning = errors.New("tailscale is not running")

// Runner executes the tailscale CLI and returns its stdout.
type Runner func(ctx context.Context, bin string, args ...string) ([]byte, error)

func execRunner(ctx context.Context, bin string, args ...string) ([]byte, error) {
	var stdout, stderr bytes.Buffer
	c := exec.CommandContext(ctx, bin, args...)
	c.Stdout = &stdout
	c.Stderr = &stderr
	if err := c.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return stdout.Bytes(), fmt.Errorf("%w: %s", err, msg)
		}
		return stdout.Bytes(), err
	}
	return stdout.Bytes(), nil
}

// Tailscale is a handle on the user's tailscale CLI.
type Tailscale struct {
	Bin string
	Run Runner
}

// Find locates the tailscale CLI: the macOS app's bundled CLI first, then PATH.
func Find() (*Tailscale, error) {
	return find(runtime.GOOS, fileExists, exec.LookPath)
}

func find(goos string, exists func(string) bool, lookPath func(string) (string, error)) (*Tailscale, error) {
	if goos == "darwin" && exists(macAppCLI) {
		return &Tailscale{Bin: macAppCLI, Run: execRunner}, nil
	}
	if bin, err := lookPath("tailscale"); err == nil {
		return &Tailscale{Bin: bin, Run: execRunner}, nil
	}
	return nil, fmt.Errorf("%w. %s", ErrTailscaleNotFound, InstallHint)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// Status is the subset of `tailscale status --json` the tunnel needs.
type Status struct {
	BackendState string `json:"BackendState"`
	Self         struct {
		HostName string `json:"HostName"`
		DNSName  string `json:"DNSName"`
		Online   bool   `json:"Online"`
		// ExitNodeOption is true once this node offers exit routes and the
		// tailnet has approved them.
		ExitNodeOption bool `json:"ExitNodeOption"`
	} `json:"Self"`
	CurrentTailnet *struct {
		Name string `json:"Name"`
	} `json:"CurrentTailnet"`
}

// MagicDNSName is this machine's MagicDNS name without the trailing dot.
func (s *Status) MagicDNSName() string {
	return strings.TrimSuffix(s.Self.DNSName, ".")
}

// Tailnet is the name of the tailnet this machine is signed in to.
func (s *Status) Tailnet() string {
	if s.CurrentTailnet == nil {
		return ""
	}
	return s.CurrentTailnet.Name
}

// Status reads the current state and fails unless Tailscale is connected.
func (t *Tailscale) Status(ctx context.Context) (*Status, error) {
	out, err := t.Run(ctx, t.Bin, "status", "--json")
	// `tailscale status --json` exits non-zero when it is not running but may
	// still print a usable state, so parse whenever there is output.
	var st Status
	if len(bytes.TrimSpace(out)) > 0 {
		if jsonErr := json.Unmarshal(out, &st); jsonErr != nil && err == nil {
			return nil, fmt.Errorf("could not parse `tailscale status --json`: %w", jsonErr)
		}
	}
	if st.BackendState == "" && err != nil {
		return nil, fmt.Errorf("%w (%v). Start Tailscale and sign in, then try again. %s", ErrTailscaleNotRunning, err, InstallHint)
	}
	if st.BackendState != "Running" {
		return nil, fmt.Errorf("%w (state: %s). Start Tailscale and sign in, then try again", ErrTailscaleNotRunning, st.BackendState)
	}
	if st.MagicDNSName() == "" {
		return nil, errors.New("tailscale reported no MagicDNS name for this machine; enable MagicDNS for your tailnet")
	}
	return &st, nil
}

// SetAdvertiseExitNode starts or stops offering this machine as an exit node.
func (t *Tailscale) SetAdvertiseExitNode(ctx context.Context, advertise bool) error {
	if _, err := t.Run(ctx, t.Bin, "set", fmt.Sprintf("--advertise-exit-node=%t", advertise)); err != nil {
		return fmt.Errorf("tailscale set --advertise-exit-node=%t failed: %w", advertise, err)
	}
	return nil
}
