package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nottelabs/notte-cli/internal/api"
	"github.com/nottelabs/notte-cli/internal/config"
)

// The retries the client does by default are correct in production and pure
// wall-clock here, where every response is deterministic.
func noRetry() api.NotteClientOption {
	return api.WithRetryConfig(&api.RetryConfig{
		MaxRetries:     0,
		InitialBackoff: time.Millisecond,
		MaxBackoff:     time.Millisecond,
	})
}

// apiStub stands in for the Notte API and records what reached it.
type apiStub struct {
	server *httptest.Server
	path   string
	auth   string
	hits   int
}

func newAPIStub(t *testing.T, status int, body string) *apiStub {
	t.Helper()
	stub := &apiStub{}
	stub.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stub.hits++
		stub.path = r.URL.Path
		stub.auth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(stub.server.Close)
	return stub
}

func TestValidateAPIKeyUsesConfiguredAPI(t *testing.T) {
	// The bug this covers: validation went to the compiled-in default API
	// regardless of NOTTE_API_URL, so `auth login` against one environment was
	// checked against another.
	stub := newAPIStub(t, http.StatusOK, `{}`)
	t.Setenv(config.EnvAPIURL, stub.server.URL)

	if err := ValidateAPIKey(context.Background(), "sk-notte-test"); err != nil {
		t.Fatalf("ValidateAPIKey() = %v, want nil", err)
	}

	if stub.hits != 1 {
		t.Fatalf("configured API received %d requests, want 1", stub.hits)
	}
	// The key has to actually be presented; a request without it proves nothing
	// about the key, which is how validating against /health passed for any
	// string at all.
	if want := "Bearer sk-notte-test"; stub.auth != want {
		t.Errorf("Authorization header = %q, want %q", stub.auth, want)
	}
}

func TestValidateAPIKeyRequiresAnAuthenticatedEndpoint(t *testing.T) {
	// The endpoint is pinned rather than merely checked against /health: any
	// other unauthenticated endpoint would reintroduce the bug just as well,
	// and a negative assertion against one path would not notice.
	//
	// If this ever needs to change, the replacement has to be an endpoint that
	// answers 401 to an unknown key. /health does not - it answers 200 with no
	// credential at all, which is what let any string validate.
	const authenticatedPath = "/usage"

	stub := newAPIStub(t, http.StatusOK, `{}`)
	t.Setenv(config.EnvAPIURL, stub.server.URL)

	if err := ValidateAPIKey(context.Background(), "sk-notte-test"); err != nil {
		t.Fatalf("ValidateAPIKey() = %v, want nil", err)
	}

	if stub.path != authenticatedPath {
		t.Errorf("validated against %q, want %q", stub.path, authenticatedPath)
	}
}

func TestValidateAPIKeyRejectsBadKey(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		stub := newAPIStub(t, status, `{"detail":"nope"}`)
		t.Setenv(config.EnvAPIURL, stub.server.URL)

		err := ValidateAPIKey(context.Background(), "sk-notte-bad", noRetry())
		if !errors.Is(err, ErrInvalidAPIKey) {
			t.Errorf("status %d: ValidateAPIKey() = %v, want ErrInvalidAPIKey", status, err)
		}
	}
}

func TestValidateAPIKeyDoesNotBlameTheKeyForServerErrors(t *testing.T) {
	// A 500 must not tell the user their key is invalid and send them off to
	// rotate a key that was fine.
	stub := newAPIStub(t, http.StatusInternalServerError, `{}`)
	t.Setenv(config.EnvAPIURL, stub.server.URL)

	err := ValidateAPIKey(context.Background(), "sk-notte-test", noRetry())
	if err == nil {
		t.Fatal("ValidateAPIKey() = nil, want an error")
	}
	if errors.Is(err, ErrInvalidAPIKey) {
		t.Errorf("ValidateAPIKey() = %v, want a non-credential error", err)
	}
}

func TestValidateAPIKeyReportsUnreachableAPI(t *testing.T) {
	stub := newAPIStub(t, http.StatusOK, `{}`)
	url := stub.server.URL
	stub.server.Close() // nothing is listening now
	t.Setenv(config.EnvAPIURL, url)

	err := ValidateAPIKey(context.Background(), "sk-notte-test", noRetry())
	if err == nil {
		t.Fatal("ValidateAPIKey() = nil, want an error")
	}
	if errors.Is(err, ErrInvalidAPIKey) {
		t.Errorf("ValidateAPIKey() = %v, want a connection error, not a credential one", err)
	}
}

func TestValidateAPIKeyRejectsEmptyKey(t *testing.T) {
	stub := newAPIStub(t, http.StatusOK, `{}`)
	t.Setenv(config.EnvAPIURL, stub.server.URL)

	err := ValidateAPIKey(context.Background(), "")
	if !errors.Is(err, ErrInvalidAPIKey) {
		t.Errorf("ValidateAPIKey(\"\") = %v, want ErrInvalidAPIKey", err)
	}
	if stub.hits != 0 {
		t.Errorf("empty key still reached the API %d times", stub.hits)
	}
}
