/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package assess

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fulmenhq/goneat/internal/dates"
)

// DatesAssessmentRunner implements AssessmentRunner for dates validation
type DatesAssessmentRunner struct {
	runner *dates.DatesRunner
}

// NewDatesAssessmentRunner creates a new dates assessment runner
func NewDatesAssessmentRunner() *DatesAssessmentRunner {
	return &DatesAssessmentRunner{
		runner: dates.NewDatesRunner(),
	}
}

// NewDatesAssessmentRunnerWithConfig creates a new dates assessment runner with custom config
func NewDatesAssessmentRunnerWithConfig(config dates.DatesConfig) *DatesAssessmentRunner {
	return &DatesAssessmentRunner{
		runner: dates.NewDatesRunnerWithConfig(config),
	}
}

// Assess implements AssessmentRunner.Assess
func (r *DatesAssessmentRunner) Assess(ctx context.Context, target string, config AssessmentConfig) (*AssessmentResult, error) {
	// Load dates configuration for this assessment
	datesConfig := dates.LoadDatesConfig(target)

	// Create a runner with the loaded configuration
	configRunner := dates.NewDatesRunnerWithConfig(datesConfig)

	// If the caller explicitly provided files (via assessment IncludeFiles), filter them and process directly.
	// Otherwise, delegate discovery and analysis to the internal dates runner (respects dates config include/exclude).
	explicitInclude := len(config.IncludeFiles) > 0
	var filteredIncludeFiles []string
	if explicitInclude {
		filteredIncludeFiles = r.filterFilesRespectingIgnores(config.IncludeFiles, target)
		return r.assessSpecificFiles(ctx, target, datesConfig, filteredIncludeFiles)
	}
	// Delegate to internal dates runner for full discovery + analysis
	dResult, err := configRunner.Assess(ctx, target, nil)
	if err != nil {
		return &AssessmentResult{
			CommandName:   "dates",
			Category:      CategoryDates,
			Success:       false,
			ExecutionTime: HumanReadableDuration(0),
			Error:         err.Error(),
		}, nil
	}

	// Convert to AssessmentResult
	issues := make([]Issue, len(dResult.Issues))
	for i, di := range dResult.Issues {
		issues[i] = Issue{
			File:        di.File,
			Line:        di.Line,
			Column:      di.Column,
			Severity:    IssueSeverity(di.Severity),
			Message:     di.Message,
			Category:    CategoryDates,
			AutoFixable: di.AutoFixable,
		}
	}

	dur, _ := time.ParseDuration(dResult.ExecutionTime)

	return &AssessmentResult{
		CommandName:   "dates",
		Category:      CategoryDates,
		Success:       dResult.Success,
		ExecutionTime: HumanReadableDuration(dur),
		Issues:        issues,
		Metrics:       dResult.Metrics,
	}, nil
}

// CanRunInParallel implements AssessmentRunner.CanRunInParallel
func (r *DatesAssessmentRunner) CanRunInParallel() bool {
	return true
}

// GetCategory implements AssessmentRunner.GetCategory
func (r *DatesAssessmentRunner) GetCategory() AssessmentCategory {
	return CategoryDates
}

// GetEstimatedTime implements AssessmentRunner.GetEstimatedTime
func (r *DatesAssessmentRunner) GetEstimatedTime(target string) time.Duration {
	return 5 * time.Second
}

// IsAvailable implements AssessmentRunner.IsAvailable
func (r *DatesAssessmentRunner) IsAvailable() bool {
	return true
}

// filterFilesRespectingIgnores filters files to respect .goneatignore patterns
func (r *DatesAssessmentRunner) filterFilesRespectingIgnores(files []string, target string) []string {
	if len(files) == 0 {
		return files
	}

	var filtered []string
	for _, file := range files {
		if !r.matchesGoneatIgnore(file, target) {
			filtered = append(filtered, file)
		}
	}
	return filtered
}

// matchesGoneatIgnore checks if a file path matches .goneatignore patterns
func (r *DatesAssessmentRunner) matchesGoneatIgnore(filePath, target string) bool {
	// Check if we're processing test fixture files
	if strings.Contains(filePath, "fixtures/") {
		// This is a test fixture - it should be ignored
		return true
	}

	// Check repo-level .goneatignore
	repoIgnorePath := filepath.Join(target, ".goneatignore")
	if r.matchesIgnoreFile(filePath, repoIgnorePath) {
		return true
	}

	// Check user-level .goneatignore
	if homeDir, err := os.UserHomeDir(); err == nil {
		userIgnorePath := filepath.Join(homeDir, ".goneatignore")
		if r.matchesIgnoreFile(filePath, userIgnorePath) {
			return true
		}
		// Also check ~/.goneat/.goneatignore
		userGoneatIgnorePath := filepath.Join(homeDir, ".goneat", ".goneatignore")
		if r.matchesIgnoreFile(filePath, userGoneatIgnorePath) {
			return true
		}
	}

	return false
}

