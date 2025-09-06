package ignore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewMatcher(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "goneat-ignore-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Errorf("Failed to clean up temp dir: %v", err)
		}
	}()

	// Create a test .gitignore file
	gitignoreContent := `# Test gitignore
*.log
node_modules/
.temp/
!.temp/keep.txt
`
	gitignorePath := filepath.Join(tempDir, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644); err != nil {
		t.Fatalf("Failed to write .gitignore: %v", err)
	}

	// Create a test .goneatignore file
	goneatignoreContent := `# Test goneatignore
*.backup
test-data/
`
	goneatignorePath := filepath.Join(tempDir, ".goneatignore")
	if err := os.WriteFile(goneatignorePath, []byte(goneatignoreContent), 0644); err != nil {
		t.Fatalf("Failed to write .goneatignore: %v", err)
	}

	// Change to temp directory for testing
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Errorf("Failed to restore directory: %v", err)
		}
	}()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Create matcher
	matcher, err := NewMatcher(".")
	if err != nil {
		t.Fatalf("Failed to create matcher: %v", err)
	}

	// Test cases for file ignore
	fileTests := []struct {
		path     string
		expected bool
		name     string
	}{
		// Default ignores
		{".git/config", true, "git directory"},
		{"node_modules/package.json", true, "node_modules directory"},
		{".scratchpad/temp.txt", true, "scratchpad directory"},

		// .gitignore patterns
		{"error.log", true, "*.log pattern"},
		{"debug.log", true, "*.log pattern nested"},
		{"logs/error.log", true, "*.log pattern in subdirectory"},
		{"node_modules/lib.js", true, "node_modules/ pattern"},
		{".temp/file.txt", true, ".temp/ pattern"},
		{".temp/keep.txt", false, "negation pattern !.temp/keep.txt"},

		// .goneatignore patterns
		{"data.backup", true, "*.backup pattern from goneatignore"},
		{"test-data/file.txt", true, "test-data/ pattern from goneatignore"},

		// Files that should not be ignored
		{"main.go", false, "regular go file"},
		{"README.md", false, "markdown file"},
		{"src/lib.go", false, "nested go file"},
	}

	for _, tt := range fileTests {
		t.Run(tt.name, func(t *testing.T) {
			result := matcher.IsIgnored(tt.path)
			if result != tt.expected {
				t.Errorf("IsIgnored(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}

	// Test cases for directory ignore
	dirTests := []struct {
		path     string
		expected bool
		name     string
	}{
		// Default ignores
		{".git", true, "git directory"},
		{"node_modules", true, "node_modules directory"},
		{".scratchpad", true, "scratchpad directory"},

		// .gitignore patterns
		{".temp", true, ".temp directory"},
		{"node_modules", true, "node_modules directory from gitignore"},

		// .goneatignore patterns
		{"test-data", true, "test-data directory from goneatignore"},

		// Directories that should not be ignored
		{"src", false, "source directory"},
		{"pkg", false, "package directory"},
		{"cmd", false, "command directory"},
	}

	for _, tt := range dirTests {
		t.Run(tt.name+"_dir", func(t *testing.T) {
			result := matcher.IsIgnoredDir(tt.path)
			if result != tt.expected {
				t.Errorf("IsIgnoredDir(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestReadIgnoreFile(t *testing.T) {
	// Create temporary file
	tempDir, err := os.MkdirTemp("", "goneat-ignore-read-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Errorf("Failed to clean up temp dir: %v", err)
		}
	}()

	ignoreContent := `# Comment line
*.log

# Another comment
node_modules/
!important.log

# Empty lines should be ignored


test/
`
	ignoreFile := filepath.Join(tempDir, "test-ignore")
	if err := os.WriteFile(ignoreFile, []byte(ignoreContent), 0644); err != nil {
		t.Fatalf("Failed to write ignore file: %v", err)
	}

	patterns, err := readIgnoreFile(ignoreFile)
	if err != nil {
		t.Fatalf("readIgnoreFile failed: %v", err)
	}

	expected := []string{
		"*.log",
		"node_modules/",
		"!important.log",
		"test/",
	}

	if len(patterns) != len(expected) {
		t.Errorf("Expected %d patterns, got %d", len(expected), len(patterns))
	}

	for i, pattern := range patterns {
		if pattern != expected[i] {
			t.Errorf("Pattern %d: expected %q, got %q", i, expected[i], pattern)
		}
	}
}

func TestReadIgnoreFileNotExists(t *testing.T) {
	_, err := readIgnoreFile("/nonexistent/file")
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}
}

func TestSplitPath(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
		name     string
	}{
		{"", []string{}, "empty string"},
		{".", []string{}, "current directory"},
		{"file.txt", []string{"file.txt"}, "simple file"},
		{"dir/file.txt", []string{"dir", "file.txt"}, "nested file"},
		{"a/b/c/file.txt", []string{"a", "b", "c", "file.txt"}, "deeply nested file"},
		{"/absolute/path", []string{"absolute", "path"}, "absolute path"},
		{"./relative/path", []string{"relative", "path"}, "relative path with ./"},
		{"path//with/empty//segments", []string{"path", "with", "empty", "segments"}, "path with empty segments"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitPath(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("splitPath(%q) returned %d parts, expected %d", tt.input, len(result), len(tt.expected))
				return
			}
			for i, part := range result {
				if part != tt.expected[i] {
					t.Errorf("splitPath(%q)[%d] = %q, expected %q", tt.input, i, part, tt.expected[i])
				}
			}
		})
	}
}

func TestMatcherWithNoIgnoreFiles(t *testing.T) {
	// Create a temporary directory with no ignore files
	tempDir, err := os.MkdirTemp("", "goneat-ignore-empty-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Errorf("Failed to clean up temp dir: %v", err)
		}
	}()

	// Change to temp directory
	originalDir, _ := os.Getwd()
	defer func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Errorf("Failed to restore original directory: %v", err)
		}
	}()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Create matcher
	matcher, err := NewMatcher(".")
	if err != nil {
		t.Fatalf("Failed to create matcher: %v", err)
	}

	// Test that default patterns still work
	tests := []struct {
		path     string
		expected bool
		name     string
	}{
		{".git/config", true, "git directory should be ignored by default"},
		{"node_modules/lib.js", true, "node_modules should be ignored by default"},
		{".scratchpad/temp.txt", true, "scratchpad should be ignored by default"},
		{"main.go", false, "regular file should not be ignored"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matcher.IsIgnored(tt.path)
			if result != tt.expected {
				t.Errorf("IsIgnored(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}
