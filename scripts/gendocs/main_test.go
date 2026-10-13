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
