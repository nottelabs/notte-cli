package output

import (
	"io"
	"os"
)

// Format represents output format type
type Format string

const (
	FormatText Format = "text"
	FormatJSON Format = "json"
)

// Formatter interface for output formatting
type Formatter interface {
	Print(data any) error
	PrintError(err error)
}

// NewFormatter creates appropriate formatter for format type
func NewFormatter(format Format, w io.Writer) Formatter {
	if w == nil {
		w = os.Stdout
	}

	switch format {
	case FormatJSON:
		return &JSONFormatter{Writer: w}
	default:
		return &TextFormatter{Writer: w}
	}
}
