package cmd

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/nottelabs/notte-cli/internal/output"
)

func TestGetFormatter_NoColor(t *testing.T) {
	origFormat := outputFormat
	origNoColor := noColor
	t.Cleanup(func() {
		outputFormat = origFormat
		noColor = origNoColor
	})

	outputFormat = "text"
	noColor = true

	f := GetFormatter()
	tf, ok := f.(*output.TextFormatter)
	if !ok {
		t.Fatalf("expected TextFormatter, got %T", f)
	}
	if !tf.NoColor {
		t.Fatal("expected NoColor to be true")
	}
}

func TestIsVerbose(t *testing.T) {
	origVerbose := verbose
	t.Cleanup(func() { verbose = origVerbose })

	verbose = true
	if !IsVerbose() {
		t.Fatal("expected IsVerbose true")
	}
}

func TestGetContextWithTimeout(t *testing.T) {
	origTimeout := requestTimeout
	t.Cleanup(func() { requestTimeout = origTimeout })

	requestTimeout = 1
	ctx, cancel := GetContextWithTimeout(context.Background())
	t.Cleanup(cancel)

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected deadline to be set")
	}
	if time.Until(deadline) <= 0 {
		t.Fatal("expected deadline in the future")
	}
}

func TestExecute_ErrorExit(t *testing.T) {
	if os.Getenv("NOTTE_EXECUTE_EXIT_TEST") == "1" {
		rootCmd.SetArgs([]string{"does-not-exist"})
		Execute()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestExecute_ErrorExit")
	cmd.Env = append(os.Environ(), "NOTTE_EXECUTE_EXIT_TEST=1")
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected non-zero exit")
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() != 1 {
			t.Fatalf("expected exit code 1, got %d", exitErr.ExitCode())
		}
		return
	}
	t.Fatalf("unexpected error: %v", err)
}
