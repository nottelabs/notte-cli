package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/testutil"
	"github.com/nottelabs/notte-cli/internal/tunnel"
)

const checkSessionBody = `{"session_id":"sess_1","status":"ACTIVE","created_at":"2020-01-01T00:00:00Z","last_accessed_at":"2020-01-01T00:00:00Z","timeout_minutes":5}`

// setupTunnelCheck saves a workspace tunnel for this machine, approves it as an
// exit node, and points the CLI at a mock API.
func setupTunnelCheck(t *testing.T, machineIP string) (*fakeTailnet, *testutil.MockServer) {
	t.Helper()
	fake, env := setupTunnelTest(t, true)
	fake.advertising = true
	workspaceTailscaleConnected = func(context.Context) (bool, error) { return true, nil }

	origLookup := lookupPublicIP
	lookupPublicIP = func(context.Context) (string, error) { return machineIP, nil }
	t.Cleanup(func() { lookupPublicIP = origLookup })

	env.SetEnv("NOTTE_API_KEY", "test-key")
	server := testutil.NewMockServer()
	t.Cleanup(server.Close)
	env.SetEnv("NOTTE_API_URL", server.URL())

	if err := tunnel.Save(tunnel.Tunnel{Name: "lucas-mbp", ExitNode: "lucas-mbp.tail1234.ts.net"}, ""); err != nil {
		t.Fatal(err)
	}
	return fake, server
}

// serveCheckSession answers a check session whose IP lookup shows sessionIP.
func serveCheckSession(server *testutil.MockServer, sessionIP string) {
	server.AddResponse("/sessions/start", 200, checkSessionBody)
	server.AddResponse("/sessions/sess_1/page/execute", 200, `{"success":true,"message":"ok"}`)
	server.AddResponse("/sessions/sess_1/page/scrape", 200, fmt.Sprintf(`{"markdown":%q}`, sessionIP))
	server.AddResponse("/sessions/sess_1/stop", 200, checkSessionBody)
}

func newTunnelCheckCmd(args ...string) *cobra.Command {
	tunnelCheckName = ""
	cmd := &cobra.Command{}
	cmd.Flags().StringVar(&tunnelCheckName, "name", "", "")
	cmd.SetContext(context.Background())
	_ = cmd.Flags().Parse(args)
	return cmd
}

