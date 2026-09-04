package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// multipartSpec is the shape FastAPI produces for a file upload endpoint: an
// inline body schema referenced by $ref, a binary part, and scalars beside it.
const multipartSpec = `{
  "openapi": "3.0.3",
  "paths": {
    "/functions": {
      "post": {
        "operationId": "function_create",
        "requestBody": {
          "content": {
            "multipart/form-data": {
              "schema": {"$ref": "#/components/schemas/BodyFunctionCreate"}
            }
          }
        }
      }
    }
  },
  "components": {
    "schemas": {
      "BodyFunctionCreate": {
        "type": "object",
        "required": ["file"],
        "properties": {
          "file": {"type": "string", "contentMediaType": "application/octet-stream"},
          "name": {"type": "string"},
          "shared": {"type": "boolean", "default": false},
          "response_format": {"type": "string", "description": "JSON Schema of run()'s return value"}
        }
      }
    }
  }
}`

func writeSpec(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "spec.json")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("writing spec: %v", err)
	}
	return path
}

func parse(t *testing.T, body string) *OpenAPISpec {
	t.Helper()
	spec, err := ParseOpenAPISpec(writeSpec(t, body))
	if err != nil {
		t.Fatalf("parsing spec: %v", err)
	}
	return spec
}

// A multipart body used to be skipped outright, which is how `functions create`
// went years without a --response-format flag the API had all along.
func TestMultipartBodyGeneratesAFileAndItsFields(t *testing.T) {
	spec := parse(t, multipartSpec)
	schemas := buildSchemaMap(spec)

	config, err := extractCommandConfig(
		"FunctionCreate", "/functions", "POST", spec.Paths["/functions"].Post, schemas)
	if err != nil {
		t.Fatalf("extracting config: %v", err)
	}
	if !config.IsMultipart {
		t.Fatal("config.IsMultipart = false, want true for a multipart/form-data body")
	}

	byName := map[string]*FieldConfig{}
	for _, fc := range config.Fields {
		byName[fc.Field.Name] = fc
	}

	// The binary part is a path on the command line and content on the wire.
	// Classified as a scalar it would upload the path as the file's contents.
	if got := byName["file"].Category; got != CategoryFileUpload {
		t.Errorf("file category = %v, want FileUpload", got)
	}
	if !byName["file"].Field.Required {
		t.Error("file should be required")
	}
	if got := byName["response_format"].Category; got != CategoryJSONDocument {
		t.Errorf("response_format category = %v, want JSONDocument", got)
	}
	if got := byName["shared"].Category; got != CategorySimpleFlag {
		t.Errorf("shared category = %v, want SimpleFlag", got)
	}

	code, errs, err := GenerateFlagsFile(config, schemas)
	if err != nil {
		t.Fatalf("generating: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("unexpected generation errors: %v", errs)
	}

	for _, want := range []string{
		"func BuildFunctionCreateBody(cmd *cobra.Command) (io.Reader, string, error)",
		`writer.CreateFormFile("file", filepath.Base(FunctionCreateFile))`,
		`readJSONDocumentFlag(cmd, "response-format", FunctionCreateResponseFormat)`,
		`if cmd.Flags().Changed("shared")`,
		"return &body, writer.FormDataContentType(), nil",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("generated code is missing %q", want)
		}
	}

	// The typed-struct path belongs to JSON bodies; a multipart command has no
	// api.* request type to build.
	if strings.Contains(code, "internal/api") {
		t.Error("multipart command should not import the api package")
	}
}

// A JSON body has no writer for a file part, so that is reported rather than
// half-generated. JSON documents are fine: they unmarshal into the body object.
func TestJSONBodyRejectsAFilePart(t *testing.T) {
	body := strings.Replace(multipartSpec, `"multipart/form-data"`, `"application/json"`, 1)
	spec := parse(t, body)
	schemas := buildSchemaMap(spec)

	config, err := extractCommandConfig(
		"FunctionCreate", "/functions", "POST", spec.Paths["/functions"].Post, schemas)
	if err != nil {
		t.Fatalf("extracting config: %v", err)
	}
	if _, _, err := GenerateFlagsFile(config, schemas); err == nil {
		t.Fatal("expected an error generating a file part into a JSON body")
	}
}

