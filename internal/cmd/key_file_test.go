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
	"github.com/nottelabs/notte-cli/internal/output"
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

// printCommandError prints err the way Execute does and returns stderr with
// the exit code.
func printCommandError(t *testing.T, err error) (string, int) {
	t.Helper()
	code, exit := classifyError(err)
	_, stderr := testutil.CaptureOutput(func() {
		switch formatter := GetFormatter().(type) {
		case *output.TextFormatter:
			formatter.PrintError(err)
		case *output.JSONFormatter:
			formatter.ErrorCode = code
			formatter.PrintError(err)
		}
	})
	return stderr, exit
}

func TestLoginAndLogout_FailWithKeyFile(t *testing.T) {
	for _, tc := range []struct {
		name      string
		run       func(*cobra.Command, []string) error
		withToken bool
	}{
		{"login", runAuthLogin, false},
		{"login --with-token", runAuthLogin, true},
		{"logout", runAuthLogout, false},
	} {
		for _, format := range []string{"text", "json"} {
			t.Run(tc.name+"/"+format, func(t *testing.T) {
				env := testutil.SetupTestEnv(t)
				path := useKeyFile(t, env, "nat_token")
				k := &recordingKeyring{key: "sk-stored"}
				auth.SetKeyring(k)
				t.Cleanup(auth.ResetKeyring)
				withOutputFormat(t, format)

				// Neither command may reach an API: there is none here.
				env.SetEnv("NOTTE_API_URL", "https://127.0.0.1:1")
				origWithToken := loginWithToken
				loginWithToken = tc.withToken
				t.Cleanup(func() { loginWithToken = origWithToken })
				origStdin := stdinReader
				stdinReader = strings.NewReader("sk-from-stdin\n")
				t.Cleanup(func() { stdinReader = origStdin })

				cmd := &cobra.Command{}
				cmd.SetContext(context.Background())
				var err error
				stdout, _ := testutil.CaptureOutput(func() { err = tc.run(cmd, nil) })
				if err == nil {
					t.Fatalf("%s succeeded with %s set; it must fail", tc.name, auth.EnvAPIKeyFile)
				}
				if stdout != "" {
					t.Errorf("stdout %q, want nothing", stdout)
				}
				if k.sets != 0 || k.deletes != 0 || k.key != "sk-stored" {
					t.Fatalf("stored key changed: sets=%d deletes=%d key=%q", k.sets, k.deletes, k.key)
				}

				stderr, exit := printCommandError(t, err)
				if exit != ExitFailure {
					t.Errorf("exit %d, want %d", exit, ExitFailure)
				}
				for _, want := range []string{auth.EnvAPIKeyFile, path} {
					if !strings.Contains(stderr, want) {
						t.Errorf("stderr %q should name %q", stderr, want)
					}
				}
				if strings.Contains(stderr, "nat_token") {
					t.Fatalf("stderr %q prints the credential", stderr)
				}
				if format == "json" {
					var got map[string]any
					if err := json.Unmarshal([]byte(stderr), &got); err != nil {
						t.Fatalf("stderr %q is not JSON: %v", stderr, err)
					}
					if got["code"] != "error" || !strings.Contains(got["error"].(string), path) {
						t.Fatalf("got %v, want code error and a message naming %s", got, path)
					}
				}
			})
		}
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

// errorServer answers every request with status, a FastAPI-style detail (an
// empty body when detail is empty, or {} for a 200) and, when class is set, an
// X-Error-Class header. It records the bearer credential it received.
func errorServer(t *testing.T, status int, class, detail string) (*httptest.Server, *string) {
	t.Helper()
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		if class != "" {
			w.Header().Set("X-Error-Class", class)
		}
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
	srv, got := errorServer(t, http.StatusOK, "", "")
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
		expired   = "access token expired at 2026-11-12T10:00:00Z; mint a new one"
		revoked   = "invalid or revoked access token"
		route     = "access tokens are not accepted on this route; use an API key"
		planLimit = "Your plan does not include this feature"
	)
	const (
		classExpired    = "NotteApiAccessTokenExpiredError"
		classInvalid    = "NotteApiAccessTokenInvalidError"
		classNotAllowed = "NotteApiAccessTokenNotAllowedError"
	)
	for _, tc := range []struct {
		name       string
		credential string
		status     int
		class      string
		detail     string
		wantReason string
		wantHint   string // the hint, before the key file suffix
		fileSuffix bool   // the hint names NOTTE_API_KEY_FILE
	}{
		{"expired", "nat_abc", 401, classExpired, expired, "expired", "the access token has expired; mint a new one", true},
		{"expired without a detail", "nat_abc", 401, classExpired, "", "expired", "the access token has expired; mint a new one", true},
		{"invalid", "nat_abc", 401, classInvalid, revoked, "invalid", "the access token is unknown or revoked; mint a new one", true},
		{"not allowed", "nat_abc", 403, classNotAllowed, route, "forbidden", "this command needs an API key; access tokens are not accepted on its route", false},
		// Without one of the three classes the API's answer is shown as is,
		// whatever its wording and whichever credential was sent.
		{"plan limit 403 on a token", "nat_abc", 403, "NottePlanLimitError", planLimit, "forbidden", "", false},
		{"route refusal without a class", "nat_abc", 403, "", route, "forbidden", "", false},
		{"expired without a class", "nat_abc", 401, "", expired, "invalid", "", false},
		{"401 without a class or detail", "nat_abc", 401, "", "", "invalid", "", false},
		{"API key 401", "sk-key", 401, "NotteApiInvalidApiKeyError", "invalid API key", "invalid", "", false},
		{"API key 403", "sk-key", 403, "", "forbidden", "forbidden", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := testutil.SetupTestEnv(t)
			srv, _ := errorServer(t, tc.status, tc.class, tc.detail)
			env.SetEnv("NOTTE_API_URL", srv.URL)
			path := useKeyFile(t, env, tc.credential)

			err := callUsage(t)
			var authErr *apierrors.AuthError
			if !errors.As(err, &authErr) {
				t.Fatalf("error %v (%T) is not an AuthError", err, err)
			}
			if authErr.StatusCode != tc.status || authErr.Message != tc.detail {
				t.Fatalf("got %d %q, want %d and the API's detail %q unchanged", authErr.StatusCode, authErr.Message, tc.status, tc.detail)
			}
			if authErr.Reason != tc.wantReason {
				t.Errorf("reason %q, want %q", authErr.Reason, tc.wantReason)
			}
			wantHint := tc.wantHint
			if tc.fileSuffix {
				wantHint += " and write it to NOTTE_API_KEY_FILE (" + path + ")"
			}
			if authErr.Hint != wantHint {
				t.Errorf("hint %q, want %q", authErr.Hint, wantHint)
			}
			if code, exit := classifyError(err); code != "unauthorized" || exit != ExitAuth {
				t.Errorf("classifyError = %s, %d, want unauthorized, %d", code, exit, ExitAuth)
			}
		})
	}
}

