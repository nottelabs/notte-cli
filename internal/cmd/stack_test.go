package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nottelabs/notte-cli/internal/api"
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

// Upstream names are not unique — a real workspace has several functions
// called "test". Slugging without resolving that collapses them onto one path,
// and the lock keeps whichever id was recorded last while the rest become
// unreachable. Found against staging, where 3,200 functions collapsed to 23.
func TestAssignNamesResolvesCollisionsDeterministically(t *testing.T) {
	name := func(s string) *string { return &s }
	remote := []api.FunctionListItemResponse{
		{FunctionId: "ccc", Name: name("test")},
		{FunctionId: "aaa", Name: name("test")},
		{FunctionId: "bbb", Name: name("test")},
		{FunctionId: "ddd", Name: name("unique")},
	}

	got := assignNames(remote)
	if len(got) != 4 {
		t.Fatalf("every function needs a name: %v", got)
	}
	seen := map[string]bool{}
	for _, n := range got {
		if seen[n] {
			t.Fatalf("two functions share the local name %q: %v", n, got)
		}
		seen[n] = true
	}
	// Lowest id keeps the bare slug, so the mapping does not shuffle when the
	// listing order changes.
	if got["aaa"] != "test" {
		t.Errorf("lowest id should keep the bare slug, got %q", got["aaa"])
	}
	if got["ddd"] != "unique" {
		t.Errorf("an uncontested name should be untouched, got %q", got["ddd"])
	}

	// Stable across a reordered listing.
	shuffled := []api.FunctionListItemResponse{remote[3], remote[1], remote[0], remote[2]}
	for id, n := range assignNames(shuffled) {
		if got[id] != n {
			t.Errorf("id %s named %q then %q — assignment is not stable", id, got[id], n)
		}
	}
}

func TestFunctionSlugSanitises(t *testing.T) {
	name := func(s string) *string { return &s }
	cases := map[string]string{
		"HN Top Posts":           "hn_top_posts",
		"managed auth - bluesky": "managed_auth_bluesky",
		"  padded  ":             "padded",
	}
	for in, want := range cases {
		if got := functionSlug(api.FunctionListItemResponse{Name: name(in)}); got != want {
			t.Errorf("%q -> %q, want %q", in, got, want)
		}
	}
	if got := functionSlug(api.FunctionListItemResponse{}); got != "" {
		t.Errorf("a nameless function should slug to empty, got %q", got)
	}
}

func TestReadEnvFileParsesTheUsualForms(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env.prod")
	body := "# comment\n\nPLAIN=value\nexport EXPORTED=two\nQUOTED=\"three\"\nSINGLE='four'\nSPACED = five \nnot_upper=ignored\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := readEnvFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"PLAIN": "value", "EXPORTED": "two", "QUOTED": "three", "SINGLE": "four", "SPACED": "five"}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
	// Secret names are uppercase by API rule, so a lowercase key is not one.
	if _, ok := got["not_upper"]; ok {
		t.Error("lowercase keys are not valid secret names and should be skipped")
	}
}

func TestSharedSourcesOnlyReportsFilesUsedTwice(t *testing.T) {
	got := sharedSources(map[string][]string{
		"_shared/http.py": {"b", "a"},
		"solo/main.py":    {"solo"},
	})
	if len(got) != 1 || got[0].Path != "_shared/http.py" {
		t.Fatalf("got %+v", got)
	}
	if got[0].Functions[0] != "a" {
		t.Errorf("users should be sorted, got %v", got[0].Functions)
	}
}

// The environment a command records under and the endpoint it writes to must
// be resolved together. Greptile caught these apart: --env chose the lockfile
// key while the client came from the ambient NOTTE_API_URL, so
// `deploy --env staging` against a prod default wrote functions to prod and
// filed their ids under staging.
func TestNamingAnEnvironmentCannotSilentlyRetarget(t *testing.T) {
	defer func() { stackEnv = "" }()
	dir := initInto(t)
	cfg, err := project.Load(dir)
	if err != nil {
		t.Fatal(err)
	}

	// staging named, nothing declares it, and the endpoint is prod.
	t.Setenv("NOTTE_API_URL", "https://api.notte.cc")
	t.Setenv("NOTTE_API_KEY", "sk-test")
	stackEnv = "staging"

	if _, err := resolveStackTarget(cfg); err == nil {
		t.Fatal("naming an undeclared environment against a prod endpoint must fail, " +
			"not quietly deploy to prod and record it as staging")
	} else {
		for _, want := range []string{"staging", "api.notte.cc"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error should name %q so the mismatch is obvious: %v", want, err)
			}
		}
	}
}

