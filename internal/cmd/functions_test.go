package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/config"
	"github.com/nottelabs/notte-cli/internal/testutil"
)

const (
	functionIDTest    = "fn_123"
	functionRunIDTest = "run_123"
)

func setupFunctionTest(t *testing.T) *testutil.MockServer {
	t.Helper()
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	t.Cleanup(func() { server.Close() })
	env.SetEnv("NOTTE_API_URL", server.URL())

	origFunctionID := functionID
	origRunID := functionRunID
	functionID = functionIDTest
	functionRunID = functionRunIDTest
	t.Cleanup(func() {
		functionID = origFunctionID
		functionRunID = origRunID
	})

	return server
}

func functionJSON() string {
	return `{"function_id":"` + functionIDTest + `","latest_version":"1","status":"active","created_at":"2020-01-01T00:00:00Z","updated_at":"2020-01-01T00:00:00Z","versions":["1"]}`
}

func functionWithLinkJSON() string {
	return `{"function_id":"` + functionIDTest + `","latest_version":"1","status":"active","created_at":"2020-01-01T00:00:00Z","updated_at":"2020-01-01T00:00:00Z","versions":["1"],"url":"https://example.com/function.json"}`
}

func functionRunJSON() string {
	return `{"function_id":"` + functionIDTest + `","function_run_id":"` + functionRunIDTest + `","status":"RUNNING","created_at":"2020-01-01T00:00:00Z","updated_at":"2020-01-01T00:00:00Z"}`
}

func updateFunctionRunJSON() string {
	return `{"function_id":"` + functionIDTest + `","function_run_id":"` + functionRunIDTest + `","updated_at":"2020-01-01T00:00:00Z","status":"STOPPED"}`
}

