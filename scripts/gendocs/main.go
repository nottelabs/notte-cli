// Command gendocs writes the CLI reference for docs.notte.cc.
//
// It walks the assembled cobra tree, so the reference cannot drift from the
// CLI: a new command, flag, or help string shows up the next time it runs.
// The output is one overview page plus one page per top-level command group,
// as Mintlify MDX:
//
//	go run ./scripts/gendocs -out <dir>
//
// The notte repository runs this against its notte-cli submodule and commits
// the result, so the docs follow every submodule bump.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/nottelabs/notte-cli/internal/cmd"
)

// docsBase is where the pages are served from on docs.notte.cc. Card links on
// the overview page point here.
const docsBase = "/cli-reference"

// icons gives each group a card icon on the overview page. A group missing
// from this map still gets a card, with the default icon.
var icons = map[string]string{
	"auth":      "key",
	"files":     "file",
	"functions": "code",
	"page":      "hand-pointer",
	"payment":   "credit-card",
	"personas":  "user",
	"profiles":  "fingerprint",
	"sessions":  "browser",
	"skill":     "robot",
	"tunnel":    "network-wired",
	"usage":     "chart-line",
	"vaults":    "lock",
}

const defaultIcon = "terminal"

func main() {
	out := flag.String("out", "", "directory to write the MDX pages into")
	flag.Parse()
	if *out == "" {
		fmt.Fprintln(os.Stderr, "usage: gendocs -out <dir>")
		os.Exit(2)
	}
	if err := run(cmd.RootCommand(), *out); err != nil {
		fmt.Fprintln(os.Stderr, "gendocs:", err)
		os.Exit(1)
	}
}

