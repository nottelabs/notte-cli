package cmd

import (
	"context"
	"io"
	"mime"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/testutil"
)

const (
	responseFormatFlag = "response-format"
	testSchema         = `{"type":"object","properties":{"is_live":{"type":"boolean"}}}`
)

// multipartFields decodes a recorded multipart body into its non-file fields.
// Asserting on the raw string would pass for a body that merely mentions
// "response_format" in the uploaded Python, which is exactly the confusion
// this flag exists to fix.
func multipartFields(t *testing.T, req testutil.RecordedRequest) map[string]string {
	t.Helper()

	_, params, err := mime.ParseMediaType(req.Headers.Get("Content-Type"))
	if err != nil {
		t.Fatalf("parsing Content-Type: %v", err)
	}

	reader := multipart.NewReader(strings.NewReader(req.Body), params["boundary"])
	fields := map[string]string{}
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("reading multipart body: %v", err)
		}
		if part.FileName() != "" {
			continue
		}
		value, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("reading part %q: %v", part.FormName(), err)
		}
		fields[part.FormName()] = string(value)
	}
	return fields
}

// writeTempFunction writes a stand-in function file and returns its path.
func writeTempFunction(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "function.py")
	if err := os.WriteFile(path, []byte("def run():\n    return None\n"), 0o600); err != nil {
		t.Fatalf("writing function file: %v", err)
	}
	return path
}

// createCmd builds a command carrying the real generated flags, so the test
// exercises the same registration the CLI does rather than a hand-rolled copy.
// Registration binds and zeroes the flag variables, so --file is set through
// the flag afterwards, the way an invocation would.
func createCmd(t *testing.T) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{}
	RegisterFunctionCreateFlags(cmd)
	cmd.SetContext(context.Background())
	if err := cmd.Flags().Set("file", writeTempFunction(t)); err != nil {
		t.Fatalf("setting --file: %v", err)
	}

	t.Cleanup(func() {
		FunctionCreateResponseFormat = ""
		FunctionCreateShared = false
		FunctionCreateName = ""
	})
	return cmd
}

func setupCreateTest(t *testing.T) *testutil.MockServer {
	t.Helper()
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")

	server := testutil.NewMockServer()
	t.Cleanup(func() { server.Close() })
	env.SetEnv("NOTTE_API_URL", server.URL())
	server.AddResponse("/functions", 200, functionJSON())

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	return server
}

