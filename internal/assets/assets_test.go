package assets

import (
	"bytes"
	"io/fs"
	"testing"
)

func TestGetTemplatesFS(t *testing.T) {
	fsys := GetTemplatesFS()
	if fsys == nil {
		t.Fatal("GetTemplatesFS returned nil")
	}

	// Test reading a known template file
	data, err := fs.ReadFile(fsys, "templates/hooks/bash/pre-commit.sh.tmpl")
	if err != nil {
		t.Fatalf("Failed to read pre-commit template: %v", err)
	}
	if len(data) == 0 {
		t.Error("Pre-commit template is empty")
	}
	if bytes.Contains(data, []byte("passed!\"}")) || bytes.Contains(data, []byte("passed!}")) {
		t.Fatalf("pre-commit template contains unexpected trailing brace")
	}
	if !bytes.Contains(data, []byte("set -f")) {
		t.Fatalf("pre-commit template should disable glob expansion (set -f)")
	}
}

func TestGetSchemasFS(t *testing.T) {
	fsys := GetSchemasFS()
	if fsys == nil {
		t.Fatal("GetSchemasFS returned nil")
	}

	// Test reading a known schema file
	data, err := fs.ReadFile(fsys, "config/goneat-config-v1.0.0.yaml")
	if err != nil {
		t.Fatalf("Failed to read schema: %v", err)
	}
	if len(data) == 0 {
		t.Error("Schema file is empty")
	}
}

func TestGetJSONSchemaMeta(t *testing.T) {
	tests := []struct {
		draft string
		want  bool
	}{
		{"draft-07", true},
		{"07", true},
		{"2020-12", true},
		{"2020", true},
		{"invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.draft, func(t *testing.T) {
			data, ok := GetJSONSchemaMeta(tt.draft)
			if ok != tt.want {
				t.Errorf("GetJSONSchemaMeta(%q) ok = %v; want %v", tt.draft, ok, tt.want)
			}
			if ok && len(data) == 0 {
				t.Error("Returned empty data when ok=true")
			}
		})
	}
}

func TestOfflineMetaSchemaEmbedding(t *testing.T) {
	path := "embedded_schemas/schemas/meta/draft-2020-12/offline.schema.json"
	data, ok := GetSchema(path)
	if !ok {
		t.Fatalf("offline meta-schema not embedded at %s", path)
	}
	if len(data) == 0 {
		t.Fatalf("offline meta-schema %s is empty", path)
	}

	t.Setenv("GONEAT_OFFLINE_SCHEMA_VALIDATION", "true")
	meta, ok := GetJSONSchemaMeta("2020-12")
	if !ok {
		t.Fatal("expected offline meta-schema lookup to succeed for draft 2020-12")
	}
	if !bytes.Equal(meta, data) {
		t.Fatal("offline meta-schema returned by GetJSONSchemaMeta does not match embedded copy")
	}
}

func TestGetDocsFS(t *testing.T) {
	fsys := GetDocsFS()
	if fsys == nil {
		t.Fatal("GetDocsFS returned nil")
	}
	// Try a few curated files
	candidates := []string{
		"docs/user-guide/install.md",
		"docs/configuration/feature-gates.md",
	}
	var found bool
	for _, p := range candidates {
		data, err := fs.ReadFile(fsys, p)
		if err == nil && len(data) > 0 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("no embedded docs found at expected paths: %v", candidates)
	}
}

func TestGetConfigFS(t *testing.T) {
	fsys := GetConfigFS()
	if fsys == nil {
		t.Fatal("GetConfigFS returned nil")
	}

	// Test reading a known config file
	data, err := fs.ReadFile(fsys, "config/ascii/terminal-overrides.yaml")
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}
	if len(data) == 0 {
		t.Error("Config file is empty")
	}
}

func TestGetEmbeddedAsset(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantData bool
	}{
		{"valid template", "embedded_templates/templates/hooks/bash/pre-commit.sh.tmpl", true},
		{"valid schema", "embedded_schemas/config/goneat-config-v1.0.0.yaml", true},
		{"invalid path", "nonexistent/file.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := GetEmbeddedAsset(tt.path)
			if tt.wantData {
				if err != nil {
					t.Errorf("GetEmbeddedAsset(%q) error = %v; want nil", tt.path, err)
				}
				if len(data) == 0 {
					t.Errorf("GetEmbeddedAsset(%q) returned empty data", tt.path)
				}
			} else {
				if err == nil {
					t.Errorf("GetEmbeddedAsset(%q) error = nil; want error", tt.path)
				}
			}
		})
	}
}

func TestGetAsset(t *testing.T) {
	tests := []struct {
		name     string
		asset    string
		wantData bool
	}{
		{"terminal-overrides", "terminal-overrides.yaml", true},
		{"unknown asset", "unknown.yaml", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, ok := GetAsset(tt.asset)
			if ok != tt.wantData {
				t.Errorf("GetAsset(%q) ok = %v; want %v", tt.asset, ok, tt.wantData)
			}
			if ok && len(data) == 0 {
				t.Errorf("GetAsset(%q) returned empty data when ok=true", tt.asset)
			}
		})
	}
}

func TestGetSchema(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantData bool
	}{
		{"valid schema", "embedded_schemas/config/goneat-config-v1.0.0.yaml", true},
		{"invalid path", "nonexistent/schema.yaml", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, ok := GetSchema(tt.path)
			if ok != tt.wantData {
				t.Errorf("GetSchema(%q) ok = %v; want %v", tt.path, ok, tt.wantData)
			}
			if ok && len(data) == 0 {
				t.Errorf("GetSchema(%q) returned empty data when ok=true", tt.path)
			}
		})
	}
}

func TestGetSchemaNames(t *testing.T) {
	schemas := GetSchemaNames()
	if len(schemas) == 0 {
		t.Fatal("GetSchemaNames returned empty list")
	}

	// Verify each schema has required fields and exists
	for _, schema := range schemas {
		if schema.Name == "" {
			t.Error("Schema has empty name")
		}
		if schema.Path == "" {
			t.Error("Schema has empty path")
		}
		if schema.Draft == "" {
			t.Error("Schema has empty draft")
		}

		// Verify the schema actually exists
		if _, ok := GetSchema(schema.Path); !ok {
			t.Errorf("Schema %q references non-existent path %q", schema.Name, schema.Path)
		}
	}
}

func TestDetectDraft(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{"draft-07 schema", "embedded_schemas/schemas/meta/draft-07/schema.json", "Draft-07"},
		{"draft-2020-12 schema", "embedded_schemas/schemas/meta/draft-2020-12/schema.json", "Draft-2020-12"},
		{"invalid path", "nonexistent.json", "Unknown (07/2020-12 supported)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectDraft(tt.path)
			if result != tt.expected {
				t.Errorf("detectDraft(%q) = %q; want %q", tt.path, result, tt.expected)
			}
		})
	}
}
