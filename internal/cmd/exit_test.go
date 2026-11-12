package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/nottelabs/notte-cli/internal/auth"
	apierrors "github.com/nottelabs/notte-cli/internal/errors"
)

func TestClassifyError(t *testing.T) {
	for _, tc := range []struct {
		err  error
		code string
		exit int
	}{
		{errors.New("something broke"), "error", ExitFailure},
		{errUsage("--count must be between 1 and 20"), "usage", ExitUsage},
		{errors.New(`unknown command "x" for "notte"`), "usage", ExitUsage},
		{errors.New(`required flag(s) "function-id" not set`), "usage", ExitUsage},
		{errors.New("--file is required"), "usage", ExitUsage},
		{errors.New("--cron cannot be empty"), "usage", ExitUsage},
		{errors.New("profile requires --profile-id to be set"), "usage", ExitUsage},
		{fmt.Errorf("failed to parse --vars: %w", errors.New("bad json")), "usage", ExitUsage},
		{fmt.Errorf("invalid browser-type: %w", errors.New("bad")), "usage", ExitUsage},
		{errors.New("accepts 1 arg(s), received 0"), "usage", ExitUsage},
		{auth.ErrNoAPIKey, "unauthorized", ExitAuth},
		{fmt.Errorf("wrapped: %w", &apierrors.AuthError{Reason: "invalid", StatusCode: 401}), "unauthorized", ExitAuth},
		{&apierrors.APIError{StatusCode: 403, Message: "forbidden"}, "unauthorized", ExitAuth},
		{&apierrors.APIError{StatusCode: 404, Message: "no such session"}, "not_found", ExitNotFound},
		{&apierrors.APIError{StatusCode: 500, Message: "boom"}, "api_error", ExitFailure},
		{&apierrors.RateLimitError{RetryAfter: time.Minute}, "rate_limited", ExitRateLimited},
		{&confirmationError{action: "delete vault v1"}, "confirmation_required", ExitConfirm},
		{&runFailedError{runID: "run_1"}, "run_failed", ExitRunFailed},
	} {
		code, exit := classifyError(tc.err)
		if code != tc.code || exit != tc.exit {
			t.Errorf("%v: got (%q, %d), want (%q, %d)", tc.err, code, exit, tc.code, tc.exit)
		}
	}
}

func TestDefaultOutputFormat(t *testing.T) {
	for value, want := range map[string]string{"": "text", "json": "json", "text": "text", "yaml": "text"} {
		t.Setenv("NOTTE_OUTPUT", value)
		if got := defaultOutputFormat(); got != want {
			t.Errorf("NOTTE_OUTPUT=%q: got %q, want %q", value, got, want)
		}
	}
}

// Without a terminal a delete must not wait for an answer or pass as done.
func TestConfirmWithoutTerminal(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	_ = w.Close()
	origStdin, origSkip := os.Stdin, skipConfirmation
	os.Stdin, skipConfirmation = r, false
	t.Cleanup(func() { os.Stdin, skipConfirmation = origStdin, origSkip; _ = r.Close() })
	t.Setenv("NOTTE_FORCE_INTERACTIVE", "")

	ok, err := ConfirmAction("vault", "v1")
	var confirm *confirmationError
	if ok || !errors.As(err, &confirm) {
		t.Errorf("delete: got (%v, %v), want a confirmation error", ok, err)
	}
	// Stopping is the default answer of its prompt, so it goes ahead.
	if ok, err := ConfirmStop("session", "s1"); !ok || err != nil {
		t.Errorf("stop: got (%v, %v), want (true, nil)", ok, err)
	}
	skipConfirmation = true
	if ok, err := ConfirmAction("vault", "v1"); !ok || err != nil {
		t.Errorf("delete with --yes: got (%v, %v)", ok, err)
	}
}

// Bad arguments must exit with the usage code through the real entry point,
// whichever command found them.
func TestExecute_BadArgumentsExitUsage(t *testing.T) {
	if args := os.Getenv("NOTTE_EXECUTE_ARGS"); args != "" {
		t.Setenv("NOTTE_API_KEY", "test-key")
		rootCmd.SetArgs(strings.Fields(args))
		Execute()
		return
	}
	for _, args := range []string{
		"sessions start --count 0",
		"sessions list --page 0",
		"sessions stop --all --session-id s1",
		"sessions stop --all s1",
		"sessions stop s1 --session-id s2",
		"sessions stop s1 s2",
		"sessions status s1 --session-id s2",
		"search --depth nope hello",
		"functions create",
	} {
		cmd := exec.Command(os.Args[0], "-test.run=TestExecute_BadArgumentsExitUsage")
		cmd.Env = append(os.Environ(), "NOTTE_EXECUTE_ARGS="+args, "NOTTE_CONFIG_DIR="+t.TempDir())
		err := cmd.Run()
		exitErr, ok := err.(*exec.ExitError)
		if !ok || exitErr.ExitCode() != ExitUsage {
			t.Errorf("notte %s: got %v, want exit code %d", args, err, ExitUsage)
		}
	}
}
