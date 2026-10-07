package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/auth"
	"github.com/nottelabs/notte-cli/internal/testutil"
	"github.com/nottelabs/notte-cli/internal/tunnel"
)

// fakeTailnet stands in for the tailscale CLI. Advertising flips the exit node
// on, and approve decides whether the tailnet approves it.
type fakeTailnet struct {
	approve     bool
	advertising bool
	calls       []string
}

func (f *fakeTailnet) run(_ context.Context, _ string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, strings.Join(args, " "))
	switch {
	case len(args) > 0 && args[0] == "status":
		return []byte(fmt.Sprintf(`{"BackendState":"Running","Self":{"DNSName":"lucas-mbp.tail1234.ts.net.","Online":true,"ExitNodeOption":%t},"CurrentTailnet":{"Name":"example.com"}}`,
			f.advertising && f.approve)), nil
	case len(args) == 2 && args[0] == "set":
		f.advertising = args[1] == "--advertise-exit-node=true"
		return nil, nil
	}
	return nil, fmt.Errorf("unexpected tailscale call: %v", args)
}

func setupTunnelTest(t *testing.T, approve bool) (*fakeTailnet, *testutil.TestEnv) {
	t.Helper()
	env := testutil.SetupTestEnv(t)
	setupSessionFileTest(t)
	auth.SetKeyring(env.MockStore)
	t.Cleanup(auth.ResetKeyring)

	fake := &fakeTailnet{approve: approve}
	orig := findTailscale
	findTailscale = func() (*tunnel.Tailscale, error) { return &tunnel.Tailscale{Bin: "tailscale", Run: fake.run}, nil }
	t.Cleanup(func() { findTailscale = orig })

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })
	return fake, env
}

func newTunnelUpCmd(args ...string) *cobra.Command {
	tunnelName, tunnelOAuthClientID, tunnelOAuthClientSecret = "", "", ""
	cmd := &cobra.Command{}
	cmd.Flags().StringVar(&tunnelName, "name", "", "")
	cmd.Flags().StringVar(&tunnelOAuthClientID, "oauth-client-id", "", "")
	cmd.Flags().StringVar(&tunnelOAuthClientSecret, "oauth-client-secret", "", "")
	cmd.SetContext(context.Background())
	_ = cmd.Flags().Parse(args)
	return cmd
}

func runCaptured(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	var err error
	stdout, _ := testutil.CaptureOutput(func() { err = fn() })
	return stdout, err
}

func TestTunnelUp_ApprovedSavesTunnel(t *testing.T) {
	fake, _ := setupTunnelTest(t, true)
	cmd := newTunnelUpCmd("--name", "home-mac", "--oauth-client-id", "client-1", "--oauth-client-secret", "s3cret")

	out, err := runCaptured(t, func() error { return runTunnelUp(cmd, nil) })
	if err != nil {
		t.Fatalf("runTunnelUp() error = %v", err)
	}
	if !fake.advertising {
		t.Error("exit node was not advertised")
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("output is not JSON: %q", out)
	}
	if result["approved"] != true || result["exit_node"] != "lucas-mbp.tail1234.ts.net" {
		t.Errorf("unexpected result: %v", result)
	}
	saved, err := tunnel.Get("home-mac")
	if err != nil || saved.OAuthClientID != "client-1" || saved.ExitNode != "lucas-mbp.tail1234.ts.net" {
		t.Errorf("saved tunnel = %+v, %v", saved, err)
	}
	if secret, _ := tunnel.Secret("home-mac"); secret != "s3cret" {
		t.Errorf("stored secret = %q", secret)
	}
}

