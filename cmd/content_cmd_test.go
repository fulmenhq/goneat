package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestContentFind_JSON(t *testing.T) {
	out, err := execRoot(t, []string{"content", "find", "--format", "json"})
	if err != nil {
		t.Fatalf("content find failed: %v\n%s", err, out)
	}
	var v struct {
		Version  string           `json:"version"`
		Root     string           `json:"root"`
		Manifest string           `json:"manifest"`
		Count    int              `json:"count"`
		Items    []map[string]any `json:"items"`
	}
	if json.Unmarshal([]byte(out), &v) != nil {
		t.Fatalf("content find output is not valid JSON: %s", out)
	}
	if v.Count == 0 || len(v.Items) == 0 {
		t.Fatalf("expected at least one curated doc in find output: %s", out)
	}
}

func TestContentFind_SchemasManifest(t *testing.T) {
	manifest := writeTempFile(t, "schemas-embed.yaml", "version: \"1.1.0\"\nasset_type: \"schemas\"\ntopics:\n  schemas:\n    include:\n      - \"**/*.yaml\"\n    exclude:\n      - \"**/docs/**\"\n")
	if _, err := os.Stat(manifest); err != nil {
		t.Fatalf("manifest not created: %v", err)
	}
	out, err := execRoot(t, []string{"content", "find", "--manifest", manifest, "--root", "schemas", "--format", "json"})
	if err != nil {
		t.Fatalf("content find (schemas) failed: %v\n%s", err, out)
	}
	var v struct {
		Count int              `json:"count"`
		Items []map[string]any `json:"items"`
	}
	if json.Unmarshal([]byte(out), &v) != nil {
		t.Fatalf("content find output is not valid JSON: %s", out)
	}
	if v.Count == 0 || len(v.Items) == 0 {
		t.Fatalf("expected at least one schema asset in find output: %s", out)
	}
	if asset, ok := v.Items[0]["asset_type"].(string); !ok || asset == "" {
		t.Fatalf("expected asset_type to be set: %v", v.Items[0])
	}
}

func TestContentVerify_OK(t *testing.T) {
	// Ensure mirror is populated
	if _, err := execRoot(t, []string{"content", "embed"}); err != nil {
		t.Fatalf("content embed failed: %v", err)
	}
	if _, err := execRoot(t, []string{"content", "verify", "--format", "json"}); err != nil {
		t.Fatalf("content verify failed: %v", err)
	}
}

