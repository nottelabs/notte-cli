package cmd

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/salmonumbrella/notte-cli/internal/testutil"
)

const personaIDTest = "persona_123"

func setupPersonaTest(t *testing.T) *testutil.MockServer {
	t.Helper()
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	t.Cleanup(func() { server.Close() })
	env.SetEnv("NOTTE_API_URL", server.URL())

	origPersonaID := personaID
	personaID = personaIDTest
	t.Cleanup(func() { personaID = origPersonaID })

	return server
}

func personaResponseJSON() string {
	return `{"persona_id":"` + personaIDTest + `","email":"test@example.com","first_name":"Test","last_name":"User","status":"active"}`
}

func TestRunPersonaShow(t *testing.T) {
	server := setupPersonaTest(t)
	server.AddResponse("/personas/"+personaIDTest, 200, personaResponseJSON())

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runPersonaShow(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunPersonaDelete(t *testing.T) {
	server := setupPersonaTest(t)
	server.AddResponse("/personas/"+personaIDTest, 200, `{"status":"deleted","message":"deleted"}`)

	SetSkipConfirmation(true)
	t.Cleanup(func() { SetSkipConfirmation(false) })

	origFormat := outputFormat
	outputFormat = "text"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runPersonaDelete(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(stdout, "deleted") {
		t.Errorf("expected delete message, got %q", stdout)
	}
}

func TestRunPersonaDeleteCancelled(t *testing.T) {
	_ = setupPersonaTest(t)

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
		err := runPersonaDelete(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(stdout, "Cancelled.") {
		t.Errorf("expected cancel message, got %q", stdout)
	}
}

func TestRunPersonaEmails(t *testing.T) {
	server := setupPersonaTest(t)
	server.AddResponse("/personas/"+personaIDTest+"/emails", 200, `[{"created_at":"2020-01-01T00:00:00Z","email_id":"email_1","subject":"Hello"}]`)

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runPersonaEmails(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunPersonaSms(t *testing.T) {
	server := setupPersonaTest(t)
	server.AddResponse("/personas/"+personaIDTest+"/sms", 200, `[{"created_at":"2020-01-01T00:00:00Z","sms_id":"sms_1","body":"Hi"}]`)

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runPersonaSms(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunPersonaPhoneCreate(t *testing.T) {
	server := setupPersonaTest(t)
	server.AddResponse("/personas/"+personaIDTest+"/sms/number", 200, `{"phone_number":"+1234567890","status":"success"}`)

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runPersonaPhoneCreate(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunPersonaPhoneDelete(t *testing.T) {
	server := setupPersonaTest(t)
	server.AddResponse("/personas/"+personaIDTest+"/sms/number", 200, `{"status":"success"}`)

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runPersonaPhoneDelete(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}
