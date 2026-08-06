package cmd

import (
	"testing"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/api"
)

// newOptOutCmd builds a command carrying both the generated flags these
// opt-outs invert and the opt-outs themselves.
func newOptOutCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "start", RunE: func(*cobra.Command, []string) error { return nil }}
	cmd.Flags().BoolVar(&SessionStartHeadless, "headless", false, "headless")
	cmd.Flags().BoolVar(&SessionStartSolveCaptchas, "solve-captchas", false, "solve captchas")
	cmd.Flags().BoolVar(&SessionStartUseFileStorage, "use-file-storage", false, "file storage")
	registerSessionStartOptOutFlags(cmd)
	return cmd
}

func runOptOut(t *testing.T, args ...string) (*api.ApiSessionStartRequest, error) {
	t.Helper()
	// Package-level flag vars are shared; reset before each case.
	sessionsStartHeaded = false
	sessionsStartNoSolveCaptchas = false
	sessionsStartNoFileStorage = false

	cmd := newOptOutCmd()
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute(%v) error = %v", args, err)
	}
	body := &api.ApiSessionStartRequest{}
	return body, applySessionStartOptOuts(cmd, body)
}

// TestOptOutsLeaveBodyUntouchedWhenAbsent is the important one: omitting the
// flags must send nothing at all, so the server default applies. Sending an
// explicit value would freeze today's default into the client.
func TestOptOutsLeaveBodyUntouchedWhenAbsent(t *testing.T) {
	body, err := runOptOut(t)
	if err != nil {
		t.Fatalf("applySessionStartOptOuts() error = %v", err)
	}
	if body.Headless != nil {
		t.Errorf("Headless = %v, want nil (unset) when --headed is omitted", *body.Headless)
	}
	if body.SolveCaptchas != nil {
		t.Errorf("SolveCaptchas = %v, want nil when --no-solve-captchas is omitted", *body.SolveCaptchas)
	}
	if body.UseFileStorage != nil {
		t.Errorf("UseFileStorage = %v, want nil when --no-file-storage is omitted", *body.UseFileStorage)
	}
}

func TestOptOutsInvertTheirCounterpart(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want func(*api.ApiSessionStartRequest) (string, *bool)
		val  bool
	}{
		{"--headed sends headless=false", []string{"--headed"},
			func(b *api.ApiSessionStartRequest) (string, *bool) { return "Headless", b.Headless }, false},
		{"--headed=false sends headless=true", []string{"--headed=false"},
			func(b *api.ApiSessionStartRequest) (string, *bool) { return "Headless", b.Headless }, true},
		{"--no-solve-captchas sends solve_captchas=false", []string{"--no-solve-captchas"},
			func(b *api.ApiSessionStartRequest) (string, *bool) { return "SolveCaptchas", b.SolveCaptchas }, false},
		{"--no-file-storage sends use_file_storage=false", []string{"--no-file-storage"},
			func(b *api.ApiSessionStartRequest) (string, *bool) { return "UseFileStorage", b.UseFileStorage }, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body, err := runOptOut(t, tc.args...)
			if err != nil {
				t.Fatalf("applySessionStartOptOuts() error = %v", err)
			}
			field, got := tc.want(body)
			if got == nil {
				t.Fatalf("%s is nil, want %v", field, tc.val)
			}
			if *got != tc.val {
				t.Errorf("%s = %v, want %v", field, *got, tc.val)
			}
		})
	}
}

// TestConflictingPairIsRejected keeps the two spellings from silently
// disagreeing - without this, precedence would decide and the loser would be
// invisible.
func TestConflictingPairIsRejected(t *testing.T) {
	for _, args := range [][]string{
		{"--headed", "--headless"},
		{"--headed", "--headless=false"},
		{"--no-solve-captchas", "--solve-captchas"},
		{"--no-file-storage", "--use-file-storage"},
	} {
		_, err := runOptOut(t, args...)
		if err == nil {
			t.Errorf("%v: expected an error for a conflicting pair, got nil", args)
		}
	}
}

// TestOriginalFlagsStillWorkAlone confirms the originals were not deprecated
// away: pinning a value explicitly stays valid, which is what a caller does
// when they cannot rely on the server default.
func TestOriginalFlagsStillWorkAlone(t *testing.T) {
	cmd := newOptOutCmd()
	for _, name := range []string{"headless", "solve-captchas", "use-file-storage"} {
		f := cmd.Flags().Lookup(name)
		if f == nil {
			t.Errorf("--%s should still be registered", name)
			continue
		}
		if f.Deprecated != "" {
			t.Errorf("--%s should not be deprecated: pinning a value explicitly is valid", name)
		}
		if f.Hidden {
			t.Errorf("--%s should stay visible in help", name)
		}
	}
}
