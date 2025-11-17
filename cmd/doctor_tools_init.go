package cmd

import (
	"fmt"
	"os"

	"github.com/fulmenhq/goneat/internal/doctor"
	"github.com/fulmenhq/goneat/pkg/tools"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var toolsInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize .goneat/tools.yaml with foundation tools",
	Long: `Initialize .goneat/tools.yaml by seeding it with foundation tools appropriate for your repository.

This command:
- Detects your repository type (Go, Python, TypeScript, Rust, C#, or unknown)
- Loads foundation tools from embedded configuration
- Filters tools based on your detected language
- Generates .goneat/tools.yaml as the single source of truth

After running this command, .goneat/tools.yaml becomes the ONLY source of tool
configuration. There are NO hidden defaults or runtime merging.

Examples:
  goneat doctor tools init                    # Auto-detect language, foundation scope
  goneat doctor tools init --minimal          # Only language-native tools (CI-safe)
  goneat doctor tools init --language python  # Force Python tools
  goneat doctor tools init --force            # Overwrite existing .goneat/tools.yaml
  goneat doctor tools init --scope security   # Seed with security tools instead

Flags:
  --minimal     Include only minimal language-native tools (e.g., go-install, uv, npm)
  --language    Force language detection (go, python, typescript, rust, csharp)
  --scope       Scope to seed (foundation, security, format, all) [default: foundation]
  --force       Overwrite existing .goneat/tools.yaml without prompting`,
	RunE: runToolsInit,
}

var (
	toolsInitMinimal  bool
	toolsInitLanguage string
	toolsInitScope    string
	toolsInitForce    bool
)

func init() {
	doctorToolsCmd.AddCommand(toolsInitCmd)
	toolsInitCmd.Flags().BoolVar(&toolsInitMinimal, "minimal", false, "Include only minimal language-native tools")
	toolsInitCmd.Flags().StringVar(&toolsInitLanguage, "language", "", "Force language detection (go, python, typescript, rust, csharp)")
	toolsInitCmd.Flags().StringVar(&toolsInitScope, "scope", "foundation", "Scope to seed (foundation, security, format, all)")
	toolsInitCmd.Flags().BoolVar(&toolsInitForce, "force", false, "Overwrite existing .goneat/tools.yaml")
}

func runToolsInit(cmd *cobra.Command, args []string) error {
	configPath := ".goneat/tools.yaml"

	// Check if file already exists
	if _, err := os.Stat(configPath); err == nil && !toolsInitForce {
		return fmt.Errorf("%s already exists. Use --force to overwrite", configPath)
	}

	// Detect or use forced language
	language := toolsInitLanguage
	if language == "" {
		repoType := doctor.DetectCurrentRepoType()
		language = repoType.String()
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "🔍 Detected repository type: %s\n", language)
	} else {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "🎯 Using forced language: %s\n", language)
	}

	// Load tools defaults config
	defaultsConfig, err := doctor.LoadToolsDefaultsConfig()
	if err != nil {
		return fmt.Errorf("failed to load tools defaults: %w", err)
	}

	// Get tools for the specified scope
	scopeTools, err := defaultsConfig.GetToolsForScope(toolsInitScope)
	if err != nil {
		return fmt.Errorf("failed to get tools for scope %s: %w", toolsInitScope, err)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "📦 Loading scope: %s (%d tools)\n", toolsInitScope, len(scopeTools))

	// Filter by language
	var filteredTools []doctor.ToolDefinition
	if toolsInitMinimal {
		filteredTools = doctor.GetMinimalToolsForLanguage(scopeTools, language)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "🎯 Minimal mode: filtered to %d language-native tools\n", len(filteredTools))
	} else {
		filteredTools = doctor.FilterToolsByLanguage(scopeTools, language)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "🔧 Filtered to %d tools for %s\n", len(filteredTools), language)
	}

	if len(filteredTools) == 0 {
		return fmt.Errorf("no tools found for language %s and scope %s", language, toolsInitScope)
	}

	// Get scope description
	scopeDesc := "Foundation tools"
	if scopeDef, exists := defaultsConfig.Scopes[toolsInitScope]; exists {
		scopeDesc = scopeDef.Description
	}

	// Convert to ToolsConfig format
	toolsConfig := doctor.ConvertToToolsConfig(filteredTools, toolsInitScope, scopeDesc)

	// Ensure .goneat directory exists
	if err := os.MkdirAll(".goneat", 0755); err != nil {
		return fmt.Errorf("failed to create .goneat directory: %w", err)
	}

	// Write to .goneat/tools.yaml
	if err := writeToolsConfig(configPath, toolsConfig); err != nil {
		return fmt.Errorf("failed to write tools config: %w", err)
	}

	// Validate the generated config
	if err := validateGeneratedConfig(configPath); err != nil {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "⚠️  Warning: Generated config validation failed: %v\n", err)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "   Config was written but may have issues.\n")
	} else {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✅ Validated generated config\n")
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\n✨ Successfully created %s\n", configPath)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "   Tools: %d\n", len(toolsConfig.Tools))
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "   Scopes: %d\n", len(toolsConfig.Scopes))
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\n📋 Next steps:\n")
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "   1. Review %s and customize as needed\n", configPath)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "   2. Run: goneat doctor tools --scope %s\n", toolsInitScope)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "   3. Install missing tools if needed\n")
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\n💡 Note: .goneat/tools.yaml is now your ONLY source of tool configuration.\n")
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "         No hidden defaults or runtime merging will occur.\n")

	return nil
}

func writeToolsConfig(path string, config *doctor.ToolsConfig) error {
	// Marshal to YAML with custom formatting
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Add header comment
	header := []byte(`# goneat Tools Configuration (v1.0.0)
# Generated by: goneat doctor tools init
#
# This file is the SINGLE SOURCE OF TRUTH for tools configuration.
# There are NO hidden defaults or runtime merging.
#
# Edit this file to:
# - Add/remove tools
# - Change tool detection commands
# - Modify installer priorities
# - Define custom scopes
#
# Validate changes with: goneat doctor tools validate
# See schema: schemas/tools/tools.v1.0.0.json

`)

	fullData := append(header, data...)

	// Write to file
	if err := os.WriteFile(path, fullData, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

func validateGeneratedConfig(path string) error {
	// Read the file we just wrote
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	// Parse it
	config, err := tools.ParseConfig(data)
	if err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	// Basic validation
	if len(config.Tools) == 0 {
		return fmt.Errorf("config has no tools defined")
	}

	if len(config.Scopes) == 0 {
		return fmt.Errorf("config has no scopes defined")
	}

	// Verify all scope tools exist in tools map
	for scopeName, scope := range config.Scopes {
		for _, toolName := range scope.Tools {
			if _, exists := config.Tools[toolName]; !exists {
				return fmt.Errorf("scope %s references undefined tool: %s", scopeName, toolName)
			}
		}
	}

	return nil
}
