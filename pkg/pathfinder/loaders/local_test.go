package loaders

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fulmenhq/goneat/pkg/pathfinder"
)

func TestNewLocalLoader(t *testing.T) {
	loader := NewLocalLoader("")
	if loader == nil {
		t.Fatal("NewLocalLoader() returned nil")
	}
	if loader.rootPath != "" {
		t.Errorf("NewLocalLoader() rootPath = %q, want empty", loader.rootPath)
	}
	if loader.maxFileSize != 100*1024*1024 {
		t.Errorf("NewLocalLoader() maxFileSize = %d, want 100MB", loader.maxFileSize)
	}
	if loader.followSymlinks {
		t.Error("NewLocalLoader() followSymlinks should be false by default")
	}
}

func TestLocalLoader_SourceType(t *testing.T) {
	loader := NewLocalLoader("")
	result := loader.SourceType()
	if result != "local" {
		t.Errorf("LocalLoader.SourceType() = %q, want %q", result, "local")
	}
}

func TestLocalLoader_SourceDescription(t *testing.T) {
	loader := NewLocalLoader("")
	result := loader.SourceDescription()
	expected := "Local filesystem loader (root: )"
	if result != expected {
		t.Errorf("LocalLoader.SourceDescription() = %q, want %q", result, expected)
	}

	loader.rootPath = "/tmp"
	result = loader.SourceDescription()
	expected = "Local filesystem loader (root: /tmp)"
	if result != expected {
		t.Errorf("LocalLoader.SourceDescription() = %q, want %q", result, expected)
	}
}

func TestLocalLoader_Validate(t *testing.T) {
	// Create temp directory for testing
	tempDir, err := os.MkdirTemp("", "goneat-loader-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name     string
		rootPath string
		hasError bool
	}{
		{"empty root path", "", true},
		{"valid directory", tempDir, false},
		{"non-existent path", "/non/existent/path", true},
		{"file instead of directory", func() string {
			filePath := filepath.Join(tempDir, "file.txt")
			_ = os.WriteFile(filePath, []byte("test"), 0644)
			return filePath
		}(), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := NewLocalLoader(tt.rootPath)
			err := loader.Validate()
			if (err != nil) != tt.hasError {
				t.Errorf("LocalLoader.Validate() error = %v, hasError %v", err, tt.hasError)
			}
		})
	}
}

func TestLocalLoader_Open(t *testing.T) {
	// Create temp directory and file for testing
	tempDir, err := os.MkdirTemp("", "goneat-loader-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "test.txt")
	testContent := "Hello, World!"
	err = os.WriteFile(testFile, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	loader := NewLocalLoader(tempDir)

	tests := []struct {
		name     string
		path     string
		hasError bool
	}{
		{"valid file", "test.txt", false},
		{"non-existent file", "nonexistent.txt", true},
		{"path traversal attempt", "../test.txt", true},
		{"empty path", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader, err := loader.Open(tt.path)
			if (err != nil) != tt.hasError {
				t.Errorf("LocalLoader.Open(%q) error = %v, hasError %v", tt.path, err, tt.hasError)
			}

			if !tt.hasError && reader != nil {
				defer func() { _ = reader.Close() }()
				content := make([]byte, len(testContent))
				n, err := reader.Read(content)
				if err != nil {
					t.Errorf("Failed to read from opened file: %v", err)
				}
				if n != len(testContent) || string(content) != testContent {
					t.Errorf("Read content = %q, want %q", string(content), testContent)
				}
			}
		})
	}
}

func TestLocalLoader_ListFiles(t *testing.T) {
	// Create temp directory structure for testing
	tempDir, err := os.MkdirTemp("", "goneat-loader-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files and directories
	_ = os.MkdirAll(filepath.Join(tempDir, "subdir"), 0755)
	_ = os.WriteFile(filepath.Join(tempDir, "file1.txt"), []byte("content1"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "file2.go"), []byte("content2"), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "subdir", "file3.txt"), []byte("content3"), 0644)

	loader := NewLocalLoader(tempDir)

	tests := []struct {
		name     string
		basePath string
		include  []string
		exclude  []string
		expected []string
		hasError bool
	}{
		{
			name:     "list all files",
			basePath: ".",
			include:  []string{},
			exclude:  []string{},
			expected: []string{"file1.txt", "file2.go", "subdir/file3.txt"},
			hasError: false,
		},
		{
			name:     "include txt files",
			basePath: ".",
			include:  []string{"**/*.txt"},
			exclude:  []string{},
			expected: []string{"file1.txt", "subdir/file3.txt"},
			hasError: false,
		},
		{
			name:     "exclude go files",
			basePath: ".",
			include:  []string{},
			exclude:  []string{"*.go"},
			expected: []string{"file1.txt", "subdir/file3.txt"},
			hasError: false,
		},
		{
			name:     "include and exclude",
			basePath: ".",
			include:  []string{"*.txt"},
			exclude:  []string{"subdir/**"},
			expected: []string{"file1.txt"},
			hasError: false,
		},
		{
			name:     "non-existent base path",
			basePath: "nonexistent",
			include:  []string{},
			exclude:  []string{},
			expected: nil,
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := loader.ListFiles(tt.basePath, tt.include, tt.exclude)
			if (err != nil) != tt.hasError {
				t.Errorf("LocalLoader.ListFiles() error = %v, hasError %v", err, tt.hasError)
			}

			if !tt.hasError {
				// Sort both slices for comparison
				resultSorted := make([]string, len(result))
				copy(resultSorted, result)
				for idx := range resultSorted {
					resultSorted[idx] = filepath.ToSlash(resultSorted[idx])
				}

				if len(resultSorted) != len(tt.expected) {
					t.Errorf("ListFiles() len = %d, want %d", len(resultSorted), len(tt.expected))
					t.Logf("Got: %v", resultSorted)
					t.Logf("Want: %v", tt.expected)
				} else {
					for _, expected := range tt.expected {
						found := false
						for _, actual := range resultSorted {
							if actual == expected {
								found = true
								break
							}
						}
						if !found {
							t.Errorf("ListFiles() missing expected file %q in result %v", expected, resultSorted)
						}
					}
				}
			}
		})
	}
}

