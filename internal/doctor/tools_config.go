package doctor

import (
	"fmt"
	"os"
	"path/filepath"

	pkgtools "github.com/fulmenhq/goneat/pkg/tools"
)

// Alias exported structures from pkg/tools for backwards compatibility within the doctor package.
type ToolsConfig = pkgtools.Config
type ScopeConfig = pkgtools.Scope
type ToolConfig = pkgtools.Tool

// LoadToolsConfig loads the tools configuration from .goneat/tools.yaml ONLY.
// CHANGED in v0.3.7: No longer merges with defaults or ensures core scopes.
// The .goneat/tools.yaml file is now the explicit SSOT with no hidden runtime behavior.
//
// Searches up the directory tree to find .goneat/tools.yaml in the repository root.
// If not found, returns an error with helpful guidance.
// Use `goneat doctor tools init` to create the file with language-specific defaults.
func LoadToolsConfig() (*ToolsConfig, error) {
	// Search up the directory tree for .goneat/tools.yaml
	configPath, err := findToolsConfig()
	if err != nil {
		return nil, fmt.Errorf(`.goneat/tools.yaml not found

This file defines which tools goneat should manage for this repository.

Initialize with:
  goneat doctor tools init           # Recommended defaults for your repo type
  goneat doctor tools init --minimal # CI-safe (only language-native tools)

For more info: goneat doctor tools init --help`)
	}

	// Read and validate user config
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", configPath, err)
	}

	if err := pkgtools.ValidateBytes(data); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	config, err := pkgtools.ParseConfig(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return config, nil
}

// findToolsConfig searches up the directory tree for .goneat/tools.yaml
func findToolsConfig() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		configPath := filepath.Join(dir, ".goneat", "tools.yaml")
		if _, err := os.Stat(configPath); err == nil {
			return configPath, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			return "", fmt.Errorf(".goneat/tools.yaml not found in current directory or any parent")
		}
		dir = parent
	}
}

// REMOVED in v0.3.7: ensureCoreScopes() and loadDefaultConfig() deleted
// These functions forced hardcoded tools into runtime configuration, causing the CI blocker.
//
// Replacement: .goneat/tools.yaml is the explicit SSOT. Use `goneat doctor tools init`
// to seed it with language-specific defaults. No runtime merging occurs.

// ParseConfig parses YAML configuration into ToolsConfig (public function for backwards compatibility).
func ParseConfig(configBytes []byte) (*ToolsConfig, error) {
	return pkgtools.ParseConfig(configBytes)
}

// ValidateConfig validates a configuration file.
func ValidateConfig(configPath string) error {
	return pkgtools.ValidateFile(configPath)
}

// CreateDefaultConfig creates a default configuration file on disk.
func CreateDefaultConfig(configPath string) error {
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	cfgBytes := GetDefaultToolsConfig()
	if err := os.WriteFile(configPath, cfgBytes, 0o600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
