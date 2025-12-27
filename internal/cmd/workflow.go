package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"

	"github.com/salmonumbrella/notte-cli/internal/api"
	"github.com/spf13/cobra"
)

var (
	workflowID         string
	workflowUpdateFile string
	runID              string
	metadataJSON       string
	cronExpression     string
)

var workflowCmd = &cobra.Command{
	Use:   "workflow",
	Short: "Operate on a specific workflow",
	Long:  "Subcommands for interacting with a specific workflow. Use --id flag to specify the workflow ID.",
}

var workflowShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show workflow details",
	Args:  cobra.NoArgs,
	RunE:  runWorkflowShow,
}

var workflowUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update workflow",
	Args:  cobra.NoArgs,
	RunE:  runWorkflowUpdate,
}

var workflowDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete workflow",
	Args:  cobra.NoArgs,
	RunE:  runWorkflowDelete,
}

var workflowRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the workflow",
	Args:  cobra.NoArgs,
	RunE:  runWorkflowRun,
}

var workflowRunsCmd = &cobra.Command{
	Use:   "runs",
	Short: "List workflow runs",
	Args:  cobra.NoArgs,
	RunE:  runWorkflowRuns,
}

var workflowForkCmd = &cobra.Command{
	Use:   "fork",
	Short: "Fork/duplicate the workflow",
	Args:  cobra.NoArgs,
	RunE:  runWorkflowFork,
}

var workflowRunStopCmd = &cobra.Command{
	Use:   "run-stop",
	Short: "Stop a workflow run",
	Args:  cobra.NoArgs,
	RunE:  runWorkflowRunStop,
}

var workflowRunMetadataCmd = &cobra.Command{
	Use:   "run-metadata",
	Short: "Get workflow run metadata",
	Args:  cobra.NoArgs,
	RunE:  runWorkflowRunMetadata,
}

var workflowRunMetadataUpdateCmd = &cobra.Command{
	Use:   "run-metadata-update",
	Short: "Update workflow run metadata",
	Args:  cobra.NoArgs,
	RunE:  runWorkflowRunMetadataUpdate,
}

var workflowScheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "Set a cron schedule for the workflow",
	Args:  cobra.NoArgs,
	RunE:  runWorkflowSchedule,
}

var workflowUnscheduleCmd = &cobra.Command{
	Use:   "unschedule",
	Short: "Remove the schedule from the workflow",
	Args:  cobra.NoArgs,
	RunE:  runWorkflowUnschedule,
}

func init() {
	rootCmd.AddCommand(workflowCmd)

	workflowCmd.AddCommand(workflowShowCmd)
	workflowCmd.AddCommand(workflowUpdateCmd)
	workflowCmd.AddCommand(workflowDeleteCmd)
	workflowCmd.AddCommand(workflowRunCmd)
	workflowCmd.AddCommand(workflowRunsCmd)
	workflowCmd.AddCommand(workflowForkCmd)
	workflowCmd.AddCommand(workflowRunStopCmd)
	workflowCmd.AddCommand(workflowRunMetadataCmd)
	workflowCmd.AddCommand(workflowRunMetadataUpdateCmd)
	workflowCmd.AddCommand(workflowScheduleCmd)
	workflowCmd.AddCommand(workflowUnscheduleCmd)

	// Persistent flag for workflow ID (required for all subcommands)
	workflowCmd.PersistentFlags().StringVar(&workflowID, "id", "", "Workflow ID (required)")
	_ = workflowCmd.MarkPersistentFlagRequired("id")

	workflowUpdateCmd.Flags().StringVar(&workflowUpdateFile, "file", "", "Path to updated workflow file (required)")
	_ = workflowUpdateCmd.MarkFlagRequired("file")

	// Flags for run control commands
	workflowRunStopCmd.Flags().StringVar(&runID, "run-id", "", "Run ID (required)")
	_ = workflowRunStopCmd.MarkFlagRequired("run-id")

	workflowRunMetadataCmd.Flags().StringVar(&runID, "run-id", "", "Run ID (required)")
	_ = workflowRunMetadataCmd.MarkFlagRequired("run-id")

	workflowRunMetadataUpdateCmd.Flags().StringVar(&runID, "run-id", "", "Run ID (required)")
	_ = workflowRunMetadataUpdateCmd.MarkFlagRequired("run-id")
	workflowRunMetadataUpdateCmd.Flags().StringVar(&metadataJSON, "data", "", "JSON metadata to update (required)")
	_ = workflowRunMetadataUpdateCmd.MarkFlagRequired("data")

	workflowScheduleCmd.Flags().StringVar(&cronExpression, "cron", "", "Cron expression (required)")
	_ = workflowScheduleCmd.MarkFlagRequired("cron")
}

