// internal/validate/validate.go
package validate

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"time"
)

// URL validates that a string is a valid HTTP/HTTPS URL
func URL(s string) error {
	if s == "" {
		return fmt.Errorf("URL cannot be empty")
	}

	u, err := url.Parse(s)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("URL must use http or https scheme, got %q", u.Scheme)
	}

	if u.Host == "" {
		return fmt.Errorf("URL must have a host")
	}

	return nil
}

// JSON validates that a string is valid JSON
func JSON(s string) error {
	if s == "" {
		return fmt.Errorf("JSON cannot be empty")
	}

	var v interface{}
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	return nil
}

// Browser validates browser type
func Browser(s string) error {
	valid := map[string]bool{
		"chromium": true,
		"firefox":  true,
		"webkit":   true,
	}

	if !valid[s] {
		return fmt.Errorf("invalid browser: expected chromium|firefox|webkit, got %q", s)
	}

	return nil
}

// PositiveInt validates that n > 0
func PositiveInt(n int) error {
	if n <= 0 {
		return fmt.Errorf("value must be positive, got %d", n)
	}
	return nil
}

// Duration validates that s is a valid Go duration string
func Duration(s string) error {
	if s == "" {
		return fmt.Errorf("duration cannot be empty")
	}

	_, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}

	return nil
}

// OutputFormat validates output format flag
func OutputFormat(s string) error {
	valid := map[string]bool{
		"text": true,
		"json": true,
	}

	if !valid[s] {
		return fmt.Errorf("invalid output format: expected text|json, got %q", s)
	}

	return nil
}

// NonEmpty validates that a string is not empty
func NonEmpty(s, name string) error {
	if s == "" {
		return fmt.Errorf("%s cannot be empty", name)
	}
	return nil
}

var (
	sessionIDPattern  = regexp.MustCompile(`^sess_[a-zA-Z0-9]{1,64}$`)
	agentIDPattern    = regexp.MustCompile(`^agent_[a-zA-Z0-9]{1,64}$`)
	workflowIDPattern = regexp.MustCompile(`^wf_[a-zA-Z0-9]{1,64}$`)
	vaultIDPattern    = regexp.MustCompile(`^vault_[a-zA-Z0-9]{1,64}$`)
	personaIDPattern  = regexp.MustCompile(`^persona_[a-zA-Z0-9]{1,64}$`)
)

// SessionID validates that a string is a valid Notte session ID
func SessionID(s string) error {
	if s == "" {
		return fmt.Errorf("session ID cannot be empty")
	}
	if !sessionIDPattern.MatchString(s) {
		return fmt.Errorf("invalid session ID: expected sess_<alphanumeric 1-64 chars>, got %q", s)
	}
	return nil
}

// AgentID validates that a string is a valid Notte agent ID
func AgentID(s string) error {
	if s == "" {
		return fmt.Errorf("agent ID cannot be empty")
	}
	if !agentIDPattern.MatchString(s) {
		return fmt.Errorf("invalid agent ID: expected agent_<alphanumeric 1-64 chars>, got %q", s)
	}
	return nil
}

// WorkflowID validates that a string is a valid Notte workflow ID
func WorkflowID(s string) error {
	if s == "" {
		return fmt.Errorf("workflow ID cannot be empty")
	}
	if !workflowIDPattern.MatchString(s) {
		return fmt.Errorf("invalid workflow ID: expected wf_<alphanumeric 1-64 chars>, got %q", s)
	}
	return nil
}

// VaultID validates that a string is a valid Notte vault ID
func VaultID(s string) error {
	if s == "" {
		return fmt.Errorf("vault ID cannot be empty")
	}
	if !vaultIDPattern.MatchString(s) {
		return fmt.Errorf("invalid vault ID: expected vault_<alphanumeric 1-64 chars>, got %q", s)
	}
	return nil
}

// PersonaID validates that a string is a valid Notte persona ID
func PersonaID(s string) error {
	if s == "" {
		return fmt.Errorf("persona ID cannot be empty")
	}
	if !personaIDPattern.MatchString(s) {
		return fmt.Errorf("invalid persona ID: expected persona_<alphanumeric 1-64 chars>, got %q", s)
	}
	return nil
}
