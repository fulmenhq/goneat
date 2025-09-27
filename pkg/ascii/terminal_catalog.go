package ascii

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fulmenhq/goneat/internal/assets"
	"github.com/fulmenhq/goneat/pkg/config"
	"github.com/fulmenhq/goneat/pkg/logger"
	"github.com/fulmenhq/goneat/pkg/schema"
	"github.com/mattn/go-runewidth"
	"gopkg.in/yaml.v3"
)

// TerminalOverrides represents the width override configuration for terminals
type TerminalOverrides struct {
	Version   string                    `yaml:"version" json:"version"`
	Terminals map[string]TerminalConfig `yaml:"terminals" json:"terminals"`
}

// TerminalConfig contains terminal-specific character width overrides
type TerminalConfig struct {
	Name      string         `yaml:"name" json:"name"`
	Overrides map[string]int `yaml:"overrides,omitempty" json:"overrides,omitempty"`
	Notes     string         `yaml:"notes,omitempty" json:"notes,omitempty"`
}

var (
	// terminalCatalog holds the loaded terminal configurations
	terminalCatalog *TerminalOverrides
	// currentTerminalConfig holds the config for the detected terminal
	currentTerminalConfig *TerminalConfig
)

func init() {
	// Load the terminal catalog
	if err := loadTerminalCatalog(); err != nil {
		// If loading fails, we'll just use defaults
		// Don't panic - this is a nice-to-have feature
		// For debugging: uncomment the line below if needed
		// fmt.Printf("Failed to load terminal catalog: %v\n", err)
		return
	}

	// Detect current terminal and set config
	detectCurrentTerminal()
}

// loadTerminalCatalog loads the terminal override configuration
func loadTerminalCatalog() error {
	// Load default configuration from embedded assets
	defaultData, ok := assets.GetAsset("terminal-overrides.yaml")
	if !ok {
		return fmt.Errorf("embedded terminal overrides not found")
	}

	// Parse default configuration
	var defaultConfig TerminalOverrides
	if err := yaml.Unmarshal(defaultData, &defaultConfig); err != nil {
		return fmt.Errorf("failed to parse default terminal overrides: %w", err)
	}

	// Validate default configuration against schema
	schemaData, ok := assets.GetSchema("embedded_schemas/schemas/ascii/v1.0.0/terminal-overrides.yaml")
	if !ok {
		return fmt.Errorf("terminal overrides schema not found")
	}

	result, err := schema.ValidateDataFromBytes(schemaData, defaultData)
	if err != nil {
		return fmt.Errorf("failed to validate default config: %w", err)
	}
	if !result.Valid {
		var errs []string
		for _, e := range result.Errors {
			errs = append(errs, fmt.Sprintf("%s: %s", e.Path, e.Message))
		}
		return fmt.Errorf("default config validation failed: %s", strings.Join(errs, "; "))
	}

	// Start with default configuration
	terminalCatalog = &defaultConfig

	// Check for user overrides in GONEAT_HOME
	if err := loadUserOverrides(schemaData); err != nil {
		// Log error but don't fail - user overrides are optional
		logger.Debug("Failed to load user terminal overrides", logger.Err(err))
	}

	return nil
}

// loadUserOverrides loads and merges user-specific terminal overrides
func loadUserOverrides(schemaData []byte) error {
	goneatHome, err := config.GetGoneatHome()
	if err != nil {
		return err
	}

	userConfigPath := filepath.Join(goneatHome, "config", "terminal-overrides.yaml")

	// Check if user config exists
	if _, err := os.Stat(userConfigPath); os.IsNotExist(err) {
		return nil // No user overrides, which is fine
	}

	// Read user configuration
	userData, err := os.ReadFile(userConfigPath)
	if err != nil {
		return fmt.Errorf("failed to read user terminal overrides: %w", err)
	}

	// Validate user configuration against schema
	result, err := schema.ValidateDataFromBytes(schemaData, userData)
	if err != nil {
		return fmt.Errorf("failed to validate user config: %w", err)
	}
	if !result.Valid {
		var errs []string
		for _, e := range result.Errors {
			errs = append(errs, fmt.Sprintf("%s: %s", e.Path, e.Message))
		}
		return fmt.Errorf("user config validation failed: %s", strings.Join(errs, "; "))
	}

	// Parse user configuration
	var userConfig TerminalOverrides
	if err := yaml.Unmarshal(userData, &userConfig); err != nil {
		return fmt.Errorf("failed to parse user terminal overrides: %w", err)
	}

	// Merge user overrides with defaults
	mergeTerminalConfigs(terminalCatalog, &userConfig)

	return nil
}

// mergeTerminalConfigs merges user overrides into the base configuration
func mergeTerminalConfigs(base, override *TerminalOverrides) {
	if override.Terminals == nil {
		return
	}

	if base.Terminals == nil {
		base.Terminals = make(map[string]TerminalConfig)
	}

	// Merge terminal configurations
	for termID, overrideConfig := range override.Terminals {
		if baseConfig, exists := base.Terminals[termID]; exists {
			// Merge overrides into existing terminal config
			if overrideConfig.Overrides != nil {
				if baseConfig.Overrides == nil {
					baseConfig.Overrides = make(map[string]int)
				}
				for char, width := range overrideConfig.Overrides {
					baseConfig.Overrides[char] = width
				}
			}
			// Update name and notes if provided
			if overrideConfig.Name != "" {
				baseConfig.Name = overrideConfig.Name
			}
			if overrideConfig.Notes != "" {
				baseConfig.Notes = overrideConfig.Notes
			}
			base.Terminals[termID] = baseConfig
		} else {
			// Add new terminal configuration
			base.Terminals[termID] = overrideConfig
		}
	}
}

