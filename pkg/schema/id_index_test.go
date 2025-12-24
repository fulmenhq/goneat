package schema

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildIDIndexFromRefDirs_DetectsConflicts(t *testing.T) {
	t.Parallel()

	id := "https://example.invalid/schemas/root.schema.json"

	schemaA := []byte(`{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "` + id + `",
  "type": "object",
  "properties": {"a": {"type": "string"}}
}`)

	schemaB := []byte(`{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "` + id + `",
  "type": "object",
  "properties": {"a": {"type": "integer"}}
}`)

	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "a.schema.json"), schemaA, 0o644); err != nil {
		t.Fatalf("write schema a: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "b.schema.json"), schemaB, 0o644); err != nil {
		t.Fatalf("write schema b: %v", err)
	}

	_, err := BuildIDIndexFromRefDirs([]string{tmp})
	if err == nil {
		t.Fatalf("expected conflict error")
	}
}

func TestBuildIDIndexFromRefDirs_AllowsIdenticalDuplicates(t *testing.T) {
	t.Parallel()

	id := "https://example.invalid/schemas/root.schema.json"
	schemaA := []byte(`{"$schema":"https://json-schema.org/draft/2020-12/schema","$id":"` + id + `","type":"object"}`)

	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "a.schema.json"), schemaA, 0o644); err != nil {
		t.Fatalf("write schema a: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "b.schema.json"), schemaA, 0o644); err != nil {
		t.Fatalf("write schema b: %v", err)
	}

	idx, err := BuildIDIndexFromRefDirs([]string{tmp})
	if err != nil {
		t.Fatalf("BuildIDIndexFromRefDirs error: %v", err)
	}
	if idx.Len() != 1 {
		t.Fatalf("expected 1 entry, got %d", idx.Len())
	}
}
