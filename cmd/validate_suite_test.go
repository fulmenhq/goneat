package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestValidateSuite_JSON_PassesWithExpectedFailures(t *testing.T) {

	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("create fake .git: %v", err)
	}

	manifestPath := filepath.Join(repo, "schema-mappings.yaml")
	if err := os.WriteFile(manifestPath, []byte(`version: "1.0.0"
mappings:
  - pattern: "**/*.yaml"
    schema_path: schemas/root.schema.json
`), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	schemasDir := filepath.Join(repo, "schemas")
	if err := os.MkdirAll(schemasDir, 0o755); err != nil {
		t.Fatalf("mkdir schemas: %v", err)
	}

	rootSchema := []byte(`{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://example.invalid/schemas/root.schema.json",
  "type": "object",
  "properties": {
    "implementation": {
      "$ref": "https://example.invalid/schemas/types.schema.json#/$defs/slug"
    }
  },
  "required": ["implementation"],
  "additionalProperties": false
}`)
	if err := os.WriteFile(filepath.Join(schemasDir, "root.schema.json"), rootSchema, 0o644); err != nil {
		t.Fatalf("write root schema: %v", err)
	}

	refSchema := []byte(`{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://example.invalid/schemas/types.schema.json",
  "$defs": {
    "slug": {
      "type": "string",
      "pattern": "^[a-z0-9-]+$"
    }
  }
}`)
	if err := os.WriteFile(filepath.Join(schemasDir, "types.schema.json"), refSchema, 0o644); err != nil {
		t.Fatalf("write ref schema: %v", err)
	}

	examplesDir := filepath.Join(repo, "examples")
	if err := os.MkdirAll(examplesDir, 0o755); err != nil {
		t.Fatalf("mkdir examples: %v", err)
	}
	if err := os.WriteFile(filepath.Join(examplesDir, "good.yaml"), []byte("implementation: nextcloud\n"), 0o644); err != nil {
		t.Fatalf("write good example: %v", err)
	}
	if err := os.WriteFile(filepath.Join(examplesDir, "bad.yaml"), []byte("implementation: 5\n"), 0o644); err != nil {
		t.Fatalf("write bad example: %v", err)
	}

	out, err := execRoot(t, []string{
		"validate", "suite",
		"--data", examplesDir,
		"--manifest", "schema-mappings.yaml",
		"--ref-dir", schemasDir,
		"--expect-fail", "**/bad.yaml",
		"--format", "json",
		"--workers", "2",
	})
	if err != nil {
		t.Fatalf("expected suite to pass, got error: %v\n%s", err, out)
	}

	var res validateSuiteResult
	if uerr := json.Unmarshal([]byte(out), &res); uerr != nil {
		t.Fatalf("expected JSON output, got parse error: %v\n%s", uerr, out)
	}
	if res.Summary.Total != 2 {
		t.Fatalf("expected total=2, got %d", res.Summary.Total)
	}
	if res.Summary.Passed != 1 || res.Summary.ExpectedFail != 1 {
		t.Fatalf("expected passed=1 expected_fail=1, got passed=%d expected_fail=%d", res.Summary.Passed, res.Summary.ExpectedFail)
	}
	if res.Summary.Failed != 0 || res.Summary.UnexpectedPass != 0 || res.Summary.Unmapped != 0 {
		t.Fatalf("expected no failures/unmapped, got failed=%d unexpected_pass=%d unmapped=%d", res.Summary.Failed, res.Summary.UnexpectedPass, res.Summary.Unmapped)
	}
}

func execRootSplit(t *testing.T, args []string) (string, string, error) {
	t.Helper()

	// Mirror execRoot() resets to prevent cross-test bleed
	validateSuiteDataRoot = ""
	validateSuiteSchemasRoot = ""
	validateSuiteManifestPath = ".goneat/schema-mappings.yaml"
	validateSuiteRefDirs = nil
	validateSuiteNoIgnore = false
	validateSuiteForceInclude = nil
	validateSuiteExclude = nil
	validateSuiteSkip = nil
	validateSuiteExpectFail = nil
	validateSuiteStrict = false
	validateSuiteEnableMeta = false
	validateSuiteMaxWorkers = 2
	validateSuiteTimeout = 3 * time.Minute
	validateSuiteFormat = "markdown"
	validateSuiteFailOnUnmapped = true
	validateSuiteSchemaResolution = "prefer-id"
	validateSchemaRefDirs = nil

	cmd := newRootCommand()
	registerSubcommands(cmd)

	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)

	full := append([]string{"--log-level", "error"}, args...)
	cmd.SetArgs(full)

	t.Setenv("GONEAT_OFFLINE_SCHEMA_VALIDATION", "true")
	err := cmd.Execute()
	return outBuf.String(), errBuf.String(), err
}

