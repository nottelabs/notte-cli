package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
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

// A read can end inside an escape sequence or carry several key presses; the
// menu must see whole keys either way.
func TestKeyStream(t *testing.T) {
	chunks := func(parts ...string) io.Reader {
		pr, pw := io.Pipe()
		go func() {
			for _, p := range parts {
				_, _ = pw.Write([]byte(p))
			}
			_ = pw.Close()
		}()
		return pr
	}
	for _, tc := range []struct {
		name  string
		input []string
		ready bool // whether more input is waiting after a partial sequence
		want  []string
	}{
		{"several keys in one read", []string{"jj\r"}, false, []string{"j", "j", "\r"}},
		{"arrow with a key after it", []string{"\x1b[Bk"}, false, []string{"\x1b[B", "k"}},
		{"arrow split across reads", []string{"\x1b", "[A"}, true, []string{"\x1b[A"}},
		{"arrow split inside the sequence", []string{"\x1b[", "B"}, true, []string{"\x1b[B"}},
		{"application-mode arrow", []string{"\x1bOA"}, false, []string{"\x1bOA"}},
		{"lone escape", []string{"\x1b"}, false, []string{"\x1b"}},
		{"escape then a key", []string{"\x1bq"}, false, []string{"\x1b", "q"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			keys := &keyStream{in: chunks(tc.input...), ready: func(time.Duration) bool { return tc.ready }}
			var got []string
			for range tc.want {
				key, err := keys.next()
				if err != nil {
					t.Fatalf("after %q: %v", got, err)
				}
				got = append(got, string(key))
			}
			if strings.Join(got, "|") != strings.Join(tc.want, "|") {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// Sessions on later pages must be found, and a prefix is only unique when no
// later page holds another match.
func TestRunSessionsPin_SearchesEveryPage(t *testing.T) {
	setupSessionFileTest(t)
	pages := map[string]string{
		"1": `{"items":[{"session_id":"aa1-first","status":"active"}],"has_next":true}`,
		"2": `{"items":[{"session_id":"aa2-second","status":"active"},{"session_id":"zz9-late","status":"active"}],"has_next":false}`,
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, pages[r.URL.Query().Get("page")])
	}))
	defer server.Close()
	env := testutil.SetupTestEnv(t)
	env.SetEnv("NOTTE_API_KEY", "test-key")
	env.SetEnv("NOTTE_API_URL", server.URL)
	origFormat := outputFormat
	outputFormat = "text"
	t.Cleanup(func() { outputFormat = origFormat })
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())

	_, _ = testutil.CaptureOutput(func() {
		if err := runSessionsPin(cmd, []string{"zz9"}); err != nil {
			t.Fatalf("a session on page 2 was not found: %v", err)
		}
	})
	if got := GetCurrentSessionID(); got != "zz9-late" {
		t.Errorf("pinned %q, want zz9-late", got)
	}
	var err error
	_, _ = testutil.CaptureOutput(func() { err = runSessionsPin(cmd, []string{"aa"}) })
	if err == nil || !strings.Contains(err.Error(), "matches 2 sessions") {
		t.Errorf("a prefix matching pages 1 and 2 must be ambiguous, got %v", err)
	}
}
