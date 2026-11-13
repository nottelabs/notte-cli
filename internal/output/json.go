package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	apierrors "github.com/nottelabs/notte-cli/internal/errors"
)

// JSONFormatter outputs data as JSON
type JSONFormatter struct {
	Writer io.Writer
	// ErrorCode, when set, is added to printed errors as "code": a stable name
	// for the kind of failure ("unauthorized", "not_found", "usage", ...).
	ErrorCode string
}

func (f *JSONFormatter) Print(data any) error {
	enc := json.NewEncoder(f.Writer)
	return enc.Encode(data)
}

func (f *JSONFormatter) PrintError(err error) {
	// For API errors, include status code and message
	if apiErr, ok := err.(*apierrors.APIError); ok && apiErr.Message != "" {
		errObj := map[string]any{
			"error":       apiErr.Message,
			"status_code": apiErr.StatusCode,
		}
		if f.ErrorCode != "" {
			errObj["code"] = f.ErrorCode
		}
		enc := json.NewEncoder(os.Stderr)
		if encErr := enc.Encode(errObj); encErr != nil {
			fmt.Fprintf(os.Stderr, "Error %d: %s\n", apiErr.StatusCode, apiErr.Message)
		}
		return
	}

	// For auth errors, include status code, reason, and message
	if authErr, ok := err.(*apierrors.AuthError); ok {
		errObj := map[string]any{
			"error":       authErr.Reason,
			"status_code": authErr.StatusCode,
		}
		if authErr.Message != "" {
			errObj["message"] = authErr.Message
		}
		if authErr.Hint != "" {
			errObj["hint"] = authErr.Hint
		}
		if f.ErrorCode != "" {
			errObj["code"] = f.ErrorCode
		}
		enc := json.NewEncoder(os.Stderr)
		if encErr := enc.Encode(errObj); encErr != nil {
			fmt.Fprintf(os.Stderr, "Error %d: %s\n", authErr.StatusCode, err.Error())
		}
		return
	}

	errObj := map[string]string{"error": err.Error()}
	if f.ErrorCode != "" {
		errObj["code"] = f.ErrorCode
	}
	enc := json.NewEncoder(os.Stderr)
	if encErr := enc.Encode(errObj); encErr != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
	}
}