func runWorkflowShow(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.WorkflowDownloadUrlParams{}
	resp, err := client.Client().WorkflowDownloadUrlWithResponse(ctx, workflowID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

func runWorkflowUpdate(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	// Open the workflow file
	file, err := os.Open(workflowUpdateFile)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Create multipart form data
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add file field
	part, err := writer.CreateFormFile("file", filepath.Base(workflowUpdateFile))
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return fmt.Errorf("failed to copy file data: %w", err)
	}

	_ = writer.Close()

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.WorkflowUpdateParams{}
	resp, err := client.Client().WorkflowUpdateWithBodyWithResponse(
		ctx,
		workflowID,
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

	return GetFormatter().Print(resp.JSON200)
}

func runWorkflowDelete(cmd *cobra.Command, args []string) error {
	confirmed, err := ConfirmAction("workflow", workflowID)
	if err != nil {
		return err
	}
	if !confirmed {
		fmt.Println("Cancelled.")
		return nil
	}

	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.WorkflowDeleteParams{}
	resp, err := client.Client().WorkflowDeleteWithResponse(ctx, workflowID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	fmt.Printf("Workflow %s deleted.\n", workflowID)
	return nil
}

func runWorkflowRun(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.WorkflowRunStartParams{}
	resp, err := client.Client().WorkflowRunStartWithResponse(ctx, workflowID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

func runWorkflowRuns(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.ListWorkflowRunsByWorkflowIdParams{}
	resp, err := client.Client().ListWorkflowRunsByWorkflowIdWithResponse(ctx, workflowID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	if resp.JSON200 == nil || len(resp.JSON200.Items) == 0 {
		fmt.Println("No workflow runs found.")
		return nil
	}

	return GetFormatter().Print(resp.JSON200.Items)
}

func runWorkflowFork(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.WorkflowForkParams{}
	resp, err := client.Client().WorkflowForkWithResponse(ctx, workflowID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

func runWorkflowRunStop(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.WorkflowRunStopParams{}
	resp, err := client.Client().WorkflowRunStopWithResponse(ctx, workflowID, runID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

func runWorkflowRunMetadata(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.WorkflowRunGetMetadataParams{}
	resp, err := client.Client().WorkflowRunGetMetadataWithResponse(ctx, workflowID, runID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

func runWorkflowRunMetadataUpdate(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	// Parse the JSON metadata
	var metadata api.WorkflowRunUpdateMetadataJSONRequestBody
	if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil {
		return fmt.Errorf("failed to parse JSON metadata: %w", err)
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.WorkflowRunUpdateMetadataParams{}
	resp, err := client.Client().WorkflowRunUpdateMetadataWithResponse(ctx, workflowID, runID, params, metadata)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

func runWorkflowSchedule(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	body := api.WorkflowScheduleCreateRequest{
		Cron: cronExpression,
	}

	params := &api.WorkflowScheduleSetParams{}
	resp, err := client.Client().WorkflowScheduleSetWithResponse(ctx, workflowID, params, body)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	fmt.Printf("Workflow %s scheduled with cron expression: %s\n", workflowID, cronExpression)
	return nil
}

func runWorkflowUnschedule(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.WorkflowScheduleDeleteParams{}
	resp, err := client.Client().WorkflowScheduleDeleteWithResponse(ctx, workflowID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse); err != nil {
		return err
	}

	fmt.Printf("Workflow %s schedule removed.\n", workflowID)
	return nil
}
