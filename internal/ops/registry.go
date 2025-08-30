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
	GroupSupport  CommandGroup = "support"  // Environment, config, info
	GroupWorkflow CommandGroup = "workflow" // Orchestration, automation
	GroupNeat     CommandGroup = "neat"     // Code quality operations
)

// CommandCategory represents functional classification within groups
type CommandCategory string

const (
	// GroupSupport Categories
	CategoryEnvironment   CommandCategory = "environment"   // envinfo, doctor, system diagnostics
	CategoryConfiguration CommandCategory = "configuration" // home, user config, preferences
	CategoryInformation   CommandCategory = "information"   // version, help, commands, licenses

	// GroupWorkflow Categories
	CategoryOrchestration CommandCategory = "orchestration" // hooks, automation, webhooks
	CategoryIntegration   CommandCategory = "integration"   // CI/CD, external system integration
	CategoryManagement    CommandCategory = "management"    // project setup, coordination

	// GroupNeat Categories
	CategoryFormatting CommandCategory = "formatting" // format, code style consistency
	CategoryAnalysis   CommandCategory = "analysis"   // lint, vet, static analysis
	CategoryAssessment CommandCategory = "assessment" // assess, multi-operation inspection
	CategoryValidation CommandCategory = "validation" // check operations, compliance
)

// CommandCapabilities defines what a command can do
type CommandCapabilities struct {
	SupportsJSON       bool     // Structured output for AI + human consumption
	SupportsWorkplan   bool     // Workplan-first execution for predictability
	SupportsParallel   bool     // Parallel execution for large codebases
	SupportsNoOp       bool     // Assessment mode without side effects
	OutputFormats      []string // ["json", "markdown", "html"]
	ExecutionModes     []string // ["check", "fix", "assess"]
	IntegrationPoints  []string // ["git-hooks", "ci-cd", "webhooks"]
}

// CommandRegistration represents a registered command with its classification
type CommandRegistration struct {
	Name         string
	Group        CommandGroup
	Category     CommandCategory
	Capabilities CommandCapabilities
	Command      *cobra.Command
	Description  string
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

// RegisterCommandWithTaxonomy registers a command with full taxonomy information
func RegisterCommandWithTaxonomy(name string, group CommandGroup, category CommandCategory, capabilities CommandCapabilities, cmd *cobra.Command, description string) error {
	reg := GetRegistry()
	return reg.RegisterWithTaxonomy(name, group, category, capabilities, cmd, description)
}

// GetDefaultCapabilities returns sensible defaults for command capabilities based on group
func GetDefaultCapabilities(group CommandGroup, category CommandCategory) CommandCapabilities {
	baseCapabilities := CommandCapabilities{
		OutputFormats:     []string{"markdown"},
		ExecutionModes:    []string{"check"},
		IntegrationPoints: []string{},
	}

	switch group {
	case GroupSupport:
		baseCapabilities.SupportsJSON = true
		baseCapabilities.OutputFormats = []string{"markdown", "json"}
		baseCapabilities.ExecutionModes = []string{"check"}

	case GroupWorkflow:
		baseCapabilities.SupportsJSON = true
		baseCapabilities.OutputFormats = []string{"markdown", "json"}
		baseCapabilities.ExecutionModes = []string{"check", "fix"}
		baseCapabilities.IntegrationPoints = []string{"git-hooks", "ci-cd", "webhooks"}

	case GroupNeat:
		baseCapabilities.SupportsJSON = true
		baseCapabilities.SupportsWorkplan = true
		baseCapabilities.SupportsNoOp = true
		baseCapabilities.OutputFormats = []string{"json", "markdown", "html"}
		baseCapabilities.ExecutionModes = []string{"check", "fix", "assess"}

		// Category-specific capabilities
		switch category {
		case CategoryFormatting:
			baseCapabilities.SupportsParallel = true
		case CategoryAnalysis:
			baseCapabilities.SupportsParallel = true
		case CategoryAssessment:
			baseCapabilities.SupportsParallel = true
		}
	}

	return baseCapabilities
}

// Register adds a command to the registry
func (r *Registry) Register(name string, group CommandGroup, cmd *cobra.Command, description string) error {
	return r.RegisterWithTaxonomy(name, group, "", CommandCapabilities{}, cmd, description)
}

// RegisterWithTaxonomy adds a command to the registry with full taxonomy information
func (r *Registry) RegisterWithTaxonomy(name string, group CommandGroup, category CommandCategory, capabilities CommandCapabilities, cmd *cobra.Command, description string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.commands[name]; exists {
		return fmt.Errorf("command %s already registered", name)
	}

	registration := &CommandRegistration{
		Name:         name,
		Group:        group,
		Category:     category,
		Capabilities: capabilities,
		Command:      cmd,
		Description:  description,
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

// GetCommandsByCategory returns all commands in a specific category
func (r *Registry) GetCommandsByCategory(category CommandCategory) []*CommandRegistration {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*CommandRegistration
	for _, cmd := range r.commands {
		if cmd.Category == category {
			result = append(result, cmd)
		}
	}
	return result
}

// GetCommandsByGroupAndCategory returns commands filtered by both group and category
func (r *Registry) GetCommandsByGroupAndCategory(group CommandGroup, category CommandCategory) []*CommandRegistration {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*CommandRegistration
	if commands, exists := r.groupIndex[group]; exists {
		for _, cmd := range commands {
			if cmd.Category == category {
				result = append(result, cmd)
			}
		}
	}
	return result
}

// GetCommandsWithCapability returns commands that support a specific capability
func (r *Registry) GetCommandsWithCapability(capability string, value bool) []*CommandRegistration {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*CommandRegistration
	for _, cmd := range r.commands {
		switch capability {
		case "SupportsJSON":
			if cmd.Capabilities.SupportsJSON == value {
				result = append(result, cmd)
			}
		case "SupportsWorkplan":
			if cmd.Capabilities.SupportsWorkplan == value {
				result = append(result, cmd)
			}
		case "SupportsParallel":
			if cmd.Capabilities.SupportsParallel == value {
				result = append(result, cmd)
			}
		case "SupportsNoOp":
			if cmd.Capabilities.SupportsNoOp == value {
				result = append(result, cmd)
			}
		}
	}
	return result
}

// GetCategoriesByGroup returns all categories used within a specific group
func (r *Registry) GetCategoriesByGroup(group CommandGroup) []CommandCategory {
	r.mu.RLock()
	defer r.mu.RUnlock()

	categorySet := make(map[CommandCategory]bool)
	if commands, exists := r.groupIndex[group]; exists {
		for _, cmd := range commands {
			if cmd.Category != "" {
				categorySet[cmd.Category] = true
			}
		}
	}

	var categories []CommandCategory
	for category := range categorySet {
		categories = append(categories, category)
	}
	return categories
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
