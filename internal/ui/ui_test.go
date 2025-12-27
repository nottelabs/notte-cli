package ui

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestUI_OutputSeparation(t *testing.T) {
	var stdout, stderr bytes.Buffer
	u := NewWithWriters(&stdout, &stderr, "never")

	u.Success("success message")
	u.Error("error message")
	u.Info("info message")
	_, _ = u.Out().Write([]byte("data output"))

	// Data should go to stdout
	if !strings.Contains(stdout.String(), "data output") {
		t.Error("data should go to stdout")
	}

	// Messages should go to stderr
	if !strings.Contains(stderr.String(), "success message") {
		t.Error("success should go to stderr")
	}
	if !strings.Contains(stderr.String(), "error message") {
		t.Error("error should go to stderr")
	}
	if !strings.Contains(stderr.String(), "info message") {
		t.Error("info should go to stderr")
	}

	// Data should NOT be in stderr
	if strings.Contains(stderr.String(), "data output") {
		t.Error("data should not go to stderr")
	}
}

func TestUI_Context(t *testing.T) {
	u := New("never")
	ctx := WithUI(context.Background(), u)

	retrieved := FromContext(ctx)
	if retrieved != u {
		t.Error("should retrieve same UI from context")
	}
}

func TestUI_FromContext_Fallback(t *testing.T) {
	// Should return default UI when not in context
	u := FromContext(context.Background())
	if u == nil {
		t.Error("should return default UI")
	}
}

func TestColorMode(t *testing.T) {
	tests := []struct {
		mode      string
		wantColor bool
	}{
		{"always", true},
		{"never", false},
	}

	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			var stderr bytes.Buffer
			u := NewWithWriters(&bytes.Buffer{}, &stderr, tt.mode)

			// Color behavior is internal, just verify no panic
			u.Success("test")
			u.Error("test")
		})
	}
}
