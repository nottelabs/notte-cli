package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/api"
	"github.com/nottelabs/notte-cli/internal/auth"
	"github.com/nottelabs/notte-cli/internal/project"
	"github.com/nottelabs/notte-cli/internal/pyenv"
)

var stackDoctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Report the toolchain, the environment and the runtime",
	Long: `Show what is installed, what the runtime reports, and whether the local
environment matches it.

Answers the questions that are otherwise support tickets: which Python will my
code run under, what may I import, and why was my function not type checked.`,
	Args: cobra.NoArgs,
	RunE: runStackDoctor,
}

func init() { stackCmd.AddCommand(stackDoctorCmd) }

func runStackDoctor(cmd *cobra.Command, args []string) error {
	report := map[string]any{}
	var lines []string
	ok := func(format string, a ...any) { lines = append(lines, "  ✓ "+fmt.Sprintf(format, a...)) }
	bad := func(format string, a ...any) { lines = append(lines, "  ✗ "+fmt.Sprintf(format, a...)) }
	warn := func(format string, a ...any) { lines = append(lines, "  ! "+fmt.Sprintf(format, a...)) }

	// --- toolchain -------------------------------------------------------
	tc, tcErr := pyenv.FindToolchain()
	if tcErr != nil {
		bad("uv not found — `notte stack` needs it")
		report["uv"] = nil
	} else {
		ok("uv %s", tc.UV)
		report["uv"] = tc.UV
	}
	ok("ty pinned at %s", pyenv.TyVersion)
	report["ty_version"] = pyenv.TyVersion

	// --- project ---------------------------------------------------------
	cfg, cfgErr := loadStack()
	if cfgErr != nil {
		warn("no stack here (%v)", cfgErr)
	} else {
		ok("stack %s at %s", cfg.Project.Name, cfg.Root)
		report["root"] = cfg.Root
		if functions, err := project.Discover(cfg); err == nil {
			ok("%d function(s) discovered", len(functions))
			report["functions"] = len(functions)
		} else {
			bad("discovery failed: %v", err)
		}
	}

	// --- runtime ---------------------------------------------------------
	//
	// Resolved through the same coupling every other stack command uses. An
	// ambient client labelled with --env would report another environment's
	// Python version and package list as if they were this one's, which is a
	// diagnostic command telling a confident lie.
	client, envLabel, clientErr := doctorClient(cfg, cfgErr)
	if clientErr != nil {
		bad("no credentials: %v", clientErr)
	} else {
		ctx, cancel := GetContextWithTimeout(cmd.Context())
		defer cancel()
		health, err := pyenv.FetchHealth(ctx, client.HTTPClient(), client.BaseURL(), client.APIKey())
		switch {
		case err != nil:
			bad("runtime: %v", err)
		case !health.Complete():
			// Normal between an API deploy and a runner rebuild, so it is
			// reported as a state rather than as a fault.
			warn("runtime %s — no package list yet; this clears on the next runner deploy", health.Status)
			report["runtime_status"] = health.Status
		default:
			ok("runtime ok — Python %s, %d package(s), digest %s",
				health.PythonVersion, len(health.Packages), short(strings.TrimPrefix(health.RuntimeDigest, "sha256:")))
			report["runtime_status"] = health.Status
			report["python_version"] = health.PythonVersion
			report["runtime_digest"] = health.RuntimeDigest

			// Allowed but absent from the image passes upload validation and
			// then dies mid-run, so it is worth naming even when nothing here
			// imports it.
			var absent []string
			for _, p := range health.Packages {
				if !p.Installed {
					absent = append(absent, p.ImportName)
				}
			}
			sort.Strings(absent)
			if len(absent) > 0 {
				warn("allowed but not shipped by the image: %s", strings.Join(absent, ", "))
				report["allowed_but_missing"] = absent
			}

			if cfgErr == nil {
				doctorEnvironment(cfg, health, ok, bad, warn, report)
			}
		}
		report["api_url"] = client.BaseURL()
		report["env"] = envLabel
		ok("api %s (env %s)", client.BaseURL(), envLabel)
	}

	if IsJSONOutput() {
		return GetFormatter().Print(report)
	}
	for _, line := range lines {
		PrintInfo(line)
	}
	return nil
}

// doctorClient resolves the endpoint to report on.
//
// Inside a stack it is the environment-coupled client, so --env means the same
// thing here as it does for deploy. Outside one there is no notte.toml to
// resolve against, and doctor still has to work — it is the command people run
// when nothing else does.
//
// The fallback keys on whether --env was promised, not on why the config is
// unavailable. An earlier version branched on cfgErr alone, which made a
// notte.toml that merely fails to parse indistinguishable from no project at
// all: `doctor --env staging` next to a malformed config silently reported on
// the ambient endpoint instead. A flag that cannot be honoured is refused.
func doctorClient(cfg *project.Config, cfgErr error) (*api.NotteClient, string, error) {
	if cfgErr == nil {
		dest, err := resolveStackTarget(cfg)
		if err != nil {
			return nil, "", err
		}
		return dest.client, dest.Env, nil
	}

	client, err := GetClient()
	if err != nil {
		return nil, "", err
	}
	label := auth.ResolveEnvLabel(client.BaseURL())

	if stackEnv != "" && stackEnv != label {
		return nil, "", fmt.Errorf(
			"--env %s cannot be resolved: %v\n"+
				"  the configured endpoint is %s (%s); fix %s, or drop --env to report on %s",
			stackEnv, cfgErr, client.BaseURL(), label, project.ConfigName, label)
	}
	return client, label, nil
}

// doctorEnvironment compares the local venv against the runtime it should
// mirror, which is the check that explains a silently useless type check.
func doctorEnvironment(cfg *project.Config, health *pyenv.Health,
	ok, bad, warn func(string, ...any), report map[string]any,
) {
	venv := cfg.StatePath("venv")
	if _, err := os.Stat(pyenv.PythonPath(venv)); err != nil {
		warn("no environment yet — run `notte stack sync`")
		report["venv"] = nil
		return
	}
	report["venv"] = venv

	stamp, err := pyenv.ReadStamp(venv)
	if err != nil {
		warn("environment has no stamp; run `notte stack sync --force`")
		return
	}
	if stamp.RuntimeDigest != health.RuntimeDigest {
		bad("environment was built for a different runtime — run `notte stack sync`")
		return
	}
	ok("environment matches the runtime (Python %s)", stamp.PythonVersion)
}
