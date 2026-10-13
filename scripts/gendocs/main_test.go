package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/cmd"
)

// Every runnable command must get a section, or the reference silently loses
// it.
func TestEveryCommandIsDocumented(t *testing.T) {
	out := t.TempDir()
	root := cmd.RootCommand()
	if err := run(root, out); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob(filepath.Join(out, "*.mdx"))
	if err != nil {
		t.Fatal(err)
	}
	var all strings.Builder
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		all.Write(b)
	}
	docs := all.String()

	var walk func(*cobra.Command)
	walk = func(c *cobra.Command) {
		if c != root && c.Runnable() {
			if !strings.Contains(docs, "## `"+usage(c)+"`") {
				t.Errorf("%s has no section in the reference", c.CommandPath())
			}
		}
		for _, k := range visibleChildren(c) {
			walk(k)
		}
	}
	walk(root)
}

func TestEscapeWrapsFlagsInProse(t *testing.T) {
	got := escape("Pass --profile-id (or --name) but not `run --runtime`")
	want := "Pass `--profile-id` (or `--name`) but not `run --runtime`"
	if got != want {
		t.Errorf("escape = %q, want %q", got, want)
	}
}

func TestEscapeLeavesInlineCodeAlone(t *testing.T) {
	got := escape("pass {\"a\": 1} or `{\"a\": 1}` to <id>")
	want := "pass &#123;\"a\": 1&#125; or `{\"a\": 1}` to &lt;id&gt;"
	if got != want {
		t.Errorf("escape = %q, want %q", got, want)
	}
}

// A local flag that shares a global flag's name is a different flag and must
// be documented on its command.
func TestShadowedGlobalFlagIsDocumented(t *testing.T) {
	root := cmd.RootCommand()
	click, _, err := root.Find([]string{"page", "click"})
	if err != nil {
		t.Fatal(err)
	}
	if commandFlags(root, click).Lookup("timeout") == nil {
		t.Error("page click --timeout is missing from its flags")
	}
	sessions, _, err := root.Find([]string{"sessions", "list"})
	if err != nil {
		t.Fatal(err)
	}
	if commandFlags(root, sessions).Lookup("timeout") != nil {
		t.Error("the global --timeout is listed as a sessions list flag")
	}
}

func TestHandWrittenPagesAreKept(t *testing.T) {
	out := t.TempDir()
	keep := filepath.Join(out, "notes.mdx")
	if err := os.WriteFile(keep, []byte("hand-written"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run(cmd.RootCommand(), out); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Errorf("hand-written page was removed: %v", err)
	}
	// A second run replaces its own pages.
	if err := run(cmd.RootCommand(), out); err != nil {
		t.Fatalf("rerun: %v", err)
	}

	// A hand-written page with a generated page's name stops the run before
	// anything changes, so the previous reference stays whole.
	clash := filepath.Join(out, "auth.mdx")
	if err := os.WriteFile(clash, []byte("hand-written"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := run(cmd.RootCommand(), out); err == nil {
		t.Error("a hand-written page with a generated page's name was overwritten")
	}
	if _, err := os.Stat(filepath.Join(out, "sessions.mdx")); err != nil {
		t.Errorf("a refused run removed generated pages: %v", err)
	}
}

// The output directory is read as a path, not a glob pattern.
func TestRerunInBracketedDirectory(t *testing.T) {
	out := filepath.Join(t.TempDir(), "[draft]")
	for i := 0; i < 2; i++ {
		if err := run(cmd.RootCommand(), out); err != nil {
			t.Fatalf("run %d: %v", i+1, err)
		}
	}
}