func TestTunnelUp_PendingApprovalExplainsNextStep(t *testing.T) {
	setupTunnelTest(t, false)
	outputFormat = "text"
	cmd := newTunnelUpCmd("--oauth-client-id", "client-1", "--oauth-client-secret", "s3cret")

	out, err := runCaptured(t, func() error { return runTunnelUp(cmd, nil) })
	if err != nil {
		t.Fatalf("runTunnelUp() error = %v", err)
	}
	for _, want := range []string{"not approved as an exit node", "autogroup:internet", `"lucas-mbp"`} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	// The default name is the MagicDNS host label.
	if _, err := tunnel.Get("lucas-mbp"); err != nil {
		t.Errorf("tunnel not saved under default name: %v", err)
	}
}

func TestTunnelUp_RequiresCredentialsFirstTime(t *testing.T) {
	fake, _ := setupTunnelTest(t, true)

	if _, err := runCaptured(t, func() error { return runTunnelUp(newTunnelUpCmd("--name", "home-mac"), nil) }); err == nil || !strings.Contains(err.Error(), "--oauth-client-id is required") {
		t.Errorf("missing client id: got %v", err)
	}
	cmd := newTunnelUpCmd("--name", "home-mac", "--oauth-client-id", "client-1")
	if _, err := runCaptured(t, func() error { return runTunnelUp(cmd, nil) }); err == nil || !strings.Contains(err.Error(), "--oauth-client-secret") {
		t.Errorf("missing secret: got %v", err)
	}
	for _, c := range fake.calls {
		if strings.HasPrefix(c, "set") {
			t.Errorf("advertised an exit node despite missing credentials: %v", fake.calls)
		}
	}
}

func TestTunnelUp_ReusesSavedCredentialsAndReadsSecretFromEnv(t *testing.T) {
	_, env := setupTunnelTest(t, true)
	env.SetEnv(tailnetSecretEnv, "from-env")

	first := newTunnelUpCmd("--name", "home-mac", "--oauth-client-id", "client-1")
	if _, err := runCaptured(t, func() error { return runTunnelUp(first, nil) }); err != nil {
		t.Fatalf("first up: %v", err)
	}
	env.SetEnv(tailnetSecretEnv, "")
	if _, err := runCaptured(t, func() error { return runTunnelUp(newTunnelUpCmd("--name", "home-mac"), nil) }); err != nil {
		t.Fatalf("second up without credentials: %v", err)
	}
	if secret, _ := tunnel.Secret("home-mac"); secret != "from-env" {
		t.Errorf("stored secret = %q, want from-env", secret)
	}
}

func TestTunnelDown_StopsAdvertising(t *testing.T) {
	fake, _ := setupTunnelTest(t, true)
	fake.advertising = true
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	if _, err := runCaptured(t, func() error { return runTunnelDown(cmd, nil) }); err != nil {
		t.Fatalf("runTunnelDown() error = %v", err)
	}
	if fake.advertising {
		t.Error("exit node still advertised")
	}
}

func TestTunnelListAndRemove(t *testing.T) {
	setupTunnelTest(t, true)
	if err := tunnel.Save(tunnel.Tunnel{Name: "home-mac", ExitNode: "lucas-mbp.tail1234.ts.net", OAuthClientID: "c"}, "s"); err != nil {
		t.Fatal(err)
	}
	if err := tunnel.Save(tunnel.Tunnel{Name: "office", ExitNode: "office.tail1234.ts.net", OAuthClientID: "c"}, "s"); err != nil {
		t.Fatal(err)
	}
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	out, err := runCaptured(t, func() error { return runTunnelList(cmd, nil) })
	if err != nil {
		t.Fatalf("runTunnelList() error = %v", err)
	}
	var rows []tunnelRow
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("output is not JSON: %q", out)
	}
	// This machine is not advertising in the fake, so it reads as not approved;
	// the other machine's approval is unknown from here.
	if len(rows) != 2 || rows[0].Approved != "false" || rows[1].Approved != "-" {
		t.Errorf("rows = %+v", rows)
	}

	if _, err := runCaptured(t, func() error { return runTunnelRemove(cmd, []string{"office"}) }); err != nil {
		t.Fatalf("runTunnelRemove() error = %v", err)
	}
	if tunnels, _ := tunnel.List(); len(tunnels) != 1 {
		t.Errorf("tunnels after remove = %+v", tunnels)
	}
}

