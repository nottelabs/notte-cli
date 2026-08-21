package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var (
	filesListUploadsFlag   bool
	filesListDownloadsFlag bool
	filesListFrom          string
	filesDownloadOutput    string
)

const (
	filesSourceUploads = "uploads"
	filesSourceSession = "session"
)

var filesCmd = &cobra.Command{
	Use:   "files",
	Short: "Manage stored files",
	Long:  "Upload, list, and download files owned by a browser session.",
}

var filesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List stored files",
	Long:  `List all files owned by a browser session. Use --from uploads or --from session to filter the source.`,
	RunE:  runFilesList,
}

var filesUploadCmd = &cobra.Command{
	Use:   "upload <file-path>",
	Short: "Upload a file",
	Long:  "Upload a file to a browser session.",
	Args:  cobra.ExactArgs(1),
	RunE:  runFilesUpload,
}

var filesDownloadCmd = &cobra.Command{
	Use:   "download <file-id>",
	Short: "Download a session file by ID",
	Long:  "Download a user upload or browser download by its immutable file ID.",
	Args:  cobra.ExactArgs(1),
	RunE:  runFilesDownload,
}

func init() {
	rootCmd.AddCommand(filesCmd)
	filesCmd.AddCommand(filesListCmd)
	filesCmd.AddCommand(filesUploadCmd)
	filesCmd.AddCommand(filesDownloadCmd)

	// List command flags
	filesListCmd.Flags().BoolVar(&filesListUploadsFlag, "uploads", false, "List uploaded files")
	filesListCmd.Flags().BoolVar(&filesListDownloadsFlag, "downloads", false, "List downloaded files from a session")
	filesListCmd.Flags().StringVar(&filesListFrom, "from", "", "File source: uploads or session (default all)")
	filesListCmd.Flags().StringVar(&sessionID, "session-id", "", "Session ID (uses current session if not specified)")
	_ = filesListCmd.Flags().MarkDeprecated("uploads", "use --from uploads instead")
	_ = filesListCmd.Flags().MarkDeprecated("downloads", "use --from session instead")

	// Download command flags
	filesDownloadCmd.Flags().StringVar(&sessionID, "session-id", "", "Session ID (uses current session if not specified)")
	filesDownloadCmd.Flags().StringVar(&filesDownloadOutput, "path", "", "Output file path (defaults to current directory)")
	filesUploadCmd.Flags().StringVar(&sessionID, "session-id", "", "Session ID (uses current session if not specified)")
}

type sessionFile struct {
	ID        string `json:"id"`
	SessionID string `json:"session_id"`
	Filename  string `json:"filename"`
	MimeType  string `json:"mime_type"`
	Size      int64  `json:"size"`
	Checksum  string `json:"checksum"`
	CreatedAt string `json:"created_at"`
	ExpiresAt string `json:"expires_at"`
	Source    string `json:"source"`
}

type sessionFilesPage struct {
	Files  []sessionFile `json:"files"`
	Total  int           `json:"total"`
	Limit  int           `json:"limit"`
	Offset int           `json:"offset"`
}

func sessionFilesURL(clientBaseURL, id string) string {
	return strings.TrimRight(clientBaseURL, "/") + "/sessions/" + url.PathEscape(sessionID) + "/files" + id
}

func resolveFilesSource(from string, uploads, downloads bool) (string, error) {
	if uploads && downloads {
		return "", fmt.Errorf("--uploads and --downloads cannot be used together")
	}
	if from != "" && (uploads || downloads) {
		return "", fmt.Errorf("--from cannot be combined with --uploads or --downloads")
	}

	if uploads {
		return filesSourceUploads, nil
	}
	if downloads {
		return filesSourceSession, nil
	}

	switch from {
	case "":
		return "", nil
	case filesSourceSession:
		return filesSourceSession, nil
	case filesSourceUploads:
		return filesSourceUploads, nil
	default:
		return "", fmt.Errorf("invalid --from value %q: must be uploads or session", from)
	}
}

func resolveDownloadOutputPath(outputPath string) (string, error) {
	resolvedPath := outputPath
	for range 255 {
		info, err := os.Lstat(resolvedPath)
		if os.IsNotExist(err) {
			return resolvedPath, nil
		}
		if err != nil {
			return "", fmt.Errorf("failed to inspect destination: %w", err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			return resolvedPath, nil
		}

		target, err := os.Readlink(resolvedPath)
		if err != nil {
			return "", fmt.Errorf("failed to read destination symlink: %w", err)
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(resolvedPath), target)
		}
		resolvedPath = filepath.Clean(target)
	}

	return "", fmt.Errorf("destination has too many symlink levels")
}