// With no --env the label follows the endpoint, so a single-environment
// project needs no configuration at all.
func TestUnnamedEnvironmentFollowsTheEndpoint(t *testing.T) {
	defer func() { stackEnv = "" }()
	dir := initInto(t)
	cfg, err := project.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("NOTTE_API_KEY", "sk-test")
	t.Setenv("NOTTE_API_URL", "https://us-staging.notte.cc")
	stackEnv = ""

	dest, err := resolveStackTarget(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if dest.Env != "staging" {
		t.Fatalf("label = %q, want staging", dest.Env)
	}
	if dest.APIURL != "https://us-staging.notte.cc" {
		t.Fatalf("url = %q", dest.APIURL)
	}
}

// A declared environment supplies its own endpoint, so it works regardless of
// what NOTTE_API_URL happens to be.
func TestDeclaredEnvironmentSuppliesItsOwnEndpoint(t *testing.T) {
	defer func() { stackEnv = "" }()
	dir := writeStack(t, map[string]string{
		"notte.toml": `[project]
name = "demo"

[env.staging]
api_url = "https://us-staging.notte.cc"
api_key = "${env:STAGING_KEY}"
`,
		"functions/__init__.py":       "",
		"functions/hello/__init__.py": "",
		"functions/hello/main.py":     "def run():\n    return 1\n",
	})
	cfg, err := project.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("STAGING_KEY", "sk-staging")
	t.Setenv("NOTTE_API_URL", "https://api.notte.cc") // ambient prod, deliberately
	stackEnv = "staging"

	dest, err := resolveStackTarget(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if dest.APIURL != "https://us-staging.notte.cc" {
		t.Fatalf("a declared api_url must win over the ambient one, got %q", dest.APIURL)
	}
	if dest.Env != "staging" {
		t.Fatalf("label = %q", dest.Env)
	}
}

// writeStack builds a project directory from a path->content map.
func writeStack(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// doctor is a diagnostic command, so an ambient client labelled with --env
// would report another environment's Python version and package list as if
// they were this one's — a confident lie from the command people run when
// nothing else works. Greptile caught this one command left unconverted.
func TestDoctorResolvesTheSelectedEnvironment(t *testing.T) {
	defer func() { stackEnv = "" }()
	dir := writeStack(t, map[string]string{
		"notte.toml": `[project]
name = "demo"

[env.staging]
api_url = "https://us-staging.notte.cc"
api_key = "${env:STAGING_KEY}"
`,
	})
	cfg, err := project.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("STAGING_KEY", "sk-staging")
	t.Setenv("NOTTE_API_URL", "https://api.notte.cc") // ambient prod
	stackEnv = "staging"

	client, label, err := doctorClient(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if label != "staging" {
		t.Fatalf("label = %q, want staging", label)
	}
	if client.BaseURL() != "https://us-staging.notte.cc" {
		t.Fatalf("doctor would report on %q while labelling it staging", client.BaseURL())
	}
}

// Outside a stack there is no notte.toml to resolve against, and doctor still
// has to work: it is what people run when nothing else does.
func TestDoctorWorksOutsideAStack(t *testing.T) {
	defer func() { stackEnv = "" }()
	stackEnv = ""
	t.Setenv("NOTTE_API_KEY", "sk-test")
	t.Setenv("NOTTE_API_URL", "https://us-dev.notte.cc")

	client, label, err := doctorClient(nil, errNoStackForTest)
	if err != nil {
		t.Fatal(err)
	}
	if label != "dev" {
		t.Fatalf("label = %q, want dev", label)
	}
	if client.BaseURL() != "https://us-dev.notte.cc" {
		t.Fatalf("url = %q", client.BaseURL())
	}
}

var errNoStackForTest = fmt.Errorf("no stack here")

// A notte.toml that exists but fails to parse is not the same as no project.
// Branching on the load error alone made them indistinguishable, so
// `doctor --env staging` beside a malformed config silently reported on the
// ambient endpoint. A flag that cannot be honoured is refused.
func TestDoctorRefusesEnvItCannotResolve(t *testing.T) {
	defer func() { stackEnv = "" }()
	t.Setenv("NOTTE_API_KEY", "sk-test")
	t.Setenv("NOTTE_API_URL", "https://api.notte.cc") // ambient prod
	stackEnv = "staging"

	_, _, err := doctorClient(nil, errNoStackForTest)
	if err == nil {
		t.Fatal("--env staging must not silently fall through to the prod endpoint")
	}
	for _, want := range []string{"staging", "api.notte.cc"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should name %q: %v", want, err)
		}
	}
}

// The fallback still works when --env agrees with the endpoint, or is absent.
func TestDoctorFallbackAllowedWhenEnvMatches(t *testing.T) {
	defer func() { stackEnv = "" }()
	t.Setenv("NOTTE_API_KEY", "sk-test")
	t.Setenv("NOTTE_API_URL", "https://us-dev.notte.cc")

	stackEnv = "dev"
	if _, label, err := doctorClient(nil, errNoStackForTest); err != nil || label != "dev" {
		t.Fatalf("matching --env should be honoured: label=%q err=%v", label, err)
	}
	stackEnv = ""
	if _, label, err := doctorClient(nil, errNoStackForTest); err != nil || label != "dev" {
		t.Fatalf("no --env should follow the endpoint: label=%q err=%v", label, err)
	}
}

// A project that exists but will not load is not the same as no project. Its
// [env.*] blocks may name an endpoint other than the ambient one, and being
// unable to read them is precisely why the ambient endpoint cannot stand in —
// even when the labels happen to agree.
func TestDoctorRefusesEnvWhenTheConfigIsUnreadable(t *testing.T) {
	defer func() { stackEnv = "" }()
	dir := writeStack(t, map[string]string{"notte.toml": "[project\nname = broken\n"})
	wd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	t.Setenv("NOTTE_API_KEY", "sk-test")
	t.Setenv("NOTTE_API_URL", "https://us-staging.notte.cc")
	stackEnv = "staging" // label matches the ambient endpoint, and still must refuse

	_, _, err := doctorClient(nil, errNoStackForTest)
	if err == nil {
		t.Fatal("an unreadable notte.toml must not let --env fall through to the ambient endpoint")
	}
	if !strings.Contains(err.Error(), project.ConfigName) {
		t.Errorf("error should point at the config: %v", err)
	}
}