func TestValidateSuite_JSON_FailsOnUnmapped(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("create fake .git: %v", err)
	}

	manifestPath := filepath.Join(repo, "schema-mappings.yaml")
	if err := os.WriteFile(manifestPath, []byte(`version: "1.0.0"
mappings:
  - pattern: "**/*.yaml"
    schema_path: schemas/root.schema.json
`), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	schemasDir := filepath.Join(repo, "schemas")
	if err := os.MkdirAll(schemasDir, 0o755); err != nil {
		t.Fatalf("mkdir schemas: %v", err)
	}
	// schema contents won't be reached because file is unmapped
	_ = os.WriteFile(filepath.Join(schemasDir, "root.schema.json"), []byte(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object"}`), 0o644)

	// This file is unmapped because our manifest only maps YAML

	examplesDir := filepath.Join(repo, "examples")
	if err := os.MkdirAll(examplesDir, 0o755); err != nil {
		t.Fatalf("mkdir examples: %v", err)
	}
	if err := os.WriteFile(filepath.Join(examplesDir, "unmapped.json"), []byte(`{"hello":"world"}`), 0o644); err != nil {
		t.Fatalf("write unmapped: %v", err)
	}

	stdout, stderr, err := execRootSplit(t, []string{
		"validate", "suite",
		"--data", examplesDir,
		"--manifest", "schema-mappings.yaml",
		"--format", "json",
	})
	if err == nil {
		t.Fatalf("expected suite to fail, got nil error\nstdout:\n%s\nstderr:\n%s", stdout, stderr)
	}

	var res validateSuiteResult
	if uerr := json.Unmarshal([]byte(stdout), &res); uerr != nil {
		t.Fatalf("expected stdout JSON output, got parse error: %v\nstdout:\n%s\nstderr:\n%s", uerr, stdout, stderr)
	}
	if res.Summary.Unmapped != 1 {
		t.Fatalf("expected unmapped=1, got %d", res.Summary.Unmapped)
	}
}

func TestValidateSuite_JSON_UsesOverridesPathForLocalSchemaID(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("create fake .git: %v", err)
	}

	manifestPath := filepath.Join(repo, "schema-mappings.yaml")
	if err := os.WriteFile(manifestPath, []byte(`version: "1.0.0"
overrides:
  - schema_id: enact-recipe-v1.0.0
    source: local
    path: schemas/root.schema.json
mappings:
  - pattern: "**/*.yaml"
    schema_id: enact-recipe-v1.0.0
    source: local
`), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	schemasDir := filepath.Join(repo, "schemas")
	if err := os.MkdirAll(schemasDir, 0o755); err != nil {
		t.Fatalf("mkdir schemas: %v", err)
	}

	refSchema := []byte(`{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://example.invalid/schemas/types.schema.json",
  "$defs": {
    "slug": {
      "type": "string",
      "pattern": "^[a-z0-9-]+$"
    }
  }
}`)
	if err := os.WriteFile(filepath.Join(schemasDir, "types.schema.json"), refSchema, 0o644); err != nil {
		t.Fatalf("write ref schema: %v", err)
	}

	rootSchema := []byte(`{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://example.invalid/schemas/root.schema.json",
  "type": "object",
  "properties": {
    "implementation": {
      "$ref": "https://example.invalid/schemas/types.schema.json#/$defs/slug"
    }
  },
  "required": ["implementation"],
  "additionalProperties": false
}`)
	if err := os.WriteFile(filepath.Join(schemasDir, "root.schema.json"), rootSchema, 0o644); err != nil {
		t.Fatalf("write root schema: %v", err)
	}

	examplesDir := filepath.Join(repo, "examples")
	if err := os.MkdirAll(examplesDir, 0o755); err != nil {
		t.Fatalf("mkdir examples: %v", err)
	}
	if err := os.WriteFile(filepath.Join(examplesDir, "good.yaml"), []byte("implementation: nextcloud\n"), 0o644); err != nil {
		t.Fatalf("write good example: %v", err)
	}

	out, err := execRoot(t, []string{
		"validate", "suite",
		"--data", examplesDir,
		"--manifest", "schema-mappings.yaml",
		"--ref-dir", schemasDir,
		"--format", "json",
	})
	if err != nil {
		t.Fatalf("expected suite to pass, got error: %v\n%s", err, out)
	}

	var res validateSuiteResult
	if uerr := json.Unmarshal([]byte(out), &res); uerr != nil {
		t.Fatalf("expected JSON output, got parse error: %v\n%s", uerr, out)
	}
	if res.Summary.Passed != 1 {
		t.Fatalf("expected passed=1, got %d", res.Summary.Passed)
	}
}

func TestValidateSuite_JSON_ResolvesCanonicalSchemaIDFromRefDir(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("create fake .git: %v", err)
	}

	canonicalID := "https://example.invalid/schemas/root.schema.json"

	manifestPath := filepath.Join(repo, "schema-mappings.yaml")
	if err := os.WriteFile(manifestPath, []byte(`version: "1.0.0"
mappings:
  - pattern: "**/*.yaml"
    schema_id: "`+canonicalID+`"
    source: external
`), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	schemasDir := filepath.Join(repo, "schemas")
	if err := os.MkdirAll(schemasDir, 0o755); err != nil {
		t.Fatalf("mkdir schemas: %v", err)
	}

	refSchema := []byte(`{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://example.invalid/schemas/types.schema.json",
  "$defs": {
    "slug": {
      "type": "string",
      "pattern": "^[a-z0-9-]+$"
    }
  }
}`)
	if err := os.WriteFile(filepath.Join(schemasDir, "types.schema.json"), refSchema, 0o644); err != nil {
		t.Fatalf("write ref schema: %v", err)
	}

	rootSchema := []byte(`{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "` + canonicalID + `",
  "type": "object",
  "properties": {
    "implementation": {
      "$ref": "https://example.invalid/schemas/types.schema.json#/$defs/slug"
    }
  },
  "required": ["implementation"],
  "additionalProperties": false
}`)
	if err := os.WriteFile(filepath.Join(schemasDir, "root.schema.json"), rootSchema, 0o644); err != nil {
		t.Fatalf("write root schema: %v", err)
	}

	examplesDir := filepath.Join(repo, "examples")
	if err := os.MkdirAll(examplesDir, 0o755); err != nil {
		t.Fatalf("mkdir examples: %v", err)
	}
	if err := os.WriteFile(filepath.Join(examplesDir, "good.yaml"), []byte("implementation: nextcloud\n"), 0o644); err != nil {
		t.Fatalf("write good example: %v", err)
	}

	out, err := execRoot(t, []string{
		"validate", "suite",
		"--data", examplesDir,
		"--manifest", "schema-mappings.yaml",
		"--ref-dir", schemasDir,
		"--format", "json",
	})
	if err != nil {
		t.Fatalf("expected suite to pass, got error: %v\n%s", err, out)
	}

	var res validateSuiteResult
	if uerr := json.Unmarshal([]byte(out), &res); uerr != nil {
		t.Fatalf("expected JSON output, got parse error: %v\n%s", uerr, out)
	}
	if res.Summary.Passed != 1 {
		t.Fatalf("expected passed=1, got %d", res.Summary.Passed)
	}
	if len(res.Files) != 1 || res.Files[0].Schema == nil {
		t.Fatalf("expected schema info in output")
	}
	if res.Files[0].Schema.ID != canonicalID {
		t.Fatalf("expected schema id %q, got %q", canonicalID, res.Files[0].Schema.ID)
	}
	if !strings.Contains(res.Files[0].Schema.Path, "root.schema.json") {
		t.Fatalf("expected schema.path to include root.schema.json, got %q", res.Files[0].Schema.Path)
	}
}
