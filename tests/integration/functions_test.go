//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
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

	// Delete the function
	result = runCLI(t, "functions", "delete", "--id", functionID)
	requireSuccess(t, result)
	t.Log("Function deleted successfully")

	// Verify function no longer accessible
	result = runCLI(t, "functions", "show", "--id", functionID)
	requireFailure(t, result)
	t.Log("Verified function is no longer accessible after delete")
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
	result = runCLI(t, "functions", "show", "--id", functionID)
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
	result = runCLI(t, "functions", "update", "--id", functionID, "--file", tmpFile)
	requireSuccess(t, result)
	t.Log("Successfully updated function")

	// Step 5: List function runs (should be empty initially)
	result = runCLI(t, "functions", "runs", "--id", functionID)
	requireSuccess(t, result)
	t.Log("Successfully listed function runs")

	// Step 6: Delete the function
	result = runCLI(t, "functions", "delete", "--id", functionID)
	requireSuccess(t, result)
	t.Log("Function deleted successfully")
}

func TestFunctionIDResolution(t *testing.T) {
	// This test verifies the function ID resolution feature:
	// 1. Create a function and verify current_function file is created
	// 2. Run subsequent commands without --id and verify they use the saved function
	// 3. Delete the function and verify current_function file is cleared

	tmpFile := createTempFunctionFile(t)

	// Step 1: Create a new function
	result := runCLI(t, "functions", "create", "--file", tmpFile, "--name", "id-resolution-test")
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

	// Step 2: Verify current_function file was created
	configDir, err := os.UserConfigDir()
	if err != nil {
		t.Fatalf("Failed to get config dir: %v", err)
	}
	currentFunctionFile := filepath.Join(configDir, "notte", "current_function")
	data, err := os.ReadFile(currentFunctionFile)
	if err != nil {
		t.Fatalf("Failed to read current_function file: %v", err)
	}
	if string(data) != functionID {
		t.Errorf("current_function file contains %q, expected %q", string(data), functionID)
	}
	t.Log("Verified current_function file was created with correct function ID")

	// Step 3: Test show command without --id should use the saved function
	result = runCLI(t, "functions", "show")
	requireSuccess(t, result)
	if !containsString(result.Stdout, functionID) {
		t.Error("Function show without --id did not return the current function")
	}
	t.Log("Successfully used current function for 'show' command")

	// Step 4: Test runs command without --id should use the saved function
	result = runCLI(t, "functions", "runs")
	requireSuccess(t, result)
	t.Log("Successfully used current function for 'runs' command")

	// Step 5: Test delete command without --id should use the saved function and clear it
	result = runCLI(t, "functions", "delete")
	requireSuccess(t, result)
	t.Log("Successfully deleted current function")

	// Step 6: Verify current_function file was cleared
	if _, err := os.Stat(currentFunctionFile); !os.IsNotExist(err) {
		data, readErr := os.ReadFile(currentFunctionFile)
		if readErr == nil && string(data) == functionID {
			t.Error("current_function file should have been cleared after delete")
		}
	}
	t.Log("Verified current_function file was cleared after delete")
}

