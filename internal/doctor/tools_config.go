package doctor

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ToolsConfig represents the complete tools configuration
type ToolsConfig struct {
	Scopes map[string]ScopeConfig `yaml:"scopes" json:"scopes"`
	Tools  map[string]ToolConfig  `yaml:"tools" json:"tools"`
}

// ScopeConfig represents a tool scope definition
type ScopeConfig struct {
	Description string   `yaml:"description" json:"description"`
	Tools       []string `yaml:"tools" json:"tools"`
}

// ToolConfig represents a single tool definition
type ToolConfig struct {
	Name            string            `yaml:"name" json:"name"`
	Description     string            `yaml:"description" json:"description"`
	Kind            string            `yaml:"kind" json:"kind"`
	DetectCommand   string            `yaml:"detect_command" json:"detect_command"`
	InstallPackage  string            `yaml:"install_package,omitempty" json:"install_package,omitempty"`
	VersionArgs     []string          `yaml:"version_args,omitempty" json:"version_args,omitempty"`
	CheckArgs       []string          `yaml:"check_args,omitempty" json:"check_args,omitempty"`
	Platforms       []string          `yaml:"platforms,omitempty" json:"platforms,omitempty"`
	InstallCommands map[string]string `yaml:"install_commands,omitempty" json:"install_commands,omitempty"`
}

// LoadToolsConfig loads the tools configuration with schema validation
func LoadToolsConfig() (*ToolsConfig, error) {
	// 1. Load embedded defaults (already validated during build)
	config, err := loadDefaultConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load default config: %w", err)
	}

	// 2. Check for user config
	userConfigPath := ".goneat/tools.yaml"
	if _, err := os.Stat(userConfigPath); err == nil {
		// Read user config bytes
		configBytes, err := os.ReadFile(userConfigPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read user config: %w", err)
		}

		// 3. CRITICAL: Validate against schema BEFORE parsing
		if err := validateToolsConfig(configBytes); err != nil {
			return nil, fmt.Errorf("user config validation failed: %w", err)
		}

		// 4. Parse validated config
		userConfig, err := parseConfig(configBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse user config: %w", err)
		}

		// 5. Merge user config with defaults
		mergeConfigs(config, userConfig)
	}

	return config, nil
}

// loadDefaultConfig loads the embedded default configuration
func loadDefaultConfig() (*ToolsConfig, error) {
	// Load from embedded defaults
	configBytes := GetDefaultToolsConfig()

	// Parse the embedded config
	config, err := parseConfig(configBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse embedded config: %w", err)
	}

	return config, nil
}

// parseConfig parses YAML configuration into ToolsConfig
func parseConfig(configBytes []byte) (*ToolsConfig, error) {
	var config ToolsConfig
	if err := yaml.Unmarshal(configBytes, &config); err != nil {
		return nil, fmt.Errorf("yaml parse error: %w", err)
	}
	return &config, nil
}

// validateToolsConfig validates configuration against JSON Schema
func validateToolsConfig(configBytes []byte) error {
	return ValidateToolsConfig(configBytes)
}

// mergeConfigs merges user config into default config
func mergeConfigs(defaultConfig, userConfig *ToolsConfig) {
	// Merge scopes
	if userConfig.Scopes != nil {
		if defaultConfig.Scopes == nil {
			defaultConfig.Scopes = make(map[string]ScopeConfig)
		}
		for name, scope := range userConfig.Scopes {
			defaultConfig.Scopes[name] = scope
		}
	}

	// Merge tools
	if userConfig.Tools != nil {
		if defaultConfig.Tools == nil {
			defaultConfig.Tools = make(map[string]ToolConfig)
		}
		for name, tool := range userConfig.Tools {
			defaultConfig.Tools[name] = tool
		}
	}
}

// GetToolsForScope returns tools for a specific scope
func (c *ToolsConfig) GetToolsForScope(scopeName string) ([]ToolConfig, error) {
	scope, exists := c.Scopes[scopeName]
	if !exists {
		return nil, fmt.Errorf("scope '%s' not found", scopeName)
	}

	var tools []ToolConfig
	for _, toolName := range scope.Tools {
		tool, exists := c.Tools[toolName]
		if !exists {
			return nil, fmt.Errorf("tool '%s' referenced in scope '%s' but not defined", toolName, scopeName)
		}
		tools = append(tools, tool)
	}

	return tools, nil
}

// GetAllScopes returns all available scope names
func (c *ToolsConfig) GetAllScopes() []string {
	var scopes []string
	for name := range c.Scopes {
		scopes = append(scopes, name)
	}
	return scopes
}

// GetTool returns a specific tool by name
func (c *ToolsConfig) GetTool(toolName string) (ToolConfig, bool) {
	tool, exists := c.Tools[toolName]
	return tool, exists
}

// ValidateConfig validates a configuration file
func ValidateConfig(configPath string) error {
	return ValidateConfigFile(configPath)
}

// CreateDefaultConfig creates a default configuration file
func CreateDefaultConfig(configPath string) error {
	// Ensure directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Load default config
	config, err := loadDefaultConfig()
	if err != nil {
		return fmt.Errorf("failed to load default config: %w", err)
	}

	// Marshal to YAML
	configBytes, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(configPath, configBytes, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// ParseConfig parses YAML configuration into ToolsConfig (public function)
func ParseConfig(configBytes []byte) (*ToolsConfig, error) {
	return parseConfig(configBytes)
}
