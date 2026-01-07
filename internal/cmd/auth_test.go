package cmd

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/salmonumbrella/notte-cli/internal/auth"
	"github.com/salmonumbrella/notte-cli/internal/testutil"
)

type stubKeyring struct {
	deleted bool
}

func (s *stubKeyring) Get(_ string) (string, error) { return "", errors.New("not found") }
func (s *stubKeyring) Set(_, _ string) error        { return nil }
func (s *stubKeyring) Delete(_ string) error {
	s.deleted = true
	return nil
}

func TestRunAuthStatus(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key-1234567890")

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runAuthStatus(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(stdout, "Authenticated") {
		t.Fatalf("expected auth output, got %q", stdout)
	}
}

func TestRunAuthStatusShortKey(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "short")

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runAuthStatus(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(stdout, "****") {
		t.Fatalf("expected masked key, got %q", stdout)
	}
}

func TestRunAuthStatusNotAuthenticated(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("HOME", t.TempDir())

	k := &stubKeyring{}
	auth.SetKeyring(k)
	t.Cleanup(func() { auth.ResetKeyring() })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runAuthStatus(cmd, nil)
	if err == nil {
		t.Fatal("expected error when not authenticated")
	}
	if !strings.Contains(err.Error(), "not authenticated") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunAuthLogout(t *testing.T) {
	k := &stubKeyring{}
	auth.SetKeyring(k)
	t.Cleanup(func() { auth.ResetKeyring() })

	origFormat := outputFormat
	outputFormat = "text"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runAuthLogout(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !k.deleted {
		t.Fatal("expected keyring delete")
	}
	if !strings.Contains(stdout, "API key removed") {
		t.Fatalf("expected logout message, got %q", stdout)
	}
}

func TestRunAuthLogin_ContextCanceled(t *testing.T) {
	origFormat := outputFormat
	outputFormat = "text"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cmd.SetContext(ctx)

	stdout, _ := testutil.CaptureOutput(func() {
		err := runAuthLogin(cmd, nil)
		if err == nil {
			t.Fatal("expected error for canceled context")
		}
		if !strings.Contains(err.Error(), "context") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Fatal("expected informational output")
	}
}
