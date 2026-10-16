package discriminators

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestSchemaConversionKeepsNullableUnions(t *testing.T) {
	script, err := os.ReadFile("../generate.sh")
	if err != nil {
		t.Fatal(err)
	}
	const start = `jq --arg excluded "$EXCLUDED_PATHS" '`
	_, program, ok := strings.Cut(string(script), start)
	if !ok {
		t.Fatal("conversion program not found")
	}
	program, _, ok = strings.Cut(program, "' /tmp/notte-openapi.json")
	if !ok {
		t.Fatal("conversion program end not found")
	}
	fixture := `{"paths":{},"components":{"schemas":{"Null":{"type":"null"},"Optional":{"anyOf":[{"type":"string"},{"type":"null"}]},"Array":{"anyOf":[{"type":"array","items":{"type":"string","enum":["en","fr"]}},{"type":"null"}]}}}}`
	command := exec.Command("jq", "--arg", "excluded", "", program)
	command.Stdin = strings.NewReader(fixture)
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Components struct{ Schemas map[string]map[string]any }
	}
	if err := json.Unmarshal(output, &schema); err != nil {
		t.Fatal(err)
	}
	for name, model := range schema.Components.Schemas {
		if model["nullable"] != true {
			t.Errorf("%s lost nullable", name)
		}
		if _, exists := model["anyOf"]; exists {
			t.Errorf("%s union was not flattened", name)
		}
	}
	if _, exists := schema.Components.Schemas["Null"]["type"]; exists {
		t.Error("standalone null was not converted")
	}
	if schema.Components.Schemas["Optional"]["type"] != "string" {
		t.Error("optional string lost its type")
	}
	if schema.Components.Schemas["Array"]["type"] != "array" || schema.Components.Schemas["Array"]["items"] == nil {
		t.Error("optional array lost its item schema")
	}
}
