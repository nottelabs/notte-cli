package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
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
	errObj := map[string]string{"error": err.Error()}
	enc := json.NewEncoder(os.Stderr)
	if encErr := enc.Encode(errObj); encErr != nil {
		// Fallback to plain text if JSON encoding fails
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
	}
}
