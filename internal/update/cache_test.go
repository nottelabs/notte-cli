package update

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadCache_MissingFile(t *testing.T) {
	dir := t.TempDir()
	cache, err := LoadCache(dir)
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
	if cache != nil {
		t.Fatal("expected nil cache for missing file")
	}
}

func TestLoadCache_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, CacheFileName), []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadCache(dir)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestSaveAndLoadCache(t *testing.T) {
	dir := t.TempDir()
	now := time.Now().Truncate(time.Second) // JSON loses sub-second precision

	original := &UpdateCache{
		LatestVersion:  "v0.0.12",
		CurrentVersion: "0.0.10",
		CheckedAt:      now,
		ReleaseURL:     "https://github.com/nottelabs/notte-cli/releases/tag/v0.0.12",
	}

	if err := SaveCache(dir, original); err != nil {
		t.Fatalf("SaveCache error: %v", err)
	}

	loaded, err := LoadCache(dir)
	if err != nil {
		t.Fatalf("LoadCache error: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected non-nil cache")
	}
	if loaded.LatestVersion != original.LatestVersion {
		t.Errorf("LatestVersion = %q, want %q", loaded.LatestVersion, original.LatestVersion)
	}
	if loaded.CurrentVersion != original.CurrentVersion {
		t.Errorf("CurrentVersion = %q, want %q", loaded.CurrentVersion, original.CurrentVersion)
	}
	if loaded.ReleaseURL != original.ReleaseURL {
		t.Errorf("ReleaseURL = %q, want %q", loaded.ReleaseURL, original.ReleaseURL)
	}
}

func TestSaveCache_CreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	cache := &UpdateCache{
		LatestVersion:  "v1.0.0",
		CurrentVersion: "0.9.0",
		CheckedAt:      time.Now(),
	}
	if err := SaveCache(dir, cache); err != nil {
		t.Fatalf("SaveCache should create nested dirs, got error: %v", err)
	}

	loaded, err := LoadCache(dir)
	if err != nil {
		t.Fatalf("LoadCache error: %v", err)
	}
	if loaded.LatestVersion != "v1.0.0" {
		t.Errorf("unexpected version: %q", loaded.LatestVersion)
	}
}

func TestUpdateCache_IsStale(t *testing.T) {
	tests := []struct {
		name           string
		cache          UpdateCache
		currentVersion string
		want           bool
	}{
		{
			name: "fresh cache same version",
			cache: UpdateCache{
				CurrentVersion: "0.0.10",
				CheckedAt:      time.Now(),
			},
			currentVersion: "0.0.10",
			want:           false,
		},
		{
			name: "old cache same version",
			cache: UpdateCache{
				CurrentVersion: "0.0.10",
				CheckedAt:      time.Now().Add(-25 * time.Hour),
			},
			currentVersion: "0.0.10",
			want:           true,
		},
		{
			name: "fresh cache different version",
			cache: UpdateCache{
				CurrentVersion: "0.0.10",
				CheckedAt:      time.Now(),
			},
			currentVersion: "0.0.11",
			want:           true,
		},
		{
			name: "old cache different version",
			cache: UpdateCache{
				CurrentVersion: "0.0.10",
				CheckedAt:      time.Now().Add(-25 * time.Hour),
			},
			currentVersion: "0.0.11",
			want:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cache.IsStale(tt.currentVersion)
			if got != tt.want {
				t.Errorf("IsStale(%q) = %v, want %v", tt.currentVersion, got, tt.want)
			}
		})
	}
}