func TestAccessTokenErrors_HintWithoutKeyFile(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	srv, _ := errorServer(t, 401, "NotteApiAccessTokenExpiredError", "access token expired")
	env.SetEnv("NOTTE_API_URL", srv.URL)
	env.SetEnv("NOTTE_API_KEY", "nat_abc")

	var authErr *apierrors.AuthError
	if err := callUsage(t); !errors.As(err, &authErr) {
		t.Fatalf("error %v is not an AuthError", err)
	}
	if want := "the access token has expired; mint a new one"; authErr.Hint != want {
		t.Fatalf("hint %q, want %q", authErr.Hint, want)
	}
}

func TestAccessTokenErrors_HintIsPrinted(t *testing.T) {
	err := &apierrors.AuthError{Reason: "expired", Message: "access token expired", StatusCode: 401, Hint: "mint a new one"}
	for _, format := range []string{"text", "json"} {
		t.Run(format, func(t *testing.T) {
			withOutputFormat(t, format)
			stderr, exit := printCommandError(t, err)
			if exit != ExitAuth {
				t.Errorf("exit %d, want %d", exit, ExitAuth)
			}
			if format == "text" {
				if !strings.Contains(stderr, "expired - access token expired") || !strings.Contains(stderr, "Hint: mint a new one") {
					t.Fatalf("stderr %q should hold the message and the hint", stderr)
				}
				return
			}
			var got map[string]any
			if err := json.Unmarshal([]byte(stderr), &got); err != nil {
				t.Fatalf("stderr %q is not JSON: %v", stderr, err)
			}
			if got["error"] != "expired" || got["message"] != "access token expired" || got["hint"] != "mint a new one" || got["code"] != "unauthorized" {
				t.Fatalf("got %v", got)
			}
		})
	}
}

func TestAccessTokenErrors_NoClassMatchesParseAPIError(t *testing.T) {
	for _, status := range []int{401, 403, 404} {
		body := []byte(`{"detail": "access tokens are not accepted on this route; use an API key"}`)
		resp := &http.Response{StatusCode: status, Header: http.Header{}, Request: &http.Request{Header: http.Header{"Authorization": {"Bearer nat_abc"}}}}
		got := HandleAPIResponse(resp, body)
		want := apierrors.ParseAPIError(resp, body)
		if got.Error() != want.Error() {
			t.Errorf("%d: got %q, want the parsed error unchanged %q", status, got, want)
		}
	}
}
