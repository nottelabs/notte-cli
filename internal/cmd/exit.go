package cmd

import (
	"errors"
	"fmt"
	"strings"

	apierrors "github.com/nottelabs/notte-cli/internal/errors"
)

// Exit codes. Scripts and agents branch on these, and on the matching "code"
// field of a JSON error, instead of parsing messages.
const (
	ExitFailure     = 1 // any failure without a code of its own
	ExitUsage       = 2 // wrong flags or arguments; nothing was sent
	ExitAuth        = 3 // not signed in, or the API rejected the key
	ExitNotFound    = 4 // the API has no such resource
	ExitConfirm     = 5 // a destructive command needs --yes
	ExitRateLimited = 6 // too many requests; retry later
	ExitRunFailed   = 7 // the function run that was waited for failed
)

// usageError marks a mistake in how the command was called.
type usageError struct{ err error }

func (e *usageError) Error() string { return e.err.Error() }
func (e *usageError) Unwrap() error { return e.err }

// errUsage builds a usageError like fmt.Errorf.
func errUsage(format string, args ...any) error {
	return &usageError{err: fmt.Errorf(format, args...)}
}

// confirmationError is returned when a destructive command would have to ask
// and nobody is there to answer.
type confirmationError struct{ action string }

func (e *confirmationError) Error() string {
	return fmt.Sprintf("refusing to %s without confirmation: pass --yes (or -y)", e.action)
}

// runFailedError is returned once a function run that the command waited for
// has failed. The run itself was already printed.
type runFailedError struct{ runID string }

func (e *runFailedError) Error() string {
	return fmt.Sprintf("function run %s failed", e.runID)
}

// cobraUsagePrefixes start the errors cobra returns for a bad command line
// before any command code runs. Flag errors are wrapped where they are raised.
var cobraUsagePrefixes = []string{
	"unknown command", "required flag(s)", "accepts ", "requires at least", "requires at most",
	"requires exactly", "invalid argument", "if any flags in the group", "at least one of the flags in the group",
	"exactly one of the flags in the group", "only one of the flags in the group",
}

// classifyError names err for machines: the "code" of a JSON error and the
// process exit code.
func classifyError(err error) (string, int) {
	var usage *usageError
	var confirm *confirmationError
	var rateLimit *apierrors.RateLimitError
	var runFailed *runFailedError
	var apiErr *apierrors.APIError
	switch {
	case errors.As(err, &usage):
		return "usage", ExitUsage
	case errors.As(err, &confirm):
		return "confirmation_required", ExitConfirm
	case errors.As(err, &runFailed):
		return "run_failed", ExitRunFailed
	case isUnauthorized(err):
		return "unauthorized", ExitAuth
	case errors.As(err, &rateLimit):
		return "rate_limited", ExitRateLimited
	case errors.As(err, &apiErr):
		if apiErr.StatusCode == 404 {
			return "not_found", ExitNotFound
		}
		return "api_error", ExitFailure
	}
	for _, prefix := range cobraUsagePrefixes {
		if strings.HasPrefix(err.Error(), prefix) {
			return "usage", ExitUsage
		}
	}
	return "error", ExitFailure
}
