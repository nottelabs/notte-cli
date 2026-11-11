package cmd

import (
	"errors"
	"fmt"
	"testing"

	"github.com/nottelabs/notte-cli/internal/auth"
	apierrors "github.com/nottelabs/notte-cli/internal/errors"
)

func TestIsUnauthorized(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want bool
	}{
		{auth.ErrNoAPIKey, true},
		{fmt.Errorf("not authenticated: %w", auth.ErrNoAPIKey), true},
		{&apierrors.AuthError{Reason: "expired", StatusCode: 401}, true},
		{&apierrors.APIError{StatusCode: 403, Message: "forbidden"}, true},
		{&apierrors.APIError{StatusCode: 404, Message: "not found"}, false},
		{errors.New("session ID required"), false},
	} {
		if got := isUnauthorized(tc.err); got != tc.want {
			t.Errorf("isUnauthorized(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}
