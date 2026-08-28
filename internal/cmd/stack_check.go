package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/bundle"
	"github.com/nottelabs/notte-cli/internal/project"
	"github.com/nottelabs/notte-cli/internal/pyenv"
)

var stackCheckCmd = &cobra.Command{
	Use:   "check [target]",
	Short: "Build and validate every function, writing nothing remote",
	Long: `Bundle each function, then check the result against the runtime's own
rules: its import allowlist, the script contract, and a type check.

Writes nothing to the API, so it is safe as a CI gate. Target may be a name, a
glob, a path, or "all".`,
	Args: cobra.MaximumNArgs(1),
	RunE: runStackCheck,
}

func init() { stackCmd.AddCommand(stackCheckCmd) }

// checked is one function's outcome.
type checked struct {
	Name           string   `json:"name"`
	Entrypoint     string   `json:"entrypoint"`
	Sources        []string `json:"sources"`
	SourceSHA256   string   `json:"source_sha256"`
	ArtifactSHA256 string   `json:"artifact_sha256"`
	Problems       []string `json:"problems,omitempty"`
}

func runStackCheck(cmd *cobra.Command, args []string) error {
	cfg, err := loadStack()
	if err != nil {
		return err
	}
	target := ""
	if len(args) == 1 {
		target = args[0]
	}

	functions, err := project.Discover(cfg)
	if err != nil {
		return err
	}
	selected, err := project.Select(functions, target)
	if err != nil {
		return err
	}
	if len(selected) == 0 {
		return PrintResult("no functions found", map[string]any{"functions": []any{}})
	}

	// Bundling comes first because it needs nothing external. A syntax or
	// layout problem is reported without ever touching the network.
	fsys := os.DirFS(cfg.FunctionsPath())
	results := make([]checked, 0, len(selected))
	artifacts := map[string]*bundle.Result{}
	imports := map[string]bool{}
	failed := 0

	for _, fn := range selected {
		res, err := bundle.Bundle(fsys, fn.Entrypoint, bundle.Options{
			Header: bundleHeader(fn.Name),
		})
		if err != nil {
			results = append(results, checked{Name: fn.Name, Entrypoint: fn.Entrypoint, Problems: []string{err.Error()}})
			failed++
			continue
		}
		artifacts[fn.Name] = res
		for _, module := range bundle.ExternalImports(res.Code) {
			imports[module] = true
		}
		results = append(results, checked{
			Name: fn.Name, Entrypoint: fn.Entrypoint, Sources: res.Sources,
			SourceSHA256: res.SourceSHA256, ArtifactSHA256: res.ArtifactSHA256,
		})
	}

	// Everything below needs the runtime's description of itself, and there is
	// no local copy of it to fall back on — that is the point.
	health, tc, err := stackRuntime(cmd)
	if err != nil {
		reportChecked(results, failed)
		return err
	}

	venv := cfg.StatePath("venv")
	sync, err := pyenv.Sync(cmd.Context(), tc, pyenv.SyncRequest{
		VenvDir: venv, Health: health, Imports: sortedKeys(imports),
	})
	if err != nil {
		reportChecked(results, failed)
		return err
	}
	reportEnvironment(sync)

	if err := pyenv.WriteTyConfig(cfg.Root, venv); err != nil {
		return err
	}

	buildDir := cfg.StatePath("build", envName())
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		return err
	}

	for i := range results {
		res, ok := artifacts[results[i].Name]
		if !ok {
			continue
		}
		artifactPath := filepath.Join(buildDir, results[i].Name+".py")
		if err := os.WriteFile(artifactPath, []byte(res.Code), 0o644); err != nil {
			return err
		}

		verdict, err := pyenv.Validate(cmd.Context(), venv, health, res.Code)
		if err != nil {
			return err
		}
		results[i].Problems = append(results[i].Problems, verdict.Errors...)

		rel, err := filepath.Rel(cfg.Root, artifactPath)
		if err != nil {
			rel = artifactPath
		}
		tyRes, err := pyenv.TypeCheck(cmd.Context(), tc, cfg.Root, []string{rel})
		if err != nil {
			return err
		}
		// An unresolved import of something the runtime ships means the venv
		// is wrong, not the code. Blaming the file would send someone to fix
		// something that is fine.
		if broken := tyRes.Misconfigured(health); len(broken) > 0 {
			return fmt.Errorf("the environment in %s cannot resolve %s, which the runtime reports as installed — "+
				"delete it and re-run to rebuild", venv, strings.Join(broken, ", "))
		}
		for _, d := range tyRes.Diagnostics {
			results[i].Problems = append(results[i].Problems, mapDiagnostic(res, d))
		}
		if len(results[i].Problems) > 0 {
			failed++
		}
	}

	reportChecked(results, failed)
	if failed > 0 {
		return fmt.Errorf("%d of %d function(s) failed", failed, len(results))
	}
	return nil
}

