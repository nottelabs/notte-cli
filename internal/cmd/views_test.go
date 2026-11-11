package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/nottelabs/notte-cli/internal/api"
	"github.com/nottelabs/notte-cli/internal/output"
	"github.com/nottelabs/notte-cli/internal/testutil"
)

var viewNow = time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)

func plainText() (*output.TextFormatter, *bytes.Buffer) {
	var buf bytes.Buffer
	return &output.TextFormatter{Writer: &buf, NoColor: true}, &buf
}

func at(hoursAgo int) api.FlexibleTime {
	return api.FlexibleTime{Time: viewNow.Add(-time.Duration(hoursAgo) * time.Hour)}
}

func assertLines(t *testing.T, got string, want ...string) {
	t.Helper()
	if got != strings.Join(want, "\n")+"\n" {
		t.Errorf("got:\n%s\nwant:\n%s", got, strings.Join(want, "\n"))
	}
}

func TestViewFunctions(t *testing.T) {
	testutil.SetupTestEnv(t)
	t.Setenv("FORCE_HYPERLINK", "0")
	t.Setenv("NOTTE_CONSOLE_URL", "https://console.test")
	text, buf := plainText()
	name := "Price tracker"
	err := viewFunctions(text, []api.FunctionListItemResponse{
		{FunctionId: "fn-1", Name: &name, Status: "active", LatestVersion: "3", UpdatedAt: at(2)},
		{FunctionId: "fn-2", Status: "active", LatestVersion: "1", UpdatedAt: at(0)},
	}, viewNow)
	if err != nil {
		t.Fatal(err)
	}
	assertLines(t, buf.String(),
		"    FUNCTION (2)   NAME            STATUS   VERSION   UPDATED    CONSOLE",
		"    fn-1           Price tracker   active   v3        2 h ago    https://console.test/functions/fn-1",
		"    fn-2           -               active   v1        just now   https://console.test/functions/fn-2",
		"Next: notte functions run --function-id <id>",
	)
}

// A run prints its result whole: reading it is the reason to run a function.
func TestViewFunctionRun(t *testing.T) {
	testutil.SetupTestEnv(t)
	t.Setenv("FORCE_HYPERLINK", "0")
	t.Setenv("NOTTE_CONSOLE_URL", "https://console.test")
	text, buf := plainText()
	result, version, session := `{"price": 18, "name": "Mug"}`, "3", "s-1"
	run := &api.GetFunctionRunResponse{
		FunctionId: "fn-1", FunctionRunId: "run-1", Status: api.GetFunctionRunResponseStatusClosed,
		FunctionVersion: &version, SessionId: &session, Result: &result, CreatedAt: at(1),
		UpdatedAt: api.FlexibleTime{Time: at(1).Add(42 * time.Second)},
	}
	if err := viewFunctionRun(text, run, viewNow); err != nil {
		t.Fatal(err)
	}
	assertLines(t, buf.String(),
		"Run run-1",
		"  Function   fn-1",
		"  Status     finished",
		"  Version    v3",
		"  Session    s-1",
		"  Took       42.0s",
		"  Created    1 h ago",
		"  Console    https://console.test/functions/fn-1/runs/run-1",
		"",
		"Result",
		"{",
		`  "name": "Mug",`,
		`  "price": 18`,
		"}",
		"",
		"Next: notte functions runs --function-id fn-1",
	)
}

