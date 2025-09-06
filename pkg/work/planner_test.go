package work

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewPlanner(t *testing.T) {
	config := PlannerConfig{
		Command: "test",
		Paths:   []string{"."},
	}

	planner := NewPlanner(config)

	if planner == nil {
		t.Fatal("expected planner to be created, got nil")
	}

	if planner.config.Command != "test" {
		t.Errorf("expected command 'test', got '%s'", planner.config.Command)
	}

	// Should have ignore matcher initialized (can be nil if initialization fails)
	// This is acceptable as it falls back gracefully
}

func TestEliminateRedundancies(t *testing.T) {
	planner := NewPlanner(PlannerConfig{})

	tests := []struct {
		name              string
		input             []string
		expectedFiltered  []string
		expectedRedundant []string
	}{
		{
			name:              "empty input",
			input:             []string{},
			expectedFiltered:  []string{},
			expectedRedundant: nil,
		},
		{
			name:              "single file",
			input:             []string{"file1.txt"},
			expectedFiltered:  []string{"file1.txt"},
			expectedRedundant: nil,
		},
		{
			name:              "no duplicates",
			input:             []string{"file1.txt", "file2.txt", "file3.txt"},
			expectedFiltered:  []string{"file1.txt", "file2.txt", "file3.txt"},
			expectedRedundant: nil,
		},
		{
			name:              "duplicate files",
			input:             []string{"file1.txt", "file2.txt", "file1.txt", "file3.txt", "file2.txt"},
			expectedFiltered:  []string{"file1.txt", "file2.txt", "file3.txt"},
			expectedRedundant: []string{"file1.txt", "file2.txt"},
		},
		{
			name:              "all duplicates",
			input:             []string{"same.txt", "same.txt", "same.txt"},
			expectedFiltered:  []string{"same.txt"},
			expectedRedundant: []string{"same.txt", "same.txt"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered, redundant := planner.eliminateRedundancies(tt.input)

			if len(filtered) != len(tt.expectedFiltered) {
				t.Errorf("expected %d filtered files, got %d", len(tt.expectedFiltered), len(filtered))
			}

			for i, expected := range tt.expectedFiltered {
				if i >= len(filtered) || filtered[i] != expected {
					t.Errorf("expected filtered[%d] = '%s', got '%s'", i, expected, filtered[i])
				}
			}

			if len(redundant) != len(tt.expectedRedundant) {
				t.Errorf("expected %d redundant files, got %d", len(tt.expectedRedundant), len(redundant))
			}

			for i, expected := range tt.expectedRedundant {
				if i >= len(redundant) || redundant[i] != expected {
					t.Errorf("expected redundant[%d] = '%s', got '%s'", i, expected, redundant[i])
				}
			}
		})
	}
}

func TestShouldIncludeFile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "planner_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Errorf("Failed to clean up temp dir: %v", err)
		}
	}()

	tests := []struct {
		name        string
		config      PlannerConfig
		filePath    string
		expected    bool
		description string
	}{
		{
			name: "no filters",
			config: PlannerConfig{
				Paths: []string{tempDir},
			},
			filePath:    "test.go",
			expected:    true,
			description: "should include file when no filters are applied",
		},
		{
			name: "content type filter - match",
			config: PlannerConfig{
				ContentTypes: []string{"go"},
			},
			filePath:    "main.go",
			expected:    true,
			description: "should include Go file when Go content type is allowed",
		},
		{
			name: "content type filter - no match",
			config: PlannerConfig{
				ContentTypes: []string{"go"},
			},
			filePath:    "style.css",
			expected:    false,
			description: "should exclude CSS file when only Go content type is allowed",
		},
		{
			name: "include pattern - match",
			config: PlannerConfig{
				IncludePatterns: []string{"*.go"},
			},
			filePath:    "main.go",
			expected:    true,
			description: "should include file matching include pattern",
		},
		{
			name: "include pattern - no match",
			config: PlannerConfig{
				IncludePatterns: []string{"*.py"},
			},
			filePath:    "main.go",
			expected:    false,
			description: "should exclude file not matching include pattern",
		},
		{
			name: "exclude pattern - match",
			config: PlannerConfig{
				ExcludePatterns: []string{"*_test.go"},
			},
			filePath:    "main_test.go",
			expected:    false,
			description: "should exclude file matching exclude pattern",
		},
		{
			name: "exclude pattern - no match",
			config: PlannerConfig{
				ExcludePatterns: []string{"*_test.go"},
			},
			filePath:    "main.go",
			expected:    true,
			description: "should include file not matching exclude pattern",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			planner := NewPlanner(tt.config)
			result := planner.shouldIncludeFile(tt.filePath)

			if result != tt.expected {
				t.Errorf("%s: expected %v, got %v", tt.description, tt.expected, result)
			}
		})
	}
}

