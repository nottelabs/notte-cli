package cmd

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/nottelabs/notte-cli/internal/api"
	"github.com/nottelabs/notte-cli/internal/testutil"
)

func pinSessions() []api.SessionResponse {
	return []api.SessionResponse{
		{SessionId: "aaa111-one", Status: "active"},
		{SessionId: "aaa222-two", Status: "active"},
		{SessionId: "bbb333-three", Status: "active"},
	}
}

func TestMatchSession(t *testing.T) {
	sessions := pinSessions()
	for query, want := range map[string]string{
		"aaa111-one": "aaa111-one",
		"bbb":        "bbb333-three",
		"aaa2":       "aaa222-two",
	} {
		got, err := matchSession(sessions, query)
		if err != nil || got.SessionId != want {
			t.Errorf("matchSession(%q) = %v, %v; want %s", query, got, err, want)
		}
	}
	if _, err := matchSession(sessions, "aaa"); err == nil || !strings.Contains(err.Error(), "matches 2 sessions") {
		t.Errorf("ambiguous prefix: got %v", err)
	}
	if _, err := matchSession(sessions, "zzz"); err == nil || !strings.Contains(err.Error(), "no active session matches") {
		t.Errorf("unknown ID: got %v", err)
	}
}

func TestPickSession(t *testing.T) {
	sessions := pinSessions()
	var out bytes.Buffer
	// A bad answer asks again; a number then picks.
	got, err := pickSession(strings.NewReader("9\n2\n"), &out, sessions, "aaa111-one", time.Now())
	if err != nil || got.SessionId != "aaa222-two" {
		t.Fatalf("got %v, %v", got, err)
	}
	for _, want := range []string{"BROWSER SESSION", "aaa111-one", "pinned", "Pin which session? [1-3]: ", "Type a number from 1 to 3."} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("menu is missing %q:\n%s", want, out.String())
		}
	}
	// An ID prefix works as an answer too.
	if got, err := pickSession(strings.NewReader("bbb\n"), &bytes.Buffer{}, sessions, "", time.Now()); err != nil || got.SessionId != "bbb333-three" {
		t.Errorf("prefix answer: got %v, %v", got, err)
	}
	// Without input (piped, or Ctrl+D) it explains how to pin directly.
	if _, err := pickSession(strings.NewReader(""), &bytes.Buffer{}, sessions, "", time.Now()); err == nil || !strings.Contains(err.Error(), "notte sessions pin <session-id>") {
		t.Errorf("no input: got %v", err)
	}
}

func TestRunSessionsPin(t *testing.T) {
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")
	setupSessionFileTest(t)

	server := testutil.NewMockServer()
	defer server.Close()
	env.SetEnv("NOTTE_API_URL", server.URL())
	server.AddResponse("/sessions", 200, `{"items": [
		{"session_id": "aaa111-one", "status": "active", "created_at": "2026-01-15T10:00:00Z", "last_accessed_at": "2026-01-15T10:00:00Z", "idle_timeout_minutes": 3},
		{"session_id": "bbb333-three", "status": "active", "created_at": "2026-01-15T10:00:00Z", "last_accessed_at": "2026-01-15T10:00:00Z", "idle_timeout_minutes": 3}]}`)

	origFormat := outputFormat
	outputFormat = "text"
	t.Cleanup(func() { outputFormat = origFormat })
	if err := setCurrentSession("aaa111-one"); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	stdout, _ := testutil.CaptureOutput(func() {
		if err := runSessionsPin(cmd, []string{"bbb"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if stdout != "Pinned session bbb333-three\nNext: notte sessions status\n" {
		t.Errorf("unexpected output %q", stdout)
	}
	if current := GetCurrentSessionID(); current != "bbb333-three" {
		t.Errorf("current session is %q, want bbb333-three", current)
	}
	if !strings.Contains(server.Requests("/sessions")[0].Query, "only_active=true") {
		t.Errorf("listed sessions without only_active: %q", server.Requests("/sessions")[0].Query)
	}
}

func TestPinMenuKey(t *testing.T) {
	for _, tc := range []struct {
		key          string
		cursor, want int
		done, cancel bool
	}{
		{"\x1b[B", 0, 1, false, false},
		{"j", 2, 0, false, false}, // wraps around
		{"\x1b[A", 0, 2, false, false},
		{"k", 1, 0, false, false},
		{"3", 0, 2, false, false},
		{"9", 1, 1, false, false}, // no such row
		{"\r", 1, 1, true, false},
		{"\x1b", 1, 1, false, true},
		{"q", 1, 1, false, true},
		{"\x03", 1, 1, false, true},
	} {
		next, done, cancel := pinMenuKey(tc.cursor, 3, []byte(tc.key))
		if next != tc.want || done != tc.done || cancel != tc.cancel {
			t.Errorf("key %q at %d: got (%d, %v, %v)", tc.key, tc.cursor, next, done, cancel)
		}
	}
}

func TestRenderPinMenu(t *testing.T) {
	menu := renderPinMenu(pinSessions(), 1, "aaa111-one", time.Now(), true)
	lines := strings.Split(strings.TrimSuffix(menu, "\r\n"), "\r\n")
	if len(lines) != 4 {
		t.Fatalf("want a title and 3 rows, got %q", menu)
	}
	if !strings.Contains(lines[1], "aaa111-one") || !strings.Contains(lines[1], "pinned") || strings.Contains(lines[1], "❯") {
		t.Errorf("row 1: %q", lines[1])
	}
	if !strings.Contains(lines[2], "❯ aaa222-two") {
		t.Errorf("the cursor row should be marked: %q", lines[2])
	}
}
