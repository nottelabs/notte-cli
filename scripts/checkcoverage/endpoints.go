package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Endpoint is one operation in the OpenAPI spec.
type Endpoint struct {
	Method      string
	Path        string
	OperationID string
}

func (e Endpoint) String() string { return fmt.Sprintf("%s %s", e.Method, e.Path) }

type spec struct {
	Paths map[string]map[string]struct {
		OperationID string `json:"operationId"`
	} `json:"paths"`
}

var httpMethods = map[string]bool{
	"get": true, "post": true, "put": true, "patch": true, "delete": true,
}

// parseEndpoints reads every operation out of an OpenAPI document.
func parseEndpoints(data []byte) ([]Endpoint, error) {
	var s spec
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing OpenAPI spec: %w", err)
	}

	var endpoints []Endpoint
	for path, item := range s.Paths {
		for method, op := range item {
			if !httpMethods[strings.ToLower(method)] {
				continue
			}
			if op.OperationID == "" {
				return nil, fmt.Errorf("%s %s has no operationId, which this check keys on",
					strings.ToUpper(method), path)
			}
			endpoints = append(endpoints, Endpoint{
				Method:      strings.ToUpper(method),
				Path:        path,
				OperationID: op.OperationID,
			})
		}
	}
	sort.Slice(endpoints, func(i, j int) bool {
		if endpoints[i].Path != endpoints[j].Path {
			return endpoints[i].Path < endpoints[j].Path
		}
		return endpoints[i].Method < endpoints[j].Method
	})
	return endpoints, nil
}

// operationGoName converts an operationId to the identifier oapi-codegen gives
// its client method: connect_link_status -> ConnectLinkStatus. Leading
// underscores mark the API's private operations and survive as nothing.
func operationGoName(operationID string) string {
	var b strings.Builder
	for _, word := range strings.Split(operationID, "_") {
		if word == "" {
			continue
		}
		b.WriteString(strings.ToUpper(word[:1]))
		b.WriteString(word[1:])
	}
	return b.String()
}

// readCommandSources concatenates the hand-written command implementations.
// Generated flag files and tests are left out: a call in either proves nothing
// about what the CLI actually offers.
func readCommandSources(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", dir, err)
	}

	var b strings.Builder
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}
		if strings.HasSuffix(name, "_test.go") || strings.HasSuffix(name, ".gen.go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return "", fmt.Errorf("reading %s: %w", name, err)
		}
		b.Write(data)
		b.WriteString("\n")
	}
	return b.String(), nil
}

// callsGeneratedClient reports whether a command invokes the generated client
// method for an operation. Both spellings count: multipart endpoints are called
// through the WithBody variant.
func callsGeneratedClient(source string, e Endpoint) bool {
	name := operationGoName(e.OperationID)
	return strings.Contains(source, name+"WithResponse(") ||
		strings.Contains(source, name+"WithBodyWithResponse(")
}

// Verdict is how an endpoint that no command calls is accounted for.
type Verdict string

const (
	// VerdictManual: a command reaches it without the generated client, by
	// building the request itself.
	VerdictManual Verdict = "manual"
	// VerdictSkip: deliberately not exposed by the CLI.
	VerdictSkip Verdict = "skip"
)

// Exception is one line of the coverage manifest.
type Exception struct {
	Verdict Verdict
	Method  string
	Path    string
	Reason  string
	Line    int
}

func (e Exception) key() string { return e.Method + " " + e.Path }

// parseManifest reads scripts/endpoint-coverage.txt. Each line is
//
//	<verdict> <METHOD> <path> # <reason>
//
// The reason is required: an unexplained exception is how a gap becomes
// permanent without anybody deciding that it should.
func parseManifest(data []byte, filename string) ([]Exception, error) {
	var exceptions []Exception
	seen := map[string]int{}

	for i, raw := range strings.Split(string(data), "\n") {
		lineNo := i + 1
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		body, reason, hasReason := strings.Cut(line, "#")
		fields := strings.Fields(body)
		if len(fields) != 3 {
			return nil, fmt.Errorf("%s:%d: expected `<verdict> <METHOD> <path> # reason`, got %q",
				filename, lineNo, line)
		}
		verdict := Verdict(fields[0])
		if verdict != VerdictManual && verdict != VerdictSkip {
			return nil, fmt.Errorf("%s:%d: unknown verdict %q, want %q or %q",
				filename, lineNo, fields[0], VerdictManual, VerdictSkip)
		}
		reason = strings.TrimSpace(reason)
		if !hasReason || reason == "" {
			return nil, fmt.Errorf("%s:%d: %s %s has no reason after `#`",
				filename, lineNo, fields[1], fields[2])
		}

		e := Exception{
			Verdict: verdict,
			Method:  strings.ToUpper(fields[1]),
			Path:    fields[2],
			Reason:  reason,
			Line:    lineNo,
		}
		if first, dup := seen[e.key()]; dup {
			return nil, fmt.Errorf("%s:%d: %s is already listed on line %d",
				filename, lineNo, e.key(), first)
		}
		seen[e.key()] = lineNo
		exceptions = append(exceptions, e)
	}
	return exceptions, nil
}

// checkEndpoints compares the live API against the CLI and its manifest.
//
// It reports three failures, and the second is the one this exists for: an
// endpoint the API grew that nobody has decided about. The map of generated
// flags could only ever catch entries that had gone stale, never an endpoint
// that was never wired at all.
func checkEndpoints(endpoints []Endpoint, source string, exceptions []Exception) []string {
	byKey := map[string]Exception{}
	for _, e := range exceptions {
		byKey[e.key()] = e
	}

	var problems []string
	live := map[string]bool{}

	for _, e := range endpoints {
		live[e.String()] = true
		exception, listed := byKey[e.String()]
		covered := callsGeneratedClient(source, e)

		switch {
		case covered && listed && exception.Verdict == VerdictSkip:
			problems = append(problems, fmt.Sprintf(
				"%s is listed as `skip` on line %d but a command now calls it — drop the line",
				e, exception.Line))
		case !covered && !listed:
			problems = append(problems, fmt.Sprintf(
				"%s (%s) is on the API but no CLI command calls it.\n"+
					"      Add a command, or record the decision in scripts/endpoint-coverage.txt:\n"+
					"        skip %s %s # why the CLI does not expose it",
				e, e.OperationID, e.Method, e.Path))
		}
	}

	for _, exception := range exceptions {
		if !live[exception.key()] {
			problems = append(problems, fmt.Sprintf(
				"%s is listed on line %d but the API no longer serves it — the path was "+
					"probably renamed, so remove or update the line",
				exception.key(), exception.Line))
		}
	}

	sort.Strings(problems)
	return problems
}

// fetchSpec downloads an OpenAPI document.
func fetchSpec(url string, timeout time.Duration) ([]byte, error) {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching %s: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}
