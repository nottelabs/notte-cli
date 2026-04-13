package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// setReleasesURL overrides the GitHub releases URL for testing and returns
// a cleanup function that restores the original value.
func setReleasesURL(t *testing.T, url string) {
	t.Helper()
	orig := githubReleasesURL
	githubReleasesURL = url
	t.Cleanup(func() { githubReleasesURL = orig })
}

func TestCheckLatestVersion_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/vnd.github+json" {
			t.Error("expected Accept header")
		}
		if r.Header.Get("User-Agent") != "notte-cli-update-checker" {
			t.Error("expected User-Agent header")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"tag_name": "v0.0.12", "html_url": "https://github.com/nottelabs/notte-cli/releases/tag/v0.0.12"}`))
	}))
	defer server.Close()
	setReleasesURL(t, server.URL)

	ctx := context.Background()
	release, err := CheckLatestVersion(ctx, server.Client())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if release == nil {
		t.Fatal("expected non-nil release")
	}
	if release.TagName != "v0.0.12" {
		t.Errorf("TagName = %q, want %q", release.TagName, "v0.0.12")
	}
	if release.HTMLURL != "https://github.com/nottelabs/notte-cli/releases/tag/v0.0.12" {
		t.Errorf("HTMLURL = %q, want release URL", release.HTMLURL)
	}
}

func TestCheckLatestVersion_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	setReleasesURL(t, server.URL)

	ctx := context.Background()
	release, err := CheckLatestVersion(ctx, server.Client())
	if err != nil {
		t.Fatalf("expected nil error on HTTP error, got %v", err)
	}
	if release != nil {
		t.Fatal("expected nil release on HTTP error")
	}
}

func TestCheckLatestVersion_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	setReleasesURL(t, server.URL)

	ctx := context.Background()
	release, err := CheckLatestVersion(ctx, server.Client())
	if err != nil {
		t.Fatalf("expected nil error on server error, got %v", err)
	}
	if release != nil {
		t.Fatal("expected nil release on server error")
	}
}

func TestCheckLatestVersion_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	setReleasesURL(t, server.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	release, err := CheckLatestVersion(ctx, server.Client())
	if err != nil {
		t.Fatalf("expected nil error on timeout, got %v", err)
	}
	if release != nil {
		t.Fatal("expected nil release on timeout")
	}
}

func TestCheckLatestVersion_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not valid json`))
	}))
	defer server.Close()
	setReleasesURL(t, server.URL)

	ctx := context.Background()
	release, err := CheckLatestVersion(ctx, server.Client())
	if err != nil {
		t.Fatalf("expected nil error on invalid JSON, got %v", err)
	}
	if release != nil {
		t.Fatal("expected nil release on invalid JSON")
	}
}

func TestCheckLatestVersion_EmptyTagName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"tag_name": "", "html_url": ""}`))
	}))
	defer server.Close()
	setReleasesURL(t, server.URL)

	ctx := context.Background()
	release, err := CheckLatestVersion(ctx, server.Client())
	if err != nil {
		t.Fatalf("expected nil error for empty tag_name, got %v", err)
	}
	if release != nil {
		t.Fatal("expected nil release for empty tag_name")
	}
}