// matchesIgnoreFile checks if a path matches patterns in an ignore file
func (r *DatesAssessmentRunner) matchesIgnoreFile(filePath, ignoreFilePath string) bool {
	// #nosec G304 -- ignoreFilePath is constructed from controlled paths (target + ".goneatignore", etc.)
	file, err := os.Open(ignoreFilePath)
	if err != nil {
		return false // Ignore file doesn't exist, no matches
	}
	defer func() {
		_ = file.Close() // Ignore close errors for ignore file reading
	}()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Check for exact match or directory pattern match
		if filePath == line {
			return true
		}

		// Handle directory patterns (ending with /)
		if strings.HasSuffix(line, "/") {
			dirPattern := strings.TrimSuffix(line, "/")
			if strings.Contains(filePath, "/"+dirPattern+"/") || strings.HasPrefix(filePath, dirPattern+"/") {
				return true
			}
		}

		// Handle general substring matches
		if strings.Contains(filePath, line) {
			return true
		}

		// Simple glob matching - check if the pattern matches the file path
		if r.matchesGlobPattern(filePath, line) {
			return true
		}
	}

	return false
}

// matchesGlobPattern performs simple glob pattern matching
func (r *DatesAssessmentRunner) matchesGlobPattern(filePath, pattern string) bool {
	// Simple implementation - check for common patterns
	if strings.HasSuffix(pattern, "/") {
		// Directory pattern - check if filePath starts with the directory
		dirPattern := strings.TrimSuffix(pattern, "/")
		return strings.HasPrefix(filePath, dirPattern+"/") || filePath == dirPattern
	}

	if strings.Contains(pattern, "*") {
		// Wildcard pattern - simple implementation
		if pattern == "*" {
			return true
		}
		if strings.HasPrefix(pattern, "**/") {
			// Recursive pattern
			suffix := strings.TrimPrefix(pattern, "**/")
			return strings.Contains(filePath, "/"+suffix) || strings.HasSuffix(filePath, suffix)
		}
	}

	return false
}

// assessSpecificFiles processes a specific list of files directly, bypassing the internal dates runner
func (r *DatesAssessmentRunner) assessSpecificFiles(ctx context.Context, target string, config dates.DatesConfig, files []string) (*AssessmentResult, error) {
	start := time.Now()
	var issues []Issue

	for _, file := range files {
		select {
		case <-ctx.Done():
			return &AssessmentResult{
				CommandName:   "dates",
				Category:      CategoryDates,
				Success:       false,
				ExecutionTime: HumanReadableDuration(time.Since(start)),
				Error:         "context cancelled",
			}, nil
		default:
		}

		// Process each file directly
		fileIssues, err := r.processFile(ctx, target, config, file)
		if err != nil {
			// Log error but continue processing other files
			fmt.Printf("DEBUG: Error processing file %s: %v\n", file, err)
			continue
		}
		issues = append(issues, fileIssues...)
	}

	return &AssessmentResult{
		CommandName:   "dates",
		Category:      CategoryDates,
		Success:       true,
		ExecutionTime: HumanReadableDuration(time.Since(start)),
		Issues:        issues,
		Metrics:       map[string]interface{}{"files_processed": len(files)},
	}, nil
}

// processFile processes a single file for date validation
func (r *DatesAssessmentRunner) processFile(ctx context.Context, target string, config dates.DatesConfig, file string) ([]Issue, error) {
	var issues []Issue

	// Determine the full path to read
	var fullPath string
	if filepath.IsAbs(file) {
		// file is already an absolute path
		fullPath = file
	} else {
		// Check if target is a single file
		if stat, err := os.Stat(target); err == nil && !stat.IsDir() {
			// target is a single file, use it directly
			fullPath = target
		} else {
			// target is a directory, join with file
			fullPath = filepath.Join(target, file)
		}
	}

	// Verify the file exists (we don't need to read it here since the dates runner will do that)
	if _, err := os.Stat(fullPath); err != nil {
		return nil, fmt.Errorf("failed to access file %s: %w", fullPath, err)
	}

	// Use the enhanced dates runner for sophisticated analysis
	tempRunner := dates.NewDatesRunnerWithConfig(config)
	dResult, err := tempRunner.Assess(ctx, target, nil)
	if err != nil {
		return issues, err
	}

	// Convert DatesIssue to Issue
	for _, di := range dResult.Issues {
		issues = append(issues, Issue{
			File:        di.File,
			Line:        di.Line,
			Column:      di.Column,
			Severity:    IssueSeverity(di.Severity),
			Message:     di.Message,
			Category:    CategoryDates,
			AutoFixable: di.AutoFixable,
		})
	}

	return issues, nil
}

func init() {
	RegisterAssessmentRunner(CategoryDates, NewDatesAssessmentRunner())
}
