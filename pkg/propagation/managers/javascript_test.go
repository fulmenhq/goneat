package managers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJavaScriptManager_Name(t *testing.T) {
	manager := NewJavaScriptManager()
	if manager.Name() != "package.json" {
		t.Errorf("Expected name 'package.json', got '%s'", manager.Name())
	}
}

func TestJavaScriptManager_ExtractVersion(t *testing.T) {
	content := `{
  "name": "test-package",
  "version": "1.2.3",
  "description": "Test package",
  "dependencies": {
    "some-dep": "^1.0.0"
  }
}`

	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "package.json")
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	manager := NewJavaScriptManager()
	version, err := manager.ExtractVersion(file)
	if err != nil {
		t.Fatalf("ExtractVersion failed: %v", err)
	}

	if version != "1.2.3" {
		t.Errorf("Expected version '1.2.3', got '%s'", version)
	}
}

func TestJavaScriptManager_ExtractVersion_NoVersion(t *testing.T) {
	content := `{
  "name": "test-package",
  "description": "Test package"
}`

	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "package.json")
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	manager := NewJavaScriptManager()
	_, err := manager.ExtractVersion(file)
	if err == nil {
		t.Error("Expected error for missing version, got nil")
	}
}

func TestJavaScriptManager_UpdateVersion(t *testing.T) {
	originalContent := `{
  "name": "test-package",
  "version": "1.2.3",
  "description": "Test package",
  "dependencies": {
    "some-dep": "^1.0.0"
  }
}`

	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "package.json")
	if err := os.WriteFile(file, []byte(originalContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	manager := NewJavaScriptManager()
	err := manager.UpdateVersion(file, "2.0.0")
	if err != nil {
		t.Fatalf("UpdateVersion failed: %v", err)
	}

	// Read back and verify
	updatedContent, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("Failed to read updated file: %v", err)
	}

	// Check that version was updated
	if !strings.Contains(string(updatedContent), `"version": "2.0.0"`) {
		t.Error("Version was not updated correctly")
	}

	// Check that other fields are preserved
	if !strings.Contains(string(updatedContent), `"name": "test-package"`) {
		t.Error("Name field was not preserved")
	}
	if !strings.Contains(string(updatedContent), `"some-dep": "^1.0.0"`) {
		t.Error("Dependencies were not preserved")
	}
}

func TestJavaScriptManager_ValidateVersion(t *testing.T) {
	content := `{
  "name": "test-package",
  "version": "1.2.3",
  "description": "Test package"
}`

	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "package.json")
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	manager := NewJavaScriptManager()

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

func TestJavaScriptManager_Detect(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	files := []string{
		"package.json",
		"subdir/package.json",
		"other/file.txt",
	}

	for _, file := range files {
		fullPath := filepath.Join(tmpDir, file)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte("{}"), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
	}

	manager := NewJavaScriptManager()
	detected, err := manager.Detect(tmpDir)
	if err != nil {
		t.Fatalf("Detect failed: %v", err)
	}

	// Managers now return relative paths from the root
	expected := []string{
		"package.json",
		"subdir/package.json",
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
