package output

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"text/tabwriter"
	"unicode/utf8"

	"github.com/muesli/termenv"

	apierrors "github.com/nottelabs/notte-cli/internal/errors"
)

// TextFormatter outputs human-readable text
type TextFormatter struct {
	Writer  io.Writer
	NoColor bool
}

var output = termenv.NewOutput(os.Stdout)

// PrintTable prints data as a table with headers
func (f *TextFormatter) PrintTable(headers []string, data []map[string]any) error {
	w := tabwriter.NewWriter(f.Writer, 0, 0, 2, ' ', 0)

	// Print headers
	coloredHeaders := make([]string, len(headers))
	for i, h := range headers {
		coloredHeaders[i] = f.colorize(h, termenv.ANSICyan)
	}
	_, _ = fmt.Fprintln(w, strings.Join(coloredHeaders, "\t"))

	// Print rows
	for _, row := range data {
		values := make([]string, len(headers))
		for i, h := range headers {
			if v, ok := row[h]; ok {
				values[i] = fmt.Sprintf("%v", v)
			}
		}
		_, _ = fmt.Fprintln(w, strings.Join(values, "\t"))
	}

	return w.Flush()
}

// Faces add a little character to text output. JSON output never has them.
const (
	FaceEmpty  = "(◡︵◡)"          // nothing to show
	FaceDenied = "( ｡ •̀ ᴖ •́ ｡)" // not signed in, or not allowed
	FaceCool   = "( •̯́ ₃ •̯̀)"   // something good just happened
)

func (f *TextFormatter) PrintError(err error) {
	f.PrintErrorWithFace(err, "")
}

// PrintErrorWithFace prints an error like PrintError, followed by face.
func (f *TextFormatter) PrintErrorWithFace(err error, face string) {
	suffix := ""
	if face != "" {
		suffix = " " + face
	}

	// For API errors, display "Error <status>: <message>"
	if apiErr, ok := err.(*apierrors.APIError); ok && apiErr.Message != "" {
		errText := f.colorize(fmt.Sprintf("Error %d:", apiErr.StatusCode), termenv.ANSIRed)
		fmt.Fprintf(os.Stderr, "%s %s%s\n", errText, apiErr.Message, suffix)
		return
	}

	// For auth errors, display "Error <status>: <reason> - <message>"
	if authErr, ok := err.(*apierrors.AuthError); ok {
		errText := f.colorize(fmt.Sprintf("Error %d:", authErr.StatusCode), termenv.ANSIRed)
		if authErr.Message != "" {
			fmt.Fprintf(os.Stderr, "%s %s - %s%s\n", errText, authErr.Reason, authErr.Message, suffix)
		} else {
			fmt.Fprintf(os.Stderr, "%s %s%s\n", errText, authErr.Reason, suffix)
		}
		if authErr.Hint != "" {
			fmt.Fprintf(os.Stderr, "Hint: %s\n", authErr.Hint)
		}
		return
	}

	errText := f.colorize("Error:", termenv.ANSIRed)
	fmt.Fprintf(os.Stderr, "%s %s%s\n", errText, err.Error(), suffix)
}

// Label styles a field label the way the generic key-value output does.
func (f *TextFormatter) Label(s string) string {
	return f.colorize(s, termenv.ANSICyan)
}

func (f *TextFormatter) colorize(s string, color termenv.ANSIColor) string {
	if f.NoColor {
		return s
	}
	return output.String(s).Foreground(color).String()
}

// Table prints rows under a header with columns padded to the widest visible
// cell. Unlike tabwriter it ignores escape sequences when measuring, so cells
// may be colored or hold OSC 8 links. The last column is not padded.
func (f *TextFormatter) Table(headers []string, rows [][]string) error {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = VisibleWidth(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if w := VisibleWidth(cell); i < len(widths) && w > widths[i] {
				widths[i] = w
			}
		}
	}
	var b strings.Builder
	line := func(cells []string, style func(string) string) {
		for i, cell := range cells {
			if i > 0 {
				b.WriteString("   ")
			}
			b.WriteString(style(cell))
			if i < len(cells)-1 {
				b.WriteString(strings.Repeat(" ", widths[i]-VisibleWidth(cell)))
			}
		}
		b.WriteString("\n")
	}
	line(headers, f.Label)
	for _, row := range rows {
		line(row, func(s string) string { return s })
	}
	_, err := io.WriteString(f.Writer, b.String())
	return err
}

// Colorize applies an ANSI color when color output is enabled.
func (f *TextFormatter) Colorize(s string, color termenv.ANSIColor) string {
	return f.colorize(s, color)
}

// Faint dims s when color output is enabled.
func (f *TextFormatter) Faint(s string) string {
	if f.NoColor {
		return s
	}
	return output.String(s).Faint().String()
}

var escapePattern = regexp.MustCompile(`\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)|\x1b\[[0-9;]*[A-Za-z]`)

// VisibleWidth is the number of characters s takes on screen.
func VisibleWidth(s string) int {
	return utf8.RuneCountInString(escapePattern.ReplaceAllString(s, ""))
}
