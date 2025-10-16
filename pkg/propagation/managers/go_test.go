package managers

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGoManager_Name(t *testing.T) {
	manager := NewGoManager()
	if manager.Name() != "go.mod" {
		t.Errorf("Expected name 'go.mod', got '%s'", manager.Name())
	}
}

func TestGoManager_ExtractVersion(t *testing.T) {
	content := `module github.com/example/myproject

go 1.21

require (
	github.com/some/dep v1.0.0
)
`

	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	manager := NewGoManager()
	moduleName, err := manager.ExtractVersion(file)
	if err != nil {
		t.Fatalf("ExtractVersion failed: %v", err)
	}

	expected := "github.com/example/myproject"
	if moduleName != expected {
		t.Errorf("Expected module name '%s', got '%s'", expected, moduleName)
	}
}

func TestGoManager_ExtractVersion_NoModule(t *testing.T) {
	content := `go 1.21

require (
	github.com/some/dep v1.0.0
)
`

	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	manager := NewGoManager()
	_, err := manager.ExtractVersion(file)
	if err == nil {
		t.Error("Expected error for missing module directive, got nil")
	}
}

func TestGoManager_UpdateVersion_NotSupported(t *testing.T) {
	content := `module github.com/example/myproject

go 1.21
`

	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	manager := NewGoManager()
	err := manager.UpdateVersion(file, "2.0.0")
	if err == nil {
		t.Error("Expected error for unsupported UpdateVersion, got nil")
	}
}

func TestGoManager_ValidateVersion(t *testing.T) {
	content := `module github.com/example/myproject

go 1.21
`

	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	manager := NewGoManager()

	// Valid version (basic validation - just checks file is parseable)
	err := manager.ValidateVersion(file, "1.2.3")
	if err != nil {
		t.Errorf("ValidateVersion failed: %v", err)
	}
}

func TestGoManager_ValidateVersion_WithVersionedModule(t *testing.T) {
	content := `module github.com/example/myproject/v2

go 1.21
`

	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	manager := NewGoManager()

	// Should validate without error (version 2 matches major version 2)
	err := manager.ValidateVersion(file, "2.0.0")
	if err != nil {
		t.Errorf("ValidateVersion failed for matching major version: %v", err)
	}
}

func TestGoManager_Detect(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	files := []string{
		"go.mod",
		"subdir/go.mod",
		"other/file.txt",
	}

	for _, file := range files {
		fullPath := filepath.Join(tmpDir, file)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		content := "module test\n\ngo 1.21\n"
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
	}

	manager := NewGoManager()
	detected, err := manager.Detect(tmpDir)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}

	// Managers now return relative paths from the root
	expected := []string{
		"go.mod",
		"subdir/go.mod",
	}

	if len(detected) != len(expected) {
		t.Errorf("Expected %d files, got %d", len(expected), len(detected))
		t.Logf("Detected: %v", detected)
	}

	for _, expectedFile := range expected {
		found := false
		for _, detectedFile := range detected {
			// Normalize paths for comparison (handle OS-specific separators)
			normalizedDetected := filepath.ToSlash(detectedFile)
			normalizedExpected := filepath.ToSlash(expectedFile)
			if normalizedDetected == normalizedExpected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected file not found: %s (detected: %v)", expectedFile, detected)
		}
	}
}
