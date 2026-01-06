package work

import (
	"bytes"
	"context"
	"fmt"
	"go/format"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/fulmenhq/goneat/pkg/config"
	formatpkg "github.com/fulmenhq/goneat/pkg/format"
	"github.com/fulmenhq/goneat/pkg/format/finalizer"
	"github.com/fulmenhq/goneat/pkg/logger"
)

// FormatProcessor implements WorkItemProcessor for formatting operations
type FormatProcessor struct {
	config             *config.Config
	finalizerOptions   finalizer.NormalizationOptions
	ignoreMissingTools bool
	jsonIndent         string
	jsonIndentCount    int
	jsonSizeWarningMB  int
	toolPaths          FormatProcessorToolPaths
}

// FormatProcessorToolPaths allows callers to supply tool paths discovered externally.
type FormatProcessorToolPaths struct {
	Yamlfmt  string
	Prettier string
}

// FormatProcessorOptions configures optional processor behavior to align with CLI flags.
type FormatProcessorOptions struct {
	FinalizerOptions   finalizer.NormalizationOptions
	IgnoreMissingTools bool
	JSONIndent         string
	JSONIndentCount    int
	JSONSizeWarningMB  int
	ToolPaths          FormatProcessorToolPaths
}

// NewFormatProcessor creates a new format processor
func NewFormatProcessor(cfg *config.Config) *FormatProcessor {
	return NewFormatProcessorWithOptions(cfg, FormatProcessorOptions{
		FinalizerOptions: finalizer.NormalizationOptions{
			EnsureEOF:                  true,
			TrimTrailingWhitespace:     true,
			NormalizeLineEndings:       "",
			RemoveUTF8BOM:              false,
			PreserveMarkdownHardBreaks: true,
			EncodingPolicy:             "utf8-only",
		},
		JSONIndent:        "  ",
		JSONIndentCount:   2,
		JSONSizeWarningMB: 500,
	})
}

