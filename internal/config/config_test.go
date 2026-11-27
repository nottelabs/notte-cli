package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_Empty(t *testing.T) {
	// Use temp dir with no config
	tmpDir := t.TempDir()
	cfg, err := LoadFromPath(filepath.Join(tmpDir, "config.json"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.APIKey != "" {
		t.Errorf("expected empty API key, got %q", cfg.APIKey)
	}
	if cfg.APIURL != DefaultAPIURL {
		t.Errorf("expected default URL %q, got %q", DefaultAPIURL, cfg.APIURL)
	}
}

func TestLoadConfig_WithValues(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")

	content := `{"api_key": "test_key_123", "api_url": "https://custom.api.com"}`
	if err := os.WriteFile(cfgPath, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := LoadFromPath(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.APIKey != "test_key_123" {
		t.Errorf("expected API key 'test_key_123', got %q", cfg.APIKey)
	}
	if cfg.APIURL != "https://custom.api.com" {
		t.Errorf("expected custom URL, got %q", cfg.APIURL)
	}
}

func TestDir_EnvOverride(t *testing.T) {
	customDir := t.TempDir()
	t.Setenv("NOTTE_CONFIG_DIR", customDir)

	dir, err := Dir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := filepath.Join(customDir, ConfigDirName)
	if dir != expected {
		t.Errorf("expected %q, got %q", expected, dir)
	}
}

func TestDir_DefaultUsesHome(t *testing.T) {
	// Ensure env var is not set
	t.Setenv("NOTTE_CONFIG_DIR", "")

	dir, err := Dir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("unexpected error getting home dir: %v", err)
	}

	expected := filepath.Join(homeDir, ConfigDirName)
	if dir != expected {
		t.Errorf("expected %q, got %q", expected, dir)
	}
}

func TestSaveConfig(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")

	cfg := &Config{
		APIKey: "saved_key",
		APIURL: "https://saved.url.com",
	}

	if err := cfg.SaveToPath(cfgPath); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	loaded, err := LoadFromPath(cfgPath)
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}
	if loaded.APIKey != cfg.APIKey {
		t.Errorf("API key mismatch: got %q, want %q", loaded.APIKey, cfg.APIKey)
	}
}

func TestLoadConfig_HasAPIURL(t *testing.T) {
	tests := []struct {
		name    string
		content string // "" writes no file
		want    bool
	}{
		{"no file", "", false},
		{"file without api_url", `{"api_key": "k"}`, false},
		{"empty api_url", `{"api_url": ""}`, false},
		{"api_url set", `{"api_url": "https://custom.api.com"}`, true},
		// Naming the default host is still an explicit choice.
		{"api_url set to the default", `{"api_url": "https://api.notte.cc"}`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfgPath := filepath.Join(t.TempDir(), "config.json")
			if tt.content != "" {
				if err := os.WriteFile(cfgPath, []byte(tt.content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			cfg, err := LoadFromPath(cfgPath)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := cfg.HasAPIURL(); got != tt.want {
				t.Errorf("HasAPIURL() = %v, want %v", got, tt.want)
			}
			if cfg.APIURL == "" {
				t.Error("APIURL is empty, want the default filled in")
			}
		})
	}
}

func TestCredentialRegion(t *testing.T) {
	tests := []struct {
		credential string
		want       string
	}{
		// Default region
		{"sk-notte-abc123", ""},
		{"sk-notte-org-abc123", ""},
		{"nat_abc123", ""},
		{"", ""},

		// EU
		{"sk-notte-eu-abc123", RegionEU},
		{"sk-notte-org-eu-abc123", RegionEU},
		{"nat_eu_abc123", RegionEU},

		// Lookalikes stay in the default region
		{"sk-notte-europe-abc123", ""},
		{"sk-notte-org-europe-abc123", ""},
		{"sk-notte-eu", ""},
		{"sk-notte-euabc", ""},
		{"sk-notte-org-org-eu-abc", ""},
		{"sk-notte-xorg-eu-abc", ""},
		{"nat_europe_abc", ""},
		{"nat_eu-abc", ""},
		{"nat-eu_abc", ""},
		{"eu-sk-notte-abc", ""},
		{"SK-NOTTE-EU-abc", ""},
		{"sk-notte-EU-abc", ""},
	}
	for _, tt := range tests {
		t.Run(tt.credential, func(t *testing.T) {
			if got := CredentialRegion(tt.credential); got != tt.want {
				t.Errorf("CredentialRegion(%q) = %q, want %q", tt.credential, got, tt.want)
			}
		})
	}
}

func TestDefaultAPIURLFor(t *testing.T) {
	tests := []struct {
		credential string
		want       string
	}{
		{"", DefaultAPIURL},
		{"sk-notte-abc", DefaultAPIURL},
		{"sk-notte-org-abc", DefaultAPIURL},
		{"nat_abc", DefaultAPIURL},
		{"sk-notte-europe-abc", DefaultAPIURL},
		{"sk-notte-eu-abc", DefaultEUAPIURL},
		{"sk-notte-org-eu-abc", DefaultEUAPIURL},
		{"nat_eu_abc", DefaultEUAPIURL},
	}
	for _, tt := range tests {
		t.Run(tt.credential, func(t *testing.T) {
			if got := DefaultAPIURLFor(tt.credential); got != tt.want {
				t.Errorf("DefaultAPIURLFor(%q) = %q, want %q", tt.credential, got, tt.want)
			}
		})
	}
}
