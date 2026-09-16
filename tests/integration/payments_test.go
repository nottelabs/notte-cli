//go:build integration

package integration

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// TestPaymentSandboxLifecycle uses real CLI subprocesses and the deployed API.
// It never approves a Link request or submits a merchant checkout. The approved
// credential path is covered separately by browser tests with simulated Link.
func TestPaymentSandboxLifecycle(t *testing.T) {
	// Avoid stopping a developer's saved session when sessions start is invoked.
	t.Setenv("NOTTE_CONFIG_DIR", t.TempDir())

	// A missing random payment should return 404 on an enabled, authenticated API.
	// Only the explicit feature-disabled response can skip this test. Auth errors,
	// outages, and missing routes must fail rather than silently hide regressions.
	probe := runCLI(t, "payment", "status", uuid.NewString())
	requireFailure(t, probe)
	var apiError struct {
		StatusCode int    `json:"status_code"`
		Error      string `json:"error"`
	}
	if err := json.NewDecoder(strings.NewReader(probe.Stderr)).Decode(&apiError); err != nil {
		t.Fatal("payment preflight did not return a structured API error")
	}
	if apiError.StatusCode == 503 && apiError.Error == "payments_disabled" && os.Getenv("NOTTE_PAYMENT_E2E_REQUIRED") != "1" {
		t.Skip("deployment has payments disabled; set NOTTE_PAYMENT_E2E_REQUIRED=1 to require an enabled deployment")
	}
	if apiError.StatusCode != 404 || apiError.Error != "payment_not_found" {
		t.Fatalf("payment preflight failed: status=%d error=%s", apiError.StatusCode, apiError.Error)
	}

	started := runCLI(t, "sessions", "start", "--proxy=false", "--no-solve-captchas", "--idle-timeout-minutes", "5", "--max-duration-minutes", "10")
	requireSuccess(t, started)
	var session struct {
		ID string `json:"session_id"`
	}
	if err := json.Unmarshal([]byte(started.Stdout), &session); err != nil || session.ID == "" {
		t.Fatal("session start returned no session ID")
	}
	stopped := false
	t.Cleanup(func() {
		if !stopped {
			cleanupSession(t, session.ID)
		}
	})

	type payment struct {
		ID               string      `json:"id"`
		SessionID        string      `json:"session_id"`
		Status           string      `json:"status"`
		Mode             string      `json:"mode"`
		Amount           json.Number `json:"amount"`
		Currency         string      `json:"currency"`
		PurchaseStatus   string      `json:"purchase_status"`
		ConnectionURL    string      `json:"connection_url"`
		ConnectionPhrase string      `json:"connection_phrase"`
		ApprovalURL      string      `json:"approval_url"`
	}
	decode := func(result CLIResult) payment {
		t.Helper()
		requireSuccess(t, result)
		var fields map[string]json.RawMessage
		if err := json.Unmarshal([]byte(result.Stdout), &fields); err != nil {
			t.Fatal("payment stdout is not a single JSON object")
		}
		for _, key := range []string{"card_number", "card_cvv", "access_token", "refresh_token", "device_code"} {
			if _, ok := fields[key]; ok {
				t.Fatalf("payment response exposed forbidden field %s", key)
			}
		}
		var p payment
		if err := json.Unmarshal([]byte(result.Stdout), &p); err != nil {
			t.Fatal("invalid payment JSON")
		}
		if p.ID == "" || p.SessionID != session.ID || p.Mode != "test" || (p.Amount != "1.00" && p.Amount != "1") || p.Currency != "usd" || p.PurchaseStatus != "unverified" {
			t.Fatal("payment identity, sandbox mode, amount, or purchase semantics did not match")
		}
		return p
	}
	key := "cli-payment-e2e-" + uuid.NewString()
	args := []string{"payment", "request", "--session-id", session.ID, "--amount", "1.00", "--currency", "usd", "--mode", "test", "--merchant-url", "https://example.com", "--merchant-name", "CLI sandbox integration test", "--description", "Exercise the sandbox payment lifecycle for a CLI integration test. No wallet approval, real purchase, or merchant order is involved.", "--idempotency-key", key}
	// Connect is independent of the browser session and does not wait for a human.
	connected := runCLI(t, "payment", "connect", "--mode", "test")
	requireSuccess(t, connected)
	var wallet struct {
		ID               string `json:"id"`
		Status           string `json:"status"`
		ConnectionURL    string `json:"connection_url"`
		ConnectionPhrase string `json:"connection_phrase"`
	}
	if err := json.Unmarshal([]byte(connected.Stdout), &wallet); err != nil {
		t.Fatal(err)
	}
	if wallet.Status != "connected" {
		deadline := time.Now().Add(30 * time.Second)
		for wallet.Status == "creating" && time.Now().Before(deadline) {
			time.Sleep(2 * time.Second)
			connected = runCLI(t, "payment", "connect", "--mode", "test")
			requireSuccess(t, connected)
			if err := json.Unmarshal([]byte(connected.Stdout), &wallet); err != nil {
				t.Fatal(err)
			}
		}
		if wallet.Status != "awaiting_connection" || wallet.ConnectionURL == "" || wallet.ConnectionPhrase == "" {
			t.Fatal("missing wallet connection instructions")
		}
		denied := runCLI(t, args...)
		requireFailure(t, denied)
		if !strings.Contains(denied.Stderr, "wallet_not_connected") || !strings.Contains(denied.Stderr, "payment connect") {
			t.Fatal("missing connect-first guidance")
		}
		return // Manual wallet authorization is intentionally not automated in CI.
	}
	requested := decode(runCLI(t, args...))
	replay := decode(runCLI(t, args...))
	if replay.ID != requested.ID {
		t.Fatal("idempotent replay created a different payment")
	}
	get := func() payment { return decode(runCLI(t, "payment", "status", requested.ID)) }

	// Connection creation can be asynchronous. Allow time for the backend's
	// reconciler, including a wallet already connected by a prior sandbox run.
	deadline := time.Now().Add(90 * time.Second)
	current := get()
	for (current.Status == "creating") && time.Now().Before(deadline) {
		time.Sleep(2 * time.Second)
		current = get()
	}
	switch current.Status {
	case "awaiting_approval":
		if current.ApprovalURL == "" {
			t.Fatal("missing spending approval URL")
		}
	default:
		t.Fatalf("expected an unapproved sandbox request, got %s", current.Status)
	}

	waiting := runCLI(t, "payment", "wait", requested.ID, "--wait-timeout", "1s")
	requireFailure(t, waiting)
	if strings.TrimSpace(waiting.Stdout) != "" || !strings.Contains(waiting.Stderr, "provisioning continues") || !strings.Contains(waiting.Stderr, requested.ID) {
		t.Fatal("wait timeout did not preserve JSON stdout and recovery instructions")
	}
	current = get()
	if current.Status != "awaiting_approval" && current.Status != "creating" {
		t.Fatalf("wait timeout changed request to unexpected status %s", current.Status)
	}

	requireSuccess(t, runCLI(t, "sessions", "stop", "--session-id", session.ID))
	stopped = true
	deadline = time.Now().Add(30 * time.Second)
	current = get()
	for current.Status != "closed" && time.Now().Before(deadline) {
		time.Sleep(time.Second)
		current = get()
	}
	if current.Status != "closed" {
		t.Fatalf("payment did not close with session: %s", current.Status)
	}
	finished := runCLI(t, "payment", "wait", requested.ID, "--wait-timeout", "5s")
	requireFailure(t, finished)
	if !strings.Contains(finished.Stderr, "status closed") {
		t.Fatal("wait did not report terminal session closure")
	}
}