func downloadFileWithContext(ctx context.Context, fileURL, outputPath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create download request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	destinationPath, err := resolveDownloadOutputPath(outputPath)
	if err != nil {
		return err
	}

	fileMode := os.FileMode(0o644)
	if info, statErr := os.Stat(destinationPath); statErr == nil {
		if info.IsDir() {
			return fmt.Errorf("destination path is a directory")
		}
		fileMode = info.Mode().Perm()
	} else if !os.IsNotExist(statErr) {
		return fmt.Errorf("failed to inspect destination: %w", statErr)
	}

	tempFile, err := os.CreateTemp(
		filepath.Dir(destinationPath),
		"."+filepath.Base(destinationPath)+".tmp-*",
	)
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	tempPath := tempFile.Name()
	committed := false
	defer func() {
		_ = tempFile.Close()
		if !committed {
			_ = os.Remove(tempPath)
		}
	}()

	if _, err := io.Copy(tempFile, resp.Body); err != nil {
		return fmt.Errorf("failed to write temporary file: %w", err)
	}
	if err := tempFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync temporary file: %w", err)
	}
	if err := tempFile.Chmod(fileMode); err != nil {
		return fmt.Errorf("failed to set file permissions: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file: %w", err)
	}
	if err := os.Rename(tempPath, destinationPath); err != nil {
		return fmt.Errorf("failed to replace destination: %w", err)
	}
	committed = true
	return nil
}

func runFilesList(cmd *cobra.Command, args []string) error {
	source, err := resolveFilesSource(filesListFrom, filesListUploadsFlag, filesListDownloadsFlag)
	if err != nil {
		return err
	}

	if err := RequireSessionID(); err != nil {
		return err
	}
	client, err := GetClient()
	if err != nil {
		return err
	}
	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()
	endpoint := sessionFilesURL(client.BaseURL(), "") + "?limit=1000"
	if source == filesSourceUploads {
		endpoint += "&source=user_upload"
	} else {
		endpoint += "&source=session_download"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := client.HTTPClient().Do(req)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if err := HandleAPIResponse(resp, body); err != nil {
		return err
	}
	var page sessionFilesPage
	if err := json.Unmarshal(body, &page); err != nil {
		return fmt.Errorf("failed to parse files response: %w", err)
	}
	if printed, err := PrintListOrEmpty(page.Files, fmt.Sprintf("No files in session %s.", sessionID)); err != nil {
		return err
	} else if printed {
		return nil
	}
	return GetFormatter().Print(page.Files)
}

func runFilesUpload(cmd *cobra.Command, args []string) error {
	filePath := args[0]

	// Check if file exists
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("failed to access file: %w", err)
	}

	if fileInfo.IsDir() {
		return fmt.Errorf("path is a directory, not a file: %s", filePath)
	}
	if err := RequireSessionID(); err != nil {
		return err
	}

	client, err := GetClient()
	if err != nil {
		return err
	}

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Create multipart form data in memory (simpler, no race condition)
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := io.Copy(part, file); err != nil {
		return fmt.Errorf("failed to copy file data: %w", err)
	}

	_ = writer.Close()

	filename := filepath.Base(filePath)
	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sessionFilesURL(client.BaseURL(), ""), &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := client.HTTPClient().Do(req)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if err := HandleAPIResponse(resp, body); err != nil {
		return err
	}
	var uploaded sessionFile
	if err := json.Unmarshal(body, &uploaded); err != nil {
		return fmt.Errorf("failed to parse upload response: %w", err)
	}
	return PrintResult(fmt.Sprintf("File uploaded successfully: %s", filename), map[string]any{
		"file": uploaded,
	})
}

func runFilesDownload(cmd *cobra.Command, args []string) error {
	fileID := args[0]
	if err := RequireSessionID(); err != nil {
		return err
	}

	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	req, err := http.NewRequestWithContext(
		ctx, http.MethodGet, sessionFilesURL(client.BaseURL(), "/"+url.PathEscape(fileID)), nil,
	)
	if err != nil {
		return err
	}
	resp, err := client.HTTPClient().Do(req)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return readErr
		}
		return HandleAPIResponse(resp, body)
	}
	outputPath := filesDownloadOutput
	if outputPath == "" {
		outputPath = fileID
		if _, params, parseErr := mime.ParseMediaType(resp.Header.Get("Content-Disposition")); parseErr == nil {
			if filename := filepath.Base(params["filename"]); filename != "." && filename != "" {
				outputPath = filename
			}
		}
	}
	destinationPath, err := resolveDownloadOutputPath(outputPath)
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(destinationPath), ".notte-download-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if _, err := io.Copy(temporary, resp.Body); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, destinationPath); err != nil {
		return err
	}
	return PrintResult(fmt.Sprintf("File downloaded successfully: %s", outputPath), map[string]any{
		"id":      fileID,
		"path":    outputPath,
		"success": true,
	})
}