// NewFormatProcessorWithOptions creates a new format processor with explicit options.
func NewFormatProcessorWithOptions(cfg *config.Config, opts FormatProcessorOptions) *FormatProcessor {
	return &FormatProcessor{
		config:             cfg,
		finalizerOptions:   opts.FinalizerOptions,
		ignoreMissingTools: opts.IgnoreMissingTools,
		jsonIndent:         opts.JSONIndent,
		jsonIndentCount:    opts.JSONIndentCount,
		jsonSizeWarningMB:  opts.JSONSizeWarningMB,
		toolPaths:          opts.ToolPaths,
	}
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
			supportedTypes := p.GetSupportedContentTypes()
			err = fmt.Errorf("unsupported content type '%s' for file %s. Supported types: %v. Use --types flag to filter specific types or check file extension", item.ContentType, item.Path, supportedTypes)
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
			supportedTypes := p.GetSupportedContentTypes()
			err = fmt.Errorf("unsupported content type '%s' for file %s. Supported types: %v. Use --types flag to filter specific types or check file extension", item.ContentType, item.Path, supportedTypes)
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

// formatGoFile formats a Go file using go/format
func (p *FormatProcessor) formatGoFile(filePath string) error {
	logger.Debug(fmt.Sprintf("Formatting Go file: %s", filePath))

	// Validate and normalize file path
	filePath = filepath.Clean(filePath)
	if !filepath.IsAbs(filePath) {
		abs, err := filepath.Abs(filePath)
		if err != nil {
			return fmt.Errorf("failed to resolve absolute path for %s: %w", filePath, err)
		}
		filePath = abs
	}

	// Read the file content
	original, err := os.ReadFile(filePath) // #nosec G304 -- repo file read after Clean+Abs normalization
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	// Apply gofmt formatting
	formatted, err := format.Source(original)
	if err != nil {
		return fmt.Errorf("gofmt failed for %s: %w", filePath, err)
	}

	// Apply finalizer normalization (EOF, trailing whitespace, etc.)
	finalContent := formatted
	finalizerChanged := false
	if p.finalizerEnabled() {
		var err error
		finalContent, finalizerChanged, err = finalizer.ComprehensiveFileNormalization(formatted, p.finalizerOptions)
		if err != nil {
			return fmt.Errorf("finalizer error for %s: %w", filePath, err)
		}
	}

	// Check if content changed
	if bytes.Equal(original, finalContent) {
		logger.Debug(fmt.Sprintf("No formatting changes needed for %s", filePath))
		return nil
	}

	// Write back the formatted content
	if err := os.WriteFile(filePath, finalContent, 0600); err != nil {
		return fmt.Errorf("failed to write formatted content to %s: %w", filePath, err)
	}

	if finalizerChanged {
		logger.Debug(fmt.Sprintf("Applied Go formatting + finalizer to %s", filePath))
	} else {
		logger.Debug(fmt.Sprintf("Applied Go formatting to %s", filePath))
	}

	return nil
}

// checkGoFile checks if a Go file needs formatting without modifying it
func (p *FormatProcessor) checkGoFile(filePath string) error {
	logger.Debug(fmt.Sprintf("Checking Go file formatting: %s", filePath))

	// Validate and normalize file path
	filePath = filepath.Clean(filePath)
	if !filepath.IsAbs(filePath) {
		abs, err := filepath.Abs(filePath)
		if err != nil {
			return fmt.Errorf("failed to resolve absolute path for %s: %w", filePath, err)
		}
		filePath = abs
	}

	// Read the file content
	original, err := os.ReadFile(filePath) // #nosec G304 -- repo file read after Clean+Abs normalization
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	// Apply gofmt formatting (without writing)
	formatted, err := format.Source(original)
	if err != nil {
		return fmt.Errorf("gofmt check failed for %s: %w", filePath, err)
	}

	finalContent := formatted
	if p.finalizerEnabled() {
		var err error
		finalContent, _, err = finalizer.ComprehensiveFileNormalization(formatted, p.finalizerOptions)
		if err != nil {
			return fmt.Errorf("finalizer check failed for %s: %w", filePath, err)
		}
	}

	// Compare original with what formatted would be
	if !bytes.Equal(original, finalContent) {
		return fmt.Errorf("file %s needs formatting", filePath)
	}

	logger.Debug(fmt.Sprintf("File %s is properly formatted", filePath))
	return nil
}

// formatYAMLFile formats a YAML file using yamlfmt + finalizer
func (p *FormatProcessor) formatYAMLFile(filePath string) error {
	logger.Debug(fmt.Sprintf("Formatting YAML file: %s", filePath))

	// Validate and normalize file path
	filePath = filepath.Clean(filePath)
	if !filepath.IsAbs(filePath) {
		abs, err := filepath.Abs(filePath)
		if err != nil {
			return fmt.Errorf("failed to resolve absolute path for %s: %w", filePath, err)
		}
		filePath = abs
	}

	// Check if yamlfmt is available
	yamlfmtPath := p.toolPath("yamlfmt")
	if yamlfmtPath == "" {
		if p.ignoreMissingTools {
			logger.Warn("yamlfmt not found; skipping YAML formatter and applying finalizer only")
			if !p.finalizerEnabled() {
				return nil
			}
			return p.formatYAMLFileFinalizerOnly(filePath)
		}
		return fmt.Errorf("yamlfmt not found. Install with: goneat doctor tools --install yamlfmt")
	}

	// Read original content for change detection
	originalContent, err := os.ReadFile(filePath) // #nosec G304 -- path already validated
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	// Get YAML config
	yamlConfig := p.config.GetYAMLConfig()

	// Build yamlfmt arguments with formatter options
	var args []string
	var formatterOpts []string
	if yamlConfig.Indent != 2 {
		formatterOpts = append(formatterOpts, fmt.Sprintf("indent=%d", yamlConfig.Indent))
	}
	if yamlConfig.LineLength != 80 {
		formatterOpts = append(formatterOpts, fmt.Sprintf("line_length=%d", yamlConfig.LineLength))
	}

	for _, opt := range formatterOpts {
		args = append(args, "-formatter", opt)
	}
	args = append(args, filePath)

	logger.Debug(fmt.Sprintf("Running yamlfmt with args: %v", args))

	// #nosec G204 -- yamlfmtPath comes from exec.LookPath which validates the path
	cmd := exec.Command(yamlfmtPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("yamlfmt failed for %s: %v\nOutput: %s", filePath, err, string(output))
	}

	// Apply finalizer normalization after yamlfmt
	if p.finalizerEnabled() {
		currentContent, err := os.ReadFile(filePath) // #nosec G304 -- path already validated
		if err != nil {
			return fmt.Errorf("failed to re-read file after yamlfmt: %w", err)
		}

		finalContent, finalizerChanged, err := finalizer.ComprehensiveFileNormalization(currentContent, p.finalizerOptions)
		if err != nil {
			return fmt.Errorf("finalizer error for %s: %w", filePath, err)
		}

		if finalizerChanged {
			if err := os.WriteFile(filePath, finalContent, 0600); err != nil {
				return fmt.Errorf("failed to write finalized content to %s: %w", filePath, err)
			}
		}
	}

	// Check if overall content changed
	finalResult, err := os.ReadFile(filePath) // #nosec G304 -- path already validated
	if err != nil {
		return fmt.Errorf("failed to read file after formatting: %w", err)
	}
	if bytes.Equal(originalContent, finalResult) {
		logger.Debug(fmt.Sprintf("No formatting changes needed for %s", filePath))
	} else {
		logger.Debug(fmt.Sprintf("Applied YAML formatting to %s", filePath))
	}

	return nil
}

// formatYAMLFileFinalizerOnly applies only finalizer normalization when yamlfmt is unavailable
func (p *FormatProcessor) formatYAMLFileFinalizerOnly(filePath string) error {
	if !p.finalizerEnabled() {
		return nil
	}
	content, err := os.ReadFile(filePath) // #nosec G304 -- path already validated by caller
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	finalContent, changed, err := finalizer.ComprehensiveFileNormalization(content, p.finalizerOptions)
	if err != nil {
		return fmt.Errorf("finalizer error for %s: %w", filePath, err)
	}

	if changed {
		if err := os.WriteFile(filePath, finalContent, 0600); err != nil {
			return fmt.Errorf("failed to write finalized content to %s: %w", filePath, err)
		}
		logger.Debug(fmt.Sprintf("Applied finalizer normalization to %s", filePath))
	} else {
		logger.Debug(fmt.Sprintf("No formatting changes needed for %s", filePath))
	}

	return nil
}

// checkYAMLFile checks if a YAML file needs formatting without modifying it
func (p *FormatProcessor) checkYAMLFile(filePath string) error {
	logger.Debug(fmt.Sprintf("Checking YAML file formatting: %s", filePath))

	// Validate and normalize file path
	filePath = filepath.Clean(filePath)
	if !filepath.IsAbs(filePath) {
		abs, err := filepath.Abs(filePath)
		if err != nil {
			return fmt.Errorf("failed to resolve absolute path for %s: %w", filePath, err)
		}
		filePath = abs
	}

	// Check if yamlfmt is available
	yamlfmtPath := p.toolPath("yamlfmt")
	if yamlfmtPath == "" {
		if p.ignoreMissingTools {
			logger.Warn("yamlfmt not found; skipping YAML formatter and checking finalizer only")
			if !p.finalizerEnabled() {
				return nil
			}
			return p.checkYAMLFileFinalizerOnly(filePath)
		}
		return fmt.Errorf("yamlfmt not found. Install with: goneat doctor tools --install yamlfmt")
	}

	// Run yamlfmt with -lint flag to check if formatting is needed
	// #nosec G204 -- yamlfmtPath comes from exec.LookPath which validates the path
	cmd := exec.Command(yamlfmtPath, "-lint", filePath)
	output, err := cmd.CombinedOutput()

	if err != nil {
		// yamlfmt returns exit code 1 if formatting is needed
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return fmt.Errorf("file %s needs formatting", filePath)
		}
		// Real error
		return fmt.Errorf("yamlfmt lint failed for %s: %v\nOutput: %s", filePath, err, string(output))
	}

	// Also check finalizer issues (EOF, trailing whitespace, etc.)
	if p.finalizerEnabled() {
		return p.checkYAMLFileFinalizerOnly(filePath)
	}
	return nil
}

