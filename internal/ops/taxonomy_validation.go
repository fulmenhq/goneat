/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package ops

import (
	"fmt"
	"strings"
)

// TaxonomyValidator validates command taxonomy consistency and correctness
type TaxonomyValidator struct {
	coreCommands      map[string]CommandClassification
	allowedGroups     []CommandGroup
	allowedCategories map[CommandGroup][]CommandCategory
}

// CommandClassification represents the expected classification for a command
type CommandClassification struct {
	Group    CommandGroup
	Category CommandCategory
}

// ErrorType represents different types of validation errors
type ErrorType int

const (
	ErrorTypeCoreCommand ErrorType = iota
	ErrorTypeExtensionWarning
	ErrorTypeTaxonomyConsistency
)

// ErrorSeverity represents the severity of validation errors
type ErrorSeverity int

const (
	SeverityError ErrorSeverity = iota
	SeverityWarning
	SeverityInfo
)

// ValidationError represents a taxonomy validation error
type ValidationError struct {
	Type     ErrorType
	Severity ErrorSeverity
	Command  string
	Message  string
}

// Error implements the error interface
func (e ValidationError) Error() string {
	return fmt.Sprintf("[%s] %s: %s", e.severityString(), e.Command, e.Message)
}

func (e ValidationError) severityString() string {
	switch e.Severity {
	case SeverityError:
		return "ERROR"
	case SeverityWarning:
		return "WARNING"
	case SeverityInfo:
		return "INFO"
	default:
		return "UNKNOWN"
	}
}

// NewTaxonomyValidator creates a new taxonomy validator with default core commands
func NewTaxonomyValidator() *TaxonomyValidator {
	return &TaxonomyValidator{
		coreCommands:      getDefaultCoreCommands(),
		allowedGroups:     getAllowedGroups(),
		allowedCategories: getAllowedCategories(),
	}
}

// Validate performs comprehensive taxonomy validation
func (v *TaxonomyValidator) Validate(registry *Registry) []ValidationError {
	var errors []ValidationError

	// 1. Validate core commands exist with correct classification
	coreErrors := v.validateCoreCommands(registry)
	errors = append(errors, coreErrors...)

	// 2. Validate taxonomy consistency
	consistencyErrors := v.validateTaxonomyConsistency(registry)
	errors = append(errors, consistencyErrors...)

	// 3. Validate extension commands (warnings only)
	extensionErrors := v.validateExtensionCommands(registry)
	errors = append(errors, extensionErrors...)

	return errors
}

// validateCoreCommands ensures all core commands exist with correct classification
func (v *TaxonomyValidator) validateCoreCommands(registry *Registry) []ValidationError {
	var errors []ValidationError

	for commandName, expected := range v.coreCommands {
		cmd, exists := registry.GetCommand(commandName)
		if !exists {
			errors = append(errors, ValidationError{
				Type:     ErrorTypeCoreCommand,
				Severity: SeverityError,
				Command:  commandName,
				Message:  "Core command is not registered",
			})
			continue
		}

		if cmd.Group != expected.Group {
			errors = append(errors, ValidationError{
				Type:     ErrorTypeCoreCommand,
				Severity: SeverityError,
				Command:  commandName,
				Message:  fmt.Sprintf("Incorrect group: expected %s, got %s", expected.Group, cmd.Group),
			})
		}

		if cmd.Category != expected.Category {
			errors = append(errors, ValidationError{
				Type:     ErrorTypeCoreCommand,
				Severity: SeverityError,
				Command:  commandName,
				Message:  fmt.Sprintf("Incorrect category: expected %s, got %s", expected.Category, cmd.Category),
			})
		}
	}

	return errors
}

