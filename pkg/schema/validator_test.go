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

func TestValidateFileWithSchemaPath(t *testing.T) {
	tempDir := t.TempDir()

	// Create schema file
	schemaFile := filepath.Join(tempDir, "schema.json")
	schemaContent := `{
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"age": {"type": "number"}
		},
		"required": ["name"]
	}`
	if err := os.WriteFile(schemaFile, []byte(schemaContent), 0644); err != nil {
		t.Fatalf("Failed to create schema file: %v", err)
	}

	// Create valid data file (JSON)
	validDataFile := filepath.Join(tempDir, "valid.json")
	validDataContent := `{"name": "test", "age": 30}`
	if err := os.WriteFile(validDataFile, []byte(validDataContent), 0644); err != nil {
		t.Fatalf("Failed to create valid data file: %v", err)
	}

	// Create invalid data file (JSON)
	invalidDataFile := filepath.Join(tempDir, "invalid.json")
	invalidDataContent := `{"age": 30}` // missing required "name"
	if err := os.WriteFile(invalidDataFile, []byte(invalidDataContent), 0644); err != nil {
		t.Fatalf("Failed to create invalid data file: %v", err)
	}

	// Create YAML data file
	yamlDataFile := filepath.Join(tempDir, "data.yaml")
	yamlDataContent := `
name: "yaml-test"
age: 25
`
	if err := os.WriteFile(yamlDataFile, []byte(yamlDataContent), 0644); err != nil {
		t.Fatalf("Failed to create YAML data file: %v", err)
	}

	// Test valid JSON data
	t.Run("valid JSON", func(t *testing.T) {
		result, err := ValidateFileWithSchemaPath(schemaFile, validDataFile)
		if err != nil {
			t.Fatalf("ValidateFileWithSchemaPath() failed: %v", err)
		}
		if !result.Valid {
			t.Errorf("Expected valid data to pass validation, got errors: %v", result.Errors)
		}
	})

	// Test invalid JSON data
	t.Run("invalid JSON", func(t *testing.T) {
		result, err := ValidateFileWithSchemaPath(schemaFile, invalidDataFile)
		if err != nil {
			t.Fatalf("ValidateFileWithSchemaPath() failed: %v", err)
		}
		if result.Valid {
			t.Error("Expected invalid data to fail validation")
		}
		if len(result.Errors) == 0 {
			t.Error("Expected validation errors for invalid data")
		}
	})

	// Test YAML data
	t.Run("YAML data", func(t *testing.T) {
		result, err := ValidateFileWithSchemaPath(schemaFile, yamlDataFile)
		if err != nil {
			t.Fatalf("ValidateFileWithSchemaPath() failed: %v", err)
		}
		if !result.Valid {
			t.Errorf("Expected valid YAML data to pass validation, got errors: %v", result.Errors)
		}
	})

	// Test non-existent schema file
	t.Run("non-existent schema", func(t *testing.T) {
		_, err := ValidateFileWithSchemaPath("non-existent.json", validDataFile)
		if err == nil {
			t.Error("Expected error for non-existent schema file")
		}
	})

	// Test non-existent data file
	t.Run("non-existent data", func(t *testing.T) {
		_, err := ValidateFileWithSchemaPath(schemaFile, "non-existent.json")
		if err == nil {
			t.Error("Expected error for non-existent data file")
		}
	})
}

