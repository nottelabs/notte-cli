package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/testutil"
)

const (
	testListenerToken   = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef_f00dfeedf00dfeedf00dfeedf00dfeedf00dfeedf00dfeedf00dfeedf00dfeed"
	listenerFingerprint = "f00dfeedf00dfeedf00dfeedf00dfeedf00dfeedf00dfeedf00dfeedf00dfeed"
)

// aacCall is one request the fake API received.
type aacCall struct {
	method string
	path   string
	body   map[string]string
}

// dropAfterConnect makes aacAPI close the connection instead of answering a connect.
const dropAfterConnect = -1

// aacAPI fakes the /aac routes, answering `status` to connects.
func aacAPI(t *testing.T, status int) func() []aacCall {
	t.Helper()
	var mu sync.Mutex
	var calls []aacCall
	paymentTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		call := aacCall{method: r.Method, path: r.URL.Path}
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
		if r.Method == http.MethodPost && status == dropAfterConnect {
			// Applied, but the answer never arrives.
			conn, _, err := w.(http.Hijacker).Hijack()
			if err == nil {
				_ = conn.Close()
			}
			return
		}
		if r.Method == http.MethodPost && status != http.StatusOK {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"detail":"refused"}`))
			return
		}
		connected := r.Method == http.MethodPost
		_ = json.NewEncoder(w).Encode(map[string]any{"provider": "bitwarden", "connected": connected, "paired": false})
	})
	return func() []aacCall {
		mu.Lock()
		defer mu.Unlock()
		return append([]aacCall(nil), calls...)
	}
}

// runVaultCommand runs `run`, as `notte vaults connect|disconnect --provider bitwarden args`.
func runVaultCommand(t *testing.T, run func(*cobra.Command, []string) error, args ...string) (string, string, error) {
	t.Helper()
	t.Setenv(aacTokenEnv, "")
	origToken := vaultConnectToken
	vaultConnectProvider, vaultDisconnectProvider, vaultConnectAACProvider = providerBitwarden, providerBitwarden, providerBitwarden
	t.Cleanup(func() { vaultConnectToken = origToken })
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

// connectOnly checks the API got exactly one connect: nothing disconnects the person's connection.
func connectOnly(t *testing.T, got []aacCall) {
	t.Helper()
	if len(got) != 1 || got[0].method != http.MethodPost || got[0].path != "/aac/connect" {
		t.Fatalf("want one connect, got %+v", got)
	}
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
	calls := aacAPI(t, http.StatusOK)
	old := findAAC
	findAAC = func() (string, error) { t.Fatal("no listener is started with a token"); return "", nil }
	t.Cleanup(func() { findAAC = old })

	stdout, _, err := runVaultCommand(t, runVaultsConnect, "--token", testListenerToken)
	if err != nil {
		t.Fatal(err)
	}
	got := calls()
	connectOnly(t, got)
	if got[0].body["token"] != testListenerToken || got[0].body["provider"] != "bitwarden" {
		t.Fatalf("unexpected body %+v", got[0].body)
	}
	if strings.Contains(stdout, testListenerToken) {
		t.Fatal("the token must never be printed")
	}
	if !strings.Contains(stdout, "--aac-vault") {
		t.Fatalf("the output must say how to use it: %q", stdout)
	}
}

func TestVaultsConnectReadsTheTokenFromTheEnvironment(t *testing.T) {
	calls := aacAPI(t, http.StatusOK)
	vaultConnectProvider = providerBitwarden
	t.Setenv(aacTokenEnv, testListenerToken)
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.Flags().StringVar(&vaultConnectToken, "token", "", "")
	var err error
	_, _ = testutil.CaptureOutput(func() { err = runVaultsConnect(cmd, nil) })
	if err != nil {
		t.Fatal(err)
	}
	if got := calls(); len(got) != 1 || got[0].body["token"] != testListenerToken {
		t.Fatalf("unexpected calls %+v", got)
	}
}

func TestVaultsConnectRefusesATokenThatPairsOnce(t *testing.T) {
	calls := aacAPI(t, http.StatusOK)

	_, _, err := runVaultCommand(t, runVaultsConnect, "--token", "ABC-DEF-GHI")

	if err == nil || !strings.Contains(err.Error(), "reusable-psk") {
		t.Fatalf("got %v", err)
	}
	if len(calls()) != 0 {
		t.Fatal("nothing must be sent")
	}
}

func TestVaultsConnectRunsTheListenerUntilItQuitsAndKeepsTheConnection(t *testing.T) {
	calls := aacAPI(t, http.StatusOK)
	home := t.TempDir()
	t.Setenv("HOME", home)
	// The listener stays up until it is connected, then the user quits it.
	binary := fakeListener(t, "while [ ! -f \"$HOME/quit\" ]; do sleep 0.05; done")
	afterConnect(calls, func() { _ = os.WriteFile(filepath.Join(home, "quit"), nil, 0o600) })

	stdout, _, err := runVaultCommand(t, runVaultsConnect)
	if err != nil {
		t.Fatal(err)
	}
	got := calls()
	connectOnly(t, got)
	if want := storedPSK(t, home) + "_" + listenerFingerprint; got[0].body["token"] != want {
		t.Fatalf("token %q, want the stored PSK and the listener fingerprint", got[0].body["token"])
	}
	invocations, err := os.ReadFile(filepath.Join(filepath.Dir(binary), "calls"))
	if err != nil || !strings.Contains(string(invocations), "listen --reusable-psk --provider bitwarden\n") {
		t.Fatalf("aac called as %q", invocations)
	}
	if !strings.Contains(stdout, "stays connected") {
		t.Fatalf("unexpected output %q", stdout)
	}
}

func TestVaultsConnectStopsTheListenerWhenTheAPIRefusesIt(t *testing.T) {
	calls := aacAPI(t, http.StatusUnprocessableEntity)
	t.Setenv("HOME", t.TempDir())
	// Quits on interrupt, as aac does.
	_ = fakeListener(t, "trap 'exit 130' INT TERM; while true; do sleep 0.05; done")

	_, _, err := runVaultCommand(t, runVaultsConnect)

	if err == nil {
		t.Fatal("the refused connect must fail the command")
	}
	connectOnly(t, calls())
}

func TestVaultsDisconnect(t *testing.T) {
	calls := aacAPI(t, http.StatusOK)

	stdout, _, err := runVaultCommand(t, runVaultsDisconnect)
	if err != nil {
		t.Fatal(err)
	}
	if got := calls(); len(got) != 1 || got[0].method != http.MethodDelete || got[0].path != "/aac/connection" {
		t.Fatalf("unexpected calls %+v", got)
	}
	if !strings.Contains(stdout, "disconnected") {
		t.Fatalf("unexpected output %q", stdout)
	}
}

func TestVaultsDisconnectRefusesAnotherProvider(t *testing.T) {
	calls := aacAPI(t, http.StatusOK)
	vaultDisconnectProvider = "1password"
	t.Cleanup(func() { vaultDisconnectProvider = providerBitwarden })
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	if err := runVaultsDisconnect(cmd, nil); err == nil {
		t.Fatal("only bitwarden can be disconnected")
	}
	if len(calls()) != 0 {
		t.Fatal("nothing must be sent")
	}
}

// stopsFor makes the command read stop signals from the returned channel.
func stopsFor(t *testing.T) chan os.Signal {
	t.Helper()
	stops := make(chan os.Signal, 1)
	old := notifyStop
	notifyStop = func() (<-chan os.Signal, func()) { return stops, func() {} }
	t.Cleanup(func() { notifyStop = old })
	return stops
}

// afterConnect runs fn once the API received its first request.
func afterConnect(calls func() []aacCall, fn func()) {
	go func() {
		for len(calls()) == 0 {
			time.Sleep(10 * time.Millisecond)
		}
		fn()
	}()
}

func TestAStopSignalToTheCLIAloneStopsTheListener(t *testing.T) {
	calls := aacAPI(t, http.StatusOK)
	home := t.TempDir()
	t.Setenv("HOME", home)
	stops := stopsFor(t)
	// Quits on SIGTERM as aac does, noting it got it.
	_ = fakeListener(t, "trap 'touch \"$HOME/stopped\"; exit 143' TERM; while true; do sleep 0.05; done")
	afterConnect(calls, func() { stops <- syscall.SIGTERM })

	_, _, err := runVaultCommand(t, runVaultsConnect)
	if err != nil {
		t.Fatalf("a requested stop is not a failure: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, "stopped")); err != nil {
		t.Fatal("the signal must be forwarded to aac")
	}
	connectOnly(t, calls())
}

func TestAListenerIgnoringTheStopIsKilled(t *testing.T) {
	calls := aacAPI(t, http.StatusOK)
	t.Setenv("HOME", t.TempDir())
	stops := stopsFor(t)
	old := listenerStopTimeout
	listenerStopTimeout = 100 * time.Millisecond
	t.Cleanup(func() { listenerStopTimeout = old })
	_ = fakeListener(t, "trap '' TERM INT; while true; do sleep 0.05; done")
	afterConnect(calls, func() { stops <- syscall.SIGTERM })

	if _, _, err := runVaultCommand(t, runVaultsConnect); err != nil {
		t.Fatal(err)
	}
	connectOnly(t, calls())
}

func TestAListenerFailingAfterTheConnectIsReported(t *testing.T) {
	calls := aacAPI(t, http.StatusOK)
	home := t.TempDir()
	t.Setenv("HOME", home)
	_ = fakeListener(t, "while [ ! -f \"$HOME/quit\" ]; do sleep 0.05; done; exit 3")
	afterConnect(calls, func() { _ = os.WriteFile(filepath.Join(home, "quit"), nil, 0o600) })

	_, _, err := runVaultCommand(t, runVaultsConnect)

	if err == nil || !strings.Contains(err.Error(), "aac listen failed") {
		t.Fatalf("got %v, want aac's failure", err)
	}
	connectOnly(t, calls())
}

func TestARefusedConnectStopsAListenerIgnoringTheInterrupt(t *testing.T) {
	calls := aacAPI(t, http.StatusUnprocessableEntity)
	t.Setenv("HOME", t.TempDir())
	old := listenerStopTimeout
	listenerStopTimeout = 100 * time.Millisecond
	t.Cleanup(func() { listenerStopTimeout = old })
	_ = fakeListener(t, "trap '' TERM INT; while true; do sleep 0.05; done")

	done := make(chan error, 1)
	go func() {
		_, _, err := runVaultCommand(t, runVaultsConnect)
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("the refused connect must fail the command")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the command hung on a listener that ignores the interrupt")
	}
	connectOnly(t, calls())
}

func TestAConnectWhoseAnswerIsLostFailsAndStopsTheListener(t *testing.T) {
	calls := aacAPI(t, dropAfterConnect)
	home := t.TempDir()
	t.Setenv("HOME", home)
	_ = fakeListener(t, "trap 'touch \"$HOME/stopped\"; exit 130' INT TERM; while true; do sleep 0.05; done")

	_, _, err := runVaultCommand(t, runVaultsConnect)

	if err == nil {
		t.Fatal("an unconfirmed connect must fail the command")
	}
	if _, statErr := os.Stat(filepath.Join(home, "stopped")); statErr != nil {
		t.Fatal("the listener must be stopped")
	}
	if got := calls(); len(got) == 0 || got[len(got)-1].method != http.MethodPost {
		t.Fatalf("an existing connection must not be deleted, got %+v", got)
	}
}