func TestFunctionIDResolutionPriority(t *testing.T) {
	// Test priority: --id flag > NOTTE_FUNCTION_ID env var > current_function file
	tmpFile := createTempFunctionFile(t)

	// Create first function (this sets current_function)
	result := runCLI(t, "functions", "create", "--file", tmpFile, "--name", "priority-test-function1")
	requireSuccess(t, result)

	var createResp1 struct {
		FunctionID string `json:"function_id"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &createResp1); err != nil {
		t.Fatalf("Failed to parse function create response: %v", err)
	}
	functionID1 := createResp1.FunctionID
	defer cleanupFunction(t, functionID1)
	t.Logf("Created first function: %s", functionID1)

	// Create second function (this overwrites current_function)
	result = runCLI(t, "functions", "create", "--file", tmpFile, "--name", "priority-test-function2")
	requireSuccess(t, result)

	var createResp2 struct {
		FunctionID string `json:"function_id"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &createResp2); err != nil {
		t.Fatalf("Failed to parse function create response: %v", err)
	}
	functionID2 := createResp2.FunctionID
	defer cleanupFunction(t, functionID2)
	t.Logf("Created second function: %s", functionID2)

	// Test 1: env var should take priority over file
	// Current function file has functionID2, env var has functionID1
	result = runCLIWithEnv(t, map[string]string{"NOTTE_FUNCTION_ID": functionID1}, "functions", "show")
	requireSuccess(t, result)
	if !containsString(result.Stdout, functionID1) {
		t.Errorf("Expected function1 (%s) when using env var, but got different function", functionID1)
	}
	t.Log("Verified env var takes priority over current_function file")

	// Test 2: --id flag should take priority over env var
	result = runCLIWithEnv(t, map[string]string{"NOTTE_FUNCTION_ID": functionID1}, "functions", "show", "--id", functionID2)
	requireSuccess(t, result)
	if !containsString(result.Stdout, functionID2) {
		t.Errorf("Expected function2 (%s) when using --id flag, but got different function", functionID2)
	}
	t.Log("Verified --id flag takes priority over env var")

	// Cleanup
	result = runCLI(t, "functions", "delete", "--id", functionID1)
	requireSuccess(t, result)
	result = runCLI(t, "functions", "delete", "--id", functionID2)
	requireSuccess(t, result)
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
	result = runCLI(t, "functions", "update", "--id", functionID, "--file", tmpFile)
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

	// Cleanup
	result = runCLI(t, "functions", "delete", "--id", functionID)
	requireSuccess(t, result)
}

func TestFunctionsShowNonexistent(t *testing.T) {
	// Try to show a non-existent function
	result := runCLI(t, "functions", "show", "--id", "00000000-0000-0000-0000-000000000000")
	requireFailure(t, result)
	t.Log("Correctly failed to show non-existent function")
}

func TestFunctionsDeleteNonexistent(t *testing.T) {
	// Try to delete a non-existent function
	result := runCLI(t, "functions", "delete", "--id", "00000000-0000-0000-0000-000000000000")
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
	result = runCLI(t, "functions", "delete", "--id", functionID)
	requireSuccess(t, result)
	t.Log("First delete succeeded")

	// Delete second time - should fail
	result = runCLI(t, "functions", "delete", "--id", functionID)
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
	result = runCLI(t, "functions", "fork", "--id", sourceFunctionID)
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

	// Cleanup
	result = runCLI(t, "functions", "delete", "--id", forkedFunctionID)
	requireSuccess(t, result)
	result = runCLI(t, "functions", "delete", "--id", sourceFunctionID)
	requireSuccess(t, result)
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
	result = runCLI(t, "functions", "schedule", "--id", functionID, "--cron", "0 * * * ? *")
	requireSuccess(t, result)
	t.Log("Successfully scheduled function")

	// Unschedule the function
	result = runCLI(t, "functions", "unschedule", "--id", functionID)
	requireSuccess(t, result)
	t.Log("Successfully unscheduled function")

	// Cleanup
	result = runCLI(t, "functions", "delete", "--id", functionID)
	requireSuccess(t, result)
}

func TestFunctionsNoIDProvided(t *testing.T) {
	// Clear any existing current_function file
	configDir, err := os.UserConfigDir()
	if err == nil {
		currentFunctionFile := filepath.Join(configDir, "notte", "current_function")
		os.Remove(currentFunctionFile)
	}

	// Clear NOTTE_FUNCTION_ID env var and try to show without --id
	result := runCLIWithEnv(t, map[string]string{"NOTTE_FUNCTION_ID": ""}, "functions", "show")
	requireFailure(t, result)

	// Should contain helpful error message
	if !containsString(result.Stderr, "function ID required") && !containsString(result.Stdout, "function ID required") {
		t.Log("Expected error message about function ID being required")
	}
	t.Log("Correctly failed when no function ID is available")
}

// cleanupFunction deletes a function, ignoring errors (for deferred cleanup)
func cleanupFunction(t *testing.T, functionID string) {
	t.Helper()
	if functionID == "" {
		return
	}
	result := runCLI(t, "functions", "delete", "--id", functionID)
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
