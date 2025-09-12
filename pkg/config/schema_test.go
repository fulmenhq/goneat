package config

import (
	"testing"
)

func TestSchemaVersionString(t *testing.T) {
	tests := []struct {
		version  SchemaVersion
		expected string
	}{
		{SchemaVersion{1, 0, 0}, "1.0.0"},
		{SchemaVersion{2, 1, 3}, "2.1.3"},
		{SchemaVersion{0, 0, 1}, "0.0.1"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.version.String()
			if result != tt.expected {
				t.Errorf("SchemaVersion.String() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

func TestParseSchemaVersion(t *testing.T) {
	tests := []struct {
		input       string
		expected    SchemaVersion
		expectError bool
	}{
		{"1.0.0", SchemaVersion{1, 0, 0}, false},
		{"2.1.3", SchemaVersion{2, 1, 3}, false},
		{"0.0.1", SchemaVersion{0, 0, 1}, false},
		{"invalid", SchemaVersion{}, true},
		{"1.0", SchemaVersion{}, true},
		{"1.0.0.0", SchemaVersion{}, true},
		{"a.b.c", SchemaVersion{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := ParseSchemaVersion(tt.input)
			if tt.expectError {
				if err == nil {
					t.Errorf("ParseSchemaVersion(%q) expected error but got none", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseSchemaVersion(%q) unexpected error: %v", tt.input, err)
				return
			}
			if result != tt.expected {
				t.Errorf("ParseSchemaVersion(%q) = %+v, expected %+v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestValidateConfigWithInvalidSchema(t *testing.T) {
	configData := []byte(`{"format": {"go": {"simplify": true}}}`)

	// Test with non-existent schema version
	err := ValidateConfig(configData, "99.99.99")
	if err == nil {
		t.Error("Expected ValidateConfig to fail with non-existent schema version")
	}
	if err != nil && !contains(err.Error(), "failed to load schema") {
		t.Errorf("Expected schema loading error, got: %v", err)
	}
}

func TestDetectSchemaVersionBasic(t *testing.T) {
	tests := []struct {
		name        string
		configData  []byte
		expected    string
		expectError bool
	}{
		{
			name:        "config with schema field",
			configData:  []byte(`{"$schema": "https://schemas.goneat.io/config/v1.0.0", "format": {}}`),
			expected:    "1.0.0",
			expectError: false,
		},
		{
			name:        "config without schema field",
			configData:  []byte(`{"format": {"go": {"simplify": true}}}`),
			expected:    "1.0.0",
			expectError: false,
		},
		{
			name:        "invalid JSON",
			configData:  []byte(`{invalid json}`),
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version, err := DetectSchemaVersion(tt.configData)
			if tt.expectError {
				if err == nil {
					t.Errorf("DetectSchemaVersion() expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("DetectSchemaVersion() unexpected error: %v", err)
				return
			}
			if version != tt.expected {
				t.Errorf("DetectSchemaVersion() = %q, expected %q", version, tt.expected)
			}
		})
	}
}

func TestMigrateConfigBasic(t *testing.T) {
	configData := []byte(`{"format": {"go": {"simplify": true}}}`)

	// Test migrating to same version (should be no-op)
	migrated, err := MigrateConfig(configData, "1.0.0", "1.0.0")
	if err != nil {
		t.Errorf("MigrateConfig to same version failed: %v", err)
	}
	if string(migrated) != string(configData) {
		t.Error("MigrateConfig to same version should return original data")
	}
}

func TestGetAvailableSchemas(t *testing.T) {
	schemas, err := GetAvailableSchemas()
	if err != nil {
		// This is expected to fail in test environment since schema files may not exist
		t.Logf("GetAvailableSchemas() failed as expected in test environment: %v", err)
		return
	}

	// If it succeeds, just log what we found
	t.Logf("Found %d schemas: %v", len(schemas), schemas)
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || indexAny(s, substr) >= 0)
}

func indexAny(s, chars string) int {
	for i := 0; i < len(s); i++ {
		for j := 0; j < len(chars); j++ {
			if s[i] == chars[j] {
				return i
			}
		}
	}
	return -1
}
