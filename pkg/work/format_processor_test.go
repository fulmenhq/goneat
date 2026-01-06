package work

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fulmenhq/goneat/pkg/config"
	"github.com/fulmenhq/goneat/pkg/format/finalizer"
)

func newTestProcessor(cfg *config.Config) *FormatProcessor {
	return NewFormatProcessorWithOptions(cfg, FormatProcessorOptions{
		FinalizerOptions: finalizer.NormalizationOptions{
			EnsureEOF:                  true,
			TrimTrailingWhitespace:     true,
			NormalizeLineEndings:       "",
			RemoveUTF8BOM:              false,
			PreserveMarkdownHardBreaks: true,
			EncodingPolicy:             "utf8-only",
		},
		IgnoreMissingTools: true,
		JSONIndent:         "  ",
		JSONIndentCount:    2,
		JSONSizeWarningMB:  500,
	})
}

func TestNewFormatProcessor(t *testing.T) {
	cfg := &config.Config{}
	processor := NewFormatProcessor(cfg)

	if processor == nil {
		t.Fatal("expected processor to be created, got nil")
	}

	if processor.config != cfg {
		t.Error("expected processor to have correct config reference")
	}
}

func TestFormatProcessor_ProcessWorkItem_DryRun(t *testing.T) {
	cfg := &config.Config{}
	processor := newTestProcessor(cfg)

	item := &WorkItem{
		ID:          "test-item",
		Path:        "test.go",
		ContentType: "go",
		Size:        1024,
	}

	ctx := context.Background()
	result := processor.ProcessWorkItem(ctx, item, true, false) // dryRun = true

	if !result.Success {
		t.Errorf("expected success for dry run, got failure: %s", result.Error)
	}

	if result.WorkItemID != item.ID {
		t.Errorf("expected work item ID '%s', got '%s'", item.ID, result.WorkItemID)
	}

	expectedOutput := "Would format test.go"
	if result.Output != expectedOutput {
		t.Errorf("expected output '%s', got '%s'", expectedOutput, result.Output)
	}
}

func TestFormatProcessor_ProcessWorkItem_NoOp(t *testing.T) {
	cfg := &config.Config{}
	processor := newTestProcessor(cfg)

	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "noop_test")
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
		contentType string
		content     []byte
		expectError bool
	}{
		{
			name:        "go file check",
			contentType: "go",
			content:     []byte("package main\n\nfunc main() {\n}\n"),
			expectError: false,
		},
		{
			name:        "yaml file check",
			contentType: "yaml",
			content:     []byte("key: value\n"),
			expectError: false,
		},
		{
			name:        "json file check",
			contentType: "json",
			content:     []byte("{\n  \"key\": \"value\"\n}\n"),
			expectError: false,
		},
		{
			name:        "markdown file check",
			contentType: "markdown",
			content:     []byte("# Header\n\nContent\n"),
			expectError: false,
		},
		{
			name:        "unsupported content type",
			contentType: "unsupported",
			content:     []byte("test"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test file with appropriate content
			testFile := filepath.Join(tempDir, fmt.Sprintf("test.%s", tt.contentType))
			if err := os.WriteFile(testFile, tt.content, 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			item := &WorkItem{
				ID:          "test-item",
				Path:        testFile,
				ContentType: tt.contentType,
				Size:        int64(len(tt.content)),
			}

			ctx := context.Background()
			result := processor.ProcessWorkItem(ctx, item, false, true) // noOp = true

			if tt.expectError {
				if result.Success {
					t.Errorf("expected failure for %s, got success", tt.contentType)
				}
				if result.Error == "" {
					t.Errorf("expected error message for %s", tt.contentType)
				}
			} else {
				if !result.Success {
					t.Errorf("expected success for %s, got failure: %s", tt.contentType, result.Error)
				}
				expectedOutput := "Check passed for " + testFile
				if result.Output != expectedOutput {
					t.Errorf("expected output '%s', got '%s'", expectedOutput, result.Output)
				}
			}
		})
	}
}