func TestFunctionsCreate_SendsResponseFormat(t *testing.T) {
	server := setupCreateTest(t)

	cmd := createCmd(t)
	if err := cmd.Flags().Set(responseFormatFlag, testSchema); err != nil {
		t.Fatalf("setting flag: %v", err)
	}

	testutil.CaptureOutput(func() {
		if err := runFunctionsCreate(cmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	requests := server.Requests("/functions")
	if len(requests) != 1 {
		t.Fatalf("got %d requests, want 1", len(requests))
	}
	if got := multipartFields(t, requests[0])["response_format"]; got != testSchema {
		t.Errorf("response_format = %q, want %q", got, testSchema)
	}
}

// The flag is optional, and a create that omits it must not send an empty
// schema: the server would store one, and "documented as nothing" reads the
// same as undocumented while being harder to notice.
func TestFunctionsCreate_OmitsResponseFormatWhenUnset(t *testing.T) {
	server := setupCreateTest(t)

	testutil.CaptureOutput(func() {
		if err := runFunctionsCreate(createCmd(t), nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	requests := server.Requests("/functions")
	if len(requests) != 1 {
		t.Fatalf("got %d requests, want 1", len(requests))
	}
	if _, present := multipartFields(t, requests[0])["response_format"]; present {
		t.Error("response_format was sent for a create that did not ask for it")
	}
}

// Pretty-printed schemas are the normal case when they come from a file, and
// they have to survive the trip as valid JSON rather than as a blob of
// newlines the console cannot parse.
func TestFunctionsCreate_ReadsResponseFormatFromFile(t *testing.T) {
	server := setupCreateTest(t)

	path := filepath.Join(t.TempDir(), "schema.json")
	if err := os.WriteFile(path, []byte("{\n  \"type\": \"object\"\n}\n"), 0o600); err != nil {
		t.Fatalf("writing schema file: %v", err)
	}

	cmd := createCmd(t)
	if err := cmd.Flags().Set(responseFormatFlag, "@"+path); err != nil {
		t.Fatalf("setting flag: %v", err)
	}

	testutil.CaptureOutput(func() {
		if err := runFunctionsCreate(cmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	requests := server.Requests("/functions")
	if len(requests) != 1 {
		t.Fatalf("got %d requests, want 1", len(requests))
	}
	if got := multipartFields(t, requests[0])["response_format"]; got != `{"type":"object"}` {
		t.Errorf("response_format = %q, want the compacted schema", got)
	}
}

// The mistake this catches is dropping the @: the server stores the field
// verbatim, so "schema.json" would upload as the schema and return 200.
func TestFunctionsCreate_RejectsResponseFormatThatIsNotJSON(t *testing.T) {
	server := setupCreateTest(t)

	cmd := createCmd(t)
	if err := cmd.Flags().Set(responseFormatFlag, "schema.json"); err != nil {
		t.Fatalf("setting flag: %v", err)
	}

	err := runFunctionsCreate(cmd, nil)
	if err == nil {
		t.Fatal("expected an error for a --response-format that is not JSON")
	}
	if !strings.Contains(err.Error(), "must be a JSON object") {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(server.Requests("/functions")) != 0 {
		t.Error("a rejected schema still reached the API")
	}
}

// `null` parses as JSON and unmarshals into a nil map without error, so the
// object check has to reject it explicitly. Sent through, it would blank the
// schema of the function being updated.
func TestFunctionsCreate_RejectsNullResponseFormat(t *testing.T) {
	server := setupCreateTest(t)

	cmd := createCmd(t)
	if err := cmd.Flags().Set(responseFormatFlag, "null"); err != nil {
		t.Fatalf("setting flag: %v", err)
	}

	err := runFunctionsCreate(cmd, nil)
	if err == nil {
		t.Fatal("expected an error for --response-format null")
	}
	if !strings.Contains(err.Error(), "not null") {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(server.Requests("/functions")) != 0 {
		t.Error("a null schema still reached the API")
	}
}

// Scalar form fields follow the same "only if asked" rule as the JSON ones:
// --shared is registered with the API's default, so sending it unconditionally
// would freeze today's server-side value into the client.
func TestFunctionsCreate_OmitsSharedWhenUnset(t *testing.T) {
	server := setupCreateTest(t)

	cmd := createCmd(t)
	if err := cmd.Flags().Set("name", "Test Function"); err != nil {
		t.Fatalf("setting --name: %v", err)
	}

	testutil.CaptureOutput(func() {
		if err := runFunctionsCreate(cmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	fields := multipartFields(t, server.Requests("/functions")[0])
	if _, present := fields["shared"]; present {
		t.Error("shared was sent for a create that did not pass --shared")
	}
	if fields["name"] != "Test Function" {
		t.Errorf("name = %q, want %q", fields["name"], "Test Function")
	}
}

func TestFunctionUpdate_SendsResponseFormat(t *testing.T) {
	server := setupFunctionTest(t)
	server.AddResponse("/functions/"+functionIDTest, 200, functionJSON())

	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() {
		outputFormat = origFormat
		FunctionUpdateResponseFormat = ""
	})

	cmd := &cobra.Command{}
	RegisterFunctionUpdateFlags(cmd)
	cmd.SetContext(context.Background())
	if err := cmd.Flags().Set("file", writeTempFunction(t)); err != nil {
		t.Fatalf("setting --file: %v", err)
	}
	if err := cmd.Flags().Set(responseFormatFlag, testSchema); err != nil {
		t.Fatalf("setting flag: %v", err)
	}

	testutil.CaptureOutput(func() {
		if err := runFunctionUpdate(cmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	requests := server.Requests("/functions/" + functionIDTest)
	if len(requests) != 1 {
		t.Fatalf("got %d requests, want 1", len(requests))
	}
	if got := multipartFields(t, requests[0])["response_format"]; got != testSchema {
		t.Errorf("response_format = %q, want %q", got, testSchema)
	}
}

// Both commands take the same flag, and a catalog that creates on one
// environment and updates on the next depends on that staying true.
func TestResponseFormatFlagRegisteredOnCreateAndUpdate(t *testing.T) {
	for _, cmd := range []*cobra.Command{functionsCreateCmd, functionsUpdateCmd} {
		if cmd.Flags().Lookup(responseFormatFlag) == nil {
			t.Errorf("functions %s has no --%s flag", cmd.Use, responseFormatFlag)
		}
	}
}
