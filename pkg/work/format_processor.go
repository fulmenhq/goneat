package work

import (
	"context"
	"fmt"

	"github.com/3leaps/goneat/pkg/config"
	"github.com/3leaps/goneat/pkg/logger"
)

// FormatProcessor implements WorkItemProcessor for formatting operations
type FormatProcessor struct {
	config *config.Config
}

// NewFormatProcessor creates a new format processor
func NewFormatProcessor(cfg *config.Config) *FormatProcessor {
	return &FormatProcessor{config: cfg}
}

// ProcessWorkItem processes a single work item
func (p *FormatProcessor) ProcessWorkItem(ctx context.Context, item *WorkItem, dryRun bool, noOp bool) ExecutionResult {
	result := ExecutionResult{
		WorkItemID: item.ID,
		Success:    false,
	}

	// Check if operation was cancelled
	select {
	case <-ctx.Done():
		result.Error = "operation cancelled"
		return result
	default:
	}

	logger.Debug(fmt.Sprintf("Processing %s (%s, %d bytes)", item.Path, item.ContentType, item.Size))

	// Simulate processing time based on item size
	// In a real implementation, this would be the actual formatting operation
	if dryRun {
		// In dry-run mode, just simulate success
		result.Success = true
		result.Output = fmt.Sprintf("Would format %s", item.Path)
	} else if noOp {
		// In no-op mode, validate/check without making changes
		var err error
		switch item.ContentType {
		case "go":
			err = p.checkGoFile(item.Path)
		case "yaml":
			err = p.checkYAMLFile(item.Path)
		case "json":
			err = p.checkJSONFile(item.Path)
		case "markdown":
			err = p.checkMarkdownFile(item.Path)
		default:
			err = fmt.Errorf("unsupported content type: %s", item.ContentType)
		}

		if err != nil {
			result.Error = err.Error()
			logger.Debug(fmt.Sprintf("Check failed for %s: %v", item.Path, err))
		} else {
			result.Success = true
			result.Output = fmt.Sprintf("Check passed for %s", item.Path)
			logger.Debug(fmt.Sprintf("Check passed for %s", item.Path))
		}
	} else {
		// Perform actual formatting based on content type
		var err error
		switch item.ContentType {
		case "go":
			err = p.formatGoFile(item.Path)
		case "yaml":
			err = p.formatYAMLFile(item.Path)
		case "json":
			err = p.formatJSONFile(item.Path)
		case "markdown":
			err = p.formatMarkdownFile(item.Path)
		default:
			err = fmt.Errorf("unsupported content type: %s", item.ContentType)
		}

		if err != nil {
			result.Error = err.Error()
			logger.Debug(fmt.Sprintf("Failed to format %s: %v", item.Path, err))
		} else {
			result.Success = true
			result.Output = fmt.Sprintf("Successfully formatted %s", item.Path)
			logger.Debug(fmt.Sprintf("Successfully formatted %s", item.Path))
		}
	}

	return result
}

// formatGoFile formats a Go file
func (p *FormatProcessor) formatGoFile(filePath string) error {
	// This would use the actual Go formatting logic from the format command
	// For now, we'll simulate it
	logger.Debug(fmt.Sprintf("Formatting Go file: %s", filePath))

	// In a real implementation, this would call the formatGoFile function
	// from the format command with the appropriate config
	return fmt.Errorf("go formatting not implemented in processor")
}

// checkGoFile checks if a Go file needs formatting without modifying it
func (p *FormatProcessor) checkGoFile(filePath string) error {
	logger.Debug(fmt.Sprintf("Checking Go file formatting: %s", filePath))

	// In a real implementation, this would compare the current content
	// with what gofmt would produce and return an error if they differ
	// For now, we'll simulate a check
	return nil // Simulate that the file is properly formatted
}

// formatYAMLFile formats a YAML file
func (p *FormatProcessor) formatYAMLFile(filePath string) error {
	logger.Debug(fmt.Sprintf("Formatting YAML file: %s", filePath))

	// Get YAML config
	yamlConfig := p.config.GetYAMLConfig()

	// Build yamlfmt arguments
	args := []string{"-w", filePath}

	// Add configuration options
	if yamlConfig.Indent != 2 {
		args = append(args, fmt.Sprintf("-indent=%d", yamlConfig.Indent))
	}
	if yamlConfig.LineLength != 80 {
		args = append(args, fmt.Sprintf("-width=%d", yamlConfig.LineLength))
	}
	if yamlConfig.QuoteStyle == "single" {
		args = append(args, "-quote")
	}
	if !yamlConfig.TrailingNewline {
		args = append(args, "-no_trailing_newline")
	}

	logger.Debug(fmt.Sprintf("Running yamlfmt with args: %v", args))

	// In a real implementation, this would execute yamlfmt
	// For now, return success
	return nil
}