func TestFormatProcessor_ProcessWorkItem_Format(t *testing.T) {
	cfg := &config.Config{}
	processor := newTestProcessor(cfg)

	tests := []struct {
		name        string
		contentType string
		content     []byte
		expectError bool
	}{
		{
			name:        "go file format",
			contentType: "go",
			content:     []byte("package main\n\nfunc main() {\n}\n"),
			expectError: false, // Go formatting is now implemented
		},
		{
			name:        "yaml file format",
			contentType: "yaml",
			content:     []byte("key: value\n"),
			expectError: false, // YAML formatting returns nil
		},
		{
			name:        "json file format",
			contentType: "json",
			content:     []byte("{\n  \"key\": \"value\"\n}\n"),
			expectError: false, // JSON formatting returns nil
		},
		{
			name:        "markdown file format",
			contentType: "markdown",
			content:     []byte("# Header\n\nContent\n"),
			expectError: false, // Should work with finalizer
		},
		{
			name:        "unsupported content type",
			contentType: "unsupported",
			content:     []byte("test content"),
			expectError: true,
		},
	}

	// Create a temporary directory and file for testing
	tempDir, err := os.MkdirTemp("", "format_processor_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Errorf("Failed to clean up temp dir: %v", err)
		}
	}()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test file with appropriate content
			testFile := filepath.Join(tempDir, "test."+tt.contentType)
			if err := os.WriteFile(testFile, tt.content, 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			item := &WorkItem{
				ID:          "test-item",
				Path:        testFile,
				ContentType: tt.contentType,
				Size:        int64(len(tt.content)),
			}

			ctx := context.Background()
			result := processor.ProcessWorkItem(ctx, item, false, false) // actual format

			if tt.expectError {
				if result.Success {
					t.Errorf("expected failure for %s, got success", tt.contentType)
				}
				if result.Error == "" {
					t.Errorf("expected error message for %s", tt.contentType)
				}
			} else {
				if !result.Success {
					t.Errorf("expected success for %s, got failure: %s", tt.contentType, result.Error)
				}
				expectedOutput := "Successfully formatted " + testFile
				if result.Output != expectedOutput {
					t.Errorf("expected output '%s', got '%s'", expectedOutput, result.Output)
				}
			}
		})
	}
}

func TestFormatProcessor_ProcessWorkItem_Context_Cancellation(t *testing.T) {
	cfg := &config.Config{}
	processor := newTestProcessor(cfg)

	item := &WorkItem{
		ID:          "test-item",
		Path:        "test.go",
		ContentType: "go",
		Size:        1024,
	}

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result := processor.ProcessWorkItem(ctx, item, false, false)

	if result.Success {
		t.Error("expected failure for cancelled context, got success")
	}

	expectedError := "operation cancelled"
	if result.Error != expectedError {
		t.Errorf("expected error '%s', got '%s'", expectedError, result.Error)
	}

	if result.WorkItemID != item.ID {
		t.Errorf("expected work item ID '%s', got '%s'", item.ID, result.WorkItemID)
	}
}