func TestViewUsage(t *testing.T) {
	testutil.SetupTestEnv(t)
	t.Setenv("FORCE_HYPERLINK", "0")
	t.Setenv("NOTTE_CONSOLE_URL", "https://console.test")
	text, buf := plainText()
	err := viewUsage(text, &api.UsageResponse{
		Period: "January 2026", BalanceAmount: 41.19, SubscriptionBalanceAmount: 31.19, SubscriptionLimitAmount: 40,
		TopupBalanceAmount: 10, TopupLimitAmount: 10, TotalCost: 8.81, BrowserUsageCost: 4.82, SessionCount: 1,
		FunctionUsageCost: 1.2, FunctionCount: 6, LlmUsageCost: 2.15, ProxyUsageCost: 0.64,
	})
	if err != nil {
		t.Fatal(err)
	}
	assertLines(t, buf.String(),
		"Usage for January 2026",
		"  Balance          $41.19",
		"    subscription   $31.19 of $40.00",
		"    top-up         $10.00",
		"  Spent            $8.81",
		"    browsers       $4.82  1 session",
		"    functions      $1.20  6 runs",
		"    LLM            $2.15",
		"    proxies        $0.64",
		"  Console          https://console.test/billing",
		"Next: notte usage logs",
	)
}

// Messages are for reading codes and links out of, so nothing is cut.
func TestViewEmailsAndSMS(t *testing.T) {
	text, buf := plainText()
	sender, name, body := "no-reply@example.com", "Shop", "Your code is 204 815.\nIt expires soon."
	if err := viewEmails(text, []api.EmailResponse{{EmailId: "e-1", Subject: "Confirm", SenderEmail: &sender, SenderName: &name, TextContent: &body, CreatedAt: at(3)}}, viewNow); err != nil {
		t.Fatal(err)
	}
	assertLines(t, buf.String(),
		"Confirm",
		"  Email     e-1",
		"  From      Shop <no-reply@example.com>",
		"  Created   3 h ago",
		"",
		"  Your code is 204 815.",
		"  It expires soon.",
	)

	text, buf = plainText()
	long := strings.Repeat("code 481920 ", 12)
	if err := viewSMS(text, []api.SMSResponse{{SmsId: "sms-1", Body: long, CreatedAt: at(0)}}, viewNow); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), strings.TrimSpace(long)) {
		t.Errorf("the message was cut:\n%s", buf.String())
	}
}

func TestViewFilesAndCookies(t *testing.T) {
	testutil.SetupTestEnv(t)
	t.Setenv("FORCE_HYPERLINK", "0")
	t.Setenv("NOTTE_CONSOLE_URL", "https://console.test")
	text, buf := plainText()
	err := viewFiles(text, []sessionFile{{ID: "f-1", SessionID: "s-1", Filename: "invoice.pdf", MimeType: "application/pdf", Size: 48213, Source: "session_download", CreatedAt: "2026-01-15T10:00:00Z", ExpiresAt: "bad"}}, viewNow)
	if err != nil {
		t.Fatal(err)
	}
	assertLines(t, buf.String(),
		"    FILE (1)   NAME          SIZE      TYPE              EXPIRES   CREATED   CONSOLE",
		"    f-1        invoice.pdf   48.2 kB   application/pdf   -         2 h ago   https://console.test/sessions/s-1",
		"Next: notte files download <id>",
	)

	text, buf = plainText()
	secure, lax := true, "Lax"
	if err := viewCookies(text, []api.Cookie{{Name: "sid", Domain: "example.com", Path: "/", HttpOnly: true, Secure: &secure, SameSite: &lax, Value: strings.Repeat("x", 60)}}, viewNow); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"COOKIE (1)", "session", "secure, http-only, same-site lax", strings.Repeat("x", 39) + "…"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("missing %q in:\n%s", want, buf.String())
		}
	}
}

// Text views never reach JSON output: it is the value as the API returned it.
func TestRenderJSONIsTheValue(t *testing.T) {
	origFormat := outputFormat
	outputFormat = "json"
	t.Cleanup(func() { outputFormat = origFormat })
	name := "Team logins"
	vaults := []api.Vault{{VaultId: "v-1", Name: name, CreatedAt: at(1)}}
	stdout, _ := testutil.CaptureOutput(func() {
		if err := render(vaults); err != nil {
			t.Fatal(err)
		}
	})
	want, _ := json.Marshal(vaults)
	if strings.TrimSpace(stdout) != string(want) {
		t.Errorf("got %s, want %s", stdout, want)
	}
}

