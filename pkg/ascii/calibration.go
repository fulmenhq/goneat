package ascii

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fulmenhq/goneat/internal/assets"
	"github.com/fulmenhq/goneat/pkg/config"
	"github.com/fulmenhq/goneat/pkg/schema"
	"gopkg.in/yaml.v3"
)

// CalibrationSession manages interactive terminal calibration
type CalibrationSession struct {
	TerminalID   string
	TerminalName string
	TestFile     string
	Adjustments  map[string]int // character -> width adjustment
	// Override options
	OverrideTerm        string
	OverrideTermProgram string
}

// NewCalibrationSession creates a new calibration session
func NewCalibrationSession(testFile string) *CalibrationSession {
	return &CalibrationSession{
		TestFile:    testFile,
		Adjustments: make(map[string]int),
	}
}

// WithTerminalOverride sets terminal detection overrides
func (cs *CalibrationSession) WithTerminalOverride(term, termProgram string) *CalibrationSession {
	cs.OverrideTerm = term
	cs.OverrideTermProgram = termProgram
	return cs
}

// initializeTerminalInfo initializes terminal ID and name based on overrides or detection
func (cs *CalibrationSession) initializeTerminalInfo() {
	if cs.TerminalID != "" {
		return // Already initialized
	}

	cs.TerminalID = cs.detectTerminalIDWithOverrides()

	var terminalName string
	if terminalCatalog != nil && terminalCatalog.Terminals != nil {
		if config, exists := terminalCatalog.Terminals[cs.TerminalID]; exists {
			terminalName = config.Name
		}
	}
	if terminalName == "" {
		// Provide better default names for known terminal IDs
		switch cs.TerminalID {
		case "iTerm.app":
			terminalName = "iTerm2"
		case "Apple_Terminal":
			terminalName = "macOS Terminal"
		case "ghostty":
			terminalName = "Ghostty"
		case "com.apple.Terminal":
			terminalName = "macOS Terminal"
		case "Hyper":
			terminalName = "Hyper"
		case "WezTerm":
			terminalName = "WezTerm"
		case "Alacritty":
			terminalName = "Alacritty"
		default:
			terminalName = fmt.Sprintf("Terminal (%s)", cs.TerminalID)
		}
	}
	cs.TerminalName = terminalName
}

// RunInteractiveCalibration runs an interactive calibration session
func (cs *CalibrationSession) RunInteractiveCalibration() error {
	// Initialize terminal info if not already done
	cs.initializeTerminalInfo()

	fmt.Printf("🎯 ASCII Terminal Calibration for %s\n", cs.TerminalName)
	fmt.Printf("Terminal ID: %s\n", cs.TerminalID)
	if cs.OverrideTerm != "" || cs.OverrideTermProgram != "" {
		fmt.Printf("Override Mode: TERM=%s TERM_PROGRAM=%s\n", cs.OverrideTerm, cs.OverrideTermProgram)
	}
	fmt.Printf("Test File: %s\n\n", cs.TestFile)

	// Read test content
	content, err := os.ReadFile(cs.TestFile)
	if err != nil {
		return fmt.Errorf("failed to read test file: %w", err)
	}

	lines := strings.Split(string(content), "\n")

	fmt.Println("📦 Rendering test box:")
	fmt.Println()

	// Create and display test box
	box := Box(lines)
	fmt.Print(box)
	fmt.Println()

	fmt.Println("Visual Inspection Instructions:")
	fmt.Println("- Look at the box borders - are they properly aligned?")
	fmt.Println("- Are any lines too short (characters extend past right border)?")
	fmt.Println("- Are any lines too long (extra space before right border)?")
	fmt.Println()

	return cs.promptForAdjustments(lines)
}

