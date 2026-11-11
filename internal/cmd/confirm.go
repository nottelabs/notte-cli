// internal/cmd/confirm.go
package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// skipConfirmation is set by --yes flag to skip prompts
var skipConfirmation bool

// ConfirmAction prompts the user to confirm a destructive action.
// Returns true if confirmed, false otherwise.
//
// Without a terminal there is nobody to ask: it fails, naming --yes, instead
// of waiting on input that never comes or reporting a delete that did not
// happen as a success.
func ConfirmAction(resource, id string) (bool, error) {
	if skipConfirmation {
		return true, nil
	}
	if !canPrompt() {
		return false, &confirmationError{action: fmt.Sprintf("delete %s %s", resource, id)}
	}
	return ConfirmActionWithIO(os.Stdin, os.Stderr, resource, id)
}

// canPrompt reports whether a question on the terminal can be answered:
// input is a terminal, or a front end relays typed input (see
// forcedInteractive).
func canPrompt() bool {
	return forcedInteractive() || term.IsTerminal(int(os.Stdin.Fd()))
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

// ConfirmStop prompts the user to confirm stopping a resource.
// Defaults to "yes" if user just presses Enter.
// Returns true if confirmed, false otherwise.
//
// Stopping is the default answer, so without a terminal it stops.
func ConfirmStop(resource, id string) (bool, error) {
	if skipConfirmation || !canPrompt() {
		return true, nil
	}
	return ConfirmStopWithIO(os.Stdin, os.Stderr, resource, id)
}

// ConfirmStopWithIO is the testable version of ConfirmStop.
func ConfirmStopWithIO(in io.Reader, out io.Writer, resource, id string) (bool, error) {
	if _, err := fmt.Fprintf(out, "Stop %s %s? This cannot be undone. [Y/n]: ", resource, id); err != nil {
		return false, fmt.Errorf("failed to write prompt: %w", err)
	}

	reader := bufio.NewReader(in)
	response, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return false, fmt.Errorf("failed to read response: %w", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))
	// Default to yes if empty, or accept explicit "y" or "yes"
	// Only "n" or "no" will cancel
	return response != "n" && response != "no", nil
}