// checkYAMLFileFinalizerOnly checks YAML file for finalizer issues only
func (p *FormatProcessor) checkYAMLFileFinalizerOnly(filePath string) error {
	if !p.finalizerEnabled() {
		return nil
	}
	content, err := os.ReadFile(filePath) // #nosec G304 -- path already validated
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	_, changed, err := finalizer.ComprehensiveFileNormalization(content, p.finalizerOptions)
	if err != nil {
		return fmt.Errorf("finalizer check failed for %s: %w", filePath, err)
	}

	if changed {
		return fmt.Errorf("file %s needs formatting (EOF, trailing whitespace, or line ending issues)", filePath)
	}

	logger.Debug(fmt.Sprintf("File %s is properly formatted", filePath))
	return nil
}

// formatJSONFile formats a JSON file using PrettifyJSON + finalizer
func (p *FormatProcessor) formatJSONFile(filePath string) error {
	logger.Debug(fmt.Sprintf("Formatting JSON file: %s", filePath))

	// Validate and normalize file path
	filePath = filepath.Clean(filePath)
	if !filepath.IsAbs(filePath) {
		abs, err := filepath.Abs(filePath)
		if err != nil {
			return fmt.Errorf("failed to resolve absolute path for %s: %w", filePath, err)
		}
		filePath = abs
	}

	// Read original content
	originalContent, err := os.ReadFile(filePath) // #nosec G304 -- path already validated
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	// Get JSON config
	jsonConfig := p.config.GetJSONConfig()

	indent, err := p.computeJSONIndent()
	if err != nil {
		return err
	}

	var formatted []byte
	if jsonConfig.Compact {
		formatted, _, err = formatpkg.PrettifyJSON(originalContent, "", p.jsonSizeWarningMB)
	} else if indent == "" {
		formatted = originalContent
	} else {
		formatted, _, err = formatpkg.PrettifyJSON(originalContent, indent, p.jsonSizeWarningMB)
	}
	if err != nil {
		return fmt.Errorf("JSON prettification failed for %s: %w", filePath, err)
	}

	formatted = p.applyJSONTrailingNewline(formatted, jsonConfig.TrailingNewline)

	finalContent := formatted
	if p.finalizerEnabled() {
		finalContent, _, err = finalizer.ComprehensiveFileNormalization(formatted, p.finalizerOptions)
		if err != nil {
			return fmt.Errorf("finalizer error for %s: %w", filePath, err)
		}
	}

	// Check if content changed
	if bytes.Equal(originalContent, finalContent) {
		logger.Debug(fmt.Sprintf("No formatting changes needed for %s", filePath))
		return nil
	}

	// Write back the formatted content
	if err := os.WriteFile(filePath, finalContent, 0600); err != nil {
		return fmt.Errorf("failed to write formatted content to %s: %w", filePath, err)
	}

	logger.Debug(fmt.Sprintf("Applied JSON formatting to %s", filePath))
	return nil
}

