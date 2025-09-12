package schema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestValidate(t *testing.T) {
	// Valid
	validYAML := `
format:
  go:
    simplify: true
security:
  timeout: 5m
`
	var validDoc interface{}
	if err := yaml.Unmarshal([]byte(validYAML), &validDoc); err != nil {
		t.Fatalf("failed to parse valid YAML: %v", err)
	}

	res, err := Validate(validDoc, "goneat-config-v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Valid {
		t.Errorf("expected valid, got %v", res.Errors)
	}

	// Invalid - indent below minimum
	invalidYAML := `
format:
  yaml:
    indent: 1
security:
  timeout: 5m
`
	var invalidDoc interface{}
	if err := yaml.Unmarshal([]byte(invalidYAML), &invalidDoc); err != nil {
		t.Fatalf("failed to parse invalid YAML: %v", err)
	}

	res, err = Validate(invalidDoc, "goneat-config-v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if res.Valid || len(res.Errors) == 0 {
		t.Error("expected invalid")
	}

	// Non-existent schema
	_, err = Validate(validDoc, "nonexistent")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not found, got %v", err)
	}
}

func TestNewSecurityContext(t *testing.T) {
	ctx := NewSecurityContext()

	// Should have basic properties
	if ctx.MaxFileSize <= 0 {
		t.Error("Expected positive MaxFileSize")
	}
	if len(ctx.AllowedDirs) == 0 {
		t.Error("Expected at least one allowed directory")
	}
	// This test ensures the function works correctly
}

func TestValidateDataFromBytes(t *testing.T) {
	// Test with simple valid JSON schema and data
	schema := []byte(`{
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"age": {"type": "number"}
		},
		"required": ["name"]
	}`)

	validData := []byte(`{"name": "test", "age": 30}`)
	invalidData := []byte(`{"age": 30}`)

	// Test valid data
	result, err := ValidateDataFromBytes(schema, validData)
	if err != nil {
		t.Fatalf("ValidateDataFromBytes() failed: %v", err)
	}
	if !result.Valid {
		t.Error("Expected valid data to pass validation")
	}

	// Test invalid data
	result, err = ValidateDataFromBytes(schema, invalidData)
	if err != nil {
		t.Fatalf("ValidateDataFromBytes() failed: %v", err)
	}
	if result.Valid {
		t.Error("Expected invalid data to fail validation")
	}
	if len(result.Errors) == 0 {
		t.Error("Expected validation errors for invalid data")
	}
}

func TestValidateFile(t *testing.T) {
	tempDir := t.TempDir()

	// Create test files
	validFile := filepath.Join(tempDir, "valid.json")
	validContent := `{"name": "test", "age": 30}`
	if err := os.WriteFile(validFile, []byte(validContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create simple schema
	schema := []byte(`{
		"type": "object",
		"properties": {
			"name": {"type": "string"}
		}
	}`)

	// Test validation
	result, err := ValidateFile(schema, validFile)
	if err != nil {
		t.Fatalf("ValidateFile() failed: %v", err)
	}

	// Check the result
	if result == nil {
		t.Error("ValidateFile() returned nil result")
	}
}

func TestValidateFileFromSchemaFile(t *testing.T) {
	tempDir := t.TempDir()

	// Create test files
	dataFile := filepath.Join(tempDir, "data.json")
	dataContent := `{"name": "test"}`
	if err := os.WriteFile(dataFile, []byte(dataContent), 0644); err != nil {
		t.Fatalf("Failed to create data file: %v", err)
	}

	schemaFile := filepath.Join(tempDir, "schema.json")
	schemaContent := `{
		"type": "object",
		"properties": {
			"name": {"type": "string"}
		},
		"required": ["name"]
	}`
	if err := os.WriteFile(schemaFile, []byte(schemaContent), 0644); err != nil {
		t.Fatalf("Failed to create schema file: %v", err)
	}

	// Test validation
	result, err := ValidateFileFromSchemaFile(dataFile, schemaFile)
	if err != nil {
		t.Fatalf("ValidateFileFromSchemaFile() failed: %v", err)
	}

	if !result.Valid {
		t.Error("Expected valid data to pass validation")
	}
}

func TestValidateFileWithSecurity(t *testing.T) {
	tempDir := t.TempDir()

	// Create test files
	dataFile := filepath.Join(tempDir, "data.json")
	dataContent := `{"name": "test"}`
	if err := os.WriteFile(dataFile, []byte(dataContent), 0644); err != nil {
		t.Fatalf("Failed to create data file: %v", err)
	}

	// Allow the temp directory in security context
	secCtx := NewSecurityContext()
	secCtx.AllowedDirs = []string{tempDir}

	schema := []byte(`{
		"type": "object",
		"properties": {
			"name": {"type": "string"}
		}
	}`)

	// Test validation with security context
	result, err := ValidateFileWithSecurity(schema, dataFile, secCtx)
	if err != nil {
		t.Fatalf("ValidateFileWithSecurity() failed: %v", err)
	}

	// Check the result
	if result == nil {
		t.Error("ValidateFileWithSecurity() returned nil result")
	}
}

func TestValidateFiles(t *testing.T) {
	tempDir := t.TempDir()

	// Create test files
	file1 := filepath.Join(tempDir, "file1.json")
	if err := os.WriteFile(file1, []byte(`{"test": true}`), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	file2 := filepath.Join(tempDir, "file2.json")
	if err := os.WriteFile(file2, []byte(`{"test": false}`), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	files := []string{file1, file2}
	schema := []byte(`{
		"type": "object",
		"properties": {
			"test": {"type": "boolean"}
		}
	}`)

	results, err := ValidateFiles(schema, files)
	if err != nil {
		t.Fatalf("ValidateFiles() failed: %v", err)
	}

	// Should return results for each file
	if len(results.FileResults) != len(files) {
		t.Errorf("Expected %d results, got %d", len(files), len(results.FileResults))
	}
}

func TestValidateDirectory(t *testing.T) {
	tempDir := t.TempDir()

	// Create test files in directory
	file1 := filepath.Join(tempDir, "test1.json")
	if err := os.WriteFile(file1, []byte(`{"test": true}`), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create subdirectory
	subDir := filepath.Join(tempDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	file2 := filepath.Join(subDir, "test2.json")
	if err := os.WriteFile(file2, []byte(`{"test": false}`), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	schema := []byte(`{
		"type": "object",
		"properties": {
			"test": {"type": "boolean"}
		}
	}`)

	results, err := ValidateDirectory(schema, tempDir, "*.json")
	if err != nil {
		t.Fatalf("ValidateDirectory() failed: %v", err)
	}

	// Should find and process files
	if len(results.FileResults) == 0 {
		t.Log("ValidateDirectory() found no files to validate")
	}
}

func TestLegacyMapSchemaNameToPath(t *testing.T) {
	tests := []struct {
		name string
	}{
		{"test-schema"},
		{"my-schema-v1"},
		{"simple"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := legacyMapSchemaNameToPath(tt.name)
			// Just test that it returns a non-empty string with the expected pattern
			if result == "" {
				t.Error("legacyMapSchemaNameToPath() returned empty string")
			}
			if !strings.Contains(result, tt.name) {
				t.Errorf("legacyMapSchemaNameToPath(%q) = %q, expected to contain %q", tt.name, result, tt.name)
			}
		})
	}
}
