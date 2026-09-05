package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// write builds a project directory from a path->content map.
func write(t *testing.T, files map[string]string) string {
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

// The common case: no environments, no credentials, prod implied. If this ever
// needs more than [project] to work, the "environments are opt-in" promise has
// been broken.
func TestMinimalConfigNeedsNoEnvironments(t *testing.T) {
	dir := write(t, map[string]string{
		"notte.toml": "[project]\nname = \"demo\"\n",
	})
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Project.FunctionsDir != DefaultFunctionsDir {
		t.Fatalf("functions_dir = %q, want %q", cfg.Project.FunctionsDir, DefaultFunctionsDir)
	}
	env, err := cfg.ResolveEnv("")
	if err != nil {
		t.Fatalf("default env must resolve without an [env.*] block: %v", err)
	}
	if env.APIURL != "" {
		t.Fatalf("expected an empty api_url to fall through to CLI defaults, got %q", env.APIURL)
	}
}

// TOML silently ignores keys it cannot place, so a typo would otherwise be a
// setting that never applies.
func TestUnknownKeyIsRejected(t *testing.T) {
	dir := write(t, map[string]string{
		"notte.toml": "[project]\nname = \"demo\"\nfunctions_dirr = \"fns\"\n",
	})
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected an error for a misspelled key")
	}
	if !strings.Contains(err.Error(), "functions_dirr") {
		t.Fatalf("error should name the key, got %v", err)
	}
}

func TestUndeclaredEnvIsRejectedButDefaultIsNot(t *testing.T) {
	dir := write(t, map[string]string{"notte.toml": "[project]\nname = \"demo\"\n"})
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cfg.ResolveEnv("staging"); err == nil {
		t.Fatal("naming an undefined env should fail")
	}
	if _, err := cfg.ResolveEnv(DefaultEnv); err != nil {
		t.Fatalf("the default env must resolve even when undeclared: %v", err)
	}
}

func TestEnvExtendsInherits(t *testing.T) {
	t.Setenv("TEST_KEY", "sk-test")
	dir := write(t, map[string]string{
		"notte.toml": `[project]
name = "demo"

[env.dev]
api_url = "https://us-dev.notte.cc"
api_key = "${env:TEST_KEY}"
headers = { "x-a" = "1" }

[env.preview]
extends = "dev"
headers = { "x-b" = "2" }
`,
	})
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	env, err := cfg.ResolveEnv("preview")
	if err != nil {
		t.Fatal(err)
	}
	if env.APIURL != "https://us-dev.notte.cc" {
		t.Fatalf("api_url not inherited: %q", env.APIURL)
	}
	if env.APIKey != "sk-test" {
		t.Fatalf("api_key not inherited/expanded: %q", env.APIKey)
	}
	if env.Headers["x-a"] != "1" || env.Headers["x-b"] != "2" {
		t.Fatalf("headers not merged: %v", env.Headers)
	}
}

func TestEnvExtendsUnknownIsRejected(t *testing.T) {
	dir := write(t, map[string]string{
		"notte.toml": "[project]\nname=\"d\"\n\n[env.preview]\nextends = \"nope\"\n",
	})
	if _, err := Load(dir); err == nil {
		t.Fatal("extends of an undefined env should fail")
	}
}

func TestEnvExtendsCycleIsRejected(t *testing.T) {
	dir := write(t, map[string]string{
		"notte.toml": "[project]\nname=\"d\"\n\n[env.a]\nextends=\"b\"\n\n[env.b]\nextends=\"a\"\n",
	})
	err := Load2(t, dir)
	if err == nil || !strings.Contains(err.Error(), "circular") {
		t.Fatalf("expected a circular-extends error, got %v", err)
	}
}

// Load2 is Load, returning only the error, for terser assertions.
func Load2(t *testing.T, dir string) error {
	t.Helper()
	_, err := Load(dir)
	return err
}

// An unresolved reference must fail loudly. Expanding to "" produces an
// api_url of "" or a credential of "", which fails far from the cause.
func TestUnsetInterpolationIsAnError(t *testing.T) {
	dir := write(t, map[string]string{
		"notte.toml": "[project]\nname=\"d\"\n\n[env.dev]\napi_key = \"${env:DEFINITELY_NOT_SET_XYZ}\"\n",
	})
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = cfg.ResolveEnv("dev")
	if err == nil {
		t.Fatal("an unset ${env:...} must be an error, never an empty string")
	}
	if !strings.Contains(err.Error(), "DEFINITELY_NOT_SET_XYZ") {
		t.Fatalf("error should name the variable, got %v", err)
	}
}

func TestUnknownInterpolationNamespaceIsAnError(t *testing.T) {
	dir := write(t, map[string]string{
		"notte.toml": "[project]\nname=\"d\"\n\n[env.dev]\napi_key = \"${vault:thing}\"\n",
	})
	cfg, _ := Load(dir)
	if _, err := cfg.ResolveEnv("dev"); err == nil {
		t.Fatal("unknown namespace should be an error")
	}
}

func TestGitInterpolation(t *testing.T) {
	dir := write(t, map[string]string{
		"notte.toml": "[project]\nname=\"d\"\n\n[env.preview]\nheaders = { \"x-db-preview\" = \"${git:branch}\" }\n",
	})
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	env, err := cfg.ResolveEnv("preview")
	if err != nil {
		t.Skipf("not in a git worktree: %v", err)
	}
	if env.Headers["x-db-preview"] == "" {
		t.Fatal("git branch did not expand")
	}
}

