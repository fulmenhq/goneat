package schema

import (
	"os"
	"path/filepath"
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
