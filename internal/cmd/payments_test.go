package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/api"
	"github.com/nottelabs/notte-cli/internal/testutil"
)

const paymentTestID = "00000000-0000-4000-8000-000000000001"

func paymentTestEnv(t *testing.T, h http.HandlerFunc) *api.NotteClient {
	t.Helper()
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")
	server := httptest.NewServer(h)
	t.Cleanup(server.Close)
	env.SetEnv("NOTTE_API_URL", server.URL)
	old := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = old })
	client, err := api.NewClientWithURL("test-key", server.URL, "")
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestPaymentRequest(t *testing.T) {
	paymentTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/sessions/"+paymentTestID+"/payments" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" || r.Header.Get("Idempotency-Key") != "replay-key" {
			t.Error("missing auth or idempotency key")
		}
		var p api.SessionPaymentRequest
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			t.Error(err)
		}
		if p.Mode != "test" || p.Amount != "100.91" {
			t.Errorf("unexpected body: %+v", p)
		}
		w.WriteHeader(202)
		_, _ = fmt.Fprintf(w, `{"id":%q,"status":"awaiting_approval","approval_url":"https://app.link.com/device","access_token":"must-not-print"}`, paymentTestID)
	})
	cmd := newPaymentCommand()
	cmd.SetArgs([]string{"request", "--session-id", paymentTestID, "--amount", "100.91", "--merchant-url", "https://example.com", "--merchant-name", "Example", "--description", strings.Repeat("x", 100), "--idempotency-key", "replay-key"})
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	out, _ := testutil.CaptureOutput(func() {
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
	})
	var result map[string]any
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatal(err)
	}
	if result["status"] != "awaiting_approval" || strings.Contains(out, "must-not-print") {
		t.Fatalf("unsafe or incorrect result: %s", out)
	}
	if !strings.Contains(stderr.String(), "replay-key") {
		t.Error("missing recovery key")
	}
}

func TestPaymentWaitInstructionsAndReady(t *testing.T) {
	calls := 0
	client := paymentTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/payments/"+paymentTestID {
			t.Errorf("unexpected mutation or path")
		}
		states := []string{"awaiting_connection", "awaiting_connection", "awaiting_approval", "provisioning", "ready"}
		state := states[calls]
		calls++
		_, _ = fmt.Fprintf(w, `{"id":%q,"status":%q,"connection_url":"https://app.link.com/device","connection_phrase":"example-phrase","approval_url":"https://app.link.com/approve","purchase_status":"unverified"}`, paymentTestID, state)
	})
	cmd := &cobra.Command{}
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	out, _ := testutil.CaptureOutput(func() {
		if err := waitPayment(cmd, context.Background(), client, paymentTestID, time.Millisecond); err != nil {
			t.Fatal(err)
		}
	})
	if strings.Count(stderr.String(), "Connect wallet:") != 1 || !strings.Contains(stderr.String(), "Approve spending:") {
		t.Fatalf("missing or duplicate instructions: %s", stderr.String())
	}
	var result api.PaymentStatus
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatal(err)
	}
	if result.Status != "ready" || result.PurchaseStatus != "unverified" {
		t.Fatal("incorrect ready semantics")
	}
}

func TestPaymentWaitFailures(t *testing.T) {
	for _, state := range []string{"declined", "expired", "failed", "closed", "future_status"} {
		t.Run(state, func(t *testing.T) {
			client := paymentTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
				_, _ = fmt.Fprintf(w, `{"id":%q,"status":%q,"error_code":"test_failure"}`, paymentTestID, state)
			})
			err := waitPayment(&cobra.Command{}, context.Background(), client, paymentTestID, time.Millisecond)
			if err == nil || !strings.Contains(err.Error(), state) {
				t.Fatalf("expected failure, got %v", err)
			}
		})
	}
}

func TestPaymentWaitCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	client := paymentTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		cancel()
		_, _ = fmt.Fprintf(w, `{"id":%q,"status":"provisioning"}`, paymentTestID)
	})
	err := waitPayment(&cobra.Command{}, ctx, client, paymentTestID, time.Hour)
	if err == nil || !strings.Contains(err.Error(), "provisioning continues") || calls != 1 {
		t.Fatalf("unexpected cancellation: %v, calls=%d", err, calls)
	}
}

