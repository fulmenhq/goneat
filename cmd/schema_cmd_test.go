package cmd

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSchemaValidateSchema_GoodDraft07(t *testing.T) {
	out, err := execRoot(t, []string{
		"schema", "validate-schema",
		"--schema-id", "json-schema-draft-07",
		"tests/fixtures/schemas/draft-07/good/good-config.json",
	})
	if err != nil {
		t.Fatalf("schema validate-schema (good) failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "✅") {
		t.Fatalf("expected success marker in output, got: %s", out)
	}
}

func TestSchemaValidateSchema_BadDraft07(t *testing.T) {
	out, err := execRoot(t, []string{
		"schema", "validate-schema",
		"--schema-id", "json-schema-draft-07",
		"tests/fixtures/schemas/draft-07/bad/bad-required-wrong.yaml",
	})
	if err == nil {
		t.Fatalf("expected schema validate-schema to fail for invalid schema\n%s", out)
	}
	if !strings.Contains(out, "❌") {
		t.Fatalf("expected failure marker in output, got: %s", out)
	}
}

func TestSchemaValidateSchema_GoodDraft2020_12(t *testing.T) {
	out, err := execRoot(t, []string{
		"schema", "validate-schema",
		"--schema-id", "json-schema-2020-12",
		"tests/fixtures/schemas/draft-2020-12/good/simple-object.json",
	})
	if err != nil {
		t.Fatalf("schema validate-schema (good draft-2020-12) failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "✅") {
		t.Fatalf("expected success marker in output, got: %s", out)
	}
}

func TestSchemaValidateSchema_BadDraft2020_12(t *testing.T) {
	out, err := execRoot(t, []string{
		"schema", "validate-schema",
		"--schema-id", "json-schema-2020-12",
		"tests/fixtures/schemas/draft-2020-12/bad/invalid-type.json",
	})
	if err == nil {
		t.Fatalf("expected schema validate-schema to fail for invalid draft-2020-12 schema\n%s", out)
	}
	if !strings.Contains(out, "❌") {
		t.Fatalf("expected failure marker in output, got: %s", out)
	}
}

func TestSchemaValidateSchema_AutoDetect(t *testing.T) {
	out, err := execRoot(t, []string{
		"schema", "validate-schema",
		"tests/fixtures/schemas/draft-07/good/good-config.json",
	})
	if err != nil {
		t.Fatalf("schema validate-schema (auto-detect) failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "✅") {
		t.Fatalf("expected success marker in output, got: %s", out)
	}
	if !strings.Contains(out, "json-schema-2020-12") {
		t.Fatalf("expected auto-detected schema id in output, got: %s", out)
	}
}

func TestSchemaValidateSchema_JSONOutput(t *testing.T) {
	out, err := execRoot(t, []string{
		"schema", "validate-schema",
		"--format", "json",
		"--schema-id", "json-schema-draft-07",
		"tests/fixtures/schemas/draft-07/good/good-config.json",
	})
	if err != nil {
		t.Fatalf("schema validate-schema (JSON output) failed: %v\n%s", err, out)
	}
	// Check if output is valid JSON array
	var results []map[string]interface{}
	if err := json.Unmarshal([]byte(out), &results); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if valid, ok := results[0]["valid"].(bool); !ok || !valid {
		t.Fatalf("expected valid=true in JSON output, got: %s", out)
	}
}

func TestSchemaValidateSchema_GlobExpansion(t *testing.T) {
	out, err := execRoot(t, []string{
		"schema", "validate-schema",
		"--format", "text",
		"--schema-id", "json-schema-draft-07",
		"tests/fixtures/schemas/draft-07/good/*.json",
	})
	if err != nil {
		t.Fatalf("schema validate-schema (glob) failed: %v\n%s", err, out)
	}
	if count := strings.Count(out, "✅"); count < 2 {
		t.Fatalf("expected multiple validated schemas, got: %s", out)
	}
}

func TestSchemaValidateSchema_DirectoryRecursive(t *testing.T) {
	out, err := execRoot(t, []string{
		"schema", "validate-schema",
		"--format", "text",
		"--recursive",
		"--schema-id", "json-schema-draft-07",
		"tests/fixtures/schemas/draft-07/good",
	})
	if err != nil {
		t.Fatalf("schema validate-schema (directory recursive) failed: %v\n%s", err, out)
	}
	if count := strings.Count(out, "✅"); count < 2 {
		t.Fatalf("expected multiple validated schemas, got: %s", out)
	}
}
