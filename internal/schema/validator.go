package schema

import (
	"encoding/json"
	"fmt"

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

// init populates the registry with known schemas.
func init() {
	known := map[string]string{
		"goneat-config-v1.0.0": "embedded_schemas/config/goneat-config-v1.0.0.yaml",
		"dates":                "embedded_schemas/schemas/config/dates.yaml",
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

			jsonBytes, err := json.Marshal(schemaData)
			if err != nil {
				// Skip on error
				continue
			}

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
			res.Errors = append(res.Errors, ValidationError{
				Path:    field,
				Message: verr.Description(),
			})
		}
	}

	return res, nil
}