// promptForAdjustments prompts user for character width adjustments
func (cs *CalibrationSession) promptForAdjustments(lines []string) error {
	reader := bufio.NewReader(os.Stdin)

	// Extract unique characters from the test content
	charSet := make(map[rune]bool)
	for _, line := range lines {
		for _, r := range line {
			if r > 127 { // Only Unicode characters beyond ASCII
				charSet[r] = true
			}
		}
	}

	// Convert to sorted slice for consistent ordering
	var chars []string
	for r := range charSet {
		chars = append(chars, string(r))
	}
	sort.Strings(chars)

	fmt.Printf("Found %d non-ASCII characters in test file.\n\n", len(chars))

	for {
		fmt.Println("Calibration Options:")
		fmt.Println("1. Mark characters as too wide (appearing wider than calculated)")
		fmt.Println("2. Mark characters as too narrow (appearing narrower than calculated)")
		fmt.Println("3. Show current adjustments")
		fmt.Println("4. Test with current adjustments")
		fmt.Println("5. Save adjustments and exit")
		fmt.Println("6. Exit without saving")
		fmt.Print("\nChoice (1-6): ")

		choice, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}
		choice = strings.TrimSpace(choice)

		switch choice {
		case "1":
			if err := cs.markCharactersWide(reader, chars); err != nil {
				fmt.Printf("Error: %v\n", err)
			}
		case "2":
			if err := cs.markCharactersNarrow(reader, chars); err != nil {
				fmt.Printf("Error: %v\n", err)
			}
		case "3":
			cs.showCurrentAdjustments()
		case "4":
			if err := cs.testWithAdjustments(lines); err != nil {
				fmt.Printf("Error: %v\n", err)
			}
		case "5":
			return cs.saveAdjustments()
		case "6":
			fmt.Println("Exiting without saving adjustments.")
			return nil
		default:
			fmt.Println("Invalid choice. Please enter 1-6.")
		}
		fmt.Println()
	}
}

// markCharactersWide prompts user to mark characters as too wide
func (cs *CalibrationSession) markCharactersWide(reader *bufio.Reader, chars []string) error {
	fmt.Println("\nCharacters available for marking as TOO WIDE:")
	for i, char := range chars {
		currentWidth := StringWidth(char)
		adjustment := cs.Adjustments[char]
		effectiveWidth := currentWidth + adjustment
		fmt.Printf("%2d. %s (current: %d, effective: %d)\n", i+1, char, currentWidth, effectiveWidth)
	}

	fmt.Print("\nEnter character numbers (comma-separated) or 'done': ")
	input, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	input = strings.TrimSpace(input)
	if input == "done" || input == "" {
		return nil
	}

	return cs.processCharacterSelection(input, chars, 1)
}

// markCharactersNarrow prompts user to mark characters as too narrow
func (cs *CalibrationSession) markCharactersNarrow(reader *bufio.Reader, chars []string) error {
	fmt.Println("\nCharacters available for marking as TOO NARROW:")
	for i, char := range chars {
		currentWidth := StringWidth(char)
		adjustment := cs.Adjustments[char]
		effectiveWidth := currentWidth + adjustment
		fmt.Printf("%2d. %s (current: %d, effective: %d)\n", i+1, char, currentWidth, effectiveWidth)
	}

	fmt.Print("\nEnter character numbers (comma-separated) or 'done': ")
	input, err := reader.ReadString('\n')
	if err != nil {
		return err
	}

	input = strings.TrimSpace(input)
	if input == "done" || input == "" {
		return nil
	}

	return cs.processCharacterSelection(input, chars, -1)
}

// processCharacterSelection processes user's character selection
func (cs *CalibrationSession) processCharacterSelection(input string, chars []string, adjustment int) error {
	parts := strings.Split(input, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		var idx int
		if _, err := fmt.Sscanf(part, "%d", &idx); err != nil {
			fmt.Printf("Invalid number: %s\n", part)
			continue
		}

		if idx < 1 || idx > len(chars) {
			fmt.Printf("Number out of range: %d (valid: 1-%d)\n", idx, len(chars))
			continue
		}

		char := chars[idx-1]
		cs.Adjustments[char] += adjustment

		action := "narrower"
		if adjustment > 0 {
			action = "wider"
		}
		fmt.Printf("Marked %s as %s (adjustment: %+d)\n", char, action, cs.Adjustments[char])
	}

	return nil
}

// showCurrentAdjustments displays current adjustments
func (cs *CalibrationSession) showCurrentAdjustments() {
	if len(cs.Adjustments) == 0 {
		fmt.Println("No adjustments made yet.")
		return
	}

	fmt.Println("Current adjustments:")
	var chars []string
	for char := range cs.Adjustments {
		chars = append(chars, char)
	}
	sort.Strings(chars)

	for _, char := range chars {
		adjustment := cs.Adjustments[char]
		if adjustment != 0 {
			currentWidth := StringWidth(char)
			newWidth := currentWidth + adjustment
			fmt.Printf("  %s: %d -> %d (%+d)\n", char, currentWidth, newWidth, adjustment)
		}
	}
}

// testWithAdjustments temporarily applies adjustments and shows test box
func (cs *CalibrationSession) testWithAdjustments(lines []string) error {
	fmt.Println("📦 Testing with current adjustments:")
	fmt.Println()

	// Temporarily modify StringWidth behavior
	// Note: This is a simplified version - in practice we'd need to
	// implement a more sophisticated override system
	originalAdjustments := make(map[string]int)
	for char, adj := range cs.Adjustments {
		originalAdjustments[char] = adj
	}

	// Apply adjustments by creating a test configuration
	box := cs.createTestBox(lines)
	fmt.Print(box)
	fmt.Println()

	return nil
}

