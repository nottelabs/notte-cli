// Command checkcoverage guards the two ways the CLI drifts out of step with
// everything around it.
//
//	endpoints  every operation the API serves is either reachable from a
//	           command or recorded as a deliberate omission
//	skills     every command a user can run is mentioned in notte-skills
//
// Both read the live source of truth rather than a checked-in copy, so both
// need the network. Without -strict a fetch failure is a warning and the check
// passes: a pre-commit hook has to work on a train. CI runs with -strict, which
// turns the same failure into an error.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nottelabs/notte-cli/internal/cmd"
)

const (
	defaultSpecURL  = "https://us-staging.notte.cc/openapi.json"
	defaultManifest = "scripts/endpoint-coverage.txt"
	commandDir      = "internal/cmd"
	fetchTimeout    = 30 * time.Second
)

func main() {
	var (
		which      = flag.String("check", "all", "which check to run: endpoints, skills, or all")
		specURL    = flag.String("spec", envOr("NOTTE_API_URL", "")+"/openapi.json", "OpenAPI spec URL")
		manifest   = flag.String("manifest", defaultManifest, "path to the endpoint coverage manifest")
		skillsDir  = flag.String("skills-dir", "", "read notte-skills from this directory instead of GitHub")
		strict     = flag.Bool("strict", false, "treat an unreachable source of truth as a failure")
		repoRoot   = flag.String("root", "", "repository root (defaults to the working directory)")
		exitStatus = 0
	)
	flag.Parse()

	if *specURL == "/openapi.json" {
		*specURL = defaultSpecURL
	}
	root := *repoRoot
	if root == "" {
		wd, err := os.Getwd()
		if err != nil {
			fail("cannot determine the working directory: %v", err)
		}
		root = wd
	}

	// A misspelled -check used to run nothing and exit 0, which is the worst
	// answer a guard can give: it reports success without having looked.
	switch *which {
	case "all", "endpoints", "skills":
	default:
		fail("unknown -check %q: want endpoints, skills, or all", *which)
	}

	if *which == "all" || *which == "endpoints" {
		if !runEndpointCheck(root, *specURL, *manifest, *strict) {
			exitStatus = 1
		}
	}
	if *which == "all" || *which == "skills" {
		if !runSkillCheck(*skillsDir, *strict) {
			exitStatus = 1
		}
	}

	os.Exit(exitStatus)
}

func runEndpointCheck(root, specURL, manifest string, strict bool) bool {
	data, err := fetchSpec(specURL, fetchTimeout)
	if err != nil {
		return degrade(strict, "endpoint coverage", err)
	}

	endpoints, err := parseEndpoints(data)
	if err != nil {
		fail("%v", err)
	}

	source, err := readCommandSources(filepath.Join(root, commandDir))
	if err != nil {
		fail("%v", err)
	}

	manifestPath := filepath.Join(root, manifest)
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		fail("reading %s: %v", manifest, err)
	}
	exceptions, err := parseManifest(manifestData, manifest)
	if err != nil {
		fail("%v", err)
	}

	problems := checkEndpoints(endpoints, source, exceptions)
	return report("endpoint coverage",
		fmt.Sprintf("%d endpoints, %d recorded exceptions", len(endpoints), len(exceptions)),
		problems)
}

func runSkillCheck(skillsDir string, strict bool) bool {
	var (
		files map[string]string
		err   error
	)
	if skillsDir != "" {
		files, err = readLocalSkills(skillsDir)
	} else {
		files, err = fetchSkills(skillsTarball, fetchTimeout)
	}
	if err != nil {
		return degrade(strict, "skill coverage", err)
	}

	commands := leafCommands(cmd.RootCommand())
	problems := checkSkills(commands, files)
	return report("skill coverage",
		fmt.Sprintf("%d commands against %d skill files", len(commands), len(files)),
		problems)
}

// degrade decides what an unreachable source of truth means. Offline, a
// pre-commit hook that blocks the commit is worse than one that says so.
func degrade(strict bool, name string, err error) bool {
	if strict {
		fmt.Fprintf(os.Stderr, "✗ %s: %v\n", name, err)
		return false
	}
	fmt.Fprintf(os.Stderr, "⚠ %s skipped: %v\n", name, err)
	return true
}

func report(name, summary string, problems []string) bool {
	if len(problems) == 0 {
		fmt.Printf("✓ %s: %s\n", name, summary)
		return true
	}
	fmt.Fprintf(os.Stderr, "\n✗ %s: %d problem(s) across %s\n\n", name, len(problems), summary)
	for _, p := range problems {
		fmt.Fprintf(os.Stderr, "  · %s\n", p)
	}
	fmt.Fprintln(os.Stderr)
	return false
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(2)
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
