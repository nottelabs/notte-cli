package bundle

import (
	"strings"
	"testing"
)

// texts is the statement text of every scanned statement, for terse assertions.
func texts(stmts []Stmt) []string {
	out := make([]string, len(stmts))
	for i, s := range stmts {
		out[i] = s.Text
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestScanSplitsSimpleStatements(t *testing.T) {
	got := texts(Scan("import os\nx = 1\n"))
	want := []string{"import os", "x = 1"}
	if !equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestScanIgnoresBlankAndCommentOnlyLines(t *testing.T) {
	got := texts(Scan("\n# a comment\n\nx = 1\n   # indented comment\n"))
	if want := []string{"x = 1"}; !equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestScanStripsTrailingComments(t *testing.T) {
	got := texts(Scan("from .parse import f  # keep f\n"))
	if want := []string{"from .parse import f"}; !equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// A '#' inside a string is not a comment. Getting this wrong truncates the
// statement and can make an import look like it imports fewer names.
func TestScanDoesNotTreatHashInStringAsComment(t *testing.T) {
	got := texts(Scan(`url = "https://x.test/#frag"` + "\n"))
	if want := []string{`url = "https://x.test/#frag"`}; !equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// The whole reason the scanner exists rather than a line split: an import can
// be parenthesised across many lines.
func TestScanJoinsParenthesisedImport(t *testing.T) {
	src := "from .parse import (\n    a,\n    b as c,\n)\nx = 1\n"
	stmts := Scan(src)
	if len(stmts) != 2 {
		t.Fatalf("expected 2 statements, got %d: %q", len(stmts), texts(stmts))
	}
	if !strings.Contains(stmts[0].Text, "a,") || !strings.Contains(stmts[0].Text, "b as c") {
		t.Fatalf("continuation not joined: %q", stmts[0].Text)
	}
	if stmts[0].StartLine != 1 || stmts[0].EndLine != 4 {
		t.Fatalf("span = %d..%d, want 1..4", stmts[0].StartLine, stmts[0].EndLine)
	}
	if stmts[1].StartLine != 5 {
		t.Fatalf("second statement starts at %d, want 5", stmts[1].StartLine)
	}
}

func TestScanJoinsBackslashContinuation(t *testing.T) {
	stmts := Scan("x = 1 + \\\n    2\ny = 3\n")
	if len(stmts) != 2 {
		t.Fatalf("expected 2 statements, got %d: %q", len(stmts), texts(stmts))
	}
	if stmts[0].EndLine != 2 {
		t.Fatalf("EndLine = %d, want 2", stmts[0].EndLine)
	}
	if stmts[1].Text != "y = 3" {
		t.Fatalf("got %q", stmts[1].Text)
	}
}

// A triple-quoted docstring containing import-like text must not produce
// statements. This is the failure that would make the bundler chase imports
// that do not exist.
func TestScanSkipsTripleQuotedContent(t *testing.T) {
	src := "\"\"\"Module doc.\n\nfrom .nope import ghost\nimport nothing\n\"\"\"\nimport requests\n"
	got := texts(Scan(src))
	if len(got) != 2 {
		t.Fatalf("expected 2 statements, got %d: %q", len(got), got)
	}
	if got[1] != "import requests" {
		t.Fatalf("second statement = %q, want %q", got[1], "import requests")
	}
	if strings.Contains(got[0], "ghost") && !strings.HasPrefix(got[0], `"""`) {
		t.Fatalf("docstring body leaked out of its literal: %q", got[0])
	}
}

func TestScanHandlesSingleQuotedTripleStrings(t *testing.T) {
	src := "x = '''\nimport ghost\n'''\ny = 1\n"
	got := texts(Scan(src))
	if len(got) != 2 || got[1] != "y = 1" {
		t.Fatalf("got %q", got)
	}
}

// An escaped quote must not close the string. If it does, everything after is
// mis-tokenized.
func TestScanHandlesEscapedQuote(t *testing.T) {
	src := `x = "she said \"hi\" # not a comment"` + "\ny = 1\n"
	got := texts(Scan(src))
	if len(got) != 2 {
		t.Fatalf("expected 2 statements, got %d: %q", len(got), got)
	}
	if got[1] != "y = 1" {
		t.Fatalf("got %q", got)
	}
}

// r"\" is a raw string whose backslash still pairs with the closing quote in
// the tokenizer. Treating raw strings as backslash-free ends the literal early.
func TestScanHandlesRawStringWithEscapedQuote(t *testing.T) {
	src := `p = r"\""` + "\n" + "y = 1\n"
	got := texts(Scan(src))
	if len(got) != 2 || got[1] != "y = 1" {
		t.Fatalf("got %q", got)
	}
}

func TestScanRecordsIndent(t *testing.T) {
	src := "def f():\n    import inner\n    return 1\nx = 2\n"
	stmts := Scan(src)
	if len(stmts) != 4 {
		t.Fatalf("expected 4 statements, got %d: %q", len(stmts), texts(stmts))
	}
	if !stmts[0].TopLevel() {
		t.Fatal("def should be top level")
	}
	if stmts[1].TopLevel() {
		t.Fatalf("indented import reported as top level (indent=%d)", stmts[1].Indent)
	}
	if !stmts[3].TopLevel() {
		t.Fatal("x = 2 should be top level")
	}
}

func TestScanHandlesFileWithoutTrailingNewline(t *testing.T) {
	got := texts(Scan("x = 1"))
	if want := []string{"x = 1"}; !equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestScanHandlesEmptyInput(t *testing.T) {
	if got := Scan(""); len(got) != 0 {
		t.Fatalf("got %d statements, want 0", len(got))
	}
}

// Line numbers are what the source map and every error message depend on, so
// they are asserted directly rather than via a golden file.
func TestScanLineNumbersSurviveStringsAndComments(t *testing.T) {
	src := "# c\n\"\"\"\ndoc\n\"\"\"\n\nimport requests\n"
	stmts := Scan(src)
	last := stmts[len(stmts)-1]
	if last.Text != "import requests" {
		t.Fatalf("last statement = %q", last.Text)
	}
	if last.StartLine != 6 {
		t.Fatalf("StartLine = %d, want 6", last.StartLine)
	}
}

func TestScanNestedBracketsStayOpen(t *testing.T) {
	src := "x = [\n  (1,\n   2),\n]\ny = 1\n"
	stmts := Scan(src)
	if len(stmts) != 2 {
		t.Fatalf("expected 2 statements, got %d: %q", len(stmts), texts(stmts))
	}
	if stmts[0].EndLine != 4 {
		t.Fatalf("EndLine = %d, want 4", stmts[0].EndLine)
	}
}

// f-strings may contain braces and quotes; they must not desynchronise the
// bracket depth or the string state.
func TestScanHandlesFString(t *testing.T) {
	src := "msg = f\"value={x['k']} #\"\ny = 1\n"
	got := texts(Scan(src))
	if len(got) != 2 || got[1] != "y = 1" {
		t.Fatalf("got %q", got)
	}
}
