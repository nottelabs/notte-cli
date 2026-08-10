package cmd

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"

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
	origSecretValue := functionSecretValue
	functionID = functionIDTest
	functionRunID = functionRunIDTest
	functionSecretValue = ""
	t.Cleanup(func() {
		functionID = origFunctionID
		functionRunID = origRunID
		functionSecretValue = origSecretValue
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

func secretMetadataJSON() string {
	return `{"id":"sec_123","name":"API_KEY","namespace":"function_env","key_hint":"API***","created_at":"2020-01-01T00:00:00Z"}`
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

func TestRunFunctionSecretsList_Success(t *testing.T) {
	server := setupFunctionTest(t)
	server.AddResponse("/secrets", 200, `{"items":[`+secretMetadataJSON()+`]}`)

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runFunctionSecretsList(cmd, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(stdout, `"namespace":"function_env"`) {
		t.Errorf("expected function_env secret in output, got %q", stdout)
	}

	requests := server.Requests("/secrets")
	if len(requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(requests))
	}
	if requests[0].Method != "GET" {
		t.Errorf("method = %q, want GET", requests[0].Method)
	}
	if !strings.Contains(requests[0].Query, "namespace=function_env") {
		t.Errorf("query = %q, want namespace=function_env", requests[0].Query)
	}
}

func TestRunFunctionSecretsGet_Success(t *testing.T) {
	server := setupFunctionTest(t)
	server.AddResponse("/secrets/API_KEY", 200, `{"value":"secret-value"}`)

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runFunctionSecretsGet(cmd, []string{"API_KEY"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(stdout, `"value":"secret-value"`) {
		t.Errorf("expected secret value in output, got %q", stdout)
	}

	requests := server.Requests("/secrets/API_KEY")
	if len(requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(requests))
	}
	if requests[0].Method != "GET" {
		t.Errorf("method = %q, want GET", requests[0].Method)
	}
	if !strings.Contains(requests[0].Query, "namespace=function_env") {
		t.Errorf("query = %q, want namespace=function_env", requests[0].Query)
	}
}

func TestRunFunctionSecretsSet_Success(t *testing.T) {
	server := setupFunctionTest(t)
	server.AddResponse("/secrets", 201, secretMetadataJSON())

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runFunctionSecretsSet(cmd, []string{"API_KEY", "secret-value"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(stdout, `"id":"sec_123"`) {
		t.Errorf("expected secret metadata in output, got %q", stdout)
	}

	requests := server.Requests("/secrets")
	if len(requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(requests))
	}
	if requests[0].Method != "POST" {
		t.Errorf("method = %q, want POST", requests[0].Method)
	}

	var body map[string]string
	if err := json.Unmarshal([]byte(requests[0].Body), &body); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}
	if body["namespace"] != "function_env" {
		t.Errorf("namespace = %q, want function_env", body["namespace"])
	}
	if body["name"] != "API_KEY" || body["value"] != "secret-value" {
		t.Errorf("unexpected request body: %#v", body)
	}
}

func TestRunFunctionSecretsDelete_Success(t *testing.T) {
	server := setupFunctionTest(t)
	server.AddResponse("/secrets/sec_123", 204, ``)
	SetSkipConfirmation(true)
	t.Cleanup(func() { SetSkipConfirmation(false) })

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		err := runFunctionSecretsDelete(cmd, []string{"sec_123"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(stdout, `"status":"deleted"`) {
		t.Errorf("expected deleted status in output, got %q", stdout)
	}

	requests := server.Requests("/secrets/sec_123")
	if len(requests) != 1 {
		t.Fatalf("expected 1 request, got %d", len(requests))
	}
	if requests[0].Method != "DELETE" {
		t.Errorf("method = %q, want DELETE", requests[0].Method)
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

func TestRequireFunctionID_RequiresExplicitID(t *testing.T) {
	origID := functionID
	functionID = ""
	t.Cleanup(func() { functionID = origID })

	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_FUNCTION_ID", "env_function")

	err := RequireFunctionID()
	if err == nil {
		t.Fatal("RequireFunctionID() should reject implicit function IDs")
	}
	if !strings.Contains(err.Error(), "use --function-id") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRequireFunctionID_WithExplicitID(t *testing.T) {
	origID := functionID
	functionID = "fn_explicit"
	t.Cleanup(func() { functionID = origID })

	if err := RequireFunctionID(); err != nil {
		t.Fatalf("RequireFunctionID() error = %v", err)
	}
}
