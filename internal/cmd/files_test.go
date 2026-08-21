package cmd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/config"
	"github.com/nottelabs/notte-cli/internal/testutil"
)

func TestDownloadFileWithContextHonorsCancellation(t *testing.T) {
	requestStarted := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("partial"))
		w.(http.Flusher).Flush()
		close(requestStarted)
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	outputPath := filepath.Join(t.TempDir(), "download.txt")
	result := make(chan error, 1)
	go func() {
		result <- downloadFileWithContext(ctx, server.URL, outputPath)
	}()

	select {
	case <-requestStarted:
		cancel()
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatal("download request did not start")
	}

	select {
	case err := <-result:
		if err == nil {
			t.Fatal("expected canceled download to fail")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("download did not stop after context cancellation")
	}

	if _, err := os.Stat(outputPath); !os.IsNotExist(err) {
		t.Fatalf("canceled download left destination file behind: %v", err)
	}
}

func TestDownloadFileWithContextPreservesDestinationOnFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("partial"))
	}))
	defer server.Close()

	outputPath := filepath.Join(t.TempDir(), "download.txt")
	if err := os.WriteFile(outputPath, []byte("existing-content"), 0o644); err != nil {
		t.Fatalf("failed to create destination: %v", err)
	}

	if err := downloadFileWithContext(context.Background(), server.URL, outputPath); err == nil {
		t.Fatal("expected interrupted download to fail")
	}

	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read destination: %v", err)
	}
	if string(content) != "existing-content" {
		t.Fatalf("destination was modified after failed download: %q", string(content))
	}

	tempFiles, err := filepath.Glob(filepath.Join(filepath.Dir(outputPath), ".download.txt.tmp-*"))
	if err != nil {
		t.Fatalf("failed to inspect temporary files: %v", err)
	}
	if len(tempFiles) != 0 {
		t.Fatalf("temporary files were not cleaned up: %v", tempFiles)
	}
}

func TestDownloadFileWithContextPreservesDestinationSymlink(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("downloaded-content"))
	}))
	defer server.Close()

	outputDir := t.TempDir()
	targetPath := filepath.Join(outputDir, "target.txt")
	if err := os.WriteFile(targetPath, []byte("existing-content"), 0o644); err != nil {
		t.Fatalf("failed to create symlink target: %v", err)
	}
	linkPath := filepath.Join(outputDir, "download.txt")
	if err := os.Symlink(filepath.Base(targetPath), linkPath); err != nil {
		t.Skipf("symlinks are not available: %v", err)
	}

	if err := downloadFileWithContext(context.Background(), server.URL, linkPath); err != nil {
		t.Fatalf("unexpected download error: %v", err)
	}

	linkInfo, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("failed to inspect destination symlink: %v", err)
	}
	if linkInfo.Mode()&os.ModeSymlink == 0 {
		t.Fatal("destination symlink was replaced")
	}

	content, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("failed to read symlink target: %v", err)
	}
	if string(content) != "downloaded-content" {
		t.Fatalf("symlink target was not updated: %q", string(content))
	}
}