func TestGetContentType(t *testing.T) {
	planner := NewPlanner(PlannerConfig{})

	tests := []struct {
		extension string
		expected  string
	}{
		{".go", "go"},
		{".py", "python"},
		{".js", "javascript"},
		{".ts", "typescript"},
		{".java", "unknown"}, // Not supported in current implementation
		{".cpp", "unknown"},  // Not supported in current implementation
		{".c", "unknown"},    // Not supported in current implementation
		{".yaml", "yaml"},
		{".yml", "yaml"},
		{".json", "json"},
		{".xml", "xml"},
		{".html", "html"},
		{".css", "css"},
		{".md", "markdown"},
		{".txt", "text"},
		{".sh", "shell"},
		{".unknown", "unknown"},
		{"", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.extension, func(t *testing.T) {
			result := planner.getContentType(tt.extension)
			if result != tt.expected {
				t.Errorf("expected content type '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestCreateWorkItems_SingleFile(t *testing.T) {
	planner := NewPlanner(PlannerConfig{})

	// Create a temporary file for testing
	tempDir, err := os.MkdirTemp("", "work_item_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Errorf("Failed to clean up temp dir: %v", err)
		}
	}()

	testFile := filepath.Join(tempDir, "test.go")
	content := []byte("package main\n\nfunc main() {\n\tfmt.Println(\"Hello, World!\")\n}")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	workItems := planner.createWorkItems([]string{testFile})

	if len(workItems) != 1 {
		t.Fatalf("expected 1 work item, got %d", len(workItems))
	}

	workItem := workItems[0]

	if workItem.Path != testFile {
		t.Errorf("expected path '%s', got '%s'", testFile, workItem.Path)
	}

	if workItem.ContentType != "go" {
		t.Errorf("expected content type 'go', got '%s'", workItem.ContentType)
	}

	if workItem.Size != int64(len(content)) {
		t.Errorf("expected size %d, got %d", len(content), workItem.Size)
	}

	if workItem.ID == "" {
		t.Error("expected non-empty ID")
	}

	if workItem.EstimatedTime <= 0 {
		t.Error("expected positive estimated time")
	}

	if workItem.Priority < 0 {
		t.Error("expected non-negative priority")
	}
}

func TestCreateWorkItems(t *testing.T) {
	planner := NewPlanner(PlannerConfig{})

	// Create temporary files for testing
	tempDir, err := os.MkdirTemp("", "work_items_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Errorf("Failed to clean up temp dir: %v", err)
		}
	}()

	files := []string{
		filepath.Join(tempDir, "main.go"),
		filepath.Join(tempDir, "utils.go"),
		filepath.Join(tempDir, "config.json"),
	}

	for _, file := range files {
		if err := os.WriteFile(file, []byte("test content"), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", file, err)
		}
	}

	workItems := planner.createWorkItems(files)

	if len(workItems) != len(files) {
		t.Errorf("expected %d work items, got %d", len(files), len(workItems))
	}

	// Check that each file has a corresponding work item
	for i, file := range files {
		if workItems[i].Path != file {
			t.Errorf("expected work item %d path '%s', got '%s'", i, file, workItems[i].Path)
		}
	}
}

func TestCreateGroups(t *testing.T) {
	planner := NewPlanner(PlannerConfig{
		GroupByContentType: true,
	})

	// Create work items with different content types
	workItems := []WorkItem{
		{ID: "1", Path: "main.go", ContentType: "go", EstimatedTime: 1.0},
		{ID: "2", Path: "utils.go", ContentType: "go", EstimatedTime: 2.0},
		{ID: "3", Path: "config.json", ContentType: "json", EstimatedTime: 0.5},
		{ID: "4", Path: "style.css", ContentType: "css", EstimatedTime: 1.5},
	}

	groups := planner.createGroups(workItems)

	// Should have groups for each content type
	contentTypes := make(map[string]bool)
	for _, item := range workItems {
		contentTypes[item.ContentType] = true
	}

	if len(groups) < len(contentTypes) {
		t.Errorf("expected at least %d groups, got %d", len(contentTypes), len(groups))
	}

	// Verify groups have correct estimated times
	for _, group := range groups {
		if group.EstimatedTotalTime <= 0 {
			t.Errorf("group '%s' should have positive estimated time, got %f", group.Name, group.EstimatedTotalTime)
		}
	}
}

func TestCalculateStatistics(t *testing.T) {
	planner := NewPlanner(PlannerConfig{})

	files := []string{"main.go", "utils.py", "config.json"}
	workItems := []WorkItem{
		{ContentType: "go", Size: 1000, EstimatedTime: 2.0},
		{ContentType: "python", Size: 500, EstimatedTime: 1.0},
		{ContentType: "json", Size: 100, EstimatedTime: 0.5},
	}

	stats := planner.calculateStatistics(workItems, files)

	// Check file type distribution
	if stats.FilesByType["go"] != 1 {
		t.Errorf("expected 1 go file, got %d", stats.FilesByType["go"])
	}
	if stats.FilesByType["python"] != 1 {
		t.Errorf("expected 1 python file, got %d", stats.FilesByType["python"])
	}
	if stats.FilesByType["json"] != 1 {
		t.Errorf("expected 1 json file, got %d", stats.FilesByType["json"])
	}

	// Check size statistics
	expectedTotal := int64(1600)
	if stats.SizeDistribution.TotalSize != expectedTotal {
		t.Errorf("expected total size %d, got %d", expectedTotal, stats.SizeDistribution.TotalSize)
	}

	if stats.SizeDistribution.MinSize != 100 {
		t.Errorf("expected min size 100, got %d", stats.SizeDistribution.MinSize)
	}

	if stats.SizeDistribution.MaxSize != 1000 {
		t.Errorf("expected max size 1000, got %d", stats.SizeDistribution.MaxSize)
	}

	// Check execution estimates
	if stats.EstimatedExecutionTime.Sequential <= 0 {
		t.Error("expected positive sequential execution time")
	}

	if stats.EstimatedExecutionTime.Parallel2 >= stats.EstimatedExecutionTime.Sequential {
		t.Error("expected parallel execution to be faster than sequential")
	}
}

func TestGenerateManifest(t *testing.T) {
	// Create a temporary directory with test files
	tempDir, err := os.MkdirTemp("", "manifest_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Errorf("Failed to clean up temp dir: %v", err)
		}
	}()

	// Create test files
	testFiles := []string{"main.go", "utils.go", "config.json"}
	for _, file := range testFiles {
		filePath := filepath.Join(tempDir, file)
		if err := os.WriteFile(filePath, []byte("test content"), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", file, err)
		}
	}

	config := PlannerConfig{
		Command:            "format",
		Paths:              []string{tempDir},
		ExecutionStrategy:  "parallel",
		GroupByContentType: true,
	}

	planner := NewPlanner(config)
	manifest, err := planner.GenerateManifest()

	if err != nil {
		t.Fatalf("Failed to generate manifest: %v", err)
	}

	if manifest == nil {
		t.Fatal("expected manifest to be generated, got nil")
	}

	// Check plan
	if manifest.Plan.Command != "format" {
		t.Errorf("expected command 'format', got '%s'", manifest.Plan.Command)
	}

	if manifest.Plan.ExecutionStrategy != "parallel" {
		t.Errorf("expected execution strategy 'parallel', got '%s'", manifest.Plan.ExecutionStrategy)
	}

	if manifest.Plan.TotalFiles != len(testFiles) {
		t.Errorf("expected %d total files, got %d", len(testFiles), manifest.Plan.TotalFiles)
	}

	// Check work items
	if len(manifest.WorkItems) != len(testFiles) {
		t.Errorf("expected %d work items, got %d", len(testFiles), len(manifest.WorkItems))
	}

	// Check groups
	if len(manifest.Groups) == 0 {
		t.Error("expected at least one group")
	}

	// Check statistics
	if len(manifest.Statistics.FilesByType) == 0 {
		t.Error("expected file type statistics")
	}

	// Check timestamp is recent
	if time.Since(manifest.Plan.Timestamp) > time.Minute {
		t.Error("expected recent timestamp")
	}
}

func TestMatchesIgnorePattern(t *testing.T) {
	// Create temporary directory with ignore file
	tempDir, err := os.MkdirTemp("", "ignore_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Errorf("Failed to clean up temp dir: %v", err)
		}
	}()

	// Change to temp directory for the test
	originalDir, _ := os.Getwd()
	defer func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Errorf("Failed to restore original directory: %v", err)
		}
	}()

	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Create .goneatignore file
	ignoreContent := []byte("*.log\n*.tmp\nnode_modules/\n.git/\n")
	if err := os.WriteFile(".goneatignore", ignoreContent, 0644); err != nil {
		t.Fatalf("Failed to create .goneatignore: %v", err)
	}

	planner := NewPlanner(PlannerConfig{})

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "log file should be ignored",
			path:     "app.log",
			expected: true,
		},
		{
			name:     "tmp file should be ignored",
			path:     "temp.tmp",
			expected: true,
		},
		{
			name:     "go file should not be ignored",
			path:     "main.go",
			expected: false,
		},
		{
			name:     "node_modules should be ignored",
			path:     "node_modules/package/index.js",
			expected: true,
		},
		{
			name:     ".git should be ignored",
			path:     ".git/config",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := planner.matchesIgnorePattern(tt.path)
			if result != tt.expected {
				t.Errorf("expected %v for path '%s', got %v", tt.expected, tt.path, result)
			}
		})
	}
}

