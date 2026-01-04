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
	workflowsCreateFile        string
	workflowsCreateName        string
	workflowsCreateDescription string
	workflowsCreateShared      bool
)

var workflowsCmd = &cobra.Command{
	Use:   "workflows",
	Short: "Manage workflows",
	Long:  "List and create workflows.",
}

var workflowsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List workflows",
	RunE:  runWorkflowsList,
}

var workflowsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new workflow",
	RunE:  runWorkflowsCreate,
}

func init() {
	rootCmd.AddCommand(workflowsCmd)
	workflowsCmd.AddCommand(workflowsListCmd)
	workflowsCmd.AddCommand(workflowsCreateCmd)

	workflowsCreateCmd.Flags().StringVar(&workflowsCreateFile, "file", "", "Path to workflow file (required)")
	_ = workflowsCreateCmd.MarkFlagRequired("file")
	workflowsCreateCmd.Flags().StringVar(&workflowsCreateName, "name", "", "Workflow name")
	workflowsCreateCmd.Flags().StringVar(&workflowsCreateDescription, "description", "", "Workflow description")
	workflowsCreateCmd.Flags().BoolVar(&workflowsCreateShared, "shared", false, "Make workflow public")
}

func runWorkflowsList(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.ListWorkflowsParams{}
	resp, err := client.Client().ListWorkflowsWithResponse(ctx, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	formatter := GetFormatter()

	if resp.JSON200 == nil || len(resp.JSON200.Items) == 0 {
		fmt.Println("No workflows found.")
		return nil
	}

	return formatter.Print(resp.JSON200.Items)
}

func runWorkflowsCreate(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	// Open the workflow file
	file, err := os.Open(workflowsCreateFile)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Create multipart form data
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add file field
	part, err := writer.CreateFormFile("file", filepath.Base(workflowsCreateFile))
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return fmt.Errorf("failed to copy file data: %w", err)
	}

	// Add optional fields
	if workflowsCreateName != "" {
		if err := writer.WriteField("name", workflowsCreateName); err != nil {
			return fmt.Errorf("failed to write name field: %w", err)
		}
	}
	if workflowsCreateDescription != "" {
		if err := writer.WriteField("description", workflowsCreateDescription); err != nil {
			return fmt.Errorf("failed to write description field: %w", err)
		}
	}
	if cmd.Flags().Changed("shared") {
		if err := writer.WriteField("shared", fmt.Sprintf("%t", workflowsCreateShared)); err != nil {
			return fmt.Errorf("failed to write shared field: %w", err)
		}
	}

	_ = writer.Close()

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.WorkflowCreateParams{}
	resp, err := client.Client().WorkflowCreateWithBodyWithResponse(
		ctx,
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
	return formatter.Print(resp.JSON200)
}