func TestTunnelCheck_PassesWhenTheSessionLeavesFromThisMachine(t *testing.T) {
	_, server := setupTunnelCheck(t, "149.154.233.221")
	serveCheckSession(server, "149.154.233.221")

	out, err := runCaptured(t, func() error { return runTunnelCheck(newTunnelCheckCmd(), nil) })
	if err != nil {
		t.Fatalf("runTunnelCheck() error = %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("output is not JSON: %q", out)
	}
	if result["status"] != "ok" || result["session_ip"] != "149.154.233.221" || result["machine_ip"] != "149.154.233.221" {
		t.Errorf("unexpected result: %v", result)
	}

	starts := server.Requests("/sessions/start")
	if len(starts) != 1 {
		t.Fatalf("got %d start requests", len(starts))
	}
	var body struct {
		Proxies            []map[string]string `json:"proxies"`
		MaxDurationMinutes int                 `json:"max_duration_minutes"`
	}
	if err := json.Unmarshal([]byte(starts[0].Body), &body); err != nil {
		t.Fatalf("bad start body %q: %v", starts[0].Body, err)
	}
	want := map[string]string{"type": "tailnet", "exit_node": "lucas-mbp.tail1234.ts.net"}
	if len(body.Proxies) != 1 || fmt.Sprint(body.Proxies[0]) != fmt.Sprint(want) {
		t.Errorf("proxies = %v, want [%v]", body.Proxies, want)
	}
	if body.MaxDurationMinutes != checkSessionMaxMinutes {
		t.Errorf("max_duration_minutes = %d", body.MaxDurationMinutes)
	}
	if !strings.Contains(server.Requests("/sessions/sess_1/page/execute")[0].Body, ipLookupURL) {
		t.Error("the session did not open the IP lookup")
	}
	if len(server.Requests("/sessions/sess_1/stop")) != 1 {
		t.Error("the check session was not stopped")
	}
}

func TestTunnelCheck_FailsAndStopsTheSessionWhenIPsDiffer(t *testing.T) {
	_, server := setupTunnelCheck(t, "149.154.233.221")
	serveCheckSession(server, "203.0.113.7")

	_, err := runCaptured(t, func() error { return runTunnelCheck(newTunnelCheckCmd("--name", "lucas-mbp"), nil) })
	if err == nil || !strings.Contains(err.Error(), "203.0.113.7") || !strings.Contains(err.Error(), "149.154.233.221") {
		t.Fatalf("got %v", err)
	}
	if len(server.Requests("/sessions/sess_1/stop")) != 1 {
		t.Error("the check session was not stopped")
	}
}

func TestTunnelCheck_StopsWhenNoConnectionOrApproval(t *testing.T) {
	t.Run("no workspace connection", func(t *testing.T) {
		_, server := setupTunnelCheck(t, "149.154.233.221")
		workspaceTailscaleConnected = func(context.Context) (bool, error) { return false, nil }

		_, err := runCaptured(t, func() error { return runTunnelCheck(newTunnelCheckCmd(), nil) })
		if err == nil || !strings.Contains(err.Error(), "/settings/integrations") {
			t.Fatalf("got %v", err)
		}
		if len(server.Requests("/sessions/start")) != 0 {
			t.Error("started a session without a workspace connection")
		}
	})

	t.Run("machine not approved", func(t *testing.T) {
		fake, server := setupTunnelCheck(t, "149.154.233.221")
		fake.approve = false

		_, err := runCaptured(t, func() error { return runTunnelCheck(newTunnelCheckCmd(), nil) })
		if err == nil || !strings.Contains(err.Error(), "not approved as an exit node") {
			t.Fatalf("got %v", err)
		}
		if len(server.Requests("/sessions/start")) != 0 {
			t.Error("started a session for an unapproved machine")
		}
	})
}

func TestTunnelCheck_ExplainsSessionStartFailures(t *testing.T) {
	cases := []struct {
		name, detail, hint string
	}{
		{"no client", "No Tailscale OAuth client is connected to this workspace.", "/settings/integrations"},
		{"bad credentials", "Tailscale rejected these credentials: Auth key minting failed (400)", "tag other than tag:notte"},
		{"exit node", "'lucas-mbp' is not available as an exit node", "autogroup:internet"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, server := setupTunnelCheck(t, "149.154.233.221")
			server.AddResponse("/sessions/start", 400, fmt.Sprintf(`{"detail":%q}`, tc.detail))

			_, err := runCaptured(t, func() error { return runTunnelCheck(newTunnelCheckCmd(), nil) })
			if err == nil || !strings.Contains(err.Error(), tc.hint) {
				t.Fatalf("got %v, want hint %q", err, tc.hint)
			}
			if len(server.Requests("/sessions/sess_1/stop")) != 0 {
				t.Error("stopped a session that never started")
			}
		})
	}
}

func TestTunnelCheck_ReportsWithoutComparingForAnotherMachine(t *testing.T) {
	_, server := setupTunnelCheck(t, "149.154.233.221")
	serveCheckSession(server, "198.51.100.4")
	if err := tunnel.Save(tunnel.Tunnel{Name: "office", ExitNode: "office-mini.tail1234.ts.net"}, ""); err != nil {
		t.Fatal(err)
	}
	lookupPublicIP = func(context.Context) (string, error) {
		t.Error("looked up this machine's IP for another machine's tunnel")
		return "", nil
	}

	out, err := runCaptured(t, func() error { return runTunnelCheck(newTunnelCheckCmd("--name", "office"), nil) })
	if err != nil {
		t.Fatalf("runTunnelCheck() error = %v", err)
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("output is not JSON: %q", out)
	}
	if result["status"] != "unverified" || result["session_ip"] != "198.51.100.4" {
		t.Errorf("unexpected result: %v", result)
	}
}

func TestTunnelCheck_ReportsNavigationFailureAndStops(t *testing.T) {
	_, server := setupTunnelCheck(t, "149.154.233.221")
	serveCheckSession(server, "149.154.233.221")
	server.AddResponse("/sessions/sess_1/page/execute", 200, `{"success":false,"message":"net::ERR_TUNNEL_CONNECTION_FAILED"}`)

	_, err := runCaptured(t, func() error { return runTunnelCheck(newTunnelCheckCmd(), nil) })
	if err == nil || !strings.Contains(err.Error(), "ERR_TUNNEL_CONNECTION_FAILED") {
		t.Fatalf("got %v", err)
	}
	if len(server.Requests("/sessions/sess_1/page/scrape")) != 0 {
		t.Error("scraped after the navigation failed")
	}
	if len(server.Requests("/sessions/sess_1/stop")) != 1 {
		t.Error("the check session was not stopped")
	}
}

func TestDirectHTTPClientIgnoresProxySettings(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "http://proxy.example:8080")
	transport, ok := directHTTPClient().Transport.(*http.Transport)
	if !ok || transport.Proxy != nil {
		t.Error("the machine IP lookup would go through HTTP(S)_PROXY")
	}
}
