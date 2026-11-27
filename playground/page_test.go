package main

import (
	"os/exec"
	"testing"
)

// TestPageShell runs the page's own checks (page_test.mjs), which load the
// script of web/index.html in Node and drive its shell.
func TestPageShell(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed; the page checks need it")
	}
	output, err := exec.Command(node, "page_test.mjs").CombinedOutput()
	if err != nil {
		t.Fatalf("page checks failed: %v\n%s", err, output)
	}
}
