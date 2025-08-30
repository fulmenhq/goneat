/*
Copyright © 2025 3 Leaps (hello@3leaps.net and https://3leaps.net)
*/
package ops

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestRegistry_BasicRegistration tests basic command registration functionality
func TestRegistry_BasicRegistration(t *testing.T) {
	registry := &Registry{
		commands:   make(map[string]*CommandRegistration),
		groupIndex: make(map[CommandGroup][]*CommandRegistration),
	}

	// Create a test command
	testCmd := &cobra.Command{
		Use:   "test",
		Short: "Test command",
	}

	// Test successful registration
	if err := registry.Register("test", GroupSupport, testCmd, "A test command"); err != nil {
		t.Fatalf("registration failed: %v", err)
	}

	// Verify command was registered
	cmd, exists := registry.GetCommand("test")
	if !exists {
		t.Fatal("Expected command to exist after registration")
	}

	if cmd.Name != "test" {
		t.Errorf("Expected command name 'test', got '%s'", cmd.Name)
	}

	if cmd.Group != GroupSupport {
		t.Errorf("Expected command group 'support', got '%s'", cmd.Group)
	}

	if cmd.Description != "A test command" {
		t.Errorf("Expected description 'A test command', got '%s'", cmd.Description)
	}

	if cmd.Command != testCmd {
		t.Error("Expected command object to match registered command")
	}
}

// TestRegistry_DuplicateRegistration tests handling of duplicate command registration
func TestRegistry_DuplicateRegistration(t *testing.T) {
	registry := &Registry{
		commands:   make(map[string]*CommandRegistration),
		groupIndex: make(map[CommandGroup][]*CommandRegistration),
	}

	testCmd1 := &cobra.Command{Use: "test", Short: "Test command 1"}
	testCmd2 := &cobra.Command{Use: "test", Short: "Test command 2"}

	// Register first command successfully
	if err := registry.Register("test", GroupSupport, testCmd1, "First test command"); err != nil {
		t.Fatalf("Expected first registration to succeed, got error: %v", err)
	}

	// Attempt to register duplicate command
	err := registry.Register("test", GroupWorkflow, testCmd2, "Second test command")
	if err == nil {
		t.Fatal("Expected duplicate registration to fail")
	}

	expectedError := "command test already registered"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}

	// Verify original command is still registered
	cmd, exists := registry.GetCommand("test")
	if !exists {
		t.Fatal("Expected original command to still exist")
	}

	if cmd.Group != GroupSupport {
		t.Errorf("Expected original command group to remain 'support', got '%s'", cmd.Group)
	}
}

// TestRegistry_GetCommand tests command retrieval functionality
func TestRegistry_GetCommand(t *testing.T) {
	registry := &Registry{
		commands:   make(map[string]*CommandRegistration),
		groupIndex: make(map[CommandGroup][]*CommandRegistration),
	}

	// Test retrieving non-existent command
	_, exists := registry.GetCommand("nonexistent")
	if exists {
		t.Error("Expected non-existent command to return false")
	}

	// Register a command
	testCmd := &cobra.Command{Use: "test", Short: "Test command"}
	if err := registry.Register("test", GroupSupport, testCmd, "A test command"); err != nil {
		t.Fatalf("registration failed: %v", err)
	}

	// Test retrieving existing command
	cmd, exists := registry.GetCommand("test")
	if !exists {
		t.Fatal("Expected existing command to be found")
	}

	if cmd.Name != "test" {
		t.Errorf("Expected retrieved command name 'test', got '%s'", cmd.Name)
	}
}