// createTestBox creates a box with temporary width adjustments
func (cs *CalibrationSession) createTestBox(lines []string) string {
	// Calculate max width with adjustments
	maxWidth := 0
	for _, line := range lines {
		width := cs.calculateAdjustedWidth(line)
		if width > maxWidth {
			maxWidth = width
		}
	}

	// Build box
	var sb strings.Builder

	// Top border
	sb.WriteString("┌")
	sb.WriteString(strings.Repeat("─", maxWidth+2))
	sb.WriteString("┐\n")

	// Content lines
	for _, line := range lines {
		sb.WriteString("│ ")
		sb.WriteString(line)

		// Padding
		currentWidth := cs.calculateAdjustedWidth(line)
		padding := maxWidth - currentWidth
		if padding > 0 {
			sb.WriteString(strings.Repeat(" ", padding))
		}
		sb.WriteString(" │\n")
	}

	// Bottom border
	sb.WriteString("└")
	sb.WriteString(strings.Repeat("─", maxWidth+2))
	sb.WriteString("┘\n")

	return sb.String()
}

// calculateAdjustedWidth calculates string width with adjustments applied
func (cs *CalibrationSession) calculateAdjustedWidth(s string) int {
	width := 0
	for _, r := range s {
		char := string(r)
		baseWidth := StringWidth(char)
		adjustment := cs.Adjustments[char]
		width += baseWidth + adjustment
	}
	return width
}

// saveAdjustments saves adjustments to user's GONEAT_HOME
func (cs *CalibrationSession) saveAdjustments() error {
	if len(cs.Adjustments) == 0 {
		fmt.Println("No adjustments to save.")
		return nil
	}

	// Filter out zero adjustments
	finalAdjustments := make(map[string]int)
	for char, adj := range cs.Adjustments {
		if adj != 0 {
			currentWidth := StringWidth(char)
			finalAdjustments[char] = currentWidth + adj
		}
	}

	if len(finalAdjustments) == 0 {
		fmt.Println("No effective adjustments to save.")
		return nil
	}

	// Get config directory (creates ~/.goneat/config/ if needed)
	configDir, err := config.GetConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}

	configFile := filepath.Join(configDir, "terminal-overrides.yaml")

	// Load existing config or initialize from embedded default
	var userConfig TerminalOverrides
	if data, err := os.ReadFile(configFile); err == nil {
		// Existing user config found
		if err := yaml.Unmarshal(data, &userConfig); err != nil {
			return fmt.Errorf("failed to parse existing config: %w", err)
		}
	} else {
		// No user config - initialize from embedded default
		if err := cs.initializeUserConfigFromEmbedded(configFile); err != nil {
			// If initialization fails, create minimal config
			fmt.Printf("Warning: Could not initialize from embedded config: %v\n", err)
			userConfig = TerminalOverrides{
				Version:   "1.0.0",
				Terminals: make(map[string]TerminalConfig),
			}
		} else {
			// Re-read the initialized config
			if data, err := os.ReadFile(configFile); err == nil {
				if err := yaml.Unmarshal(data, &userConfig); err != nil {
					return fmt.Errorf("failed to parse initialized config: %w", err)
				}
			} else {
				return fmt.Errorf("failed to read initialized config: %w", err)
			}
		}
	}

	// Update terminal configuration
	termConfig := userConfig.Terminals[cs.TerminalID]
	termConfig.Name = cs.TerminalName
	if termConfig.Overrides == nil {
		termConfig.Overrides = make(map[string]int)
	}

	// Apply adjustments
	for char, width := range finalAdjustments {
		termConfig.Overrides[char] = width
	}

	termConfig.Notes = fmt.Sprintf("Calibrated via interactive session on %s", cs.TerminalName)
	userConfig.Terminals[cs.TerminalID] = termConfig

	// Validate against schema
	schemaData, ok := getTerminalOverridesSchema()
	if ok {
		var configInterface interface{}
		configBytes, _ := yaml.Marshal(userConfig)
		if err := yaml.Unmarshal(configBytes, &configInterface); err == nil {
			if result, err := schema.ValidateDataFromBytes(schemaData, configBytes); err == nil {
				if !result.Valid {
					fmt.Println("Warning: Generated config doesn't validate against schema:")
					for _, e := range result.Errors {
						fmt.Printf("  %s: %s\n", e.Path, e.Message)
					}
				}
			}
		}
	}

	// Save config
	configBytes, err := yaml.Marshal(userConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configFile, configBytes, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("✅ Saved %d character adjustments to %s\n", len(finalAdjustments), configFile)
	fmt.Println("Restart goneat or reload configuration to apply changes.")

	return nil
}

