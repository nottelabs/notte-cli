package ui

import "context"

type formatKey struct{}

// WithFormat adds output format to context
func WithFormat(ctx context.Context, format string) context.Context {
	return context.WithValue(ctx, formatKey{}, format)
}

// FormatFromContext retrieves output format from context
func FormatFromContext(ctx context.Context) string {
	if f, ok := ctx.Value(formatKey{}).(string); ok {
		return f
	}
	return "text"
}

// IsJSONFormat returns true if output format is JSON
func IsJSONFormat(ctx context.Context) bool {
	return FormatFromContext(ctx) == "json"
}
