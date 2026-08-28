package cmd

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/project"
)

//go:embed stacktmpl/*.tmpl
var stackTemplates embed.FS

// stackEnv is the --env flag. It defaults to prod because almost every project
// has exactly one environment; multi-environment support exists for internal
// catalogs and should not be visible to anyone who does not need it.
var stackEnv string

var stackCmd = &cobra.Command{
	Use:   "stack",
	Short: "Manage a git-versioned project of Notte functions",
	Long: `Manage a directory of Notte functions as one project.

A stack is a folder of Python functions, a notte.toml describing them, and a
lockfile mapping each to the function it became in every environment. Shared
code is imported normally and flattened into each artifact at deploy time.

These commands require uv, which supplies the Python the runtime uses:
  curl -LsSf https://astral.sh/uv/install.sh | sh`,
}

func init() {
	rootCmd.AddCommand(stackCmd)
	stackCmd.PersistentFlags().StringVar(&stackEnv, "env", "",
		"Environment to target (default: prod)")
}

// loadStack finds and loads the project containing the working directory.
func loadStack() (*project.Config, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	root, err := project.Find(wd)
	if err != nil {
		return nil, fmt.Errorf("%w\n  run `notte stack init` to create one", err)
	}
	return project.Load(root)
}

// ---------------------------------------------------------------- init

var (
	stackInitName  string
	stackInitForce bool
)

var stackInitCmd = &cobra.Command{
	Use:   "init [dir]",
	Short: "Scaffold a new stack",
	Long: `Create notte.toml, a functions package, and the editor configuration
that makes an editor resolve notte_sdk correctly.

Nothing is written that already exists unless --force is passed.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runStackInit,
}

func init() {
	stackCmd.AddCommand(stackInitCmd)
	stackInitCmd.Flags().StringVar(&stackInitName, "name", "", "Project name (default: directory name)")
	stackInitCmd.Flags().BoolVar(&stackInitForce, "force", false, "Overwrite existing files")
}

func runStackInit(cmd *cobra.Command, args []string) error {
	dir := "."
	if len(args) == 1 {
		dir = args[0]
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return err
	}

	name := stackInitName
	if name == "" {
		name = filepath.Base(abs)
	}
	data := map[string]string{"Name": name, "Example": "hello"}

	// A function is scaffolded alongside the config so that `notte stack check`
	// has something to check immediately, rather than reporting an empty stack.
	files := []struct{ path, tmpl string }{
		{project.ConfigName, "notte.toml.tmpl"},
		{".gitignore", "gitignore.tmpl"},
		{"AGENTS.md", "AGENTS.md.tmpl"},
		{"pyrightconfig.json", "pyrightconfig.json.tmpl"},
		{filepath.Join(project.DefaultFunctionsDir, "__init__.py"), ""},
		{filepath.Join(project.DefaultFunctionsDir, "_shared", "__init__.py"), ""},
		{filepath.Join(project.DefaultFunctionsDir, "hello", "__init__.py"), ""},
		{filepath.Join(project.DefaultFunctionsDir, "hello", project.EntrypointName), "main.py.tmpl"},
	}

	var written, skipped []string
	for _, f := range files {
		target := filepath.Join(abs, f.path)
		if _, err := os.Stat(target); err == nil && !stackInitForce {
			skipped = append(skipped, f.path)
			continue
		}
		body, err := renderTemplate(f.tmpl, data)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, body, 0o644); err != nil {
			return err
		}
		written = append(written, f.path)
	}

	for _, p := range written {
		PrintInfo("  created  " + p)
	}
	for _, p := range skipped {
		PrintInfo("  exists   " + p + " (use --force to overwrite)")
	}

	// The editor will report unresolved imports until .notte/venv exists, and
	// init deliberately does not build it: scaffolding should work offline and
	// without credentials. Saying so here costs a line and saves someone
	// debugging a project that is not actually broken.
	return PrintResult(
		fmt.Sprintf("\nStack ready in %s.\n\n"+
			"  next: notte stack sync    builds .notte/venv so your editor resolves imports\n"+
			"        notte stack check   bundles and validates every function\n\n"+
			"  Until sync runs, an editor will report pydantic and notte_sdk as unresolved:\n"+
			"  there is no environment for it to resolve against yet.",
			abs),
		map[string]any{"root": abs, "created": written, "skipped": skipped},
	)
}

// renderTemplate expands a scaffold template. An empty name means an empty
// file, which is what __init__.py is.
func renderTemplate(name string, data map[string]string) ([]byte, error) {
	if name == "" {
		return nil, nil
	}
	raw, err := stackTemplates.ReadFile("stacktmpl/" + name)
	if err != nil {
		return nil, err
	}
	tmpl, err := template.New(name).Parse(string(raw))
	if err != nil {
		return nil, err
	}
	var out strings.Builder
	if err := tmpl.Execute(&out, data); err != nil {
		return nil, err
	}
	return []byte(out.String()), nil
}

// ---------------------------------------------------------------- new

var stackNewCmd = &cobra.Command{
	Use:   "new <name>",
	Short: "Scaffold one function",
	Args:  cobra.ExactArgs(1),
	RunE:  runStackNew,
}

func init() { stackCmd.AddCommand(stackNewCmd) }

func runStackNew(cmd *cobra.Command, args []string) error {
	cfg, err := loadStack()
	if err != nil {
		return err
	}
	name := args[0]
	dir := filepath.Join(cfg.FunctionsPath(), name)
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("%s already exists", filepath.Join(cfg.Project.FunctionsDir, name))
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	body, err := renderTemplate("main.py.tmpl", map[string]string{"Name": name})
	if err != nil {
		return err
	}
	for path, content := range map[string][]byte{
		filepath.Join(dir, "__init__.py"):          nil,
		filepath.Join(dir, project.EntrypointName): body,
	} {
		if err := os.WriteFile(path, content, 0o644); err != nil {
			return err
		}
	}

	entry := filepath.Join(cfg.Project.FunctionsDir, name, project.EntrypointName)
	return PrintResult(
		fmt.Sprintf("created %s", entry),
		map[string]any{"name": name, "entrypoint": entry},
	)
}
