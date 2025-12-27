// internal/cmd/validation_test.go
package cmd

import (
	"errors"
	"testing"
)

func TestValidateFlags(t *testing.T) {
	t.Run("all pass", func(t *testing.T) {
		err := ValidateFlags(
			func() error { return nil },
			func() error { return nil },
		)
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})

	t.Run("first fails", func(t *testing.T) {
		firstErr := errors.New("first error")
		err := ValidateFlags(
			func() error { return firstErr },
			func() error { return nil },
		)
		if err != firstErr {
			t.Errorf("expected first error, got %v", err)
		}
	})

	t.Run("second fails", func(t *testing.T) {
		secondErr := errors.New("second error")
		err := ValidateFlags(
			func() error { return nil },
			func() error { return secondErr },
		)
		if err != secondErr {
			t.Errorf("expected second error, got %v", err)
		}
	})

	t.Run("empty validators", func(t *testing.T) {
		err := ValidateFlags()
		if err != nil {
			t.Errorf("expected nil for empty validators, got %v", err)
		}
	})
}
