package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/api"
)

// Several session-start options default to true server-side, which left their
// flags shaped as opt-ins for something already on. Passing --solve-captchas
// changed nothing, and turning it off required the double negative
// `--solve-captchas=false`. Each now has a positive way to opt out.
//
// Naming follows one rule: use the term the ecosystem already has, otherwise
// prefix with --no-. The prefix follows the convention used by git, docker,
// npm and curl.
//
// --no- rather than --disable- specifically because Chromium's own flags are
// --disable-* (--disable-gpu, --disable-extensions) and this command forwards
// them verbatim through --chrome-args. Keeping the prefixes distinct means
// `--no-file-storage --chrome-args="--disable-gpu"` reads unambiguously as one
// Notte flag and one browser flag.
var (
	sessionsStartNoSolveCaptchas bool
	sessionsStartNoFileStorage   bool
)

// sessionStartOptOut pairs a new negative flag with the generated flag it
// supersedes. The original stays registered and undeprecated: pinning a value
// explicitly is legitimate defensive scripting, since a caller should not
// have to trust that the server default stays true. Only passing both at once
// is an error.
type sessionStartOptOut struct {
	negative string
	original string
	set      *bool
	apply    func(body *api.ApiSessionStartRequest, enabled bool)
}

func sessionStartOptOuts() []sessionStartOptOut {
	return []sessionStartOptOut{
		{
			negative: "no-solve-captchas",
			original: "solve-captchas",
			set:      &sessionsStartNoSolveCaptchas,
			apply:    func(b *api.ApiSessionStartRequest, enabled bool) { b.SolveCaptchas = &enabled },
		},
		{
			negative: "no-file-storage",
			original: "use-file-storage",
			set:      &sessionsStartNoFileStorage,
			apply:    func(b *api.ApiSessionStartRequest, enabled bool) { b.UseFileStorage = &enabled },
		},
	}
}

// registerSessionStartOptOutFlags adds the negative flags alongside the
// generated ones.
func registerSessionStartOptOutFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&sessionsStartNoSolveCaptchas, "no-solve-captchas", false,
		"Do not attempt to solve captchas automatically")
	// No backticks in usage strings: cobra's UnquoteUsage treats the first
	// backquoted span as the flag's value placeholder, so this rendered as
	// "--no-file-storage notte page download" in --help.
	cmd.Flags().BoolVar(&sessionsStartNoFileStorage, "no-file-storage", false,
		"Do not attach FileStorage. Disables 'notte page download' and 'notte files --from session'")
}

// validateSessionStartOptOuts rejects a pair whose two spellings were both
// supplied. It is deliberately separate from applying the values: `sessions
// start` stops and clears any current session before it builds the request, so
// validation that runs at build time would take the old session away and then
// refuse to start a new one. This runs from PreRunE, ahead of any side effect.
func validateSessionStartOptOuts(cmd *cobra.Command) error {
	for _, o := range sessionStartOptOuts() {
		if cmd.Flags().Changed(o.negative) && cmd.Flags().Changed(o.original) {
			return fmt.Errorf(
				"--%s and --%s set the same option; pass only one", o.negative, o.original)
		}
	}
	return nil
}

// applySessionStartOptOuts folds the negative flags into an already-built
// request body. Each one inverts its counterpart.
// Revalidates so the invariant holds even if a caller reaches this without
// going through PreRunE.
func applySessionStartOptOuts(cmd *cobra.Command, body *api.ApiSessionStartRequest) error {
	if err := validateSessionStartOptOuts(cmd); err != nil {
		return err
	}

	for _, o := range sessionStartOptOuts() {
		if cmd.Flags().Changed(o.negative) {
			o.apply(body, !*o.set)
		}
	}
	return nil
}

// validateSessionStartProxyFlags reports more than one proxy kind being
// selected. Same reasoning as above: this used to run after the current session
// had already been stopped.
func validateSessionStartProxyFlags(cmd *cobra.Command) error {
	var set []string
	for _, name := range []string{"proxy", "proxy-country", "proxy-external-server", "proxy-tailnet-client-id"} {
		if cmd.Flags().Changed(name) {
			set = append(set, "--"+name)
		}
	}
	if len(set) > 1 {
		return fmt.Errorf("proxy flags are mutually exclusive, got: %s", strings.Join(set, ", "))
	}
	return nil
}

// validateSessionStartFlags runs every flag-consistency check that must happen
// before `sessions start` touches anything.
func validateSessionStartFlags(cmd *cobra.Command, _ []string) error {
	if err := validateSessionStartOptOuts(cmd); err != nil {
		return err
	}
	return validateSessionStartProxyFlags(cmd)
}
