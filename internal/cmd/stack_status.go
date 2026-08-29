package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/bundle"
	"github.com/nottelabs/notte-cli/internal/project"
)

var stackStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show what has changed since the last deploy",
	Long: `Compare each function against what the lockfile says was last deployed,
and report which functions a shared module edit would affect.

Offline: it bundles locally and compares hashes, and does not call the API.`,
	Args: cobra.NoArgs,
	RunE: runStackStatus,
}

func init() { stackCmd.AddCommand(stackStatusCmd) }

type statusRow struct {
	Name    string   `json:"name"`
	State   string   `json:"state"`
	Sources []string `json:"sources"`
	Version string   `json:"version,omitempty"`
	Problem string   `json:"problem,omitempty"`
}

func runStackStatus(cmd *cobra.Command, args []string) error {
	cfg, err := loadStack()
	if err != nil {
		return err
	}
	env := envName()
	lock, err := project.LoadLock(cfg.Root)
	if err != nil {
		return err
	}
	functions, err := project.Discover(cfg)
	if err != nil {
		return err
	}

	fsys := os.DirFS(cfg.FunctionsPath())
	rows := make([]statusRow, 0, len(functions))
	// usedBy is the inverse of the import graph, which is what makes a shared
	// edit legible: a change to _shared/http.py is invisible in a per-function
	// diff, but it changes the artifact of everything that imports it.
	usedBy := map[string][]string{}

	for _, fn := range functions {
		row := statusRow{Name: fn.Name}
		res, err := bundle.Bundle(fsys, fn.Entrypoint, bundle.Options{Header: bundleHeader(fn.Name)})
		if err != nil {
			row.State, row.Problem = "broken", err.Error()
			rows = append(rows, row)
			continue
		}
		row.Sources = res.Sources
		for _, src := range res.Sources {
			usedBy[src] = append(usedBy[src], fn.Name)
		}

		state, known := lock.State(fn.Entrypoint, env)
		switch {
		case !known:
			row.State = "not deployed"
		case state.SourceSHA256 != res.SourceSHA256:
			row.State, row.Version = "drifted", state.Version
		default:
			row.State, row.Version = "up to date", state.Version
		}
		rows = append(rows, row)
	}

	// Anything in the lock with no file is a rename or a deletion. Reported,
	// never acted on: absence is not an instruction.
	var orphaned []string
	present := map[string]bool{}
	for _, fn := range functions {
		present[fn.Entrypoint] = true
	}
	for _, entry := range lock.Functions {
		if _, deployed := entry.Envs[env]; deployed && !present[entry.Path] {
			orphaned = append(orphaned, entry.Path)
		}
	}
	sort.Strings(orphaned)

	shared := sharedSources(usedBy)

	if IsJSONOutput() {
		return GetFormatter().Print(map[string]any{
			"env": env, "functions": rows, "orphaned": orphaned, "shared": shared,
		})
	}

	PrintInfo(fmt.Sprintf("environment: %s\n", env))
	for _, r := range rows {
		switch r.State {
		case "up to date":
			PrintInfo(fmt.Sprintf("  ✓ %-24s %s", r.Name, r.Version))
		case "drifted":
			PrintInfo(fmt.Sprintf("  ✗ %-24s drifted from %s", r.Name, r.Version))
		case "not deployed":
			PrintInfo(fmt.Sprintf("  + %-24s not deployed to %s", r.Name, env))
		default:
			PrintInfo(fmt.Sprintf("  ! %-24s %s", r.Name, r.Problem))
		}
	}
	for _, path := range orphaned {
		PrintInfo(fmt.Sprintf("  ? %-24s in the lockfile but not on disk (renamed or removed)", path))
	}
	if len(shared) > 0 {
		PrintInfo("\nshared modules:")
		for _, s := range shared {
			PrintInfo(fmt.Sprintf("  %-30s → %d function(s): %s", s.Path, len(s.Functions), strings.Join(s.Functions, ", ")))
		}
	}
	return nil
}

type sharedSource struct {
	Path      string   `json:"path"`
	Functions []string `json:"functions"`
}

// sharedSources are the files more than one function bundles in.
func sharedSources(usedBy map[string][]string) []sharedSource {
	var out []sharedSource
	for path, users := range usedBy {
		if len(users) < 2 {
			continue
		}
		sort.Strings(users)
		out = append(out, sharedSource{Path: path, Functions: users})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}