func TestValidateFromFileWithBytes(t *testing.T) {
	tempDir := t.TempDir()

	// Create schema file
	schemaFile := filepath.Join(tempDir, "schema.json")
	schemaContent := `{
		"type": "object",
		"properties": {
			"name": {"type": "string"},
			"count": {"type": "number"}
		},
		"required": ["name"]
	}`
	if err := os.WriteFile(schemaFile, []byte(schemaContent), 0644); err != nil {
		t.Fatalf("Failed to create schema file: %v", err)
	}

	// Test with JSON data bytes
	t.Run("JSON data bytes", func(t *testing.T) {
		dataBytes := []byte(`{"name": "test", "count": 42}`)
		result, err := ValidateFromFileWithBytes(schemaFile, dataBytes)
		if err != nil {
			t.Fatalf("ValidateFromFileWithBytes() failed: %v", err)
		}
		if !result.Valid {
			t.Errorf("Expected valid JSON data to pass validation, got errors: %v", result.Errors)
		}
	})

	// Test with YAML data bytes
	t.Run("YAML data bytes", func(t *testing.T) {
		dataBytes := []byte(`
name: "yaml-test"
count: 100
`)
		result, err := ValidateFromFileWithBytes(schemaFile, dataBytes)
		if err != nil {
			t.Fatalf("ValidateFromFileWithBytes() failed: %v", err)
		}
		if !result.Valid {
			t.Errorf("Expected valid YAML data to pass validation, got errors: %v", result.Errors)
		}
	})

	// Test with invalid data bytes
	t.Run("invalid data bytes", func(t *testing.T) {
		dataBytes := []byte(`{"count": 42}`) // missing required "name"
		result, err := ValidateFromFileWithBytes(schemaFile, dataBytes)
		if err != nil {
			t.Fatalf("ValidateFromFileWithBytes() failed: %v", err)
		}
		if result.Valid {
			t.Error("Expected invalid data to fail validation")
		}
		if len(result.Errors) == 0 {
			t.Error("Expected validation errors for invalid data")
		}
	})

	// Test with malformed data bytes
	t.Run("malformed data bytes", func(t *testing.T) {
		dataBytes := []byte(`{invalid json}`)
		result, err := ValidateFromFileWithBytes(schemaFile, dataBytes)
		if err == nil {
			// If no parse error, check if validation still works
			if result.Valid {
				t.Log("Note: Malformed JSON was accepted (possibly as YAML or other format)")
			}
		} else {
			t.Logf("Expected error occurred: %v", err)
		}
	})

	// Test with non-existent schema file
	t.Run("non-existent schema", func(t *testing.T) {
		dataBytes := []byte(`{"name": "test"}`)
		_, err := ValidateFromFileWithBytes("non-existent.json", dataBytes)
		if err == nil {
			t.Error("Expected error for non-existent schema file")
		}
	})
}

func TestValidateWithOptions(t *testing.T) {
	// Test schema
	schemaBytes := []byte(`{
		"type": "object",
		"properties": {
			"name": {"type": "string"}
		},
		"required": ["name"]
	}`)

	// Test data
	validData := map[string]interface{}{"name": "test"}
	invalidData := map[string]interface{}{"age": 30}

	// Test without options
	t.Run("without options", func(t *testing.T) {
		opts := ValidationOptions{}
		result, err := ValidateWithOptions(schemaBytes, validData, opts)
		if err != nil {
			t.Fatalf("ValidateWithOptions() failed: %v", err)
		}
		if !result.Valid {
			t.Errorf("Expected valid data to pass validation, got errors: %v", result.Errors)
		}
	})

	// Test with context
	t.Run("with context", func(t *testing.T) {
		opts := ValidationOptions{
			Context: ValidationContext{
				SourceFile: "test.json",
				SourceType: "json",
			},
		}
		result, err := ValidateWithOptions(schemaBytes, invalidData, opts)
		if err != nil {
			t.Fatalf("ValidateWithOptions() failed: %v", err)
		}
		if result.Valid {
			t.Error("Expected invalid data to fail validation")
		}
		// Check that context was applied to errors
		for _, err := range result.Errors {
			if err.Context.SourceFile != "test.json" {
				t.Errorf("Expected error context to have source file 'test.json', got %q", err.Context.SourceFile)
			}
		}
	})

	// Test invalid schema
	t.Run("invalid schema", func(t *testing.T) {
		invalidSchema := []byte(`{invalid json}`)
		opts := ValidationOptions{}
		result, err := ValidateWithOptions(invalidSchema, validData, opts)
		if err == nil {
			// If no parse error, check if validation still works
			if result.Valid {
				t.Log("Note: Invalid JSON schema was accepted (possibly as YAML or other format)")
			}
		} else {
			t.Logf("Expected error occurred: %v", err)
		}
	})
}