func TestPaymentRequestValidation(t *testing.T) {
	for _, args := range [][]string{{"--amount", "1e2"}, {"--amount", "NaN"}, {"--amount", "-1"}, {"--amount", "0"}, {"--amount", "50001"}, {"--currency", "USD"}, {"--merchant-url", "http://example.com"}, {"--merchant-url", "https://user:pass@example.com"}, {"--description", "short"}, {"--mode", "production"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			cmd := newPaymentCommand()
			base := []string{"request", "--session-id", paymentTestID, "--amount", "100.91", "--merchant-url", "https://example.com", "--merchant-name", "Example", "--description", strings.Repeat("x", 100)}
			cmd.SetArgs(append(base, args...))
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})
			if err := cmd.Execute(); err == nil {
				t.Fatal("expected validation failure")
			}
		})
	}
}

func TestPaymentStatusAndAPIErrors(t *testing.T) {
	for _, code := range []int{200, 401, 403, 409} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			paymentTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(code)
				if code == 200 {
					_, _ = fmt.Fprintf(w, `{"id":%q,"status":"awaiting_approval"}`, paymentTestID)
				} else {
					_, _ = w.Write([]byte(`{"detail":"payment_card_slot_occupied"}`))
				}
			})
			cmd := newPaymentCommand()
			cmd.SetArgs([]string{"status", paymentTestID})
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})
			testutil.CaptureOutput(func() {
				err := cmd.Execute()
				if (err != nil) != (code != 200) {
					t.Fatalf("status %d: %v", code, err)
				}
			})
		})
	}
}

func TestPaymentWaitVerification(t *testing.T) {
	for _, resolution := range []string{"auto_resume", "create_new_spend_request", "create_new_spend_request_after_completion"} {
		t.Run(resolution, func(t *testing.T) {
			calls := 0
			client := paymentTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" {
					t.Error("verification must not create or approve spending")
				}
				calls++
				if calls > 2 {
					_, _ = fmt.Fprintf(w, `{"id":%q,"status":"ready"}`, paymentTestID)
					return
				}
				status := "requires_action"
				if resolution != "auto_resume" {
					status = "failed"
				}
				_, _ = fmt.Fprintf(w, `{"id":%q,"status":%q,"next_action":{"type":"three_d_secure","resolution":%q,"action_url":"https://app.link.com/verify"},"error_code":"link_new_spend_request_required"}`, paymentTestID, status, resolution)
			})
			cmd := &cobra.Command{}
			var stderr bytes.Buffer
			cmd.SetErr(&stderr)
			var resultErr error
			testutil.CaptureOutput(func() { resultErr = waitPayment(cmd, context.Background(), client, paymentTestID, time.Millisecond) })
			if strings.Count(stderr.String(), "Complete verification:") != 1 {
				t.Fatalf("missing or duplicate instructions: %s", stderr.String())
			}
			if resolution == "auto_resume" {
				if resultErr != nil || calls != 3 {
					t.Fatalf("did not resume: %v (%d calls)", resultErr, calls)
				}
			} else if resultErr == nil || calls != 1 || !strings.Contains(stderr.String(), "new idempotency key") {
				t.Fatalf("must stop and explain new request: %v %s", resultErr, stderr.String())
			}
		})
	}
}

func TestPaymentConnectWithoutSession(t *testing.T) {
	for _, state := range []string{"awaiting_connection", "connected"} {
		t.Run(state, func(t *testing.T) {
			paymentTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" || r.URL.Path != "/payments/connect" || r.Header.Get("Authorization") != "Bearer test-key" {
					t.Fatalf("unexpected connection request")
				}
				var body map[string]string
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if body["mode"] != "test" {
					t.Fatal("wrong mode")
				}
				_, _ = fmt.Fprintf(w, `{"id":%q,"status":%q,"mode":"test","connection_url":"https://app.link.com/device/setup","connection_phrase":"test-phrase","access_token":"secret-must-not-print","device_code":"private-device"}`, paymentTestID, state)
			})
			cmd := newPaymentCommand()
			cmd.SetArgs([]string{"connect", "--mode", "test"})
			out, _ := testutil.CaptureOutput(func() {
				if err := cmd.Execute(); err != nil {
					t.Fatal(err)
				}
			})
			var result map[string]any
			if err := json.Unmarshal([]byte(out), &result); err != nil {
				t.Fatal(err)
			}
			if result["status"] != state || strings.Contains(out, "secret-must-not-print") || strings.Contains(out, "private-device") {
				t.Fatalf("unsafe response %s", out)
			}
		})
	}
}

func TestPaymentRequestRequiresWalletConnection(t *testing.T) {
	client := paymentTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(409)
		_, _ = w.Write([]byte(`{"detail":"wallet_not_connected"}`))
	})
	_, err := fetchPayment(context.Background(), client, paymentTestID, "", "key", &api.SessionPaymentRequest{Mode: "live"})
	if err == nil || !strings.Contains(err.Error(), "notte payment connect --mode live") {
		t.Fatalf("missing connection guidance: %v", err)
	}
}
