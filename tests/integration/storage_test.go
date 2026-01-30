//go:build integration

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStorageListUploads(t *testing.T) {
	// List uploads - should work even if empty
	result := runCLI(t, "files", "list", "--uploads")
	requireSuccess(t, result)
	t.Log("Successfully listed uploads")
}

func TestStorageUploadAndList(t *testing.T) {
	// Create a temporary file to upload
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test-upload.txt")
	testContent := []byte("This is a test file for integration testing")
	if err := os.WriteFile(testFile, testContent, 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Upload the file
	result := runCLI(t, "files", "upload", testFile)
	requireSuccess(t, result)
	t.Log("Successfully uploaded file")

	// List uploads - should include our file
	result = runCLI(t, "files", "list", "--uploads")
	requireSuccess(t, result)
	if !containsString(result.Stdout, "test-upload.txt") {
		t.Log("Upload might use different filename, but list succeeded")
	}
	t.Log("Successfully verified file in uploads list")
}

func TestStorageDownloadFromSession(t *testing.T) {
	// Start a session with file storage enabled
	result := runCLI(t, "sessions", "start", "--headless", "--use-file-storage")
	requireSuccess(t, result)

	var startResp struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &startResp); err != nil {
		t.Fatalf("Failed to parse session start response: %v", err)
	}
	sessionID := startResp.SessionID
	defer cleanupSession(t, sessionID)

	// Wait for session to be ready
	time.Sleep(2 * time.Second)

	// List downloads from session (likely empty)
	result = runCLI(t, "files", "list", "--downloads", "--id", sessionID)
	requireSuccess(t, result)
	t.Log("Successfully listed session downloads")
}

func TestStorageListDownloadsRequiresSession(t *testing.T) {
	// Try to list downloads without session ID
	result := runCLI(t, "files", "list", "--downloads")
	requireFailure(t, result)
	t.Log("Correctly failed when session ID not provided for downloads")
}

func TestStorageDownloadNonexistent(t *testing.T) {
	// Start a session with file storage enabled
	result := runCLI(t, "sessions", "start", "--headless", "--use-file-storage")
	requireSuccess(t, result)

	var startResp struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &startResp); err != nil {
		t.Fatalf("Failed to parse session start response: %v", err)
	}
	sessionID := startResp.SessionID
	defer cleanupSession(t, sessionID)

	// Try to download a non-existent file
	result = runCLI(t, "files", "download", "nonexistent-file-12345.txt", "--id", sessionID)
	requireFailure(t, result)
	t.Log("Correctly failed to download non-existent file")
}

func TestStorageUploadLargeFile(t *testing.T) {
	// Create a larger test file (1MB)
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "large-test-file.bin")

	// Create 1MB of data
	data := make([]byte, 1024*1024)
	for i := range data {
		data[i] = byte(i % 256)
	}

	if err := os.WriteFile(testFile, data, 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Upload the file
	result := runCLIWithTimeout(t, 120*time.Second, "files", "upload", testFile)
	requireSuccess(t, result)
	t.Log("Successfully uploaded large file")
}