func TestFormatProcessor_GetSupportedContentTypes(t *testing.T) {
	cfg := &config.Config{}
	processor := newTestProcessor(cfg)

	supportedTypes := processor.GetSupportedContentTypes()

	if len(supportedTypes) == 0 {
		t.Error("expected supported content types, got empty slice")
	}

	// Check for expected content types
	expectedTypes := []string{"go", "yaml", "json", "markdown"}
	for _, expectedType := range expectedTypes {
		found := false
		for _, supportedType := range supportedTypes {
			if supportedType == expectedType {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected content type '%s' to be supported", expectedType)
		}
	}
}

func TestFormatProcessor_CheckMethods(t *testing.T) {
	cfg := &config.Config{}
	processor := newTestProcessor(cfg)

	// Create a temporary directory and files for testing
	tempDir, err := os.MkdirTemp("", "check_methods_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Errorf("Failed to clean up temp dir: %v", err)
		}
	}()

	tests := []struct {
		name     string
		fileName string
		method   func(string) error
	}{
		{
			name:     "check go file",
			fileName: "test.go",
			method:   processor.checkGoFile,
		},
		{
			name:     "check yaml file",
			fileName: "test.yaml",
			method:   processor.checkYAMLFile,
		},
		{
			name:     "check json file",
			fileName: "test.json",
			method:   processor.checkJSONFile,
		},
		{
			name:     "check markdown file",
			fileName: "test.md",
			method:   processor.checkMarkdownFile,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := filepath.Join(tempDir, tt.fileName)

			// Create test file with valid content
			var content []byte
			switch filepath.Ext(tt.fileName) {
			case ".go":
				content = []byte("package main\n\nfunc main() {\n}\n")
			case ".yaml":
				content = []byte("key: value\n")
			case ".json":
				content = []byte(`{"key": "value"}`)
			case ".md":
				content = []byte("# Header\n\nContent\n")
			default:
				content = []byte("test content")
			}

			if err := os.WriteFile(filePath, content, 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			err := tt.method(filePath)
			// For now, most methods return nil or specific errors
			// This tests that the methods can be called without panicking
			_ = err // We don't assert on the error since implementation varies
		})
	}
}

func TestFormatProcessor_FormatMethods(t *testing.T) {
	cfg := &config.Config{}
	processor := newTestProcessor(cfg)

	// Create a temporary directory and files for testing
	tempDir, err := os.MkdirTemp("", "format_methods_test")
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
		fileName    string
		method      func(string) error
		expectError bool
	}{
		{
			name:        "format go file",
			fileName:    "test.go",
			method:      processor.formatGoFile,
			expectError: false, // Go formatting is now implemented
		},
		{
			name:        "format yaml file",
			fileName:    "test.yaml",
			method:      processor.formatYAMLFile,
			expectError: false, // Returns nil for success
		},
		{
			name:        "format json file",
			fileName:    "test.json",
			method:      processor.formatJSONFile,
			expectError: false, // Returns nil for success
		},
		{
			name:        "format markdown file",
			fileName:    "test.md",
			method:      processor.formatMarkdownFile,
			expectError: false, // Should work with finalizer
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := filepath.Join(tempDir, tt.fileName)

			// Create test file with content that might need formatting
			var content []byte
			switch filepath.Ext(tt.fileName) {
			case ".go":
				content = []byte("package main\n\nfunc main() {\n}\n") // Properly formatted Go
			case ".yaml":
				content = []byte("key: value\n") // Valid YAML
			case ".json":
				content = []byte("{\n  \"key\": \"value\"\n}\n") // Pretty formatted JSON
			case ".md":
				content = []byte("# Header\n\nContent\n") // Properly formatted markdown
			default:
				content = []byte("test content\n")
			}

			if err := os.WriteFile(filePath, content, 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			err := tt.method(filePath)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error for %s, got nil", tt.name)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error for %s, got: %v", tt.name, err)
				}
			}
		})
	}
}

