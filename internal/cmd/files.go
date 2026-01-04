package cmd

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/salmonumbrella/notte-cli/internal/api"
)

var (
	filesListUploadsFlag   bool
	filesListDownloadsFlag bool
	filesDownloadSession   string
	filesDownloadOutput    string
)

var filesCmd = &cobra.Command{
	Use:   "files",
	Short: "Manage uploaded files",
	Long:  "Upload, list, and download files from notte.cc storage.",
}

var filesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List uploaded files",
	Long: `List files in storage. Use --uploads to list uploaded files,
or --downloads with --session to list downloaded files from a session.`,
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
	Long:  "Download a file from a session by its filename.",
	Args:  cobra.ExactArgs(1),
	RunE:  runFilesDownload,
}

func init() {
	rootCmd.AddCommand(filesCmd)
	filesCmd.AddCommand(filesListCmd)
	filesCmd.AddCommand(filesUploadCmd)
	filesCmd.AddCommand(filesDownloadCmd)

	// List command flags
	filesListCmd.Flags().BoolVar(&filesListUploadsFlag, "uploads", true, "List uploaded files")
	filesListCmd.Flags().BoolVar(&filesListDownloadsFlag, "downloads", false, "List downloaded files from a session")
	filesListCmd.Flags().StringVar(&filesDownloadSession, "session", "", "Session ID (required for --downloads)")

	// Download command flags
	filesDownloadCmd.Flags().StringVar(&filesDownloadSession, "session", "", "Session ID (required)")
	filesDownloadCmd.Flags().StringVarP(&filesDownloadOutput, "output", "o", "", "Output file path (defaults to current directory)")
	_ = filesDownloadCmd.MarkFlagRequired("session")
}

func runFilesList(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	formatter := GetFormatter()

	// If downloads flag is set, list downloads for a session
	if filesListDownloadsFlag {
		if filesDownloadSession == "" {
			return fmt.Errorf("--session is required when using --downloads")
		}

		ctx, cancel := GetContextWithTimeout(cmd.Context())
		defer cancel()

		params := &api.FileListDownloadsParams{}
		resp, err := client.Client().FileListDownloadsWithResponse(ctx, filesDownloadSession, params)
		if err != nil {
			return fmt.Errorf("API request failed: %w", err)
		}

		if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
			return err
		}

		if resp.JSON200 == nil || len(resp.JSON200.Files) == 0 {
			fmt.Println("No downloaded files in session.")
			return nil
		}

		return formatter.Print(resp.JSON200.Files)
	}

	// Default: list uploads
	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.FileListUploadsParams{}
	resp, err := client.Client().FileListUploadsWithResponse(ctx, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	if resp.JSON200 == nil || len(resp.JSON200.Files) == 0 {
		fmt.Println("No uploaded files.")
		return nil
	}

	return formatter.Print(resp.JSON200.Files)
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

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	formatter := GetFormatter()
	if resp.JSON200 != nil && resp.JSON200.Success {
		fmt.Printf("File uploaded successfully: %s\n", filename)
		return nil
	}

	return formatter.Print(resp.JSON200)
}

func runFilesDownload(cmd *cobra.Command, args []string) error {
	filename := args[0]

	if filesDownloadSession == "" {
		return fmt.Errorf("--session is required")
	}

	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.FileDownloadParams{}
	resp, err := client.Client().FileDownloadWithResponse(
		ctx,
		filesDownloadSession,
		filename,
		params,
	)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	// Determine output path
	outputPath := filesDownloadOutput
	if outputPath == "" {
		outputPath = filename
	}

	// Write the file
	err = os.WriteFile(outputPath, resp.Body, 0o644)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Printf("File downloaded successfully: %s\n", outputPath)
	return nil
}