func TestLocalLoader_SetMaxFileSize(t *testing.T) {
	loader := NewLocalLoader("")
	loader.SetMaxFileSize(50 * 1024 * 1024) // 50MB

	if loader.maxFileSize != 50*1024*1024 {
		t.Errorf("SetMaxFileSize() maxFileSize = %d, want %d", loader.maxFileSize, 50*1024*1024)
	}
}

func TestLocalLoader_SetFollowSymlinks(t *testing.T) {
	loader := NewLocalLoader("")
	loader.SetFollowSymlinks(true)

	if !loader.followSymlinks {
		t.Error("SetFollowSymlinks(true) did not set followSymlinks to true")
	}
}

func TestValidateAndCleanPath(t *testing.T) {
	// Create temp directory for testing
	tempDir, err := os.MkdirTemp("", "goneat-loader-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	loader := NewLocalLoader(tempDir)

	tests := []struct {
		name     string
		path     string
		hasError bool
	}{
		{"empty path", "", true},
		{"valid relative path", "test.txt", false},
		{"path with traversal", "../test.txt", false}, // Will be cleaned but may fail constraint
		{"absolute path", "/tmp/test.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := loader.validateAndCleanPath(tt.path)
			if (err != nil) != tt.hasError {
				t.Errorf("validateAndCleanPath(%q) error = %v, hasError %v", tt.path, err, tt.hasError)
			}

			if !tt.hasError && result == "" {
				t.Errorf("validateAndCleanPath(%q) returned empty result", tt.path)
			}
		})
	}
}

func TestShouldInclude(t *testing.T) {
	loader := NewLocalLoader("")

	tests := []struct {
		name     string
		path     string
		include  []string
		exclude  []string
		expected bool
	}{
		{"no patterns", "test.txt", []string{}, []string{}, true},
		{"include match", "test.txt", []string{"*.txt"}, []string{}, true},
		{"include no match", "test.go", []string{"*.txt"}, []string{}, false},
		{"exclude match", "test.txt", []string{}, []string{"*.txt"}, false},
		{"include and exclude", "test.txt", []string{"*.txt"}, []string{"test.txt"}, false},
		{"complex pattern", "src/main.go", []string{"src/**"}, []string{"**/*_test.go"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := loader.shouldInclude(tt.path, tt.include, tt.exclude)
			if result != tt.expected {
				t.Errorf("shouldInclude(%q, %v, %v) = %v, want %v", tt.path, tt.include, tt.exclude, result, tt.expected)
			}
		})
	}
}

func TestMatchesAnyPattern(t *testing.T) {
	loader := NewLocalLoader("")

	tests := []struct {
		name     string
		path     string
		patterns []string
		expected bool
	}{
		{"no patterns", "test.txt", []string{}, false},
		{"single match", "test.txt", []string{"*.txt"}, true},
		{"single no match", "test.go", []string{"*.txt"}, false},
		{"multiple patterns match", "test.txt", []string{"*.go", "*.txt"}, true},
		{"multiple patterns no match", "test.md", []string{"*.go", "*.txt"}, false},
		{"globstar pattern", "src/main.go", []string{"src/**"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := loader.matchesAnyPattern(tt.path, tt.patterns)
			if result != tt.expected {
				t.Errorf("matchesAnyPattern(%q, %v) = %v, want %v", tt.path, tt.patterns, result, tt.expected)
			}
		})
	}
}

func TestNewFileInfo(t *testing.T) {
	// Create a temp file for testing
	tempDir, err := os.MkdirTemp("", "goneat-loader-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "test.txt")
	content := "Hello, World!"
	err = os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Get file info
	info, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("Failed to stat test file: %v", err)
	}

	// Create a wrapper that implements pathfinder.FileInfo
	fileInfo := &mockFileInfo{
		name:    info.Name(),
		size:    info.Size(),
		mode:    pathfinder.FileMode(info.Mode()),
		modTime: info.ModTime(),
		isDir:   info.IsDir(),
		sys:     info.Sys(),
	}

	result := NewFileInfo(testFile, fileInfo)

	if result.Path != testFile {
		t.Errorf("NewFileInfo() Path = %q, want %q", result.Path, testFile)
	}
	if result.Name != "test.txt" {
		t.Errorf("NewFileInfo() Name = %q, want %q", result.Name, "test.txt")
	}
	if result.Size != int64(len(content)) {
		t.Errorf("NewFileInfo() Size = %d, want %d", result.Size, len(content))
	}
	if result.IsDir {
		t.Error("NewFileInfo() IsDir should be false for a file")
	}
}

// mockFileInfo implements pathfinder.FileInfo for testing
type mockFileInfo struct {
	name    string
	size    int64
	mode    pathfinder.FileMode
	modTime time.Time
	isDir   bool
	sys     interface{}
}

func (m *mockFileInfo) Name() string              { return m.name }
func (m *mockFileInfo) Size() int64               { return m.size }
func (m *mockFileInfo) Mode() pathfinder.FileMode { return m.mode }
func (m *mockFileInfo) ModTime() time.Time        { return m.modTime }
func (m *mockFileInfo) IsDir() bool               { return m.isDir }
func (m *mockFileInfo) Sys() interface{}          { return m.sys }
