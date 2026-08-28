package bundle

import "testing"

// parseOne scans a single statement and parses it as an import.
func parseOne(t *testing.T, src string) Import {
	t.Helper()
	stmts := Scan(src)
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement from %q, got %d", src, len(stmts))
	}
	im, ok := ParseImport(stmts[0])
	if !ok {
		t.Fatalf("%q did not parse as an import", src)
	}
	return im
}

func TestParseImportAbsolute(t *testing.T) {
	im := parseOne(t, "import requests\n")
	if im.Kind != ImportAbsolute {
		t.Fatalf("kind = %v", im.Kind)
	}
	if len(im.Names) != 1 || im.Names[0].Name != "requests" {
		t.Fatalf("names = %+v", im.Names)
	}
}

func TestParseImportDottedBindsFirstSegment(t *testing.T) {
	im := parseOne(t, "import os.path\n")
	if got := im.Names[0].Binding(); got != "os" {
		t.Fatalf("binding = %q, want %q", got, "os")
	}
}

func TestParseImportAliasBindsAlias(t *testing.T) {
	im := parseOne(t, "import numpy.linalg as la\n")
	if got := im.Names[0].Binding(); got != "la" {
		t.Fatalf("binding = %q, want %q", got, "la")
	}
}

func TestParseImportMultipleNames(t *testing.T) {
	im := parseOne(t, "import json, re as regex\n")
	if len(im.Names) != 2 {
		t.Fatalf("names = %+v", im.Names)
	}
	if im.Names[0].Binding() != "json" || im.Names[1].Binding() != "regex" {
		t.Fatalf("bindings = %v", im.Bindings())
	}
}

func TestParseFromImport(t *testing.T) {
	im := parseOne(t, "from pydantic import BaseModel\n")
	if im.Kind != ImportFrom {
		t.Fatalf("kind = %v", im.Kind)
	}
	if im.Module != "pydantic" || im.Level != 0 {
		t.Fatalf("module = %q level = %d", im.Module, im.Level)
	}
}

func TestParseRelativeImportLevels(t *testing.T) {
	one := parseOne(t, "from .parse import f\n")
	if one.Kind != ImportRelative || one.Level != 1 || one.Module != "parse" {
		t.Fatalf("got kind=%v level=%d module=%q", one.Kind, one.Level, one.Module)
	}

	two := parseOne(t, "from .._shared.http import fetch\n")
	if two.Level != 2 || two.Module != "_shared.http" {
		t.Fatalf("level = %d module = %q", two.Level, two.Module)
	}
}

// `from . import mod` has no module part; the bundler rejects it, but the
// parser still has to represent it so the rejection can name it.
func TestParseFromDotImportHasEmptyModule(t *testing.T) {
	im := parseOne(t, "from . import parse\n")
	if im.Kind != ImportRelative || im.Level != 1 || im.Module != "" {
		t.Fatalf("got kind=%v level=%d module=%q", im.Kind, im.Level, im.Module)
	}
}

func TestParseStarImport(t *testing.T) {
	im := parseOne(t, "from .parse import *\n")
	if !im.Star {
		t.Fatal("star not detected")
	}
}

func TestParseFutureImport(t *testing.T) {
	im := parseOne(t, "from __future__ import annotations\n")
	if im.Kind != ImportFuture {
		t.Fatalf("kind = %v", im.Kind)
	}
}

func TestParseParenthesisedRelativeImport(t *testing.T) {
	im := parseOne(t, "from .parse import (\n    parse_rows,\n    clean as scrub,\n)\n")
	if len(im.Names) != 2 {
		t.Fatalf("names = %+v", im.Names)
	}
	if im.Names[0].Binding() != "parse_rows" || im.Names[1].Binding() != "scrub" {
		t.Fatalf("bindings = %v", im.Bindings())
	}
	if im.Names[1].Name != "clean" {
		t.Fatalf("aliased name = %q, want clean", im.Names[1].Name)
	}
}

func TestParseImportRejectsNonImports(t *testing.T) {
	for _, src := range []string{"x = 1\n", "def f():\n    pass\n", "important = 1\n"} {
		stmts := Scan(src)
		if _, ok := ParseImport(stmts[0]); ok {
			t.Fatalf("%q parsed as an import", src)
		}
	}
}

func bindingsOf(t *testing.T, src string) []string {
	t.Helper()
	stmts := Scan(src)
	if len(stmts) == 0 {
		t.Fatalf("no statements in %q", src)
	}
	return TopLevelBindings(stmts[0])
}

func TestTopLevelBindingsDefClass(t *testing.T) {
	if got := bindingsOf(t, "def clean(x):\n    return x\n"); !equal(got, []string{"clean"}) {
		t.Fatalf("got %q", got)
	}
	if got := bindingsOf(t, "async def fetch(x):\n    return x\n"); !equal(got, []string{"fetch"}) {
		t.Fatalf("got %q", got)
	}
	if got := bindingsOf(t, "class Response(BaseModel):\n    pass\n"); !equal(got, []string{"Response"}) {
		t.Fatalf("got %q", got)
	}
	if got := bindingsOf(t, "class Bare:\n    pass\n"); !equal(got, []string{"Bare"}) {
		t.Fatalf("got %q", got)
	}
}

func TestTopLevelBindingsAssignments(t *testing.T) {
	cases := []struct {
		src  string
		want []string
	}{
		{"TARGET = 1\n", []string{"TARGET"}},
		{"TARGET: str = \"x\"\n", []string{"TARGET"}},
		{"A, B = 1, 2\n", []string{"A", "B"}},
		{"A = B = 1\n", []string{"A", "B"}},
	}
	for _, tc := range cases {
		if got := bindingsOf(t, tc.src); !equal(got, tc.want) {
			t.Fatalf("%q: got %q, want %q", tc.src, got, tc.want)
		}
	}
}

// These bind nothing at module level. Counting them would produce phantom
// collisions between modules that merely mutate the same kind of object.
func TestTopLevelBindingsIgnoresNonBindingForms(t *testing.T) {
	for _, src := range []string{
		"obj.attr = 1\n",
		"items[0] = 1\n",
		"COUNT += 1\n",
		"if a == b:\n    pass\n",
		"print(\"x = 1\")\n",
	} {
		if got := bindingsOf(t, src); len(got) != 0 {
			t.Fatalf("%q bound %q, want nothing", src, got)
		}
	}
}

// A default argument containing '=' must not be read as an assignment target.
func TestTopLevelBindingsIgnoresEqualsInsideBrackets(t *testing.T) {
	if got := bindingsOf(t, "CONFIG = dict(a=1, b=2)\n"); !equal(got, []string{"CONFIG"}) {
		t.Fatalf("got %q", got)
	}
}

func TestTopLevelBindingsFutureImportBindsNothing(t *testing.T) {
	if got := bindingsOf(t, "from __future__ import annotations\n"); len(got) != 0 {
		t.Fatalf("got %q", got)
	}
}
