package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/bundle"
	"github.com/nottelabs/notte-cli/internal/project"
	"github.com/nottelabs/notte-cli/internal/pyenv"
)

var stackSyncCmd = &cobra.Command{
	Use:     "sync",
	Aliases: []string{"install"},
	Short:   "Build the local Python environment that mirrors the runtime",
	Long: `Create .notte/venv with the Python version the runtime runs and the
packages it ships, so an editor resolves imports the way the runtime will.

Only the allowlisted packages your functions actually import are installed:
the runtime's list is closed, so this is an intersection rather than
dependency resolution. Deploy and check do this automatically; sync is the
explicit form, and the one to run after cloning a stack.`,
	Args: cobra.NoArgs,
	RunE: runStackSync,
}

var stackSyncForce bool

func init() {
	stackCmd.AddCommand(stackSyncCmd)
	stackSyncCmd.Flags().BoolVar(&stackSyncForce, "force", false,
		"Rebuild even if the environment already matches the runtime")
}

func runStackSync(cmd *cobra.Command, args []string) error {
	cfg, err := loadStack()
	if err != nil {
		return err
	}
	health, tc, err := stackRuntime(cmd)
	if err != nil {
		return err
	}

	imports, err := sourceImports(cfg)
	if err != nil {
		return err
	}

	sync, err := pyenv.Sync(cmd.Context(), tc, pyenv.SyncRequest{
		VenvDir: cfg.StatePath("venv"), Health: health, Imports: imports, Force: stackSyncForce,
	})
	if err != nil {
		return err
	}
	reportEnvironment(sync)

	installed := make([]string, 0, len(sync.Installed))
	for _, p := range sync.Installed {
		installed = append(installed, p.ImportName+" "+p.Version)
	}
	if !IsJSONOutput() {
		for _, line := range installed {
			PrintInfo("    " + line)
		}
	}

	return PrintResult(
		fmt.Sprintf("\n%s is ready.", cfg.StatePath("venv")),
		map[string]any{
			"venv":                cfg.StatePath("venv"),
			"python":              sync.Python,
			"installed":           installed,
			"allowed_but_missing": sync.AllowedButMissing,
			"not_allowed":         sync.NotAllowed,
			"reused":              sync.Reused,
		},
	)
}

// sourceImports is every non-relative module the stack's Python files import.
//
// Read from the sources rather than from bundled artifacts on purpose: a
// function that fails to bundle still has imports, and its author still needs
// an environment in which to fix it. Scanning sources also covers shared
// modules that no function imports yet, which an artifact never mentions.
func sourceImports(cfg *project.Config) ([]string, error) {
	root := cfg.FunctionsPath()
	seen := map[string]bool{}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".py") {
			return nil
		}
		// Tests are included deliberately. They are never bundled, so their
		// imports are not the runtime's concern — but they are the editor's,
		// and an environment that cannot resolve a test file is half useful.
		src, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for _, module := range bundle.ExternalImports(string(src)) {
			seen[module] = true
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	out := make([]string, 0, len(seen))
	for module := range seen {
		out = append(out, module)
	}
	sort.Strings(out)
	return out, nil
}
