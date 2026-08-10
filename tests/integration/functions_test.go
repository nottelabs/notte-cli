//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// validFunctionContent returns valid Python function code for testing
// Must contain a notte session to pass API validation
func validFunctionContent() string {
	return "def run(test: str = 'test'):\n\tprint(f'Hello, World! {test}')\n\tnotte.Session()\n"
}

// createTempFunctionFile creates a temporary file with valid function content
func createTempFunctionFile(t *testing.T) string {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "test-function-*.py")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	if _, err := tmpFile.WriteString(validFunctionContent()); err != nil {
		t.Fatalf("Failed to write function file: %v", err)
	}
	tmpFile.Close()
	t.Cleanup(func() { os.Remove(tmpFile.Name()) })
	return tmpFile.Name()
}

func TestFunctionsList(t *testing.T) {
	// List functions - this should always work, even if empty
	result := runCLI(t, "functions", "list")
	requireSuccess(t, result)
	t.Log("Successfully listed functions")

	// The API may return either a paginated response or an array directly
	// Try to parse as array first (common case)
	var items []struct {
		FunctionID    string `json:"function_id"`
		LatestVersion string `json:"latest_version"`
		Status        string `json:"status"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &items); err != nil {
		// Try paginated response format
		var listResp struct {
			Items []struct {
				FunctionID    string `json:"function_id"`
				LatestVersion string `json:"latest_version"`
				Status        string `json:"status"`
			} `json:"items"`
		}
		if err := json.Unmarshal([]byte(result.Stdout), &listResp); err != nil {
			t.Fatalf("Failed to parse list response: %v", err)
		}
		items = listResp.Items
	}
	t.Logf("Found %d functions", len(items))
}

func TestFunctionsCreateThenDelete(t *testing.T) {
	// Create a function and immediately delete it
	tmpFile := createTempFunctionFile(t)

	// Create function
	result := runCLI(t, "functions", "create", "--file", tmpFile, "--name", "test-create-delete", "--description", "Integration test function")
	requireSuccess(t, result)

	var createResp struct {
		FunctionID    string   `json:"function_id"`
		LatestVersion string   `json:"latest_version"`
		Status        string   `json:"status"`
		Name          string   `json:"name"`
		Description   string   `json:"description"`
		Versions      []string `json:"versions"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &createResp); err != nil {
		t.Fatalf("Failed to parse create response: %v", err)
	}

	functionID := createResp.FunctionID
	if functionID == "" {
		t.Fatal("No function ID returned from create command")
	}
	defer cleanupFunction(t, functionID)

	// Validate create response
	if createResp.Status != "active" {
		t.Errorf("Expected status 'active', got %q", createResp.Status)
	}
	if createResp.Name != "test-create-delete" {
		t.Errorf("Expected name 'test-create-delete', got %q", createResp.Name)
	}
	if createResp.Description != "Integration test function" {
		t.Errorf("Expected description 'Integration test function', got %q", createResp.Description)
	}
	if createResp.LatestVersion == "" {
		t.Error("Expected latest_version to be set")
	}
	if len(createResp.Versions) == 0 {
		t.Error("Expected versions to be non-empty")
	}
	t.Logf("Created function: %s (version: %s)", functionID, createResp.LatestVersion)

	t.Log("Function create then delete test completed successfully")
}

func TestFunctionSecretsLifecycle(t *testing.T) {
	secretName := "INTEGRATION_SECRET_" + strings.NewReplacer("-", "_", ":", "_", ".", "_").Replace(time.Now().UTC().Format(time.RFC3339Nano))
	secretValue := "integration-secret-value"

	result := runCLI(t, "function", "secrets", "set", secretName, secretValue)
	requireSuccess(t, result)

	var setResp struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &setResp); err != nil {
		t.Fatalf("Failed to parse secret set response: %v", err)
	}
	if setResp.ID == "" {
		t.Fatal("No secret ID returned from set command")
	}
	if setResp.Name != secretName {
		t.Errorf("Expected secret name %q, got %q", secretName, setResp.Name)
	}
	if setResp.Namespace != "function_env" {
		t.Errorf("Expected namespace function_env, got %q", setResp.Namespace)
	}
	deleted := false
	defer func() {
		if !deleted {
			result := runCLI(t, "function", "secrets", "delete", setResp.ID)
			if result.ExitCode != 0 {
				t.Logf("Warning: failed to cleanup function secret %s: %s", setResp.ID, result.Stderr)
			}
		}
	}()

	result = runCLI(t, "function", "secrets", "get", secretName)
	requireSuccess(t, result)
	var getResp struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &getResp); err != nil {
		t.Fatalf("Failed to parse secret get response: %v", err)
	}
	if getResp.Value != secretValue {
		t.Errorf("Expected secret value %q, got %q", secretValue, getResp.Value)
	}

	result = runCLI(t, "function", "secrets", "list")
	requireSuccess(t, result)
	if !containsString(result.Stdout, secretName) {
		t.Errorf("Function secrets list did not contain %q", secretName)
	}

	result = runCLI(t, "function", "secrets", "delete", setResp.ID)
	requireSuccess(t, result)
	deleted = true
	t.Log("Function secrets lifecycle test completed successfully")
}

