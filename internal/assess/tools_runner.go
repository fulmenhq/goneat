/*
Copyright © 2025 3 Leaps <info@3leaps.com>
*/
package assess

import (
	"context"
	"fmt"
	"strings"
	"time"

	intdoctor "github.com/fulmenhq/goneat/internal/doctor"
	"github.com/fulmenhq/goneat/pkg/logger"
)

// ToolsRunner implements AssessmentRunner for external tools validation
type ToolsRunner struct{}

// NewToolsRunner creates a new tools runner
func NewToolsRunner() *ToolsRunner {
	return &ToolsRunner{}
}

// Assess runs the tools assessment
func (r *ToolsRunner) Assess(ctx context.Context, target string, config AssessmentConfig) (*AssessmentResult, error) {
	startTime := time.Now()

	// Load tools configuration
	toolsConfig, err := intdoctor.LoadToolsConfig()
	if err != nil {
		return &AssessmentResult{
			CommandName:   "tools",
			Category:      r.GetCategory(),
			Success:       false,
			Issues:        []Issue{},
			Metrics:       map[string]interface{}{},
			ExecutionTime: HumanReadableDuration(time.Since(startTime)),
			Error:         fmt.Sprintf("failed to load tools configuration: %v", err),
		}, nil
	}

	// Get tools for foundation scope (most critical for CI/CD)
	infraTools, err := toolsConfig.GetToolsForScope("foundation")
	if err != nil {
		return &AssessmentResult{
			CommandName:   "tools",
			Category:      r.GetCategory(),
			Success:       false,
			Issues:        []Issue{},
			Metrics:       map[string]interface{}{},
			ExecutionTime: HumanReadableDuration(time.Since(startTime)),
			Error:         fmt.Sprintf("failed to get foundation tools: %v", err),
		}, nil
	}

	// Convert to legacy Tool format for compatibility
	selected, err := convertToolsConfigToLegacy(infraTools)
	if err != nil {
		return &AssessmentResult{
			CommandName:   "tools",
			Category:      r.GetCategory(),
			Success:       false,
			Issues:        []Issue{},
			Metrics:       map[string]interface{}{},
			ExecutionTime: HumanReadableDuration(time.Since(startTime)),
			Error:         fmt.Sprintf("failed to convert tools configuration: %v", err),
		}, nil
	}

	if len(selected) == 0 {
		logger.Info("No foundation tools configured")
		return &AssessmentResult{
			CommandName:   "tools",
			Category:      r.GetCategory(),
			Success:       true,
			Issues:        []Issue{},
			Metrics:       map[string]interface{}{"tools_checked": 0},
			ExecutionTime: HumanReadableDuration(time.Since(startTime)),
		}, nil
	}

	// Check each tool
	var issues []Issue
	var presentCount int
	var missingTools []string

	for _, tool := range selected {
		status := intdoctor.CheckTool(tool)
		if status.Present {
			presentCount++
			logger.Debug(fmt.Sprintf("Tool %s is present (version: %s)", tool.Name, status.Version))
		} else {
			missingTools = append(missingTools, tool.Name)
			issues = append(issues, Issue{
				File:     "tools",
				Line:     0,
				Severity: SeverityHigh,
				Message:  fmt.Sprintf("Required tool '%s' is not installed: %s", tool.Name, status.Instructions),
				Category: r.GetCategory(),
			})
		}
	}

	// Determine overall success
	success := len(issues) == 0

	// Build metrics
	metrics := map[string]interface{}{
		"tools_checked": len(selected),
		"tools_present": presentCount,
		"tools_missing": len(missingTools),
		"missing_tools": missingTools,
	}

	return &AssessmentResult{
		CommandName:   "tools",
		Category:      r.GetCategory(),
		Success:       success,
		Issues:        issues,
		Metrics:       metrics,
		ExecutionTime: HumanReadableDuration(time.Since(startTime)),
	}, nil
}

// CanRunInParallel returns true - tools checking can run in parallel
func (r *ToolsRunner) CanRunInParallel() bool {
	return true
}

// GetCategory returns the tools category
func (r *ToolsRunner) GetCategory() AssessmentCategory {
	return CategoryTools
}

// GetEstimatedTime provides a rough time estimate for tools checking
func (r *ToolsRunner) GetEstimatedTime(target string) time.Duration {
	// Tools checking is typically very fast (just command lookups)
	return 2 * time.Second
}

// IsAvailable returns whether tools checking is available
func (r *ToolsRunner) IsAvailable() bool {
	// Tools checking is always available (it checks for tools, doesn't require them)
	return true
}

// convertToolsConfigToLegacy converts ToolConfig slice to legacy Tool slice
func convertToolsConfigToLegacy(toolConfigs []intdoctor.ToolConfig) ([]intdoctor.Tool, error) {
	var tools []intdoctor.Tool

	for _, tc := range toolConfigs {
		tool := intdoctor.Tool{
			Name:        tc.Name,
			Description: tc.Description,
			Kind:        tc.Kind,
		}

		// Set install package for Go tools
		if tc.Kind == "go" {
			tool.InstallPackage = tc.InstallPackage
		}

		// Set install methods for system tools
		if tc.Kind == "system" {
			tool.InstallMethods = make(map[string]intdoctor.InstallMethod)
			for platform, command := range tc.InstallCommands {
				// Create detector closure for this platform
				detectCmd := tc.DetectCommand
				tool.InstallMethods[platform] = intdoctor.InstallMethod{
					Detector: func() (string, bool) {
						// Parse detect command into name and args
						parts := strings.Fields(detectCmd)
						if len(parts) == 0 {
							return "", false
						}
						name := parts[0]
						args := parts[1:]
						return intdoctor.TryCommand(name, args...)
					},
					Instructions: command,
				}
			}
		} else {
			// For Go tools, create a default install method with detector
			detectCmd := tc.DetectCommand
			tool.InstallMethods = map[string]intdoctor.InstallMethod{
				"all": {
					Detector: func() (string, bool) {
						// Parse detect command into name and args
						parts := strings.Fields(detectCmd)
						if len(parts) == 0 {
							return "", false
						}
						name := parts[0]
						args := parts[1:]
						return intdoctor.TryCommand(name, args...)
					},
					Instructions: fmt.Sprintf("go install %s", tc.InstallPackage),
				},
			}
		}

		tools = append(tools, tool)
	}

	return tools, nil
}

// init registers the tools assessment runner
func init() {
	RegisterAssessmentRunner(CategoryTools, NewToolsRunner())
}
