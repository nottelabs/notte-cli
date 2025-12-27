// internal/testutil/command_test.go
package testutil

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRunCommand_CapturesOutput(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Println("hello stdout")
			cmd.PrintErr("hello stderr")
		},
	}

	result := RunCommand(t, cmd, []string{})

	if !strings.Contains(result.Stdout, "hello stdout") {
		t.Errorf("stdout should contain 'hello stdout', got %q", result.Stdout)
	}
	if !strings.Contains(result.Stderr, "hello stderr") {
		t.Errorf("stderr should contain 'hello stderr', got %q", result.Stderr)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRunCommand_CapturesError(t *testing.T) {
	cmd := &cobra.Command{
		Use: "test",
		RunE: func(cmd *cobra.Command, args []string) error {
			return &testError{"test error"}
		},
	}

	result := RunCommand(t, cmd, []string{})

	if result.Err == nil {
		t.Error("expected error")
	}
	if result.Err.Error() != "test error" {
		t.Errorf("got error %q, want 'test error'", result.Err.Error())
	}
}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }
