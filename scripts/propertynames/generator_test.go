package propertynames

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestMultiplePropertyNamesCompile(t *testing.T) {
	dir := t.TempDir()
	spec := filepath.Join(dir, "openapi.json")
	output := filepath.Join(dir, "property_names.gen.go")
	// Different key sets must retain separate validators even on one schema.
	fixture := `{"components":{"schemas":{
		"FormFillAction":{"properties":{"value":{"propertyNames":{"enum":["email"]}}}},
		"GetFunctionRunResponse":{"properties":{
			"payloads":{"propertyNames":{"enum":["result","logs"]}},
			"payload_urls":{"propertyNames":{"enum":["result","variables"]}}
		}}
	}}}`
	if err := os.WriteFile(spec, []byte(fixture), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("python3", "../gen-property-names.py", spec, output)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate: %v\n%s", err, out)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, output, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	config := types.Config{Importer: importer.Default()}
	pkg, err := config.Check("api", fset, []*ast.File{file}, nil)
	if err != nil {
		t.Fatalf("generated code does not compile: %v", err)
	}
	for _, name := range []string{
		"FormFillActionKey", "ValidateFormFillActionKeys",
		"GetFunctionRunResponsePayloadsKey", "ValidateGetFunctionRunResponsePayloadsKeys",
		"GetFunctionRunResponsePayloadUrlsKey", "ValidateGetFunctionRunResponsePayloadUrlsKeys",
	} {
		if pkg.Scope().Lookup(name) == nil {
			t.Errorf("missing generated declaration %s", name)
		}
	}
}
