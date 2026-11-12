package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/nottelabs/notte-cli/internal/api"
	"github.com/nottelabs/notte-cli/internal/output"
)

// IsJSONOutput returns true if the global output format is set to JSON.
func IsJSONOutput() bool {
	return outputFormat == "json"
}

// PrintInfo prints an informational message to stdout in text mode,
// or to stderr in JSON mode to keep stdout clean for machine parsing.
func PrintInfo(message string) {
	if IsJSONOutput() {
		_, _ = fmt.Fprintln(os.Stderr, message)
		return
	}
	_, _ = fmt.Fprintln(os.Stdout, message)
}

// PrintResult prints a success result. In JSON mode, outputs structured data
// to stdout. In text mode, prints the human-readable message.
func PrintResult(message string, data map[string]any) error {
	if IsJSONOutput() {
		if data == nil {
			data = map[string]any{}
		}
		if _, ok := data["message"]; !ok && message != "" {
			data["message"] = message
		}
		return GetFormatter().Print(data)
	}

	if message == "" {
		return nil
	}
	_, err := fmt.Fprintln(os.Stdout, message)
	return err
}

// PrintListOrEmpty handles empty or nil slice output. If the slice is nil or empty,
// it prints an empty JSON array in JSON mode or the provided message in text mode.
// Returns (true, nil) if output was handled, (false, nil) if the caller should handle
// non-empty output, or (false, error) if items is not a slice type.
// withEmptyFace ends an empty-state message with output.FaceEmpty in place of
// its final period: "No active sessions (◡︵◡)".
func withEmptyFace(message string) string {
	return strings.TrimSuffix(message, ".") + " " + output.FaceEmpty
}

func PrintListOrEmpty(items any, emptyMsg string, next ...string) (bool, error) {
	// In text mode an empty list is one "No ..." line, then, like every other
	// view, the command to run from here.
	printEmpty := func() {
		if emptyMsg != "" {
			_, _ = fmt.Fprintln(os.Stdout, withEmptyFace(emptyMsg))
		}
		if text := textFormatter(); text != nil {
			for _, command := range next {
				printHint(text, command)
			}
		}
	}
	if items == nil {
		if IsJSONOutput() {
			return true, GetFormatter().Print([]any{})
		}
		printEmpty()
		return true, nil
	}

	v := reflect.ValueOf(items)
	if v.Kind() != reflect.Slice {
		return false, fmt.Errorf("PrintListOrEmpty: expected slice, got %s", v.Kind())
	}

	if v.Len() == 0 {
		if IsJSONOutput() {
			empty := reflect.MakeSlice(v.Type(), 0, 0).Interface()
			return true, GetFormatter().Print(empty)
		}
		printEmpty()
		return true, nil
	}

	return false, nil
}

// PrintScrapeResponse formats scrape output consistently across all scrape commands.
// In JSON mode without instructions, returns the full response. With instructions,
// returns the extracted structured data directly. In text mode without instructions,
// returns just the markdown.
func PrintScrapeResponse(resp *api.DataSpace, hasInstructions bool) error {
	if IsJSONOutput() {
		if hasInstructions {
			data, err := extractScrapeStructuredData(resp)
			if err != nil {
				return err
			}
			return GetFormatter().Print(data)
		}
		return GetFormatter().Print(resp)
	}

	if !hasInstructions {
		// Simple mode: just return markdown
		fmt.Println(resp.Markdown)
		return nil
	}

	// Structured mode: check data.success
	data, err := extractScrapeStructuredData(resp)
	if err == nil {
		fmt.Println("Scraped content from the current page:")
		fmt.Println()
		jsonBytes, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return GetFormatter().Print(data)
		}
		fmt.Println(string(jsonBytes))
		return nil
	}
	if resp != nil && resp.Structured != nil && resp.Structured.Success != nil && !*resp.Structured.Success {
		return err
	}

	fmt.Println("Scraped content from the current page:")
	fmt.Println()
	// Print as indented JSON for better readability
	jsonBytes, err := json.MarshalIndent(resp.Structured, "", "  ")
	if err != nil {
		return GetFormatter().Print(resp.Structured)
	}
	fmt.Println(string(jsonBytes))
	return nil
}

// extractScrapeStructuredData returns the direct payload from structured scrape
// responses, matching the SDK default for instruction-based scrapes.
func extractScrapeStructuredData(resp *api.DataSpace) (any, error) {
	if resp == nil || resp.Structured == nil {
		return nil, fmt.Errorf("scrape did not return structured data")
	}
	if resp.Structured.Success != nil && !*resp.Structured.Success {
		if resp.Structured.Error != nil && *resp.Structured.Error != "" {
			return nil, fmt.Errorf("%s", *resp.Structured.Error)
		}
		return nil, fmt.Errorf("scrape failed")
	}
	if resp.Structured.Data == nil {
		return nil, fmt.Errorf("scrape did not return structured data")
	}

	raw, err := json.Marshal(resp.Structured.Data)
	if err != nil {
		return nil, err
	}
	var data any
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	return data, nil
}

