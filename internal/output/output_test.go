package output

import (
	"bytes"
	"strings"
	"testing"
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
	if !strings.Contains(got, "Name:") || !strings.Contains(got, "test-session") {
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
