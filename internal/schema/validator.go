package schema

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/fulmenhq/goneat/internal/assets"
	"github.com/fulmenhq/goneat/internal/schemaengine"
	"github.com/fulmenhq/goneat/internal/schemarun"
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

// registrySchemaPaths maps schema names to embedded paths. Paths must be under
// embedded_schemas/schemas/, the tree make embed-assets syncs from schemas/
// (see TestRegistryUsesSyncedSchemas).
var registrySchemaPaths = map[string]string{
	"goneat-config-v1.0.0":       "embedded_schemas/schemas/config/v1.0.0/goneat-config.yaml",
	"dates":                      "embedded_schemas/schemas/config/v1.0.0/dates.yaml",
	"tools-config-v1.0.0":        "embedded_schemas/schemas/tools/v1.0.0/tools-config.yaml",
	"tools-config-v1.1.0":        "embedded_schemas/schemas/tools/v1.1.0/tools-config.yaml",
	"assess-config-v1.0.0":       "embedded_schemas/schemas/config/v1.0.0/assess-config.yaml",
	"dependencies-policy-v1.0.0": "embedded_schemas/schemas/config/v1.0.0/dependencies-policy.yaml",
	// Add more as needed
}

type registryEntry struct {
	once     sync.Once
	compiled *schemaengine.Compiled
	err      error
}

// registry compiles each known schema on first use and keeps the result,
// including a compile failure, so a broken schema is reported rather than
// appearing to be missing.
var registry = func() map[string]*registryEntry {
	m := make(map[string]*registryEntry, len(registrySchemaPaths))
	for name := range registrySchemaPaths {
		m[name] = &registryEntry{}
	}
	return m
}()

func compiledSchema(schemaName string) (*schemaengine.Compiled, error) {
	e, ok := registry[schemaName]
	if !ok {
		return nil, fmt.Errorf("schema %s not found in registry", schemaName)
	}
	e.once.Do(func() {
		path := registrySchemaPaths[schemaName]
		data, ok := assets.GetSchema(path)
		if !ok || len(data) == 0 {
			e.err = fmt.Errorf("schema %s: embedded schema %s is missing", schemaName, path)
			return
		}
		e.compiled, e.err = schemarun.Compile(data, nil)
		if e.err != nil {
			e.err = fmt.Errorf("schema %s: %w", schemaName, e.err)
		}
	})
	return e.compiled, e.err
}

// Validate validates data (interface{}) against the named schema.
func Validate(data interface{}, schemaName string) (*Result, error) {
	c, err := compiledSchema(schemaName)
	if err != nil {
		return nil, err
	}
	valid, issues, err := schemarun.Validate(c, data)
	if err != nil {
		return nil, err
	}
	res := &Result{Valid: valid}
	for _, is := range issues {
		res.Errors = append(res.Errors, ValidationError{
			Path:    is.Path,
			Message: schemarun.ImproveMessage(is, schemaName),
		})
	}
	return res, nil
}

func ValidateConfigFile(path string, schemaName string) error {
	data, err := os.ReadFile(path) // #nosec G304 -- path is user-specified config file, intentional for CLI tool
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
