package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/salmonumbrella/notte-cli/internal/api"
	"github.com/salmonumbrella/notte-cli/internal/config"
)

var (
	functionsCreateFile        string
	functionsCreateName        string
	functionsCreateDescription string
	functionsCreateShared      bool
)

var (
	functionID             string
	functionUpdateFile     string
	functionRunID          string
	functionMetadataJSON   string
	functionCronExpression string
)

// GetCurrentFunctionID returns the function ID from flag, env var, or file (in priority order)
func GetCurrentFunctionID() string {
	// 1. Check --id flag (already in functionID variable if set)
	if functionID != "" {
		return functionID
	}

	// 2. Check NOTTE_FUNCTION_ID env var
	if envID := os.Getenv(config.EnvFunctionID); envID != "" {
		return envID
	}

	// 3. Check current_function file
	configDir, err := config.Dir()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(configDir, config.CurrentFunctionFile))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// setCurrentFunction saves the function ID to the current_function file
func setCurrentFunction(id string) error {
	configDir, err := config.Dir()
	if err != nil {
		return err
	}
	// Ensure directory exists
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(configDir, config.CurrentFunctionFile), []byte(id), 0o600)
}

// clearCurrentFunction removes the current_function file
func clearCurrentFunction() error {
	configDir, err := config.Dir()
	if err != nil {
		return err
	}
	path := filepath.Join(configDir, config.CurrentFunctionFile)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// RequireFunctionID ensures a function ID is available from flag, env, or file
func RequireFunctionID() error {
	functionID = GetCurrentFunctionID()
	if functionID == "" {
		return errors.New("function ID required: use --id flag, set NOTTE_FUNCTION_ID env var, or create a function first")
	}
	return nil
}

var functionsCmd = &cobra.Command{
	Use:   "functions",
	Short: "Manage functions",
	Long:  "List, create, and operate on functions.",
}

var functionsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List functions",
	RunE:  runFunctionsList,
}

var functionsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new function",
	RunE:  runFunctionsCreate,
}

var functionsShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show function details",
	Args:  cobra.NoArgs,
	RunE:  runFunctionShow,
}

var functionsUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update function",
	Args:  cobra.NoArgs,
	RunE:  runFunctionUpdate,
}

var functionsDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete function",
	Args:  cobra.NoArgs,
	RunE:  runFunctionDelete,
}

var functionsRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the function",
	Args:  cobra.NoArgs,
	RunE:  runFunctionRun,
}

var functionsRunsCmd = &cobra.Command{
	Use:   "runs",
	Short: "List function runs",
	Args:  cobra.NoArgs,
	RunE:  runFunctionRuns,
}

var functionsForkCmd = &cobra.Command{
	Use:   "fork",
	Short: "Fork/duplicate the function",
	Args:  cobra.NoArgs,
	RunE:  runFunctionFork,
}

var functionsRunStopCmd = &cobra.Command{
	Use:   "run-stop",
	Short: "Stop a function run",
	Args:  cobra.NoArgs,
	RunE:  runFunctionRunStop,
}

var functionsRunMetadataCmd = &cobra.Command{
	Use:   "run-metadata",
	Short: "Get function run metadata",
	Args:  cobra.NoArgs,
	RunE:  runFunctionRunMetadata,
}

var functionsRunMetadataUpdateCmd = &cobra.Command{
	Use:   "run-metadata-update",
	Short: "Update function run metadata",
	Args:  cobra.NoArgs,
	Example: `  # Direct JSON
  notte functions run-metadata-update --id <function-id> --run-id <run-id> --data '{"key": "value"}'

  # From file
  notte functions run-metadata-update --id <function-id> --run-id <run-id> --data @metadata.json

  # From stdin
  echo '{"key": "value"}' | notte functions run-metadata-update --id <function-id> --run-id <run-id>`,
	RunE: runFunctionRunMetadataUpdate,
}

var functionsScheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "Set a cron schedule for the function",
	Args:  cobra.NoArgs,
	RunE:  runFunctionSchedule,
}

