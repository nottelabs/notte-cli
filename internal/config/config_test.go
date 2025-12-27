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
