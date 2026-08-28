package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nottelabs/notte-cli/internal/bundle"
	"github.com/nottelabs/notte-cli/internal/project"
)

// initInto scaffolds a stack in a temp dir by running the command's own logic,
// so the test covers what users get rather than a reimplementation.
func initInto(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	stackInitName, stackInitForce = "demo", false
	if err := runStackInit(nil, nil); err != nil {
		t.Fatalf("init: %v", err)
	}
	return dir
}

func TestInitScaffoldsALoadableStack(t *testing.T) {
	dir := initInto(t)

	for _, rel := range []string{
		"notte.toml", ".gitignore", "AGENTS.md", "pyrightconfig.json",
		"functions/__init__.py", "functions/_shared/__init__.py",
		"functions/hello/main.py",
	} {
		if _, err := os.Stat(filepath.Join(dir, rel)); err != nil {
			t.Errorf("missing %s", rel)
		}
	}

	cfg, err := project.Load(dir)
	if err != nil {
		t.Fatalf("the scaffolded config must load: %v", err)
	}
	if cfg.Project.Name != "demo" {
		t.Fatalf("name = %q", cfg.Project.Name)
	}
}

// The scaffolded project must have no [env.*] blocks. If init ever writes them
// again, a first-time user is back to thinking three credentials are a
// prerequisite for deploying anything.
func TestInitWritesNoEnvironments(t *testing.T) {
	dir := initInto(t)
	raw, err := os.ReadFile(filepath.Join(dir, project.ConfigName))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[env.") {
			t.Fatalf("scaffold declares an environment: %q", trimmed)
		}
	}
	cfg, err := project.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Envs) != 0 {
		t.Fatalf("expected no environments, got %v", cfg.Envs)
	}
	// And the default must still resolve.
	if _, err := cfg.ResolveEnv(""); err != nil {
		t.Fatalf("prod must resolve with no [env.*] block: %v", err)
	}
}

// .notte holds the venv and build output and must never be committed.
func TestInitGitignoresStateAndSecrets(t *testing.T) {
	dir := initInto(t)
	raw, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{".notte/", ".env"} {
		if !strings.Contains(string(raw), want) {
			t.Errorf(".gitignore should cover %q", want)
		}
	}
}

// The scaffolded function must survive the bundler, or `init` hands the user a
// stack that fails `check` immediately.
func TestScaffoldedFunctionBundles(t *testing.T) {
	dir := initInto(t)
	cfg, err := project.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	functions, err := project.Discover(cfg)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(functions) != 1 || functions[0].Name != "hello" {
		t.Fatalf("expected one function named hello, got %+v", functions)
	}
	res, err := bundle.Bundle(os.DirFS(cfg.FunctionsPath()), functions[0].Entrypoint, bundle.Options{})
	if err != nil {
		t.Fatalf("the scaffolded function must bundle: %v", err)
	}
	if !strings.Contains(res.Code, "def run(") {
		t.Fatalf("artifact has no entrypoint:\n%s", res.Code)
	}
	// It must also satisfy the documented contract: run() returns a BaseModel
	// declared in the same file.
	if !strings.Contains(res.Code, "class Response(BaseModel)") {
		t.Fatalf("scaffold should model the return-type contract:\n%s", res.Code)
	}
}

func TestInitIsIdempotentWithoutForce(t *testing.T) {
	dir := initInto(t)
	marker := "# edited by hand\n"
	path := filepath.Join(dir, project.ConfigName)
	raw, _ := os.ReadFile(path)
	if err := os.WriteFile(path, append([]byte(marker), raw...), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runStackInit(nil, nil); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	if !strings.HasPrefix(string(after), marker) {
		t.Fatal("re-running init clobbered a hand-edited notte.toml")
	}
}

func TestNewCreatesADiscoverableFunction(t *testing.T) {
	dir := initInto(t)
	if err := runStackNew(nil, []string{"scraper"}); err != nil {
		t.Fatalf("new: %v", err)
	}
	cfg, err := project.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	functions, err := project.Discover(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, f := range functions {
		if f.Name == "scraper" {
			found = true
		}
	}
	if !found {
		t.Fatalf("new function not discovered: %+v", functions)
	}
}

func TestNewRefusesToOverwrite(t *testing.T) {
	initInto(t)
	if err := runStackNew(nil, []string{"hello"}); err == nil {
		t.Fatal("creating over an existing function should fail")
	}
}

// The lock key must describe the endpoint being written to. Recording a
// staging deploy under "prod" would make the next real prod deploy update
// whatever id happened to be filed there.
func TestEnvNameFollowsTheEndpoint(t *testing.T) {
	defer func() { stackEnv = "" }()

	stackEnv = "staging"
	if got := envName(); got != "staging" {
		t.Fatalf("an explicit --env must win: %q", got)
	}

	stackEnv = ""
	t.Setenv("NOTTE_API_URL", "https://us-staging.notte.cc")
	if got := envName(); got != "staging" {
		t.Fatalf("with NOTTE_API_URL on staging the lock key must be staging, got %q", got)
	}

	t.Setenv("NOTTE_API_URL", "https://api.notte.cc")
	if got := envName(); got != project.DefaultEnv {
		t.Fatalf("prod endpoint should map to %q, got %q", project.DefaultEnv, got)
	}
}

// sourceImports must read the sources, not bundled artifacts. A function that
// fails to bundle still has imports, and its author still needs an environment
// in which to fix it — otherwise every later diagnostic is a spurious
// unresolved-import piled on top of the real error.
func TestSourceImportsCoversUnbundlableAndUnimportedFiles(t *testing.T) {
	dir := initInto(t)
	cfg, err := project.Load(dir)
	if err != nil {
		t.Fatal(err)
	}

	// A function that cannot bundle: star import from a relative module.
	broken := filepath.Join(cfg.FunctionsPath(), "broken")
	if err := os.MkdirAll(broken, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"__init__.py": "",
		"main.py":     "import httpx\nfrom .helpers import *\n\n\ndef run():\n    return 1\n",
		"helpers.py":  "def h():\n    return 1\n",
	} {
		if err := os.WriteFile(filepath.Join(broken, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// A shared module no function imports.
	if err := os.WriteFile(filepath.Join(cfg.FunctionsPath(), "_shared", "orphan.py"),
		[]byte("import requests\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	imports, err := sourceImports(cfg)
	if err != nil {
		t.Fatal(err)
	}
	has := func(want string) bool {
		for _, got := range imports {
			if got == want {
				return true
			}
		}
		return false
	}
	if !has("httpx") {
		t.Errorf("imports of a function that cannot bundle must still be collected: %v", imports)
	}
	if !has("requests") {
		t.Errorf("imports of an unreferenced shared module must be collected: %v", imports)
	}
	if !has("pydantic") {
		t.Errorf("the scaffolded function's imports are missing: %v", imports)
	}
	for _, got := range imports {
		if strings.HasPrefix(got, ".") {
			t.Errorf("relative import leaked into the environment list: %q", got)
		}
	}
}

func TestSyncIsAliasedToInstall(t *testing.T) {
	var found bool
	for _, alias := range stackSyncCmd.Aliases {
		if alias == "install" {
			found = true
		}
	}
	if !found {
		t.Fatalf("sync should answer to install too, got %v", stackSyncCmd.Aliases)
	}
}