var functionsUnscheduleCmd = &cobra.Command{
	Use:   "unschedule",
	Short: "Remove the schedule from the function",
	Args:  cobra.NoArgs,
	RunE:  runFunctionUnschedule,
}

func init() {
	rootCmd.AddCommand(functionsCmd)
	functionsCmd.AddCommand(functionsListCmd)
	functionsCmd.AddCommand(functionsCreateCmd)
	functionsCmd.AddCommand(functionsShowCmd)
	functionsCmd.AddCommand(functionsUpdateCmd)
	functionsCmd.AddCommand(functionsDeleteCmd)
	functionsCmd.AddCommand(functionsRunCmd)
	functionsCmd.AddCommand(functionsRunsCmd)
	functionsCmd.AddCommand(functionsForkCmd)
	functionsCmd.AddCommand(functionsRunStopCmd)
	functionsCmd.AddCommand(functionsRunMetadataCmd)
	functionsCmd.AddCommand(functionsRunMetadataUpdateCmd)
	functionsCmd.AddCommand(functionsScheduleCmd)
	functionsCmd.AddCommand(functionsUnscheduleCmd)

	// Create command flags
	functionsCreateCmd.Flags().StringVar(&functionsCreateFile, "file", "", "Path to function file (required)")
	_ = functionsCreateCmd.MarkFlagRequired("file")
	functionsCreateCmd.Flags().StringVar(&functionsCreateName, "name", "", "Function name")
	functionsCreateCmd.Flags().StringVar(&functionsCreateDescription, "description", "", "Function description")
	functionsCreateCmd.Flags().BoolVar(&functionsCreateShared, "shared", false, "Make function public")

	// Show command flags
	functionsShowCmd.Flags().StringVar(&functionID, "id", "", "Function ID (uses current function if not specified)")

	// Update command flags
	functionsUpdateCmd.Flags().StringVar(&functionID, "id", "", "Function ID (uses current function if not specified)")
	functionsUpdateCmd.Flags().StringVar(&functionUpdateFile, "file", "", "Path to updated function file (required)")
	_ = functionsUpdateCmd.MarkFlagRequired("file")

	// Delete command flags
	functionsDeleteCmd.Flags().StringVar(&functionID, "id", "", "Function ID (uses current function if not specified)")

	// Run command flags
	functionsRunCmd.Flags().StringVar(&functionID, "id", "", "Function ID (uses current function if not specified)")

	// Runs command flags
	functionsRunsCmd.Flags().StringVar(&functionID, "id", "", "Function ID (uses current function if not specified)")

	// Fork command flags
	functionsForkCmd.Flags().StringVar(&functionID, "id", "", "Function ID (uses current function if not specified)")

	// Run-stop command flags
	functionsRunStopCmd.Flags().StringVar(&functionID, "id", "", "Function ID (uses current function if not specified)")
	functionsRunStopCmd.Flags().StringVar(&functionRunID, "run-id", "", "Run ID (required)")
	_ = functionsRunStopCmd.MarkFlagRequired("run-id")

	// Run-metadata command flags
	functionsRunMetadataCmd.Flags().StringVar(&functionID, "id", "", "Function ID (uses current function if not specified)")
	functionsRunMetadataCmd.Flags().StringVar(&functionRunID, "run-id", "", "Run ID (required)")
	_ = functionsRunMetadataCmd.MarkFlagRequired("run-id")

	// Run-metadata-update command flags
	functionsRunMetadataUpdateCmd.Flags().StringVar(&functionID, "id", "", "Function ID (uses current function if not specified)")
	functionsRunMetadataUpdateCmd.Flags().StringVar(&functionRunID, "run-id", "", "Run ID (required)")
	_ = functionsRunMetadataUpdateCmd.MarkFlagRequired("run-id")
	functionsRunMetadataUpdateCmd.Flags().StringVar(&functionMetadataJSON, "data", "", "JSON metadata, @file, or '-' for stdin")

	// Schedule command flags
	functionsScheduleCmd.Flags().StringVar(&functionID, "id", "", "Function ID (uses current function if not specified)")
	functionsScheduleCmd.Flags().StringVar(&functionCronExpression, "cron", "", "Cron expression (required)")
	_ = functionsScheduleCmd.MarkFlagRequired("cron")

	// Unschedule command flags
	functionsUnscheduleCmd.Flags().StringVar(&functionID, "id", "", "Function ID (uses current function if not specified)")
}

