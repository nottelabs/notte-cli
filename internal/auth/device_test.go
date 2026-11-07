package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nottelabs/notte-cli/internal/config"
)

// fakeDeviceAPI answers /auth/device/token with the scripted responses in order.
type fakeDeviceAPI struct {
	mu        sync.Mutex
	responses []func(w http.ResponseWriter)
	polls     int
	codeBody  map[string]string
	revoked   []string
}

func (f *fakeDeviceAPI) handler(t *testing.T) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/auth/device/code", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("code request content type = %q", r.Header.Get("Content-Type"))
		}
		_ = json.NewDecoder(r.Body).Decode(&f.codeBody)
		writeJSON(w, http.StatusOK, map[string]any{
			"device_code":               "secret-device-code",
			"user_code":                 "HNVS-GHLG",
			"verification_uri":          "https://console.notte.cc/oauth/device",
			"verification_uri_complete": "https://console.notte.cc/oauth/device?user_code=HNVS-GHLG",
			"expires_in":                900,
			"interval":                  5,
		})
	})
	mux.HandleFunc("/auth/device/token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		if got := r.PostForm.Get("grant_type"); got != deviceCodeGrantType {
			t.Errorf("grant_type = %q", got)
		}
		if got := r.PostForm.Get("device_code"); got != "secret-device-code" {
			t.Errorf("device_code = %q", got)
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.polls >= len(f.responses) {
			t.Fatalf("unexpected poll %d", f.polls+1)
		}
		f.responses[f.polls](w)
		f.polls++
	})
	mux.HandleFunc("/auth/device/revoke", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		token := r.PostForm.Get("token")
		f.revoked = append(f.revoked, token)
		writeJSON(w, http.StatusOK, map[string]any{"revoked": strings.HasPrefix(token, "sk-notte-cli")})
	})
	return mux
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func oauthErr(status int, code string) func(w http.ResponseWriter) {
	return func(w http.ResponseWriter) {
		writeJSON(w, status, map[string]string{"error": code, "error_description": code + " description"})
	}
}

func issued(w http.ResponseWriter) {
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":   "sk-notte-cli-new",
		"token_type":     "Bearer",
		"key_name":       "CLI · mac · 2026-11-07",
		"email":          "dev@example.com",
		"workspace_name": nil,
	})
}

func newTestFlow(t *testing.T, api *fakeDeviceAPI) (*DeviceFlow, *[]time.Duration) {
	t.Helper()
	server := httptest.NewServer(api.handler(t))
	t.Cleanup(server.Close)
	waits := []time.Duration{}
	flow := NewDeviceFlow(server.URL, "test")
	flow.Sleep = func(ctx context.Context, d time.Duration) error {
		waits = append(waits, d)
		return ctx.Err()
	}
	return flow, &waits
}

func TestDeviceFlowReportsTheMachineAndReturnsTheCodes(t *testing.T) {
	api := &fakeDeviceAPI{}
	flow, _ := newTestFlow(t, api)

	code, err := flow.RequestCode(context.Background(), DeviceClientInfo{Hostname: "mac", OS: "darwin (arm64)", Version: "0.0.68"})
	if err != nil {
		t.Fatalf("RequestCode: %v", err)
	}
	if code.UserCode != "HNVS-GHLG" || code.Interval != 5 || code.ExpiresIn != 900 {
		t.Errorf("unexpected code %+v", code)
	}
	want := map[string]string{"client_hostname": "mac", "client_os": "darwin (arm64)", "client_version": "0.0.68"}
	for k, v := range want {
		if api.codeBody[k] != v {
			t.Errorf("code request %s = %q, want %q", k, api.codeBody[k], v)
		}
	}
}

func TestDeviceFlowPollsUntilApproved(t *testing.T) {
	api := &fakeDeviceAPI{responses: []func(http.ResponseWriter){
		oauthErr(http.StatusBadRequest, "authorization_pending"),
		oauthErr(http.StatusBadRequest, "slow_down"),
		oauthErr(http.StatusServiceUnavailable, "server_error"),
		oauthErr(http.StatusBadRequest, "authorization_pending"),
		issued,
	}}
	flow, waits := newTestFlow(t, api)
	code, err := flow.RequestCode(context.Background(), DeviceClientInfo{})
	if err != nil {
		t.Fatal(err)
	}

	token, err := flow.Poll(context.Background(), code)
	if err != nil {
		t.Fatalf("Poll: %v", err)
	}
	if token.AccessToken != "sk-notte-cli-new" || token.Email != "dev@example.com" || token.WorkspaceName != "" {
		t.Errorf("unexpected token %+v", token)
	}
	// slow_down adds 5 s for every later poll; a 5xx is retried at the same pace.
	wantWaits := []time.Duration{5 * time.Second, 5 * time.Second, 10 * time.Second, 10 * time.Second, 10 * time.Second}
	if len(*waits) != len(wantWaits) {
		t.Fatalf("waits = %v, want %v", *waits, wantWaits)
	}
	for i := range wantWaits {
		if (*waits)[i] != wantWaits[i] {
			t.Errorf("wait %d = %v, want %v", i, (*waits)[i], wantWaits[i])
		}
	}
}

