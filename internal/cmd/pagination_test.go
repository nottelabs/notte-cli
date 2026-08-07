package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

// newListCmd builds a bare list command carrying one renamed filter flag plus
// the deprecated --only-active alias, the way registerFilterFlag does.
func newListCmd(name, shorthand string) *cobra.Command {
	cmd := &cobra.Command{Use: "list", RunE: func(*cobra.Command, []string) error { return nil }}
	registerFilterFlag(cmd, name, shorthand, "usage")
	return cmd
}

func execute(t *testing.T, cmd *cobra.Command, args ...string) {
	t.Helper()
	cmd.SetArgs(args)
	// Deprecated flags print a notice to stderr; keep test output quiet.
	cmd.SetOut(nil)
	cmd.SetErr(nil)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute(%v) error = %v", args, err)
	}
}

// TestResolveOnlyActiveArtifacts covers functions/vaults/personas, where the
// API's only_active means "not deleted". The default must keep deleted records
// hidden - surfacing tombstones in a plain `list` would be a regression.
func TestResolveOnlyActiveArtifacts(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{"default hides deleted", []string{}, true},
		{"--include-deleted shows them", []string{"--include-deleted"}, false},
		{"--include-deleted=false", []string{"--include-deleted=false"}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newListCmd(flagIncludeDeleted, "")
			execute(t, cmd, tc.args...)

			got := resolveOnlyActive(cmd, flagIncludeDeleted, true)
			if got == nil {
				t.Fatal("resolveOnlyActive returned nil; the value must always be sent so the " +
					"API cannot substitute its own default")
			}
			if *got != tc.want {
				t.Errorf("only_active = %v, want %v", *got, tc.want)
			}
		})
	}
}

// TestResolveOnlyActiveInstances covers sessions, where only_active
// means "still running". The default stays running-only, matching `docker ps`.
func TestResolveOnlyActiveInstances(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{"default shows running only", []string{}, true},
		{"--all includes finished", []string{"--all"}, false},
		{"-a includes finished", []string{"-a"}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newListCmd(flagAll, "a")
			execute(t, cmd, tc.args...)

			got := resolveOnlyActive(cmd, flagAll, true)
			if got == nil {
				t.Fatal("resolveOnlyActive returned nil")
			}
			if *got != tc.want {
				t.Errorf("only_active = %v, want %v", *got, tc.want)
			}
		})
	}
}

// TestResolveOnlyActiveRuns is the regression test for the bug that started
// this: for function runs, only_active means "still executing", so defaulting
// to it made run history permanently empty. `notte functions runs` returned []
// for a function whose runs had all completed, which reads as "this function
// never ran" and misleads anything diagnosing a failure.
func TestResolveOnlyActiveRuns(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{"default returns full history", []string{}, false},
		{"--running narrows to in-flight", []string{"--running"}, true},
		{"--running=false", []string{"--running=false"}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newListCmd(flagRunning, "")
			execute(t, cmd, tc.args...)

			got := resolveOnlyActive(cmd, flagRunning, false)
			if got == nil {
				t.Fatal("resolveOnlyActive returned nil")
			}
			if *got != tc.want {
				t.Errorf("only_active = %v, want %v", *got, tc.want)
			}
		})
	}
}

// TestDeprecatedOnlyActiveStillWins keeps the escape hatch working: --only-active
// maps straight onto the API parameter, so anyone already passing it - including
// scripts written against the old behaviour - is unaffected by the rename.
func TestDeprecatedOnlyActiveStillWins(t *testing.T) {
	for _, tc := range []struct {
		flagName string
		negates  bool
		args     []string
		want     bool
	}{
		{flagIncludeDeleted, true, []string{"--only-active=false"}, false},
		{flagIncludeDeleted, true, []string{"--only-active"}, true},
		{flagAll, true, []string{"--only-active=false"}, false},
		{flagRunning, false, []string{"--only-active"}, true},
	} {
		cmd := newListCmd(tc.flagName, "")
		execute(t, cmd, tc.args...)

		got := resolveOnlyActive(cmd, tc.flagName, tc.negates)
		if got == nil || *got != tc.want {
			t.Errorf("--%s with %v: only_active = %v, want %v", tc.flagName, tc.args, got, tc.want)
		}
	}
}

// TestOnlyActiveIsHiddenButUsable checks the deprecation is a soft one: the flag
// no longer clutters --help, but still parses.
func TestOnlyActiveIsHiddenButUsable(t *testing.T) {
	cmd := newListCmd(flagAll, "a")
	f := cmd.Flags().Lookup(flagOnlyActive)
	if f == nil {
		t.Fatal("--only-active should still exist as a deprecated alias")
	}
	if f.Deprecated == "" {
		t.Error("--only-active should be marked deprecated")
	}
	if !f.Hidden {
		t.Error("a deprecated --only-active should be hidden from help output")
	}
}

// TestAlwaysSendReturnsValue covers a boolean list filter that must be
// sent explicitly even when false.
func TestAlwaysSendReturnsValue(t *testing.T) {
	cmd := &cobra.Command{Use: "list", RunE: func(*cobra.Command, args []string) error { return nil }}
	cmd.Flags().Bool("include-deleted", false, "Include deleted records")
	execute(t, cmd)

	got := alwaysSend(cmd, "include-deleted")
	if got == nil || *got != false {
		t.Errorf("alwaysSend(include-deleted) = %v, want non-nil false", got)
	}
}

// TestPageFlagsStayUnsetWhenOmitted guards the opposite convention: pagination
// parameters are genuinely optional and must stay absent when not supplied, so
// the API applies its own page defaults. Don't "unify" these with the filters.
func TestPageFlagsStayUnsetWhenOmitted(t *testing.T) {
	cmd := &cobra.Command{Use: "list", RunE: func(*cobra.Command, []string) error { return nil }}
	registerPaginationFlags(cmd)
	execute(t, cmd)

	page, err := getPageFlag(cmd)
	if err != nil {
		t.Fatalf("getPageFlag() error = %v", err)
	}
	if page != nil {
		t.Errorf("getPageFlag() = %v, want nil when --page is omitted", *page)
	}

	pageSize, err := getPageSizeFlag(cmd)
	if err != nil {
		t.Fatalf("getPageSizeFlag() error = %v", err)
	}
	if pageSize != nil {
		t.Errorf("getPageSizeFlag() = %v, want nil when --page-size is omitted", *pageSize)
	}
}

// TestPageFlagsRejectOutOfRange keeps the existing 1-indexed validation covered.
func TestPageFlagsRejectOutOfRange(t *testing.T) {
	for _, flag := range []string{"--page=0", "--page-size=0"} {
		cmd := &cobra.Command{Use: "list", RunE: func(*cobra.Command, []string) error { return nil }}
		registerPaginationFlags(cmd)
		execute(t, cmd, flag)

		var err error
		if flag == "--page=0" {
			_, err = getPageFlag(cmd)
		} else {
			_, err = getPageSizeFlag(cmd)
		}
		if err == nil {
			t.Errorf("%s: expected an error for a value < 1", flag)
		}
	}
}
