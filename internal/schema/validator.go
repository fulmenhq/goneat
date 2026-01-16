package schema

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/fulmenhq/goneat/internal/assets"
	"github.com/xeipuuv/gojsonschema"
	"gopkg.in/yaml.v3"
)

// ValidationError represents a single validation error.
type ValidationError struct {
	Path    string `json:"path,omitempty"` // Single string path (e.g., "format.go.simplify")
	Message string `json:"message"`
}

// Result holds the validation result.
type Result struct {
	Valid  bool              `json:"valid"`
	Errors []ValidationError `json:"errors,omitempty"`
}

// registry holds pre-compiled schemas for known schema names (e.g., "goneat-config-v1.0.0").
var registry = make(map[string]*gojsonschema.Schema)

// isOfflineMode checks if offline schema validation is enabled via environment variable
func isOfflineMode() bool {
	return os.Getenv("GONEAT_OFFLINE_SCHEMA_VALIDATION") == "true"
}

// init populates the registry with known schemas.
func init() {
	known := map[string]string{
		"goneat-config-v1.0.0": "embedded_schemas/config/goneat-config-v1.0.0.yaml",
		"dates":                "embedded_schemas/schemas/config/dates.yaml",
		"tools-config-v1.0.0":  "embedded_schemas/schemas/tools/v1.0.0/tools-config.yaml",
		"tools-config-v1.1.0":  "embedded_schemas/schemas/tools/v1.1.0/tools-config.yaml",
		"assess-config-v1.0.0": "embedded_schemas/schemas/config/v1.0.0/assess-config.yaml",
		// Add more as needed
	}
	for name, path := range known {
		if schemaBytes, ok := assets.GetSchema(path); ok && len(schemaBytes) > 0 {
			// Convert YAML to JSON for gojsonschema
			var schemaData interface{}
			if err := yaml.Unmarshal(schemaBytes, &schemaData); err != nil {
				// Skip on error
				continue
			}

			// Conditionally remove $schema field to prevent remote fetching in offline mode
			if isOfflineMode() {
				if m, ok := schemaData.(map[string]interface{}); ok {
					delete(m, "$schema")
				}
			}

			jsonBytes, err := json.Marshal(schemaData)
			if err != nil {
				// Skip on error
				continue
			}

			// Create schema with offline-only loading (no remote references)
			schemaLoader := gojsonschema.NewBytesLoader(jsonBytes)
			schema, err := gojsonschema.NewSchema(schemaLoader)
			if err != nil {
				// Skip on error
				continue
			}
			registry[name] = schema
		}
	}
}

// improveErrorMessage translates cryptic JSON Schema validator messages into more actionable ones.
func improveErrorMessage(path, message, schemaName string) string {
	// Detect mutual exclusivity violations in tools-config schemas
	// Match any path under tools (e.g., "tools.badtool", "tools.goneat")
	if len(path) >= 6 && path[:6] == "tools." {
		if message == "Must not validate the schema (not)" {
			return "Both 'install' and 'install_commands' cannot be present (mutually exclusive). Use only 'install' for v1.1.0+ package managers, or only 'install_commands' for legacy scripts. See schema: " + schemaName
		}
		if message == "Additional property install is not allowed" {
			return "The 'install' property requires schema v1.1.0+. Either upgrade to v1.1.0 schema or use 'install_commands' instead. See schema: " + schemaName
		}
	}

	// Generic improvement for "not" schema failures
	if message == "Must not validate the schema (not)" {
		return message + " (Schema constraint violation - check for mutually exclusive properties or invalid combinations)"
	}

	// Return original message if no improvement available
	return message
}

// Validate validates data (interface{}) against the named schema.
func Validate(data interface{}, schemaName string) (*Result, error) {
	schema, ok := registry[schemaName]
	if !ok {
		return nil, fmt.Errorf("schema %s not found in registry", schemaName)
	}

	docLoader := gojsonschema.NewGoLoader(data)
	result, err := schema.Validate(docLoader)
	if err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	res := &Result{Valid: result.Valid()}
	if !result.Valid() {
		for _, verr := range result.Errors() {
			field := verr.Field()
			if field == "" {
				field = "root"
			}
			originalMsg := verr.Description()
			improvedMsg := improveErrorMessage(field, originalMsg, schemaName)
			res.Errors = append(res.Errors, ValidationError{
				Path:    field,
				Message: improvedMsg,
			})
		}
	}

	return res, nil
}

func ValidateConfigFile(path string, schemaName string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", path, err)
	}

	var decoded interface{}
	if err := yaml.Unmarshal(data, &decoded); err != nil {
		return fmt.Errorf("failed to parse %s: %w", path, err)
	}

	result, err := Validate(decoded, schemaName)
	if err != nil {
		return err
	}
	if !result.Valid {
		var errLines []string
		for _, v := range result.Errors {
			errLines = append(errLines, fmt.Sprintf("%s: %s", v.Path, v.Message))
		}
		return fmt.Errorf("schema validation failed:\n%s", strings.Join(errLines, "\n"))
	}

	return nil
}
