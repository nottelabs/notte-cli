// internal/testutil/testutil.go
package testutil

import (
	"bytes"
	"os"
	"testing"
)

// TestEnv holds test environment state
type TestEnv struct {
	t         *testing.T
	origEnv   map[string]string
	TempDir   string
	MockStore *MockKeyring
}

// SetupTestEnv creates an isolated test environment
func SetupTestEnv(t *testing.T) *TestEnv {
	t.Helper()

	env := &TestEnv{
		t:         t,
		origEnv:   make(map[string]string),
		TempDir:   t.TempDir(),
		MockStore: NewMockKeyring(),
	}

	// Clear auth-related env vars
	for _, key := range []string{"NOTTE_API_KEY", "NOTTE_API_URL"} {
		env.origEnv[key] = os.Getenv(key)
		_ = os.Unsetenv(key)
	}

	t.Cleanup(func() {
		for key, val := range env.origEnv {
			if val != "" {
				_ = os.Setenv(key, val)
			} else {
				_ = os.Unsetenv(key)
			}
		}
	})

	return env
}

// SetEnv sets an environment variable for the test duration
func (e *TestEnv) SetEnv(key, value string) {
	e.t.Helper()
	if _, exists := e.origEnv[key]; !exists {
		e.origEnv[key] = os.Getenv(key)
	}
	_ = os.Setenv(key, value)
}

// CaptureOutput captures stdout and stderr during function execution
func CaptureOutput(fn func()) (stdout, stderr string) {
	oldStdout := os.Stdout
	oldStderr := os.Stderr

	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()

	os.Stdout = wOut
	os.Stderr = wErr

	fn()

	_ = wOut.Close()
	_ = wErr.Close()

	var bufOut, bufErr bytes.Buffer
	_, _ = bufOut.ReadFrom(rOut)
	_, _ = bufErr.ReadFrom(rErr)

	os.Stdout = oldStdout
	os.Stderr = oldStderr

	return bufOut.String(), bufErr.String()
}