func TestGetWorkingDirectory(t *testing.T) {
	planner := NewPlanner(PlannerConfig{})

	workDir := planner.getWorkingDirectory()

	if workDir == "" {
		t.Error("expected non-empty working directory")
	}

	// Should be an absolute path
	if !filepath.IsAbs(workDir) {
		t.Error("expected absolute path for working directory")
	}
}

func TestPlannerConfigDefaults(t *testing.T) {
	config := PlannerConfig{}
	planner := NewPlanner(config)

	// Test that planner handles empty config gracefully
	if planner == nil {
		t.Fatal("expected planner to be created even with empty config")
	}

	// Test with minimal config
	config = PlannerConfig{
		Command: "test",
		Paths:   []string{"."},
	}
	planner = NewPlanner(config)

	if planner.config.Command != "test" {
		t.Errorf("expected command 'test', got '%s'", planner.config.Command)
	}

	if len(planner.config.Paths) != 1 || planner.config.Paths[0] != "." {
		t.Errorf("expected paths ['.'], got %v", planner.config.Paths)
	}
}

// TestWorkItemStructure tests the work item data structure
func TestWorkItemStructure(t *testing.T) {
	item := WorkItem{
		ID:            "test-id",
		Path:          "test.go",
		ContentType:   "go",
		Size:          1024,
		Priority:      1,
		EstimatedTime: 2.5,
		Dependencies:  []string{"dep1", "dep2"},
		Metadata:      map[string]interface{}{"key": "value"},
	}

	if item.ID != "test-id" {
		t.Errorf("expected ID 'test-id', got '%s'", item.ID)
	}

	if item.ContentType != "go" {
		t.Errorf("expected content type 'go', got '%s'", item.ContentType)
	}

	if len(item.Dependencies) != 2 {
		t.Errorf("expected 2 dependencies, got %d", len(item.Dependencies))
	}

	if item.Metadata["key"] != "value" {
		t.Errorf("expected metadata key 'value', got '%v'", item.Metadata["key"])
	}
}

