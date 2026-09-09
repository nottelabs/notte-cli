//go:build integration

package integration

import (
	"net/http"
	"testing"
	"time"

	"github.com/nottelabs/notte-cli/internal/auth"
	"github.com/nottelabs/notte-cli/internal/config"
)

// TestConsoleURL_MappedConsolesExist checks the consoles sign-in derives from
// the API environment are real.
//
// The mapping is hostnames written down in the CLI. Unit tests only assert the
// CLI returns the string it was told to; nothing there notices if a console is
// renamed or retired, and the symptom would be `notte auth login` opening a
// browser at a dead address with no error from the CLI itself.
func TestConsoleURL_MappedConsolesExist(t *testing.T) {
	// Representative API URLs for each environment the mapping covers. Driving
	// it through ConsoleURL rather than a copy of the table means a newly
	// mapped environment is covered here as soon as it is added.
	apiURLs := []string{
		"https://api.notte.cc",
		"https://us-staging.notte.cc",
	}

	seen := map[string]bool{}
	for _, apiURL := range apiURLs {
		t.Setenv(config.EnvConsoleURL, "")
		t.Setenv(config.EnvAPIURL, apiURL)
		seen[auth.ConsoleURL()] = true
	}

	client := &http.Client{
		Timeout: 15 * time.Second,
		// Sign-in redirects through an identity provider, which is a healthy
		// response here. Following it would test that provider instead.
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	for consoleURL := range seen {
		t.Run(consoleURL, func(t *testing.T) {
			target := consoleURL + "/auth/cli"
			resp, err := client.Get(target)
			if err != nil {
				t.Fatalf("GET %s: %v", target, err)
			}
			defer func() { _ = resp.Body.Close() }()

			// Deliberately narrow. The page is behind sign-in and may answer a
			// redirect, a challenge or a rate limit depending on where this
			// runs; a 404 is the one answer that means the CLI is pointing
			// somewhere that no longer serves this flow.
			if resp.StatusCode == http.StatusNotFound {
				t.Errorf("GET %s returned 404, so this console no longer serves CLI sign-in", target)
			}
			t.Logf("GET %s -> %d", target, resp.StatusCode)
		})
	}
}