func TestFunctionsLifecycle(t *testing.T) {
	tmpFile := createTempFunctionFile(t)

	// Step 1: Create a new function
	result := runCLI(t, "functions", "create", "--file", tmpFile, "--name", "lifecycle-test-function")
	requireSuccess(t, result)

	var createResp struct {
		FunctionID string `json:"function_id"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &createResp); err != nil {
		t.Fatalf("Failed to parse function create response: %v", err)
	}
	functionID := createResp.FunctionID
	if functionID == "" {
		t.Fatal("No function ID returned from create command")
	}
	t.Logf("Created function: %s", functionID)
	defer cleanupFunction(t, functionID)

	// Step 2: Show function details
	result = runCLI(t, "functions", "show", "--function-id", functionID)
	requireSuccess(t, result)
	if !containsString(result.Stdout, functionID) {
		t.Error("Function show did not contain function ID")
	}
	t.Log("Successfully retrieved function details")

	// Step 3: List functions - should include our function
	result = runCLI(t, "functions", "list")
	requireSuccess(t, result)
	if !containsString(result.Stdout, functionID) {
		t.Error("Function list did not contain our function")
	}
	t.Log("Function appears in list")

	// Step 4: Update function with new code
	result = runCLI(t, "functions", "update", "--function-id", functionID, "--file", tmpFile)
	requireSuccess(t, result)
	t.Log("Successfully updated function")

	// Step 5: List function runs (should be empty initially)
	result = runCLI(t, "functions", "runs", "--function-id", functionID)
	requireSuccess(t, result)
	t.Log("Function lifecycle test completed successfully")
}

func TestFunctionsUpdate(t *testing.T) {
	tmpFile := createTempFunctionFile(t)

	// Create function
	result := runCLI(t, "functions", "create", "--file", tmpFile, "--name", "update-test-function")
	requireSuccess(t, result)

	var createResp struct {
		FunctionID    string `json:"function_id"`
		LatestVersion string `json:"latest_version"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &createResp); err != nil {
		t.Fatalf("Failed to parse create response: %v", err)
	}
	functionID := createResp.FunctionID
	originalVersion := createResp.LatestVersion
	defer cleanupFunction(t, functionID)
	t.Logf("Created function: %s (version: %s)", functionID, originalVersion)

	// Update function
	result = runCLI(t, "functions", "update", "--function-id", functionID, "--file", tmpFile)
	requireSuccess(t, result)

	var updateResp struct {
		FunctionID    string   `json:"function_id"`
		LatestVersion string   `json:"latest_version"`
		Versions      []string `json:"versions"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &updateResp); err != nil {
		t.Fatalf("Failed to parse update response: %v", err)
	}

	// Version should have changed or versions list should have grown
	if len(updateResp.Versions) < 1 {
		t.Error("Expected at least one version after update")
	}
	t.Logf("Updated function: %s (new version: %s, total versions: %d)", functionID, updateResp.LatestVersion, len(updateResp.Versions))
}

func TestFunctionsShowNonexistent(t *testing.T) {
	// Try to show a non-existent function
	result := runCLI(t, "functions", "show", "--function-id", "00000000-0000-0000-0000-000000000000")
	requireFailure(t, result)
	t.Log("Correctly failed to show non-existent function")
}

func TestFunctionsDeleteNonexistent(t *testing.T) {
	// Try to delete a non-existent function
	result := runCLI(t, "functions", "delete", "--function-id", "00000000-0000-0000-0000-000000000000")
	requireFailure(t, result)
	t.Log("Correctly failed to delete non-existent function")
}

func TestFunctionsDeleteAlreadyDeleted(t *testing.T) {
	tmpFile := createTempFunctionFile(t)

	// Create function
	result := runCLI(t, "functions", "create", "--file", tmpFile, "--name", "double-delete-test")
	requireSuccess(t, result)

	var createResp struct {
		FunctionID string `json:"function_id"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &createResp); err != nil {
		t.Fatalf("Failed to parse create response: %v", err)
	}
	functionID := createResp.FunctionID

	// Delete first time - should succeed
	result = runCLI(t, "functions", "delete", "--function-id", functionID)
	requireSuccess(t, result)
	t.Log("First delete succeeded")

	// Delete second time - should fail
	result = runCLI(t, "functions", "delete", "--function-id", functionID)
	requireFailure(t, result)
	t.Log("Correctly failed on second delete (already deleted)")
}

