package auth

import (
	"net/url"
	"os"

	"github.com/nottelabs/notte-cli/internal/config"
)

// hostToEnvLabel maps known API hostnames to canonical environment labels.
var hostToEnvLabel = map[string]string{
	"api.notte.cc":         "prod",
	"us-prod.notte.cc":     "prod",
	"us-staging.notte.cc":  "staging",
	"us-dev.notte.cc":      "dev",
	"us-dev-test.notte.cc": "dev",
}

// ResolveEnvLabel maps an API URL to a canonical environment label.
// Known hostnames are mapped to "prod", "staging", or "dev".
// Unknown hostnames use the hostname itself as the label.
// An empty URL defaults to "prod".
func ResolveEnvLabel(apiURL string) string {
	if apiURL == "" {
		return "prod"
	}
	u, err := url.Parse(apiURL)
	if err != nil || u.Host == "" {
		return "prod"
	}
	host := u.Hostname() // strips port
	if label, ok := hostToEnvLabel[host]; ok {
		return label
	}
	return host
}

// IsKnownEnvHost reports whether an API URL maps to one of the canonical
// environments, as opposed to a hostname used verbatim as its own label.
//
// The difference matters to callers that want to detect a contradiction: a URL
// that is definitively prod disagreeing with a section called staging is a
// misconfiguration, while a self-hosted or preview host simply has no
// canonical label to disagree with.
func IsKnownEnvHost(apiURL string) bool {
	u, err := url.Parse(apiURL)
	if err != nil || u.Host == "" {
		return false
	}
	_, ok := hostToEnvLabel[u.Hostname()]
	return ok
}

// KeyringKeyForEnv returns the env-qualified keyring key for the given label.
func KeyringKeyForEnv(envLabel string) string {
	return KeyringKey + ":" + envLabel
}

// GetCurrentAPIURL resolves the current API URL using the same logic as GetClient():
// NOTTE_API_URL env var -> config file -> DefaultAPIURL.
func GetCurrentAPIURL() string {
	if u := os.Getenv(config.EnvAPIURL); u != "" {
		return u
	}
	cfg, err := config.Load()
	if err == nil && cfg.APIURL != "" {
		return cfg.APIURL
	}
	return config.DefaultAPIURL
}