func TestRunFilesListUploads(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	server.AddResponse("/sessions/sess_123/files", 200, `{"files":[{"id":"f1","session_id":"sess_123","filename":"a.txt","mime_type":"text/plain","size":100,"checksum":"abc","created_at":"2026-08-21T00:00:00Z","expires_at":"2026-08-22T00:00:00Z","source":"user_upload"}],"total":1,"limit":1000,"offset":0}`)

	origUploadsFlag := filesListUploadsFlag
	origFrom := filesListFrom
	origSession := sessionID
	t.Cleanup(func() {
		filesListUploadsFlag = origUploadsFlag
		filesListFrom = origFrom
		sessionID = origSession
	})
	filesListUploadsFlag = false
	filesListFrom = filesSourceUploads
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

func TestRunFilesListUploadsEmpty(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	server.AddResponse("/sessions/sess_123/files", 200, `{"files":[],"total":0,"limit":1000,"offset":0}`)

	origUploadsFlag := filesListUploadsFlag
	origFrom := filesListFrom
	origSession := sessionID
	t.Cleanup(func() {
		filesListUploadsFlag = origUploadsFlag
		filesListFrom = origFrom
		sessionID = origSession
	})
	filesListUploadsFlag = true
	filesListFrom = ""
	sessionID = "sess_123"

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

	if !strings.Contains(stdout, "No files in session") {
		t.Fatalf("expected empty message, got %q", stdout)
	}
}

func TestRunFilesListDownloads(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	server.AddResponse("/sessions/sess_123/files", 200, `{"files":[{"id":"f2","session_id":"sess_123","filename":"b.txt","mime_type":"text/plain","size":200,"checksum":"abc","created_at":"2026-08-21T00:00:00Z","expires_at":"2026-08-22T00:00:00Z","source":"session_download"}],"total":1,"limit":1000,"offset":0}`)

	origDownloadsFlag := filesListDownloadsFlag
	origFrom := filesListFrom
	origSession := sessionID
	t.Cleanup(func() {
		filesListDownloadsFlag = origDownloadsFlag
		filesListFrom = origFrom
		sessionID = origSession
	})
	filesListDownloadsFlag = true
	filesListFrom = ""
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
	origFrom := filesListFrom
	origSession := sessionID
	t.Cleanup(func() {
		filesListDownloadsFlag = origDownloadsFlag
		filesListFrom = origFrom
		sessionID = origSession
	})
	filesListDownloadsFlag = true
	filesListFrom = ""
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

	server.AddResponse("/sessions/sess_123/files", 201, `{"id":"f1","session_id":"sess_123","filename":"`+filepath.Base(tmpFile.Name())+`","mime_type":"text/plain","size":5,"checksum":"abc","created_at":"2026-08-21T00:00:00Z","expires_at":"2026-08-22T00:00:00Z","source":"user_upload"}`)
	origSession := sessionID
	sessionID = "sess_123"
	t.Cleanup(func() { sessionID = origSession })

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

	server.AddResponse("/sessions/sess_123/files/file-id", 200, "filedata")

	origFormat := outputFormat
	outputFormat = "text"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runFilesDownload(cmd, []string{"file-id"})
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

func TestRunFilesDownloadFromUploads(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")
	env.SetEnv("NOTTE_SESSION_ID", "")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())
	server.AddResponse("/sessions/sess_123/files/upload-id", 200, "uploaded-file-data")

	origSession := sessionID
	origOutput := filesDownloadOutput
	t.Cleanup(func() {
		sessionID = origSession
		filesDownloadOutput = origOutput
	})
	sessionID = "sess_123"
	filesDownloadOutput = filepath.Join(t.TempDir(), "input.txt")

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	if err := runFilesDownload(cmd, []string{"upload-id"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := os.ReadFile(filesDownloadOutput)
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}
	if string(got) != "uploaded-file-data" {
		t.Fatalf("unexpected file content: %q", string(got))
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

func TestResolveFilesSource(t *testing.T) {
	tests := []struct {
		name      string
		from      string
		uploads   bool
		downloads bool
		want      string
		wantErr   bool
	}{
		{name: "defaults to all"},
		{name: "from uploads", from: filesSourceUploads, want: filesSourceUploads},
		{name: "from session", from: filesSourceSession, want: filesSourceSession},
		{name: "legacy uploads", uploads: true, want: filesSourceUploads},
		{name: "legacy downloads", downloads: true, want: filesSourceSession},
		{name: "invalid source", from: "local", wantErr: true},
		{name: "conflicting legacy flags", uploads: true, downloads: true, wantErr: true},
		{name: "mixed source flags", from: filesSourceUploads, downloads: true, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveFilesSource(tt.from, tt.uploads, tt.downloads)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
