package cmd

import (
	"fmt"
	"os"
	"path/filepath"
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

	// Imports come from the sources rather than the artifacts: a function that
	// failed to bundle above still needs its dependencies present, or every
	// later diagnostic is a spurious unresolved-import.
	imports, err := sourceImports(cfg)
	if err != nil {
		return err
	}
	venv := cfg.StatePath("venv")
	sync, err := pyenv.Sync(cmd.Context(), tc, pyenv.SyncRequest{
		VenvDir: venv, Health: health, Imports: imports,
	})
	if err != nil {
		reportChecked(results, failed)
		return err
	}
	reportEnvironment(sync)

	buildDir := cfg.StatePath("build", envName())
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		return err
	}

	// The sources are checked as well as the artifacts, and not only for
	// tidiness: a module under _shared/ that no function imports appears in no
	// artifact, so checking artifacts alone would never look at it. Source
	// diagnostics also land on the real file directly, with no map in between.
	srcRes, err := pyenv.TypeCheck(cmd.Context(), tc, cfg.Root, venv, []string{cfg.Project.FunctionsDir})
	if err != nil {
		return err
	}
	if broken := srcRes.Misconfigured(health); len(broken) > 0 {
		return fmt.Errorf("the environment in %s cannot resolve %s, which the runtime reports as installed — "+
			"delete it and re-run to rebuild", venv, strings.Join(broken, ", "))
	}
	sourceProblems := map[string][]string{}
	for _, d := range srcRes.Diagnostics {
		owner := functionOwning(d.Path, selected, cfg.Project.FunctionsDir)
		sourceProblems[owner] = append(sourceProblems[owner],
			fmt.Sprintf("%s:%d: %s [%s]", d.Path, d.Line, d.Message, d.Rule))
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

		// run()'s parameters are known now, so a cron_variables typo is caught
		// here rather than at 09:00 on a Sunday when the schedule fires.
		params := make([]project.Param, 0, len(verdict.Variables))
		for _, v := range verdict.Variables {
			params = append(params, project.Param{Name: v.Name, HasDefault: v.Default != nil})
		}
		results[i].Problems = append(results[i].Problems,
			cfg.Functions[results[i].Name].ScheduleProblems(results[i].Name, params)...)

		rel, err := filepath.Rel(cfg.Root, artifactPath)
		if err != nil {
			rel = artifactPath
		}
		tyRes, err := pyenv.TypeCheck(cmd.Context(), tc, cfg.Root, venv, []string{rel})
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
		results[i].Problems = append(results[i].Problems, sourceProblems[results[i].Name]...)
		if len(results[i].Problems) > 0 {
			failed++
		}
	}

	// Diagnostics in shared code belong to no single function. Reporting them
	// under whichever function happened to import it would be arbitrary, and
	// dropping them would hide the case this whole pass exists for.
	if shared := sourceProblems[""]; len(shared) > 0 {
		failed++
		results = append(results, checked{Name: "(shared)", Problems: shared})
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

// functionOwning maps a source path to the function whose directory contains
// it, or "" for shared code that belongs to none.
func functionOwning(path string, functions []project.Function, functionsDir string) string {
	rel := strings.TrimPrefix(filepath.ToSlash(path), functionsDir+"/")
	for _, f := range functions {
		if f.Dir && strings.HasPrefix(rel, f.Name+"/") {
			return f.Name
		}
		if !f.Dir && rel == f.Entrypoint {
			return f.Name
		}
	}
	return ""
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
