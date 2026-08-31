package main

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

const testSpec = `{
  "paths": {
    "/functions": {"post": {"operationId": "function_create"}},
    "/functions/{function_id}": {
      "post":  {"operationId": "function_update"},
      "patch": {"operationId": "function_metadata_update"}
    },
    "/mailboxes": {"get": {"operationId": "mailbox_list"}}
  }
}`

func endpointsFor(t *testing.T) []Endpoint {
	t.Helper()
	endpoints, err := parseEndpoints([]byte(testSpec))
	if err != nil {
		t.Fatalf("parsing spec: %v", err)
	}
	return endpoints
}

// A path carrying two methods is two endpoints. Collapsing them is how
// `functions update` (POST) and the metadata patch (PATCH) were treated as one.
func TestParseEndpointsKeepsMethodsApart(t *testing.T) {
	endpoints := endpointsFor(t)
	if len(endpoints) != 4 {
		t.Fatalf("got %d endpoints, want 4: %v", len(endpoints), endpoints)
	}
	var patch, post bool
	for _, e := range endpoints {
		if e.Path == "/functions/{function_id}" && e.Method == "PATCH" {
			patch = true
		}
		if e.Path == "/functions/{function_id}" && e.Method == "POST" {
			post = true
		}
	}
	if !patch || !post {
		t.Errorf("both methods on /functions/{function_id} should appear, got %v", endpoints)
	}
}

// The failure this whole check exists for: the API grows an endpoint and
// nothing in the repository notices.
func TestUnwiredEndpointIsReported(t *testing.T) {
	source := `FunctionCreateWithBodyWithResponse(ctx)
	FunctionUpdateWithBodyWithResponse(ctx)
	FunctionMetadataUpdateWithResponse(ctx)`

	problems := checkEndpoints(endpointsFor(t), source, nil)
	if len(problems) != 1 {
		t.Fatalf("got %d problems, want 1: %v", len(problems), problems)
	}
	if !strings.Contains(problems[0], "GET /mailboxes") {
		t.Errorf("expected the uncalled endpoint to be named, got %q", problems[0])
	}
	if !strings.Contains(problems[0], "endpoint-coverage.txt") {
		t.Error("the message should say how to record the decision")
	}
}

func TestRecordedExceptionSilencesTheEndpoint(t *testing.T) {
	source := `FunctionCreateWithBodyWithResponse(ctx)
	FunctionUpdateWithBodyWithResponse(ctx)
	FunctionMetadataUpdateWithResponse(ctx)`
	exceptions := []Exception{{Verdict: VerdictSkip, Method: "GET", Path: "/mailboxes", Reason: "console only", Line: 3}}

	if problems := checkEndpoints(endpointsFor(t), source, exceptions); len(problems) != 0 {
		t.Errorf("a recorded skip should pass, got %v", problems)
	}
}

// The other half of the ratchet: an exception whose endpoint the API renamed
// has to be noticed, or the file rots the way the flag generator's map did.
func TestExceptionForAVanishedEndpointIsReported(t *testing.T) {
	source := `FunctionCreateWithBodyWithResponse(ctx)
	FunctionUpdateWithBodyWithResponse(ctx)
	FunctionMetadataUpdateWithResponse(ctx)`
	exceptions := []Exception{
		{Verdict: VerdictSkip, Method: "GET", Path: "/mailboxes", Reason: "console only", Line: 3},
		{Verdict: VerdictSkip, Method: "POST", Path: "/functions/schedule", Reason: "stale", Line: 4},
	}

	problems := checkEndpoints(endpointsFor(t), source, exceptions)
	if len(problems) != 1 {
		t.Fatalf("got %d problems, want 1: %v", len(problems), problems)
	}
	if !strings.Contains(problems[0], "/functions/schedule") {
		t.Errorf("expected the stale line to be named, got %q", problems[0])
	}
}

