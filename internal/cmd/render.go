package cmd

import (
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/muesli/termenv"

	"github.com/nottelabs/notte-cli/internal/auth"
	"github.com/nottelabs/notte-cli/internal/output"
)

// Text output is written for people, and every command follows one layout so
// that reading one teaches the rest:
//
//   - A list is a table (listTable): a gutter that marks the current item, the
//     ID column headed by the noun and the row count, the name, the state, what
//     is specific to the resource, when it was created or updated, and last a
//     link to the console.
//   - One item is a block (printDetails): a title, its ID, its state, its
//     fields, its dates, and last the console link.
//   - Both end with one "Next:" line, a command to run from here.
//   - An empty list is one "No ..." line.
//   - A command that changes something says what it did in one line, "Deleted
//     function <id>" (printDone), or that line followed by the changed item
//     (renderDone). Both end with the same "Next:" line.
//
// JSON output (-o json) is the contract for scripts and agents; it is the
// API's own data and none of the helpers here touch it. IDs are never
// shortened in either form, because both people and agents copy them into
// the next command.

// textFormatter returns the formatter for human output, or nil in JSON mode.
func textFormatter() *output.TextFormatter {
	text, _ := GetFormatter().(*output.TextFormatter)
	return text
}

// countHeader names a list's main column with how many rows follow; "+" means
// more are on the next page.
func countHeader(noun string, count int, more bool) string {
	if more {
		return fmt.Sprintf("%s (%d+)", noun, count)
	}
	return fmt.Sprintf("%s (%d)", noun, count)
}

// printDetails prints one item: a title line, then its fields as aligned
// "label value" rows. Rows without a value are left out.
func printDetails(text *output.TextFormatter, title string, rows [][2]string) error {
	width := 0
	for _, row := range rows {
		if row[1] != "" && len(row[0]) > width {
			width = len(row[0])
		}
	}
	var b strings.Builder
	if title != "" {
		b.WriteString(title + "\n")
	}
	for _, row := range rows {
		if row[1] == "" {
			continue
		}
		// Later lines of a value line up under its first.
		value := strings.ReplaceAll(row[1], "\n", "\n"+strings.Repeat(" ", width+5))
		fmt.Fprintf(&b, "  %s%s   %s\n", text.Label(row[0]), strings.Repeat(" ", width-len(row[0])), value)
	}
	_, err := io.WriteString(text.Writer, b.String())
	return err
}

// printHint ends a view with the command to run next.
func printHint(text *output.TextFormatter, command string) {
	_, _ = fmt.Fprintln(text.Writer, text.Faint("Next: "+command))
}

// printDone reports what a command changed when there is no item left to
// show: one line in text mode with the command to run next, and data in JSON
// mode (PrintResult adds the line to it as "message").
func printDone(message string, data map[string]any, next string) error {
	if err := PrintResult(message, data); err != nil {
		return err
	}
	if text := textFormatter(); text != nil && next != "" {
		printHint(text, next)
	}
	return nil
}

// renderDone reports what a command changed and prints the item it changed.
// JSON mode prints only the item, as the API returned it.
func renderDone(message string, v any) error {
	if text := textFormatter(); text != nil {
		if _, err := fmt.Fprintln(text.Writer, message); err != nil {
			return err
		}
	}
	return render(v)
}

// consoleLabel is the text of every link to the console.
const consoleLabel = "open in console ↗"

// listTable prints a list. The first column of each row is the item's ID; the
// row whose ID is current gets the "*" mark in the gutter, which is there for
// every list so that all of them line up the same way.
func listTable(text *output.TextFormatter, current string, headers []string, rows [][]string) error {
	headers = append([]string{" "}, headers...)
	for i, row := range rows {
		marker := " "
		if current != "" && len(row) > 0 && row[0] == current {
			marker = text.Colorize("*", termenv.ANSIGreen)
		}
		rows[i] = append([]string{marker}, row...)
	}
	return text.Table(headers, rows)
}

// statusText colors a status by what it means: green while things are fine,
// red for failures, yellow for anything waiting or cut short.
func statusText(text *output.TextFormatter, status string) string {
	label := strings.ReplaceAll(status, "_", " ")
	switch strings.ToLower(status) {
	case "":
		return text.Faint("-")
	case "active", "running", "enabled", "success", "succeeded", "completed", "ready", "healthy", "approved", "connected":
		return text.Colorize(label, termenv.ANSIGreen)
	case "error", "failed", "failure", "rejected", "declined", "unhealthy":
		return text.Colorize(label, termenv.ANSIRed)
	case "timed_out", "pending", "queued", "paused", "awaiting_approval", "stopping", "cancelled", "canceled":
		return text.Colorize(label, termenv.ANSIYellow)
	}
	return text.Faint(label)
}

// formatBytes prints a size the way file managers do: "1.8 kB", "48.2 kB".
func formatBytes(n int64) string {
	const unit = 1000
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "kMGTPE"[exp])
}

// formatDuration prints an elapsed time compactly: "842ms", "4.2s", "2m 30s".
func formatDuration(d time.Duration) string {
	switch {
	case d < time.Second:
		return fmt.Sprintf("%dms", d.Milliseconds())
	case d < time.Minute:
		return fmt.Sprintf("%.1fs", d.Seconds())
	case d < time.Hour:
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
}

// formatMoney prints an amount of dollars: "$41.19".
func formatMoney(amount float64) string {
	return fmt.Sprintf("$%.2f", amount)
}

// yesNo prints a flag as a word.
func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

// shorten cuts free text (never an ID) to at most n characters on one line.
func shorten(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if runes := []rune(s); len(runes) > n {
		return string(runes[:n-1]) + "…"
	}
	return s
}

// consoleLink links to a page of the console, such as "functions", "<id>".
// Paths without a workspace are sent to the signed-in user's own.
func consoleLink(path ...string) string {
	escaped := make([]string, len(path))
	for i, part := range path {
		escaped[i] = url.PathEscape(part)
	}
	return output.Link(strings.TrimSuffix(auth.ConsoleURL(), "/")+"/"+strings.Join(escaped, "/"), consoleLabel)
}

// deref returns the value a pointer holds, or the zero value for nil.
func deref[T any](p *T) T {
	if p == nil {
		var zero T
		return zero
	}
	return *p
}
