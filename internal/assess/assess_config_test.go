package assess

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestLoadAssessOverridesDefaultsVersion(t *testing.T) {
	assessConfigCache = sync.Map{}
	root := t.TempDir()
	configDir := filepath.Join(root, ".goneat")
	if err := os.MkdirAll(configDir, 0o750); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	configPath := filepath.Join(configDir, "assess.yaml")
	content := []byte("lint:\n  yamllint:\n    enabled: true\n")
	if err := os.WriteFile(configPath, content, 0o600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	overrides := loadAssessOverrides(root)
	if overrides == nil {
		t.Fatalf("expected overrides to load")
	}
	if overrides.Version != 1 {
		t.Fatalf("expected version default to 1, got %d", overrides.Version)
	}
}

func TestLoadAssessOverridesInvalidSchema(t *testing.T) {
	assessConfigCache = sync.Map{}
	root := t.TempDir()
	configDir := filepath.Join(root, ".goneat")
	if err := os.MkdirAll(configDir, 0o750); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}
	configPath := filepath.Join(configDir, "assess.yaml")
	content := []byte("version: 2\nunknown_key: true\n")
	if err := os.WriteFile(configPath, content, 0o600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	overrides := loadAssessOverrides(root)
	if overrides != nil {
		t.Fatalf("expected overrides to be nil for invalid config")
	}
}
