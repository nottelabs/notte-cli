package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/salmonumbrella/notte-cli/internal/config"
	"github.com/salmonumbrella/notte-cli/internal/testutil"
)

func TestRunFilesListUploads(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	server.AddResponse("/storage/uploads", 200, `{"files":["a.txt"]}`)

	origDownloadsFlag := filesListDownloadsFlag
	origSession := sessionID
	t.Cleanup(func() {
		filesListDownloadsFlag = origDownloadsFlag
		sessionID = origSession
	})
	filesListDownloadsFlag = false
	sessionID = ""

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runFilesList(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunFilesListUploadsEmpty(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	server.AddResponse("/storage/uploads", 200, `{"files":[]}`)

	origDownloadsFlag := filesListDownloadsFlag
	origSession := sessionID
	t.Cleanup(func() {
		filesListDownloadsFlag = origDownloadsFlag
		sessionID = origSession
	})
	filesListDownloadsFlag = false
	sessionID = ""

	origFormat := outputFormat
	outputFormat = "text"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runFilesList(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(stdout, "No uploaded files.") {
		t.Fatalf("expected empty message, got %q", stdout)
	}
}

func TestRunFilesListDownloads(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	server.AddResponse("/storage/sess_123/downloads", 200, `{"files":["b.txt"]}`)

	origDownloadsFlag := filesListDownloadsFlag
	origSession := sessionID
	t.Cleanup(func() {
		filesListDownloadsFlag = origDownloadsFlag
		sessionID = origSession
	})
	filesListDownloadsFlag = true
	sessionID = "sess_123"

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runFilesList(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunFilesListDownloadsMissingSession(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key") // Need API key for GetClient()
	env.SetEnv("NOTTE_SESSION_ID", "")      // Clear session env var

	// Set up empty config dir (no session file)
	tmpDir := t.TempDir()
	config.SetTestConfigDir(tmpDir)
	t.Cleanup(func() { config.SetTestConfigDir("") })

	origDownloadsFlag := filesListDownloadsFlag
	origSession := sessionID
	t.Cleanup(func() {
		filesListDownloadsFlag = origDownloadsFlag
		sessionID = origSession
	})
	filesListDownloadsFlag = true
	sessionID = ""

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runFilesList(cmd, nil)
	if err == nil {
		t.Fatal("expected error for missing session")
	}
	if !strings.Contains(err.Error(), "session ID required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunFilesUpload(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	tmpFile, err := os.CreateTemp("", "upload-*.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	if _, err := tmpFile.WriteString("hello"); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("failed to close temp file: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(tmpFile.Name()) })

	server.AddResponse("/storage/uploads/"+filepath.Base(tmpFile.Name()), 200, `{"success":true}`)

	origFormat := outputFormat
	outputFormat = "text"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runFilesUpload(cmd, []string{tmpFile.Name()})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(stdout, "File uploaded successfully") {
		t.Fatalf("expected upload message, got %q", stdout)
	}
}

func TestRunFilesUploadDirectory(t *testing.T) {
	dir := t.TempDir()
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runFilesUpload(cmd, []string{dir})
	if err == nil {
		t.Fatal("expected error for directory path")
	}
	if !strings.Contains(err.Error(), "path is a directory") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunFilesDownload(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	origSession := sessionID
	origOutput := filesDownloadOutput
	t.Cleanup(func() {
		sessionID = origSession
		filesDownloadOutput = origOutput
	})
	sessionID = "sess_123"

	outDir := t.TempDir()
	outputPath := filepath.Join(outDir, "download.txt")
	filesDownloadOutput = outputPath

	server.AddResponseWithHeaders("/storage/sess_123/downloads/file.txt", 200, "filedata", map[string]string{
		"Content-Type": "application/octet-stream",
	})

	origFormat := outputFormat
	outputFormat = "text"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runFilesDownload(cmd, []string{"file.txt"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	got, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}
	if string(got) != "filedata" {
		t.Fatalf("unexpected file content: %q", string(got))
	}
	if !strings.Contains(stdout, "File downloaded successfully") {
		t.Fatalf("expected download message, got %q", stdout)
	}
}

func TestRunFilesDownloadMissingSession(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_SESSION_ID", "") // Clear session env var

	// Set up empty config dir (no session file)
	tmpDir := t.TempDir()
	config.SetTestConfigDir(tmpDir)
	t.Cleanup(func() { config.SetTestConfigDir("") })

	origSession := sessionID
	t.Cleanup(func() { sessionID = origSession })
	sessionID = ""

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runFilesDownload(cmd, []string{"file.txt"})
	if err == nil {
		t.Fatal("expected error for missing session")
	}
	if !strings.Contains(err.Error(), "session ID required") {
		t.Fatalf("unexpected error: %v", err)
	}
}
