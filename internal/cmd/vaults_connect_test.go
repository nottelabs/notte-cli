package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/testutil"
)

const (
	vaultConnectSessionID = "00000000-0000-4000-8000-000000000002"
	testListenerToken     = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef_f00dfeedf00dfeedf00dfeedf00dfeedf00dfeedf00dfeedf00dfeedf00dfeed"
	listenerFingerprint   = "f00dfeedf00dfeedf00dfeedf00dfeedf00dfeedf00dfeedf00dfeedf00dfeed"
)

// vaultConnectionCall is one request the fake API received.
type vaultConnectionCall struct {
	method string
	path   string
	body   map[string]string
}

// vaultConnectionAPI fakes the session's vault connection routes, answering `status` to connects.
func vaultConnectionAPI(t *testing.T, status int) func() []vaultConnectionCall {
	t.Helper()
	var mu sync.Mutex
	var calls []vaultConnectionCall
	paymentTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		call := vaultConnectionCall{method: r.Method, path: r.URL.Path}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("missing auth")
		}
		if r.Method == http.MethodPost {
			if err := json.NewDecoder(r.Body).Decode(&call.body); err != nil {
				t.Error(err)
			}
		}
		mu.Lock()
		calls = append(calls, call)
		mu.Unlock()
		if r.Method == http.MethodPost && status != http.StatusOK {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"detail":"Session not found"}`))
			return
		}
		connected := r.Method == http.MethodPost
		_ = json.NewEncoder(w).Encode(map[string]any{"session_id": vaultConnectSessionID, "provider": "bitwarden", "connected": connected})
	})
	return func() []vaultConnectionCall {
		mu.Lock()
		defer mu.Unlock()
		return append([]vaultConnectionCall(nil), calls...)
	}
}

// runVaultCommand runs `run` for the session, as `notte vaults connect|disconnect --session-id ... args`.
func runVaultCommand(t *testing.T, run func(*cobra.Command, []string) error, args ...string) (string, string, error) {
	t.Helper()
	t.Setenv(aacTokenEnv, "")
	origSession, origToken := sessionID, vaultConnectToken
	sessionID, vaultConnectProvider, vaultConnectAACProvider = vaultConnectSessionID, providerBitwarden, providerBitwarden
	t.Cleanup(func() { sessionID, vaultConnectToken = origSession, origToken })
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.Flags().StringVar(&vaultConnectToken, "token", "", "")
	if err := cmd.Flags().Parse(args); err != nil {
		t.Fatal(err)
	}
	var err error
	stdout, stderr := testutil.CaptureOutput(func() { err = run(cmd, nil) })
	return stdout, stderr, err
}

// fakeListener writes an aac whose `listen` runs `listen` (shell) and whose `connections list` prints the identity.
func fakeListener(t *testing.T, listen string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "aac")
	script := "#!/bin/sh\n" +
		"echo \"$*\" >> \"" + filepath.Join(dir, "calls") + "\"\n" +
		"if [ \"$1\" = connections ]; then echo \"  Your identity: " + listenerFingerprint + "\"; exit 0; fi\n" +
		listen + "\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	old := findAAC
	findAAC = func() (string, error) { return path, nil }
	t.Cleanup(func() { findAAC = old })
	return path
}

func storedPSK(t *testing.T, home string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(home, ".access-protocol", "psk_store_user_client.json"))
	if err != nil {
		t.Fatal(err)
	}
	var store struct {
		Psks []struct {
			PskHex string `json:"psk_hex"`
		} `json:"psks"`
	}
	if err := json.Unmarshal(data, &store); err != nil || len(store.Psks) != 1 {
		t.Fatalf("unexpected store %s: %v", data, err)
	}
	return store.Psks[0].PskHex
}

func TestVaultsConnectWithATokenConnectsAndReturns(t *testing.T) {
	calls := vaultConnectionAPI(t, http.StatusOK)
	old := findAAC
	findAAC = func() (string, error) { t.Fatal("no listener is started with a token"); return "", nil }
	t.Cleanup(func() { findAAC = old })

	stdout, _, err := runVaultCommand(t, runVaultsConnect, "--token", testListenerToken)
	if err != nil {
		t.Fatal(err)
	}
	got := calls()
	if len(got) != 1 || got[0].method != http.MethodPost || got[0].path != "/sessions/"+vaultConnectSessionID+"/vault/connection" {
		t.Fatalf("unexpected calls %+v", got)
	}
	if got[0].body["token"] != testListenerToken || got[0].body["provider"] != "bitwarden" {
		t.Fatalf("unexpected body %+v", got[0].body)
	}
	if strings.Contains(stdout, testListenerToken) {
		t.Fatal("the token must never be printed")
	}
}

func TestVaultsConnectRefusesATokenThatPairsOnce(t *testing.T) {
	calls := vaultConnectionAPI(t, http.StatusOK)

	_, _, err := runVaultCommand(t, runVaultsConnect, "--token", "ABC-DEF-GHI")

	if err == nil || !strings.Contains(err.Error(), "reusable-psk") {
		t.Fatalf("got %v", err)
	}
	if len(calls()) != 0 {
		t.Fatal("nothing must be sent")
	}
}

func TestVaultsConnectRunsTheListenerUntilItQuitsThenDisconnects(t *testing.T) {
	calls := vaultConnectionAPI(t, http.StatusOK)
	home := t.TempDir()
	t.Setenv("HOME", home)
	// The listener stays up until the session is connected, then the user quits it.
	binary := fakeListener(t, "while [ ! -f \"$HOME/quit\" ]; do sleep 0.05; done")
	go func() {
		for len(calls()) == 0 {
			time.Sleep(10 * time.Millisecond)
		}
		_ = os.WriteFile(filepath.Join(home, "quit"), nil, 0o600)
	}()

	stdout, _, err := runVaultCommand(t, runVaultsConnect)
	if err != nil {
		t.Fatal(err)
	}
	got := calls()
	if len(got) != 2 || got[0].method != http.MethodPost || got[1].method != http.MethodDelete {
		t.Fatalf("want a connect then a disconnect, got %+v", got)
	}
	if want := storedPSK(t, home) + "_" + listenerFingerprint; got[0].body["token"] != want {
		t.Fatalf("token %q, want the stored PSK and the listener fingerprint", got[0].body["token"])
	}
	invocations, err := os.ReadFile(filepath.Join(filepath.Dir(binary), "calls"))
	if err != nil || !strings.Contains(string(invocations), "listen --reusable-psk --provider bitwarden\n") {
		t.Fatalf("aac called as %q", invocations)
	}
	if !strings.Contains(stdout, "disconnected") {
		t.Fatalf("unexpected output %q", stdout)
	}
}

func TestVaultsConnectStopsTheListenerWhenTheSessionRefusesIt(t *testing.T) {
	calls := vaultConnectionAPI(t, http.StatusNotFound)
	t.Setenv("HOME", t.TempDir())
	// Quits on interrupt, as aac does.
	_ = fakeListener(t, "trap 'exit 130' INT TERM; while true; do sleep 0.05; done")

	_, _, err := runVaultCommand(t, runVaultsConnect)

	if err == nil {
		t.Fatal("the refused connect must fail the command")
	}
	if got := calls(); len(got) != 1 || got[0].method != http.MethodPost {
		t.Fatalf("nothing to disconnect after a refused connect, got %+v", got)
	}
}

func TestVaultsDisconnect(t *testing.T) {
	calls := vaultConnectionAPI(t, http.StatusOK)

	_, _, err := runVaultCommand(t, runVaultsDisconnect)
	if err != nil {
		t.Fatal(err)
	}
	if got := calls(); len(got) != 1 || got[0].method != http.MethodDelete || got[0].path != "/sessions/"+vaultConnectSessionID+"/vault/connection" {
		t.Fatalf("unexpected calls %+v", got)
	}
}
