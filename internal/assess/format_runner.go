/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package assess

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/3leaps/goneat/pkg/logger"
)

// FormatAssessmentRunner implements AssessmentRunner for the format command
type FormatAssessmentRunner struct {
	commandName string
}

// NewFormatAssessmentRunner creates a new format assessment runner
func NewFormatAssessmentRunner() *FormatAssessmentRunner {
	return &FormatAssessmentRunner{
		commandName: "format",
	}
}

// Assess implements AssessmentRunner.Assess
func (r *FormatAssessmentRunner) Assess(ctx context.Context, target string, config AssessmentConfig) (*AssessmentResult, error) {
	startTime := time.Now()

	logger.Info(fmt.Sprintf("Running format assessment on %s", target))

	// Check if gofmt is available
	if !r.IsAvailable() {
		return &AssessmentResult{
			CommandName:   r.commandName,
			Category:      CategoryFormat,
			Success:       false,
			ExecutionTime: time.Since(startTime),
			Error:         "gofmt command not found in PATH",
		}, nil
	}

	// Find Go files to assess
	goFiles, err := r.findGoFiles(target, config)
	if err != nil {
		return &AssessmentResult{
			CommandName:   r.commandName,
			Category:      CategoryFormat,
			Success:       false,
			ExecutionTime: time.Since(startTime),
			Error:         fmt.Sprintf("failed to find Go files: %v", err),
		}, nil
	}

	if len(goFiles) == 0 {
		logger.Info("No Go files found for format assessment")
		return &AssessmentResult{
			CommandName:   r.commandName,
			Category:      CategoryFormat,
			Success:       true,
			ExecutionTime: time.Since(startTime),
			Issues:        []Issue{},
		}, nil
	}

	// Run gofmt to check formatting
	issues, err := r.checkFormatting(goFiles, config)
	if err != nil {
		return &AssessmentResult{
			CommandName:   r.commandName,
			Category:      CategoryFormat,
			Success:       false,
			ExecutionTime: time.Since(startTime),
			Error:         fmt.Sprintf("format check failed: %v", err),
		}, nil
	}

	logger.Info(fmt.Sprintf("Format assessment completed: %d issues found in %d files", len(issues), len(goFiles)))

	return &AssessmentResult{
		CommandName:   r.commandName,
		Category:      CategoryFormat,
		Success:       true,
		ExecutionTime: time.Since(startTime),
		Issues:        issues,
	}, nil
}

// CanRunInParallel implements AssessmentRunner.CanRunInParallel
func (r *FormatAssessmentRunner) CanRunInParallel() bool {
	return true // Format checks can run in parallel on different files
}

// GetCategory implements AssessmentRunner.GetCategory
func (r *FormatAssessmentRunner) GetCategory() AssessmentCategory {
	return CategoryFormat
}

// GetEstimatedTime implements AssessmentRunner.GetEstimatedTime
func (r *FormatAssessmentRunner) GetEstimatedTime(target string) time.Duration {
	// Estimate based on typical file counts and processing speed
	// Rough estimate: 100ms per file for format checking
	goFiles, _ := r.findGoFiles(target, DefaultAssessmentConfig())
	estimatedMs := len(goFiles) * 100
	if estimatedMs < 500 {
		estimatedMs = 500 // Minimum 500ms
	}
	return time.Duration(estimatedMs) * time.Millisecond
}

// IsAvailable implements AssessmentRunner.IsAvailable
func (r *FormatAssessmentRunner) IsAvailable() bool {
	_, err := exec.LookPath("gofmt")
	return err == nil
}

// findGoFiles finds all Go files in the target directory
func (r *FormatAssessmentRunner) findGoFiles(target string, config AssessmentConfig) ([]string, error) {
	var goFiles []string

	err := filepath.Walk(target, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			// Skip common directories we don't want to format
			dirName := filepath.Base(path)
			if dirName == ".git" || dirName == "vendor" || dirName == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if it's a Go file
		if strings.HasSuffix(path, ".go") {
			// Apply include/exclude filters
			if r.shouldIncludeFile(path, config) {
				goFiles = append(goFiles, path)
			}
		}

		return nil
	})

	return goFiles, err
}

// shouldIncludeFile checks if a file should be included based on configuration
func (r *FormatAssessmentRunner) shouldIncludeFile(filePath string, config AssessmentConfig) bool {
	// Check exclude patterns
	for _, exclude := range config.ExcludeFiles {
		if strings.Contains(filePath, exclude) {
			return false
		}
	}

	// If include patterns are specified, file must match at least one
	if len(config.IncludeFiles) > 0 {
		included := false
		for _, include := range config.IncludeFiles {
			if strings.Contains(filePath, include) {
				included = true
				break
			}
		}
		if !included {
			return false
		}
	}

	return true
}

// checkFormatting runs gofmt to check for formatting issues
func (r *FormatAssessmentRunner) checkFormatting(goFiles []string, config AssessmentConfig) ([]Issue, error) {
	var allIssues []Issue

	// Run gofmt with -l flag to list files that need formatting
	args := append([]string{"-l"}, goFiles...)
	cmd := exec.CommandContext(context.Background(), "gofmt", args...)

	output, err := cmd.Output()
	if err != nil {
		// gofmt returns non-zero exit code when files need formatting
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			// This is expected - files need formatting
			output = exitErr.Stderr
		} else {
			return nil, fmt.Errorf("gofmt command failed: %v", err)
		}
	}

	// Parse output - gofmt -l returns one file path per line
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Create an issue for each file that needs formatting
		issue := Issue{
			File:          line,
			Severity:      SeverityLow,
			Message:       "File needs formatting (run 'gofmt -w' to fix)",
			Category:      CategoryFormat,
			SubCategory:   "whitespace",
			AutoFixable:   true,
			EstimatedTime: 30 * time.Second, // Rough estimate for manual formatting
		}

		allIssues = append(allIssues, issue)
	}

	return allIssues, nil
}

// init registers the format assessment runner
func init() {
	RegisterAssessmentRunner(CategoryFormat, NewFormatAssessmentRunner())
}