// A skip that stops being true should be deleted, not left asserting something
// false about the CLI.
func TestSkipContradictedByACallIsReported(t *testing.T) {
	source := `MailboxListWithResponse(ctx)
	FunctionCreateWithBodyWithResponse(ctx)
	FunctionUpdateWithBodyWithResponse(ctx)
	FunctionMetadataUpdateWithResponse(ctx)`
	exceptions := []Exception{{Verdict: VerdictSkip, Method: "GET", Path: "/mailboxes", Reason: "console only", Line: 3}}

	problems := checkEndpoints(endpointsFor(t), source, exceptions)
	if len(problems) != 1 || !strings.Contains(problems[0], "drop the line") {
		t.Fatalf("expected a contradiction to be reported, got %v", problems)
	}
}

func TestManifestRequiresAReason(t *testing.T) {
	if _, err := parseManifest([]byte("skip GET /mailboxes\n"), "test.txt"); err == nil {
		t.Fatal("an exception without a reason should be rejected")
	}
	if _, err := parseManifest([]byte("maybe GET /mailboxes # hm\n"), "test.txt"); err == nil {
		t.Fatal("an unknown verdict should be rejected")
	}
	if _, err := parseManifest([]byte("skip GET /a # one\nskip GET /a # two\n"), "test.txt"); err == nil {
		t.Fatal("a duplicate entry should be rejected")
	}

	exceptions, err := parseManifest([]byte("# comment\n\nskip GET /mailboxes # console only\n"), "test.txt")
	if err != nil {
		t.Fatalf("valid manifest rejected: %v", err)
	}
	if len(exceptions) != 1 || exceptions[0].Reason != "console only" || exceptions[0].Line != 3 {
		t.Errorf("unexpected parse: %+v", exceptions)
	}
}

func TestOperationGoName(t *testing.T) {
	for operation, want := range map[string]string{
		"function_create":      "FunctionCreate",
		"_connect_link_status": "ConnectLinkStatus",
		"ready_check":          "ReadyCheck",
	} {
		if got := operationGoName(operation); got != want {
			t.Errorf("operationGoName(%q) = %q, want %q", operation, got, want)
		}
	}
}

func testTree() *cobra.Command {
	root := &cobra.Command{Use: "notte"}
	functions := &cobra.Command{Use: "functions"}
	functions.AddCommand(&cobra.Command{Use: "rollback"})
	functions.AddCommand(&cobra.Command{Use: "run-metadata-update", Hidden: true})
	root.AddCommand(functions)
	root.AddCommand(&cobra.Command{Use: "search"})
	return root
}

// Leaves, not groups: `notte functions` is not something to document, and a
// hidden command is deliberately undocumented, so requiring prose for it would
// contradict the reason it is hidden.
func TestLeafCommandsSkipsGroupsAndHiddenCommands(t *testing.T) {
	got := leafCommands(testTree())
	want := []string{"notte functions rollback", "notte search"}

	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestUndocumentedCommandIsReported(t *testing.T) {
	files := map[string]string{
		"plugins/notte/skills/notte-functions-build/SKILL.md": "Run `notte search` to find things.",
	}

	problems := checkSkills(leafCommands(testTree()), files)
	if len(problems) != 1 {
		t.Fatalf("got %d problems, want 1: %v", len(problems), problems)
	}
	if !strings.Contains(problems[0], "notte functions rollback") {
		t.Errorf("expected the undocumented command to be named, got %q", problems[0])
	}
}

// A mention anywhere counts, in any file and across line breaks: which skill
// documents which command is an editorial decision, not this check's business.
func TestMentionIsFoundInAnyFileAndAcrossWhitespace(t *testing.T) {
	files := map[string]string{
		"plugins/notte/skills/notte-browser/references/function-management.md": "notte search",
		"plugins/notte/skills/notte-functions-doctor/SKILL.md":                 "notte functions\n  rollback --version v1",
	}

	if problems := checkSkills(leafCommands(testTree()), files); len(problems) != 0 {
		t.Errorf("both commands are documented, got %v", problems)
	}
}

// "notte search" must not be satisfied by "notte searching", which is what a
// plain substring match would accept.
func TestMentionDoesNotMatchALongerWord(t *testing.T) {
	files := map[string]string{"a.md": "notte searching the web"}

	if mentions(files, "notte search") {
		t.Error("a prefix of a longer word should not count as a mention")
	}
}
