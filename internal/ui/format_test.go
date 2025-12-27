package ui

import (
	"context"
	"testing"
)

func TestFormatContext(t *testing.T) {
	ctx := WithFormat(context.Background(), "json")

	format := FormatFromContext(ctx)
	if format != "json" {
		t.Errorf("format = %q, want 'json'", format)
	}
}

func TestFormatContext_Default(t *testing.T) {
	format := FormatFromContext(context.Background())
	if format != "text" {
		t.Errorf("default format = %q, want 'text'", format)
	}
}

func TestIsJSONFormat(t *testing.T) {
	jsonCtx := WithFormat(context.Background(), "json")
	textCtx := WithFormat(context.Background(), "text")

	if !IsJSONFormat(jsonCtx) {
		t.Error("should be JSON format")
	}
	if IsJSONFormat(textCtx) {
		t.Error("should not be JSON format")
	}
}