// checkJSONFile checks if a JSON file needs formatting without modifying it
func (p *FormatProcessor) checkJSONFile(filePath string) error {
	logger.Debug(fmt.Sprintf("Checking JSON file formatting: %s", filePath))

	// Validate and normalize file path
	filePath = filepath.Clean(filePath)
	if !filepath.IsAbs(filePath) {
		abs, err := filepath.Abs(filePath)
		if err != nil {
			return fmt.Errorf("failed to resolve absolute path for %s: %w", filePath, err)
		}
		filePath = abs
	}

	// Read original content
	original, err := os.ReadFile(filePath) // #nosec G304 -- path already validated
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	// Get JSON config
	jsonConfig := p.config.GetJSONConfig()

	indent, err := p.computeJSONIndent()
	if err != nil {
		return err
	}

	var formatted []byte
	if jsonConfig.Compact {
		formatted, _, err = formatpkg.PrettifyJSON(original, "", p.jsonSizeWarningMB)
	} else if indent == "" {
		formatted = original
	} else {
		formatted, _, err = formatpkg.PrettifyJSON(original, indent, p.jsonSizeWarningMB)
	}
	if err != nil {
		return fmt.Errorf("JSON prettification check failed for %s: %w", filePath, err)
	}

	formatted = p.applyJSONTrailingNewline(formatted, jsonConfig.TrailingNewline)

	finalContent := formatted
	if p.finalizerEnabled() {
		finalContent, _, err = finalizer.ComprehensiveFileNormalization(formatted, p.finalizerOptions)
		if err != nil {
			return fmt.Errorf("finalizer check failed for %s: %w", filePath, err)
		}
	}

	// Only compare final output to original - intermediate change flags are irrelevant
	// if the final result matches the original
	if !bytes.Equal(original, finalContent) {
		return fmt.Errorf("file %s needs formatting", filePath)
	}

	logger.Debug(fmt.Sprintf("File %s is properly formatted", filePath))
	return nil
}

