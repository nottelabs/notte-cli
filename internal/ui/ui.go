package ui

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/muesli/termenv"
)

type contextKey struct{}

// UI handles all user-facing output with proper stdout/stderr separation
type UI struct {
	out   io.Writer
	err   io.Writer
	term  *termenv.Output
	color bool
}

// New creates a UI with default stdout/stderr
func New(colorMode string) *UI {
	return NewWithWriters(os.Stdout, os.Stderr, colorMode)
}

// NewWithWriters creates a UI with custom writers (for testing)
func NewWithWriters(stdout, stderr io.Writer, colorMode string) *UI {
	color := shouldUseColor(colorMode, stderr)

	var term *termenv.Output
	if f, ok := stderr.(*os.File); ok {
		term = termenv.NewOutput(f)
	} else {
		term = termenv.NewOutput(os.Stderr)
	}

	return &UI{
		out:   stdout,
		err:   stderr,
		term:  term,
		color: color,
	}
}

func shouldUseColor(mode string, w io.Writer) bool {
	// Check NO_COLOR env var (Unix standard)
	if os.Getenv("NO_COLOR") != "" {
		return false
	}

	switch mode {
	case "always":
		return true
	case "never":
		return false
	default: // "auto"
		// Check if stderr is a terminal
		if f, ok := w.(*os.File); ok {
			return termenv.NewOutput(f).ColorProfile() != termenv.Ascii
		}
		return false
	}
}

// Out returns the stdout writer for data output
func (u *UI) Out() io.Writer {
	return u.out
}

// Err returns the stderr writer
func (u *UI) Err() io.Writer {
	return u.err
}

// Success prints a success message to stderr
func (u *UI) Success(msg string) {
	if u.color {
		msg = u.term.String(msg).Foreground(u.term.Color("2")).String()
	}
	_, _ = fmt.Fprintln(u.err, msg)
}

// Error prints an error message to stderr
func (u *UI) Error(msg string) {
	if u.color {
		msg = u.term.String(msg).Foreground(u.term.Color("1")).String()
	}
	_, _ = fmt.Fprintln(u.err, msg)
}

// Info prints an info message to stderr
func (u *UI) Info(msg string) {
	_, _ = fmt.Fprintln(u.err, msg)
}

// Warn prints a warning message to stderr
func (u *UI) Warn(msg string) {
	if u.color {
		msg = u.term.String(msg).Foreground(u.term.Color("3")).String()
	}
	_, _ = fmt.Fprintln(u.err, msg)
}

// Printf prints formatted output to stderr
func (u *UI) Printf(format string, args ...interface{}) {
	_, _ = fmt.Fprintf(u.err, format, args...)
}

// Println prints a line to stderr
func (u *UI) Println(args ...interface{}) {
	_, _ = fmt.Fprintln(u.err, args...)
}

// WithUI adds UI to context
func WithUI(ctx context.Context, u *UI) context.Context {
	return context.WithValue(ctx, contextKey{}, u)
}

// FromContext retrieves UI from context, returns default if not found
func FromContext(ctx context.Context) *UI {
	if u, ok := ctx.Value(contextKey{}).(*UI); ok {
		return u
	}
	return New("auto")
}