// TestRegistry_GetCommandsByGroup tests group-based command retrieval
func TestRegistry_GetCommandsByGroup(t *testing.T) {
	registry := &Registry{
		commands:   make(map[string]*CommandRegistration),
		groupIndex: make(map[CommandGroup][]*CommandRegistration),
	}

	// Test empty group
	commands := registry.GetCommandsByGroup(GroupSupport)
	if len(commands) != 0 {
		t.Errorf("Expected empty group to return 0 commands, got %d", len(commands))
	}

	// Register commands in different groups
	cmd1 := &cobra.Command{Use: "version", Short: "Version command"}
	cmd2 := &cobra.Command{Use: "format", Short: "Format command"}
	cmd3 := &cobra.Command{Use: "envinfo", Short: "Environment info"}

	if err := registry.Register("version", GroupSupport, cmd1, "Version management"); err != nil {
		t.Fatalf("registration failed: %v", err)
	}
	if err := registry.Register("format", GroupNeat, cmd2, "Code formatting"); err != nil {
		t.Fatalf("registration failed: %v", err)
	}
	if err := registry.Register("envinfo", GroupSupport, cmd3, "Environment information"); err != nil {
		t.Fatalf("registration failed: %v", err)
	}

	// Test support group
	supportCommands := registry.GetCommandsByGroup(GroupSupport)
	if len(supportCommands) != 2 {
		t.Errorf("Expected 2 support commands, got %d", len(supportCommands))
	}
	// Check that both version and envinfo are in support group
	commandNames := make(map[string]bool)
	for _, cmd := range supportCommands {
		commandNames[cmd.Name] = true
	}
	if !commandNames["version"] {
		t.Error("Expected 'version' command in support group")
	}
	if !commandNames["envinfo"] {
		t.Error("Expected 'envinfo' command in support group")
	}

	// Test neat group
	neatCommands := registry.GetCommandsByGroup(GroupNeat)
	if len(neatCommands) != 1 {
		t.Errorf("Expected 1 neat command, got %d", len(neatCommands))
	}
	if neatCommands[0].Name != "format" {
		t.Errorf("Expected neat command 'format', got '%s'", neatCommands[0].Name)
	}

	// Test workflow group (should be empty)
	workflowCommands := registry.GetCommandsByGroup(GroupWorkflow)
	if len(workflowCommands) != 0 {
		t.Errorf("Expected 0 workflow commands, got %d", len(workflowCommands))
	}
}

// TestRegistry_GetNeatCommands tests the convenience method for neat commands
func TestRegistry_GetNeatCommands(t *testing.T) {
	registry := &Registry{
		commands:   make(map[string]*CommandRegistration),
		groupIndex: make(map[CommandGroup][]*CommandRegistration),
	}

	// Register commands in different groups
	cmd1 := &cobra.Command{Use: "version", Short: "Version command"}
	cmd2 := &cobra.Command{Use: "format", Short: "Format command"}
	cmd3 := &cobra.Command{Use: "lint", Short: "Lint command"}

	if err := registry.Register("version", GroupSupport, cmd1, "Version management"); err != nil {
		t.Fatalf("registration failed: %v", err)
	}
	if err := registry.Register("format", GroupNeat, cmd2, "Code formatting"); err != nil {
		t.Fatalf("registration failed: %v", err)
	}
	if err := registry.Register("lint", GroupNeat, cmd3, "Code linting"); err != nil {
		t.Fatalf("registration failed: %v", err)
	}

	// Test GetNeatCommands
	neatCommands := registry.GetNeatCommands()
	if len(neatCommands) != 2 {
		t.Errorf("Expected 2 neat commands, got %d", len(neatCommands))
	}

	// Verify both commands are in the neat group
	commandNames := make(map[string]bool)
	for _, cmd := range neatCommands {
		commandNames[cmd.Name] = true
	}

	if !commandNames["format"] {
		t.Error("Expected 'format' command in neat commands")
	}
	if !commandNames["lint"] {
		t.Error("Expected 'lint' command in neat commands")
	}
}

