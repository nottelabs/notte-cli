package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/api"
	"github.com/nottelabs/notte-cli/internal/auth"
	"github.com/nottelabs/notte-cli/internal/config"
	apierrors "github.com/nottelabs/notte-cli/internal/errors"
	"github.com/nottelabs/notte-cli/internal/testutil"
)

// recordingKeyring records writes and deletes; it holds one key.
type recordingKeyring struct {
	key     string
	sets    int
	deletes int
}

func (k *recordingKeyring) Get(_ string) (string, error) {
	if k.key == "" {
		return "", os.ErrNotExist
	}
	return k.key, nil
}

func (k *recordingKeyring) Set(_, v string) error {
	k.sets++
	k.key = v
	return nil
}

func (k *recordingKeyring) Delete(_ string) error {
	k.deletes++
	k.key = ""
	return nil
}

func useKeyFile(t *testing.T, env *testutil.TestEnv, content string) string {
	t.Helper()
	path := filepath.Join(env.TempDir, "token")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	env.SetEnv(auth.EnvAPIKeyFile, path)
	return path
}

func withOutputFormat(t *testing.T, format string) {
	t.Helper()
	orig := outputFormat
	outputFormat = format
	t.Cleanup(func() { outputFormat = orig })
}

func TestLoginAndLogout_LeaveStoredKeyAloneWithKeyFile(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func(*cobra.Command, []string) error
	}{
		{"login", runAuthLogin},
		{"logout", runAuthLogout},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := testutil.SetupTestEnv(t)
			path := useKeyFile(t, env, "nat_token")
			k := &recordingKeyring{key: "sk-stored"}
			auth.SetKeyring(k)
			t.Cleanup(auth.ResetKeyring)
			withOutputFormat(t, "text")

			// Neither command may reach an API: there is none here.
			env.SetEnv("NOTTE_API_URL", "https://127.0.0.1:1")
			origStdin := stdinReader
			stdinReader = strings.NewReader("sk-from-stdin\n")
			t.Cleanup(func() { stdinReader = origStdin })

			cmd := &cobra.Command{}
			cmd.SetContext(context.Background())
			var err error
			stdout, _ := testutil.CaptureOutput(func() { err = tc.run(cmd, nil) })
			if err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if k.sets != 0 || k.deletes != 0 || k.key != "sk-stored" {
				t.Fatalf("stored key changed: sets=%d deletes=%d key=%q", k.sets, k.deletes, k.key)
			}
			for _, want := range []string{auth.EnvAPIKeyFile, path} {
				if !strings.Contains(stdout, want) {
					t.Errorf("output %q should name %q", stdout, want)
				}
			}
		})
	}
}

func TestLoginWithToken_LeavesStoredKeyAloneWithKeyFile(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	useKeyFile(t, env, "nat_token")
	k := &recordingKeyring{}
	auth.SetKeyring(k)
	t.Cleanup(auth.ResetKeyring)
	withOutputFormat(t, "json")

	origWithToken := loginWithToken
	loginWithToken = true
	t.Cleanup(func() { loginWithToken = origWithToken })
	origStdin := stdinReader
	stdinReader = strings.NewReader("sk-from-stdin\n")
	t.Cleanup(func() { stdinReader = origStdin })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	var err error
	stdout, _ := testutil.CaptureOutput(func() { err = runAuthLogin(cmd, nil) })
	if err != nil {
		t.Fatal(err)
	}
	if k.sets != 0 {
		t.Fatalf("login --with-token stored a key while %s is set", auth.EnvAPIKeyFile)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("output %q is not JSON: %v", stdout, err)
	}
	if got["source"] != "file" || got["changed"] != false {
		t.Fatalf("got %v, want source file and changed false", got)
	}
}

func TestRunAuthStatus_NamesKeyFile(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	path := useKeyFile(t, env, "nat_0123456789abcdef\n")
	env.SetEnv("NOTTE_API_KEY", "sk-env-key-0123456789")

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	withOutputFormat(t, "text")
	stdout, _ := testutil.CaptureOutput(func() {
		if err := runAuthStatus(cmd, nil); err != nil {
			t.Fatal(err)
		}
	})
	for _, want := range []string{auth.EnvAPIKeyFile, path, "nat_0123"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("status %q should contain %q", stdout, want)
		}
	}

	withOutputFormat(t, "json")
	stdout, _ = testutil.CaptureOutput(func() {
		if err := runAuthStatus(cmd, nil); err != nil {
			t.Fatal(err)
		}
	})
	var got map[string]any
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("output %q is not JSON: %v", stdout, err)
	}
	if got["source"] != "file" || got["key_file"] != path {
		t.Fatalf("got %v, want source file and key_file %s", got, path)
	}
}

func TestRunAuthStatus_KeyFileErrorDoesNotFallBack(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	path := useKeyFile(t, env, "")
	env.SetEnv("NOTTE_API_KEY", "sk-env-key-0123456789")

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	err := runAuthStatus(cmd, nil)
	if err == nil {
		t.Fatal("expected an error for an empty key file")
	}
	if !strings.Contains(err.Error(), auth.EnvAPIKeyFile) || !strings.Contains(err.Error(), path) {
		t.Fatalf("error %q should name %s and %s", err, auth.EnvAPIKeyFile, path)
	}
	if code, exit := classifyError(err); code != "unauthorized" || exit != ExitAuth {
		t.Fatalf("classifyError = %s, %d, want unauthorized, %d", code, exit, ExitAuth)
	}
}