func TestFunctionsCreateInvalidFile(t *testing.T) {
	// Create a temp file with invalid extension
	tmpFile, err := os.CreateTemp("", "test-function-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	if _, err := tmpFile.WriteString("invalid content"); err != nil {
		t.Fatalf("Failed to write function file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	// Try to create function with invalid file
	result := runCLI(t, "functions", "create", "--file", tmpFile.Name(), "--name", "invalid-file-test")
	requireFailure(t, result)
	t.Log("Correctly rejected invalid file type")
}

func TestFunctionsFork(t *testing.T) {
	tmpFile := createTempFunctionFile(t)

	// Create a shared function
	result := runCLI(t, "functions", "create", "--file", tmpFile, "--name", "fork-source-function", "--shared")
	requireSuccess(t, result)

	var createResp struct {
		FunctionID string `json:"function_id"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &createResp); err != nil {
		t.Fatalf("Failed to parse create response: %v", err)
	}
	sourceFunctionID := createResp.FunctionID
	defer cleanupFunction(t, sourceFunctionID)
	t.Logf("Created source function: %s", sourceFunctionID)

	// Fork the function
	result = runCLI(t, "functions", "fork", "--function-id", sourceFunctionID)
	requireSuccess(t, result)

	var forkResp struct {
		FunctionID          string `json:"function_id"`
		ReferenceFunctionID string `json:"reference_function_id"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &forkResp); err != nil {
		t.Fatalf("Failed to parse fork response: %v", err)
	}

	forkedFunctionID := forkResp.FunctionID
	defer cleanupFunction(t, forkedFunctionID)

	if forkedFunctionID == "" {
		t.Fatal("No function ID returned from fork")
	}
	if forkedFunctionID == sourceFunctionID {
		t.Error("Forked function should have different ID than source")
	}
	t.Logf("Forked function: %s (from: %s)", forkedFunctionID, sourceFunctionID)
}

func TestFunctionsScheduleAndUnschedule(t *testing.T) {
	tmpFile := createTempFunctionFile(t)

	// Create function
	result := runCLI(t, "functions", "create", "--file", tmpFile, "--name", "schedule-test-function")
	requireSuccess(t, result)

	var createResp struct {
		FunctionID string `json:"function_id"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &createResp); err != nil {
		t.Fatalf("Failed to parse create response: %v", err)
	}
	functionID := createResp.FunctionID
	defer cleanupFunction(t, functionID)
	t.Logf("Created function: %s", functionID)

	// Schedule the function
	result = runCLI(t, "functions", "schedule", "--function-id", functionID, "--cron", "0 * * * ? *")
	requireSuccess(t, result)
	t.Log("Successfully scheduled function")

	// Unschedule the function
	result = runCLI(t, "functions", "unschedule", "--function-id", functionID)
	requireSuccess(t, result)
	t.Log("Successfully unscheduled function")
}

func TestFunctionsNoIDProvided(t *testing.T) {
	// An environment variable must not substitute for the required flag.
	result := runCLIWithEnv(t, map[string]string{"NOTTE_FUNCTION_ID": "fn_ignored"}, "functions", "show")
	requireFailure(t, result)

	// Should contain helpful error message
	if !containsString(result.Stderr, "function-id") && !containsString(result.Stdout, "function-id") {
		t.Fatalf("expected missing --function-id error, stdout=%q stderr=%q", result.Stdout, result.Stderr)
	}
	t.Log("Correctly rejected an implicit function ID")
}

func TestFunctionRun(t *testing.T) {
	tmpFile := createTempFunctionFile(t)

	// Create function
	result := runCLI(t, "functions", "create", "--file", tmpFile, "--name", "run-test-function")
	requireSuccess(t, result)

	var createResp struct {
		FunctionID string `json:"function_id"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &createResp); err != nil {
		t.Fatalf("Failed to parse create response: %v", err)
	}
	functionID := createResp.FunctionID
	defer cleanupFunction(t, functionID)
	t.Logf("Created function: %s", functionID)

	// Run the function
	result = runCLI(t, "functions", "run", "--function-id", functionID)
	requireSuccess(t, result)

	// Parse the run response to verify it contains expected fields
	var runResp map[string]interface{}
	if err := json.Unmarshal([]byte(result.Stdout), &runResp); err != nil {
		t.Fatalf("Failed to parse run response: %v", err)
	}

	t.Logf("Function run response: %+v", runResp)
	t.Log("Successfully ran function")
}

func TestFunctionRunWithVariables(t *testing.T) {
	tmpFile := createTempFunctionFile(t)

	// Create function
	result := runCLI(t, "functions", "create", "--file", tmpFile, "--name", "run-test-function-with-vars")
	requireSuccess(t, result)

	var createResp struct {
		FunctionID string `json:"function_id"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &createResp); err != nil {
		t.Fatalf("Failed to parse create response: %v", err)
	}
	functionID := createResp.FunctionID
	defer cleanupFunction(t, functionID)
	t.Logf("Created function: %s", functionID)

	// Test 1: Run with --var flag
	result = runCLI(t, "functions", "run", "--function-id", functionID, "--var", "test=mytest_variable")
	requireSuccess(t, result)

	var runResp map[string]interface{}
	if err := json.Unmarshal([]byte(result.Stdout), &runResp); err != nil {
		t.Fatalf("Failed to parse run response: %v", err)
	}
	t.Logf("Function run response with --var: %+v", runResp)

	// Test 2: Run with --vars JSON
	result = runCLI(t, "functions", "run", "--function-id", functionID, "--vars", `{"test":"json_variable"}`)
	requireSuccess(t, result)

	if err := json.Unmarshal([]byte(result.Stdout), &runResp); err != nil {
		t.Fatalf("Failed to parse run response: %v", err)
	}
	t.Logf("Function run response with --vars: %+v", runResp)

	// Test 3: Run with multiple --var flags
	result = runCLI(t, "functions", "run", "--function-id", functionID, "--var", "test=first", "--var", "another=second")
	requireSuccess(t, result)

	if err := json.Unmarshal([]byte(result.Stdout), &runResp); err != nil {
		t.Fatalf("Failed to parse run response: %v", err)
	}
	t.Logf("Function run response with multiple vars: %+v", runResp)

	t.Log("Successfully ran function with variables")
}

// cleanupFunction deletes a function, ignoring errors (for deferred cleanup)
func cleanupFunction(t *testing.T, functionID string) {
	t.Helper()
	if functionID == "" {
		return
	}
	result := runCLI(t, "functions", "delete", "--function-id", functionID)
	if result.ExitCode != 0 {
		t.Logf("Warning: failed to cleanup function %s: %s", functionID, result.Stderr)
	}
}

// runCLIWithEnv executes the notte CLI with additional environment variables
func runCLIWithEnv(t *testing.T, env map[string]string, args ...string) CLIResult {
	t.Helper()
	return runCLIWithEnvAndTimeout(t, env, 60*time.Second, args...)
}

// runCLIWithEnvAndTimeout executes the notte CLI with additional environment variables and custom timeout
func runCLIWithEnvAndTimeout(t *testing.T, env map[string]string, timeout time.Duration, args ...string) CLIResult {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Add -o json flag for machine-readable output and --yes to skip confirmations
	fullArgs := append([]string{"-o", "json", "--yes"}, args...)

	cmd := exec.CommandContext(ctx, "go", append([]string{"run", "./cmd/notte"}, fullArgs...)...)
	cmd.Dir = getProjectRoot()

	// Set environment variables
	cmd.Env = append(os.Environ(),
		"NOTTE_API_KEY="+os.Getenv("NOTTE_API_KEY"),
	)
	if apiURL := os.Getenv("NOTTE_API_URL"); apiURL != "" {
		cmd.Env = append(cmd.Env, "NOTTE_API_URL="+apiURL)
	}
	// Add custom env vars
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else if ctx.Err() == context.DeadlineExceeded {
			t.Logf("Command timed out after %v", timeout)
			exitCode = -1
		} else {
			exitCode = -1
		}
	}

	return CLIResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}
}
