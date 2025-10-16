package managers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPythonManager_Name(t *testing.T) {
	manager := NewPythonManager()
	if manager.Name() != "pyproject.toml" {
		t.Errorf("Expected name 'pyproject.toml', got '%s'", manager.Name())
	}
}

func TestPythonManager_ExtractVersion_ProjectSection(t *testing.T) {
	content := `[project]
name = "test-package"
version = "1.2.3"
description = "Test package"

[tool.poetry]
name = "test-package"
version = "1.2.3"
description = "Test package"
`

	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "pyproject.toml")
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	manager := NewPythonManager()
	version, err := manager.ExtractVersion(file)
	if err != nil {
		t.Fatalf("ExtractVersion failed: %v", err)
	}

	if version != "1.2.3" {
		t.Errorf("Expected version '1.2.3', got '%s'", version)
	}
}

func TestPythonManager_ExtractVersion_PoetrySection(t *testing.T) {
	content := `[tool.poetry]
name = "test-package"
version = "2.0.0"
description = "Test package"
`

	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "pyproject.toml")
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	manager := NewPythonManager()
	version, err := manager.ExtractVersion(file)
	if err != nil {
		t.Fatalf("ExtractVersion failed: %v", err)
	}

	if version != "2.0.0" {
		t.Errorf("Expected version '2.0.0', got '%s'", version)
	}
}

func TestPythonManager_ExtractVersion_NoVersion(t *testing.T) {
	content := `[project]
name = "test-package"
description = "Test package"
`

	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "pyproject.toml")
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	manager := NewPythonManager()
	_, err := manager.ExtractVersion(file)
	if err == nil {
		t.Error("Expected error for missing version, got nil")
	}
}

func TestPythonManager_UpdateVersion_PreserveStructure(t *testing.T) {
	testCases := []struct {
		name       string
		fixture    string
		newVersion string
		checks     []string // strings that must be present in result
	}{
		{
			name:       "project section with comments",
			fixture:    "../../../tests/fixtures/pyproject-toml/project-section.toml",
			newVersion: "2.0.0",
			checks: []string{
				`version = "2.0.0"`,
				"# This is a comment that should be preserved",
				"[tool.black]",
				"[build-system]",
			},
		},
		{
			name:       "poetry section",
			fixture:    "../../../tests/fixtures/pyproject-toml/poetry-section.toml",
			newVersion: "2.1.0",
			checks: []string{
				`version = "2.1.0"`,
				"# Poetry-based project configuration",
				"[tool.black]",
				"[build-system]",
			},
		},
		{
			name:       "both sections",
			fixture:    "../../../tests/fixtures/pyproject-toml/both-sections.toml",
			newVersion: "1.6.0",
			checks: []string{
				`version = "1.6.0"`,
				"# Project with both [project] and [tool.poetry] sections",
				"[tool.poetry]",
				"[build-system]",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fixtureData, err := os.ReadFile(tc.fixture)
			if err != nil {
				t.Fatalf("Failed to read fixture %s: %v", tc.fixture, err)
			}

			tmpDir := t.TempDir()
			file := filepath.Join(tmpDir, "pyproject.toml")
			if err := os.WriteFile(file, fixtureData, 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			manager := NewPythonManager()
			err = manager.UpdateVersion(file, tc.newVersion)
			if err != nil {
				t.Fatalf("UpdateVersion failed: %v", err)
			}

			updatedContent, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("Failed to read updated file: %v", err)
			}

			contentStr := string(updatedContent)

			// Verify all required strings are present
			for _, check := range tc.checks {
				if !strings.Contains(contentStr, check) {
					t.Errorf("Required content missing: %s", check)
				}
			}

			// Verify original structure is preserved (basic check)
			lines := strings.Split(string(fixtureData), "\n")
			updatedLines := strings.Split(contentStr, "\n")

			// Should have similar number of lines (allowing for minor differences)
			if len(updatedLines) < len(lines)-2 || len(updatedLines) > len(lines)+2 {
				t.Errorf("Structure significantly changed: %d lines -> %d lines", len(lines), len(updatedLines))
			}
		})
	}
}

func TestPythonManager_ValidateVersion(t *testing.T) {
	content := `[project]
name = "test-package"
version = "1.2.3"
description = "Test package"
`

	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "pyproject.toml")
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	manager := NewPythonManager()

	// Valid version
	err := manager.ValidateVersion(file, "1.2.3")
	if err != nil {
		t.Errorf("ValidateVersion failed for correct version: %v", err)
	}

	// Invalid version
	err = manager.ValidateVersion(file, "2.0.0")
	if err == nil {
		t.Error("Expected validation error for mismatched version, got nil")
	}
}