// TestRegistry_GetAllCommands tests retrieval of all registered commands
func TestRegistry_GetAllCommands(t *testing.T) {
	registry := &Registry{
		commands:   make(map[string]*CommandRegistration),
		groupIndex: make(map[CommandGroup][]*CommandRegistration),
	}

	// Test empty registry
	allCommands := registry.GetAllCommands()
	if len(allCommands) != 0 {
		t.Errorf("Expected empty registry to return 0 commands, got %d", len(allCommands))
	}

	// Register multiple commands
	cmd1 := &cobra.Command{Use: "version", Short: "Version command"}
	cmd2 := &cobra.Command{Use: "format", Short: "Format command"}

	if err := registry.Register("version", GroupSupport, cmd1, "Version management"); err != nil {
		t.Fatalf("registration failed: %v", err)
	}
	if err := registry.Register("format", GroupNeat, cmd2, "Code formatting"); err != nil {
		t.Fatalf("registration failed: %v", err)
	}

	// Test retrieval of all commands
	allCommands = registry.GetAllCommands()
	if len(allCommands) != 2 {
		t.Errorf("Expected 2 commands, got %d", len(allCommands))
	}

	// Verify both commands are present
	if _, exists := allCommands["version"]; !exists {
		t.Error("Expected 'version' command in all commands")
	}
	if _, exists := allCommands["format"]; !exists {
		t.Error("Expected 'format' command in all commands")
	}

	// Verify command details
	versionCmd := allCommands["version"]
	if versionCmd.Group != GroupSupport {
		t.Errorf("Expected version command group 'support', got '%s'", versionCmd.Group)
	}
	if versionCmd.Description != "Version management" {
		t.Errorf("Expected version command description 'Version management', got '%s'", versionCmd.Description)
	}
}

// TestRegistry_ListGroups tests group listing functionality
func TestRegistry_ListGroups(t *testing.T) {
	registry := &Registry{
		commands:   make(map[string]*CommandRegistration),
		groupIndex: make(map[CommandGroup][]*CommandRegistration),
	}

	// Test empty registry
	groups := registry.ListGroups()
	if len(groups) != 0 {
		t.Errorf("Expected empty registry to have 0 groups, got %d", len(groups))
	}

	// Register commands in different groups
	cmd1 := &cobra.Command{Use: "version", Short: "Version command"}
	cmd2 := &cobra.Command{Use: "format", Short: "Format command"}
	cmd3 := &cobra.Command{Use: "lint", Short: "Lint command"}
	cmd4 := &cobra.Command{Use: "envinfo", Short: "Environment info"}
	cmd5 := &cobra.Command{Use: "hooks", Short: "Hooks command"}

	if err := registry.Register("version", GroupSupport, cmd1, "Version management"); err != nil {
		t.Fatalf("registration failed: %v", err)
	}
	if err := registry.Register("format", GroupNeat, cmd2, "Code formatting"); err != nil {
		t.Fatalf("registration failed: %v", err)
	}
	if err := registry.Register("lint", GroupNeat, cmd3, "Code linting"); err != nil {
		t.Fatalf("registration failed: %v", err)
	}
	if err := registry.Register("envinfo", GroupSupport, cmd4, "Environment information"); err != nil {
		t.Fatalf("registration failed: %v", err)
	}
	if err := registry.Register("hooks", GroupWorkflow, cmd5, "Hooks management"); err != nil {
		t.Fatalf("registration failed: %v", err)
	}

	// Test group listing
	groups = registry.ListGroups()
	if len(groups) != 3 {
		t.Errorf("Expected 3 groups, got %d", len(groups))
	}

	// Verify group counts
	if groups[GroupSupport] != 2 {
		t.Errorf("Expected 2 support commands, got %d", groups[GroupSupport])
	}
	if groups[GroupNeat] != 2 {
		t.Errorf("Expected 2 neat commands, got %d", groups[GroupNeat])
	}
	if groups[GroupWorkflow] != 1 {
		t.Errorf("Expected 1 workflow command, got %d", groups[GroupWorkflow])
	}
}