// printSessionStatus prints one session: its state, how it is set up, what it
// has done so far and where to watch it. JSON mode prints the full response.
func printSessionStatus(resp *api.SessionResponse) error {
	text := textFormatter()
	if text == nil || resp == nil {
		return GetFormatter().Print(resp)
	}
	now := time.Now()

	browser := "chromium"
	if resp.BrowserType != nil && *resp.BrowserType != "" {
		browser = string(*resp.BrowserType)
	}
	if resp.ViewportWidth != nil && resp.ViewportHeight != nil {
		browser += fmt.Sprintf(" · %d×%d", *resp.ViewportWidth, *resp.ViewportHeight)
	}
	if deref(resp.Proxies) {
		browser += " · proxies"
	}
	if deref(resp.SolveCaptchas) {
		browser += " · captcha solving"
	}

	stops := fmt.Sprintf("after %d min idle", resp.IdleTimeoutMinutes)
	if resp.MaxDurationMinutes != nil {
		stops += fmt.Sprintf(", %d min max", *resp.MaxDurationMinutes)
	}
	closed := deref(resp.CloseReason)
	if t, err := time.Parse(time.RFC3339, deref(resp.ClosedAt)); err == nil {
		closed = strings.TrimSpace(relativeTime(t, now) + "  " + closed)
	}
	if resp.Status != api.SessionResponseStatusActive {
		stops = ""
	}

	network := ""
	if resp.NetworkRequestBytes != nil || resp.NetworkResponseBytes != nil {
		network = fmt.Sprintf("%s sent, %s received", formatBytes(int64(deref(resp.NetworkRequestBytes))), formatBytes(int64(deref(resp.NetworkResponseBytes))))
	}

	// Steps are summarized by what each one did, in order.
	var steps []string
	for _, step := range deref(resp.Steps) {
		stepType, _ := step["type"].(string)
		if stepType == "execution_result" {
			if value, ok := step["value"].(map[string]any); ok {
				if action, ok := value["action"].(map[string]any); ok {
					if actionType, ok := action["type"].(string); ok {
						stepType = actionType
					}
				}
			}
		}
		if stepType != "" {
			steps = append(steps, stepType)
		}
	}
	stepSummary := ""
	if len(steps) > 0 {
		stepSummary = fmt.Sprintf("%d: %s", len(steps), strings.Join(steps, " → "))
	}

	// The live viewer exists only while the session runs.
	viewer := ""
	if resp.Status == api.SessionResponseStatusActive && deref(resp.ViewerUrl) != "" {
		viewer = output.Link(*resp.ViewerUrl, "open live viewer ↗")
	}
	if err := printDetails(text, "Session "+resp.SessionId, [][2]string{
		{"Status", sessionStatus(text, string(resp.Status))},
		{"Error", deref(resp.Error)},
		{"Browser", browser},
		{"Running for", deref(resp.Duration)},
		{"Stops", stops},
		{"Closed", closed},
		{"Network", network},
		{"Steps", stepSummary},
		{"Last active", relativeTime(resp.LastAccessedAt.Time, now)},
		{"Created", relativeTime(resp.CreatedAt.Time, now)},
		{"Viewer", viewer},
		{"Console", consoleLink("sessions", resp.SessionId)},
	}); err != nil {
		return err
	}
	if resp.Status == api.SessionResponseStatusActive {
		printHint(text, "notte page observe --session-id "+resp.SessionId)
	} else {
		printHint(text, "notte sessions start")
	}
	return nil
}

// executionFailureError builds the error for a failed page action.
//
// The API serializes failures twice: `exception` is a bare string rendered in
// whatever ErrorConfig mode the server ran in (often the generic user-facing
// sentence), while `exception_detail` carries the concrete error type, the
// per-audience messages and the retry/notify flags. Prefer the structured
// field so the CLI reports what actually went wrong, and fall back to the
// legacy string for API builds that predate it.
func executionFailureError(detail *api.SerializedError, exception *string, message string) error {
	if detail != nil {
		reason := detail.DevMessage
		if reason == "" {
			reason = detail.UserMessage
		}
		if reason == "" {
			reason = message
		}
		if reason == "" {
			return fmt.Errorf("%s", detail.ErrorType)
		}
		if detail.ErrorType != "" {
			return fmt.Errorf("%s: %s", detail.ErrorType, reason)
		}
		return fmt.Errorf("%s", reason)
	}

	if exception != nil && *exception != "" {
		if message != "" && *exception != message {
			return fmt.Errorf("%s: %s", *exception, message)
		}
		return fmt.Errorf("%s", *exception)
	}
	if message != "" {
		return fmt.Errorf("action failed: %s", message)
	}
	return fmt.Errorf("action failed")
}
