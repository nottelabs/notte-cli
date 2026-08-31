package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/api"
	"github.com/nottelabs/notte-cli/internal/config"
)

var (
	functionID               string
	functionRunID            string
	functionMetadataJSON     string
	functionRunVariables     []string // Variables as key=value pairs
	functionRunVariablesJSON string   // Variables as JSON string
	functionRunNoStream      bool     // Opt out of log streaming
	functionSecretValue      string
)

// GetCurrentFunctionID returns the function ID from flag, env var, or file (in priority order)
func GetCurrentFunctionID() string {
	// 1. Check --function-id flag (already in functionID variable if set)
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
		return errors.New("function ID required: use --function-id flag, set NOTTE_FUNCTION_ID env var, or create a function first")
	}
	return nil
}

var functionsCmd = &cobra.Command{
	Use:     "functions",
	Aliases: []string{"function"},
	Short:   "Manage functions",
	Long:    "List, create, and operate on functions.",
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

var functionsConfigureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Set usage notes and self-healing",
	Long: "Update a function's metadata.\n\n" +
		"--run-instructions is documentation for whoever calls the function - how long a " +
		"run takes, what each variable is for, which sites it trips over. It is not " +
		"input to the self-healing agent.\n\n" +
		"Only the flags you pass are sent, so setting instructions leaves self-healing " +
		"as it was, and vice versa.",
	Args: cobra.NoArgs,
	RunE: runFunctionConfigure,
}

var functionsRollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Restore an earlier version of the function",
	Args:  cobra.NoArgs,
	RunE:  runFunctionRollback,
}