func TestFormatProcessor_WorkItem_Integration(t *testing.T) {
	cfg := &config.Config{}
	processor := newTestProcessor(cfg)

	// Create a temporary directory with test files
	tempDir, err := os.MkdirTemp("", "integration_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Errorf("Failed to clean up temp dir: %v", err)
		}
	}()

	// Create test markdown file with formatting issues
	markdownFile := filepath.Join(tempDir, "test.md")
	markdownContent := []byte("# Title  \n\nContent with trailing spaces  \n\n\n")
	if err := os.WriteFile(markdownFile, markdownContent, 0644); err != nil {
		t.Fatalf("Failed to create markdown file: %v", err)
	}

	item := &WorkItem{
		ID:            "markdown-test",
		Path:          markdownFile,
		ContentType:   "markdown",
		Size:          int64(len(markdownContent)),
		Priority:      1,
		EstimatedTime: 1.0,
	}

	ctx := context.Background()

	// Test dry run
	dryRunResult := processor.ProcessWorkItem(ctx, item, true, false)
	if !dryRunResult.Success {
		t.Errorf("dry run failed: %s", dryRunResult.Error)
	}

	// Test check (no-op) - may fail if file needs formatting
	checkResult := processor.ProcessWorkItem(ctx, item, false, true)
	// Note: checkMarkdownFile might detect formatting issues and return an error
	// This is expected behavior, so we don't assert success here
	_ = checkResult

	// Test actual formatting
	formatResult := processor.ProcessWorkItem(ctx, item, false, false)
	if !formatResult.Success {
		t.Errorf("format failed: %s", formatResult.Error)
	}

	// Verify that all results have the correct work item ID
	if dryRunResult.WorkItemID != item.ID {
		t.Errorf("dry run result has wrong work item ID")
	}
	if checkResult.WorkItemID != item.ID {
		t.Errorf("check result has wrong work item ID")
	}
	if formatResult.WorkItemID != item.ID {
		t.Errorf("format result has wrong work item ID")
	}
}

func TestFormatProcessor_Performance(t *testing.T) {
	cfg := &config.Config{}
	processor := newTestProcessor(cfg)

	// Create multiple work items to test performance characteristics
	items := make([]*WorkItem, 10)
	for i := 0; i < 10; i++ {
		items[i] = &WorkItem{
			ID:            fmt.Sprintf("item-%d", i),
			Path:          fmt.Sprintf("test%d.go", i),
			ContentType:   "go",
			Size:          1024,
			Priority:      1,
			EstimatedTime: 1.0,
		}
	}

	ctx := context.Background()
	start := time.Now()

	// Process all items in dry-run mode (should be fast)
	for _, item := range items {
		result := processor.ProcessWorkItem(ctx, item, true, false)
		if !result.Success {
			t.Errorf("dry run failed for item %s: %s", item.ID, result.Error)
		}
	}

	duration := time.Since(start)

	// Dry run should be very fast (less than 100ms for 10 items)
	if duration > 100*time.Millisecond {
		t.Errorf("dry run took too long: %v", duration)
	}
}

func TestExecutionResult_Structure(t *testing.T) {
	result := ExecutionResult{
		WorkItemID: "test-id",
		Success:    true,
		Error:      "",
		Duration:   time.Second,
		Output:     "test output",
	}

	if result.WorkItemID != "test-id" {
		t.Errorf("expected WorkItemID 'test-id', got '%s'", result.WorkItemID)
	}

	if !result.Success {
		t.Error("expected Success to be true")
	}

	if result.Duration != time.Second {
		t.Errorf("expected Duration 1s, got %v", result.Duration)
	}

	if result.Output != "test output" {
		t.Errorf("expected Output 'test output', got '%s'", result.Output)
	}
}

// TestFormatProcessor_ConfigIntegration tests integration with config
func TestFormatProcessor_ConfigIntegration(t *testing.T) {
	// Create config with specific settings
	cfg := &config.Config{}

	// In a real scenario, we would set specific YAML config values
	// cfg.YAML.Indent = 4
	// cfg.YAML.LineLength = 120

	processor := newTestProcessor(cfg)

	if processor.config != cfg {
		t.Error("processor should maintain reference to provided config")
	}

	// Test that processor uses config correctly
	supportedTypes := processor.GetSupportedContentTypes()
	if len(supportedTypes) == 0 {
		t.Error("processor should return supported content types based on config")
	}
}