// validateTaxonomyConsistency ensures taxonomy structure is valid
func (v *TaxonomyValidator) validateTaxonomyConsistency(registry *Registry) []ValidationError {
	var errors []ValidationError

	allCommands := registry.GetAllCommands()

	// Check that all commands use valid group/category combinations
	for name, cmd := range allCommands {
		// Validate group is allowed
		if !v.isGroupAllowed(cmd.Group) {
			errors = append(errors, ValidationError{
				Type:     ErrorTypeTaxonomyConsistency,
				Severity: SeverityError,
				Command:  name,
				Message:  fmt.Sprintf("Uses invalid group: %s", cmd.Group),
			})
		}

		// Validate category is allowed for this group
		if !v.isCategoryAllowedForGroup(cmd.Category, cmd.Group) {
			errors = append(errors, ValidationError{
				Type:     ErrorTypeTaxonomyConsistency,
				Severity: SeverityError,
				Command:  name,
				Message:  fmt.Sprintf("Category %s not allowed for group %s", cmd.Category, cmd.Group),
			})
		}
	}

	return errors
}

// validateExtensionCommands checks for unexpected commands (warnings only)
func (v *TaxonomyValidator) validateExtensionCommands(registry *Registry) []ValidationError {
	var errors []ValidationError

	allCommands := registry.GetAllCommands()

	for name := range allCommands {
		if _, isCore := v.coreCommands[name]; !isCore {
			errors = append(errors, ValidationError{
				Type:     ErrorTypeExtensionWarning,
				Severity: SeverityWarning,
				Command:  name,
				Message:  "Extension command detected - ensure proper documentation",
			})
		}
	}

	return errors
}

// Helper methods

func (v *TaxonomyValidator) isGroupAllowed(group CommandGroup) bool {
	for _, allowed := range v.allowedGroups {
		if allowed == group {
			return true
		}
	}
	return false
}

func (v *TaxonomyValidator) isCategoryAllowedForGroup(category CommandCategory, group CommandGroup) bool {
	allowedCategories, exists := v.allowedCategories[group]
	if !exists {
		return false
	}

	for _, allowed := range allowedCategories {
		if allowed == category {
			return true
		}
	}
	return false
}

// Default configuration

func getDefaultCoreCommands() map[string]CommandClassification {
	return map[string]CommandClassification{
		"assess":  {Group: GroupNeat, Category: CategoryAssessment},
		"format":  {Group: GroupNeat, Category: CategoryFormatting},
		"envinfo": {Group: GroupSupport, Category: CategoryEnvironment},
		"version": {Group: GroupSupport, Category: CategoryInformation},
		"home":    {Group: GroupSupport, Category: CategoryConfiguration},
		"hooks":   {Group: GroupWorkflow, Category: CategoryOrchestration},
		// Note: lint, doctor, help, commands will be added as core when implemented
	}
}

func getAllowedGroups() []CommandGroup {
	return []CommandGroup{
		GroupSupport,
		GroupWorkflow,
		GroupNeat,
	}
}

func getAllowedCategories() map[CommandGroup][]CommandCategory {
	return map[CommandGroup][]CommandCategory{
		GroupSupport: {
			CategoryEnvironment,
			CategoryConfiguration,
			CategoryInformation,
		},
		GroupWorkflow: {
			CategoryOrchestration,
			CategoryIntegration,
			CategoryManagement,
		},
		GroupNeat: {
			CategoryFormatting,
			CategoryAnalysis,
			CategoryAssessment,
			CategoryValidation,
		},
	}
}

// Utility functions for filtering errors

// FilterErrors returns errors of a specific type
func FilterErrors(errors []ValidationError, errorType ErrorType) []ValidationError {
	var filtered []ValidationError
	for _, err := range errors {
		if err.Type == errorType {
			filtered = append(filtered, err)
		}
	}
	return filtered
}

// FilterErrorsBySeverity returns errors of a specific severity
func FilterErrorsBySeverity(errors []ValidationError, severity ErrorSeverity) []ValidationError {
	var filtered []ValidationError
	for _, err := range errors {
		if err.Severity == severity {
			filtered = append(filtered, err)
		}
	}
	return filtered
}

// FormatErrors formats validation errors for display
func FormatErrors(errors []ValidationError) string {
	if len(errors) == 0 {
		return "No validation errors found"
	}

	var builder strings.Builder
	fmt.Fprintf(&builder, "Found %d validation errors:\n", len(errors))

	for i, err := range errors {
		fmt.Fprintf(&builder, "%d. %s\n", i+1, err.Error())
	}

	return builder.String()
}
