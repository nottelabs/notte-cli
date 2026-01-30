// internal/cmd/validation.go
package cmd

import (
	"github.com/nottelabs/notte-cli/internal/validate"
)

// ValidateFlags runs a series of validation functions, returning the first error
func ValidateFlags(validators ...func() error) error {
	for _, v := range validators {
		if err := v(); err != nil {
			return err
		}
	}
	return nil
}

// Common validation wrappers for flags

// ValidateSessionID returns a validator for session ID
func ValidateSessionID(id string) func() error {
	return func() error {
		return validate.SessionID(id)
	}
}

// ValidateBrowser returns a validator for browser type
func ValidateBrowser(browser string) func() error {
	return func() error {
		return validate.Browser(browser)
	}
}

// ValidateURL returns a validator for URLs
func ValidateURL(url, name string) func() error {
	return func() error {
		if url == "" {
			return nil // Optional URLs are OK when empty
		}
		return validate.URL(url)
	}
}

// ValidateRequiredURL returns a validator for required URLs
func ValidateRequiredURL(url, name string) func() error {
	return func() error {
		if url == "" {
			return validate.NonEmpty(url, name)
		}
		return validate.URL(url)
	}
}

// ValidateJSON returns a validator for JSON strings
func ValidateJSON(s, name string) func() error {
	return func() error {
		if s == "" {
			return nil // Optional JSON is OK when empty
		}
		return validate.JSON(s)
	}
}

// ValidateOutputFormat returns a validator for output format
func ValidateOutputFormat(format string) func() error {
	return func() error {
		return validate.OutputFormat(format)
	}
}