// detectCurrentTerminal identifies the current terminal and loads its config
func detectCurrentTerminal() {
	termProgram := os.Getenv("TERM_PROGRAM")
	if termProgram == "" {
		return
	}

	if terminalCatalog != nil && terminalCatalog.Terminals != nil {
		if config, ok := terminalCatalog.Terminals[termProgram]; ok {
			// Make a copy of the config to avoid pointer issues
			configCopy := config
			currentTerminalConfig = &configCopy
		}
	}
}

// ReloadTerminalDetection forces a reload of terminal detection
// This is useful when environment variables change after initialization
func ReloadTerminalDetection() {
	detectCurrentTerminal()
}

// GetTerminalWidth returns the width for a string, considering terminal-specific overrides
func GetTerminalWidth(s string) int {
	// Ensure terminal detection is current (in case env vars changed)
	if currentTerminalConfig == nil {
		detectCurrentTerminal()
	}

	// First check if we have a terminal-specific override for this exact string
	if currentTerminalConfig != nil && currentTerminalConfig.Overrides != nil {
		if width, ok := currentTerminalConfig.Overrides[s]; ok {
			return width
		}

		// For longer strings, check if they contain emoji sequences with overrides
		if len(s) > 1 && len(currentTerminalConfig.Overrides) > 0 {
			baseWidth := runewidth.StringWidth(s)
			adjustment := 0

			// Check each override to see if it appears in the string
			for emoji, expectedWidth := range currentTerminalConfig.Overrides {
				count := strings.Count(s, emoji)
				if count > 0 {
					currentWidth := runewidth.StringWidth(emoji)
					adjustment += count * (expectedWidth - currentWidth)
				}
			}

			if adjustment != 0 {
				return baseWidth + adjustment
			}
		}
	}

	// Fall back to go-runewidth
	return runewidth.StringWidth(s)
}

// GetTerminalConfig returns the current terminal configuration (for diagnostics)
func GetTerminalConfig() *TerminalConfig {
	return currentTerminalConfig
}

// DebugTerminalCatalog prints debug information about the loaded terminal catalog
func DebugTerminalCatalog() {
	fmt.Printf("=== Terminal Catalog Debug ===\n")
	fmt.Printf("terminalCatalog loaded: %v\n", terminalCatalog != nil)

	if terminalCatalog != nil {
		fmt.Printf("Version: %s\n", terminalCatalog.Version)
		fmt.Printf("Number of terminals: %d\n", len(terminalCatalog.Terminals))

		for termID, config := range terminalCatalog.Terminals {
			fmt.Printf("\nTerminal: %s\n", termID)
			fmt.Printf("  Name: %s\n", config.Name)
			fmt.Printf("  Overrides: %d\n", len(config.Overrides))
			for char, width := range config.Overrides {
				fmt.Printf("    %q -> %d\n", char, width)
			}
		}
	}

	fmt.Printf("\ncurrentTerminalConfig: %v\n", currentTerminalConfig != nil)
	if currentTerminalConfig != nil {
		fmt.Printf("Current terminal name: %s\n", currentTerminalConfig.Name)
		fmt.Printf("Current overrides: %d\n", len(currentTerminalConfig.Overrides))
		for char, width := range currentTerminalConfig.Overrides {
			fmt.Printf("  %q -> %d\n", char, width)
		}
	}

	fmt.Printf("\nEnvironment:\n")
	fmt.Printf("TERM_PROGRAM: %s\n", os.Getenv("TERM_PROGRAM"))
	fmt.Printf("TERM: %s\n", os.Getenv("TERM"))
}

// GetAllTerminalConfigs returns all known terminal configurations (for testing)
func GetAllTerminalConfigs() map[string]TerminalConfig {
	if terminalCatalog == nil {
		return nil
	}
	return terminalCatalog.Terminals
}

// ExportTerminalData exports the terminal data as JSON for analysis
func ExportTerminalData() (string, error) {
	data, err := json.MarshalIndent(terminalCatalog, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// TerminalTestReport generates a report for testing a terminal
func TerminalTestReport(termProgram string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Terminal Test Report: %s\n", termProgram))
	sb.WriteString(strings.Repeat("=", 50) + "\n\n")

	// Get the config if it exists
	config, exists := terminalCatalog.Terminals[termProgram]
	if exists {
		sb.WriteString(fmt.Sprintf("Known Terminal: %s\n", config.Name))
		sb.WriteString(fmt.Sprintf("Notes: %s\n", config.Notes))
		sb.WriteString(fmt.Sprintf("Override Count: %d\n\n", len(config.Overrides)))

		if len(config.Overrides) > 0 {
			sb.WriteString("Overrides:\n")
			for char, width := range config.Overrides {
				sb.WriteString(fmt.Sprintf("  %q -> width %d\n", char, width))
			}
		}
	} else {
		sb.WriteString("Unknown terminal - no overrides configured\n")
		sb.WriteString("Please test and report any width issues\n")
	}

	return sb.String()
}