func runFunctionsList(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.ListFunctionsParams{}
	resp, err := client.Client().ListFunctionsWithResponse(ctx, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	formatter := GetFormatter()

	var items []api.GetFunctionResponse
	if resp.JSON200 != nil {
		items = resp.JSON200.Items
	}
	if printed, err := PrintListOrEmpty(items, "No functions found."); err != nil {
		return err
	} else if printed {
		return nil
	}

	return formatter.Print(items)
}

func runFunctionsCreate(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	// Open the function file
	file, err := os.Open(functionsCreateFile)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Create multipart form data
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add file field
	part, err := writer.CreateFormFile("file", filepath.Base(functionsCreateFile))
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return fmt.Errorf("failed to copy file data: %w", err)
	}

	// Add optional fields
	if functionsCreateName != "" {
		if err := writer.WriteField("name", functionsCreateName); err != nil {
			return fmt.Errorf("failed to write name field: %w", err)
		}
	}
	if functionsCreateDescription != "" {
		if err := writer.WriteField("description", functionsCreateDescription); err != nil {
			return fmt.Errorf("failed to write description field: %w", err)
		}
	}
	if cmd.Flags().Changed("shared") {
		if err := writer.WriteField("shared", fmt.Sprintf("%t", functionsCreateShared)); err != nil {
			return fmt.Errorf("failed to write shared field: %w", err)
		}
	}

	_ = writer.Close()

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.FunctionCreateParams{}
	resp, err := client.Client().FunctionCreateWithBodyWithResponse(
		ctx,
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

	// Save function ID as current function
	if resp.JSON200 != nil && resp.JSON200.FunctionId != "" {
		if err := setCurrentFunction(resp.JSON200.FunctionId); err != nil {
			PrintInfo(fmt.Sprintf("Warning: could not save current function: %v", err))
		}
	}

	formatter := GetFormatter()
	return formatter.Print(resp.JSON200)
}

func runFunctionShow(cmd *cobra.Command, args []string) error {
	if err := RequireFunctionID(); err != nil {
		return err
	}

	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.FunctionDownloadUrlParams{}
	resp, err := client.Client().FunctionDownloadUrlWithResponse(ctx, functionID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

func runFunctionUpdate(cmd *cobra.Command, args []string) error {
	if err := RequireFunctionID(); err != nil {
		return err
	}

	client, err := GetClient()
	if err != nil {
		return err
	}

	// Open the function file
	file, err := os.Open(functionUpdateFile)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Create multipart form data
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add file field
	part, err := writer.CreateFormFile("file", filepath.Base(functionUpdateFile))
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return fmt.Errorf("failed to copy file data: %w", err)
	}

	_ = writer.Close()

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.FunctionUpdateParams{}
	resp, err := client.Client().FunctionUpdateWithBodyWithResponse(
		ctx,
		functionID,
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

	return GetFormatter().Print(resp.JSON200)
}

func runFunctionDelete(cmd *cobra.Command, args []string) error {
	if err := RequireFunctionID(); err != nil {
		return err
	}

	confirmed, err := ConfirmAction("function", functionID)
	if err != nil {
		return err
	}
	if !confirmed {
		return PrintResult("Cancelled.", map[string]any{"cancelled": true})
	}

	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.FunctionDeleteParams{}
	resp, err := client.Client().FunctionDeleteWithResponse(ctx, functionID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	// Clear current function only if it matches the deleted function
	configDir, _ := config.Dir()
	if configDir != "" {
		data, _ := os.ReadFile(filepath.Join(configDir, config.CurrentFunctionFile))
		if strings.TrimSpace(string(data)) == functionID {
			_ = clearCurrentFunction()
		}
	}

	return PrintResult(fmt.Sprintf("Function %s deleted.", functionID), map[string]any{
		"id":     functionID,
		"status": "deleted",
	})
}

func runFunctionRun(cmd *cobra.Command, args []string) error {
	if err := RequireFunctionID(); err != nil {
		return err
	}

	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.FunctionRunStartParams{}
	resp, err := client.Client().FunctionRunStartWithResponse(ctx, functionID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

func runFunctionRuns(cmd *cobra.Command, args []string) error {
	if err := RequireFunctionID(); err != nil {
		return err
	}

	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.ListFunctionRunsByFunctionIdParams{}
	resp, err := client.Client().ListFunctionRunsByFunctionIdWithResponse(ctx, functionID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	var items []api.GetFunctionRunResponse
	if resp.JSON200 != nil {
		items = resp.JSON200.Items
	}
	if printed, err := PrintListOrEmpty(items, "No function runs found."); err != nil {
		return err
	} else if printed {
		return nil
	}

	return GetFormatter().Print(items)
}

func runFunctionFork(cmd *cobra.Command, args []string) error {
	if err := RequireFunctionID(); err != nil {
		return err
	}

	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.FunctionForkParams{}
	resp, err := client.Client().FunctionForkWithResponse(ctx, functionID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

func runFunctionRunStop(cmd *cobra.Command, args []string) error {
	if err := RequireFunctionID(); err != nil {
		return err
	}

	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.FunctionRunStopParams{}
	resp, err := client.Client().FunctionRunStopWithResponse(ctx, functionID, functionRunID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

func runFunctionRunMetadata(cmd *cobra.Command, args []string) error {
	if err := RequireFunctionID(); err != nil {
		return err
	}

	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.FunctionRunGetMetadataParams{}
	resp, err := client.Client().FunctionRunGetMetadataWithResponse(ctx, functionID, functionRunID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

func runFunctionRunMetadataUpdate(cmd *cobra.Command, args []string) error {
	if err := RequireFunctionID(); err != nil {
		return err
	}

	client, err := GetClient()
	if err != nil {
		return err
	}

	metadataPayload, err := readJSONInput(cmd, functionMetadataJSON, "data")
	if err != nil {
		return err
	}

	// Parse the JSON metadata
	var metadata api.FunctionRunUpdateMetadataJSONRequestBody
	if err := json.Unmarshal(metadataPayload, &metadata); err != nil {
		return fmt.Errorf("failed to parse JSON metadata: %w", err)
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.FunctionRunUpdateMetadataParams{}
	resp, err := client.Client().FunctionRunUpdateMetadataWithResponse(ctx, functionID, functionRunID, params, metadata)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

func runFunctionSchedule(cmd *cobra.Command, args []string) error {
	if err := RequireFunctionID(); err != nil {
		return err
	}

	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	body := api.FunctionScheduleSetJSONRequestBody{
		Cron: functionCronExpression,
	}

	params := &api.FunctionScheduleSetParams{}
	resp, err := client.Client().FunctionScheduleSetWithResponse(ctx, functionID, params, body)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	return PrintResult(fmt.Sprintf("Function %s scheduled with cron expression: %s", functionID, functionCronExpression), map[string]any{
		"id":   functionID,
		"cron": functionCronExpression,
	})
}

func runFunctionUnschedule(cmd *cobra.Command, args []string) error {
	if err := RequireFunctionID(); err != nil {
		return err
	}

	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.FunctionScheduleDeleteParams{}
	resp, err := client.Client().FunctionScheduleDeleteWithResponse(ctx, functionID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	return PrintResult(fmt.Sprintf("Function %s schedule removed.", functionID), map[string]any{
		"id":     functionID,
		"status": "unscheduled",
	})
}
