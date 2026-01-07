package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestReadJSONInput_FromValue(t *testing.T) {
	cmd := &cobra.Command{}
	data, err := readJSONInput(cmd, `{"ok":true}`, "action")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != `{"ok":true}` {
		t.Fatalf("unexpected data: %s", string(data))
	}
}

func TestReadJSONInput_FromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "payload.json")
	if err := os.WriteFile(path, []byte(`{"file":1}`), 0o600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	cmd := &cobra.Command{}
	data, err := readJSONInput(cmd, "@"+path, "data")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != `{"file":1}` {
		t.Fatalf("unexpected data: %s", string(data))
	}
}

func TestReadJSONInput_FromStdin(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetIn(bytes.NewBufferString(`{"stdin":true}`))
	data, err := readJSONInput(cmd, "-", "data")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != `{"stdin":true}` {
		t.Fatalf("unexpected data: %s", string(data))
	}
}

func TestReadJSONInput_EmptyStdin(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetIn(bytes.NewBufferString(""))
	_, err := readJSONInput(cmd, "-", "data")
	if err == nil {
		t.Fatalf("expected error for empty stdin")
	}
}

func TestReadJSONInput_FromFileStdin(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetIn(bytes.NewBufferString(`{"filestdin":true}`))
	data, err := readJSONInput(cmd, "@-", "data")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != `{"filestdin":true}` {
		t.Fatalf("unexpected data: %s", string(data))
	}
}

func TestReadJSONInput_InvalidAtPath(t *testing.T) {
	cmd := &cobra.Command{}
	_, err := readJSONInput(cmd, "@", "data")
	if err == nil {
		t.Fatal("expected error for missing @ path")
	}
}

func TestReadJSONInput_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.json")
	if err := os.WriteFile(path, []byte(""), 0o600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	cmd := &cobra.Command{}
	_, err := readJSONInput(cmd, "@"+path, "data")
	if err == nil {
		t.Fatal("expected error for empty file")
	}
}

func TestReadJSONInput_MissingFile(t *testing.T) {
	cmd := &cobra.Command{}
	_, err := readJSONInput(cmd, "@does-not-exist.json", "data")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestStdinHasData(t *testing.T) {
	if !stdinHasData(bytes.NewBufferString("x")) {
		t.Fatal("expected true for non-file reader")
	}

	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("failed to open %s: %v", os.DevNull, err)
	}
	t.Cleanup(func() {
		if err := devNull.Close(); err != nil {
			t.Fatalf("failed to close %s: %v", os.DevNull, err)
		}
	})

	if stdinHasData(devNull) {
		t.Fatal("expected false for char device")
	}
}
