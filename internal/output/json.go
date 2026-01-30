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
		enc := json.NewEncoder(os.Stderr)
		if encErr := enc.Encode(errObj); encErr != nil {
			fmt.Fprintf(os.Stderr, "Error %d: %s\n", authErr.StatusCode, err.Error())
		}
		return
	}

	errObj := map[string]string{"error": err.Error()}
	enc := json.NewEncoder(os.Stderr)
	if encErr := enc.Encode(errObj); encErr != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
	}
}
