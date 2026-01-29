package output

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
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
	if !strings.Contains(output, "Name:") || !strings.Contains(output, "test") {
		t.Errorf("expected struct fields in output, got %q", output)
	}
	if !strings.Contains(output, "Value:") || !strings.Contains(output, "42") {
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
	if !strings.Contains(output, "key1:") || !strings.Contains(output, "value1") {
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
		if !strings.Contains(output, "Name:") || !strings.Contains(output, "test") {
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

		output := buf.String()
		if !strings.Contains(output, "<nil>") {
			t.Errorf("expected <nil> for nil pointer, got %q", output)
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
		if strings.Contains(output, "Name:") || strings.Contains(output, "Value:") {
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
