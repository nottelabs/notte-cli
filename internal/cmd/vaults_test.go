package cmd

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/testutil"
)

const vaultIDTest = "vault_123"

func setupVaultTest(t *testing.T) *testutil.MockServer {
	t.Helper()
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	t.Cleanup(func() { server.Close() })
	env.SetEnv("NOTTE_API_URL", server.URL())

	origVaultID := vaultID
	vaultID = vaultIDTest
	t.Cleanup(func() { vaultID = origVaultID })

	return server
}

func TestRunVaultsList_Success(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	server.AddResponse("/vaults", 200, `{"items":[{"vault_id":"vault_1","name":"Test Vault","created_at":"2020-01-01T00:00:00Z"}]}`)

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runVaultsList(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunVaultsList_Empty(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	server.AddResponse("/vaults", 200, `{"items":[]}`)

	origFormat := outputFormat
	outputFormat = "text"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runVaultsList(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(stdout, "No vaults found.") {
		t.Errorf("expected empty message, got %q", stdout)
	}
}

func TestRunVaultsCreate_Success(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	server.AddResponse("/vaults/create", 200, `{"vault_id":"vault_1","name":"Test Vault","created_at":"2020-01-01T00:00:00Z"}`)

	origName := vaultsCreateName
	t.Cleanup(func() { vaultsCreateName = origName })
	vaultsCreateName = "Test Vault"

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runVaultsCreate(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunVaultsCreate_NoName(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	server.AddResponse("/vaults/create", 200, `{"vault_id":"vault_2","name":"","created_at":"2020-01-01T00:00:00Z"}`)

	origName := vaultsCreateName
	t.Cleanup(func() { vaultsCreateName = origName })
	vaultsCreateName = ""

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runVaultsCreate(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunVaultUpdate(t *testing.T) {
	server := setupVaultTest(t)
	server.AddResponse("/vaults/"+vaultIDTest, 200, `{"vault_id":"`+vaultIDTest+`","name":"Vault","created_at":"2020-01-01T00:00:00Z"}`)

	origName := vaultUpdateName
	vaultUpdateName = "New Name"
	t.Cleanup(func() { vaultUpdateName = origName })

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runVaultUpdate(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunVaultDelete(t *testing.T) {
	server := setupVaultTest(t)
	server.AddResponse("/vaults/"+vaultIDTest, 200, `{"status":"deleted","message":"deleted"}`)

	SetSkipConfirmation(true)
	t.Cleanup(func() { SetSkipConfirmation(false) })

	origFormat := outputFormat
	outputFormat = "text"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runVaultDelete(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(stdout, "deleted") {
		t.Errorf("expected delete message, got %q", stdout)
	}
}

func TestRunVaultDeleteCancelled(t *testing.T) {
	_ = setupVaultTest(t)

	origSkip := skipConfirmation
	t.Cleanup(func() { skipConfirmation = origSkip })
	skipConfirmation = false

	origFormat := outputFormat
	outputFormat = "text"
	t.Cleanup(func() { outputFormat = origFormat })

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	_, _ = w.WriteString("n\n")
	_ = w.Close()

	origStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = origStdin
		_ = r.Close()
	})

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runVaultDelete(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(stdout, "Cancelled.") {
		t.Errorf("expected cancel message, got %q", stdout)
	}
}

func TestRunVaultCredentialsList(t *testing.T) {
	server := setupVaultTest(t)
	server.AddResponse("/vaults/"+vaultIDTest, 200, `{"credentials":[{"url":"https://example.com","email":"test@example.com"}]}`)

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runVaultCredentialsList(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunVaultCredentialsList_Empty(t *testing.T) {
	server := setupVaultTest(t)
	server.AddResponse("/vaults/"+vaultIDTest, 200, `{"credentials":[]}`)

	origFormat := outputFormat
	outputFormat = "text"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runVaultCredentialsList(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(stdout, "No credentials found.") {
		t.Errorf("expected empty message, got %q", stdout)
	}
}

func TestRunVaultCredentialsAdd(t *testing.T) {
	server := setupVaultTest(t)
	server.AddResponse("/vaults/"+vaultIDTest+"/credentials", 200, `{"status":"ok"}`)

	origURL := vaultCredentialsAddURL
	origEmail := vaultCredentialsAddEmail
	origUser := vaultCredentialsAddUsername
	origPass := vaultCredentialsAddPassword
	origMFA := vaultCredentialsAddMFA
	t.Cleanup(func() {
		vaultCredentialsAddURL = origURL
		vaultCredentialsAddEmail = origEmail
		vaultCredentialsAddUsername = origUser
		vaultCredentialsAddPassword = origPass
		vaultCredentialsAddMFA = origMFA
	})

	vaultCredentialsAddURL = "https://example.com"
	vaultCredentialsAddEmail = "test@example.com"
	vaultCredentialsAddUsername = "user"
	vaultCredentialsAddPassword = "pass"
	vaultCredentialsAddMFA = "mfa"

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runVaultCredentialsAdd(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunVaultCredentialsAdd_InvalidURL(t *testing.T) {
	_ = setupVaultTest(t)

	origURL := vaultCredentialsAddURL
	origEmail := vaultCredentialsAddEmail
	origUser := vaultCredentialsAddUsername
	origPass := vaultCredentialsAddPassword
	origMFA := vaultCredentialsAddMFA
	t.Cleanup(func() {
		vaultCredentialsAddURL = origURL
		vaultCredentialsAddEmail = origEmail
		vaultCredentialsAddUsername = origUser
		vaultCredentialsAddPassword = origPass
		vaultCredentialsAddMFA = origMFA
	})

	vaultCredentialsAddURL = "://bad"
	vaultCredentialsAddPassword = "pass"

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runVaultCredentialsAdd(cmd, nil)
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
	if !strings.Contains(err.Error(), "invalid URL format") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunVaultCredentialsAdd_EmptyPassword(t *testing.T) {
	_ = setupVaultTest(t)

	origURL := vaultCredentialsAddURL
	origPass := vaultCredentialsAddPassword
	t.Cleanup(func() {
		vaultCredentialsAddURL = origURL
		vaultCredentialsAddPassword = origPass
	})

	vaultCredentialsAddURL = "https://example.com"
	vaultCredentialsAddPassword = "   "

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runVaultCredentialsAdd(cmd, nil)
	if err == nil {
		t.Fatal("expected error for empty password")
	}
	if !strings.Contains(err.Error(), "password cannot be empty") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunVaultCredentialsAdd_InvalidEmail(t *testing.T) {
	_ = setupVaultTest(t)

	origURL := vaultCredentialsAddURL
	origEmail := vaultCredentialsAddEmail
	origPass := vaultCredentialsAddPassword
	t.Cleanup(func() {
		vaultCredentialsAddURL = origURL
		vaultCredentialsAddEmail = origEmail
		vaultCredentialsAddPassword = origPass
	})

	vaultCredentialsAddURL = "https://example.com"
	vaultCredentialsAddEmail = "not-an-email"
	vaultCredentialsAddPassword = "pass"

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runVaultCredentialsAdd(cmd, nil)
	if err == nil {
		t.Fatal("expected error for invalid email")
	}
	if !strings.Contains(err.Error(), "invalid email format") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunVaultCredentialsGet(t *testing.T) {
	server := setupVaultTest(t)
	server.AddResponse("/vaults/"+vaultIDTest+"/credentials", 200, `{"credentials":{"password":"pass","email":"test@example.com"}}`)

	origURL := vaultCredentialsGetURL
	vaultCredentialsGetURL = "https://example.com"
	t.Cleanup(func() { vaultCredentialsGetURL = origURL })

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runVaultCredentialsGet(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunVaultCredentialsDelete(t *testing.T) {
	server := setupVaultTest(t)
	server.AddResponse("/vaults/"+vaultIDTest+"/credentials", 200, `{"status":"deleted","message":"deleted"}`)

	origURL := vaultCredentialsDeleteURL
	vaultCredentialsDeleteURL = "https://example.com"
	t.Cleanup(func() { vaultCredentialsDeleteURL = origURL })

	SetSkipConfirmation(true)
	t.Cleanup(func() { SetSkipConfirmation(false) })

	origFormat := outputFormat
	outputFormat = "text"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runVaultCredentialsDelete(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(stdout, "deleted") {
		t.Errorf("expected delete message, got %q", stdout)
	}
}

func TestRunVaultCredentialsDeleteCancelled(t *testing.T) {
	_ = setupVaultTest(t)

	origURL := vaultCredentialsDeleteURL
	vaultCredentialsDeleteURL = "https://example.com"
	t.Cleanup(func() { vaultCredentialsDeleteURL = origURL })

	origSkip := skipConfirmation
	t.Cleanup(func() { skipConfirmation = origSkip })
	skipConfirmation = false

	origFormat := outputFormat
	outputFormat = "text"
	t.Cleanup(func() { outputFormat = origFormat })

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	_, _ = w.WriteString("n\n")
	_ = w.Close()

	origStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = origStdin
		_ = r.Close()
	})

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runVaultCredentialsDelete(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(stdout, "Cancelled.") {
		t.Errorf("expected cancel message, got %q", stdout)
	}
}

func TestRunVaultCard(t *testing.T) {
	server := setupVaultTest(t)
	server.AddResponse("/vaults/"+vaultIDTest+"/card", 200, `{"credit_card":{"card_cvv":"123","card_full_expiration":"12/25","card_holder_name":"Tester","card_number":"4111111111111111"}}`)

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runVaultCard(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunVaultCardSet(t *testing.T) {
	server := setupVaultTest(t)
	server.AddResponse("/vaults/"+vaultIDTest+"/card", 200, `{"status":"ok"}`)

	origNumber := vaultCardSetNumber
	origExpiry := vaultCardSetExpiry
	origCVV := vaultCardSetCVV
	origName := vaultCardSetName
	t.Cleanup(func() {
		vaultCardSetNumber = origNumber
		vaultCardSetExpiry = origExpiry
		vaultCardSetCVV = origCVV
		vaultCardSetName = origName
	})

	vaultCardSetNumber = "4111111111111111"
	vaultCardSetExpiry = "12/25"
	vaultCardSetCVV = "123"
	vaultCardSetName = "Tester"

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runVaultCardSet(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunVaultCardSet_EmptyNumber(t *testing.T) {
	_ = setupVaultTest(t)

	origNumber := vaultCardSetNumber
	origExpiry := vaultCardSetExpiry
	origCVV := vaultCardSetCVV
	origName := vaultCardSetName
	t.Cleanup(func() {
		vaultCardSetNumber = origNumber
		vaultCardSetExpiry = origExpiry
		vaultCardSetCVV = origCVV
		vaultCardSetName = origName
	})

	vaultCardSetNumber = ""
	vaultCardSetExpiry = "12/25"
	vaultCardSetCVV = "123"
	vaultCardSetName = "Tester"

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runVaultCardSet(cmd, nil)
	if err == nil {
		t.Fatal("expected error for empty card number")
	}
	if !strings.Contains(err.Error(), "card number cannot be empty") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunVaultCardDelete(t *testing.T) {
	server := setupVaultTest(t)
	server.AddResponse("/vaults/"+vaultIDTest+"/card", 200, `{"status":"deleted","message":"deleted"}`)

	SetSkipConfirmation(true)
	t.Cleanup(func() { SetSkipConfirmation(false) })

	origFormat := outputFormat
	outputFormat = "text"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runVaultCardDelete(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(stdout, "deleted") {
		t.Errorf("expected delete message, got %q", stdout)
	}
}

func TestRunVaultCardDeleteCancelled(t *testing.T) {
	_ = setupVaultTest(t)

	origSkip := skipConfirmation
	t.Cleanup(func() { skipConfirmation = origSkip })
	skipConfirmation = false

	origFormat := outputFormat
	outputFormat = "text"
	t.Cleanup(func() { outputFormat = origFormat })

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	_, _ = w.WriteString("n\n")
	_ = w.Close()

	origStdin := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = origStdin
		_ = r.Close()
	})

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runVaultCardDelete(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(stdout, "Cancelled.") {
		t.Errorf("expected cancel message, got %q", stdout)
	}
}
