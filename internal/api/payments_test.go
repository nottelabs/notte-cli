package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPaymentTransport(t *testing.T) {
	for _, status := range []int{202, 409, 429} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Header.Get("Authorization") != "Bearer key" || r.Header.Get(IdempotencyKeyHeader) != "stable-key" {
					t.Error("missing auth or stable key")
				}
				if r.URL.Path != "/sessions/sess_test/payments" || r.Method != "POST" {
					t.Error("wrong request")
				}
				w.WriteHeader(status)
				_, _ = w.Write([]byte(`{"id":"payment","status":"awaiting_approval","amount":"100.91","card_number":"secret"}`))
			}))
			defer server.Close()
			client, err := NewClientWithURL("key", server.URL, "")
			if err != nil {
				t.Fatal(err)
			}
			result, resp, _, err := client.Payment(context.Background(), "sess_test", "", "stable-key", &SessionPaymentRequest{Amount: "100.91", Mode: "test"})
			if err != nil || resp.StatusCode != status || calls != 1 {
				t.Fatalf("unexpected response: %v %v calls=%d", resp, err, calls)
			}
			if status == 202 && (result == nil || result.ID != "payment" || result.Amount != "100.91") {
				t.Fatal("missing response")
			}
			if status != 202 && result != nil {
				t.Fatal("error interpreted as success")
			}
		})
	}
}

func TestPaymentTransportCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	client, err := NewClientWithURL("key", server.URL, "")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, _, _, err := client.Payment(ctx, "", "id", "", nil); err == nil {
		t.Fatal("expected cancellation")
	}
}