// TestWorkGroupStructure tests the work group data structure
func TestWorkGroupStructure(t *testing.T) {
	group := WorkGroup{
		ID:                         "group-1",
		Name:                       "Go Files",
		Strategy:                   "parallel",
		WorkItemIDs:                []string{"item1", "item2"},
		EstimatedTotalTime:         5.0,
		RecommendedParallelization: 2,
	}

	if group.Name != "Go Files" {
		t.Errorf("expected name 'Go Files', got '%s'", group.Name)
	}

	if len(group.WorkItemIDs) != 2 {
		t.Errorf("expected 2 work item IDs, got %d", len(group.WorkItemIDs))
	}

	if group.RecommendedParallelization != 2 {
		t.Errorf("expected parallelization 2, got %d", group.RecommendedParallelization)
	}
}

func TestMatchesForceInclude(t *testing.T) {
	testCases := []struct {
		name                 string
		forceIncludePatterns []string
		path                 string
		expected             bool
	}{
		{
			name:                 "no patterns",
			forceIncludePatterns: []string{},
			path:                 "test.go",
			expected:             false,
		},
		{
			name:                 "exact match",
			forceIncludePatterns: []string{"test.go"},
			path:                 "test.go",
			expected:             true,
		},
		{
			name:                 "glob pattern match",
			forceIncludePatterns: []string{"*.go"},
			path:                 "main.go",
			expected:             true,
		},
		{
			name:                 "recursive directory pattern",
			forceIncludePatterns: []string{"src/**"},
			path:                 "src/main.go",
			expected:             true,
		},
		{
			name:                 "recursive directory pattern deep",
			forceIncludePatterns: []string{"src/**"},
			path:                 "src/utils/helper.go",
			expected:             true,
		},
		{
			name:                 "no match",
			forceIncludePatterns: []string{"*.js"},
			path:                 "main.go",
			expected:             false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := PlannerConfig{
				ForceIncludePatterns: tc.forceIncludePatterns,
			}
			planner := NewPlanner(config)
			
			result := planner.matchesForceInclude(tc.path)
			if result != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestDirHasForcedDescendant(t *testing.T) {
	testCases := []struct {
		name                 string
		forceIncludePatterns []string
		dir                  string
		expected             bool
	}{
		{
			name:                 "no patterns",
			forceIncludePatterns: []string{},
			dir:                  "src",
			expected:             false,
		},
		{
			name:                 "pattern under directory",
			forceIncludePatterns: []string{"src/main.go"},
			dir:                  "src",
			expected:             true,
		},
		{
			name:                 "recursive pattern",
			forceIncludePatterns: []string{"src/**"},
			dir:                  "src",
			expected:             true,
		},
		{
			name:                 "pattern not under directory",
			forceIncludePatterns: []string{"lib/helper.go"},
			dir:                  "src",
			expected:             false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := PlannerConfig{
				ForceIncludePatterns: tc.forceIncludePatterns,
			}
			planner := NewPlanner(config)
			
			result := planner.dirHasForcedDescendant(tc.dir)
			if result != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestPathMatch(t *testing.T) {
	testCases := []struct {
		name     string
		pattern  string
		path     string
		expected bool
	}{
		{
			name:     "exact match",
			pattern:  "main.go",
			path:     "main.go",
			expected: true,
		},
		{
			name:     "glob match",
			pattern:  "*.go",
			path:     "main.go",
			expected: true,
		},
		{
			name:     "glob no match",
			pattern:  "*.js",
			path:     "main.go",
			expected: false,
		},
		{
			name:     "directory match",
			pattern:  "src/*",
			path:     "src/main.go",
			expected: true,
		},
		{
			name:     "no wildcards exact match",
			pattern:  "test.txt",
			path:     "test.txt",
			expected: true,
		},
		{
			name:     "no wildcards no match",
			pattern:  "test.txt",
			path:     "other.txt",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := pathMatch(tc.pattern, tc.path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestGroupBySize(t *testing.T) {
	planner := NewPlanner(PlannerConfig{})
	
	testItems := []WorkItem{
		{ID: "small1", Path: "test1.go", EstimatedTime: 0.010},  // 10ms as float64
		{ID: "small2", Path: "test2.go", EstimatedTime: 0.020},  // 20ms as float64
		{ID: "large1", Path: "test3.go", EstimatedTime: 0.100},  // 100ms as float64  
		{ID: "large2", Path: "test4.go", EstimatedTime: 0.150},  // 150ms as float64
	}

	groups := planner.groupBySize(testItems)
	
	if len(groups) == 0 {
		t.Error("expected at least one group, got none")
	}
	
	// Verify all items are included
	totalItems := 0
	for _, group := range groups {
		totalItems += len(group.WorkItemIDs)
	}
	
	if totalItems != len(testItems) {
		t.Errorf("expected %d items total, got %d", len(testItems), totalItems)
	}
}

func TestCreateSizeGroup(t *testing.T) {
	planner := NewPlanner(PlannerConfig{})
	
	items := []WorkItem{
		{ID: "item1", Path: "test1.go", EstimatedTime: 0.010},  // 10ms as float64
		{ID: "item2", Path: "test2.go", EstimatedTime: 0.020},  // 20ms as float64
	}
	
	group := planner.createSizeGroup("test-id", "test-size", items)
	
	if group.ID != "test-id" {
		t.Errorf("expected group ID 'test-id', got '%s'", group.ID)
	}
	
	if group.Name != "test-size" {
		t.Errorf("expected group name 'test-size', got '%s'", group.Name)
	}
	
	if len(group.WorkItemIDs) != 2 {
		t.Errorf("expected 2 items, got %d", len(group.WorkItemIDs))
	}
	
	if group.Strategy != "size_based" {
		t.Errorf("expected size_based strategy, got %s", group.Strategy)
	}
	
	if group.EstimatedTotalTime <= 0 {
		t.Error("expected positive estimated time")
	}
}
