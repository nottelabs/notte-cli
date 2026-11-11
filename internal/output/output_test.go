package output

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nottelabs/notte-cli/internal/api"
)

type testData struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

func TestJSONFormatter(t *testing.T) {
	var buf bytes.Buffer
	f := &JSONFormatter{Writer: &buf}

	data := testData{Name: "test", Count: 42}
	if err := f.Print(data); err != nil {
		t.Fatalf("Print failed: %v", err)
	}

	got := strings.TrimSpace(buf.String())
	want := `{"name":"test","count":42}`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestJSONFormatter_Slice(t *testing.T) {
	var buf bytes.Buffer
	f := &JSONFormatter{Writer: &buf}

	data := []testData{
		{Name: "one", Count: 1},
		{Name: "two", Count: 2},
	}
	if err := f.Print(data); err != nil {
		t.Fatalf("Print failed: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, `"name":"one"`) {
		t.Errorf("expected 'one' in output: %s", got)
	}
}

func TestTextFormatter_SingleItem(t *testing.T) {
	var buf bytes.Buffer
	f := &TextFormatter{Writer: &buf}

	data := map[string]any{
		"Name":   "test-session",
		"Status": "active",
	}
	if err := f.Print(data); err != nil {
		t.Fatalf("Print failed: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "Name") || !strings.Contains(got, "test-session") {
		t.Errorf("expected formatted key-value output, got: %s", got)
	}
}

func TestTextFormatter_Table(t *testing.T) {
	var buf bytes.Buffer
	f := &TextFormatter{Writer: &buf}

	data := []map[string]any{
		{"ID": "abc123", "Status": "active"},
		{"ID": "def456", "Status": "closed"},
	}

	if err := f.PrintTable([]string{"ID", "Status"}, data); err != nil {
		t.Fatalf("PrintTable failed: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "ID") || !strings.Contains(got, "abc123") {
		t.Errorf("expected table output with headers and data, got: %s", got)
	}
}

func TestTextFormatter_Print_Struct(t *testing.T) {
	type TestStruct struct {
		Name  string
		Value int
	}

	var buf bytes.Buffer
	f := &TextFormatter{Writer: &buf, NoColor: true}

	err := f.Print(TestStruct{Name: "test", Value: 42})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Name") || !strings.Contains(output, "test") {
		t.Errorf("expected struct fields in output, got %q", output)
	}
	if !strings.Contains(output, "Value") || !strings.Contains(output, "42") {
		t.Errorf("expected struct fields in output, got %q", output)
	}
}

func TestTextFormatter_Print_Slice(t *testing.T) {
	type Item struct {
		ID string
	}

	var buf bytes.Buffer
	f := &TextFormatter{Writer: &buf, NoColor: true}

	items := []Item{{ID: "item1"}, {ID: "item2"}}
	err := f.Print(items)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "item1") || !strings.Contains(output, "item2") {
		t.Errorf("expected slice items in output, got %q", output)
	}
}

func TestTextFormatter_Print_Map(t *testing.T) {
	var buf bytes.Buffer
	f := &TextFormatter{Writer: &buf, NoColor: true}

	data := map[string]any{"key1": "value1", "key2": 42}
	err := f.Print(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "key1") || !strings.Contains(output, "value1") {
		t.Errorf("expected map entries in output, got %q", output)
	}
}

func TestTextFormatter_Print_Pointer(t *testing.T) {
	type TestStruct struct {
		Name string
	}

	t.Run("non-nil pointer", func(t *testing.T) {
		var buf bytes.Buffer
		f := &TextFormatter{Writer: &buf, NoColor: true}

		data := &TestStruct{Name: "test"}
		err := f.Print(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "Name") || !strings.Contains(output, "test") {
			t.Errorf("expected dereferenced struct in output, got %q", output)
		}
	})

	t.Run("nil pointer", func(t *testing.T) {
		var buf bytes.Buffer
		f := &TextFormatter{Writer: &buf, NoColor: true}

		var data *TestStruct
		err := f.Print(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Nothing to show is no output, not a literal "<nil>".
		if output := buf.String(); output != "" {
			t.Errorf("expected no output for a nil pointer, got %q", output)
		}
	})
}

func TestTextFormatter_Print_NestedStruct(t *testing.T) {
	type InnerStruct struct {
		Email    *string
		Password string
		Username *string
	}

	type OuterStruct struct {
		Credentials InnerStruct
	}

	t.Run("nested struct with pointer fields", func(t *testing.T) {
		var buf bytes.Buffer
		f := &TextFormatter{Writer: &buf, NoColor: true}

		username := "hello"
		data := OuterStruct{
			Credentials: InnerStruct{
				Email:    nil,
				Password: "secret",
				Username: &username,
			},
		}
		err := f.Print(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := buf.String()
		// Should NOT contain <nil> or memory addresses
		if strings.Contains(output, "<nil>") {
			t.Errorf("expected nested struct to hide nil fields, got %q", output)
		}
		if strings.Contains(output, "0x") {
			t.Errorf("expected nested struct to dereference pointers, got %q", output)
		}
		// Should contain the actual values
		if !strings.Contains(output, "Credentials") {
			t.Errorf("expected Credentials label, got %q", output)
		}
		if !strings.Contains(output, "Password") || !strings.Contains(output, "secret") {
			t.Errorf("expected Password value, got %q", output)
		}
		if !strings.Contains(output, "Username") || !strings.Contains(output, "hello") {
			t.Errorf("expected Username value, got %q", output)
		}
	})
}

func TestTextFormatter_Print_PointerField(t *testing.T) {
	type TestStruct struct {
		Name  *string
		Value *int
	}

	t.Run("nil pointer fields", func(t *testing.T) {
		var buf bytes.Buffer
		f := &TextFormatter{Writer: &buf, NoColor: true}

		data := TestStruct{Name: nil, Value: nil}
		err := f.Print(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := buf.String()
		// Nil pointer fields should be skipped entirely
		if strings.Contains(output, "Name") || strings.Contains(output, "Value") {
			t.Errorf("expected nil fields to be hidden, got %q", output)
		}
	})

	t.Run("non-nil pointer fields", func(t *testing.T) {
		var buf bytes.Buffer
		f := &TextFormatter{Writer: &buf, NoColor: true}

		name := "test"
		value := 42
		data := TestStruct{Name: &name, Value: &value}
		err := f.Print(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "test") || !strings.Contains(output, "42") {
			t.Errorf("expected dereferenced values, got %q", output)
		}
	})
}

func TestTextFormatter_Print_StructWithSliceOfStructs(t *testing.T) {
	type ParameterInfo struct {
		Default *string
		Name    string
		Type    *string
	}

	type FunctionResponse struct {
		Name      string
		Variables []ParameterInfo
		Versions  []string
	}

	t.Run("slice of structs with pointer fields", func(t *testing.T) {
		var buf bytes.Buffer
		f := &TextFormatter{Writer: &buf, NoColor: true}

		typeStr := "string"
		defaultVal := "today"
		data := FunctionResponse{
			Name: "my-function",
			Variables: []ParameterInfo{
				{Name: "restaurant_id"},
				{Name: "date", Type: &typeStr, Default: &defaultVal},
				{Name: "party_size"},
			},
			Versions: []string{"v1", "v2"},
		}

		err := f.Print(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := buf.String()

		// Must NOT contain memory addresses
		if strings.Contains(output, "0x") {
			t.Errorf("expected no memory addresses in output, got %q", output)
		}

		// Must contain the variable names
		if !strings.Contains(output, "restaurant_id") {
			t.Errorf("expected 'restaurant_id' in output, got %q", output)
		}
		if !strings.Contains(output, "date") {
			t.Errorf("expected 'date' in output, got %q", output)
		}
		if !strings.Contains(output, "party_size") {
			t.Errorf("expected 'party_size' in output, got %q", output)
		}

		// Must contain optional fields when set
		if !strings.Contains(output, "string") {
			t.Errorf("expected Type 'string' in output, got %q", output)
		}
		if !strings.Contains(output, "today") {
			t.Errorf("expected Default 'today' in output, got %q", output)
		}

		// Must still contain the Versions field properly
		if !strings.Contains(output, "v1") || !strings.Contains(output, "v2") {
			t.Errorf("expected versions in output, got %q", output)
		}

		// Must contain the Variables label
		if !strings.Contains(output, "Variables\n") {
			t.Errorf("expected 'Variables' label in output, got %q", output)
		}
	})

	t.Run("empty slice of structs", func(t *testing.T) {
		var buf bytes.Buffer
		f := &TextFormatter{Writer: &buf, NoColor: true}

		data := FunctionResponse{
			Name:      "my-function",
			Variables: []ParameterInfo{},
			Versions:  []string{"v1"},
		}

		err := f.Print(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := buf.String()
		if strings.Contains(output, "0x") {
			t.Errorf("expected no memory addresses in output, got %q", output)
		}
	})

	t.Run("nil slice of structs is hidden", func(t *testing.T) {
		var buf bytes.Buffer
		f := &TextFormatter{Writer: &buf, NoColor: true}

		data := FunctionResponse{
			Name:      "my-function",
			Variables: nil,
			Versions:  []string{"v1"},
		}

		err := f.Print(data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output := buf.String()
		// Nil slices should be skipped entirely
		if strings.Contains(output, "Variables") {
			t.Errorf("expected nil Variables to be hidden, got %q", output)
		}
	})
}

func TestTextFormatter_PrintError(t *testing.T) {
	var buf bytes.Buffer
	f := &TextFormatter{Writer: &buf, NoColor: true}

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	f.PrintError(errors.New("test error"))

	_ = w.Close()
	os.Stderr = oldStderr

	var errBuf bytes.Buffer
	_, _ = io.Copy(&errBuf, r)

	if !strings.Contains(errBuf.String(), "Error:") || !strings.Contains(errBuf.String(), "test error") {
		t.Errorf("expected error message, got %q", errBuf.String())
	}
}

func TestTextFormatter_PrintTable(t *testing.T) {
	var buf bytes.Buffer
	f := &TextFormatter{Writer: &buf, NoColor: true}

	headers := []string{"ID", "Name"}
	data := []map[string]any{
		{"ID": "1", "Name": "Alice"},
		{"ID": "2", "Name": "Bob"},
	}

	err := f.PrintTable(headers, data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "ID") || !strings.Contains(output, "Name") {
		t.Errorf("expected headers in output, got %q", output)
	}
	if !strings.Contains(output, "Alice") || !strings.Contains(output, "Bob") {
		t.Errorf("expected data in output, got %q", output)
	}
}

func TestJSONFormatter_Print(t *testing.T) {
	type TestStruct struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	var buf bytes.Buffer
	f := &JSONFormatter{Writer: &buf}

	err := f.Print(TestStruct{Name: "test", Value: 42})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, `"name":"test"`) {
		t.Errorf("expected JSON name field, got %q", output)
	}
	if !strings.Contains(output, `"value":42`) {
		t.Errorf("expected JSON value field, got %q", output)
	}
}

func TestJSONFormatter_PrintError(t *testing.T) {
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	f := &JSONFormatter{Writer: os.Stdout}
	f.PrintError(errors.New("test error"))

	_ = w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	output := buf.String()
	if !strings.Contains(output, `"error"`) || !strings.Contains(output, "test error") {
		t.Errorf("expected JSON error, got %q", output)
	}
}

func TestNewFormatter(t *testing.T) {
	tests := []struct {
		format   Format
		wantType string
	}{
		{FormatJSON, "*output.JSONFormatter"},
		{FormatText, "*output.TextFormatter"},
		{Format("unknown"), "*output.TextFormatter"},
	}

	for _, tt := range tests {
		t.Run(string(tt.format), func(t *testing.T) {
			f := NewFormatter(tt.format, os.Stdout)
			got := fmt.Sprintf("%T", f)
			if got != tt.wantType {
				t.Errorf("NewFormatter(%q) = %s, want %s", tt.format, got, tt.wantType)
			}
		})
	}
}

// A timestamp is a value to print, not a struct to descend into. Before the
// Stringer check, time.Time reached only unexported fields and printed a bare
// "CreatedAt:", and FlexibleTime - which embeds it - added an empty "Time:"
// underneath, so every timestamp the CLI showed in text form came out blank.
func TestTextFormatter_Print_Timestamps(t *testing.T) {
	var buf bytes.Buffer
	f := &TextFormatter{Writer: &buf, NoColor: true}
	orig := now
	now = func() time.Time { return time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC) }
	t.Cleanup(func() { now = orig })

	moment := time.Date(2026, 9, 9, 15, 4, 5, 0, time.UTC)
	err := f.Print(struct {
		Name      string
		CreatedAt api.FlexibleTime
		PlainAt   time.Time
	}{
		Name:      "checkout",
		CreatedAt: api.FlexibleTime{Time: moment},
		PlainAt:   moment,
	})
	if err != nil {
		t.Fatalf("Print failed: %v", err)
	}

	got := buf.String()
	if strings.Contains(got, "Time") {
		t.Errorf("descended into the timestamp instead of printing it:\n%s", got)
	}
	// Timestamps drop the "At" of their name, print as a date and come last.
	want := "Name      checkout\nCreated   Sep 9 15:04\nPlain     Sep 9 15:04\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestHumanize(t *testing.T) {
	for name, want := range map[string]string{
		"FunctionRunId":        "Function run ID",
		"IsUsageLimitExceeded": "Is usage limit exceeded",
		"CdpUrl":               "CDP URL",
		"ID":                   "ID",
		"LlmUsageCost":         "LLM usage cost",
		"persisted_domains":    "Persisted domains",
	} {
		if got := Humanize(name, false); got != want {
			t.Errorf("Humanize(%q) = %q, want %q", name, got, want)
		}
	}
	if got := Humanize("LastAccessedAt", true); got != "Last accessed" {
		t.Errorf("timestamp label = %q", got)
	}
}

// The fallback output puts what identifies a thing first, hides empty fields,
// prints numbers plainly and keeps map keys in a fixed order.
func TestTextFormatter_Print_Generic(t *testing.T) {
	type item struct {
		Cost        float64
		Description string
		Domains     []string
		Empty       string
		Extra       map[string]any
		ItemId      string
		Name        string
		Shared      bool
		Status      string
	}
	var buf bytes.Buffer
	f := &TextFormatter{Writer: &buf, NoColor: true}
	if err := f.Print(item{Cost: 1768816800, Description: "d", Domains: []string{"a.com", "b.com"}, ItemId: "i1", Name: "n", Status: "active", Extra: map[string]any{"z": 1, "a": "x"}}); err != nil {
		t.Fatal(err)
	}
	want := "Name          n\nItem ID       i1\nStatus        active\nDescription   d\nCost          1768816800\nDomains       a.com, b.com\nExtra\n  a   x\n  z   1\nShared        false\n"
	if got := buf.String(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRelativeTime(t *testing.T) {
	now := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		t    time.Time
		want string
	}{
		{time.Time{}, "-"},
		{now.Add(-20 * time.Second), "just now"},
		{now.Add(-5 * time.Minute), "5 min ago"},
		{now.Add(-3 * time.Hour), "3 h ago"},
		{now.Add(-72 * time.Hour), "Mar 7 12:00"},
		{now.AddDate(-1, 0, 0), "Mar 10 2025"},
		{now.Add(48 * time.Hour), "Mar 12 2026 12:00"},
	} {
		if got := RelativeTime(tc.t, now); got != tc.want {
			t.Errorf("RelativeTime(%v) = %q, want %q", tc.t, got, tc.want)
		}
	}
}
