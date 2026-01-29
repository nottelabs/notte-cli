package cmd

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/salmonumbrella/notte-cli/internal/testutil"
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
