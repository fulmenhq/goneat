package safeio

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanUserPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		hasError bool
	}{
		{
			name:     "simple path",
			input:    "file.txt",
			expected: "file.txt",
			hasError: false,
		},
		{
			name:     "relative path",
			input:    "./subdir/file.txt",
			expected: "subdir/file.txt",
			hasError: false,
		},
		{
			name:     "absolute path",
			input:    "/tmp/file.txt",
			expected: "/tmp/file.txt",
			hasError: false,
		},
		{
			name:     "path with traversal",
			input:    "../../../etc/passwd",
			expected: "",
			hasError: true,
		},
		{
			name:     "path with traversal in middle",
			input:    "valid/../../../etc/passwd",
			expected: "",
			hasError: true,
		},
		{
			name:     "path with dots but no traversal",
			input:    "file.with.dots.txt",
			expected: "file.with.dots.txt",
			hasError: false,
		},
		{
			name:     "empty path",
			input:    "",
			expected: ".",
			hasError: false,
		},
		{
			name:     "current directory",
			input:    ".",
			expected: ".",
			hasError: false,
		},
		{
			name:     "parent directory",
			input:    "..",
			expected: "",
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := CleanUserPath(tt.input)

			if tt.hasError {
				if err == nil {
					t.Errorf("CleanUserPath(%q) expected error but got none", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("CleanUserPath(%q) unexpected error: %v", tt.input, err)
				}
				if result != tt.expected {
					t.Errorf("CleanUserPath(%q) = %q, expected %q", tt.input, result, tt.expected)
				}
			}
		})
	}
}

func TestWriteFilePreservePerms(t *testing.T) {
	// Create a temporary file for testing
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")
	testData := []byte("test data for safeio")

	// Test writing to non-existent file (should use default 0644)
	err := WriteFilePreservePerms(testFile, testData)
	if err != nil {
		t.Fatalf("WriteFilePreservePerms() failed for new file: %v", err)
	}

	// Verify file was created with correct content
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}
	if string(content) != string(testData) {
		t.Errorf("File content mismatch: got %q, expected %q", string(content), string(testData))
	}

	// Check file permissions (should be readable/writable by owner, readable by others)
	stat, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("Failed to stat test file: %v", err)
	}
	mode := stat.Mode()
	expectedMode := os.FileMode(0o644)
	if mode.Perm() != expectedMode {
		t.Errorf("File permissions: got %s, expected %s", mode.Perm(), expectedMode)
	}
}

func TestWriteFilePreservePermsExisting(t *testing.T) {
	// Create a temporary file with specific permissions
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")

	// Create file with specific permissions
	initialData := []byte("initial data")
	err := os.WriteFile(testFile, initialData, 0o755)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Verify initial permissions
	stat, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("Failed to stat test file: %v", err)
	}
	initialMode := stat.Mode()

	// Write new data using WriteFilePreservePerms
	newData := []byte("new data for safeio")
	err = WriteFilePreservePerms(testFile, newData)
	if err != nil {
		t.Fatalf("WriteFilePreservePerms() failed for existing file: %v", err)
	}

	// Verify content was updated
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}
	if string(content) != string(newData) {
		t.Errorf("File content mismatch: got %q, expected %q", string(content), string(newData))
	}

	// Verify permissions were preserved
	stat, err = os.Stat(testFile)
	if err != nil {
		t.Fatalf("Failed to stat test file after write: %v", err)
	}
	finalMode := stat.Mode()
	if finalMode != initialMode {
		t.Errorf("File permissions changed: was %s, now %s", initialMode, finalMode)
	}
}

func TestWriteFilePreservePermsError(t *testing.T) {
	// Test writing to a directory that doesn't exist
	nonExistentDir := "/non/existent/directory/file.txt"
	testData := []byte("test data")

	err := WriteFilePreservePerms(nonExistentDir, testData)
	if err == nil {
		t.Error("WriteFilePreservePerms() should fail for non-existent directory")
	}
}

func TestReadFileContained(t *testing.T) {
	// Create temporary directory structure for testing
	tempDir := t.TempDir()

	// Create a subdirectory and a file inside it
	subDir := filepath.Join(tempDir, "subdir")
	err := os.MkdirAll(subDir, 0o755)
	if err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	testFile := filepath.Join(subDir, "test.txt")
	testData := []byte("test data for safe reading")
	err = os.WriteFile(testFile, testData, 0o644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create a file outside the base directory for traversal tests
	outsideFile := filepath.Join(filepath.Dir(tempDir), "outside.txt")
	outsideData := []byte("outside data")
	err = os.WriteFile(outsideFile, outsideData, 0o644)
	if err != nil {
		t.Fatalf("Failed to create outside file: %v", err)
	}
	defer func() {
		if err := os.Remove(outsideFile); err != nil {
			t.Logf("Warning: failed to remove outside file: %v", err)
		}
	}()

	tests := []struct {
		name      string
		baseDir   string
		filePath  string
		wantError bool
		wantData  []byte
	}{
		{
			name:      "file within baseDir",
			baseDir:   tempDir,
			filePath:  testFile,
			wantError: false,
			wantData:  testData,
		},
		{
			name:      "file in subdirectory",
			baseDir:   tempDir,
			filePath:  filepath.Join(tempDir, "subdir", "test.txt"),
			wantError: false,
			wantData:  testData,
		},
		{
			name:      "path traversal attempt",
			baseDir:   subDir,
			filePath:  filepath.Join(subDir, "..", "..", "outside.txt"),
			wantError: true,
		},
		{
			name:      "file outside baseDir",
			baseDir:   tempDir,
			filePath:  outsideFile,
			wantError: true,
		},
		{
			name:      "non-existent file within baseDir",
			baseDir:   tempDir,
			filePath:  filepath.Join(tempDir, "nonexistent.txt"),
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := ReadFileContained(tt.baseDir, tt.filePath)

			if tt.wantError {
				if err == nil {
					t.Errorf("ReadFileContained(%q, %q) expected error but got none", tt.baseDir, tt.filePath)
				}
			} else {
				if err != nil {
					t.Errorf("ReadFileContained(%q, %q) unexpected error: %v", tt.baseDir, tt.filePath, err)
				}
				if string(data) != string(tt.wantData) {
					t.Errorf("ReadFileContained(%q, %q) = %q, expected %q", tt.baseDir, tt.filePath, string(data), string(tt.wantData))
				}
			}
		})
	}
}

func TestIntegration(t *testing.T) {
	// Test that CleanUserPath and WriteFilePreservePerms work together
	tempDir := t.TempDir()

	// Clean a user path
	userPath := "subdir/file.txt"
	cleanPath, err := CleanUserPath(userPath)
	if err != nil {
		t.Fatalf("CleanUserPath() failed: %v", err)
	}

	// Create full path
	fullPath := filepath.Join(tempDir, cleanPath)

	// Ensure parent directory exists
	parentDir := filepath.Dir(fullPath)
	err = os.MkdirAll(parentDir, 0o755)
	if err != nil {
		t.Fatalf("Failed to create parent directory: %v", err)
	}

	// Write file using WriteFilePreservePerms
	testData := []byte("integration test data")
	err = WriteFilePreservePerms(fullPath, testData)
	if err != nil {
		t.Fatalf("WriteFilePreservePerms() failed: %v", err)
	}

	// Verify file was written correctly
	content, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("Failed to read written file: %v", err)
	}
	if string(content) != string(testData) {
		t.Errorf("File content mismatch: got %q, expected %q", string(content), string(testData))
	}
}
