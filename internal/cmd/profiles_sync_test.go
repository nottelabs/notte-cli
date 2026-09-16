package cmd

import (
	"strings"
	"testing"

	"github.com/nottelabs/notte-cli/internal/browser"
)

func TestToAPICookies(t *testing.T) {
	in := []browser.Cookie{
		{Name: "a", Value: "1", Domain: "github.com", Path: "/", Secure: true, HTTPOnly: true, SameSite: "Lax", Expires: 2000000000},
		{Name: "s", Value: "2", Domain: ".github.com", Path: "/", Session: true, SameSite: ""},
	}
	out := toAPICookies(in)
	if len(out) != 2 {
		t.Fatalf("got %d cookies, want 2", len(out))
	}

	persistent := out[0]
	if persistent.Name != "a" || persistent.Value != "1" || persistent.Domain != "github.com" || !persistent.HttpOnly {
		t.Errorf("persistent cookie mapped wrong: %+v", persistent)
	}
	if persistent.Secure == nil || !*persistent.Secure {
		t.Error("secure should be true")
	}
	if persistent.SameSite == nil || *persistent.SameSite != "Lax" {
		t.Error("sameSite should be Lax")
	}
	if persistent.Expires == nil || persistent.ExpirationDate == nil {
		t.Error("persistent cookie should carry an expiry")
	}
	if persistent.Session != nil && *persistent.Session {
		t.Error("persistent cookie should not be marked session")
	}

	session := out[1]
	if session.Session == nil || !*session.Session {
		t.Error("session cookie should be marked session")
	}
	if session.Expires != nil {
		t.Error("session cookie should not carry an expiry")
	}
	if session.SameSite != nil {
		t.Error("unspecified sameSite should be omitted")
	}
}

func TestDomainCounts(t *testing.T) {
	cookies := toAPICookies([]browser.Cookie{
		{Name: "a", Domain: "github.com"},
		{Name: "b", Domain: ".github.com"},
		{Name: "c", Domain: "google.com"},
	})
	counts := domainCounts(cookies)
	if len(counts) != 2 {
		t.Fatalf("got %d domains, want 2", len(counts))
	}
	// Sorted, and the leading dot is normalized away so both github entries fold.
	if counts[0].Domain != "github.com" || counts[0].Count != 2 {
		t.Errorf("counts[0] = %+v, want github.com x2", counts[0])
	}
	if counts[1].Domain != "google.com" || counts[1].Count != 1 {
		t.Errorf("counts[1] = %+v, want google.com x1", counts[1])
	}
}

func TestFilterByProfileName(t *testing.T) {
	chrome, _ := browser.BrowserByID("chrome")
	profiles := []browser.Profile{
		{Browser: chrome, Dir: "Default", Name: "Personal"},
		{Browser: chrome, Dir: "Profile 1", Name: "Work"},
	}
	if got := filterByProfileName(profiles, "work"); len(got) != 1 || got[0].Dir != "Profile 1" {
		t.Errorf("match by name failed: %+v", got)
	}
	if got := filterByProfileName(profiles, "Default"); len(got) != 1 || got[0].Name != "Personal" {
		t.Errorf("match by dir failed: %+v", got)
	}
	if got := filterByProfileName(profiles, "nope"); len(got) != 0 {
		t.Errorf("expected no match, got %+v", got)
	}
}

func TestConfirmSyncWithIO(t *testing.T) {
	var out strings.Builder
	for _, answer := range []string{"y\n", "yes\n", "Y\n"} {
		out.Reset()
		if !confirmSyncWithIO(strings.NewReader(answer), &out, 3, "profile x") {
			t.Errorf("answer %q should confirm", answer)
		}
	}
	for _, answer := range []string{"n\n", "\n", "no\n", "nope\n"} {
		out.Reset()
		if confirmSyncWithIO(strings.NewReader(answer), &out, 3, "profile x") {
			t.Errorf("answer %q should decline", answer)
		}
	}
	if !strings.Contains(out.String(), "3 cookies") {
		t.Error("prompt should state the cookie count")
	}
}

func TestPickProfileWithIO(t *testing.T) {
	chrome, _ := browser.BrowserByID("chrome")
	profiles := []browser.Profile{
		{Browser: chrome, Dir: "Default", Name: "Personal", Email: "me@example.com"},
		{Browser: chrome, Dir: "Profile 1", Name: "Work"},
	}

	var out strings.Builder
	got, err := pickProfileWithIO(strings.NewReader("2\n"), &out, profiles)
	if err != nil {
		t.Fatalf("pick: %v", err)
	}
	if got.Name != "Work" {
		t.Errorf("picked %q, want Work", got.Name)
	}
	if !strings.Contains(out.String(), "me@example.com") {
		t.Error("prompt should show the signed-in email")
	}

	if _, err := pickProfileWithIO(strings.NewReader("9\n"), &out, profiles); err == nil {
		t.Error("out-of-range choice should error")
	}
	if _, err := pickProfileWithIO(strings.NewReader("abc\n"), &out, profiles); err == nil {
		t.Error("non-numeric choice should error")
	}
}
