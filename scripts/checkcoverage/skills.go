package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	// skillsTarball is the notte-skills repository, which documents the CLI for
	// agents. A tarball rather than a clone: one request, no git, no checkout to
	// keep in sync.
	skillsTarball = "https://codeload.github.com/nottelabs/notte-skills/tar.gz/refs/heads/main"
	// skillsRoot is the directory inside that repository holding the skills.
	skillsRoot = "plugins/notte/skills/"
)

// commandPath renders a command the way someone writes it in prose:
// "notte functions rollback".
func commandPath(c *cobra.Command) string {
	var parts []string
	for cur := c; cur != nil; cur = cur.Parent() {
		parts = append([]string{cur.Name()}, parts...)
	}
	return strings.Join(parts, " ")
}

// leafCommands lists every command a user can actually run.
//
// Hidden commands are left out: they are deliberately undocumented, so
// requiring documentation for them would be a contradiction. So are cobra's own
// help and completion, which belong to cobra rather than to this CLI.
func leafCommands(root *cobra.Command) []string {
	var leaves []string

	var walk func(*cobra.Command)
	walk = func(c *cobra.Command) {
		if c.Hidden || c.Name() == "help" || c.Name() == "completion" {
			return
		}
		children := c.Commands()
		runnable := false
		for _, child := range children {
			if !child.Hidden && child.Name() != "help" && child.Name() != "completion" {
				runnable = true
				walk(child)
			}
		}
		if !runnable && c != root {
			leaves = append(leaves, commandPath(c))
		}
	}
	walk(root)

	sort.Strings(leaves)
	return leaves
}

// fetchSkills downloads the skills repository and returns every documentation
// file under skillsRoot, keyed by path.
func fetchSkills(url string, timeout time.Duration) (map[string]string, error) {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching %s: %s", url, resp.Status)
	}

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("decompressing skills archive: %w", err)
	}
	defer func() { _ = gz.Close() }()

	files := map[string]string{}
	reader := tar.NewReader(gz)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading skills archive: %w", err)
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		// Drop the "notte-skills-main/" prefix the archive wraps everything in.
		name := header.Name
		if i := strings.Index(name, "/"); i >= 0 {
			name = name[i+1:]
		}
		if !strings.HasPrefix(name, skillsRoot) {
			continue
		}
		content, err := io.ReadAll(reader)
		if err != nil {
			return nil, fmt.Errorf("reading %s from skills archive: %w", name, err)
		}
		files[name] = string(content)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no files under %s in the skills archive: has the layout moved?", skillsRoot)
	}
	return files, nil
}

// readLocalSkills reads the same files from a directory, for running the check
// against a working copy of notte-skills instead of its main branch.
func readLocalSkills(dir string) (map[string]string, error) {
	files := map[string]string{}
	root := filepath.Join(dir, filepath.FromSlash(skillsRoot))
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return relErr
		}
		files[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("reading skills from %s: %w", dir, err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no skill files under %s", root)
	}
	return files, nil
}

// mentions reports whether the skills document a command anywhere.
//
// The match is on the command path with flexible whitespace, so a line broken
// across a continuation still counts, and it deliberately does not require the
// mention to be in any particular file: which skill covers which command is an
// editorial decision, and this check only cares that one of them does.
func mentions(files map[string]string, command string) bool {
	pattern := regexp.MustCompile(`\b` + strings.Join(splitEscaped(command), `\s+`) + `\b`)
	for _, content := range files {
		if pattern.MatchString(content) {
			return true
		}
	}
	return false
}

func splitEscaped(command string) []string {
	parts := strings.Fields(command)
	for i, p := range parts {
		parts[i] = regexp.QuoteMeta(p)
	}
	return parts
}

// checkSkills reports every command the skills never mention.
func checkSkills(commands []string, files map[string]string) []string {
	var problems []string
	for _, command := range commands {
		if !mentions(files, command) {
			problems = append(problems, fmt.Sprintf(
				"`%s` is not mentioned anywhere in %s.\n"+
					"      Document it in the notte-skills repository, or hide the command if it is not for users.",
				command, skillsRoot))
		}
	}
	return problems
}
