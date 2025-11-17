package doctor

import (
	"fmt"
	"io/fs"

	"github.com/fulmenhq/goneat/internal/assets"
	"gopkg.in/yaml.v3"
)

// ToolsDefaultsConfig represents the foundation-tools-defaults.yaml structure
type ToolsDefaultsConfig struct {
	Version         string                     `yaml:"version"`
	FoundationTools []ToolDefinition           `yaml:"foundation_tools"`
	SecurityTools   []ToolDefinition           `yaml:"security_tools"`
	FormatTools     []ToolDefinition           `yaml:"format_tools"`
	PythonTools     []ToolDefinition           `yaml:"python_tools"`
	TypeScriptTools []ToolDefinition           `yaml:"typescript_tools"`
	Scopes          map[string]ScopeDefinition `yaml:"scopes"`
}

// ToolDefinition represents a tool definition from the defaults config
type ToolDefinition struct {
	Name                 string      `yaml:"name"`
	Description          string      `yaml:"description"`
	Kind                 string      `yaml:"kind"`
	DetectCommand        string      `yaml:"detect_command"`
	Platforms            []string    `yaml:"platforms,omitempty"`
	PackageManagers      interface{} `yaml:"package_managers"` // can be map[string][]string or simple
	AutoInstallSafe      bool        `yaml:"auto_install_safe"`
	RequiredForLanguages []string    `yaml:"required_for_languages,omitempty"`
	InstallPackage       string      `yaml:"install_package,omitempty"`
	VersionArgs          []string    `yaml:"version_args,omitempty"`
	CheckArgs            []string    `yaml:"check_args,omitempty"`
	VersionScheme        string      `yaml:"version_scheme,omitempty"`
	MinimumVersion       string      `yaml:"minimum_version,omitempty"`
	RecommendedVersion   string      `yaml:"recommended_version,omitempty"`
}

// ScopeDefinition represents a scope definition from the defaults config
type ScopeDefinition struct {
	Description string   `yaml:"description"`
	Tools       []string `yaml:"tools"`
}

// LoadToolsDefaultsConfig loads foundation-tools-defaults.yaml from embedded assets
func LoadToolsDefaultsConfig() (*ToolsDefaultsConfig, error) {
	configFS := assets.GetConfigFS()
	data, err := fs.ReadFile(configFS, "config/tools/foundation-tools-defaults.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to read tools defaults config: %w", err)
	}

	var config ToolsDefaultsConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse tools defaults config: %w", err)
	}

	return &config, nil
}

// GetAllTools returns all tool definitions from all categories
func (c *ToolsDefaultsConfig) GetAllTools() []ToolDefinition {
	allTools := make([]ToolDefinition, 0)
	allTools = append(allTools, c.FoundationTools...)
	allTools = append(allTools, c.SecurityTools...)
	allTools = append(allTools, c.FormatTools...)
	allTools = append(allTools, c.PythonTools...)
	allTools = append(allTools, c.TypeScriptTools...)
	return allTools
}

// GetToolsForScope returns tools for a specific scope
func (c *ToolsDefaultsConfig) GetToolsForScope(scopeName string) ([]ToolDefinition, error) {
	scope, exists := c.Scopes[scopeName]
	if !exists {
		return nil, fmt.Errorf("scope %s not found", scopeName)
	}

	// Build a map of all tools by name
	allTools := c.GetAllTools()
	toolsByName := make(map[string]ToolDefinition)
	for _, tool := range allTools {
		toolsByName[tool.Name] = tool
	}

	// Get tools in scope order
	result := make([]ToolDefinition, 0, len(scope.Tools))
	for _, toolName := range scope.Tools {
		if tool, exists := toolsByName[toolName]; exists {
			result = append(result, tool)
		}
	}

	return result, nil
}

// FilterToolsByLanguage filters tools to only those required for or compatible with the given language
func FilterToolsByLanguage(tools []ToolDefinition, language string) []ToolDefinition {
	if language == "" || language == "unknown" {
		// For unknown languages, include only tools with no language requirements
		result := make([]ToolDefinition, 0)
		for _, tool := range tools {
			if len(tool.RequiredForLanguages) == 0 {
				result = append(result, tool)
			}
		}
		return result
	}

	result := make([]ToolDefinition, 0)
	for _, tool := range tools {
		// Include tools with no language requirements (universal tools)
		if len(tool.RequiredForLanguages) == 0 {
			result = append(result, tool)
			continue
		}

		// Include tools explicitly required for this language
		for _, lang := range tool.RequiredForLanguages {
			if lang == language {
				result = append(result, tool)
				break
			}
		}
	}

	return result
}

// GetMinimalToolsForLanguage returns only language-native package managers and essential tools
func GetMinimalToolsForLanguage(tools []ToolDefinition, language string) []ToolDefinition {
	result := make([]ToolDefinition, 0)

	for _, tool := range tools {
		// Include only tools explicitly required for this language
		for _, lang := range tool.RequiredForLanguages {
			if lang == language {
				// For minimal mode, only include language-native package managers
				// and core build tools (go, python toolchain, etc.)
				if tool.Kind == "go" && language == "go" {
					result = append(result, tool)
					break
				}
				if tool.Kind == "python" && language == "python" {
					result = append(result, tool)
					break
				}
				if tool.Kind == "node" && language == "typescript" {
					result = append(result, tool)
					break
				}
				if tool.Kind == "system" {
					// Include system tools like go toolchain, ripgrep, jq if required for language
					result = append(result, tool)
					break
				}
			}
		}
	}

	return result
}

// ConvertToToolsConfig converts ToolDefinition to the ToolsConfig format used by .goneat/tools.yaml
func ConvertToToolsConfig(tools []ToolDefinition, scopeName string, scopeDescription string) *ToolsConfig {
	config := &ToolsConfig{
		Tools:  make(map[string]ToolConfig),
		Scopes: make(map[string]ScopeConfig),
	}

	toolNames := make([]string, 0, len(tools))
	for _, toolDef := range tools {
		tool := ToolConfig{
			Name:               toolDef.Name,
			Description:        toolDef.Description,
			Kind:               toolDef.Kind,
			DetectCommand:      toolDef.DetectCommand,
			Platforms:          toolDef.Platforms,
			InstallPackage:     toolDef.InstallPackage,
			VersionArgs:        toolDef.VersionArgs,
			CheckArgs:          toolDef.CheckArgs,
			VersionScheme:      toolDef.VersionScheme,
			MinimumVersion:     toolDef.MinimumVersion,
			RecommendedVersion: toolDef.RecommendedVersion,
		}

		// Handle package_managers field - convert to installer_priority
		// foundation-tools-defaults.yaml uses package_managers, but .goneat/tools.yaml uses installer_priority
		if toolDef.PackageManagers != nil {
			switch pm := toolDef.PackageManagers.(type) {
			case map[string]interface{}:
				tool.InstallerPriority = make(map[string][]string)
				for platform, value := range pm {
					if slice, ok := value.([]interface{}); ok {
						managers := make([]string, 0, len(slice))
						for _, item := range slice {
							if str, ok := item.(string); ok {
								managers = append(managers, str)
							}
						}
						tool.InstallerPriority[platform] = managers
					}
				}
			}
		}

		config.Tools[tool.Name] = tool
		toolNames = append(toolNames, tool.Name)
	}

	// Create scope
	config.Scopes[scopeName] = ScopeConfig{
		Description: scopeDescription,
		Tools:       toolNames,
	}

	return config
}