func TestFunctionsDirMustStayInsideTheProject(t *testing.T) {
	for _, bad := range []string{"/etc", "../outside"} {
		dir := write(t, map[string]string{
			"notte.toml": "[project]\nname=\"d\"\nfunctions_dir = \"" + bad + "\"\n",
		})
		if _, err := Load(dir); err == nil {
			t.Fatalf("functions_dir %q should be rejected", bad)
		}
	}
}

// Commands must work from a subdirectory, the way git does.
func TestFindWalksUp(t *testing.T) {
	dir := write(t, map[string]string{
		"notte.toml":           "[project]\nname=\"d\"\n",
		"functions/fn/main.py": "def run():\n    return 1\n",
	})
	found, err := Find(filepath.Join(dir, "functions", "fn"))
	if err != nil {
		t.Fatal(err)
	}
	if resolved, _ := filepath.EvalSymlinks(found); resolved != mustEval(t, dir) {
		t.Fatalf("found %q, want %q", found, dir)
	}
}

func mustEval(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestFindReportsWhenThereIsNoProject(t *testing.T) {
	if _, err := Find(t.TempDir()); err == nil {
		t.Fatal("expected an error outside a project")
	}
}

func TestCronVariablesParse(t *testing.T) {
	dir := write(t, map[string]string{
		"notte.toml": `[project]
name = "d"

[functions.hello]
cron = "cron(0 9 * * ? *)"
cron_variables = { name = "scheduled", limit = 10 }
`,
	})
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	fn := cfg.Functions["hello"]
	if fn.CronVariables["name"] != "scheduled" {
		t.Fatalf("name = %v", fn.CronVariables["name"])
	}
	// TOML integers decode as int64; the schedule endpoint takes any JSON value.
	if got, ok := fn.CronVariables["limit"].(int64); !ok || got != 10 {
		t.Fatalf("limit = %#v", fn.CronVariables["limit"])
	}
}

// A schedule that supplies a key run() does not take is rejected by the server
// only when the schedule is set, and a cron failing at 09:00 is a bad way to
// find a typo.
func TestScheduleProblemsRejectsUnknownVariable(t *testing.T) {
	fn := FunctionConfig{Cron: "cron(0 9 * * ? *)", CronVariables: map[string]any{"naem": "x"}}
	problems := fn.ScheduleProblems("hello", []Param{{Name: "name", HasDefault: true}})
	if len(problems) != 1 {
		t.Fatalf("got %v", problems)
	}
	if !strings.Contains(problems[0], "naem") || !strings.Contains(problems[0], "name") {
		t.Fatalf("error should name both the typo and the real parameter: %v", problems[0])
	}
}

// A parameter without a default cannot be filled in by the runtime, so a
// schedule that omits it can never succeed.
func TestScheduleProblemsRequiresParametersWithoutDefaults(t *testing.T) {
	fn := FunctionConfig{Cron: "cron(0 9 * * ? *)"}
	problems := fn.ScheduleProblems("hello", []Param{{Name: "url"}})
	if len(problems) != 1 || !strings.Contains(problems[0], "url") {
		t.Fatalf("got %v", problems)
	}

	fn.CronVariables = map[string]any{"url": "https://x.test"}
	if problems := fn.ScheduleProblems("hello", []Param{{Name: "url"}}); len(problems) != 0 {
		t.Fatalf("supplying it should satisfy the check: %v", problems)
	}
}

func TestScheduleProblemsRejectsVariablesWithoutACron(t *testing.T) {
	fn := FunctionConfig{CronVariables: map[string]any{"name": "x"}}
	problems := fn.ScheduleProblems("hello", []Param{{Name: "name", HasDefault: true}})
	if len(problems) != 1 || !strings.Contains(problems[0], "never used") {
		t.Fatalf("got %v", problems)
	}
}

// An unscheduled function is not required to supply anything.
func TestScheduleProblemsSilentWithoutASchedule(t *testing.T) {
	fn := FunctionConfig{}
	if problems := fn.ScheduleProblems("hello", []Param{{Name: "url"}}); len(problems) != 0 {
		t.Fatalf("got %v", problems)
	}
}

// The fields `notte functions configure` can set are declarable, so a deploy
// pushes them rather than leaving copy editable only upstream.
func TestFunctionMetadataFieldsParse(t *testing.T) {
	dir := write(t, map[string]string{
		"notte.toml": `[project]
name = "d"

[functions.hello]
name         = "Hello"
description  = "Says hello"
domain       = "example.com"
instructions = "Takes ~30s."
self_healing = true
`,
	})
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	fn := cfg.Functions["hello"]
	if fn.Domain != "example.com" || fn.Instructions != "Takes ~30s." {
		t.Fatalf("got %+v", fn)
	}
	if fn.SelfHealing == nil || !*fn.SelfHealing {
		t.Fatalf("self_healing = %v", fn.SelfHealing)
	}
}

// self_healing is a pointer so "unset" differs from "explicitly false".
// Turning a feature off because a config did not mention it would be a
// surprising deploy.
func TestSelfHealingDistinguishesUnsetFromFalse(t *testing.T) {
	unset := write(t, map[string]string{
		"notte.toml": "[project]\nname=\"d\"\n\n[functions.hello]\ndescription = \"x\"\n",
	})
	cfg, err := Load(unset)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Functions["hello"].SelfHealing != nil {
		t.Fatal("an unmentioned self_healing must stay nil, not become false")
	}

	explicit := write(t, map[string]string{
		"notte.toml": "[project]\nname=\"d\"\n\n[functions.hello]\nself_healing = false\n",
	})
	cfg, err = Load(explicit)
	if err != nil {
		t.Fatal(err)
	}
	sh := cfg.Functions["hello"].SelfHealing
	if sh == nil || *sh {
		t.Fatalf("an explicit false must be sent, got %v", sh)
	}
}
