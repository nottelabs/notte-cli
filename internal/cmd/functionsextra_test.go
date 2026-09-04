package cmd

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/testutil"
)

// configureCmd builds a command with the real generated flags on it.
func configureCmd(t *testing.T) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{}
	RegisterFunctionConfigureFlags(cmd)
	cmd.SetContext(context.Background())
	t.Cleanup(func() {
		FunctionConfigureName = ""
		FunctionConfigureDescription = ""
		FunctionConfigureDomain = ""
		FunctionConfigureInstructions = ""
		FunctionConfigureResponseFormat = ""
		FunctionConfigureSelfHealing = false
	})
	return cmd
}

func requestBody(t *testing.T, req testutil.RecordedRequest) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		t.Fatalf("parsing request body %q: %v", req.Body, err)
	}
	return body
}

// Only what was asked for goes on the wire. A PATCH carrying self_healing:false
// because the flag defaults to false would silently disable self-healing on a
// call that meant to set instructions.
func TestFunctionConfigure_SendsOnlyTheFlagsPassed(t *testing.T) {
	server := setupFunctionTest(t)
	server.AddResponse("/functions/"+functionIDTest, 200, functionJSON())

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := configureCmd(t)
	if err := cmd.Flags().Set("run-instructions", "retry the login step"); err != nil {
		t.Fatalf("setting --run-instructions: %v", err)
	}

	testutil.CaptureOutput(func() {
		if err := runFunctionConfigure(cmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	requests := server.Requests("/functions/" + functionIDTest)
	if len(requests) != 1 {
		t.Fatalf("got %d requests, want 1", len(requests))
	}
	if requests[0].Method != "PATCH" {
		t.Errorf("method = %s, want PATCH", requests[0].Method)
	}

	body := requestBody(t, requests[0])
	if body["instructions"] != "retry the login step" {
		t.Errorf("instructions = %v", body["instructions"])
	}
	if _, present := body["self_healing"]; present {
		t.Error("self_healing was sent for a call that did not pass --self-healing")
	}
}

// Turning it off has to be expressible, and --self-healing=false is the only
// spelling: the field is nullable with no server-side default, so an absent
// field means "leave it alone" rather than "off".
func TestFunctionConfigure_SendsSelfHealingFalseWhenSetExplicitly(t *testing.T) {
	server := setupFunctionTest(t)
	server.AddResponse("/functions/"+functionIDTest, 200, functionJSON())

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := configureCmd(t)
	if err := cmd.Flags().Set("self-healing", "false"); err != nil {
		t.Fatalf("setting --self-healing: %v", err)
	}

	testutil.CaptureOutput(func() {
		if err := runFunctionConfigure(cmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	body := requestBody(t, server.Requests("/functions/" + functionIDTest)[0])
	value, present := body["self_healing"]
	if !present {
		t.Fatal("self_healing was not sent despite --self-healing=false")
	}
	if value != false {
		t.Errorf("self_healing = %v, want false", value)
	}
}

// An empty PATCH is a 200 that changed nothing, which reads as success for a
// command that did not do what the caller meant.
func TestFunctionConfigure_RefusesToSendNothing(t *testing.T) {
	server := setupFunctionTest(t)
	server.AddResponse("/functions/"+functionIDTest, 200, functionJSON())

	err := runFunctionConfigure(configureCmd(t), nil)
	if err == nil {
		t.Fatal("expected an error when no configure flags are passed")
	}
	if !strings.Contains(err.Error(), "nothing to configure") {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(server.Requests("/functions/"+functionIDTest)) != 0 {
		t.Error("an empty configure still reached the API")
	}
}

// Metadata fields added by the OpenAPI regen must be sendable on their own;
// otherwise configure only works when paired with the older flags.
func TestFunctionConfigure_SendsMetadataFieldsAlone(t *testing.T) {
	for _, tc := range []struct {
		flag  string
		value string
		field string
	}{
		{flag: "name", value: "checkout", field: "name"},
		{flag: "description", value: "buys the cart", field: "description"},
		{flag: "domain", value: "shop.example", field: "domain"},
	} {
		t.Run(tc.flag, func(t *testing.T) {
			server := setupFunctionTest(t)
			server.AddResponse("/functions/"+functionIDTest, 200, functionJSON())

			origFormat := outputFormat
			outputFormat = "json"
			t.Cleanup(func() { outputFormat = origFormat })

			cmd := configureCmd(t)
			if err := cmd.Flags().Set(tc.flag, tc.value); err != nil {
				t.Fatalf("setting --%s: %v", tc.flag, err)
			}

			testutil.CaptureOutput(func() {
				if err := runFunctionConfigure(cmd, nil); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			})

			body := requestBody(t, server.Requests("/functions/" + functionIDTest)[0])
			if body[tc.field] != tc.value {
				t.Errorf("%s = %v, want %q", tc.field, body[tc.field], tc.value)
			}
			for _, absent := range []string{"instructions", "self_healing", "name", "description", "domain", "response_format"} {
				if absent == tc.field {
					continue
				}
				if _, present := body[absent]; present {
					t.Errorf("%s was sent for a call that only passed --%s", absent, tc.flag)
				}
			}
		})
	}
}

// --response-format must be sendable alone, as a JSON object on the PATCH body.
func TestFunctionConfigure_SendsResponseFormatAlone(t *testing.T) {
	server := setupFunctionTest(t)
	server.AddResponse("/functions/"+functionIDTest, 200, functionJSON())

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	schema := `{"type":"object","properties":{"ok":{"type":"boolean"}}}`
	cmd := configureCmd(t)
	if err := cmd.Flags().Set("response-format", schema); err != nil {
		t.Fatalf("setting --response-format: %v", err)
	}

	testutil.CaptureOutput(func() {
		if err := runFunctionConfigure(cmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	body := requestBody(t, server.Requests("/functions/" + functionIDTest)[0])
	got, ok := body["response_format"].(map[string]any)
	if !ok {
		t.Fatalf("response_format = %T %v, want object", body["response_format"], body["response_format"])
	}
	if got["type"] != "object" {
		t.Errorf("response_format.type = %v, want object", got["type"])
	}
	for _, absent := range []string{"instructions", "self_healing", "name", "description", "domain"} {
		if _, present := body[absent]; present {
			t.Errorf("%s was sent for a call that only passed --response-format", absent)
		}
	}
}

// Response schemas are arbitrary JSON Schema documents, so a numeric constraint
// may be larger than a float64 can represent exactly. The configure builder
// must preserve that token on the wire rather than round it during decoding.
func TestFunctionConfigure_PreservesLargeResponseSchemaInteger(t *testing.T) {
	server := setupFunctionTest(t)
	server.AddResponse("/functions/"+functionIDTest, 200, functionJSON())

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := configureCmd(t)
	if err := cmd.Flags().Set(
		"response-format",
		`{"type":"object","properties":{"id":{"const":9007199254740993}}}`,
	); err != nil {
		t.Fatalf("setting --response-format: %v", err)
	}

	testutil.CaptureOutput(func() {
		if err := runFunctionConfigure(cmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	body := server.Requests("/functions/" + functionIDTest)[0].Body
	if !strings.Contains(body, `"const":9007199254740993`) {
		t.Errorf("response schema integer was changed on the wire: %s", body)
	}
}

// The generated builder sends an optional string only when it is non-empty, so
// `--flag ""` would reach the API as an empty PATCH: 200, nothing changed, and
// the caller told it worked. Refused up front instead.
func TestFunctionConfigure_RejectsEmptyStringFlags(t *testing.T) {
	for _, flag := range []string{"name", "description", "domain", "run-instructions"} {
		t.Run(flag, func(t *testing.T) {
			server := setupFunctionTest(t)
			server.AddResponse("/functions/"+functionIDTest, 200, functionJSON())

			cmd := configureCmd(t)
			if err := cmd.Flags().Set(flag, ""); err != nil {
				t.Fatalf("setting --%s: %v", flag, err)
			}

			err := runFunctionConfigure(cmd, nil)
			if err == nil {
				t.Fatalf("expected an error for --%s \"\"", flag)
			}
			if !strings.Contains(err.Error(), "cannot be empty") {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(server.Requests("/functions/"+functionIDTest)) != 0 {
				t.Errorf("an empty --%s still reached the API", flag)
			}
		})
	}
}

func TestFunctionRollback_SendsVersion(t *testing.T) {
	server := setupFunctionTest(t)
	server.AddResponse("/functions/"+functionIDTest+"/rollback", 200, functionJSON())

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() {
		outputFormat = origFormat
		FunctionRollbackVersion = ""
	})

	cmd := &cobra.Command{}
	RegisterFunctionRollbackFlags(cmd)
	cmd.SetContext(context.Background())
	if err := cmd.Flags().Set("version", "v20260824_104232"); err != nil {
		t.Fatalf("setting --version: %v", err)
	}

	testutil.CaptureOutput(func() {
		if err := runFunctionRollback(cmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	requests := server.Requests("/functions/" + functionIDTest + "/rollback")
	if len(requests) != 1 {
		t.Fatalf("got %d requests, want 1", len(requests))
	}
	if got := requestBody(t, requests[0])["version"]; got != "v20260824_104232" {
		t.Errorf("version = %v, want v20260824_104232", got)
	}
}

func TestFunctionHealth_ReadsTheRuntime(t *testing.T) {
	server := setupFunctionTest(t)
	server.AddResponse("/functions/health", 200,
		`{"status":"ok","reachable":true,"latency_ms":12,"python_version":"3.13.1","packages":[{"import_name":"notte_sdk","installed":true}],"stdlib_modules":[],"reserved_env_names":[],"runtime_digest":"abc"}`)

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	stdout, _ := testutil.CaptureOutput(func() {
		if err := runFunctionHealth(cmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(stdout, "3.13.1") {
		t.Errorf("expected the runtime's python version in the output, got %q", stdout)
	}
	if len(server.Requests("/functions/health")) != 1 {
		t.Error("health did not call /functions/health")
	}
}

// The API streams by default, so the field is sent only to turn it off.
// Transmitting it either way would freeze today's server-side default into the
// client — the same reasoning behind the generated builders' Changed() guards.
func TestFunctionRun_SendsStreamOnlyWithNoStream(t *testing.T) {
	for _, tc := range []struct {
		name     string
		noStream bool
		want     any
	}{
		{name: "default omits stream", noStream: false, want: nil},
		{name: "--no-stream sends false", noStream: true, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := setupFunctionTest(t)
			server.AddResponse("/functions/"+functionIDTest+"/runs/start", 200, functionRunJSON())

			origFormat := outputFormat
			origNoStream := functionRunNoStream
			outputFormat = "json"
			functionRunNoStream = tc.noStream
			t.Cleanup(func() {
				outputFormat = origFormat
				functionRunNoStream = origNoStream
			})

			cmd := &cobra.Command{}
			cmd.SetContext(context.Background())

			testutil.CaptureOutput(func() {
				if err := runFunctionRun(cmd, nil); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			})

			requests := server.Requests("/functions/" + functionIDTest + "/runs/start")
			if len(requests) != 1 {
				t.Fatalf("got %d requests, want 1", len(requests))
			}
			body := requestBody(t, requests[0])
			got, present := body["stream"]
			if tc.want == nil {
				if present {
					t.Errorf("stream was sent as %v without --no-stream", got)
				}
				return
			}
			if !present {
				t.Fatal("stream was not sent despite --no-stream")
			}
			if got != tc.want {
				t.Errorf("stream = %v, want %v", got, tc.want)
			}
		})
	}
}
