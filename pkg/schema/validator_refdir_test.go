package schema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateFromBytesWithRefDirs_ResolvesRemoteRef(t *testing.T) {
	t.Parallel()

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

	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "types.schema.json"), refSchema, 0o644); err != nil {
		t.Fatalf("failed to write ref schema: %v", err)
	}

	data := map[string]any{"implementation": "nextcloud"}
	res, err := ValidateFromBytesWithRefDirs(rootSchema, data, []string{tmpDir})
	if err != nil {
		t.Fatalf("ValidateFromBytesWithRefDirs error: %v", err)
	}
	if !res.Valid {
		t.Fatalf("expected valid, got errors: %+v", res.Errors)
	}
}

func TestValidateFromBytesWithRefDirs_SkipsRootSchemaInRefDir(t *testing.T) {
	t.Parallel()

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

	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "types.schema.json"), refSchema, 0o644); err != nil {
		t.Fatalf("failed to write ref schema: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "root.schema.json"), rootSchema, 0o644); err != nil {
		t.Fatalf("failed to write root schema: %v", err)
	}

	data := map[string]any{"implementation": "nextcloud"}
	res, err := ValidateFromBytesWithRefDirs(rootSchema, data, []string{tmpDir})
	if err != nil {
		t.Fatalf("ValidateFromBytesWithRefDirs error: %v", err)
	}
	if !res.Valid {
		t.Fatalf("expected valid, got errors: %+v", res.Errors)
	}
}

func TestValidateFromBytesWithRefDirs_DuplicateIDConflictErrors(t *testing.T) {
	t.Parallel()

	rootSchema := []byte(`{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://example.invalid/schemas/root.schema.json",
  "type": "object",
  "properties": {
    "implementation": {"type": "string"}
  },
  "required": ["implementation"],
  "additionalProperties": false
}`)

	conflictingRootSchema := []byte(`{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://example.invalid/schemas/root.schema.json",
  "type": "object",
  "properties": {
    "implementation": {"type": "integer"}
  },
  "required": ["implementation"],
  "additionalProperties": false
}`)

	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "root.schema.json"), conflictingRootSchema, 0o644); err != nil {
		t.Fatalf("failed to write conflicting root schema: %v", err)
	}

	data := map[string]any{"implementation": "nextcloud"}
	_, err := ValidateFromBytesWithRefDirs(rootSchema, data, []string{tmpDir})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate schema $id") {
		t.Fatalf("expected duplicate id error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "https://example.invalid/schemas/root.schema.json") {
		t.Fatalf("expected error to mention schema id, got: %v", err)
	}
}

func TestValidateFromBytesWithRefDirs_DuplicateIdenticalSkips(t *testing.T) {
	t.Parallel()

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

	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "types.schema.json"), refSchema, 0o644); err != nil {
		t.Fatalf("failed to write ref schema: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "types-copy.schema.json"), refSchema, 0o644); err != nil {
		t.Fatalf("failed to write ref schema copy: %v", err)
	}

	data := map[string]any{"implementation": "nextcloud"}
	res, err := ValidateFromBytesWithRefDirs(rootSchema, data, []string{tmpDir})
	if err != nil {
		t.Fatalf("ValidateFromBytesWithRefDirs error: %v", err)
	}
	if !res.Valid {
		t.Fatalf("expected valid, got errors: %+v", res.Errors)
	}
}
