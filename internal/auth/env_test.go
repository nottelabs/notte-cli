package auth

import (
	"testing"
)

func TestResolveEnvLabel(t *testing.T) {
	tests := []struct {
		name   string
		apiURL string
		want   string
	}{
		// Known prod aliases
		{"default prod", "https://api.notte.cc", "prod"},
		{"us-prod", "https://us-prod.notte.cc", "prod"},
		{"prod with path", "https://api.notte.cc/v1/foo", "prod"},

		// Staging
		{"staging", "https://us-staging.notte.cc", "staging"},
		{"staging with path", "https://us-staging.notte.cc/v1", "staging"},

		// Dev aliases
		{"dev", "https://us-dev.notte.cc", "dev"},
		{"dev-test", "https://us-dev-test.notte.cc", "dev"},

		// Unknown hosts -> hostname as label
		{"localhost", "http://localhost:8080", "localhost"},
		{"custom host", "https://my-api.example.com", "my-api.example.com"},

		// Empty URL -> prod
		{"empty URL", "", "prod"},

		// Invalid URL -> prod
		{"invalid URL", "://bad", "prod"},

		// Port stripping
		{"prod with port", "https://api.notte.cc:443", "prod"},
		{"localhost with port", "http://localhost:3000", "localhost"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveEnvLabel(tt.apiURL)
			if got != tt.want {
				t.Errorf("ResolveEnvLabel(%q) = %q, want %q", tt.apiURL, got, tt.want)
			}
		})
	}
}

func TestKeyringKeyForEnv(t *testing.T) {
	tests := []struct {
		label string
		want  string
	}{
		{"prod", "api_key:prod"},
		{"staging", "api_key:staging"},
		{"dev", "api_key:dev"},
		{"localhost", "api_key:localhost"},
	}

	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			got := KeyringKeyForEnv(tt.label)
			if got != tt.want {
				t.Errorf("KeyringKeyForEnv(%q) = %q, want %q", tt.label, got, tt.want)
			}
		})
	}
}