// formatMarkdownFile formats a Markdown file using prettier + finalizer
func (p *FormatProcessor) formatMarkdownFile(filePath string) error {
	logger.Debug(fmt.Sprintf("Formatting Markdown file: %s", filePath))

	// Validate file path to prevent path traversal
	filePath = filepath.Clean(filePath)
	if !filepath.IsAbs(filePath) {
		abs, err := filepath.Abs(filePath)
		if err != nil {
			return fmt.Errorf("failed to resolve absolute path for %s: %w", filePath, err)
		}
		filePath = abs
	}

	// Read the original content
	originalContent, err := os.ReadFile(filePath) // #nosec G304 -- repo file read after Clean+Abs normalization
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	// Check if prettier is available
	prettierPath := p.toolPath("prettier")
	if prettierPath == "" {
		if p.ignoreMissingTools {
			logger.Warn("prettier not found; skipping Markdown formatter and applying finalizer only")
			if !p.finalizerEnabled() {
				return nil
			}
			return p.formatMarkdownFileFinalizerOnly(filePath, originalContent)
		}
		return fmt.Errorf("prettier not found. Install with: goneat doctor tools --install prettier")
	}

	// Get Markdown config
	mdConfig := p.config.GetMarkdownConfig()

	// Build prettier arguments
	args := []string{"--parser", "markdown"}

	// Add configuration options
	if mdConfig.LineLength > 0 {
		args = append(args, "--print-width", fmt.Sprintf("%d", mdConfig.LineLength))
	}

	// Handle reference style
	switch mdConfig.ReferenceStyle {
	case "collapsed":
		args = append(args, "--reference-style", "collapsed")
	case "full":
		args = append(args, "--reference-style", "full")
	case "shortcut":
		args = append(args, "--reference-style", "shortcut")
	}

	// Use stdin for input
	args = append(args, "--stdin-filepath", filePath)

	logger.Debug(fmt.Sprintf("Running prettier with args: %v", args))

	// #nosec G204 -- prettierPath comes from exec.LookPath which validates the path
	cmd := exec.Command(prettierPath, args...)
	cmd.Stdin = bytes.NewReader(originalContent)
	prettierOutput, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("prettier failed for %s: %w", filePath, err)
	}

	finalContent, err := p.normalizeMarkdownContent(prettierOutput, mdConfig)
	if err != nil {
		return fmt.Errorf("finalizer error for %s: %w", filePath, err)
	}

	// Check if content changed
	if bytes.Equal(originalContent, finalContent) {
		logger.Debug(fmt.Sprintf("No formatting changes needed for %s", filePath))
		return nil
	}

	// Write back the formatted content
	if err := os.WriteFile(filePath, finalContent, 0600); err != nil {
		return fmt.Errorf("failed to write formatted content to %s: %w", filePath, err)
	}

	logger.Debug(fmt.Sprintf("Applied Markdown formatting to %s", filePath))
	return nil
}

