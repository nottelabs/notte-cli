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

func TestEnvNameDefaultsToProd(t *testing.T) {
	stackEnv = ""
	if got := envName(); got != project.DefaultEnv {
		t.Fatalf("envName() = %q, want %q", got, project.DefaultEnv)
	}
	stackEnv = "staging"
	defer func() { stackEnv = "" }()
	if got := envName(); got != "staging" {
		t.Fatalf("envName() = %q", got)
	}
}
