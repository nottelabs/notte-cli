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
// that the field the API populates is wired to it - so run the two commands
// that print a function's created_at and look at what a user would see.
func TestFunctionsListRendersCreatedAt(t *testing.T) {
	result := runCLIText(t, "functions", "list")
	requireSuccess(t, result)

	if strings.Contains(result.Stdout, "Time:") {
		t.Errorf("timestamp printed as a nested struct rather than a value:\n%s", result.Stdout)
	}

	// A date is the part worth asserting: the clock and zone vary by row, and
	// pinning the layout would break on a formatting change that is not this
	// bug.
	date := regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)
	for _, line := range strings.Split(result.Stdout, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "CreatedAt:") {
			continue
		}
		if !date.MatchString(trimmed) {
			t.Errorf("created_at rendered without a date: %q", trimmed)
		}
		return
	}

	t.Skip("no functions in this workspace to render a created_at for")
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
	if !regexp.MustCompile(`\d{4}-\d{2}-\d{2}`).MatchString(result.Stdout) {
		t.Errorf("no timestamp in the output of functions show:\n%s", result.Stdout)
	}
}

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