// formatMarkdownFileFinalizerOnly applies only finalizer normalization when prettier is unavailable
func (p *FormatProcessor) formatMarkdownFileFinalizerOnly(filePath string, content []byte) error {
	if !p.finalizerEnabled() {
		return nil
	}

	finalContent, changed, err := finalizer.ComprehensiveFileNormalization(content, p.finalizerOptions)
	if err != nil {
		return fmt.Errorf("finalizer error for %s: %w", filePath, err)
	}

	if changed {
		if err := os.WriteFile(filePath, finalContent, 0600); err != nil {
			return fmt.Errorf("failed to write finalized content to %s: %w", filePath, err)
		}
		logger.Debug(fmt.Sprintf("Applied finalizer normalization to %s", filePath))
	} else {
		logger.Debug(fmt.Sprintf("No formatting changes needed for %s", filePath))
	}

	return nil
}

// checkMarkdownFile checks if a Markdown file needs formatting without modifying it
func (p *FormatProcessor) checkMarkdownFile(filePath string) error {
	logger.Debug(fmt.Sprintf("Checking Markdown file formatting: %s", filePath))

	// Validate file path to prevent path traversal
	filePath = filepath.Clean(filePath)
	if !filepath.IsAbs(filePath) {
		abs, err := filepath.Abs(filePath)
		if err != nil {
			return fmt.Errorf("failed to resolve absolute path for %s: %w", filePath, err)
		}
		filePath = abs
	}

	// Read the original content
	originalContent, err := os.ReadFile(filePath) // #nosec G304 -- path already validated
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	// Check if prettier is available
	prettierPath := p.toolPath("prettier")
	if prettierPath == "" {
		if p.ignoreMissingTools {
			logger.Warn("prettier not found; skipping Markdown formatter and checking finalizer only")
			if !p.finalizerEnabled() {
				return nil
			}
			return p.checkMarkdownFileFinalizerOnly(filePath, originalContent)
		}
		return fmt.Errorf("prettier not found. Install with: goneat doctor tools --install prettier")
	}

	// Get Markdown config
	mdConfig := p.config.GetMarkdownConfig()

	// Build prettier arguments (same as format)
	args := []string{"--parser", "markdown"}
	if mdConfig.LineLength > 0 {
		args = append(args, "--print-width", fmt.Sprintf("%d", mdConfig.LineLength))
	}
	switch mdConfig.ReferenceStyle {
	case "collapsed":
		args = append(args, "--reference-style", "collapsed")
	case "full":
		args = append(args, "--reference-style", "full")
	case "shortcut":
		args = append(args, "--reference-style", "shortcut")
	}
	args = append(args, "--stdin-filepath", filePath)

	// #nosec G204 -- prettierPath comes from exec.LookPath which validates the path
	cmd := exec.Command(prettierPath, args...)
	cmd.Stdin = bytes.NewReader(originalContent)
	prettierOutput, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("prettier check failed for %s: %w", filePath, err)
	}

	finalContent, err := p.normalizeMarkdownContent(prettierOutput, mdConfig)
	if err != nil {
		return fmt.Errorf("finalizer check failed for %s: %w", filePath, err)
	}

	// Compare original with what formatted would be
	if !bytes.Equal(originalContent, finalContent) {
		return fmt.Errorf("file %s needs formatting", filePath)
	}

	logger.Debug(fmt.Sprintf("File %s is properly formatted", filePath))
	return nil
}

