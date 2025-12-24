package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateData_ResolvesCanonicalSchemaIDFromRefDir(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("create fake .git: %v", err)
	}

	canonicalID := "https://example.invalid/schemas/root.schema.json"

	schemasDir := filepath.Join(repo, "schemas")
	if err := os.MkdirAll(schemasDir, 0o755); err != nil {
		t.Fatalf("mkdir schemas: %v", err)
	}

	rootSchema := []byte(`{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "` + canonicalID + `",
  "type": "object",
  "properties": {
    "implementation": {"type": "string"}
  },
  "required": ["implementation"],
  "additionalProperties": false
}`)
	if err := os.WriteFile(filepath.Join(schemasDir, "root.schema.json"), rootSchema, 0o644); err != nil {
		t.Fatalf("write root schema: %v", err)
	}

	dataPath := filepath.Join(repo, "good.yaml")
	if err := os.WriteFile(dataPath, []byte("implementation: nextcloud\n"), 0o644); err != nil {
		t.Fatalf("write data: %v", err)
	}

	out, err := execRoot(t, []string{
		"validate", "data",
		"--schema", canonicalID,
		"--data", dataPath,
		"--ref-dir", schemasDir,
		"--format", "json",
	})
	if err != nil {
		t.Fatalf("expected validation to pass, got error: %v\n%s", err, out)
	}

	var res struct {
		Valid bool `json:"valid"`
	}
	if uerr := json.Unmarshal([]byte(out), &res); uerr != nil {
		t.Fatalf("expected JSON output, got parse error: %v\n%s", uerr, out)
	}
	if !res.Valid {
		t.Fatalf("expected valid=true")
	}
}
