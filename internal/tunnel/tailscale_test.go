package tunnel

import (
	"context"
	"errors"
	"strings"
	"testing"
)

const runningStatus = `{
  "BackendState": "Running",
  "Self": {"HostName": "Lucas's MacBook Pro", "DNSName": "lucas-mbp.tail1234.ts.net.", "Online": true, "ExitNodeOption": true},
  "CurrentTailnet": {"Name": "example.com"}
}`

func fakeTailscale(out string, err error, calls *[][]string) *Tailscale {
	return &Tailscale{Bin: "tailscale", Run: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if calls != nil {
			*calls = append(*calls, args)
		}
		return []byte(out), err
	}}
}

func TestFind_PrefersMacAppThenPath(t *testing.T) {
	exists := func(p string) bool { return p == macAppCLI }
	lookPath := func(string) (string, error) { return "/opt/homebrew/bin/tailscale", nil }
	noPath := func(string) (string, error) { return "", errors.New("not found") }

	if ts, err := find("darwin", exists, lookPath); err != nil || ts.Bin != macAppCLI {
		t.Errorf("darwin with app: got %v, %v; want app CLI", ts, err)
	}
	if ts, err := find("linux", exists, lookPath); err != nil || ts.Bin != "/opt/homebrew/bin/tailscale" {
		t.Errorf("linux: got %v, %v; want PATH CLI", ts, err)
	}
	if _, err := find("darwin", func(string) bool { return false }, noPath); !errors.Is(err, ErrTailscaleNotFound) {
		t.Errorf("nothing installed: got %v, want ErrTailscaleNotFound", err)
	} else if !strings.Contains(err.Error(), "brew install --cask tailscale") {
		t.Errorf("missing install hint: %v", err)
	}
}

func TestStatus_ParsesRunningNode(t *testing.T) {
	st, err := fakeTailscale(runningStatus, nil, nil).Status(context.Background())
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if st.MagicDNSName() != "lucas-mbp.tail1234.ts.net" || st.Tailnet() != "example.com" || !st.Self.ExitNodeOption {
		t.Errorf("unexpected status: %+v", st)
	}
}

func TestStatus_FailsUnlessRunning(t *testing.T) {
	cases := map[string]*Tailscale{
		"daemon unreachable": fakeTailscale("", errors.New("failed to connect to local tailscaled"), nil),
		"signed out":         fakeTailscale(`{"BackendState": "NeedsLogin", "Self": {}}`, errors.New("exit status 1"), nil),
		"stopped":            fakeTailscale(`{"BackendState": "Stopped", "Self": {}}`, nil, nil),
	}
	for name, ts := range cases {
		if _, err := ts.Status(context.Background()); !errors.Is(err, ErrTailscaleNotRunning) {
			t.Errorf("%s: got %v, want ErrTailscaleNotRunning", name, err)
		}
	}
}

func TestSetAdvertiseExitNode_PassesFlag(t *testing.T) {
	var calls [][]string
	ts := fakeTailscale("", nil, &calls)
	if err := ts.SetAdvertiseExitNode(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if err := ts.SetAdvertiseExitNode(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	want := []string{"set --advertise-exit-node=true", "set --advertise-exit-node=false"}
	for i, c := range calls {
		if got := strings.Join(c, " "); got != want[i] {
			t.Errorf("call %d = %q, want %q", i, got, want[i])
		}
	}
}