// checkMarkdownFileFinalizerOnly checks Markdown file for finalizer issues only
func (p *FormatProcessor) checkMarkdownFileFinalizerOnly(filePath string, content []byte) error {
	if !p.finalizerEnabled() {
		return nil
	}

	_, changed, err := finalizer.ComprehensiveFileNormalization(content, p.finalizerOptions)
	if err != nil {
		return fmt.Errorf("finalizer check failed for %s: %w", filePath, err)
	}

	if changed {
		return fmt.Errorf("file %s needs formatting (EOF, trailing whitespace, or line ending issues)", filePath)
	}

	logger.Debug(fmt.Sprintf("File %s is properly formatted", filePath))
	return nil
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

	return fmt.Errorf("unsupported content type '%s' for file %s. Supported types: %v. Use --types flag to filter specific types or check file extension", item.ContentType, item.Path, p.GetSupportedContentTypes())
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

func (p *FormatProcessor) finalizerEnabled() bool {
	return p.finalizerOptions.EnsureEOF ||
		p.finalizerOptions.TrimTrailingWhitespace ||
		p.finalizerOptions.NormalizeLineEndings != "" ||
		p.finalizerOptions.RemoveUTF8BOM
}

func (p *FormatProcessor) toolPath(name string) string {
	switch name {
	case "yamlfmt":
		if p.toolPaths.Yamlfmt != "" {
			return p.toolPaths.Yamlfmt
		}
	case "prettier":
		if p.toolPaths.Prettier != "" {
			return p.toolPaths.Prettier
		}
	}
	if path, err := exec.LookPath(name); err == nil {
		return path
	}
	return ""
}

func (p *FormatProcessor) computeJSONIndent() (string, error) {
	if p.jsonIndent != "  " && p.jsonIndentCount != 2 {
		return "", fmt.Errorf("cannot specify both --json-indent and --json-indent-count")
	}
	if p.jsonIndentCount < 0 || p.jsonIndentCount > 10 {
		return "", fmt.Errorf("--json-indent-count must be between 0 and 10")
	}
	if p.jsonIndentCount == 0 {
		return "", nil
	}
	if p.jsonIndentCount != 2 {
		return strings.Repeat(" ", p.jsonIndentCount), nil
	}
	return p.jsonIndent, nil
}

func (p *FormatProcessor) applyJSONTrailingNewline(content []byte, trailingNewline bool) []byte {
	if trailingNewline {
		if len(content) > 0 && content[len(content)-1] != '\n' {
			return append(content, '\n')
		}
		return content
	}
	return bytes.TrimSuffix(content, []byte("\n"))
}

func (p *FormatProcessor) normalizeMarkdownContent(input []byte, mdConfig config.MarkdownFormatConfig) ([]byte, error) {
	formatted := string(input)
	if p.finalizerOptions.TrimTrailingWhitespace {
		lines := strings.Split(formatted, "\n")
		for i, line := range lines {
			if p.finalizerOptions.PreserveMarkdownHardBreaks {
				n := 0
				for j := len(line) - 1; j >= 0; j-- {
					if line[j] == ' ' {
						n++
						continue
					}
					break
				}
				if n >= 2 {
					lines[i] = strings.TrimRight(line, " \t") + "  "
					continue
				}
			}
			lines[i] = strings.TrimRight(line, " \t")
		}
		formatted = strings.Join(lines, "\n")
	} else if mdConfig.TrailingSpaces {
		lines := strings.Split(formatted, "\n")
		for i, line := range lines {
			lines[i] = strings.TrimRight(line, " \t")
		}
		formatted = strings.Join(lines, "\n")
	}

	if p.finalizerEnabled() {
		finalizerOptions := p.finalizerOptions
		finalizerOptions.TrimTrailingWhitespace = false
		finalized, _, err := finalizer.ComprehensiveFileNormalization([]byte(formatted), finalizerOptions)
		if err != nil {
			return nil, err
		}
		return finalized, nil
	}

	return []byte(formatted), nil
}