func TestDeviceFlowStopsOnDenialAndExpiry(t *testing.T) {
	for _, tc := range []struct {
		code string
		want error
	}{
		{"access_denied", ErrLoginDenied},
		{"expired_token", ErrLoginExpired},
	} {
		t.Run(tc.code, func(t *testing.T) {
			api := &fakeDeviceAPI{responses: []func(http.ResponseWriter){oauthErr(http.StatusBadRequest, tc.code)}}
			flow, _ := newTestFlow(t, api)
			code, _ := flow.RequestCode(context.Background(), DeviceClientInfo{})
			if _, err := flow.Poll(context.Background(), code); !errors.Is(err, tc.want) {
				t.Fatalf("Poll error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestDeviceFlowSurfacesOtherErrors(t *testing.T) {
	api := &fakeDeviceAPI{responses: []func(http.ResponseWriter){oauthErr(http.StatusBadRequest, "invalid_grant")}}
	flow, _ := newTestFlow(t, api)
	code, _ := flow.RequestCode(context.Background(), DeviceClientInfo{})
	_, err := flow.Poll(context.Background(), code)
	if err == nil || !strings.Contains(err.Error(), "invalid_grant description") {
		t.Fatalf("Poll error = %v", err)
	}
}

func TestDeviceFlowExpiresWhenTheCodeDoes(t *testing.T) {
	api := &fakeDeviceAPI{}
	flow, _ := newTestFlow(t, api)
	flow.Sleep = sleepContext
	code := &DeviceCode{DeviceCode: "secret-device-code", ExpiresIn: 1, Interval: 5}
	start := time.Now()
	if _, err := flow.Poll(context.Background(), code); !errors.Is(err, ErrLoginExpired) {
		t.Fatalf("Poll error = %v, want ErrLoginExpired", err)
	}
	if time.Since(start) > 3*time.Second {
		t.Errorf("expiry took %v", time.Since(start))
	}
}

func TestDeviceFlowRevoke(t *testing.T) {
	api := &fakeDeviceAPI{}
	flow, _ := newTestFlow(t, api)
	revoked, err := flow.Revoke(context.Background(), "sk-notte-cli-old")
	if err != nil || !revoked {
		t.Fatalf("Revoke = %v, %v", revoked, err)
	}
	revoked, err = flow.Revoke(context.Background(), "sk-notte-pasted")
	if err != nil || revoked {
		t.Fatalf("Revoke of a pasted key = %v, %v", revoked, err)
	}
}

func TestDeviceFlowExplainsAnOlderAPI(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(server.Close)
	_, err := NewDeviceFlow(server.URL, "test").RequestCode(context.Background(), DeviceClientInfo{})
	if err == nil || !strings.Contains(err.Error(), "does not support `notte login` yet") {
		t.Fatalf("RequestCode error = %v", err)
	}
}

func TestVerificationURL(t *testing.T) {
	const fromAPI = "https://console.notte.cc/oauth/device?user_code=HNVS-GHLG"

	t.Setenv(config.EnvConsoleURL, "")
	if got := VerificationURL(fromAPI); got != fromAPI {
		t.Errorf("without override = %q", got)
	}

	t.Setenv(config.EnvConsoleURL, "http://localhost:3000/")
	if got, want := VerificationURL(fromAPI), "http://localhost:3000/oauth/device?user_code=HNVS-GHLG"; got != want {
		t.Errorf("with override = %q, want %q", got, want)
	}

	t.Setenv(config.EnvConsoleURL, "not a url")
	if got := VerificationURL(fromAPI); got != fromAPI {
		t.Errorf("with an unusable override = %q", got)
	}
}

func TestDeviceFlowFailsFastOnPermanentErrors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		want   string
	}{
		{"proxy 403", http.StatusForbidden, "unexpected status 403"},
		{"missing route", http.StatusNotFound, "does not support `notte login` yet"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			api := &fakeDeviceAPI{responses: []func(http.ResponseWriter){
				func(w http.ResponseWriter) { http.Error(w, "nope", tc.status) },
			}}
			flow, _ := newTestFlow(t, api)
			code, _ := flow.RequestCode(context.Background(), DeviceClientInfo{})
			_, err := flow.Poll(context.Background(), code)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Poll error = %v, want %q", err, tc.want)
			}
			if api.polls != 1 {
				t.Errorf("polled %d times, want 1", api.polls)
			}
		})
	}
}

func TestDeviceFlowRetriesAPlainRateLimit(t *testing.T) {
	api := &fakeDeviceAPI{responses: []func(http.ResponseWriter){
		func(w http.ResponseWriter) { http.Error(w, "busy", http.StatusTooManyRequests) },
		func(w http.ResponseWriter) { http.Error(w, "bad gateway", http.StatusBadGateway) },
		issued,
	}}
	flow, _ := newTestFlow(t, api)
	code, _ := flow.RequestCode(context.Background(), DeviceClientInfo{})
	if _, err := flow.Poll(context.Background(), code); err != nil {
		t.Fatalf("Poll: %v", err)
	}
}

func TestDeviceFlowDoesNotRetryAnUnreadableKey(t *testing.T) {
	api := &fakeDeviceAPI{responses: []func(http.ResponseWriter){
		func(w http.ResponseWriter) { w.WriteHeader(http.StatusOK); _, _ = w.Write([]byte("not json")) },
	}}
	flow, _ := newTestFlow(t, api)
	code, _ := flow.RequestCode(context.Background(), DeviceClientInfo{})
	_, err := flow.Poll(context.Background(), code)
	if err == nil || !strings.Contains(err.Error(), "unexpected token response") {
		t.Fatalf("Poll error = %v", err)
	}
}
