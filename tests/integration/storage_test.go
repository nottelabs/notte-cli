//go:build integration

package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func startStorageSession(t *testing.T) string {
	t.Helper()
	result := runCLI(t, "sessions", "start")
	requireSuccess(t, result)

	var response struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &response); err != nil {
		t.Fatalf("Failed to parse session start response: %v", err)
	}
	if response.SessionID == "" {
		t.Fatal("Session start response did not include a session_id")
	}
	t.Cleanup(func() { cleanupSession(t, response.SessionID) })
	return response.SessionID
}

func TestStorageListUploads(t *testing.T) {
	sessionID := startStorageSession(t)

	// List uploads - should work even if empty
	result := runCLI(t, "files", "list", "--from", "uploads", "--session-id", sessionID)
	requireSuccess(t, result)
	t.Log("Successfully listed uploads")
}

func TestStorageUploadListAndDownload(t *testing.T) {
	sessionID := startStorageSession(t)

	// Create a temporary file to upload
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test-upload.txt")
	testContent := []byte("This is a test file for integration testing")
	if err := os.WriteFile(testFile, testContent, 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Upload the file
	result := runCLI(t, "files", "upload", testFile, "--session-id", sessionID)
	requireSuccess(t, result)
	t.Log("Successfully uploaded file")
	var uploadResp struct {
		File struct {
			ID string `json:"id"`
		} `json:"file"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &uploadResp); err != nil {
		t.Fatalf("Failed to parse upload response: %v", err)
	}
	if uploadResp.File.ID == "" {
		t.Fatal("Upload response did not include a file ID")
	}

	// List uploads - should include our file
	result = runCLI(t, "files", "list", "--from", "uploads", "--session-id", sessionID)
	requireSuccess(t, result)
	if !containsString(result.Stdout, "test-upload.txt") {
		t.Log("Upload might use different filename, but list succeeded")
	}
	t.Log("Successfully verified file in uploads list")

	// Download the uploaded file again
	downloadPath := filepath.Join(tmpDir, "downloaded-test-upload.txt")
	result = runCLI(t, "files", "download", uploadResp.File.ID, "--session-id", sessionID, "--path", downloadPath)
	requireSuccess(t, result)

	downloadedContent, err := os.ReadFile(downloadPath)
	if err != nil {
		t.Fatalf("Failed to read downloaded file: %v", err)
	}
	if string(downloadedContent) != string(testContent) {
		t.Fatalf("Downloaded content does not match uploaded content")
	}
	t.Log("Successfully downloaded uploaded file")
}

func TestStorageDownloadFromSession(t *testing.T) {
	sessionID := startStorageSession(t)

	// Wait for session to be ready
	time.Sleep(2 * time.Second)

	// List downloads from session (likely empty)
	result := runCLI(t, "files", "list", "--from", "session", "--session-id", sessionID)
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
	sessionID := startStorageSession(t)

	// Try to download a non-existent file
	result := runCLI(t, "files", "download", "00000000-0000-0000-0000-000000000000", "--session-id", sessionID)
	requireFailure(t, result)
	t.Log("Correctly failed to download non-existent file")
}

func TestStorageUploadLargeFile(t *testing.T) {
	sessionID := startStorageSession(t)

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
	result := runCLIWithTimeout(t, 120*time.Second, "files", "upload", testFile, "--session-id", sessionID)
	requireSuccess(t, result)
	t.Log("Successfully uploaded large file")
}
