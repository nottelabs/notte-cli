// internal/cmd/confirm.go
package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// skipConfirmation is set by --yes flag to skip prompts
var skipConfirmation bool

// ConfirmAction prompts the user to confirm a destructive action.
// Returns true if confirmed, false otherwise.
func ConfirmAction(resource, id string) (bool, error) {
	if skipConfirmation {
		return true, nil
	}
	return ConfirmActionWithIO(os.Stdin, os.Stderr, resource, id)
}

// ConfirmActionWithIO is the testable version of ConfirmAction.
func ConfirmActionWithIO(in io.Reader, out io.Writer, resource, id string) (bool, error) {
	if _, err := fmt.Fprintf(out, "Delete %s %s? This cannot be undone. [y/N]: ", resource, id); err != nil {
		return false, fmt.Errorf("failed to write prompt: %w", err)
	}

	reader := bufio.NewReader(in)
	response, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return false, fmt.Errorf("failed to read response: %w", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes", nil
}

// SetSkipConfirmation sets whether to skip confirmation prompts (for --yes flag).
func SetSkipConfirmation(skip bool) {
	skipConfirmation = skip
}
