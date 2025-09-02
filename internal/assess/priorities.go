/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package assess

import (
	"fmt"
	"sort"
	"strings"
)

// DefaultPriorities defines the expert-driven priority order for assessment categories
var DefaultPriorities = map[AssessmentCategory]int{
	CategoryFormat:         1, // Quick wins, often auto-fixable
	CategorySecurity:       2, // Critical issues, block progress
	CategoryStaticAnalysis: 3, // Code correctness, potential bugs
	CategoryLint:           4, // Code quality, variable effort
	CategoryPerformance:    5, // Optimization, may be deferred
}

// PriorityManager handles category prioritization and ordering
type PriorityManager struct {
	customPriorities map[AssessmentCategory]int
}

// NewPriorityManager creates a new priority manager with default priorities
func NewPriorityManager() *PriorityManager {
	return &PriorityManager{
		customPriorities: make(map[AssessmentCategory]int),
	}
}

// SetCustomPriority sets a custom priority for a category
func (pm *PriorityManager) SetCustomPriority(category AssessmentCategory, priority int) {
	pm.customPriorities[category] = priority
}

// GetPriority returns the priority for a category (custom or default)
func (pm *PriorityManager) GetPriority(category AssessmentCategory) int {
	if priority, exists := pm.customPriorities[category]; exists {
		return priority
	}
	if priority, exists := DefaultPriorities[category]; exists {
		return priority
	}
	return 999 // Default for unknown categories
}

// GetOrderedCategories returns categories sorted by priority (low number = high priority)
func (pm *PriorityManager) GetOrderedCategories(categories []AssessmentCategory) []AssessmentCategory {
	ordered := make([]AssessmentCategory, len(categories))
	copy(ordered, categories)

	sort.Slice(ordered, func(i, j int) bool {
		return pm.GetPriority(ordered[i]) < pm.GetPriority(ordered[j])
	})

	return ordered
}

// ParsePriorityString parses a priority string like "security=1,format=2,lint=3"
func (pm *PriorityManager) ParsePriorityString(priorityStr string) error {
	if priorityStr == "" {
		return nil
	}

	parts := strings.Split(priorityStr, ",")
	validEntries := 0

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			return fmt.Errorf("invalid priority format: %s (expected category=priority)", part)
		}

		category := AssessmentCategory(strings.TrimSpace(kv[0]))
		var priority int
		if kv[1] == "default" {
			delete(pm.customPriorities, category)
			validEntries++
			continue
		}

		// Parse priority number
		switch strings.TrimSpace(kv[1]) {
		case "1", "highest":
			priority = 1
		case "2", "high":
			priority = 2
		case "3", "medium":
			priority = 3
		case "4", "low":
			priority = 4
		case "5", "lowest":
			priority = 5
		default:
			return fmt.Errorf("invalid priority value: %s (expected 1-5 or highest/lowest)", kv[1])
		}

		pm.SetCustomPriority(category, priority)
		validEntries++
	}

	if validEntries == 0 {
		return fmt.Errorf("no valid priority entries found in: %s", priorityStr)
	}

	return nil
}

// GetPriorityDescription returns a human-readable description of a category's priority
func (pm *PriorityManager) GetPriorityDescription(category AssessmentCategory) string {
	priority := pm.GetPriority(category)

	switch priority {
	case 1:
		return "Highest Priority - Quick wins, often auto-fixable"
	case 2:
		return "High Priority - Critical issues that may block progress"
	case 3:
		return "Medium Priority - Code correctness and potential bugs"
	case 4:
		return "Medium Priority - Code quality improvements"
	case 5:
		return "Low Priority - Optimization opportunities"
	default:
		return "Unknown Priority"
	}
}

// GetAllCategories returns all known categories in priority order
func (pm *PriorityManager) GetAllCategories() []AssessmentCategory {
	categories := make([]AssessmentCategory, 0, len(DefaultPriorities))
	for category := range DefaultPriorities {
		categories = append(categories, category)
	}
	return pm.GetOrderedCategories(categories)
}
