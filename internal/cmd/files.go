package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/api"
)

var (
	filesListUploadsFlag   bool
	filesListDownloadsFlag bool
	filesListFrom          string
	filesDownloadFrom      string
	filesDownloadOutput    string
)

const (
	filesSourceUploads = "uploads"
	filesSourceSession = "session"
)

var filesCmd = &cobra.Command{
	Use:   "files",
	Short: "Manage stored files",
	Long:  "Upload, list, and download files from notte.cc storage.",
}

var filesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List stored files",
	Long: `List user-uploaded files or files downloaded by a browser session.
Use --from uploads for user uploads or --from session for session downloads.`,
	RunE: runFilesList,
}

var filesUploadCmd = &cobra.Command{
	Use:   "upload <file-path>",
	Short: "Upload a file",
	Long:  "Upload a file to notte.cc storage.",
	Args:  cobra.ExactArgs(1),
	RunE:  runFilesUpload,
}

var filesDownloadCmd = &cobra.Command{
	Use:   "download <filename>",
	Short: "Download a file by name",
	Long: `Download a user-uploaded file or a file produced by a browser session.
Use --from uploads for user uploads or --from session for session downloads.`,
	Args: cobra.ExactArgs(1),
	RunE: runFilesDownload,
}

func init() {
	rootCmd.AddCommand(filesCmd)
	filesCmd.AddCommand(filesListCmd)
	filesCmd.AddCommand(filesUploadCmd)
	filesCmd.AddCommand(filesDownloadCmd)

	// List command flags
	filesListCmd.Flags().BoolVar(&filesListUploadsFlag, "uploads", false, "List uploaded files")
	filesListCmd.Flags().BoolVar(&filesListDownloadsFlag, "downloads", false, "List downloaded files from a session")
	filesListCmd.Flags().StringVar(&filesListFrom, "from", "", "File source: uploads or session (default session)")
	filesListCmd.Flags().StringVar(&sessionID, "session-id", "", "Session ID (required with --from session)")
	_ = filesListCmd.Flags().MarkDeprecated("uploads", "use --from uploads instead")
	_ = filesListCmd.Flags().MarkDeprecated("downloads", "use --from session instead")

	// Download command flags
	filesDownloadCmd.Flags().StringVar(&sessionID, "session-id", "", "Session ID (required with --from session)")
	filesDownloadCmd.Flags().StringVar(&filesDownloadFrom, "from", "", "File source: uploads or session (default session)")
	filesDownloadCmd.Flags().StringVar(&filesDownloadOutput, "path", "", "Output file path (defaults to current directory)")
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
	case "", filesSourceSession:
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
	if source == filesSourceSession {
		if err := RequireSessionID(); err != nil {
			return err
		}
	}

	client, err := GetClient()
	if err != nil {
		return err
	}

	formatter := GetFormatter()

	if source == filesSourceUploads {
		ctx, cancel := GetContextWithTimeout(cmd.Context())
		defer cancel()

		params := &api.FileListUploadsParams{}
		resp, err := client.Client().FileListUploadsWithResponse(ctx, params)
		if err != nil {
			return fmt.Errorf("API request failed: %w", err)
		}

		if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
			return err
		}

		var fileNames []string
		if resp.JSON200 != nil {
			for _, f := range resp.JSON200.Files {
				fileNames = append(fileNames, f.Name)
			}
		}
		if printed, err := PrintListOrEmpty(fileNames, "No uploaded files."); err != nil {
			return err
		} else if printed {
			return nil
		}

		if !IsJSONOutput() {
			fmt.Println("Your uploaded files:")
		}
		return formatter.Print(fileNames)
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.FileListDownloadsParams{}
	resp, err := client.Client().FileListDownloadsWithResponse(ctx, sessionID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	var fileNames []string
	if resp.JSON200 != nil {
		for _, f := range resp.JSON200.Files {
			fileNames = append(fileNames, f.Name)
		}
	}
	if printed, err := PrintListOrEmpty(fileNames, fmt.Sprintf("No downloaded files in session %s.", sessionID)); err != nil {
		return err
	} else if printed {
		return nil
	}

	if !IsJSONOutput() {
		fmt.Printf("Downloaded files in session %s:\n", sessionID)
		fmt.Printf("Fetch locally with: notte files download <filename> --from session --session-id %s\n", sessionID)
		fmt.Println()
	}
	return formatter.Print(fileNames)
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

	// Get the filename to use in the API call
	filename := filepath.Base(filePath)

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.FileUploadParams{}
	resp, err := client.Client().FileUploadWithBodyWithResponse(
		ctx,
		filename,
		params,
		writer.FormDataContentType(),
		&buf,
	)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	formatter := GetFormatter()
	if resp.JSON200 != nil && resp.JSON200.Success {
		if IsJSONOutput() {
			return formatter.Print(resp.JSON200)
		}
		return PrintResult(fmt.Sprintf("File uploaded successfully: %s", filename), map[string]any{
			"filename": filename,
			"success":  true,
		})
	}

	return formatter.Print(resp.JSON200)
}

func runFilesDownload(cmd *cobra.Command, args []string) error {
	filename := args[0]

	source, err := resolveFilesSource(filesDownloadFrom, false, false)
	if err != nil {
		return err
	}

	if source == filesSourceSession {
		if err := RequireSessionID(); err != nil {
			return err
		}
	}

	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	var httpResponse *http.Response
	var responseBody []byte
	if source == filesSourceUploads {
		httpResponse, responseBody, err = client.DownloadUploadedFile(ctx, filename)
		if err != nil {
			return fmt.Errorf("API request failed: %w", err)
		}
	} else {
		params := &api.FileDownloadParams{}
		resp, err := client.Client().FileDownloadWithResponse(
			ctx,
			sessionID,
			filename,
			params,
		)
		if err != nil {
			return fmt.Errorf("API request failed: %w", err)
		}
		httpResponse = resp.HTTPResponse
		responseBody = resp.Body
	}

	if err := HandleAPIResponse(httpResponse, responseBody); err != nil {
		return err
	}

	// Parse the JSON response to get the presigned URL
	var downloadResp struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(responseBody, &downloadResp); err != nil {
		return fmt.Errorf("failed to parse download response: %w", err)
	}

	if downloadResp.URL == "" {
		return fmt.Errorf("no download URL in response")
	}

	// Determine output path
	outputPath := filesDownloadOutput
	if outputPath == "" {
		outputPath = filename
	}

	if err := downloadFileWithContext(ctx, downloadResp.URL, outputPath); err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}

	return PrintResult(fmt.Sprintf("File downloaded successfully: %s", outputPath), map[string]any{
		"filename": filename,
		"path":     outputPath,
		"success":  true,
	})
}