var functionsHealthCmd = &cobra.Command{
	Use:   "health",
	Short: "Show the function runtime's health and available packages",
	Args:  cobra.NoArgs,
	RunE:  runFunctionHealth,
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
  notte functions run-metadata-update --function-id <function-id> --run-id <run-id> --data '{"key": "value"}'

  # From file
  notte functions run-metadata-update --function-id <function-id> --run-id <run-id> --data @metadata.json

  # From stdin
  echo '{"key": "value"}' | notte functions run-metadata-update --function-id <function-id> --run-id <run-id>`,
	RunE:   runFunctionRunMetadataUpdate,
	Hidden: true,
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

var functionSecretsCmd = &cobra.Command{
	Use:   "secrets",
	Short: "Manage function environment secrets",
}

var functionSecretsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List function environment secrets",
	Args:  cobra.NoArgs,
	RunE:  runFunctionSecretsList,
}

var functionSecretsGetCmd = &cobra.Command{
	Use:   "get <name>",
	Short: "Get a function environment secret value",
	Args:  cobra.ExactArgs(1),
	RunE:  runFunctionSecretsGet,
}

var functionSecretsSetCmd = &cobra.Command{
	Use:   "set <name> [value]",
	Short: "Set a function environment secret",
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runFunctionSecretsSet,
}

var functionSecretsDeleteCmd = &cobra.Command{
	Use:     "delete <secret-id>",
	Aliases: []string{"rm"},
	Short:   "Delete a function environment secret",
	Args:    cobra.ExactArgs(1),
	RunE:    runFunctionSecretsDelete,
}

func init() {
	rootCmd.AddCommand(functionsCmd)
	functionsCmd.AddCommand(functionsListCmd)
	registerPaginationFlags(functionsListCmd)
	registerFilterFlag(functionsListCmd, flagIncludeDeleted, "", "Include deleted functions")

	functionsCmd.AddCommand(functionsCreateCmd)
	functionsCmd.AddCommand(functionsShowCmd)
	functionsCmd.AddCommand(functionsUpdateCmd)
	functionsCmd.AddCommand(functionsConfigureCmd)
	functionsCmd.AddCommand(functionsRollbackCmd)
	functionsCmd.AddCommand(functionsHealthCmd)
	functionsCmd.AddCommand(functionsDeleteCmd)
	functionsCmd.AddCommand(functionsRunCmd)
	functionsCmd.AddCommand(functionsRunsCmd)
	registerPaginationFlags(functionsRunsCmd)
	registerFilterFlag(functionsRunsCmd, flagRunning, "", "Only show runs that are still executing")

	functionsCmd.AddCommand(functionsForkCmd)
	functionsCmd.AddCommand(functionsRunStopCmd)
	functionsCmd.AddCommand(functionsRunMetadataCmd)
	functionsCmd.AddCommand(functionsRunMetadataUpdateCmd)
	functionsCmd.AddCommand(functionsScheduleCmd)
	functionsCmd.AddCommand(functionsUnscheduleCmd)
	functionsCmd.AddCommand(functionSecretsCmd)
	functionSecretsCmd.AddCommand(functionSecretsListCmd)
	functionSecretsCmd.AddCommand(functionSecretsGetCmd)
	functionSecretsCmd.AddCommand(functionSecretsSetCmd)
	functionSecretsCmd.AddCommand(functionSecretsDeleteCmd)

	// Create command flags
	RegisterFunctionCreateFlags(functionsCreateCmd)

	// Show command flags
	functionsShowCmd.Flags().StringVar(&functionID, "function-id", "", "Function ID (uses current function if not specified)")

	// Update command flags
	functionsUpdateCmd.Flags().StringVar(&functionID, "function-id", "", "Function ID (uses current function if not specified)")
	RegisterFunctionUpdateFlags(functionsUpdateCmd)

	// Configure command flags
	functionsConfigureCmd.Flags().StringVar(&functionID, "function-id", "", "Function ID (uses current function if not specified)")
	RegisterFunctionConfigureFlags(functionsConfigureCmd)

	// Rollback command flags
	functionsRollbackCmd.Flags().StringVar(&functionID, "function-id", "", "Function ID (uses current function if not specified)")
	RegisterFunctionRollbackFlags(functionsRollbackCmd)
	_ = functionsRollbackCmd.MarkFlagRequired("version")

	// Delete command flags
	functionsDeleteCmd.Flags().StringVar(&functionID, "function-id", "", "Function ID (uses current function if not specified)")

	// Run command flags
	functionsRunCmd.Flags().StringVar(&functionID, "function-id", "", "Function ID (uses current function if not specified)")
	functionsRunCmd.Flags().StringArrayVar(&functionRunVariables, "var", []string{}, "Variable as key=value pair (can be used multiple times)")
	functionsRunCmd.Flags().StringVar(&functionRunVariablesJSON, "vars", "", "Variables as JSON object string")
	// An opt-out rather than a --stream toggle: the API streams by default, so
	// a positive flag would be an opt-in to something already on. Same shape,
	// and the same reasoning, as --no-solve-captchas on sessions start.
	functionsRunCmd.Flags().BoolVar(&functionRunNoStream, "no-stream", false, "Return only the final response instead of streaming logs")

	// Runs command flags
	functionsRunsCmd.Flags().StringVar(&functionID, "function-id", "", "Function ID (uses current function if not specified)")

	// Fork command flags
	functionsForkCmd.Flags().StringVar(&functionID, "function-id", "", "Function ID (uses current function if not specified)")

	// Run-stop command flags
	functionsRunStopCmd.Flags().StringVar(&functionID, "function-id", "", "Function ID (uses current function if not specified)")
	functionsRunStopCmd.Flags().StringVar(&functionRunID, "run-id", "", "Run ID (required)")
	_ = functionsRunStopCmd.MarkFlagRequired("run-id")

	// Run-metadata command flags
	functionsRunMetadataCmd.Flags().StringVar(&functionID, "function-id", "", "Function ID (uses current function if not specified)")
	functionsRunMetadataCmd.Flags().StringVar(&functionRunID, "run-id", "", "Run ID (required)")
	_ = functionsRunMetadataCmd.MarkFlagRequired("run-id")

	// Run-metadata-update command flags
	functionsRunMetadataUpdateCmd.Flags().StringVar(&functionID, "function-id", "", "Function ID (uses current function if not specified)")
	functionsRunMetadataUpdateCmd.Flags().StringVar(&functionRunID, "run-id", "", "Run ID (required)")
	_ = functionsRunMetadataUpdateCmd.MarkFlagRequired("run-id")
	functionsRunMetadataUpdateCmd.Flags().StringVar(&functionMetadataJSON, "data", "", "JSON metadata, @file, or '-' for stdin")

	// Schedule command flags
	functionsScheduleCmd.Flags().StringVar(&functionID, "function-id", "", "Function ID (uses current function if not specified)")
	RegisterFunctionScheduleSetFlags(functionsScheduleCmd)
	_ = functionsScheduleCmd.MarkFlagRequired("cron")

	// Unschedule command flags
	functionsUnscheduleCmd.Flags().StringVar(&functionID, "function-id", "", "Function ID (uses current function if not specified)")

	// Function secrets command flags
	functionSecretsSetCmd.Flags().StringVar(&functionSecretValue, "value", "", "Secret value")
}

func runFunctionsList(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	page, err := getPageFlag(cmd)
	if err != nil {
		return err
	}
	pageSize, err := getPageSizeFlag(cmd)
	if err != nil {
		return err
	}
	params := &api.ListFunctionsParams{
		Page:     page,
		PageSize: pageSize,
	}
	params.OnlyActive = resolveOnlyActive(cmd, flagIncludeDeleted, true)
	resp, err := client.Client().ListFunctionsWithResponse(ctx, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	formatter := GetFormatter()

	var items []api.FunctionListItemResponse
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

	body, contentType, err := BuildFunctionCreateBody(cmd)
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.FunctionCreateParams{}
	resp, err := client.Client().FunctionCreateWithBodyWithResponse(
		ctx,
		params,
		contentType,
		body,
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

	body, contentType, err := BuildFunctionUpdateBody(cmd)
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.FunctionUpdateParams{}
	resp, err := client.Client().FunctionUpdateWithBodyWithResponse(
		ctx,
		functionID,
		params,
		contentType,
		body,
	)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

func runFunctionConfigure(cmd *cobra.Command, args []string) error {
	if err := RequireFunctionID(); err != nil {
		return err
	}

	// An empty PATCH is accepted by the API and changes nothing, which reads as
	// success for a command that did not do what the caller meant.
	if !cmd.Flags().Changed("run-instructions") && !cmd.Flags().Changed("self-healing") {
		return errors.New("nothing to configure: pass --run-instructions, --self-healing, or both")
	}
	// `--run-instructions ""` is refused rather than sent. The generated builder
	// omits an empty string, so it would otherwise travel as far as an empty
	// PATCH: accepted, 200, nothing changed, and the caller told it worked.
	if cmd.Flags().Changed("run-instructions") && FunctionConfigureInstructions == "" {
		return errors.New("--run-instructions cannot be empty")
	}

	client, err := GetClient()
	if err != nil {
		return err
	}

	body, err := BuildFunctionConfigureRequest(cmd)
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.FunctionMetadataUpdateParams{}
	resp, err := client.Client().FunctionMetadataUpdateWithResponse(ctx, functionID, params, *body)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

func runFunctionRollback(cmd *cobra.Command, args []string) error {
	if err := RequireFunctionID(); err != nil {
		return err
	}

	client, err := GetClient()
	if err != nil {
		return err
	}

	body, err := BuildFunctionRollbackRequest(cmd)
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.FunctionRollbackParams{}
	resp, err := client.Client().FunctionRollbackWithResponse(ctx, functionID, params, *body)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

// runFunctionHealth reports on the runtime functions execute in: which Python
// it is, what is installed, and whether it is reachable at all. Not generated —
// a GET has no request body, which is all the generator reads.
func runFunctionHealth(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.FunctionRuntimeHealthParams{}
	resp, err := client.Client().FunctionRuntimeHealthWithResponse(ctx, params)
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

	// Parse variables
	variables := make(map[string]interface{})

	// First, parse JSON variables if provided
	if functionRunVariablesJSON != "" {
		if err := json.Unmarshal([]byte(functionRunVariablesJSON), &variables); err != nil {
			return fmt.Errorf("failed to parse --vars JSON: %w", err)
		}
	}

	// Then, parse key=value pairs (these override JSON if there's a conflict)
	for _, kv := range functionRunVariables {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid variable format %q: expected key=value", kv)
		}
		variables[parts[0]] = parts[1]
	}

	// The generated client doesn't support request body for FunctionRunStart,
	// so we need to make a manual request with the function_id in the body
	requestBody := map[string]interface{}{
		"function_id": functionID,
		"variables":   variables,
	}
	// Sent only when asked. The API's own default is true, so transmitting it
	// unconditionally would freeze today's server-side behaviour into the client.
	if functionRunNoStream {
		requestBody["stream"] = false
	}

	bodyJSON, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	url := fmt.Sprintf("%s/functions/%s/runs/start", client.BaseURL(), functionID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyJSON))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-notte-api-key", client.APIKey())

	httpResp, err := client.HTTPClient().Do(req)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	defer func() { _ = httpResp.Body.Close() }()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if err := HandleAPIResponse(httpResp, body); err != nil {
		return err
	}

	// Parse and print the response
	var result interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	return GetFormatter().Print(result)
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

	page, err := getPageFlag(cmd)
	if err != nil {
		return err
	}
	pageSize, err := getPageSizeFlag(cmd)
	if err != nil {
		return err
	}
	params := &api.ListFunctionRunsByFunctionIdParams{
		Page:     page,
		PageSize: pageSize,
	}
	// Runs default to the full history: "active" here means still executing, so
	// filtering by it would make run history permanently empty.
	params.OnlyActive = resolveOnlyActive(cmd, flagRunning, false)
	resp, err := client.Client().ListFunctionRunsByFunctionIdWithResponse(ctx, functionID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	var items []api.FunctionRunListItemResponse
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

	body, err := BuildFunctionScheduleSetRequest(cmd)
	if err != nil {
		return err
	}
	// Supplied here rather than generated: `variables` is required by the
	// schema and is a free-form object, which no flag can express, so it is
	// skipped in the generator (see CommandSkippedFields) and defaulted to the
	// empty map the API expects.
	emptyVars := make(map[string]interface{})
	body.Variables = &emptyVars

	params := &api.FunctionScheduleSetParams{}
	resp, err := client.Client().FunctionScheduleSetWithResponse(ctx, functionID, params, *body)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	return PrintResult(fmt.Sprintf("Function %s scheduled with cron expression: %s", functionID, FunctionScheduleSetCron), map[string]any{
		"id":   functionID,
		"cron": FunctionScheduleSetCron,
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

func functionSecretsNamespace() api.SecretNamespace {
	return api.FunctionEnv
}

func runFunctionSecretsList(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	namespace := functionSecretsNamespace()
	params := &api.ListSecretsParams{Namespace: &namespace}
	resp, err := client.Client().ListSecretsWithResponse(ctx, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	var items []api.SecretMetadata
	if resp.JSON200 != nil {
		items = resp.JSON200.Items
	}
	if printed, err := PrintListOrEmpty(items, "No function environment secrets found."); err != nil {
		return err
	} else if printed {
		return nil
	}

	return GetFormatter().Print(items)
}

func runFunctionSecretsGet(cmd *cobra.Command, args []string) error {
	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	params := &api.GetSecretParams{Namespace: functionSecretsNamespace()}
	resp, err := client.Client().GetSecretWithResponse(ctx, args[0], params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON200)
}

func runFunctionSecretsSet(cmd *cobra.Command, args []string) error {
	value := functionSecretValue
	if len(args) == 2 {
		value = args[1]
	}
	if value == "" && !cmd.Flags().Changed("value") {
		return fmt.Errorf("secret value required: pass it as an argument or with --value")
	}

	client, err := GetClient()
	if err != nil {
		return err
	}

	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	body := api.StoreSecretJSONRequestBody{
		Name:      args[0],
		Namespace: functionSecretsNamespace(),
		Value:     value,
	}
	params := &api.StoreSecretParams{}
	resp, err := client.Client().StoreSecretWithResponse(ctx, params, body)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	return GetFormatter().Print(resp.JSON201)
}

func runFunctionSecretsDelete(cmd *cobra.Command, args []string) error {
	secretID := args[0]

	confirmed, err := ConfirmAction("function environment secret", secretID)
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

	params := &api.DeleteSecretParams{}
	resp, err := client.Client().DeleteSecretWithResponse(ctx, secretID, params)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}

	if err := HandleAPIResponse(resp.HTTPResponse, resp.Body); err != nil {
		return err
	}

	return PrintResult(fmt.Sprintf("Function environment secret %s deleted.", secretID), map[string]any{
		"id":     secretID,
		"status": "deleted",
	})
}