func TestRunFunctionsList_Success(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	server.AddResponse("/functions", 200, `{"items":[{"function_id":"fn_1","latest_version":"1","status":"active","created_at":"2020-01-01T00:00:00Z","updated_at":"2020-01-01T00:00:00Z","versions":["1"]}]}`)

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runFunctionsList(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunFunctionsList_Empty(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	server.AddResponse("/functions", 200, `{"items":[]}`)

	origFormat := outputFormat
	outputFormat = "text"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runFunctionsList(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(stdout, "No functions found.") {
		t.Errorf("expected empty message, got %q", stdout)
	}
}

func TestRunFunctionsCreate_Success(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	server.AddResponse("/functions", 200, `{"function_id":"fn_1","latest_version":"1","status":"active","created_at":"2020-01-01T00:00:00Z","updated_at":"2020-01-01T00:00:00Z","versions":["1"]}`)

	tmpFile, err := os.CreateTemp("", "function-*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	if _, err := tmpFile.WriteString(`{"steps":[]}`); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("failed to close temp file: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(tmpFile.Name()) })

	origFile := functionsCreateFile
	origName := functionsCreateName
	origDesc := functionsCreateDescription
	origShared := functionsCreateShared
	t.Cleanup(func() {
		functionsCreateFile = origFile
		functionsCreateName = origName
		functionsCreateDescription = origDesc
		functionsCreateShared = origShared
	})

	functionsCreateFile = tmpFile.Name()
	functionsCreateName = "Test Function"
	functionsCreateDescription = "Test description"

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.Flags().BoolVar(&functionsCreateShared, "shared", false, "")
	_ = cmd.Flags().Set("shared", "true")
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runFunctionsCreate(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunFunctionsCreate_MissingFile(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	origFile := functionsCreateFile
	functionsCreateFile = "missing-function.json"
	t.Cleanup(func() { functionsCreateFile = origFile })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	err := runFunctionsCreate(cmd, nil)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "failed to open file") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunFunctionShow(t *testing.T) {
	server := setupFunctionTest(t)
	server.AddResponse("/functions/"+functionIDTest, 200, functionWithLinkJSON())

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runFunctionShow(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunFunctionUpdate(t *testing.T) {
	server := setupFunctionTest(t)
	server.AddResponse("/functions/"+functionIDTest, 200, functionJSON())

	tmpFile, err := os.CreateTemp("", "function-update-*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	if _, err := tmpFile.WriteString(`{"steps":[]}`); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("failed to close temp file: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(tmpFile.Name()) })

	origFile := functionUpdateFile
	functionUpdateFile = tmpFile.Name()
	t.Cleanup(func() { functionUpdateFile = origFile })

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runFunctionUpdate(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunFunctionDelete(t *testing.T) {
	server := setupFunctionTest(t)
	server.AddResponse("/functions/"+functionIDTest, 200, `{"message":"deleted","status":"deleted"}`)

	SetSkipConfirmation(true)
	t.Cleanup(func() { SetSkipConfirmation(false) })

	origFormat := outputFormat
	outputFormat = "text"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runFunctionDelete(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(stdout, "deleted") {
		t.Errorf("expected delete message, got %q", stdout)
	}
}

func TestRunFunctionRun(t *testing.T) {
	server := setupFunctionTest(t)
	server.AddResponse("/functions/"+functionIDTest+"/runs/start", 200, `{"run_id":"`+functionRunIDTest+`"}`)

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runFunctionRun(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunFunctionRuns(t *testing.T) {
	server := setupFunctionTest(t)
	server.AddResponse("/functions/"+functionIDTest+"/runs", 200, `{"items":[`+functionRunJSON()+`]}`)

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runFunctionRuns(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunFunctionRuns_Empty(t *testing.T) {
	server := setupFunctionTest(t)
	server.AddResponse("/functions/"+functionIDTest+"/runs", 200, `{"items":[]}`)

	origFormat := outputFormat
	outputFormat = "text"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runFunctionRuns(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(stdout, "No function runs found.") {
		t.Errorf("expected empty message, got %q", stdout)
	}
}

func TestRunFunctionFork(t *testing.T) {
	server := setupFunctionTest(t)
	server.AddResponse("/functions/"+functionIDTest+"/fork", 200, functionJSON())

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runFunctionFork(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunFunctionRunStop(t *testing.T) {
	server := setupFunctionTest(t)
	server.AddResponse("/functions/"+functionIDTest+"/runs/"+functionRunIDTest, 200, updateFunctionRunJSON())

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runFunctionRunStop(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunFunctionRunMetadata(t *testing.T) {
	server := setupFunctionTest(t)
	server.AddResponse("/functions/"+functionIDTest+"/runs/"+functionRunIDTest, 200, functionRunJSON())

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runFunctionRunMetadata(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunFunctionRunMetadataUpdate(t *testing.T) {
	server := setupFunctionTest(t)
	server.AddResponse("/functions/"+functionIDTest+"/runs/"+functionRunIDTest, 200, updateFunctionRunJSON())

	origMetadata := functionMetadataJSON
	functionMetadataJSON = `{"result":{"ok":true},"status":"DONE"}`
	t.Cleanup(func() { functionMetadataJSON = origMetadata })

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runFunctionRunMetadataUpdate(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}

func TestRunFunctionSchedule(t *testing.T) {
	server := setupFunctionTest(t)
	server.AddResponse("/functions/"+functionIDTest+"/schedule", 200, `{"status":"scheduled"}`)

	origCron := functionCronExpression
	functionCronExpression = "0 0 * * *"
	t.Cleanup(func() { functionCronExpression = origCron })

	origFormat := outputFormat
	outputFormat = "text"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runFunctionSchedule(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(stdout, "scheduled") {
		t.Errorf("expected schedule message, got %q", stdout)
	}
}

func TestRunFunctionUnschedule(t *testing.T) {
	server := setupFunctionTest(t)
	server.AddResponse("/functions/"+functionIDTest+"/schedule", 200, `{"status":"removed"}`)

	origFormat := outputFormat
	outputFormat = "text"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runFunctionUnschedule(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(stdout, "schedule removed") {
		t.Errorf("expected unschedule message, got %q", stdout)
	}
}

// Tests for function ID resolution (file-based tracking)

func setupFunctionFileTest(t *testing.T) string {
	t.Helper()

	// Create a temporary config directory
	tmpDir := t.TempDir()
	config.SetTestConfigDir(tmpDir)
	t.Cleanup(func() { config.SetTestConfigDir("") })

	return tmpDir
}

func TestGetCurrentFunctionID_FromFlag(t *testing.T) {
	origID := functionID
	functionID = "flag_function"
	t.Cleanup(func() { functionID = origID })

	got := GetCurrentFunctionID()
	if got != "flag_function" {
		t.Errorf("GetCurrentFunctionID() = %q, want %q", got, "flag_function")
	}
}

func TestGetCurrentFunctionID_FromEnvVar(t *testing.T) {
	origID := functionID
	functionID = ""
	t.Cleanup(func() { functionID = origID })

	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_FUNCTION_ID", "env_function")

	got := GetCurrentFunctionID()
	if got != "env_function" {
		t.Errorf("GetCurrentFunctionID() = %q, want %q", got, "env_function")
	}
}

func TestGetCurrentFunctionID_FromFile(t *testing.T) {
	origID := functionID
	functionID = ""
	t.Cleanup(func() { functionID = origID })

	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_FUNCTION_ID", "") // Ensure env var is empty

	// Create temp config dir
	tmpDir := setupFunctionFileTest(t)

	// Write function file
	configDir := filepath.Join(tmpDir, config.ConfigDirName)
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	functionFile := filepath.Join(configDir, config.CurrentFunctionFile)
	if err := os.WriteFile(functionFile, []byte("file_function"), 0o600); err != nil {
		t.Fatalf("failed to write function file: %v", err)
	}

	got := GetCurrentFunctionID()
	if got != "file_function" {
		t.Errorf("GetCurrentFunctionID() = %q, want %q", got, "file_function")
	}
}

func TestGetCurrentFunctionID_Priority(t *testing.T) {
	origID := functionID
	t.Cleanup(func() { functionID = origID })

	env := testutil.SetupTestEnv(t)
	tmpDir := setupFunctionFileTest(t)

	// Create function file
	configDir := filepath.Join(tmpDir, config.ConfigDirName)
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	functionFile := filepath.Join(configDir, config.CurrentFunctionFile)
	if err := os.WriteFile(functionFile, []byte("file_function"), 0o600); err != nil {
		t.Fatalf("failed to write function file: %v", err)
	}

	// Test: flag > env > file
	functionID = "flag_function"
	env.SetEnv("NOTTE_FUNCTION_ID", "env_function")

	got := GetCurrentFunctionID()
	if got != "flag_function" {
		t.Errorf("flag should have highest priority: got %q, want %q", got, "flag_function")
	}

	// Test: env > file
	functionID = ""
	got = GetCurrentFunctionID()
	if got != "env_function" {
		t.Errorf("env should have priority over file: got %q, want %q", got, "env_function")
	}

	// Test: file as fallback
	env.SetEnv("NOTTE_FUNCTION_ID", "")
	got = GetCurrentFunctionID()
	if got != "file_function" {
		t.Errorf("file should be fallback: got %q, want %q", got, "file_function")
	}
}

func TestSetCurrentFunction(t *testing.T) {
	tmpDir := setupFunctionFileTest(t)

	err := setCurrentFunction("test_function_id")
	if err != nil {
		t.Fatalf("setCurrentFunction() error = %v", err)
	}

	// Verify file was created
	configDir := filepath.Join(tmpDir, config.ConfigDirName)
	functionFile := filepath.Join(configDir, config.CurrentFunctionFile)

	data, err := os.ReadFile(functionFile)
	if err != nil {
		t.Fatalf("failed to read function file: %v", err)
	}

	if string(data) != "test_function_id" {
		t.Errorf("function file content = %q, want %q", string(data), "test_function_id")
	}
}

func TestClearCurrentFunction(t *testing.T) {
	tmpDir := setupFunctionFileTest(t)

	// First create a function file
	configDir := filepath.Join(tmpDir, config.ConfigDirName)
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	functionFile := filepath.Join(configDir, config.CurrentFunctionFile)
	if err := os.WriteFile(functionFile, []byte("test_function"), 0o600); err != nil {
		t.Fatalf("failed to write function file: %v", err)
	}

	// Clear it
	err := clearCurrentFunction()
	if err != nil {
		t.Fatalf("clearCurrentFunction() error = %v", err)
	}

	// Verify file was removed
	if _, err := os.Stat(functionFile); !os.IsNotExist(err) {
		t.Error("function file should have been removed")
	}
}

func TestClearCurrentFunction_NoFile(t *testing.T) {
	_ = setupFunctionFileTest(t)

	// Should not error when file doesn't exist
	err := clearCurrentFunction()
	if err != nil {
		t.Errorf("clearCurrentFunction() should not error when file doesn't exist: %v", err)
	}
}

func TestRequireFunctionID_NoFunction(t *testing.T) {
	origID := functionID
	functionID = ""
	t.Cleanup(func() { functionID = origID })

	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_FUNCTION_ID", "")
	_ = setupFunctionFileTest(t)

	err := RequireFunctionID()
	if err == nil {
		t.Fatal("RequireFunctionID() should error when no function ID available")
	}

	expectedMsg := "function ID required"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("error message should contain %q, got %q", expectedMsg, err.Error())
	}
}

func TestRequireFunctionID_FromFile(t *testing.T) {
	origID := functionID
	functionID = ""
	t.Cleanup(func() { functionID = origID })

	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_FUNCTION_ID", "")
	tmpDir := setupFunctionFileTest(t)

	// Create function file
	configDir := filepath.Join(tmpDir, config.ConfigDirName)
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	functionFile := filepath.Join(configDir, config.CurrentFunctionFile)
	if err := os.WriteFile(functionFile, []byte("file_function"), 0o600); err != nil {
		t.Fatalf("failed to write function file: %v", err)
	}

	err := RequireFunctionID()
	if err != nil {
		t.Fatalf("RequireFunctionID() error = %v", err)
	}

	if functionID != "file_function" {
		t.Errorf("functionID = %q, want %q", functionID, "file_function")
	}
}

func TestFunctionsCreate_SetsCurrentFunction(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	tmpDir := setupFunctionFileTest(t)

	server.AddResponse("/functions", 200, `{"function_id":"fn_new_123","latest_version":"1","status":"active","created_at":"2020-01-01T00:00:00Z","updated_at":"2020-01-01T00:00:00Z","versions":["1"]}`)

	tmpFile, err := os.CreateTemp("", "function-*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	if _, err := tmpFile.WriteString(`{"steps":[]}`); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("failed to close temp file: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(tmpFile.Name()) })

	origFile := functionsCreateFile
	origName := functionsCreateName
	origDesc := functionsCreateDescription
	t.Cleanup(func() {
		functionsCreateFile = origFile
		functionsCreateName = origName
		functionsCreateDescription = origDesc
	})

	functionsCreateFile = tmpFile.Name()
	functionsCreateName = "Test Function"
	functionsCreateDescription = "Test description"

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.Flags().BoolVar(&functionsCreateShared, "shared", false, "")
	cmd.SetContext(context.Background())

	testutil.CaptureOutput(func() {
		err := runFunctionsCreate(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// Verify function was saved to file
	configDir := filepath.Join(tmpDir, config.ConfigDirName)
	functionFile := filepath.Join(configDir, config.CurrentFunctionFile)

	data, err := os.ReadFile(functionFile)
	if err != nil {
		t.Fatalf("failed to read function file: %v", err)
	}

	if string(data) != "fn_new_123" {
		t.Errorf("function file content = %q, want %q", string(data), "fn_new_123")
	}
}

func TestFunctionDelete_ClearsCurrentFunction(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	tmpDir := setupFunctionFileTest(t)

	// Create function file first
	configDir := filepath.Join(tmpDir, config.ConfigDirName)
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	functionFile := filepath.Join(configDir, config.CurrentFunctionFile)
	if err := os.WriteFile(functionFile, []byte(functionIDTest), 0o600); err != nil {
		t.Fatalf("failed to write function file: %v", err)
	}

	server.AddResponse("/functions/"+functionIDTest, 200, `{"message":"deleted","status":"deleted"}`)

	origID := functionID
	functionID = functionIDTest
	t.Cleanup(func() { functionID = origID })

	SetSkipConfirmation(true)
	t.Cleanup(func() { SetSkipConfirmation(false) })

	origFormat := outputFormat
	outputFormat = "text"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	testutil.CaptureOutput(func() {
		err := runFunctionDelete(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// Verify function file was cleared
	if _, err := os.Stat(functionFile); !os.IsNotExist(err) {
		t.Error("function file should have been removed after delete")
	}
}

func TestFunctionDelete_DifferentFunction_DoesNotClearCurrentFunction(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	tmpDir := setupFunctionFileTest(t)

	// Create function file with "fn_current"
	configDir := filepath.Join(tmpDir, config.ConfigDirName)
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	functionFile := filepath.Join(configDir, config.CurrentFunctionFile)
	if err := os.WriteFile(functionFile, []byte("fn_current"), 0o600); err != nil {
		t.Fatalf("failed to write function file: %v", err)
	}

	// Delete a different function "fn_different"
	server.AddResponse("/functions/fn_different", 200, `{"message":"deleted","status":"deleted"}`)

	origID := functionID
	functionID = "fn_different"
	t.Cleanup(func() { functionID = origID })

	SetSkipConfirmation(true)
	t.Cleanup(func() { SetSkipConfirmation(false) })

	origFormat := outputFormat
	outputFormat = "text"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	testutil.CaptureOutput(func() {
		err := runFunctionDelete(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// Verify function file still contains "fn_current"
	data, err := os.ReadFile(functionFile)
	if err != nil {
		t.Fatalf("function file should still exist: %v", err)
	}
	if strings.TrimSpace(string(data)) != "fn_current" {
		t.Errorf("function file content = %q, want %q", string(data), "fn_current")
	}
}

func TestFunctionShow_UsesCurrentFunction(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())

	tmpDir := setupFunctionFileTest(t)

	// Create function file
	configDir := filepath.Join(tmpDir, config.ConfigDirName)
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	functionFile := filepath.Join(configDir, config.CurrentFunctionFile)
	if err := os.WriteFile(functionFile, []byte(functionIDTest), 0o600); err != nil {
		t.Fatalf("failed to write function file: %v", err)
	}

	server.AddResponse("/functions/"+functionIDTest, 200, functionWithLinkJSON())

	// Clear functionID to test file-based resolution
	origID := functionID
	functionID = ""
	t.Cleanup(func() { functionID = origID })

	// Clear env var too
	env.SetEnv("NOTTE_FUNCTION_ID", "")

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runFunctionShow(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if stdout == "" {
		t.Error("expected output, got empty string")
	}
}