func TestSessionsStart_TunnelSendsTailnetProxyWithExitNode(t *testing.T) {
	_, env := setupTunnelTest(t, true)
	env.SetEnv("NOTTE_API_KEY", "test-key")
	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())
	server.AddResponse("/sessions/start", 200, `{"session_id":"sess_1","status":"ACTIVE","created_at":"2020-01-01T00:00:00Z","last_accessed_at":"2020-01-01T00:00:00Z","timeout_minutes":5}`)

	if err := tunnel.Save(tunnel.Tunnel{Name: "home-mac", ExitNode: "lucas-mbp.tail1234.ts.net", OAuthClientID: "client-1"}, "s3cret"); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	cmd.Flags().StringVar(&sessionsStartTunnel, "tunnel", "", "")
	cmd.SetContext(context.Background())
	_ = cmd.Flags().Parse([]string{"--tunnel", "home-mac"})

	if _, err := runCaptured(t, func() error { return runSessionsStart(cmd, nil) }); err != nil {
		t.Fatalf("runSessionsStart() error = %v", err)
	}
	reqs := server.Requests("/sessions/start")
	if len(reqs) != 1 {
		t.Fatalf("got %d start requests", len(reqs))
	}
	var body struct {
		Proxies []map[string]string `json:"proxies"`
	}
	if err := json.Unmarshal([]byte(reqs[0].Body), &body); err != nil {
		t.Fatalf("bad request body %q: %v", reqs[0].Body, err)
	}
	want := map[string]string{"type": "tailnet", "oauth_client_id": "client-1", "oauth_client_secret": "s3cret", "exit_node": "lucas-mbp.tail1234.ts.net"}
	if len(body.Proxies) != 1 || fmt.Sprint(body.Proxies[0]) != fmt.Sprint(want) {
		t.Errorf("proxies = %v, want [%v]", body.Proxies, want)
	}
}

func TestSessionsStart_TunnelErrors(t *testing.T) {
	setupTunnelTest(t, true)
	if _, err := tunnelProxyItem("missing"); err == nil || !strings.Contains(err.Error(), "notte tunnel list") {
		t.Errorf("unknown tunnel: got %v", err)
	}

	cmd := &cobra.Command{}
	cmd.Flags().Bool("proxy", false, "")
	cmd.Flags().String("tunnel", "", "")
	_ = cmd.Flags().Parse([]string{"--proxy", "--tunnel=home-mac"})
	if err := validateSessionStartProxyFlags(cmd); err == nil || !strings.Contains(err.Error(), "--tunnel") {
		t.Errorf("--proxy with --tunnel: got %v", err)
	}
}

func TestSessionsStart_UnknownTunnelDoesNotStopCurrentSession(t *testing.T) {
	_, env := setupTunnelTest(t, true)
	env.SetEnv("NOTTE_API_KEY", "test-key")
	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	if err := setCurrentSession("sess_existing"); err != nil {
		t.Fatal(err)
	}
	origSkip := skipConfirmation
	skipConfirmation = true
	t.Cleanup(func() { skipConfirmation = origSkip })

	cmd := &cobra.Command{}
	cmd.Flags().StringVar(&sessionsStartTunnel, "tunnel", "", "")
	cmd.SetContext(context.Background())
	_ = cmd.Flags().Parse([]string{"--tunnel", "typo"})

	if _, err := runCaptured(t, func() error { return runSessionsStart(cmd, nil) }); err == nil {
		t.Fatal("expected an error for an unknown tunnel")
	}
	for path := range server.AllRequests() {
		t.Errorf("unexpected API call to %s before the tunnel was resolved", path)
	}
	if GetCurrentSessionID() != "sess_existing" {
		t.Error("current session was cleared")
	}
}
