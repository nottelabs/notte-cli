package cmd

import (
	"fmt"
	"os"
	"reflect"

	"github.com/nottelabs/notte-cli/internal/api"
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
func PrintListOrEmpty(items any, emptyMsg string) (bool, error) {
	if items == nil {
		if IsJSONOutput() {
			return true, GetFormatter().Print([]any{})
		}
		if emptyMsg != "" {
			_, _ = fmt.Fprintln(os.Stdout, emptyMsg)
		}
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
		if emptyMsg != "" {
			_, _ = fmt.Fprintln(os.Stdout, emptyMsg)
		}
		return true, nil
	}

	return false, nil
}

// PrintScrapeResponse formats scrape output consistently across all scrape commands.
// In JSON mode, returns the full response. In text mode without instructions,
// returns just the markdown. With instructions, checks data.success and returns
// the extracted data or an error message.
func PrintScrapeResponse(resp *api.ScrapeResponse, hasInstructions bool) error {
	// JSON mode: return full response
	if IsJSONOutput() {
		return GetFormatter().Print(resp)
	}

	if !hasInstructions {
		// Simple mode: just return markdown
		fmt.Println(resp.Markdown)
		return nil
	}

	// Structured mode: check data.success
	if data, ok := resp.Structured.(map[string]any); ok {
		if success, ok := data["success"].(bool); ok && !success {
			if errMsg, ok := data["error"].(string); ok {
				return fmt.Errorf("%s", errMsg)
			}
			return fmt.Errorf("scrape failed")
		}
		if resultData, ok := data["data"]; ok {
			return GetFormatter().Print(resultData)
		}
	}
	return GetFormatter().Print(resp.Structured)
}
