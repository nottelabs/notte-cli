package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"

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
	if resp.Structured != nil {
		// Check success field
		if resp.Structured.Success != nil && !*resp.Structured.Success {
			if resp.Structured.Error != nil {
				return fmt.Errorf("%s", *resp.Structured.Error)
			}
			return fmt.Errorf("scrape failed")
		}
		// Return data if present
		if resp.Structured.Data != nil {
			// Extract actual data from the union type wrapper
			if data, err := resp.Structured.Data.AsBaseModel(); err == nil && data != nil {
				// Print with a nice header for scrape results
				fmt.Println("Scraped content from the current page:")
				fmt.Println()
				// Print as indented JSON for better readability
				jsonBytes, err := json.MarshalIndent(data, "", "  ")
				if err != nil {
					return GetFormatter().Print(data)
				}
				fmt.Println(string(jsonBytes))
				return nil
			}
		}
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

// printSessionStatus formats session status output with simplified Steps display.
// In JSON mode, returns the full response. In text mode, formats Steps as a simple list.
func printSessionStatus(resp *api.SessionResponse) error {
	if IsJSONOutput() {
		return GetFormatter().Print(resp)
	}

	// Create a copy of the response data without Steps
	v := reflect.ValueOf(resp).Elem()
	t := v.Type()

	// Print all fields except Steps using the standard formatter
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}

		// Skip Steps field - we'll handle it separately
		if field.Name == "Steps" {
			continue
		}

		fieldValue := v.Field(i)

		// Skip nil pointers, nil slices, nil maps, and nil interfaces
		switch fieldValue.Kind() {
		case reflect.Ptr, reflect.Slice, reflect.Map, reflect.Interface:
			if fieldValue.IsNil() {
				continue
			}
		}

		var displayValue any
		if fieldValue.Kind() == reflect.Ptr {
			displayValue = fieldValue.Elem().Interface()
		} else {
			displayValue = fieldValue.Interface()
		}

		fmt.Printf("%-23s %v\n", field.Name+":", displayValue)
	}

	// Now handle Steps specially
	if resp.Steps != nil && len(*resp.Steps) > 0 {
		fmt.Printf("%-23s ", "Steps:")
		var stepSummaries []string
		for _, step := range *resp.Steps {
			stepType, _ := step["type"].(string)
			switch stepType {
			case "execution_result":
				// For execution results, show only the action type
				if value, ok := step["value"].(map[string]any); ok {
					if action, ok := value["action"].(map[string]any); ok {
						if actionType, ok := action["type"].(string); ok {
							stepSummaries = append(stepSummaries, actionType)
						}
					}
				}
			case "observation":
				// For observations, just say "observation"
				stepSummaries = append(stepSummaries, "observation")
			default:
				// For other types, show the type
				stepSummaries = append(stepSummaries, stepType)
			}
		}
		fmt.Printf("[%s]\n", strings.Join(stepSummaries, " "))
	}

	return nil
}