// response_format on a JSON PATCH body must land as an object, not a form
// string: FunctionConfigure is application/json and the client field is a map.
func TestJSONBodyGeneratesAJSONDocumentField(t *testing.T) {
	spec := parse(t, `{
  "openapi": "3.0.3",
  "paths": {
    "/functions/{function_id}": {
      "patch": {
        "operationId": "function_metadata_update",
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/FunctionMetadataUpdateRequest"}
            }
          }
        }
      }
    }
  },
  "components": {
    "schemas": {
      "FunctionMetadataUpdateRequest": {
        "type": "object",
        "properties": {
          "name": {"type": "string"},
          "response_format": {
            "type": "object",
            "additionalProperties": true,
            "description": "JSON Schema of run()'s return value"
          }
        }
      }
    }
  }
}`)
	schemas := buildSchemaMap(spec)

	config, err := extractCommandConfig(
		"FunctionConfigure",
		"/functions/{function_id}",
		"PATCH",
		spec.Paths["/functions/{function_id}"].Patch,
		schemas,
	)
	if err != nil {
		t.Fatalf("extracting config: %v", err)
	}
	if config.IsMultipart {
		t.Fatal("config.IsMultipart = true, want false for application/json")
	}

	code, errs, err := GenerateFlagsFile(config, schemas)
	if err != nil {
		t.Fatalf("generating: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("unexpected generation errors: %v", errs)
	}

	for _, want := range []string{
		"func BuildFunctionConfigureRequest(cmd *cobra.Command) (*api.FunctionMetadataUpdateRequest, error)",
		`readJSONDocumentFlag(cmd, "response-format", FunctionConfigureResponseFormat)`,
		"var document map[string]interface{}",
		"decoder := json.NewDecoder(strings.NewReader(responseFormat))",
		"decoder.UseNumber()",
		"body.ResponseFormat = &document",
		`"encoding/json"`,
		`"strings"`,
	} {
		if !strings.Contains(code, want) {
			t.Errorf("generated code is missing %q\n%s", want, code)
		}
	}
}

// The six stale entries this replaces pointed at paths the API had renamed, and
// every one of them failed silently by falling out of the loop.
func TestEndpointMapEntryMissingFromSpecIsAnError(t *testing.T) {
	spec := parse(t, `{"openapi":"3.0.3","paths":{},"components":{"schemas":{}}}`)
	_, err := ExtractCommandConfigs(spec)
	if err == nil {
		t.Fatal("expected an error when no endpointMap entry matches the spec")
	}
	if !strings.Contains(err.Error(), "matched nothing in the spec") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// GET is not in `methods`, and a body-less operation cannot contribute flags.
func TestOperationLookupCoversTheBodyCarryingMethods(t *testing.T) {
	var item PathItem
	if err := json.Unmarshal([]byte(`{
		"get": {"operationId": "g"},
		"post": {"operationId": "p"},
		"put": {"operationId": "u"},
		"patch": {"operationId": "a"}
	}`), &item); err != nil {
		t.Fatalf("unmarshalling path item: %v", err)
	}

	for _, method := range methods {
		if item.operation(method) == nil {
			t.Errorf("operation(%q) = nil, want the parsed operation", method)
		}
	}
	if item.operation("GET") != nil {
		t.Error("operation(\"GET\") should be nil: this generator reads request bodies")
	}
}

// Fields the hand-written command supplies itself are skipped per command, not
// globally: `variables` is free-form on FunctionScheduleSet, but a different
// endpoint's `variables` should still be classified on its merits.
func TestCommandScopedSkipOnlyAppliesToItsCommand(t *testing.T) {
	field := &Field{Name: "variables", JSONName: "variables", Type: "object"}

	got, err := ClassifyField("FunctionScheduleSet", field, nil)
	if err != nil {
		t.Fatalf("classifying: %v", err)
	}
	if got != CategorySkipped {
		t.Errorf("category for FunctionScheduleSet = %v, want Skipped", got)
	}

	got, err = ClassifyField("SomeOtherCommand", field, nil)
	if err != nil {
		t.Fatalf("classifying: %v", err)
	}
	if got == CategorySkipped {
		t.Error("variables should not be skipped outside FunctionScheduleSet")
	}
}

// A flag may be named differently from its API field when the field name is
// ambiguous on a command line: `instructions` on FunctionConfigure documents
// the function for its callers, and bare --instructions was read as input to
// the self-healing agent.
func TestFlagNameOverrideRenamesOnlyItsOwnCommand(t *testing.T) {
	if got := FlagNameFor("FunctionConfigure", "instructions"); got != "run-instructions" {
		t.Errorf("FlagNameFor(FunctionConfigure, instructions) = %q, want run-instructions", got)
	}
	// The same field name elsewhere is untouched: `notte page scrape
	// --instructions` means what it says.
	if got := FlagNameFor("ScrapeWebpage", "instructions"); got != "instructions" {
		t.Errorf("FlagNameFor(ScrapeWebpage, instructions) = %q, want instructions", got)
	}
	if got := FlagNameFor("FunctionConfigure", "self_healing"); got != "self-healing" {
		t.Errorf("FlagNameFor(FunctionConfigure, self_healing) = %q, want self-healing", got)
	}
}
