package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func registerPaginationFlags(cmd *cobra.Command) {
	cmd.Flags().Int("page", 0, "Page number (1-indexed)")
	cmd.Flags().Int("page-size", 0, "Number of items per page")
}

func getPageFlag(cmd *cobra.Command) (*int, error) {
	if cmd.Flags().Changed("page") {
		v, _ := cmd.Flags().GetInt("page")
		if v < 1 {
			return nil, fmt.Errorf("--page must be >= 1 (got %d)", v)
		}
		return &v, nil
	}
	return nil, nil
}

func getPageSizeFlag(cmd *cobra.Command) (*int, error) {
	if cmd.Flags().Changed("page-size") {
		v, _ := cmd.Flags().GetInt("page-size")
		if v < 1 {
			return nil, fmt.Errorf("--page-size must be >= 1 (got %d)", v)
		}
		return &v, nil
	}
	return nil, nil
}

// The API exposes a single `only_active` parameter, but it means a different
// thing for each resource:
//
//   - stored artifacts (functions, vaults, personas): active == not deleted
//   - live instances (sessions, agents):               active == still running
//   - function runs:                                   active == still executing
//
// One flag named --only-active for all three is unlearnable: whatever a user
// infers from `sessions list` is wrong on `functions list`. So each resource
// gets a flag named after what it actually does, and --only-active is kept as a
// deprecated alias that still maps straight onto the API parameter.
const (
	flagIncludeDeleted = "include-deleted" // artifacts
	flagAll            = "all"             // live instances
	flagRunning        = "running"         // function runs
	flagOnlyActive     = "only-active"     // deprecated alias for all of the above
)

// registerFilterFlag registers a list command's filter flag plus the deprecated
// --only-active it replaces.
func registerFilterFlag(cmd *cobra.Command, name, shorthand, usage string) {
	cmd.Flags().BoolP(name, shorthand, false, usage)
	cmd.Flags().Bool(flagOnlyActive, false, "Deprecated: use --"+name)
	_ = cmd.Flags().MarkDeprecated(flagOnlyActive, "use --"+name)
}

// resolveOnlyActive maps a command's user-facing filter flag onto the API's
// only_active parameter. Set negates when the flag widens the result set
// (--all, --include-deleted) rather than narrowing it (--running).
//
// The returned pointer is always non-nil, so the value travels on every
// request. That matters independently of the defaults: previously the CLI only
// sent the parameter when cobra reported the flag as Changed, which left the
// server's default in charge of the CLI's documented behaviour. Sending it
// always keeps these defaults authoritative and immune to a server-side change.
func resolveOnlyActive(cmd *cobra.Command, name string, negates bool) *bool {
	// The deprecated flag maps directly onto the API parameter, so an explicit
	// --only-active=false keeps working for anyone already relying on it.
	if cmd.Flags().Changed(flagOnlyActive) {
		v, _ := cmd.Flags().GetBool(flagOnlyActive)
		return &v
	}

	v, _ := cmd.Flags().GetBool(name)
	if negates {
		v = !v
	}
	return &v
}

// alwaysSend returns a non-nil pointer to a plain boolean filter flag that
// needs no renaming, for the same reason resolveOnlyActive does.
func alwaysSend(cmd *cobra.Command, name string) *bool {
	v, _ := cmd.Flags().GetBool(name)
	return &v
}
