/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package ops

import (
	"fmt"
	"sync"

	"github.com/spf13/cobra"
)

// CommandGroup represents the operational classification of commands
type CommandGroup string

const (
	GroupSupport CommandGroup = "support" // envinfo, help, version info
	GroupUtility CommandGroup = "utility" // version management, config
	GroupNeat    CommandGroup = "neat"    // format, lint, check, analyze
)

// CommandRegistration represents a registered command with its classification
type CommandRegistration struct {
	Name        string
	Group       CommandGroup
	Command     *cobra.Command
	Description string
}

// Registry manages command classifications and registrations
type Registry struct {
	mu         sync.RWMutex
	commands   map[string]*CommandRegistration
	groupIndex map[CommandGroup][]*CommandRegistration
}

// Global registry instance
var globalRegistry = &Registry{
	commands:   make(map[string]*CommandRegistration),
	groupIndex: make(map[CommandGroup][]*CommandRegistration),
}

// GetRegistry returns the global command registry
func GetRegistry() *Registry {
	return globalRegistry
}

// RegisterCommand registers a command with its operational classification
func RegisterCommand(name string, group CommandGroup, cmd *cobra.Command, description string) error {
	reg := GetRegistry()
	return reg.Register(name, group, cmd, description)
}

// Register adds a command to the registry
func (r *Registry) Register(name string, group CommandGroup, cmd *cobra.Command, description string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.commands[name]; exists {
		return fmt.Errorf("command %s already registered", name)
	}

	registration := &CommandRegistration{
		Name:        name,
		Group:       group,
		Command:     cmd,
		Description: description,
	}

	r.commands[name] = registration
	r.groupIndex[group] = append(r.groupIndex[group], registration)

	return nil
}

// GetCommand returns a registered command by name
func (r *Registry) GetCommand(name string) (*CommandRegistration, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cmd, exists := r.commands[name]
	return cmd, exists
}

// GetCommandsByGroup returns all commands in a specific group
func (r *Registry) GetCommandsByGroup(group CommandGroup) []*CommandRegistration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.groupIndex[group]
}

// GetNeatCommands returns all commands classified as "neat" operations
func (r *Registry) GetNeatCommands() []*CommandRegistration {
	return r.GetCommandsByGroup(GroupNeat)
}

// GetAllCommands returns all registered commands
func (r *Registry) GetAllCommands() map[string]*CommandRegistration {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]*CommandRegistration)
	for k, v := range r.commands {
		result[k] = v
	}
	return result
}

// ListGroups returns all command groups and their command counts
func (r *Registry) ListGroups() map[CommandGroup]int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[CommandGroup]int)
	for group, commands := range r.groupIndex {
		result[group] = len(commands)
	}
	return result
}