func TestRenderHelpers(t *testing.T) {
	for n, want := range map[int64]string{0: "0 B", 999: "999 B", 1832: "1.8 kB", 48213: "48.2 kB", 5_300_000: "5.3 MB"} {
		if got := formatBytes(n); got != want {
			t.Errorf("formatBytes(%d) = %q, want %q", n, got, want)
		}
	}
	for d, want := range map[time.Duration]string{842 * time.Millisecond: "842ms", 4200 * time.Millisecond: "4.2s", 150 * time.Second: "2m 30s", 3900 * time.Second: "1h 5m"} {
		if got := formatDuration(d); got != want {
			t.Errorf("formatDuration(%v) = %q, want %q", d, got, want)
		}
	}
	if got := shorten("a  long\ndescription here", 10); got != "a long de…" {
		t.Errorf("shorten = %q", got)
	}
	if got := domainSummary([]string{"a.com", "b.com", "c.com", "d.com", "e.com"}, 3); got != "a.com, b.com, c.com +2 more" {
		t.Errorf("domainSummary = %q", got)
	}
	text, buf := plainText()
	if err := printDetails(text, "Title", [][2]string{{"Name", "x"}, {"Empty", ""}, {"Variables", "a\nb"}}); err != nil {
		t.Fatal(err)
	}
	assertLines(t, buf.String(), "Title", "  Name        x", "  Variables   a", "              b")
}

// Every list has the same frame: the gutter, the counted ID column first and,
// for resources the console shows, its link last.
func TestListsShareOneLayout(t *testing.T) {
	testutil.SetupTestEnv(t)
	t.Setenv("FORCE_HYPERLINK", "0")
	t.Setenv("NOTTE_CONSOLE_URL", "https://console.test")
	name := "n"
	for noun, view := range map[string]func(*output.TextFormatter) error{
		"FUNCTION": func(text *output.TextFormatter) error {
			return viewFunctions(text, []api.FunctionListItemResponse{{FunctionId: "id-1", Name: &name}}, viewNow)
		},
		"RUN": func(text *output.TextFormatter) error {
			return viewFunctionRuns(text, []api.FunctionRunListItemResponse{{FunctionRunId: "id-1", FunctionId: "fn"}}, viewNow)
		},
		"SECRET": func(text *output.TextFormatter) error {
			return viewSecrets(text, []api.SecretMetadata{{Id: "id-1"}}, viewNow)
		},
		"PERSONA": func(text *output.TextFormatter) error {
			return viewPersonas(text, []api.PersonaResponse{{PersonaId: "id-1"}}, viewNow)
		},
		"PROFILE": func(text *output.TextFormatter) error {
			return viewProfiles(text, []api.ProfileResponse{{ProfileId: "id-1"}}, viewNow)
		},
		"VAULT": func(text *output.TextFormatter) error {
			return viewVaults(text, []api.Vault{{VaultId: "id-1"}}, viewNow)
		},
		"FILE": func(text *output.TextFormatter) error {
			return viewFiles(text, []sessionFile{{ID: "id-1", SessionID: "s"}}, viewNow)
		},
		"REQUEST": func(text *output.TextFormatter) error {
			return viewUsageLogs(text, []api.UsageLog{{Endpoint: "id-1"}}, viewNow)
		},
	} {
		text, buf := plainText()
		if err := view(text); err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
		if !strings.HasPrefix(lines[0], "    "+noun+" (1)") || !strings.HasSuffix(lines[0], "CONSOLE") {
			t.Errorf("%s header: %q", noun, lines[0])
		}
		if !strings.HasPrefix(lines[1], "    id-1") || !strings.Contains(lines[1], "https://console.test/") {
			t.Errorf("%s row: %q", noun, lines[1])
		}
		if !strings.HasPrefix(lines[len(lines)-1], "Next: notte ") {
			t.Errorf("%s has no Next line: %q", noun, lines[len(lines)-1])
		}
	}
}
