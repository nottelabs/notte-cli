package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nottelabs/notte-cli/internal/config"
)

func TestClearLegacyCurrentResource(t *testing.T) {
	config.SetTestConfigDir(t.TempDir())
	t.Cleanup(func() { config.SetTestConfigDir("") })

	configDir, err := config.Dir()
	if err != nil {
		t.Fatalf("config.Dir() error = %v", err)
	}
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	agentPath := filepath.Join(configDir, "current_agent")
	if err := os.WriteFile(agentPath, []byte("agent-123"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := clearLegacyCurrentResource("current_agent"); err != nil {
		t.Fatalf("clearLegacyCurrentResource() error = %v", err)
	}
	if _, err := os.Stat(agentPath); !os.IsNotExist(err) {
		t.Errorf("legacy current agent file still exists; Stat() error = %v", err)
	}
	if err := clearLegacyCurrentResource("current_agent"); err != nil {
		t.Errorf("clearLegacyCurrentResource() should ignore a missing file: %v", err)
	}
}
