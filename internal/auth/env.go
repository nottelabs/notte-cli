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

// consoleByEnvLabel maps an environment to the console that issues its keys.
//
// Only environments whose console is known are listed. Guessing one would
// recreate the problem this exists to solve: an unlisted environment falls back
// to the default rather than to a plausible-looking hostname that may belong to
// something else.
var consoleByEnvLabel = map[string]string{
	"prod":    config.DefaultConsoleURL,
	"staging": "https://staging-console.notte.cc",
}

// ConsoleURL returns the console to sign in through.
//
// NOTTE_CONSOLE_URL wins when set. Otherwise the console is derived from the
// API being used, because the two have to agree: the console hands back one of
// its own API keys, and login stores it under the environment NOTTE_API_URL
// names. Defaulting to production regardless meant that pointing only
// NOTTE_API_URL at another environment sent the browser to the production
// console and filed a production key under that environment's name.
//
// An environment with no known console falls back to the default. That
// combination no longer passes silently: the key is validated against the
// configured API, which rejects a key issued by a different one.
func ConsoleURL() string {
	if u := os.Getenv(config.EnvConsoleURL); u != "" {
		return u
	}
	if u, ok := consoleByEnvLabel[ResolveEnvLabel(GetCurrentAPIURL())]; ok {
		return u
	}
	return config.DefaultConsoleURL
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