// initializeUserConfigFromEmbedded copies the embedded default config to user config
func (cs *CalibrationSession) initializeUserConfigFromEmbedded(userConfigFile string) error {
	// Get embedded config data
	embeddedData, ok := assets.GetAsset("terminal-overrides.yaml")
	if !ok {
		return fmt.Errorf("embedded terminal config not found")
	}

	// Write embedded config to user location
	if err := os.WriteFile(userConfigFile, embeddedData, 0644); err != nil {
		return fmt.Errorf("failed to initialize user config: %w", err)
	}

	fmt.Printf("ℹ️  Initialized user terminal config from embedded defaults: %s\n", userConfigFile)
	return nil
}

// detectTerminalIDWithOverrides detects terminal ID with override support
func (cs *CalibrationSession) detectTerminalIDWithOverrides() string {
	// Use overrides if provided
	if cs.OverrideTermProgram != "" {
		return cs.OverrideTermProgram
	}

	if cs.OverrideTerm != "" {
		// Handle some common TERM values
		if strings.Contains(cs.OverrideTerm, "ghostty") {
			return "ghostty"
		}
		return cs.OverrideTerm
	}

	// Fall back to environment detection
	return detectTerminalID()
}

// detectTerminalID detects the current terminal identifier
func detectTerminalID() string {
	if termProgram := os.Getenv("TERM_PROGRAM"); termProgram != "" {
		return termProgram
	}

	if term := os.Getenv("TERM"); term != "" {
		// Handle some common TERM values
		if strings.Contains(term, "ghostty") {
			return "ghostty"
		}
		return term
	}

	return "unknown"
}

// getTerminalOverridesSchema gets the schema for validation
func getTerminalOverridesSchema() ([]byte, bool) {
	// Get schema from embedded assets
	return assets.GetSchema("embedded_schemas/schemas/ascii/v1.0.0/terminal-overrides.yaml")
}

// Quick calibration functions for CLI convenience

// MarkCharactersWide marks specified characters as wider in current terminal
func MarkCharactersWide(characters []string) error {
	return MarkCharactersWideWithOverride(characters, "", "")
}

// MarkCharactersWideWithOverride marks characters as wider with terminal overrides
func MarkCharactersWideWithOverride(characters []string, term, termProgram string) error {
	cs := &CalibrationSession{
		Adjustments:         make(map[string]int),
		OverrideTerm:        term,
		OverrideTermProgram: termProgram,
	}
	cs.initializeTerminalInfo()

	for _, char := range characters {
		cs.Adjustments[char] = 1
	}

	return cs.saveAdjustments()
}

// MarkCharactersNarrow marks specified characters as narrower in current terminal
func MarkCharactersNarrow(characters []string) error {
	return MarkCharactersNarrowWithOverride(characters, "", "")
}

// MarkCharactersNarrowWithOverride marks characters as narrower with terminal overrides
func MarkCharactersNarrowWithOverride(characters []string, term, termProgram string) error {
	cs := &CalibrationSession{
		Adjustments:         make(map[string]int),
		OverrideTerm:        term,
		OverrideTermProgram: termProgram,
	}
	cs.initializeTerminalInfo()

	for _, char := range characters {
		cs.Adjustments[char] = -1
	}

	return cs.saveAdjustments()
}

// ResetUserConfig resets user terminal configuration to embedded defaults
func ResetUserConfig() error {
	// Get config directory
	configDir, err := config.GetConfigDir()
	if err != nil {
		return fmt.Errorf("failed to get config directory: %w", err)
	}

	userConfigFile := filepath.Join(configDir, "terminal-overrides.yaml")

	// Get embedded config data
	embeddedData, ok := assets.GetAsset("terminal-overrides.yaml")
	if !ok {
		return fmt.Errorf("embedded terminal config not found")
	}

	// Write embedded config to user location (overwrite existing)
	if err := os.WriteFile(userConfigFile, embeddedData, 0644); err != nil {
		return fmt.Errorf("failed to reset user config: %w", err)
	}

	fmt.Printf("✅ Reset user terminal config to defaults: %s\n", userConfigFile)
	fmt.Println("ℹ️  Restart goneat or reload configuration to apply changes.")

	return nil
}
