package cmd

import (
	"fmt"
	"os"
	"path/filepath"

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
	configPath := filepath.Clean(".goneat/tools.yaml")

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

	// Generate complete config with ALL standard scopes
	// This ensures .goneat/tools.yaml is fully functional regardless of which scope was requested
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "📦 Generating tools.yaml with all standard scopes...\n")

	toolsConfig := doctor.ConvertToToolsConfigWithAllScopes(defaultsConfig, language, toolsInitMinimal)

	if len(toolsConfig.Tools) == 0 {
		return fmt.Errorf("no tools found for language %s", language)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "🔧 Generated %d tools across %d scopes for %s\n", len(toolsConfig.Tools), len(toolsConfig.Scopes), language)

	// Ensure .goneat directory exists
	if err := os.MkdirAll(".goneat", 0750); err != nil {
		return fmt.Errorf("failed to create .goneat directory: %w", err)
	}

	// Write to .goneat/tools.yaml
	// #nosec G304 -- configPath is a repo-rooted, cleaned path (.goneat/tools.yaml)
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
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "   2. Run: goneat doctor tools --scope foundation\n")
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "   3. Install missing tools: goneat doctor tools --scope foundation --install --yes\n")
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\n💡 Note: .goneat/tools.yaml is now your ONLY source of tool configuration.\n")
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "         No hidden defaults or runtime merging will occur.\n")

	return nil
}

func writeToolsConfig(path string, config *doctor.ToolsConfig) (err error) {
	// Create file for writing
	file, createErr := os.Create(path) // #nosec G304 -- path validated and repo-rooted (.goneat/tools.yaml)
	if createErr != nil {
		return fmt.Errorf("failed to create file: %w", createErr)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("failed to close file: %w", closeErr)
		}
	}()

	// Write header comment
	// Note: No trailing newline - the YAML encoder will add one before the first key
	header := `# goneat Tools Configuration (v1.0.0)
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
`
	if _, err := file.WriteString(header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Create YAML encoder with indentation matching local .yamlfmt config
	indent := detectYamlfmtIndent()
	encoder := yaml.NewEncoder(file)
	encoder.SetIndent(indent)

	if err := encoder.Encode(config); err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}

	if err := encoder.Close(); err != nil {
		return fmt.Errorf("failed to close encoder: %w", err)
	}

	// Ensure file permissions are secure
	if err := file.Chmod(0600); err != nil {
		return fmt.Errorf("failed to set file permissions: %w", err)
	}

	return nil
}

func validateGeneratedConfig(path string) error {
	// Read the file we just wrote
	// #nosec G304 - path is the hardcoded ".goneat/tools.yaml" passed from runToolsInit
	// This function only validates the config file we just created in writeToolsConfig
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

// yamlfmtConfig represents the structure of a .yamlfmt configuration file.
// We only parse the fields we need for indent detection.
type yamlfmtConfig struct {
	Formatter struct {
		Indent int `yaml:"indent"`
	} `yaml:"formatter"`
}

// detectYamlfmtIndent searches for a .yamlfmt configuration file starting from
// the current directory and walking up to the filesystem root. Returns the
// configured indent value if found, otherwise returns the default of 2.
//
// Security: Applies hardcoded limits to reject malicious/corrupt indent values.
// Valid indent range: 1-8 spaces (inclusive). Values outside this range are
// treated as invalid and the default is returned.
func detectYamlfmtIndent() int {
	const (
		defaultIndent = 2
		minIndent     = 1
		maxIndent     = 8 // No legitimate use case for more than 8 spaces
	)

	// Start from current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return defaultIndent
	}

	// Walk up the directory tree looking for .yamlfmt
	dir := cwd
	for {
		yamlfmtPath := filepath.Join(dir, ".yamlfmt")

		// Try to read the .yamlfmt file
		// #nosec G304 - yamlfmtPath is constructed from cwd + ".yamlfmt"
		data, err := os.ReadFile(yamlfmtPath)
		if err == nil {
			// Parse the YAML config
			var cfg yamlfmtConfig
			if err := yaml.Unmarshal(data, &cfg); err == nil {
				indent := cfg.Formatter.Indent
				// Only use the value if it's within sane limits
				// This protects against malicious/corrupt configs
				if indent >= minIndent && indent <= maxIndent {
					return indent
				}
			}
			// Found file but couldn't parse or indent out of range, use default
			return defaultIndent
		}

		// Move up to parent directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			break
		}
		dir = parent
	}

	return defaultIndent
}
