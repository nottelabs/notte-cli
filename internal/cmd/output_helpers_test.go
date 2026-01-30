package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/nottelabs/notte-cli/internal/testutil"
)

func TestPrintListOrEmpty_JSON(t *testing.T) {
	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })

	stdout, _ := testutil.CaptureOutput(func() {
		printed, err := PrintListOrEmpty([]string{}, "No files.")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !printed {
			t.Fatalf("expected printed=true")
		}
	})

	if strings.TrimSpace(stdout) != "[]" {
		t.Fatalf("unexpected stdout: %q", stdout)
	}
}

func TestPrintListOrEmpty_Text(t *testing.T) {
	origFormat := outputFormat
	outputFormat = "text"
	t.Cleanup(func() { outputFormat = origFormat })

	stdout, _ := testutil.CaptureOutput(func() {
		printed, err := PrintListOrEmpty([]string{}, "No files.")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !printed {
			t.Fatalf("expected printed=true")
		}
	})

	if strings.TrimSpace(stdout) != "No files." {
		t.Fatalf("unexpected stdout: %q", stdout)
	}
}

func TestPrintListOrEmpty_NonSlice(t *testing.T) {
	origFormat := outputFormat
	outputFormat = "text"
	t.Cleanup(func() { outputFormat = origFormat })

	printed, err := PrintListOrEmpty("not a slice", "No files.")
	if err == nil {
		t.Fatalf("expected error for non-slice type")
	}
	if printed {
		t.Fatalf("expected printed=false for non-slice type")
	}
	if !strings.Contains(err.Error(), "expected slice") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestPrintListOrEmpty_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		items     any
		emptyMsg  string
		wantPrint bool
		wantErr   bool
	}{
		{
			name:      "nil slice",
			items:     nil,
			emptyMsg:  "No items",
			wantPrint: true,
			wantErr:   false,
		},
		{
			name:      "empty string slice",
			items:     []string{},
			emptyMsg:  "No strings",
			wantPrint: true,
			wantErr:   false,
		},
		{
			name:      "non-empty slice",
			items:     []string{"a", "b"},
			emptyMsg:  "No items",
			wantPrint: false,
			wantErr:   false,
		},
		{
			name:      "not a slice",
			items:     "string",
			emptyMsg:  "Error",
			wantPrint: false,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origFormat := outputFormat
			outputFormat = "text"
			t.Cleanup(func() { outputFormat = origFormat })

			printed, err := PrintListOrEmpty(tt.items, tt.emptyMsg)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if printed != tt.wantPrint {
				t.Errorf("printed = %v, want %v", printed, tt.wantPrint)
			}
		})
	}
}

func TestPrintInfo(t *testing.T) {
	tests := []struct {
		name         string
		outputFormat string
		message      string
		wantStdout   bool
	}{
		{"text mode prints to stdout", "text", "test message", true},
		{"json mode prints to stderr", "json", "test message", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orig := outputFormat
			outputFormat = tt.outputFormat
			t.Cleanup(func() { outputFormat = orig })

			stdout, stderr := testutil.CaptureOutput(func() {
				PrintInfo(tt.message)
			})

			if tt.wantStdout {
				if !strings.Contains(stdout, tt.message) {
					t.Errorf("expected message in stdout, got stdout=%q stderr=%q", stdout, stderr)
				}
			} else {
				if !strings.Contains(stderr, tt.message) {
					t.Errorf("expected message in stderr, got stdout=%q stderr=%q", stdout, stderr)
				}
			}
		})
	}
}

func TestPrintResult(t *testing.T) {
	t.Run("text mode prints message", func(t *testing.T) {
		orig := outputFormat
		outputFormat = "text"
		t.Cleanup(func() { outputFormat = orig })

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err := PrintResult("success message", nil)

		_ = w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !strings.Contains(buf.String(), "success message") {
			t.Errorf("expected message in output, got %q", buf.String())
		}
	})

	t.Run("empty message returns nil", func(t *testing.T) {
		orig := outputFormat
		outputFormat = "text"
		t.Cleanup(func() { outputFormat = orig })

		err := PrintResult("", nil)
		if err != nil {
			t.Errorf("expected nil for empty message, got %v", err)
		}
	})

	t.Run("json mode prints structured output", func(t *testing.T) {
		orig := outputFormat
		outputFormat = "json"
		t.Cleanup(func() { outputFormat = orig })

		stdout, _ := testutil.CaptureOutput(func() {
			err := PrintResult("all good", map[string]any{"ok": true})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		var got map[string]any
		if err := json.Unmarshal([]byte(stdout), &got); err != nil {
			t.Fatalf("failed to parse json: %v", err)
		}
		if got["message"] != "all good" {
			t.Fatalf("expected message to be preserved, got %v", got["message"])
		}
		if got["ok"] != true {
			t.Fatalf("expected ok=true, got %v", got["ok"])
		}
	})

	t.Run("json mode preserves existing message", func(t *testing.T) {
		orig := outputFormat
		outputFormat = "json"
		t.Cleanup(func() { outputFormat = orig })

		stdout, _ := testutil.CaptureOutput(func() {
			err := PrintResult("ignored", map[string]any{"message": "custom"})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		var got map[string]any
		if err := json.Unmarshal([]byte(stdout), &got); err != nil {
			t.Fatalf("failed to parse json: %v", err)
		}
		if got["message"] != "custom" {
			t.Fatalf("expected custom message, got %v", got["message"])
		}
	})
}
