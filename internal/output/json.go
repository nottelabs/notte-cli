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
	errObj := map[string]string{"error": err.Error()}
	enc := json.NewEncoder(os.Stderr)
	if encErr := enc.Encode(errObj); encErr != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
	}
}
