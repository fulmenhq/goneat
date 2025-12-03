package mapping

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManagerLoadFallbackToBuiltin(t *testing.T) {
	t.Parallel()
	mgr, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager error: %v", err)
	}

	repoDir := t.TempDir()

	res, err := mgr.Load(LoadOptions{RepoRoot: repoDir})
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if res.Repository != nil {
		t.Fatalf("expected no repository manifest, got %#v", res.Repository)
	}
	if len(res.Diagnostics) == 0 {
		t.Fatalf("expected diagnostics noting missing manifest")
	}
	if res.Effective.Version != ManifestVersionV1 {
		t.Fatalf("expected effective manifest version %s, got %s", ManifestVersionV1, res.Effective.Version)
	}
	if len(res.Effective.Mappings) != len(res.Builtin.Mappings) {
		t.Fatalf("expected builtin mappings only, got %d vs %d", len(res.Effective.Mappings), len(res.Builtin.Mappings))
	}
}

func TestManagerLoadsRepositoryManifest(t *testing.T) {
	t.Parallel()
	mgr, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager error: %v", err)
	}

	repoDir := t.TempDir()
	manifestPath := filepath.Join(repoDir, ".goneat")
	if err := os.MkdirAll(manifestPath, 0o755); err != nil {
		t.Fatalf("mkdir manifest dir: %v", err)
	}

	manifestFile := filepath.Join(manifestPath, "schema-mappings.yaml")
	manifestContent := `version: "1.0.0"
config:
  strict_mode: true
  min_confidence: 0.9
mappings:
  - pattern: "config/custom.yaml"
    schema_id: "custom-schema-v1"
    source: "external"
exclusions:
  - pattern: "tmp/**/*.yaml"
    action: "skip"
`
	if err := os.WriteFile(manifestFile, []byte(manifestContent), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	res, err := mgr.Load(LoadOptions{RepoRoot: repoDir})
	if err != nil {
		t.Fatalf("Load returned unexpected error: %v", err)
	}

	if res.Repository == nil {
		t.Fatalf("expected repository manifest to be loaded")
	}
	if res.RepositoryPath != manifestFile {
		t.Fatalf("unexpected repository path: %s", res.RepositoryPath)
	}

	// Verify config defaults applied with override.
	if res.Effective.Config.StrictMode == nil || !*res.Effective.Config.StrictMode {
		t.Fatalf("expected strict_mode to be true")
	}
	if res.Effective.Config.MinConfidence == nil || *res.Effective.Config.MinConfidence != 0.9 {
		t.Fatalf("expected min_confidence 0.9")
	}

	// Ensure mapping appended.
	found := false
	for _, mapping := range res.Effective.Mappings {
		if mapping.Pattern == "config/custom.yaml" && mapping.SchemaID == "custom-schema-v1" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected custom mapping to be part of effective manifest")
	}
}

func TestManagerRejectsInvalidManifest(t *testing.T) {
	t.Parallel()
	mgr, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager error: %v", err)
	}

	repoDir := t.TempDir()
	manifestPath := filepath.Join(repoDir, ".goneat")
	if err := os.MkdirAll(manifestPath, 0o755); err != nil {
		t.Fatalf("mkdir manifest dir: %v", err)
	}

	manifestFile := filepath.Join(manifestPath, "schema-mappings.yaml")
	manifestContent := `version: "2.0.0"`
	if err := os.WriteFile(manifestFile, []byte(manifestContent), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	if _, err := mgr.Load(LoadOptions{RepoRoot: repoDir}); err == nil {
		t.Fatalf("expected error for unsupported manifest version")
	}
}

func TestManagerGuardsAgainstTraversal(t *testing.T) {
	t.Parallel()
	mgr, err := NewManager()
	if err != nil {
		t.Fatalf("NewManager error: %v", err)
	}

	repoDir := t.TempDir()
	opts := LoadOptions{RepoRoot: repoDir, ManifestPath: "../evil.yaml"}
	if _, err := mgr.Load(opts); err == nil {
		t.Fatalf("expected error for manifest path traversal")
	}
}
