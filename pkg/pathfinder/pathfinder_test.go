package pathfinder

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSafetyValidator_ValidatePath(t *testing.T) {
	validator := NewSafetyValidator()
	validator.SetAllowSymlinks(true)

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"valid path", "/tmp/test", false},
		{"empty path", "", true},
		{"traversal attempt", "/tmp/../etc/passwd", true},
		{"current dir", ".", false},
		{"relative path", "test/file.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidatePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSafetyValidator_SafeJoin(t *testing.T) {
	validator := NewSafetyValidator()
	validator.SetAllowSymlinks(true)

	tests := []struct {
		name    string
		base    string
		path    string
		want    string
		wantErr bool
	}{
		{"simple join", "/tmp", "test.txt", "/tmp/test.txt", false},
		{"empty base", "", "test.txt", "", true},
		{"traversal in path", "/tmp", "../etc/passwd", "", true},
		{"relative components", "/tmp", "foo/bar", "/tmp/foo/bar", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validator.SafeJoin(tt.base, tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("SafeJoin() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("SafeJoin() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRepositoryConstraint(t *testing.T) {
	// Create a temporary directory structure
	tmpDir, err := os.MkdirTemp("", "pathfinder_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	// Create a mock git repo
	repoDir := filepath.Join(tmpDir, "repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	constraint, err := NewRepositoryConstraint(repoDir)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{"inside repo", filepath.Join(repoDir, "src/main.go"), true},
		{"repo root", repoDir, true},
		{"outside repo", tmpDir, false},
		{"traversal attempt", filepath.Join(repoDir, "../outside"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := constraint.Contains(tt.path); got != tt.want {
				t.Errorf("Contains() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAuditLogger_LogOperation(t *testing.T) {
	logger := NewAuditLogger()

	record := AuditRecord{
		Operation:    OpOpen,
		Path:         "/tmp/test.txt",
		SourceLoader: "local",
		Result: OperationResult{
			Status: "success",
			Code:   200,
		},
	}

	err := logger.LogOperation(record)
	if err != nil {
		t.Errorf("LogOperation() error = %v", err)
	}

	// Query the record back
	query := AuditQuery{
		Operation: &record.Operation,
		Limit:     10,
	}

	results, err := logger.Query(query)
	if err != nil {
		t.Errorf("Query() error = %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	if results[0].Operation != OpOpen {
		t.Errorf("Operation mismatch: got %v, want %v", results[0].Operation, OpOpen)
	}
}

func TestAuditLogger_ComplianceModes(t *testing.T) {
	logger := NewAuditLogger()

	// Test HIPAA compliance
	err := logger.SetComplianceMode(ComplianceHIPAA)
	if err != nil {
		t.Errorf("SetComplianceMode() error = %v", err)
	}

	// Log a PHI-related operation
	record := AuditRecord{
		Operation:    OpOpen,
		Path:         "/data/phi/patient123.txt",
		SourceLoader: "local",
		Result: OperationResult{
			Status: "success",
			Code:   200,
		},
	}

	err = logger.LogOperation(record)
	if err != nil {
		t.Errorf("LogOperation() error = %v", err)
	}

	// Query and check for HIPAA flag
	query := AuditQuery{Limit: 10}
	results, err := logger.Query(query)
	if err != nil {
		t.Errorf("Query() error = %v", err)
	}

	if len(results) == 0 {
		t.Fatal("No audit records found")
	}

	if !strings.Contains(strings.Join(results[0].SecurityFlags, ","), "HIPAA_PHI_ACCESS") {
		t.Error("HIPAA compliance flag not set for PHI access")
	}
}

func TestDiscoveryEngine_DiscoverFiles(t *testing.T) {
	// Create temporary directory structure
	tmpDir, err := os.MkdirTemp("", "discovery_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	// Create test files
	testFiles := []string{
		"main.go",
		"utils.go",
		"README.md",
		"test.txt",
	}

	for _, file := range testFiles {
		path := filepath.Join(tmpDir, file)
		if err := os.WriteFile(path, []byte("test content"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Create subdirectory with more files
	subDir := filepath.Join(tmpDir, "pkg")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	subFile := filepath.Join(subDir, "helper.go")
	if err := os.WriteFile(subFile, []byte("package helper"), 0644); err != nil {
		t.Fatal(err)
	}

	validator := NewSafetyValidator()
	validator.SetAllowSymlinks(true)
	engine := NewDiscoveryEngine(validator)

	opts := DiscoveryOptions{
		IncludePatterns: []string{"*.go"},
	}

	files, err := engine.DiscoverFiles(tmpDir, opts)
	if err != nil {
		t.Errorf("DiscoverFiles() error = %v", err)
	}

	// Should find at least the .go files we created
	goFiles := 0
	for _, file := range files {
		if strings.HasSuffix(file, ".go") {
			goFiles++
		}
	}

	if goFiles < 2 {
		t.Errorf("Expected at least 2 .go files, got %d from files: %v", goFiles, files)
	}

	// Check that all returned files have .go extension
	for _, file := range files {
		if !strings.HasSuffix(file, ".go") {
			t.Errorf("Unexpected file in results: %s", file)
		}
	}
}

func TestSafeWalker_WalkDirectory(t *testing.T) {
	// Use testdata directory to avoid temp dir symlink issues
	testDir := "testdata"
	walkerTestDir := filepath.Join(testDir, "walker_test")
	if err := os.MkdirAll(walkerTestDir, 0755); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.RemoveAll(walkerTestDir)
	}()

	// Create test files
	files := []string{"a.txt", "b.txt", "c.go"}
	for _, file := range files {
		path := filepath.Join(walkerTestDir, file)
		if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Create subdirectory
	subDir := filepath.Join(walkerTestDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	subFile := filepath.Join(subDir, "d.txt")
	if err := os.WriteFile(subFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	validator := NewSafetyValidator()
	validator.SetAllowSymlinks(true)
	walker := NewSafeWalker(validator)

	var (
		foundFiles []string
		mu         sync.Mutex
	)
	walkFunc := func(path string, info FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			relPath, _ := filepath.Rel(walkerTestDir, path)
			mu.Lock()
			foundFiles = append(foundFiles, relPath)
			mu.Unlock()
		}
		return nil
	}

	err := walker.WalkDirectory(walkerTestDir, walkFunc, WalkOptions{})
	if err != nil {
		t.Errorf("WalkDirectory() error = %v", err)
	}

	expectedFiles := 4
	if len(foundFiles) != expectedFiles {
		t.Errorf("Expected %d files, got %d: %v", expectedFiles, len(foundFiles), foundFiles)
	}
}

func TestAuditLogger_DeterministicMode(t *testing.T) {
	logger := NewAuditLogger()
	logger.SetDeterministicMode(true, "test-seed")

	// Create identical records with current timestamps
	now := time.Now()
	record1 := AuditRecord{
		Operation:    OpOpen,
		Path:         "/tmp/test.txt",
		SourceLoader: "local",
		Result:       OperationResult{Status: "success", Code: 200},
		Timestamp:    now,
	}

	record2 := AuditRecord{
		Operation:    OpOpen,
		Path:         "/tmp/test.txt",
		SourceLoader: "local",
		Result:       OperationResult{Status: "success", Code: 200},
		Timestamp:    now,
	}

	err := logger.LogOperation(record1)
	if err != nil {
		t.Errorf("LogOperation() error = %v", err)
	}

	err = logger.LogOperation(record2)
	if err != nil {
		t.Errorf("LogOperation() error = %v", err)
	}

	// Query records
	results, err := logger.Query(AuditQuery{Limit: 10})
	if err != nil {
		t.Errorf("Query() error = %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("Expected 2 records, got %d", len(results))
	}

	// Records should have the same deterministic ID
	if results[0].ID != results[1].ID {
		t.Errorf("Deterministic IDs should be identical: %s != %s", results[0].ID, results[1].ID)
	}
}

func BenchmarkDiscoveryEngine_DiscoverFiles(b *testing.B) {
	// Create a larger test directory for benchmarking
	tmpDir, err := os.MkdirTemp("", "bench_test")
	if err != nil {
		b.Fatal(err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	// Create many files
	for i := 0; i < 1000; i++ {
		filename := filepath.Join(tmpDir, fmt.Sprintf("file%d.txt", i))
		if err := os.WriteFile(filename, []byte("benchmark content"), 0644); err != nil {
			b.Fatal(err)
		}
	}

	validator := NewSafetyValidator()
	engine := NewDiscoveryEngine(validator)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := engine.DiscoverFiles(tmpDir, DiscoveryOptions{})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAuditLogger_LogOperation(b *testing.B) {
	logger := NewAuditLogger()

	record := AuditRecord{
		Operation:    OpOpen,
		Path:         "/tmp/benchmark.txt",
		SourceLoader: "local",
		Result:       OperationResult{Status: "success", Code: 200},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		record.ID = "" // Reset ID for each iteration
		_ = logger.LogOperation(record)
	}
}

// TestDiscoveryEngine_PatternNormalization tests that pattern matching handles
// various path conventions correctly across platforms (Unix/Windows)
func TestDiscoveryEngine_PatternNormalization(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pattern_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files
	testFiles := map[string]string{
		"pyproject.toml":           "root file",
		"package.json":             "root package",
		"apps/web/package.json":    "web package",
		"apps/api/package.json":    "api package",
		"services/auth/go.mod":     "auth module",
		"node_modules/foo/test.js": "should be excluded",
		"docs/README.md":           "docs",
	}

	for relPath, content := range testFiles {
		fullPath := filepath.Join(tmpDir, filepath.FromSlash(relPath))
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	validator := NewSafetyValidator()
	validator.SetAllowSymlinks(true)
	engine := NewDiscoveryEngine(validator)

	tests := []struct {
		name            string
		includePatterns []string
		excludePatterns []string
		wantFiles       []string
		wantExcluded    []string
	}{
		{
			name:            "Unix-style ./ prefix for root file",
			includePatterns: []string{"./pyproject.toml"},
			excludePatterns: []string{},
			wantFiles:       []string{"pyproject.toml"},
			wantExcluded:    []string{"package.json", "apps/web/package.json"},
		},
		{
			name:            "Windows-style .\\ prefix for root file (simulated)",
			includePatterns: []string{".\\pyproject.toml"}, // Windows convention
			excludePatterns: []string{},
			wantFiles:       []string{"pyproject.toml"},
			wantExcluded:    []string{"package.json"},
		},
		{
			name:            "Simple filename matches across directories",
			includePatterns: []string{"package.json"},
			excludePatterns: []string{"**/node_modules/**"},
			wantFiles:       []string{"package.json", "apps/web/package.json", "apps/api/package.json"},
			wantExcluded:    []string{"node_modules/foo/test.js"},
		},
		{
			name:            "Glob pattern with ** for deep matching",
			includePatterns: []string{"apps/**/package.json"},
			excludePatterns: []string{},
			wantFiles:       []string{"apps/web/package.json", "apps/api/package.json"},
			wantExcluded:    []string{"package.json"}, // Root should be excluded
		},
		{
			name:            "Redundant separators in patterns",
			includePatterns: []string{"./apps//web//package.json"}, // Extra slashes
			excludePatterns: []string{},
			wantFiles:       []string{"apps/web/package.json"},
			wantExcluded:    []string{"apps/api/package.json"},
		},
		{
			name:            "Multiple file types with exclusions",
			includePatterns: []string{"*.toml", "*.json"},
			excludePatterns: []string{"**/node_modules/**", "**/apps/**"},
			wantFiles:       []string{"pyproject.toml", "package.json"},
			wantExcluded:    []string{"apps/web/package.json", "node_modules/foo/test.js"},
		},
		{
			name:            "Parent directory reference in pattern (..)",
			includePatterns: []string{"apps/../pyproject.toml"}, // Resolves to pyproject.toml
			excludePatterns: []string{},
			wantFiles:       []string{"pyproject.toml"},
			wantExcluded:    []string{"package.json"},
		},
		{
			name:            "Exclude with ./ prefix",
			includePatterns: []string{"*.json"},
			excludePatterns: []string{"./package.json"}, // Exclude root only
			wantFiles:       []string{"apps/web/package.json", "apps/api/package.json"},
			wantExcluded:    []string{"package.json"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := DiscoveryOptions{
				IncludePatterns: tt.includePatterns,
				ExcludePatterns: tt.excludePatterns,
			}

			files, err := engine.DiscoverFiles(tmpDir, opts)
			if err != nil {
				t.Fatalf("DiscoverFiles() error = %v", err)
			}

			// Normalize file paths for comparison (use forward slashes)
			normalizedFiles := make([]string, len(files))
			for i, f := range files {
				normalizedFiles[i] = filepath.ToSlash(f)
			}

			// Check that all wanted files are present
			for _, want := range tt.wantFiles {
				found := false
				for _, got := range normalizedFiles {
					if got == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected file %q not found in results: %v", want, normalizedFiles)
				}
			}

			// Check that excluded files are NOT present
			for _, excluded := range tt.wantExcluded {
				for _, got := range normalizedFiles {
					if got == excluded {
						t.Errorf("File %q should have been excluded but was found in results", excluded)
					}
				}
			}
		})
	}
}

// TestDiscoveryEngine_CrossPlatformPatterns tests pattern matching edge cases
// that might behave differently on Unix vs Windows
func TestDiscoveryEngine_CrossPlatformPatterns(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "crossplatform_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a simple test file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	validator := NewSafetyValidator()
	validator.SetAllowSymlinks(true)
	engine := NewDiscoveryEngine(validator)

	// Test various pattern representations that should all match the same file
	patterns := []string{
		"test.txt",           // Simple filename
		"./test.txt",         // Unix current directory
		".\\test.txt",        // Windows current directory
		"./././test.txt",     // Multiple current directory refs
		".//test.txt",        // Redundant separator (Unix)
		".\\\\test.txt",      // Redundant separator (Windows)
		"foo/../test.txt",    // Parent directory reference
		"./foo/../test.txt",  // Combined
	}

	for _, pattern := range patterns {
		t.Run(fmt.Sprintf("pattern=%q", pattern), func(t *testing.T) {
			opts := DiscoveryOptions{
				IncludePatterns: []string{pattern},
			}

			files, err := engine.DiscoverFiles(tmpDir, opts)
			if err != nil {
				t.Fatalf("DiscoverFiles() error = %v", err)
			}

			if len(files) != 1 {
				t.Errorf("Expected 1 file for pattern %q, got %d files: %v", pattern, len(files), files)
			}

			if len(files) > 0 && filepath.ToSlash(files[0]) != "test.txt" {
				t.Errorf("Expected file %q for pattern %q, got %q", "test.txt", pattern, files[0])
			}
		})
	}
}

// TestDiscoveryEngine_PyFulmenRegressionCase tests the exact scenario reported by PyFulmen team
// where "./pyproject.toml" pattern failed to match "pyproject.toml" file
func TestDiscoveryEngine_PyFulmenRegressionCase(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "pyfulmen_regression")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Simulate PyFulmen's exact setup
	pyprojectPath := filepath.Join(tmpDir, "pyproject.toml")
	content := `[project]
name = "pyfulmen"
version = "0.1.2"
`
	if err := os.WriteFile(pyprojectPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Create subdirectories that should be excluded
	for _, dir := range []string{"tests", "docs", ".venv"} {
		if err := os.MkdirAll(filepath.Join(tmpDir, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}

	validator := NewSafetyValidator()
	validator.SetAllowSymlinks(true)
	engine := NewDiscoveryEngine(validator)

	// This is the exact pattern PyFulmen team used in their policy file
	opts := DiscoveryOptions{
		IncludePatterns: []string{"./pyproject.toml"}, // Root pyproject.toml only
		ExcludePatterns: []string{
			"**/node_modules/**",
			"docs/**",
			"tests/**",
			".venv/**",
		},
	}

	files, err := engine.DiscoverFiles(tmpDir, opts)
	if err != nil {
		t.Fatalf("DiscoverFiles() error = %v", err)
	}

	// Should find exactly 1 file
	if len(files) != 1 {
		t.Fatalf("Expected 1 file, got %d files: %v", len(files), files)
	}

	// Should be pyproject.toml
	if filepath.ToSlash(files[0]) != "pyproject.toml" {
		t.Errorf("Expected 'pyproject.toml', got %q", files[0])
	}

	t.Logf("✅ Regression test passed: './pyproject.toml' pattern correctly matched 'pyproject.toml' file")
}