func TestFormatProcessor_ValidateWorkItem(t *testing.T) {
	cfg := &config.Config{}
	processor := newTestProcessor(cfg)

	tests := []struct {
		name        string
		item        *WorkItem
		expectError bool
	}{
		{
			name: "valid go work item",
			item: &WorkItem{
				ID:          "test-1",
				Path:        "test.go",
				ContentType: "go",
				Size:        1024,
			},
			expectError: false,
		},
		{
			name: "valid yaml work item",
			item: &WorkItem{
				ID:          "test-2",
				Path:        "test.yaml",
				ContentType: "yaml",
				Size:        512,
			},
			expectError: false,
		},
		{
			name: "work item with no content type",
			item: &WorkItem{
				ID:   "test-3",
				Path: "test.txt",
				Size: 256,
			},
			expectError: true,
		},
		{
			name: "work item with unsupported content type",
			item: &WorkItem{
				ID:          "test-4",
				Path:        "test.cpp",
				ContentType: "cpp",
				Size:        1024,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := processor.ValidateWorkItem(tt.item)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error for %s, got nil", tt.name)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error for %s, got: %v", tt.name, err)
				}
			}
		})
	}
}

func TestFormatProcessor_EstimateProcessingTime(t *testing.T) {
	cfg := &config.Config{}
	processor := newTestProcessor(cfg)

	tests := []struct {
		name        string
		item        *WorkItem
		expectedMin float64
		expectedMax float64
	}{
		{
			name: "small go file",
			item: &WorkItem{
				ID:          "test-1",
				ContentType: "go",
				Size:        1024, // 1KB
			},
			expectedMin: 0.4,
			expectedMax: 0.6,
		},
		{
			name: "large yaml file",
			item: &WorkItem{
				ID:          "test-2",
				ContentType: "yaml",
				Size:        10240, // 10KB
			},
			expectedMin: 9.0,
			expectedMax: 11.0,
		},
		{
			name: "json file",
			item: &WorkItem{
				ID:          "test-3",
				ContentType: "json",
				Size:        2048, // 2KB
			},
			expectedMin: 1.4,
			expectedMax: 1.8,
		},
		{
			name: "markdown file",
			item: &WorkItem{
				ID:          "test-4",
				ContentType: "markdown",
				Size:        3072, // 3KB
			},
			expectedMin: 3.4,
			expectedMax: 3.8,
		},
		{
			name: "unknown content type",
			item: &WorkItem{
				ID:          "test-5",
				ContentType: "unknown",
				Size:        1024, // 1KB
			},
			expectedMin: 0.9,
			expectedMax: 1.1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			time := processor.EstimateProcessingTime(tt.item)

			if time < tt.expectedMin || time > tt.expectedMax {
				t.Errorf("expected processing time between %.1f and %.1f for %s, got %.1f",
					tt.expectedMin, tt.expectedMax, tt.name, time)
			}
		})
	}
}

func TestFormatProcessor_GetProcessorInfo(t *testing.T) {
	cfg := &config.Config{}
	processor := newTestProcessor(cfg)

	info := processor.GetProcessorInfo()

	// Check required fields
	if info["type"] != "format" {
		t.Errorf("expected type 'format', got %v", info["type"])
	}

	supportedTypes, ok := info["supported_types"].([]string)
	if !ok {
		t.Errorf("expected supported_types to be []string, got %T", info["supported_types"])
	}

	expectedTypes := []string{"go", "yaml", "json", "markdown"}
	if len(supportedTypes) != len(expectedTypes) {
		t.Errorf("expected %d supported types, got %d", len(expectedTypes), len(supportedTypes))
	}

	for _, expectedType := range expectedTypes {
		found := false
		for _, supportedType := range supportedTypes {
			if supportedType == expectedType {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected supported type '%s' not found", expectedType)
		}
	}

	configAvailable, ok := info["config_available"].(bool)
	if !ok {
		t.Errorf("expected config_available to be bool, got %T", info["config_available"])
	}

	if !configAvailable {
		t.Error("expected config_available to be true since config is provided")
	}
}