func run(root *cobra.Command, out string) error {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	// Remove the pages a previous run wrote, so a removed group does not leave
	// a stale page behind. Only generated pages: anything else in the
	// directory was written by hand and is left alone.
	existing, err := filepath.Glob(filepath.Join(out, "*.mdx"))
	if err != nil {
		return err
	}
	for _, f := range existing {
		generated, err := isGenerated(f)
		if err != nil {
			return err
		}
		if generated {
			if err := os.Remove(f); err != nil {
				return err
			}
		}
	}

	var groups, singles []*cobra.Command
	for _, c := range visibleChildren(root) {
		if len(visibleChildren(c)) > 0 {
			groups = append(groups, c)
		} else {
			singles = append(singles, c)
		}
	}

	pages := map[string]string{"index.mdx": overviewPage(root, groups, singles)}
	for _, g := range groups {
		pages[g.Name()+".mdx"] = groupPage(root, g)
	}
	if len(singles) > 0 {
		pages["other.mdx"] = otherPage(root, singles)
	}
	for name, body := range pages {
		path := filepath.Join(out, name)
		// Generated pages were removed above, so anything still here was
		// written by hand.
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("%s is not a generated page; refusing to overwrite it", path)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func isGenerated(path string) (bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	return strings.Contains(string(b), generatedNotice), nil
}

// visibleChildren lists the commands a user can see, sorted by name. Hidden
// commands and cobra's own help and completion are left out.
func visibleChildren(c *cobra.Command) []*cobra.Command {
	var kids []*cobra.Command
	for _, k := range c.Commands() {
		if k.Hidden || k.Name() == "help" || k.Name() == "completion" {
			continue
		}
		kids = append(kids, k)
	}
	sort.Slice(kids, func(i, j int) bool { return kids[i].Name() < kids[j].Name() })
	return kids
}

func overviewPage(root *cobra.Command, groups, singles []*cobra.Command) string {
	var b strings.Builder
	// The sidebar already groups these pages under "CLI", so the overview is
	// listed as "Overview" rather than a second "CLI".
	b.WriteString("---\ntitle: \"CLI\"\nsidebarTitle: \"Overview\"\ndescription: \"Drive Notte browser sessions, functions, and vaults from your terminal\"\n---\n\n")
	b.WriteString(generatedNotice)
	b.WriteString(`The [Notte CLI](https://github.com/nottelabs/notte-cli) gives you every Notte resource from the command line. Pair it with shell scripts, CI pipelines, or coding agents.

## Installation

` + "```bash" + `
# Install script (macOS and Linux)
curl -fsSL https://notte.cc/install-cli.sh | sh

# Homebrew
brew tap nottelabs/notte-cli https://github.com/nottelabs/notte-cli.git
brew install notte

# Go
go install github.com/nottelabs/notte-cli/cmd/notte@latest
` + "```" + `

Verify the installation:

` + "```bash" + `
notte version
` + "```" + `

## Authentication

The CLI reads your API key from, in order: the ` + "`NOTTE_API_KEY`" + ` environment variable, the system keyring (set by ` + "`notte auth login`" + `), then ` + "`~/.notte/cli/config.json`" + `. See [Authentication](/api-reference/authentication) to get a key.

` + "```bash" + `
export NOTTE_API_KEY=your_api_key_here
# or, for local development
notte auth login
` + "```" + `

## Commands

<Columns cols={2}>
`)
	for _, g := range groups {
		card(&b, title(g.Name()), icons[g.Name()], docsBase+"/"+g.Name(), g.Short)
	}
	if len(singles) > 0 {
		names := make([]string, len(singles))
		for i, s := range singles {
			names[i] = s.Name()
		}
		card(&b, "Other commands", defaultIcon, docsBase+"/other", "`"+strings.Join(names, "`, `")+"`")
	}
	b.WriteString("</Columns>\n\n## Global flags\n\nThese flags work with every command.\n\n")
	flagTable(&b, root.PersistentFlags())
	b.WriteString(`
## JSON output

Commands that return data accept ` + "`--output json`" + ` (or ` + "`-o json`" + `) for machine-readable output. Commands that wrap another tool, such as ` + "`skill add`" + `, print that tool's output as is:

` + "```bash" + `
notte sessions start -o json
notte functions list -o json
` + "```" + `
`)
	return b.String()
}

func groupPage(root, g *cobra.Command) string {
	var b strings.Builder
	frontmatter(&b, title(g.Name()), g.Short)
	b.WriteString(generatedNotice)
	if g.Long != "" {
		b.WriteString(prose(g.Long) + "\n\n")
	}
	var walk func(*cobra.Command)
	walk = func(c *cobra.Command) {
		if c.Runnable() {
			commandSection(&b, root, c)
		}
		for _, k := range visibleChildren(c) {
			walk(k)
		}
	}
	walk(g)
	return b.String()
}

func otherPage(root *cobra.Command, singles []*cobra.Command) string {
	var b strings.Builder
	frontmatter(&b, "Other commands", "Standalone Notte CLI commands")
	b.WriteString(generatedNotice)
	for _, c := range singles {
		commandSection(&b, root, c)
	}
	return b.String()
}

func commandSection(b *strings.Builder, root, c *cobra.Command) {
	fmt.Fprintf(b, "## `%s`\n\n", usage(c))
	desc := c.Long
	if desc == "" {
		desc = c.Short
	}
	if desc != "" {
		b.WriteString(prose(desc) + "\n\n")
	}
	if len(c.Aliases) > 0 {
		aliases := make([]string, len(c.Aliases))
		for i, a := range c.Aliases {
			aliases[i] = "`" + a + "`"
		}
		fmt.Fprintf(b, "Aliases: %s\n\n", strings.Join(aliases, ", "))
	}
	flags := commandFlags(root, c)
	if flags.HasAvailableFlags() {
		flagTable(b, flags)
		b.WriteString("\n")
	}
	if ex := strings.Trim(c.Example, "\n"); strings.TrimSpace(ex) != "" {
		b.WriteString("```bash\n" + strings.TrimRight(dedent(ex), " \t\n") + "\n```\n\n")
	}
}

// usage is the command as typed, with its positional arguments and without
// cobra's "[flags]" suffix: "notte files upload <file-path>".
func usage(c *cobra.Command) string {
	line := c.CommandPath()
	if _, args, ok := strings.Cut(c.Use, " "); ok {
		line += " " + args
	}
	return line
}

// commandFlags is what a command accepts beyond the global flags: its own
// flags plus persistent flags from a parent group, such as page --session-id.
// A local flag that shadows a global one, such as page click --timeout in
// milliseconds, is the command's own and is kept.
func commandFlags(root, c *cobra.Command) *pflag.FlagSet {
	fs := pflag.NewFlagSet(c.Name(), pflag.ContinueOnError)
	fs.SortFlags = true
	add := func(f *pflag.Flag) {
		if f.Name == "help" || f.Hidden || root.PersistentFlags().Lookup(f.Name) == f {
			return
		}
		if fs.Lookup(f.Name) == nil {
			fs.AddFlag(f)
		}
	}
	c.LocalFlags().VisitAll(add)
	c.InheritedFlags().VisitAll(add)
	return fs
}

func flagTable(b *strings.Builder, fs *pflag.FlagSet) {
	b.WriteString("| Flag | Description |\n| - | - |\n")
	fs.VisitAll(func(f *pflag.Flag) {
		if f.Hidden || f.Name == "help" {
			return
		}
		name := "--" + f.Name
		varname, text := pflag.UnquoteUsage(f)
		if varname != "" {
			name += " <" + varname + ">"
		}
		name = "`" + name + "`"
		if f.Shorthand != "" {
			name += ", `-" + f.Shorthand + "`"
		}
		text = cell(text)
		if isRequired(f) && !strings.Contains(strings.ToLower(text), "required") {
			text = strings.TrimSuffix(text, ".") + ". Required."
		}
		if d := defaultValue(f); d != "" {
			text += " (default: `" + strings.ReplaceAll(d, "|", "\\|") + "`)"
		}
		fmt.Fprintf(b, "| %s | %s |\n", strings.ReplaceAll(name, "|", "\\|"), strings.TrimSpace(text))
	})
}

func isRequired(f *pflag.Flag) bool {
	v, ok := f.Annotations[cobra.BashCompOneRequiredFlag]
	return ok && len(v) > 0 && v[0] == "true"
}

// defaultValue returns the flag's default when it is worth printing: zero
// values (false, 0, "", []) say nothing.
func defaultValue(f *pflag.Flag) string {
	switch f.DefValue {
	case "", "false", "0", "[]", "0s":
		return ""
	}
	return f.DefValue
}

func frontmatter(b *strings.Builder, title, description string) {
	fmt.Fprintf(b, "---\ntitle: %q\ndescription: %q\n---\n\n", title, strings.TrimSpace(description))
}

const generatedNotice = "{/* Generated by notte-cli scripts/gendocs. Do not edit by hand. */}\n\n"

func card(b *strings.Builder, title, icon, href, text string) {
	if icon == "" {
		icon = defaultIcon
	}
	fmt.Fprintf(b, "  <Card icon=%q title=%q href=%q>\n    %s\n  </Card>\n", icon, title, href, escape(text))
}

// title turns a command name into a page title: "functions" -> "Functions".
func title(name string) string {
	words := strings.Split(name, "-")
	for i, w := range words {
		if w != "" {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// prose renders help text as Markdown. Help strings are hard-wrapped for a
// terminal, so wrapped lines are joined back into paragraphs, list items stay
// on their own lines, and indented lines (usually examples) become a code
// block.
func prose(s string) string {
	var out []string
	var para, code []string
	flushPara := func() {
		if len(para) > 0 {
			out = append(out, escape(strings.Join(para, " ")))
			para = nil
		}
	}
	flushCode := func() {
		if len(code) > 0 {
			out = append(out, "```bash\n"+dedent(strings.Join(code, "\n"))+"\n```")
			code = nil
		}
	}
	for _, l := range strings.Split(strings.TrimSpace(s), "\n") {
		switch {
		case strings.TrimSpace(l) == "":
			flushPara()
			flushCode()
		case strings.HasPrefix(l, "  ") || strings.HasPrefix(l, "\t"):
			flushPara()
			code = append(code, l)
		case isListItem(l):
			flushCode()
			flushPara()
			para = append(para, l)
		default:
			// A wrapped line, including the continuation of a list item.
			flushCode()
			para = append(para, l)
		}
	}
	flushPara()
	flushCode()
	return strings.Join(out, "\n\n")
}

func isListItem(l string) bool {
	return strings.HasPrefix(l, "- ") || strings.HasPrefix(l, "* ")
}

// cell renders help text inside a table cell, on one line.
func cell(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	return strings.ReplaceAll(escape(s), "|", "\\|")
}

// escape makes help text safe for MDX, where < and { start JSX. Text inside
// backticks is left alone, since MDX treats inline code literally. Flags in
// prose become inline code: rendered as text, "--name" turns into an em dash.
func escape(s string) string {
	parts := strings.Split(s, "`")
	for i := 0; i < len(parts); i += 2 {
		p := mdxEscaper.Replace(parts[i])
		parts[i] = flagPattern.ReplaceAllString(p, "${1}`${2}`")
	}
	return strings.Join(parts, "`")
}

var (
	mdxEscaper  = strings.NewReplacer("<", "&lt;", ">", "&gt;", "{", "&#123;", "}", "&#125;")
	flagPattern = regexp.MustCompile(`(^|[\s(])(--[a-z][a-z0-9-]*)`)
)

// dedent strips the indentation common to every non-blank line.
func dedent(s string) string {
	lines := strings.Split(s, "\n")
	indent := -1
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		n := len(l) - len(strings.TrimLeft(l, " \t"))
		if indent == -1 || n < indent {
			indent = n
		}
	}
	for i, l := range lines {
		if len(l) >= indent && indent > 0 {
			lines[i] = l[indent:]
		}
	}
	return strings.Join(lines, "\n")
}