func TestContentManifests_JSON(t *testing.T) {
	out, err := execRoot(t, []string{"content", "manifests", "--format", "json"})
	if err != nil {
		t.Fatalf("content manifests failed: %v\n%s", err, out)
	}
	var payload struct {
		Count     int           `json:"count"`
		Manifests []interface{} `json:"manifests"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("content manifests output is not valid JSON: %v\n%s", err, out)
	}
	if payload.Count == 0 || len(payload.Manifests) == 0 {
		t.Fatalf("expected at least one manifest in listing: %s", out)
	}
}

func TestContentMigrateManifest(t *testing.T) {
	repo := findRepoRoot()
	if repo == "" {
		t.Fatal("repository root not found")
	}
	srcPath := filepath.Join(repo, "docs", "embed-manifest.yaml")
	data, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatalf("failed to read source manifest: %v", err)
	}
	manifestPath := writeTempFile(t, "legacy-manifest.yaml", string(data))
	outputPath := filepath.ToSlash(filepath.Join(filepath.Dir(manifestPath), "migrated.yaml"))
	out, err := execRoot(t, []string{"content", "migrate-manifest", "--manifest", manifestPath, "--output", outputPath})
	if err != nil {
		t.Fatalf("content migrate-manifest failed: %v\n%s", err, out)
	}
	readPath := outputPath
	if !filepath.IsAbs(readPath) {
		readPath = filepath.Join(repo, readPath)
	}
	bytes, err := os.ReadFile(readPath)
	if err != nil {
		t.Fatalf("failed to read migrated manifest: %v", err)
	}
	var migrated struct {
		Version string `yaml:"version"`
	}
	if err := yaml.Unmarshal(bytes, &migrated); err != nil {
		t.Fatalf("failed to parse migrated manifest: %v\n%s", err, string(bytes))
	}
	if migrated.Version != "1.1.0" {
		t.Fatalf("expected migrated manifest version 1.1.0, got %q", migrated.Version)
	}
}

func TestContentEmbed_DryRun_JSON(t *testing.T) {
	out, err := execRoot(t, []string{"content", "embed", "--dry-run", "--format", "json"})
	if err != nil {
		t.Fatalf("content embed dry-run failed: %v\n%s", err, out)
	}
	var payload struct {
		Targets []struct {
			DryRun bool `json:"dry_run"`
			Count  int  `json:"count"`
		}
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("dry-run output is not valid JSON: %v\n%s", err, out)
	}
	if len(payload.Targets) == 0 {
		t.Fatalf("expected at least one target in dry-run output: %s", out)
	}
	for _, target := range payload.Targets {
		if !target.DryRun {
			t.Fatalf("expected dry_run=true for all targets: %s", out)
		}
	}
	if payload.Targets[0].Count == 0 {
		t.Fatalf("expected dry-run target to report at least one asset: %s", out)
	}
}

func TestContentConflicts_JSON(t *testing.T) {
	manifest := writeTempFile(t, "conflict-embed.yaml", "version: \"1.1.0\"\nasset_type: \"docs\"\ntopics:\n  first:\n    include:\n      - \"docs/user-guide/install.md\"\n  second:\n    include:\n      - \"docs/user-guide/install.md\"\n")
	out, err := execRoot(t, []string{"content", "conflicts", "--manifest", manifest, "--root", ".", "--format", "json"})
	if err != nil {
		t.Fatalf("content conflicts failed: %v\n%s", err, out)
	}
	var payload struct {
		Count     int `json:"count"`
		Conflicts []struct {
			Severity string `json:"severity"`
			Message  string `json:"message"`
		}
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("conflicts output is not valid JSON: %v\n%s", err, out)
	}
	if payload.Count == 0 || len(payload.Conflicts) == 0 {
		t.Fatalf("expected at least one conflict: %s", out)
	}
	if payload.Conflicts[0].Severity != "conflict" {
		t.Fatalf("expected conflict severity, got %q", payload.Conflicts[0].Severity)
	}
	if !strings.Contains(payload.Conflicts[0].Message, "install") {
		t.Fatalf("expected conflict message to reference file: %s", payload.Conflicts[0].Message)
	}
}

func TestContentInit_JSON(t *testing.T) {
	repo := findRepoRoot()
	if repo == "" {
		t.Fatal("repository root not found")
	}
	outputRel := filepath.ToSlash(filepath.Join("test_temp", strings.ReplaceAll(t.Name(), "/", "_"), "init.yaml"))
	out, err := execRoot(t, []string{
		"content", "init",
		"--asset-type", "schemas",
		"--root", "schemas",
		"--topic", "core-schemas",
		"--include", "**/*.json",
		"--include", "**/*.yaml",
		"--output", outputRel,
		"--format", "json",
		"--overwrite",
	})
	if err != nil {
		t.Fatalf("content init failed: %v\n%s", err, out)
	}
	var payload struct {
		Path      string `json:"path"`
		AssetType string `json:"asset_type"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("init output is not valid JSON: %v\n%s", err, out)
	}
	if payload.AssetType != "schemas" {
		t.Fatalf("expected asset_type schemas, got %q", payload.AssetType)
	}
	readPath := payload.Path
	if !filepath.IsAbs(readPath) {
		readPath = filepath.Join(repo, filepath.FromSlash(readPath))
	}
	bytes, err := os.ReadFile(readPath)
	if err != nil {
		t.Fatalf("failed to read generated manifest: %v", err)
	}
	var manifest embedManifest
	if err := yaml.Unmarshal(bytes, &manifest); err != nil {
		t.Fatalf("failed to parse generated manifest: %v", err)
	}
	if manifest.Version != "1.1.0" {
		t.Fatalf("expected version 1.1.0, got %q", manifest.Version)
	}
	if manifest.AssetType != "schemas" {
		t.Fatalf("expected asset type schemas, got %q", manifest.AssetType)
	}
}

func writeTempFile(t *testing.T, name, contents string) string {
	t.Helper()
	repo := findRepoRoot()
	if repo == "" {
		repo = "."
	}
	dir := filepath.Join(repo, "test_temp", strings.ReplaceAll(t.Name(), "/", "_"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return filepath.ToSlash(path)
}
