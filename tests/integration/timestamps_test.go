//go:build integration

package integration

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

// The bug this covers only exists against a real response: the generator
// rewrites timestamps to FlexibleTime so a payload without timezone info still
// decodes, and a substitution that missed a field left it as time.Time, which
// fails on exactly that payload. A synthetic fixture proves the decoder, not
// that the field the API populates is wired to it - so run the commands that
// decode a function's timestamps and look at what a user would see.
func TestFunctionsListDecodesTimestamps(t *testing.T) {
	result := runCLIText(t, "functions", "list")
	requireSuccess(t, result)

	if strings.Contains(result.Stdout, "No functions") {
		t.Skip("no functions in this workspace to decode a timestamp for")
	}
	if strings.Contains(result.Stdout, "Time:") {
		t.Errorf("timestamp printed as a nested struct rather than a value:\n%s", result.Stdout)
	}
	if !strings.Contains(result.Stdout, "UPDATED") {
		t.Errorf("no UPDATED column in the output of functions list:\n%s", result.Stdout)
	}
}

// `functions show` decodes FunctionResponse, a different generated type from
// the one `functions list` returns, and it regressed alongside it.
func TestFunctionShowDecodesCreatedAt(t *testing.T) {
	list := runCLI(t, "functions", "list")
	requireSuccess(t, list)

	functionID := firstFunctionID(t, list.Stdout)
	if functionID == "" {
		t.Skip("no functions in this workspace to show")
	}

	result := runCLIText(t, "functions", "show", "--function-id", functionID)
	requireSuccess(t, result)

	if strings.Contains(result.Stdout, "Time:") {
		t.Errorf("timestamp printed as a nested struct rather than a value:\n%s", result.Stdout)
	}
	match := createdRow.FindStringSubmatch(result.Stdout)
	if match == nil || match[1] == "-" {
		t.Errorf("no created_at in the output of functions show:\n%s", result.Stdout)
	}
}

// createdRow captures the value of the Created row ("Created   9 min ago",
// "Created   Oct 16 13:43"). A zero timestamp prints as "-".
var createdRow = regexp.MustCompile(`(?m)^\s*Created\s+(\S.*?)\s*$`)

// firstFunctionID pulls an id out of `functions list -o json`, which returns
// either a bare array or a paginated object depending on the workspace.
func firstFunctionID(t *testing.T, stdout string) string {
	t.Helper()

	type item struct {
		FunctionID string `json:"function_id"`
	}

	var items []item
	if err := json.Unmarshal([]byte(stdout), &items); err != nil {
		var paginated struct {
			Items []item `json:"items"`
		}
		if err := json.Unmarshal([]byte(stdout), &paginated); err != nil {
			t.Fatalf("parsing list response: %v", err)
		}
		items = paginated.Items
	}

	for _, it := range items {
		if it.FunctionID != "" {
			return it.FunctionID
		}
	}
	return ""
}
