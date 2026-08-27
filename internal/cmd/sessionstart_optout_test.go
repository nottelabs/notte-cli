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
	cmd.Flags().BoolVar(&SessionStartSolveCaptchas, "solve-captchas", false, "solve captchas")
	cmd.Flags().BoolVar(&SessionStartUseFileStorage, "use-file-storage", false, "file storage")
	registerSessionStartOptOutFlags(cmd)
	return cmd
}

func runOptOut(t *testing.T, args ...string) (*api.ApiSessionStartRequest, error) {
	t.Helper()
	// Package-level flag vars are shared; reset before each case.
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
	if body.SolveCaptchas != nil {
		t.Errorf("SolveCaptchas = %v, want nil when --no-solve-captchas is omitted", *body.SolveCaptchas)
	}
	if body.UseFileStorage != nil {
		t.Errorf("UseFileStorage = %v, want nil when --no-file-storage is omitted", *body.UseFileStorage)
	}
}

func TestOptOutsInvertTheirCounterpart(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		field string
		get   func(*api.ApiSessionStartRequest) *bool
		want  bool
	}{
		{
			name:  "--no-solve-captchas sends solve_captchas=false",
			args:  []string{"--no-solve-captchas"},
			field: "SolveCaptchas",
			get:   func(b *api.ApiSessionStartRequest) *bool { return b.SolveCaptchas },
			want:  false,
		},
		{
			name:  "--no-file-storage sends use_file_storage=false",
			args:  []string{"--no-file-storage"},
			field: "UseFileStorage",
			get:   func(b *api.ApiSessionStartRequest) *bool { return b.UseFileStorage },
			want:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body, err := runOptOut(t, tc.args...)
			if err != nil {
				t.Fatalf("applySessionStartOptOuts() error = %v", err)
			}
			got := tc.get(body)
			if got == nil {
				t.Fatalf("%s is nil, want %v", tc.field, tc.want)
			}
			if *got != tc.want {
				t.Errorf("%s = %v, want %v", tc.field, *got, tc.want)
			}
		})
	}
}

// TestConflictingPairIsRejected keeps the two spellings from silently
// disagreeing - without this, precedence would decide and the loser would be
// invisible.
func TestConflictingPairIsRejected(t *testing.T) {
	for _, args := range [][]string{
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
	for _, name := range []string{"solve-captchas", "use-file-storage"} {
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

// TestConflictsRejectedBeforeSideEffects pins the ordering, not just the error.
// runSessionsStart stops and clears the current session before it builds the
// request, so a conflict detected at build time would cost the user their
// existing session and give them nothing back. Validation therefore has to hang
// off PreRunE, which cobra runs before RunE.
func TestConflictsRejectedBeforeSideEffects(t *testing.T) {
	if sessionsStartCmd.PreRunE == nil {
		t.Fatal("sessions start must validate flags in PreRunE; validating inside RunE " +
			"runs after the current session has already been stopped")
	}

	ranRunE := false
	cmd := &cobra.Command{
		Use:     "start",
		PreRunE: validateSessionStartFlags,
		RunE: func(*cobra.Command, []string) error {
			ranRunE = true
			return nil
		},
	}
	// --solve-captchas has to be registered here too: without it the pair below
	// fails on cobra's "unknown flag" before PreRunE ever runs, so the case would
	// pass without exercising validateSessionStartOptOuts at all.
	cmd.Flags().BoolVar(&SessionStartSolveCaptchas, "solve-captchas", false, "solve captchas")
	cmd.Flags().Bool("proxy", false, "proxy")
	cmd.Flags().String("proxy-country", "", "proxy country")
	registerSessionStartOptOutFlags(cmd)

	for _, args := range [][]string{
		{"--no-solve-captchas", "--solve-captchas"},
		{"--proxy", "--proxy-country=us"},
	} {
		ranRunE = false
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil {
			t.Errorf("%v: expected PreRunE to reject the combination", args)
		}
		if ranRunE {
			t.Errorf("%v: RunE ran despite invalid flags - the session would already be gone", args)
		}
	}
}
