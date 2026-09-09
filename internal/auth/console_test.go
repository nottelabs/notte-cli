package auth

import (
	"testing"

	"github.com/nottelabs/notte-cli/internal/config"
)

func TestConsoleURLFollowsTheAPIEnvironment(t *testing.T) {
	// The bug this covers: only NOTTE_API_URL was set, the console defaulted to
	// production regardless, and login stored a production key under the name of
	// whichever environment NOTTE_API_URL pointed at.
	tests := []struct {
		name   string
		apiURL string
		want   string
	}{
		{"prod default", "https://api.notte.cc", "https://console.notte.cc"},
		{"prod alias", "https://us-prod.notte.cc", "https://console.notte.cc"},
		{"staging", "https://us-staging.notte.cc", "https://staging-console.notte.cc"},
		{"staging with path", "https://us-staging.notte.cc/v1", "https://staging-console.notte.cc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(config.EnvConsoleURL, "")
			t.Setenv(config.EnvAPIURL, tt.apiURL)

			if got := ConsoleURL(); got != tt.want {
				t.Errorf("ConsoleURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConsoleURLPrefersTheExplicitOverride(t *testing.T) {
	// A local or otherwise unlisted console has to remain reachable, and an
	// explicit value must not be second-guessed from the API URL.
	t.Setenv(config.EnvAPIURL, "https://us-staging.notte.cc")
	t.Setenv(config.EnvConsoleURL, "http://localhost:3000")

	if got := ConsoleURL(); got != "http://localhost:3000" {
		t.Errorf("ConsoleURL() = %q, want the explicit override", got)
	}
}

func TestConsoleURLFallsBackForUnknownEnvironments(t *testing.T) {
	// No console is guessed for an environment that has none recorded. The
	// resulting mismatch is caught downstream, where the key is validated
	// against the configured API.
	for _, apiURL := range []string{
		"https://us-dev.notte.cc",
		"http://localhost:8080",
		"https://my-api.example.com",
	} {
		t.Setenv(config.EnvConsoleURL, "")
		t.Setenv(config.EnvAPIURL, apiURL)

		if got := ConsoleURL(); got != config.DefaultConsoleURL {
			t.Errorf("ConsoleURL() with API %q = %q, want the default", apiURL, got)
		}
	}
}

func TestConsoleAuthURLTargetsTheDerivedConsole(t *testing.T) {
	// End of the chain: the link the sign-in page renders has to point at the
	// console the environment resolves to, not the compiled-in default.
	t.Setenv(config.EnvConsoleURL, "")
	t.Setenv(config.EnvAPIURL, "https://us-staging.notte.cc")

	server := NewSetupServer()
	server.baseURL = "http://127.0.0.1:12345"

	got := server.GetConsoleAuthURL()
	const want = "https://staging-console.notte.cc/auth/cli"
	if len(got) < len(want) || got[:len(want)] != want {
		t.Errorf("GetConsoleAuthURL() = %q, want it to start with %q", got, want)
	}
}