// errorServer answers every request with status and a FastAPI-style detail
// (an empty body when detail is empty, or {} for a 200),
// and records the bearer credential it received.
func errorServer(t *testing.T, status int, detail string) (*httptest.Server, *string) {
	t.Helper()
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		switch {
		case detail != "":
			_ = json.NewEncoder(w).Encode(map[string]string{"detail": detail})
		case status == http.StatusOK:
			_, _ = w.Write([]byte(`{}`))
		}
		// An error without a detail has an empty body.
	}))
	t.Cleanup(srv.Close)
	return srv, &got
}

// callUsage calls /usage through GetClient and HandleAPIResponse, as commands do.
func callUsage(t *testing.T) error {
	t.Helper()
	client, err := GetClient()
	if err != nil {
		t.Fatalf("GetClient: %v", err)
	}
	resp, err := client.Client().GetUsageWithResponse(context.Background(), &api.GetUsageParams{})
	if err != nil {
		t.Fatal(err)
	}
	return HandleAPIResponse(resp.HTTPResponse, resp.Body)
}

func TestKeyFile_ReadPerInvocationAndSent(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	auth.SetKeyring(env.MockStore)
	t.Cleanup(auth.ResetKeyring)
	config.SetTestConfigDir(env.TempDir)
	t.Cleanup(func() { config.SetTestConfigDir("") })
	srv, got := errorServer(t, http.StatusOK, "")
	env.SetEnv("NOTTE_API_URL", srv.URL)
	env.SetEnv("NOTTE_API_KEY", "sk-env-key")
	path := useKeyFile(t, env, "nat_first\n")

	if err := callUsage(t); err != nil {
		t.Fatal(err)
	}
	if *got != "Bearer nat_first" {
		t.Fatalf("sent %q, want the key file's token", *got)
	}
	if err := os.WriteFile(path, []byte("nat_second"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := callUsage(t); err != nil {
		t.Fatal(err)
	}
	if *got != "Bearer nat_second" {
		t.Fatalf("sent %q after rotation, want nat_second", *got)
	}
}

func TestAccessTokenErrors(t *testing.T) {
	const (
		expired = "access token expired at 2026-11-12T10:00:00Z; mint a new one"
		revoked = "invalid or revoked access token"
		route   = "access tokens are not accepted on this route; use an API key"
	)
	for _, tc := range []struct {
		name       string
		credential string
		status     int
		detail     string
		want       string // the whole AuthError message
	}{
		{"expired token keeps the API message", "nat_abc", 401, expired, expired},
		{"revoked token says to mint", "nat_abc", 401, revoked, revoked + "; the access token is expired or revoked, mint a new one"},
		{"401 without a message", "nat_abc", 401, "", "the access token is expired or revoked; mint a new one"},
		{"route refuses tokens", "nat_abc", 403, route, "this command needs an API key: access tokens are not accepted on this route"},
		{"403 without a message", "nat_abc", 403, "", "this command needs an API key: access tokens are not accepted on this route"},
		{"403 for another reason is kept", "nat_abc", 403, "plan limit reached", "this command needs an API key: access tokens are not accepted on this route (plan limit reached)"},
		{"API key 401 is unchanged", "sk-key", 401, "invalid API key", "invalid API key"},
		{"API key 403 is unchanged", "sk-key", 403, "forbidden", "forbidden"},
		{"API key 401 without a message", "sk-key", 401, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := testutil.SetupTestEnv(t)
			srv, _ := errorServer(t, tc.status, tc.detail)
			env.SetEnv("NOTTE_API_URL", srv.URL)
			useKeyFile(t, env, tc.credential)

			err := callUsage(t)
			var authErr *apierrors.AuthError
			if !errors.As(err, &authErr) {
				t.Fatalf("error %v (%T) is not an AuthError", err, err)
			}
			if authErr.StatusCode != tc.status || authErr.Message != tc.want {
				t.Fatalf("got %d %q, want %d %q", authErr.StatusCode, authErr.Message, tc.status, tc.want)
			}
			wantReason := map[int]string{401: "invalid", 403: "forbidden"}[tc.status]
			if authErr.Reason != wantReason {
				t.Errorf("reason %q, want %q", authErr.Reason, wantReason)
			}
			if code, exit := classifyError(err); code != "unauthorized" || exit != ExitAuth {
				t.Errorf("classifyError = %s, %d, want unauthorized, %d", code, exit, ExitAuth)
			}
		})
	}
}

func TestAccessTokenErrors_APIKeyMatchesParseAPIError(t *testing.T) {
	for _, status := range []int{401, 403} {
		body := []byte(`{"detail": "nope"}`)
		resp := &http.Response{StatusCode: status, Request: &http.Request{Header: http.Header{"Authorization": {"Bearer sk-key"}}}}
		got := HandleAPIResponse(resp, body)
		want := apierrors.ParseAPIError(resp, body)
		if got.Error() != want.Error() {
			t.Errorf("%d: got %q, want today's %q", status, got, want)
		}
	}
}

func TestAccessTokenErrors_OtherStatusesUnchanged(t *testing.T) {
	body := []byte(`{"detail": "not found"}`)
	resp := &http.Response{StatusCode: 404, Request: &http.Request{Header: http.Header{"Authorization": {"Bearer nat_abc"}}}}
	got := HandleAPIResponse(resp, body)
	want := apierrors.ParseAPIError(resp, body)
	if got.Error() != want.Error() {
		t.Errorf("got %q, want %q", got, want)
	}
}
