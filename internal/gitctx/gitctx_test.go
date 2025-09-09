package gitctx

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func TestClassifyScope(t *testing.T) {
	tests := []struct {
		name     string
		total    int
		expected string
	}{
		{"small changes", 25, "small"},
		{"boundary small", 50, "small"},
		{"medium changes", 100, "medium"},
		{"boundary medium", 200, "medium"},
		{"large changes", 500, "large"},
		{"zero changes", 0, "small"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyScope(tt.total)
			if result != tt.expected {
				t.Errorf("classifyScope(%d) = %q, expected %q", tt.total, result, tt.expected)
			}
		})
	}
}

func TestClassifyByFileCount(t *testing.T) {
	tests := []struct {
		name     string
		count    int
		expected string
	}{
		{"small files", 3, "small"},
		{"boundary small", 5, "small"},
		{"medium files", 10, "medium"},
		{"boundary medium", 20, "medium"},
		{"large files", 50, "large"},
		{"zero files", 0, "small"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyByFileCount(tt.count)
			if result != tt.expected {
				t.Errorf("classifyByFileCount(%d) = %q, expected %q", tt.count, result, tt.expected)
			}
		})
	}
}

func TestAtoiSafe(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"valid number", "42", 42},
		{"zero", "0", 0},
		{"negative", "-10", -10},
		{"invalid", "abc", 0},
		{"empty", "", 0},
		{"float", "3.14", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := atoiSafe(tt.input)
			if result != tt.expected {
				t.Errorf("atoiSafe(%q) = %d, expected %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseNumstat(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedTotal int
		expectedFiles []string
	}{
		{
			name:          "single file",
			input:         "5\t3\tsrc/main.go\n",
			expectedTotal: 8,
			expectedFiles: []string{"src/main.go"},
		},
		{
			name:          "multiple files",
			input:         "5\t3\tsrc/main.go\n2\t1\tREADME.md\n",
			expectedTotal: 11,
			expectedFiles: []string{"src/main.go", "README.md"},
		},
		{
			name:          "binary file",
			input:         "-\t-\tbinary.exe\n",
			expectedTotal: 0,
			expectedFiles: []string{"binary.exe"},
		},
		{
			name:          "empty input",
			input:         "",
			expectedTotal: 0,
			expectedFiles: []string{},
		},
		{
			name:          "malformed line",
			input:         "invalid line\n5\t3\tvalid.go\n",
			expectedTotal: 8,
			expectedFiles: []string{"valid.go"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			total, files := parseNumstat([]byte(tt.input))

			if total != tt.expectedTotal {
				t.Errorf("parseNumstat total = %d, expected %d", total, tt.expectedTotal)
			}

			if len(files) != len(tt.expectedFiles) {
				t.Errorf("parseNumstat files count = %d, expected %d", len(files), len(tt.expectedFiles))
				return
			}

			for _, expectedFile := range tt.expectedFiles {
				if _, exists := files[expectedFile]; !exists {
					t.Errorf("parseNumstat missing file %q in result", expectedFile)
				}
			}
		})
	}
}

func TestParseUnifiedInto(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string][]int
	}{
		{
			name: "single addition",
			input: `+++ b/src/main.go
@@ -1,3 +1,4 @@
 package main
+import "fmt"
 func main() {
`,
			expected: map[string][]int{
				"src/main.go": {1, 2, 3, 4},
			},
		},
		{
			name: "multiple additions",
			input: `+++ b/README.md
@@ -5,6 +5,8 @@
 ## Features
+- New feature 1
+- New feature 2
+- New feature 3
 ## Usage
`,
			expected: map[string][]int{
				"README.md": {5, 6, 7, 8, 9, 10, 11, 12},
			},
		},
		{
			name: "modification",
			input: `+++ b/src/main.go
@@ -1,4 +1,4 @@
 package main
-import "fmt"
+import "os"
 func main() {
`,
			expected: map[string][]int{
				"src/main.go": {1, 2, 3, 4},
			},
		},
		{
			name:     "empty input",
			input:    "",
			expected: map[string][]int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := make(map[string][]int)
			parseUnifiedInto(result, []byte(tt.input))

			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("parseUnifiedInto() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestCollectNonExistentDirectory(t *testing.T) {
	// Test with non-existent directory
	ctx, files, err := Collect("/non/existent/directory")
	if ctx != nil {
		t.Error("Collect() should return nil context for non-existent directory")
	}
	if files != nil {
		t.Error("Collect() should return nil files for non-existent directory")
	}
	if err != nil {
		t.Logf("Collect() returned error (expected for non-git dir): %v", err)
	}
}

func TestCollectWithLinesNonExistentDirectory(t *testing.T) {
	// Test CollectWithLines with non-existent directory
	ctx, files, lines, err := CollectWithLines("/non/existent/directory")
	if ctx != nil {
		t.Error("CollectWithLines() should return nil context for non-existent directory")
	}
	if files != nil {
		t.Error("CollectWithLines() should return nil files for non-existent directory")
	}
	if lines != nil {
		t.Error("CollectWithLines() should return nil lines for non-existent directory")
	}
	if err != nil {
		t.Logf("CollectWithLines() returned error (expected for non-git dir): %v", err)
	}
}

func TestCollectEmptyDirectory(t *testing.T) {
	// Create a temporary empty directory (not a git repo)
	tempDir := t.TempDir()

	ctx, files, err := Collect(tempDir)
	if ctx != nil {
		t.Error("Collect() should return nil context for non-git directory")
	}
	if files != nil {
		t.Error("Collect() should return nil files for non-git directory")
	}
	if err != nil {
		t.Logf("Collect() returned error (expected for non-git dir): %v", err)
	}
}

func TestChangeContextStructure(t *testing.T) {
	ctx := &ChangeContext{
		ModifiedFiles: []string{"file1.go", "file2.go"},
		TotalChanges:  25,
		ChangeScope:   "small",
		GitSHA:        "abc123",
		Branch:        "main",
	}

	if len(ctx.ModifiedFiles) != 2 {
		t.Errorf("Expected 2 modified files, got %d", len(ctx.ModifiedFiles))
	}
	if ctx.TotalChanges != 25 {
		t.Errorf("Expected 25 total changes, got %d", ctx.TotalChanges)
	}
	if ctx.ChangeScope != "small" {
		t.Errorf("Expected 'small' scope, got %q", ctx.ChangeScope)
	}
	if ctx.GitSHA != "abc123" {
		t.Errorf("Expected 'abc123' SHA, got %q", ctx.GitSHA)
	}
	if ctx.Branch != "main" {
		t.Errorf("Expected 'main' branch, got %q", ctx.Branch)
	}
}

func TestRunGit(t *testing.T) {
	// Test runGit function with a simple command
	tempDir := t.TempDir()

	// This will fail because tempDir is not a git repo, but we can test the function structure
	result := runGit(tempDir, "status", "--porcelain")
	if result != "" {
		t.Logf("runGit returned: %q", result)
	}
}

func TestRunGitBytes(t *testing.T) {
	// Test runGitBytes function
	tempDir := t.TempDir()

	// This will fail because tempDir is not a git repo, but we can test the function structure
	result := runGitBytes(tempDir, "status", "--porcelain")
	if len(result) > 0 {
		t.Logf("runGitBytes returned: %q", string(result))
	}
}

func TestIsRepoCLI(t *testing.T) {
	// Test with non-git directory
	tempDir := t.TempDir()
	result := isRepoCLI(tempDir)
	if result {
		t.Error("isRepoCLI() should return false for non-git directory")
	}
}

// Integration test that requires git setup
func TestCollectIntegration(t *testing.T) {
	// Skip if git is not available
	if _, err := os.Stat("/usr/bin/git"); os.IsNotExist(err) {
		if _, err := os.Stat("/usr/local/bin/git"); os.IsNotExist(err) {
			t.Skip("git not available, skipping integration test")
		}
	}

	// Create a temporary git repo for integration testing
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp dir: %v", err)
	}

	// Initialize git repo
	if err := runGitCmd("init"); err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}
	if err := runGitCmd("config", "user.name", "Test User"); err != nil {
		t.Fatalf("Failed to config git user: %v", err)
	}
	if err := runGitCmd("config", "user.email", "test@example.com"); err != nil {
		t.Fatalf("Failed to config git email: %v", err)
	}

	// Create and commit a file
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("initial content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	if err := runGitCmd("add", "test.txt"); err != nil {
		t.Fatalf("Failed to add file: %v", err)
	}
	if err := runGitCmd("commit", "-m", "Initial commit"); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	// Modify the file to create changes
	if err := os.WriteFile(testFile, []byte("modified content"), 0644); err != nil {
		t.Fatalf("Failed to modify test file: %v", err)
	}

	// Test collection
	ctx, files, err := Collect(tempDir)
	if err != nil {
		t.Fatalf("Collect() failed: %v", err)
	}
	if ctx == nil {
		t.Fatal("Collect() returned nil context")
	}
	if len(files) == 0 {
		t.Error("Expected at least one modified file")
	}

	// Test CollectWithLines
	ctx2, files2, lines, err := CollectWithLines(tempDir)
	if err != nil {
		t.Fatalf("CollectWithLines() failed: %v", err)
	}
	if ctx2 == nil {
		t.Fatal("CollectWithLines() returned nil context")
	}
	if !reflect.DeepEqual(files, files2) {
		t.Error("Collect and CollectWithLines should return same files")
	}
	if lines == nil {
		t.Log("CollectWithLines returned nil lines (expected for simple diff)")
	}
}

// Helper function to run git commands in tests
func runGitCmd(args ...string) error {
	cmd := exec.Command("git", args...)
	return cmd.Run()
}
