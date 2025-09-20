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

// LoadToolsConfig loads the tools configuration with schema validation and user overrides.
func LoadToolsConfig() (*ToolsConfig, error) {
	config, err := loadDefaultConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load default config: %w", err)
	}

	userConfigPath := ".goneat/tools.yaml"
	if _, err := os.Stat(userConfigPath); err == nil {
		data, err := os.ReadFile(userConfigPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read user config: %w", err)
		}
		if err := pkgtools.ValidateBytes(data); err != nil {
			return nil, fmt.Errorf("user config validation failed: %w", err)
		}
		userConfig, err := pkgtools.ParseConfig(data)
		if err != nil {
			return nil, fmt.Errorf("failed to parse user config: %w", err)
		}
		config.Merge(userConfig)

		// Ensure core scopes are always available (fallback guarantee)
		ensureCoreScopes(config)
	}

	return config, nil
}

// ensureCoreScopes guarantees that essential scopes (foundation, format, security, all)
// are always available, even if user config tries to replace or remove them.
func ensureCoreScopes(config *ToolsConfig) {
	coreScopes := map[string]ScopeConfig{
		"foundation": {
			Description: "Core foundation tools required for goneat and basic AI agent operation",
			Tools:       []string{"ripgrep", "jq", "go-licenses", "golangci-lint", "yamlfmt", "yq"},
		},
		"format": {
			Description: "Code formatting tools",
			Tools:       []string{"goimports", "gofmt"},
		},
		"security": {
			Description: "Security scanning tools",
			Tools:       []string{"gosec", "govulncheck", "gitleaks"},
		},
		"all": {
			Description: "All tools from all scopes",
			Tools:       []string{"gosec", "govulncheck", "gitleaks", "goimports", "gofmt", "ripgrep", "jq", "go-licenses", "golangci-lint", "yamlfmt", "yq"},
		},
	}

	if config.Scopes == nil {
		config.Scopes = make(map[string]ScopeConfig)
	}

	for name, coreScope := range coreScopes {
		if _, exists := config.Scopes[name]; !exists {
			config.Scopes[name] = coreScope
		}
		// Note: We don't override existing scopes to preserve user customizations,
		// but we ensure they exist if missing
	}
}

func loadDefaultConfig() (*ToolsConfig, error) {
	configBytes := GetDefaultToolsConfig()
	if len(configBytes) == 0 {
		return nil, fmt.Errorf("embedded default config is empty")
	}

	cfg, err := pkgtools.ParseConfig(configBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse embedded config: %w", err)
	}
	return cfg, nil
}

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
