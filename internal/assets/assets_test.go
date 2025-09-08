package assets

import (
	"io/fs"
	"testing"
)

func TestGetTemplatesFS(t *testing.T) {
	fsys := GetTemplatesFS()
	if fsys == nil {
		t.Fatal("GetTemplatesFS returned nil")
	}

	// Test reading a known template file
	data, err := fs.ReadFile(fsys, "hooks/bash/pre-commit.sh.tmpl")
	if err != nil {
		t.Fatalf("Failed to read pre-commit template: %v", err)
	}
	if len(data) == 0 {
		t.Error("Pre-commit template is empty")
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