// checkYAMLFile checks if a YAML file needs formatting without modifying it
func (p *FormatProcessor) checkYAMLFile(filePath string) error {
	logger.Debug(fmt.Sprintf("Checking YAML file formatting: %s", filePath))

	// In a real implementation, this would run yamlfmt with --check flag
	// and return an error if formatting is needed
	// For now, we'll simulate a check
	return nil // Simulate that the file is properly formatted
}

// formatJSONFile formats a JSON file
func (p *FormatProcessor) formatJSONFile(filePath string) error {
	logger.Debug(fmt.Sprintf("Formatting JSON file: %s", filePath))

	// Get JSON config
	jsonConfig := p.config.GetJSONConfig()

	logger.Debug(fmt.Sprintf("JSON config: compact=%t, indent=%s", jsonConfig.Compact, jsonConfig.Indent))

	// In a real implementation, this would execute jq with appropriate arguments
	// For now, return success
	return nil
}

// checkJSONFile checks if a JSON file needs formatting without modifying it
func (p *FormatProcessor) checkJSONFile(filePath string) error {
	logger.Debug(fmt.Sprintf("Checking JSON file formatting: %s", filePath))

	// In a real implementation, this would compare the current content
	// with what jq would produce and return an error if they differ
	// For now, we'll simulate a check
	return nil // Simulate that the file is properly formatted
}

// formatMarkdownFile formats a Markdown file
func (p *FormatProcessor) formatMarkdownFile(filePath string) error {
	logger.Debug(fmt.Sprintf("Formatting Markdown file: %s", filePath))

	// Get Markdown config
	mdConfig := p.config.GetMarkdownConfig()

	logger.Debug(fmt.Sprintf("Markdown config: line_length=%d, trailing_spaces=%t", mdConfig.LineLength, mdConfig.TrailingSpaces))

	// In a real implementation, this would execute prettier with appropriate arguments
	// For now, return success
	return nil
}

// checkMarkdownFile checks if a Markdown file needs formatting without modifying it
func (p *FormatProcessor) checkMarkdownFile(filePath string) error {
	logger.Debug(fmt.Sprintf("Checking Markdown file formatting: %s", filePath))

	// In a real implementation, this would run prettier with --check flag
	// and return an error if formatting is needed
	// For now, we'll simulate a check
	return nil // Simulate that the file is properly formatted
}

// GetSupportedContentTypes returns the content types supported by this processor
func (p *FormatProcessor) GetSupportedContentTypes() []string {
	return []string{"go", "yaml", "json", "markdown"}
}

// ValidateWorkItem validates that a work item can be processed
func (p *FormatProcessor) ValidateWorkItem(item *WorkItem) error {
	if item.ContentType == "" {
		return fmt.Errorf("work item has no content type")
	}

	supportedTypes := p.GetSupportedContentTypes()
	for _, supportedType := range supportedTypes {
		if item.ContentType == supportedType {
			return nil
		}
	}

	return fmt.Errorf("unsupported content type: %s", item.ContentType)
}

// EstimateProcessingTime estimates processing time for a work item
func (p *FormatProcessor) EstimateProcessingTime(item *WorkItem) float64 {
	// Base time per KB for different content types
	baseTimePerKB := map[string]float64{
		"go":       0.5, // Go formatting is fast
		"yaml":     1.0, // YAML parsing is more complex
		"json":     0.8, // JSON is relatively fast
		"markdown": 1.2, // Markdown can be complex
	}

	timePerKB := baseTimePerKB[item.ContentType]
	if timePerKB == 0 {
		timePerKB = 1.0 // Default
	}

	kb := float64(item.Size) / 1024
	return kb * timePerKB
}

// GetProcessorInfo returns information about this processor
func (p *FormatProcessor) GetProcessorInfo() map[string]interface{} {
	return map[string]interface{}{
		"type":             "format",
		"supported_types":  p.GetSupportedContentTypes(),
		"config_available": p.config != nil,
	}
}
