package cmd

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/spf13/cobra"

	"github.com/salmonumbrella/notte-cli/internal/api"
	"github.com/salmonumbrella/notte-cli/internal/auth"
	"github.com/salmonumbrella/notte-cli/internal/testutil"
)

// newTestCommand creates a cobra.Command with a context for testing.
func newTestCommand() *cobra.Command {
	cmd := &cobra.Command{}
	ctx := context.Background()
	cmd.SetContext(ctx)
	return cmd
}

func TestRunAPICommand_Success(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	cmd := newTestCommand()
	outputFormat = "json"

	type TestResult struct {
		Status string `json:"status"`
	}

	// Capture stdout to verify output
	stdout, _ := testutil.CaptureOutput(func() {
		err := RunAPICommand(cmd, func(ctx context.Context, client *api.ClientWithResponses) (*TestResult, *http.Response, []byte, error) {
			return &TestResult{Status: "ok"}, &http.Response{
				StatusCode: 200,
			}, nil, nil
		})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunAPICommand_APIError(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	cmd := newTestCommand()
	outputFormat = "json"

	type TestResult struct {
		Status string `json:"status"`
	}

	err := RunAPICommand(cmd, func(ctx context.Context, client *api.ClientWithResponses) (*TestResult, *http.Response, []byte, error) {
		return nil, nil, nil, errors.New("network error")
	})

	if err == nil {
		t.Error("expected error, got nil")
	}
	if err.Error() != "API request failed: network error" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRunAPICommand_HTTPError(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	cmd := newTestCommand()
	outputFormat = "json"

	type TestResult struct {
		Status string `json:"status"`
	}

	err := RunAPICommand(cmd, func(ctx context.Context, client *api.ClientWithResponses) (*TestResult, *http.Response, []byte, error) {
		body := []byte(`{"error": "server error"}`)
		return nil, &http.Response{
			StatusCode: 500,
		}, body, nil
	})

	if err == nil {
		t.Error("expected error for 500 response, got nil")
	}
}

func TestRunAPICommand_NoAPIKey(t *testing.T) {
	// Ensure clean environment (clears env vars)
	env := testutil.SetupTestEnv(t)
	// Use mock keyring to prevent reading from real system keyring
	auth.SetKeyring(env.MockStore)
	defer auth.ResetKeyring()

	cmd := newTestCommand()

	type TestResult struct {
		Status string `json:"status"`
	}

	err := RunAPICommand(cmd, func(ctx context.Context, client *api.ClientWithResponses) (*TestResult, *http.Response, []byte, error) {
		t.Error("API function should not be called without API key")
		return nil, nil, nil, nil
	})

	if err == nil {
		t.Error("expected error when no API key is set")
	}
}