// TestGlobalRegistry tests the global registry functionality
func TestGlobalRegistry(t *testing.T) {
	// Get the global registry
	registry := GetRegistry()
	if registry == nil {
		t.Fatal("Expected global registry to be non-nil")
	}

	// Test global registration function
	testCmd := &cobra.Command{Use: "global-test", Short: "Global test command"}
	if err := RegisterCommand("global-test", GroupSupport, testCmd, "Global test command"); err != nil {
		t.Fatalf("Expected global registration to succeed, got error: %v", err)
	}

	// Verify command was registered globally
	cmd, exists := registry.GetCommand("global-test")
	if !exists {
		t.Fatal("Expected globally registered command to exist")
	}

	if cmd.Name != "global-test" {
		t.Errorf("Expected global command name 'global-test', got '%s'", cmd.Name)
	}

	if cmd.Group != GroupSupport {
		t.Errorf("Expected global command group 'support', got '%s'", cmd.Group)
	}
}

// TestCommandGroups tests the command group constants
func TestCommandGroups(t *testing.T) {
	// Test group constants
	if GroupSupport != "support" {
		t.Errorf("Expected GroupSupport to be 'support', got '%s'", GroupSupport)
	}
	if GroupWorkflow != "workflow" {
		t.Errorf("Expected GroupWorkflow to be 'workflow', got '%s'", GroupWorkflow)
	}
	if GroupNeat != "neat" {
		t.Errorf("Expected GroupNeat to be 'neat', got '%s'", GroupNeat)
	}

	// Test group type conversion
	var group CommandGroup = "support"
	if group != GroupSupport {
		t.Errorf("Expected group conversion to work, got '%s'", group)
	}
}

// TestTaxonomyValidation tests the taxonomy validation system
func TestTaxonomyValidation(t *testing.T) {
	// Create a test registry with known commands
	registry := &Registry{
		commands:   make(map[string]*CommandRegistration),
		groupIndex: make(map[CommandGroup][]*CommandRegistration),
	}

	// Register test commands
	testCmd1 := &cobra.Command{Use: "assess", Short: "Assessment command"}
	testCmd2 := &cobra.Command{Use: "format", Short: "Format command"}
	testCmd3 := &cobra.Command{Use: "envinfo", Short: "Environment info"}
	testCmd4 := &cobra.Command{Use: "version", Short: "Version command"}
	testCmd5 := &cobra.Command{Use: "home", Short: "Home command"}
	testCmd6 := &cobra.Command{Use: "hooks", Short: "Hooks command"}

	// Register with correct taxonomy
	if err := registry.RegisterWithTaxonomy("assess", GroupNeat, CategoryAssessment,
		GetDefaultCapabilities(GroupNeat, CategoryAssessment), testCmd1, "Assessment command"); err != nil {
		t.Fatalf("Failed to register assess: %v", err)
	}
	if err := registry.RegisterWithTaxonomy("format", GroupNeat, CategoryFormatting,
		GetDefaultCapabilities(GroupNeat, CategoryFormatting), testCmd2, "Format command"); err != nil {
		t.Fatalf("Failed to register format: %v", err)
	}
	if err := registry.RegisterWithTaxonomy("envinfo", GroupSupport, CategoryEnvironment,
		GetDefaultCapabilities(GroupSupport, CategoryEnvironment), testCmd3, "Environment info"); err != nil {
		t.Fatalf("Failed to register envinfo: %v", err)
	}
	if err := registry.RegisterWithTaxonomy("version", GroupSupport, CategoryInformation,
		GetDefaultCapabilities(GroupSupport, CategoryInformation), testCmd4, "Version command"); err != nil {
		t.Fatalf("Failed to register version: %v", err)
	}
	if err := registry.RegisterWithTaxonomy("home", GroupSupport, CategoryConfiguration,
		GetDefaultCapabilities(GroupSupport, CategoryConfiguration), testCmd5, "Home command"); err != nil {
		t.Fatalf("Failed to register home: %v", err)
	}
	if err := registry.RegisterWithTaxonomy("hooks", GroupWorkflow, CategoryOrchestration,
		GetDefaultCapabilities(GroupWorkflow, CategoryOrchestration), testCmd6, "Hooks command"); err != nil {
		t.Fatalf("Failed to register hooks: %v", err)
	}

	// Create validator and test
	validator := NewTaxonomyValidator()
	errors := validator.Validate(registry)

	// Should have no core command errors (all expected commands are registered correctly)
	coreErrors := FilterErrors(errors, ErrorTypeCoreCommand)
	if len(coreErrors) != 0 {
		t.Errorf("Expected no core command errors, got %d: %v", len(coreErrors), coreErrors)
	}

	// Should have extension warnings for any unexpected commands (none in this test)
	extensionWarnings := FilterErrors(errors, ErrorTypeExtensionWarning)
	// We expect warnings for commands not in the core set
	expectedWarnings := 0 // All our test commands are in the core set
	if len(extensionWarnings) != expectedWarnings {
		t.Errorf("Expected %d extension warnings, got %d: %v", expectedWarnings, len(extensionWarnings), extensionWarnings)
	}

	// Should have no taxonomy consistency errors
	consistencyErrors := FilterErrors(errors, ErrorTypeTaxonomyConsistency)
	if len(consistencyErrors) != 0 {
		t.Errorf("Expected no taxonomy consistency errors, got %d: %v", len(consistencyErrors), consistencyErrors)
	}
}

