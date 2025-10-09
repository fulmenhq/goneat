package ssot

import (
	"os"
	"path/filepath"
	"testing"
)

const primaryConfig = `version: v1.1.0

sources:
  - name: crucible
    repo: fulmenhq/crucible
    ref: main
    sync_path_base: lang/go
    assets:
      - type: doc
        paths:
          - docs/**/*
        subdir: docs/crucible-go
      - type: schema
        paths:
          - schemas/**/*
        subdir: schemas/crucible-go

strategy:
  on_conflict: overwrite
  prune_stale: true
  verify_checksums: false
`

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()

	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", full, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", full, err)
	}
}

func withWorkingDir(t *testing.T, dir string) func() {
	t.Helper()

	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	return func() {
		_ = os.Chdir(orig)
	}
}

func TestLoadSyncConfig_PrimaryOnly(t *testing.T) {
	tmp := t.TempDir()
	writeFile(t, tmp, ".goneat/ssot-consumer.yaml", primaryConfig)

	restore := withWorkingDir(t, tmp)
	defer restore()

	cfg, err := LoadSyncConfig()
	if err != nil {
		t.Fatalf("LoadSyncConfig() error = %v", err)
	}
	if cfg.isLocal {
		t.Fatalf("expected isLocal=false, got true")
	}
	if len(cfg.Sources) != 1 {
		t.Fatalf("expected 1 source, got %d", len(cfg.Sources))
	}

	source := cfg.Sources[0]
	if source.Name != "crucible" {
		t.Fatalf("unexpected source name: %s", source.Name)
	}
	if source.LocalPath != "" {
		t.Fatalf("expected empty LocalPath, got %s", source.LocalPath)
	}
	if len(source.Assets) != 2 {
		t.Fatalf("expected 2 assets, got %d", len(source.Assets))
	}
}

func TestLoadSyncConfig_LocalOverrideMinimal(t *testing.T) {
	tmp := t.TempDir()
	writeFile(t, tmp, ".goneat/ssot-consumer.yaml", primaryConfig)
	writeFile(t, tmp, ".goneat/ssot-consumer.local.yaml", `version: v1.1.0

sources:
  - name: crucible
    localPath: ../crucible
`)

	restore := withWorkingDir(t, tmp)
	defer restore()

	cfg, err := LoadSyncConfig()
	if err != nil {
		t.Fatalf("LoadSyncConfig() error = %v", err)
	}
	if !cfg.isLocal {
		t.Fatalf("expected isLocal=true")
	}

	source := cfg.Sources[0]
	if got, want := source.LocalPath, "../crucible"; got != want {
		t.Fatalf("localPath mismatch: got %s want %s", got, want)
	}
	if len(source.Assets) != 2 {
		t.Fatalf("expected assets preserved from primary manifest")
	}
}

func TestLoadSyncConfig_EnvOverride(t *testing.T) {
	tmp := t.TempDir()
	writeFile(t, tmp, ".goneat/ssot-consumer.yaml", primaryConfig)

	restore := withWorkingDir(t, tmp)
	defer restore()

	envPath := filepath.Join(tmp, "crucible-cache")
	t.Setenv("GONEAT_SSOT_CONSUMER_CRUCIBLE_LOCAL_PATH", envPath)

	cfg, err := LoadSyncConfig()
	if err != nil {
		t.Fatalf("LoadSyncConfig() error = %v", err)
	}

	source := cfg.Sources[0]
	if got, want := source.LocalPath, envPath; got != want {
		t.Fatalf("env override mismatch: got %s want %s", got, want)
	}
}
