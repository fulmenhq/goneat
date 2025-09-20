package pathfinder

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	// Create temporary directory structure
	tmpDir, err := os.MkdirTemp("", "walker_test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	// Create test files
	files := []string{"a.txt", "b.txt", "c.go"}
	for _, file := range files {
		path := filepath.Join(tmpDir, file)
		if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Create subdirectory
	subDir := filepath.Join(tmpDir, "subdir")
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

	var foundFiles []string
	walkFunc := func(path string, info FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			relPath, _ := filepath.Rel(tmpDir, path)
			foundFiles = append(foundFiles, relPath)
		}
		return nil
	}

	err = walker.WalkDirectory(tmpDir, walkFunc, WalkOptions{})
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