// mapDiagnostic rewrites an artifact location back to the source it came from.
//
// Without this a report points into a concatenated file, and the reader has to
// work out which module line 612 belonged to.
func mapDiagnostic(res *bundle.Result, d pyenv.Diagnostic) string {
	if path, line, ok := res.Map.Lookup(d.Line); ok {
		return fmt.Sprintf("%s:%d: %s [%s]", path, line, d.Message, d.Rule)
	}
	return fmt.Sprintf("(generated):%d: %s [%s]", d.Line, d.Message, d.Rule)
}

func bundleHeader(name string) string {
	return fmt.Sprintf("# generated by notte from the %q stack function — do not edit", name)
}

func reportEnvironment(sync *pyenv.SyncResult) {
	verb := "built"
	if sync.Reused {
		verb = "reused"
	}
	PrintInfo(fmt.Sprintf("environment %s (Python %s, %d package(s))", verb, sync.Python, len(sync.Installed)))

	// Allowed but absent from the image: passes upload validation, then dies
	// on ModuleNotFoundError mid-run. Nothing else makes this visible.
	if len(sync.AllowedButMissing) > 0 {
		PrintInfo("  warning: the runtime allows but does not ship: " +
			strings.Join(sync.AllowedButMissing, ", "))
	}
	if len(sync.NotAllowed) > 0 {
		PrintInfo("  warning: not available at run time: " + strings.Join(sync.NotAllowed, ", "))
	}
}

func reportChecked(results []checked, failed int) {
	if IsJSONOutput() {
		_ = GetFormatter().Print(map[string]any{"functions": results, "failed": failed})
		return
	}
	for _, r := range results {
		if len(r.Problems) == 0 {
			PrintInfo(fmt.Sprintf("  ok   %-24s %d source(s)  %s", r.Name, len(r.Sources), short(r.ArtifactSHA256)))
			continue
		}
		PrintInfo(fmt.Sprintf("  FAIL %s", r.Name))
		for _, p := range r.Problems {
			PrintInfo("         " + p)
		}
	}
}

func short(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func envName() string {
	if stackEnv == "" {
		return project.DefaultEnv
	}
	return stackEnv
}

// stackRuntime resolves credentials and fetches the runtime's report.
func stackRuntime(cmd *cobra.Command) (*pyenv.Health, *pyenv.Toolchain, error) {
	tc, err := pyenv.FindToolchain()
	if err != nil {
		return nil, nil, err
	}
	client, err := GetClient()
	if err != nil {
		return nil, nil, err
	}
	ctx, cancel := GetContextWithTimeout(cmd.Context())
	defer cancel()

	health, err := pyenv.FetchHealth(ctx, client.HTTPClient(), client.BaseURL(), client.APIKey())
	if err != nil {
		return nil, nil, err
	}
	if !health.Complete() {
		return nil, nil, fmt.Errorf(
			"the function runtime reported %q, so its package list is unavailable: %s\n"+
				"  this is normal between an API deploy and a runner rebuild; try again shortly",
			health.Status, health.Error)
	}
	return health, tc, nil
}