// TestTaxonomyValidation_MissingCoreCommand tests validation when core commands are missing
func TestTaxonomyValidation_MissingCoreCommand(t *testing.T) {
	// Create registry with missing core command
	registry := &Registry{
		commands:   make(map[string]*CommandRegistration),
		groupIndex: make(map[CommandGroup][]*CommandRegistration),
	}

	// Register only some core commands (missing assess)
	testCmd := &cobra.Command{Use: "format", Short: "Format command"}
	if err := registry.RegisterWithTaxonomy("format", GroupNeat, CategoryFormatting,
		GetDefaultCapabilities(GroupNeat, CategoryFormatting), testCmd, "Format command"); err != nil {
		t.Fatalf("Failed to register format: %v", err)
	}

	validator := NewTaxonomyValidator()
	errors := validator.Validate(registry)

	// Should have core command error for missing assess
	coreErrors := FilterErrors(errors, ErrorTypeCoreCommand)
	if len(coreErrors) == 0 {
		t.Error("Expected core command error for missing assess, got none")
	}

	// Check that the error is for the missing assess command
	foundAssessError := false
	for _, err := range coreErrors {
		if err.Command == "assess" && err.Message == "Core command is not registered" {
			foundAssessError = true
			break
		}
	}
	if !foundAssessError {
		t.Errorf("Expected error for missing assess command, got: %v", coreErrors)
	}
}

// TestTaxonomyValidation_WrongClassification tests validation when commands have wrong classification
func TestTaxonomyValidation_WrongClassification(t *testing.T) {
	registry := &Registry{
		commands:   make(map[string]*CommandRegistration),
		groupIndex: make(map[CommandGroup][]*CommandRegistration),
	}

	// Register assess with wrong group (should be GroupNeat, registering as GroupSupport)
	testCmd := &cobra.Command{Use: "assess", Short: "Assessment command"}
	if err := registry.RegisterWithTaxonomy("assess", GroupSupport, CategoryEnvironment, // Wrong!
		GetDefaultCapabilities(GroupSupport, CategoryEnvironment), testCmd, "Assessment command"); err != nil {
		t.Fatalf("Failed to register assess: %v", err)
	}

	validator := NewTaxonomyValidator()
	errors := validator.Validate(registry)

	coreErrors := FilterErrors(errors, ErrorTypeCoreCommand)

	// Should have error for wrong group
	foundGroupError := false
	for _, err := range coreErrors {
		if err.Command == "assess" && strings.Contains(err.Message, "Incorrect group") {
			foundGroupError = true
			break
		}
	}
	if !foundGroupError {
		t.Errorf("Expected group classification error for assess, got: %v", coreErrors)
	}
}

