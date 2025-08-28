package config

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/xeipuuv/gojsonschema"
)

// Schema content loaded at runtime for easier development and updates

// SchemaVersion represents a configuration schema version
type SchemaVersion struct {
	Major int
	Minor int
	Patch int
}

// String returns the string representation of the version
func (v SchemaVersion) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// ParseSchemaVersion parses a version string into SchemaVersion
func ParseSchemaVersion(version string) (SchemaVersion, error) {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return SchemaVersion{}, fmt.Errorf("invalid version format: %s", version)
	}

	var v SchemaVersion
	_, err := fmt.Sscanf(version, "%d.%d.%d", &v.Major, &v.Minor, &v.Patch)
	if err != nil {
		return SchemaVersion{}, fmt.Errorf("failed to parse version: %v", err)
	}

	return v, nil
}

// ValidateConfig validates configuration against the appropriate schema
func ValidateConfig(configData []byte, schemaVersion string) error {
	schemaLoader, err := getSchemaLoader(schemaVersion)
	if err != nil {
		return fmt.Errorf("failed to load schema for version %s: %v", schemaVersion, err)
	}

	documentLoader := gojsonschema.NewBytesLoader(configData)

	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return fmt.Errorf("schema validation error: %v", err)
	}

	if !result.Valid() {
		var errors []string
		for _, desc := range result.Errors() {
			errors = append(errors, desc.String())
		}
		return fmt.Errorf("configuration validation failed:\n%s", strings.Join(errors, "\n"))
	}

	return nil
}

// getSchemaLoader returns the appropriate schema loader for the given version
func getSchemaLoader(version string) (gojsonschema.JSONLoader, error) {
	switch version {
	case "1.0.0", "v1.0.0":
		schemaPath := "../../schemas/config/goneat-config-v1.0.0.yaml"
		return gojsonschema.NewReferenceLoader(schemaPath), nil
	default:
		return nil, fmt.Errorf("unsupported schema version: %s", version)
	}
}

// DetectSchemaVersion detects the schema version from config data
func DetectSchemaVersion(configData []byte) (string, error) {
	var config map[string]interface{}
	if err := json.Unmarshal(configData, &config); err != nil {
		return "", fmt.Errorf("failed to parse config as JSON: %v", err)
	}

	// Check for $schema field
	if schema, ok := config["$schema"]; ok {
		schemaStr, ok := schema.(string)
		if !ok {
			return "", fmt.Errorf("$schema must be a string")
		}

		// Extract version from schema URL
		if strings.Contains(schemaStr, "/v1.0.0") {
			return "1.0.0", nil
		}
	}

	// Default to latest version if no schema specified
	return "1.0.0", nil
}

// MigrateConfig migrates configuration from older schema versions to newer ones
func MigrateConfig(configData []byte, fromVersion, toVersion string) ([]byte, error) {
	fromVer, err := ParseSchemaVersion(fromVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid from version: %v", err)
	}

	toVer, err := ParseSchemaVersion(toVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid to version: %v", err)
	}

	if fromVer.Major == toVer.Major && fromVer.Minor == toVer.Minor && fromVer.Patch == toVer.Patch {
		// No migration needed
		return configData, nil
	}

	// For now, we only support v1.0.0, so no migrations are needed
	// In the future, we would implement migration logic here
	return nil, fmt.Errorf("migration from %s to %s not supported", fromVersion, toVersion)
}

// GetAvailableSchemas returns a list of available schema versions
func GetAvailableSchemas() ([]string, error) {
	schemasDir := "schemas/config"
	files, err := filepath.Glob(filepath.Join(schemasDir, "goneat-config-v*.yaml"))
	if err != nil {
		return nil, fmt.Errorf("failed to list schema files: %v", err)
	}

	var versions []string
	for _, file := range files {
		base := filepath.Base(file)
		// Extract version from filename: goneat-config-v1.0.0.yaml -> 1.0.0
		if strings.HasPrefix(base, "goneat-config-v") && strings.HasSuffix(base, ".yaml") {
			version := strings.TrimPrefix(base, "goneat-config-v")
			version = strings.TrimSuffix(version, ".yaml")
			versions = append(versions, version)
		}
	}

	return versions, nil
}
