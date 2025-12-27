// internal/cmd/confirm_test.go
package cmd

import (
	"bytes"
	"testing"
)

func TestConfirmAction_Yes(t *testing.T) {
	input := bytes.NewReader([]byte("y\n"))
	output := &bytes.Buffer{}

	confirmed, err := ConfirmActionWithIO(input, output, "workflow", "wf_123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !confirmed {
		t.Error("expected confirmed=true for 'y' input")
	}
}

func TestConfirmAction_No(t *testing.T) {
	input := bytes.NewReader([]byte("n\n"))
	output := &bytes.Buffer{}

	confirmed, err := ConfirmActionWithIO(input, output, "workflow", "wf_123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if confirmed {
		t.Error("expected confirmed=false for 'n' input")
	}
}

func TestConfirmAction_Default(t *testing.T) {
	input := bytes.NewReader([]byte("\n"))
	output := &bytes.Buffer{}

	confirmed, err := ConfirmActionWithIO(input, output, "workflow", "wf_123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if confirmed {
		t.Error("expected confirmed=false for empty input (default No)")
	}
}

func TestConfirmAction_YesVariants(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"lowercase yes", "yes\n"},
		{"uppercase Y", "Y\n"},
		{"uppercase YES", "YES\n"},
		{"mixed case Yes", "Yes\n"},
		{"y with spaces", "  y  \n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := bytes.NewReader([]byte(tt.input))
			output := &bytes.Buffer{}

			confirmed, err := ConfirmActionWithIO(input, output, "workflow", "wf_123")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !confirmed {
				t.Errorf("expected confirmed=true for input %q", tt.input)
			}
		})
	}
}

func TestConfirmAction_NoVariants(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"uppercase N", "N\n"},
		{"lowercase no", "no\n"},
		{"uppercase NO", "NO\n"},
		{"random text", "maybe\n"},
		{"just space", " \n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := bytes.NewReader([]byte(tt.input))
			output := &bytes.Buffer{}

			confirmed, err := ConfirmActionWithIO(input, output, "workflow", "wf_123")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if confirmed {
				t.Errorf("expected confirmed=false for input %q", tt.input)
			}
		})
	}
}

func TestConfirmAction_PromptFormat(t *testing.T) {
	input := bytes.NewReader([]byte("n\n"))
	output := &bytes.Buffer{}

	if _, err := ConfirmActionWithIO(input, output, "session", "sess_456"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedPrompt := "Delete session sess_456? This cannot be undone. [y/N]: "
	if output.String() != expectedPrompt {
		t.Errorf("expected prompt %q, got %q", expectedPrompt, output.String())
	}
}

func TestConfirmAction_SkipConfirmation(t *testing.T) {
	// Save original state
	originalSkipConfirmation := skipConfirmation
	defer func() { skipConfirmation = originalSkipConfirmation }()

	// Enable skip confirmation
	SetSkipConfirmation(true)

	// ConfirmAction should return true without prompting
	confirmed, err := ConfirmAction("workflow", "wf_123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !confirmed {
		t.Error("expected confirmed=true when skipConfirmation is enabled")
	}

	// Disable skip confirmation
	SetSkipConfirmation(false)

	// This would normally prompt, but we can't test that easily without IO
	// The behavior is tested indirectly through integration tests
}
