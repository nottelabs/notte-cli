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

// confirmReplaceSession prompts the user to confirm stopping an existing session before starting a new one.
// Defaults to "yes" if user just presses Enter.
func confirmReplaceSession(id string) (bool, error) {
	if skipConfirmation {
		return true, nil
	}
	return confirmReplaceSessionWithIO(os.Stdin, os.Stderr, id)
}

// confirmReplaceSessionWithIO is the testable version of confirmReplaceSession.
func confirmReplaceSessionWithIO(in io.Reader, out io.Writer, id string) (bool, error) {
	if _, err := fmt.Fprintf(out, "Session %s is currently active. A new session will be created either way.\nStop the existing session before starting the new one? [Y/n]: ", id); err != nil {
		return false, fmt.Errorf("failed to write prompt: %w", err)
	}

	reader := bufio.NewReader(in)
	response, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return false, fmt.Errorf("failed to read response: %w", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response != "n" && response != "no", nil
}

// confirmReplaceAgent prompts the user to confirm stopping an existing agent before starting a new one.
// Defaults to "yes" if user just presses Enter.
func confirmReplaceAgent(id string) (bool, error) {
	if skipConfirmation {
		return true, nil
	}
	return confirmReplaceAgentWithIO(os.Stdin, os.Stderr, id)
}

// confirmReplaceAgentWithIO is the testable version of confirmReplaceAgent.
func confirmReplaceAgentWithIO(in io.Reader, out io.Writer, id string) (bool, error) {
	if _, err := fmt.Fprintf(out, "Agent %s is currently active. A new agent will be started either way.\nStop the existing agent before starting the new one? [Y/n]: ", id); err != nil {
		return false, fmt.Errorf("failed to write prompt: %w", err)
	}

	reader := bufio.NewReader(in)
	response, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return false, fmt.Errorf("failed to read response: %w", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response != "n" && response != "no", nil
}

// SetSkipConfirmation sets whether to skip confirmation prompts (for --yes flag).
func SetSkipConfirmation(skip bool) {
	skipConfirmation = skip
}

// ConfirmStop prompts the user to confirm stopping a resource.
// Defaults to "yes" if user just presses Enter.
// Returns true if confirmed, false otherwise.
func ConfirmStop(resource, id string) (bool, error) {
	if skipConfirmation {
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
