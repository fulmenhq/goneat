/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package dependencies_test

import (
	"os"
	"testing"

	"github.com/fulmenhq/goneat/pkg/schema" // ✅ Dogfooding goneat's schema package
)

func TestDependencyAnalysisSchema(t *testing.T) {
	// Enable offline mode to prevent URL fetching
	if err := os.Setenv("GONEAT_OFFLINE_SCHEMA_VALIDATION", "true"); err != nil {
		t.Fatalf("Failed to set environment variable: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("GONEAT_OFFLINE_SCHEMA_VALIDATION"); err != nil {
			t.Logf("Warning: failed to unset environment variable: %v", err)
		}
	}()

	// Get embedded validator for our schema using direct path approach
	validator, err := schema.NewValidatorFromEmbeddedPath("embedded_schemas/schemas/dependencies/v1.0.0/dependency-analysis.schema.json")
	if err != nil {
		t.Fatalf("Failed to load schema: %v", err)
	}

	tests := []struct {
		name    string
		data    interface{}
		wantErr bool
	}{
		{
			name: "valid_analysis_result",
			data: map[string]interface{}{
				"version": "v1",
				"metadata": map[string]interface{}{
					"generated_at": "2025-10-16T12:00:00Z",
					"target":       ".",
					"language":     "go",
					"duration_ms":  1000,
				},
				"summary": map[string]interface{}{
					"dependency_count":   10,
					"license_violations": 0,
					"cooling_violations": 0,
					"passed":             true,
				},
				"dependencies": []interface{}{},
				"issues":       []interface{}{},
			},
			wantErr: false,
		},
		{
			name: "missing_required_version",
			data: map[string]interface{}{
				"metadata":     map[string]interface{}{},
				"summary":      map[string]interface{}{},
				"dependencies": []interface{}{},
				"issues":       []interface{}{},
			},
			wantErr: true,
		},
		{
			name: "invalid_severity_enum",
			data: map[string]interface{}{
				"version": "v1",
				"metadata": map[string]interface{}{
					"generated_at": "2025-10-16T12:00:00Z",
					"target":       ".",
					"language":     "go",
					"duration_ms":  1000,
				},
				"summary": map[string]interface{}{
					"dependency_count":   10,
					"license_violations": 1,
					"cooling_violations": 0,
					"passed":             false,
				},
				"dependencies": []interface{}{},
				"issues": []interface{}{
					map[string]interface{}{
						"type":     "license",
						"severity": "invalid-severity", // ❌ Invalid
						"message":  "Test issue",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "sbom_metadata_optional",
			data: map[string]interface{}{
				"version": "v1",
				"metadata": map[string]interface{}{
					"generated_at": "2025-10-16T12:00:00Z",
					"target":       ".",
					"language":     "go",
					"duration_ms":  1000,
				},
				"summary": map[string]interface{}{
					"dependency_count":   10,
					"license_violations": 0,
					"cooling_violations": 0,
					"passed":             true,
				},
				"dependencies": []interface{}{},
				"issues":       []interface{}{},
				"sbom_metadata": map[string]interface{}{
					"path":         "sbom/goneat.cdx.json",
					"format":       "cyclonedx-json",
					"tool_version": "v0.100.0",
					"generated_at": "2025-10-16T12:00:00Z",
				},
			},
			wantErr: false,
		},
		{
			name: "valid_severity_critical",
			data: map[string]interface{}{
				"version": "v1",
				"metadata": map[string]interface{}{
					"generated_at": "2025-10-16T12:00:00Z",
					"target":       ".",
					"language":     "go",
					"duration_ms":  1000,
				},
				"summary": map[string]interface{}{
					"dependency_count":   10,
					"license_violations": 1,
					"cooling_violations": 0,
					"passed":             false,
				},
				"dependencies": []interface{}{},
				"issues": []interface{}{
					map[string]interface{}{
						"type":     "license",
						"severity": "critical", // ✅ Valid Crucible severity
						"message":  "Forbidden license detected",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid_severity_high",
			data: map[string]interface{}{
				"version": "v1",
				"metadata": map[string]interface{}{
					"generated_at": "2025-10-16T12:00:00Z",
					"target":       ".",
					"language":     "go",
					"duration_ms":  1000,
				},
				"summary": map[string]interface{}{
					"dependency_count":   10,
					"license_violations": 0,
					"cooling_violations": 1,
					"passed":             false,
				},
				"dependencies": []interface{}{},
				"issues": []interface{}{
					map[string]interface{}{
						"type":     "cooling",
						"severity": "high", // ✅ Valid Crucible severity
						"message":  "Package too new",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid_severity_medium",
			data: map[string]interface{}{
				"version": "v1",
				"metadata": map[string]interface{}{
					"generated_at": "2025-10-16T12:00:00Z",
					"target":       ".",
					"language":     "go",
					"duration_ms":  1000,
				},
				"summary": map[string]interface{}{
					"dependency_count":   10,
					"license_violations": 0,
					"cooling_violations": 0,
					"passed":             true,
				},
				"dependencies": []interface{}{},
				"issues": []interface{}{
					map[string]interface{}{
						"type":     "license",
						"severity": "medium", // ✅ Valid Crucible severity
						"message":  "License review recommended",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid_severity_low",
			data: map[string]interface{}{
				"version": "v1",
				"metadata": map[string]interface{}{
					"generated_at": "2025-10-16T12:00:00Z",
					"target":       ".",
					"language":     "go",
					"duration_ms":  1000,
				},
				"summary": map[string]interface{}{
					"dependency_count":   10,
					"license_violations": 0,
					"cooling_violations": 0,
					"passed":             true,
				},
				"dependencies": []interface{}{},
				"issues": []interface{}{
					map[string]interface{}{
						"type":     "license",
						"severity": "low", // ✅ Valid Crucible severity
						"message":  "Minor license concern",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid_severity_info",
			data: map[string]interface{}{
				"version": "v1",
				"metadata": map[string]interface{}{
					"generated_at": "2025-10-16T12:00:00Z",
					"target":       ".",
					"language":     "go",
					"duration_ms":  1000,
				},
				"summary": map[string]interface{}{
					"dependency_count":   10,
					"license_violations": 0,
					"cooling_violations": 0,
					"passed":             true,
				},
				"dependencies": []interface{}{},
				"issues": []interface{}{
					map[string]interface{}{
						"type":     "license",
						"severity": "info", // ✅ Valid Crucible severity
						"message":  "License information",
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := validator.Validate(tt.data) // ✅ Using goneat's validator
			if err != nil {
				t.Fatalf("Validation error: %v", err)
			}

			if tt.wantErr && result.Valid {
				t.Error("Expected validation to fail, but it passed")
			}
			if !tt.wantErr && !result.Valid {
				t.Errorf("Expected validation to pass, but got errors: %v", result.Errors)
			}
		})
	}
}
