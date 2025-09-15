package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fulmenhq/goneat/internal/schema"
	"gopkg.in/yaml.v3"
)

// ValidateToolsConfig validates a tools configuration against the JSON Schema
func ValidateToolsConfig(configBytes []byte) error {
	// Parse YAML to interface{} for validation
	var configData interface{}
	if err := yaml.Unmarshal(configBytes, &configData); err != nil {
		return fmt.Errorf("yaml parse error: %w", err)
	}

	// Validate against schema
	result, err := schema.Validate(configData, "tools-config-v1.0.0")
	if err != nil {
		return fmt.Errorf("schema validation error: %w", err)
	}
	if !result.Valid {
		// Provide detailed error feedback
		var errMsgs []string
		for _, e := range result.Errors {
			errMsgs = append(errMsgs, fmt.Sprintf("%s: %s", e.Path, e.Message))
		}
		return fmt.Errorf("invalid config:\n%s", strings.Join(errMsgs, "\n"))
	}
	return nil
}

// ValidateConfigFile validates a configuration file
func ValidateConfigFile(configPath string) error {
	// Validate path to prevent directory traversal
	if configPath == "" {
		return fmt.Errorf("config path cannot be empty")
	}
	cleanPath := filepath.Clean(configPath)
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("config path contains invalid path traversal")
	}

	configBytes, err := os.ReadFile(cleanPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	return ValidateToolsConfig(configBytes)
}