// TestTaxonomyValidation_ExtensionCommands tests validation of extension commands
func TestTaxonomyValidation_ExtensionCommands(t *testing.T) {
	registry := &Registry{
		commands:   make(map[string]*CommandRegistration),
		groupIndex: make(map[CommandGroup][]*CommandRegistration),
	}

	// Register core commands
	coreCmd := &cobra.Command{Use: "assess", Short: "Assessment command"}
	if err := registry.RegisterWithTaxonomy("assess", GroupNeat, CategoryAssessment,
		GetDefaultCapabilities(GroupNeat, CategoryAssessment), coreCmd, "Assessment command"); err != nil {
		t.Fatalf("Failed to register assess: %v", err)
	}

	// Register extension command (not in core set)
	extCmd := &cobra.Command{Use: "custom-tool", Short: "Custom tool"}
	if err := registry.RegisterWithTaxonomy("custom-tool", GroupNeat, CategoryAnalysis,
		GetDefaultCapabilities(GroupNeat, CategoryAnalysis), extCmd, "Custom tool"); err != nil {
		t.Fatalf("Failed to register custom-tool: %v", err)
	}

	validator := NewTaxonomyValidator()
	errors := validator.Validate(registry)

	extensionWarnings := FilterErrors(errors, ErrorTypeExtensionWarning)

	// Should have warning for extension command
	if len(extensionWarnings) == 0 {
		t.Error("Expected extension warning for custom-tool, got none")
	}

	foundWarning := false
	for _, warning := range extensionWarnings {
		if warning.Command == "custom-tool" {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Errorf("Expected warning for custom-tool extension, got: %v", extensionWarnings)
	}
}

// TestTaxonomyValidation_InvalidCategory tests validation of invalid category usage
func TestTaxonomyValidation_InvalidCategory(t *testing.T) {
	registry := &Registry{
		commands:   make(map[string]*CommandRegistration),
		groupIndex: make(map[CommandGroup][]*CommandRegistration),
	}

	// Register command with invalid category for group
	testCmd := &cobra.Command{Use: "test", Short: "Test command"}
	if err := registry.RegisterWithTaxonomy("test", GroupSupport, CategoryFormatting, // Formatting not allowed in Support
		GetDefaultCapabilities(GroupSupport, CategoryEnvironment), testCmd, "Test command"); err != nil {
		t.Fatalf("Failed to register test: %v", err)
	}

	validator := NewTaxonomyValidator()
	errors := validator.Validate(registry)

	consistencyErrors := FilterErrors(errors, ErrorTypeTaxonomyConsistency)

	// Should have consistency error for invalid category
	if len(consistencyErrors) == 0 {
		t.Error("Expected taxonomy consistency error for invalid category, got none")
	}

	foundError := false
	for _, err := range consistencyErrors {
		if err.Command == "test" && strings.Contains(err.Message, "not allowed for group") {
			foundError = true
			break
		}
	}
	if !foundError {
		t.Errorf("Expected consistency error for invalid category, got: %v", consistencyErrors)
	}
}

// TestTaxonomyValidationUtilities tests utility functions
func TestTaxonomyValidationUtilities(t *testing.T) {
	// Test error filtering
	errors := []ValidationError{
		{Type: ErrorTypeCoreCommand, Command: "test1", Message: "error1"},
		{Type: ErrorTypeExtensionWarning, Command: "test2", Message: "warning1"},
		{Type: ErrorTypeCoreCommand, Command: "test3", Message: "error2"},
	}

	coreErrors := FilterErrors(errors, ErrorTypeCoreCommand)
	if len(coreErrors) != 2 {
		t.Errorf("Expected 2 core errors, got %d", len(coreErrors))
	}

	warningErrors := FilterErrors(errors, ErrorTypeExtensionWarning)
	if len(warningErrors) != 1 {
		t.Errorf("Expected 1 warning error, got %d", len(warningErrors))
	}

	// Test severity filtering
	severityErrors := FilterErrorsBySeverity(errors, SeverityError)
	if len(severityErrors) != 3 {
		t.Errorf("Expected 3 severity errors, got %d", len(severityErrors))
	}

	// Test error formatting
	formatted := FormatErrors(errors)
	if !strings.Contains(formatted, "Found 3 validation errors") {
		t.Errorf("Expected formatted output to contain error count, got: %s", formatted)
	}
}
