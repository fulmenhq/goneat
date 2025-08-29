/*
Copyright © 2025 3 Leaps (hello@3leaps.net and https://3leaps.net)
*/
package ops

import (
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
	if err := registry.Register("test", GroupUtility, testCmd, "A test command"); err != nil {
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

	if cmd.Group != GroupUtility {
		t.Errorf("Expected command group 'utility', got '%s'", cmd.Group)
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
	if err := registry.Register("test", GroupUtility, testCmd1, "First test command"); err != nil {
		t.Fatalf("Expected first registration to succeed, got error: %v", err)
	}

	// Attempt to register duplicate command
	err := registry.Register("test", GroupSupport, testCmd2, "Second test command")
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

	if cmd.Group != GroupUtility {
		t.Errorf("Expected original command group to remain 'utility', got '%s'", cmd.Group)
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
	if err := registry.Register("test", GroupUtility, testCmd, "A test command"); err != nil {
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
	commands := registry.GetCommandsByGroup(GroupUtility)
	if len(commands) != 0 {
		t.Errorf("Expected empty group to return 0 commands, got %d", len(commands))
	}

	// Register commands in different groups
	cmd1 := &cobra.Command{Use: "version", Short: "Version command"}
	cmd2 := &cobra.Command{Use: "format", Short: "Format command"}
	cmd3 := &cobra.Command{Use: "envinfo", Short: "Environment info"}

	if err := registry.Register("version", GroupUtility, cmd1, "Version management"); err != nil {
		t.Fatalf("registration failed: %v", err)
	}
	if err := registry.Register("format", GroupNeat, cmd2, "Code formatting"); err != nil {
		t.Fatalf("registration failed: %v", err)
	}
	if err := registry.Register("envinfo", GroupSupport, cmd3, "Environment information"); err != nil {
		t.Fatalf("registration failed: %v", err)
	}

	// Test utility group
	utilityCommands := registry.GetCommandsByGroup(GroupUtility)
	if len(utilityCommands) != 1 {
		t.Errorf("Expected 1 utility command, got %d", len(utilityCommands))
	}
	if utilityCommands[0].Name != "version" {
		t.Errorf("Expected utility command 'version', got '%s'", utilityCommands[0].Name)
	}

	// Test neat group
	neatCommands := registry.GetCommandsByGroup(GroupNeat)
	if len(neatCommands) != 1 {
		t.Errorf("Expected 1 neat command, got %d", len(neatCommands))
	}
	if neatCommands[0].Name != "format" {
		t.Errorf("Expected neat command 'format', got '%s'", neatCommands[0].Name)
	}

	// Test support group
	supportCommands := registry.GetCommandsByGroup(GroupSupport)
	if len(supportCommands) != 1 {
		t.Errorf("Expected 1 support command, got %d", len(supportCommands))
	}
	if supportCommands[0].Name != "envinfo" {
		t.Errorf("Expected support command 'envinfo', got '%s'", supportCommands[0].Name)
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

	if err := registry.Register("version", GroupUtility, cmd1, "Version management"); err != nil {
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

	if err := registry.Register("version", GroupUtility, cmd1, "Version management"); err != nil {
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
	if versionCmd.Group != GroupUtility {
		t.Errorf("Expected version command group 'utility', got '%s'", versionCmd.Group)
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

	if err := registry.Register("version", GroupUtility, cmd1, "Version management"); err != nil {
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

	// Test group listing
	groups = registry.ListGroups()
	if len(groups) != 3 {
		t.Errorf("Expected 3 groups, got %d", len(groups))
	}

	// Verify group counts
	if groups[GroupUtility] != 1 {
		t.Errorf("Expected 1 utility command, got %d", groups[GroupUtility])
	}
	if groups[GroupNeat] != 2 {
		t.Errorf("Expected 2 neat commands, got %d", groups[GroupNeat])
	}
	if groups[GroupSupport] != 1 {
		t.Errorf("Expected 1 support command, got %d", groups[GroupSupport])
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
	if err := RegisterCommand("global-test", GroupUtility, testCmd, "Global test command"); err != nil {
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

	if cmd.Group != GroupUtility {
		t.Errorf("Expected global command group 'utility', got '%s'", cmd.Group)
	}
}

// TestCommandGroups tests the command group constants
func TestCommandGroups(t *testing.T) {
	// Test group constants
	if GroupSupport != "support" {
		t.Errorf("Expected GroupSupport to be 'support', got '%s'", GroupSupport)
	}
	if GroupUtility != "utility" {
		t.Errorf("Expected GroupUtility to be 'utility', got '%s'", GroupUtility)
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
