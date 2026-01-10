package doctor

import (
	"fmt"
	"io/fs"

	"github.com/fulmenhq/goneat/internal/assets"
	"gopkg.in/yaml.v3"
)

// ToolsDefaultsConfig represents the foundation-tools-defaults.yaml structure
// v0.4.4+: Organized into language-specific toolchain scopes
type ToolsDefaultsConfig struct {
	Version         string                     `yaml:"version"`
	FoundationTools []ToolDefinition           `yaml:"foundation_tools"` // Language-agnostic tools
	GoTools         []ToolDefinition           `yaml:"go_tools"`         // Go development tools
	RustTools       []ToolDefinition           `yaml:"rust_tools"`       // Rust Cargo plugins
	PythonTools     []ToolDefinition           `yaml:"python_tools"`     // Python tools (ruff)
	TypeScriptTools []ToolDefinition           `yaml:"typescript_tools"` // TS/JS tools (biome)
	SecurityTools   []ToolDefinition           `yaml:"security_tools"`   // Cross-language security
	SbomTools       []ToolDefinition           `yaml:"sbom_tools"`       // SBOM generation
	CicdTools       []ToolDefinition           `yaml:"cicd_tools"`       // Local CI/CD runners
	Scopes          map[string]ScopeDefinition `yaml:"scopes"`
}

// ToolDefinition represents a tool definition from the defaults config
type ToolDefinition struct {
	Name                 string              `yaml:"name"`
	Description          string              `yaml:"description"`
	Kind                 string              `yaml:"kind"`
	DetectCommand        string              `yaml:"detect_command"`
	Platforms            []string            `yaml:"platforms,omitempty"`
	PackageManagers      interface{}         `yaml:"package_managers"` // can be map[string][]string or simple
	AutoInstallSafe      bool                `yaml:"auto_install_safe"`
	RequiredForLanguages []string            `yaml:"required_for_languages,omitempty"`
	InstallPackage       string              `yaml:"install_package,omitempty"`
	VersionArgs          []string            `yaml:"version_args,omitempty"`
	CheckArgs            []string            `yaml:"check_args,omitempty"`
	InstallCommands      map[string]string   `yaml:"install_commands,omitempty"`
	InstallerPriority    map[string][]string `yaml:"installer_priority,omitempty"`
	VersionScheme        string              `yaml:"version_scheme,omitempty"`
	MinimumVersion       string              `yaml:"minimum_version,omitempty"`
	RecommendedVersion   string              `yaml:"recommended_version,omitempty"`
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
	allTools = append(allTools, c.GoTools...)
	allTools = append(allTools, c.RustTools...)
	allTools = append(allTools, c.PythonTools...)
	allTools = append(allTools, c.TypeScriptTools...)
	allTools = append(allTools, c.SecurityTools...)
	allTools = append(allTools, c.SbomTools...)
	allTools = append(allTools, c.CicdTools...)
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
			InstallCommands:    toolDef.InstallCommands,
			VersionScheme:      toolDef.VersionScheme,
			MinimumVersion:     toolDef.MinimumVersion,
			RecommendedVersion: toolDef.RecommendedVersion,
		}

		if len(toolDef.InstallerPriority) > 0 {
			tool.InstallerPriority = toolDef.InstallerPriority
		} else if toolDef.PackageManagers != nil {
			// Handle package_managers field - convert to installer_priority
			// foundation-tools-defaults.yaml uses package_managers, but .goneat/tools.yaml uses installer_priority
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

// ConvertToToolsConfigWithAllScopes generates a complete tools.yaml config
// with all standard scopes populated.
// v0.4.4+: Uses language-specific toolchain scopes (foundation, go, rust, python, typescript, security, sbom, cicd)
// This ensures users get a fully functional config regardless of which scope
// they specify during init.
//
// When minimal=true, only the language-specific scope is included (e.g., "go" for Go projects).
// When minimal=false, all scopes are included.
func ConvertToToolsConfigWithAllScopes(defaultsConfig *ToolsDefaultsConfig, language string, minimal bool) *ToolsConfig {
	config := &ToolsConfig{
		Tools:  make(map[string]ToolConfig),
		Scopes: make(map[string]ScopeConfig),
	}

	// Define standard scopes to generate (v0.4.4+ toolchain scopes)
	standardScopes := []string{"foundation", "go", "rust", "python", "typescript", "security", "sbom", "cicd", "all"}

	// In minimal mode, only include the language-specific scope
	if minimal && language != "" && language != "unknown" {
		standardScopes = []string{language}
	}

	// Collect all unique tools across all scopes
	allToolDefs := make(map[string]ToolDefinition)

	for _, scopeName := range standardScopes {
		scopeTools, err := defaultsConfig.GetToolsForScope(scopeName)
		if err != nil {
			continue // Skip scopes that don't exist
		}

		// In non-minimal mode, filter by language for universal tool compatibility
		var filteredTools []ToolDefinition
		if minimal {
			// In minimal mode, include all tools from the language-specific scope
			filteredTools = scopeTools
		} else {
			filteredTools = FilterToolsByLanguage(scopeTools, language)
		}

		// Build scope definition
		scopeToolNames := make([]string, 0, len(filteredTools))
		for _, toolDef := range filteredTools {
			allToolDefs[toolDef.Name] = toolDef
			scopeToolNames = append(scopeToolNames, toolDef.Name)
		}

		// Add scope if it has tools
		if len(scopeToolNames) > 0 {
			scopeDesc := "Tools scope"
			if scopeDef, exists := defaultsConfig.Scopes[scopeName]; exists {
				scopeDesc = scopeDef.Description
			}
			config.Scopes[scopeName] = ScopeConfig{
				Description: scopeDesc,
				Tools:       scopeToolNames,
			}
		}
	}

	// Convert all collected tool definitions to ToolConfig
	for _, toolDef := range allToolDefs {
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
	}

	return config
}
