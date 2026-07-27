package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/testutil"
)

const profileIDTest = "notte-profile-abc123"

func setupProfileTest(t *testing.T) *testutil.MockServer {
	t.Helper()
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	t.Cleanup(func() { server.Close() })
	env.SetEnv("NOTTE_API_URL", server.URL())

	origProfileID := profileID
	profileID = profileIDTest
	t.Cleanup(func() { profileID = origProfileID })

	return server
}

func profileJSON() string {
	return `{"profile_id":"` + profileIDTest + `","name":"Test Profile","created_at":"2020-01-01T00:00:00Z","updated_at":"2020-01-01T00:00:00Z"}`
}

func TestRunProfilesList_Success(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	server.AddResponse("/profiles", 200, `{"items":[{"profile_id":"notte-profile-abc123","name":"Test Profile","created_at":"2020-01-01T00:00:00Z","updated_at":"2020-01-01T00:00:00Z"}]}`)

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runProfilesList(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunProfilesList_Empty(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	server.AddResponse("/profiles", 200, `{"items":[]}`)

	origFormat := outputFormat
	outputFormat = "text"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runProfilesList(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(stdout, "No profiles found.") {
		t.Errorf("expected empty message, got %q", stdout)
	}
}

func TestRunProfilesCreate_Success(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	server.AddResponse("/profiles/create", 200, `{"profile_id":"notte-profile-abc123","name":"New Profile","created_at":"2020-01-01T00:00:00Z","updated_at":"2020-01-01T00:00:00Z"}`)

	origName := ProfileCreateName
	ProfileCreateName = "New Profile"
	t.Cleanup(func() { ProfileCreateName = origName })

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runProfilesCreate(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunProfileShow(t *testing.T) {
	server := setupProfileTest(t)
	server.AddResponse("/profiles/"+profileIDTest, 200, profileJSON())

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runProfileShow(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunProfileDelete(t *testing.T) {
	server := setupProfileTest(t)
	server.AddResponse("/profiles/"+profileIDTest, 200, `{"message":"deleted","success":true}`)

	SetSkipConfirmation(true)
	t.Cleanup(func() { SetSkipConfirmation(false) })

	origFormat := outputFormat
	outputFormat = "text"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runProfileDelete(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(stdout, "deleted") {
		t.Errorf("expected delete message, got %q", stdout)
	}
}

func TestRunProfileDuplicate(t *testing.T) {
	server := setupProfileTest(t)
	path := "/profiles/" + profileIDTest + "/duplicate"
	server.AddResponse(path, http.StatusOK, `{"profile_id":"notte-profile-copy123","name":"Copied Profile","created_at":"2020-01-01T00:00:00Z","updated_at":"2020-01-01T00:00:00Z"}`)

	originalName := profileDuplicateName
	profileDuplicateName = "Copied Profile"
	t.Cleanup(func() { profileDuplicateName = originalName })

	originalFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = originalFormat })

	cmd := &cobra.Command{}
	cmd.Flags().String("name", "", "")
	_ = cmd.Flags().Set("name", profileDuplicateName)
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		if err := runProfileDuplicate(cmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(stdout, "notte-profile-copy123") {
		t.Errorf("expected duplicate profile output, got %q", stdout)
	}
	requests := server.Requests(path)
	if len(requests) != 1 {
		t.Fatalf("expected one duplicate request, got %d", len(requests))
	}
	if requests[0].Method != http.MethodPost {
		t.Errorf("expected POST, got %s", requests[0].Method)
	}
	var body map[string]string
	if err := json.Unmarshal([]byte(requests[0].Body), &body); err != nil {
		t.Fatalf("invalid request JSON: %v", err)
	}
	if body["name"] != "Copied Profile" {
		t.Errorf("expected destination name, got %#v", body)
	}
}
