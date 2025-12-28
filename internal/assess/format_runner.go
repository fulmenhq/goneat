/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package assess

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/fulmenhq/goneat/pkg/format/finalizer"
	"github.com/fulmenhq/goneat/pkg/logger"
	"github.com/fulmenhq/goneat/pkg/work"
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

	var allIssues []Issue

	// Use unified file discovery (same as format command) with layered ignore support
	plannerConfig := work.PlannerConfig{
		Command:            "format",
		Paths:              []string{target},
		ExecutionStrategy:  "sequential",
		GroupBySize:        false,
		GroupByContentType: false,
		IgnoreFile:         ".goneatignore", // Match format command ignore behavior
	}
	planner := work.NewPlanner(plannerConfig)
	manifest, err := planner.GenerateManifest()
	if err != nil {
		return &AssessmentResult{
			CommandName:   r.commandName,
			Category:      CategoryFormat,
			Success:       false,
			ExecutionTime: HumanReadableDuration(time.Since(startTime)),
			Error:         fmt.Sprintf("failed to discover files: %v", err),
		}, nil
	}

	// Extract files from work manifest
	var supportedFiles []string
	for _, item := range manifest.WorkItems {
		logger.Debug(fmt.Sprintf("Adding file to supportedFiles: '%s' (ContentType: '%s', ID: '%s')", item.Path, item.ContentType, item.ID))
		supportedFiles = append(supportedFiles, item.Path)
	}

	// Apply IncludeFiles scoping if provided: restrict to matching files
	if len(config.IncludeFiles) > 0 {
		var scoped []string
		for _, f := range supportedFiles {
			if r.shouldIncludeFile(f, config) {
				scoped = append(scoped, f)
			}
		}
		supportedFiles = scoped
	}

	// Subset: Go files for gofmt-based structural formatting checks
	var goFiles []string
	for _, f := range supportedFiles {
		if strings.HasSuffix(f, ".go") {
			goFiles = append(goFiles, f)
		}
	}

	// Run gofmt -l when available (do not fail overall if missing)
	if _, lookErr := exec.LookPath("gofmt"); lookErr != nil {
		logger.Warn("gofmt not found; skipping Go structural format checks, proceeding with normalization checks")
	} else if len(goFiles) > 0 {
		gofmtIssues, fmtErr := r.checkFormatting(goFiles, config)
		if fmtErr != nil {
			// Treat tool failure as non-fatal; record as error in result while proceeding
			logger.Warn(fmt.Sprintf("gofmt check failed: %v", fmtErr))
		} else {
			allIssues = append(allIssues, gofmtIssues...)
		}
	}

	// Language-aware format tools (only when tool present)
	pyFiles := filterByExtensions(supportedFiles, []string{".py"})
	jsFiles := filterByExtensions(supportedFiles, []string{".js", ".jsx", ".ts", ".tsx"})

	ruffFmtIssues, ruffFmtErr := runRuffFormat(target, config, pyFiles)
	if ruffFmtErr != nil {
		return &AssessmentResult{
			CommandName:   r.commandName,
			Category:      CategoryFormat,
			Success:       false,
			ExecutionTime: HumanReadableDuration(time.Since(startTime)),
			Error:         fmt.Sprintf("ruff format failed: %v", ruffFmtErr),
		}, nil
	}
	allIssues = append(allIssues, ruffFmtIssues...)

	biomeFmtIssues, biomeFmtErr := runBiomeFormat(target, config, jsFiles)
	if biomeFmtErr != nil {
		return &AssessmentResult{
			CommandName:   r.commandName,
			Category:      CategoryFormat,
			Success:       false,
			ExecutionTime: HumanReadableDuration(time.Since(startTime)),
			Error:         fmt.Sprintf("biome format failed: %v", biomeFmtErr),
		}, nil
	}
	allIssues = append(allIssues, biomeFmtIssues...)

	// Normalization policy for assess: enforce LF, single EOF, trim trailing whitespace, remove BOM
	for _, filePath := range supportedFiles {
		// Validate file path to prevent path traversal
		cleanFilePath := filepath.Clean(filePath)
		if strings.Contains(cleanFilePath, "..") {
			logger.Warn(fmt.Sprintf("Skipping file with path traversal: %s", cleanFilePath))
			continue
		}
		content, readErr := os.ReadFile(cleanFilePath)
		if readErr != nil {
			logger.Warn(fmt.Sprintf("Failed to read %s: %v", cleanFilePath, readErr))
			continue
		}

		// Skip non-text/binary content conservatively
		if !finalizer.IsProcessableText(content) {
			continue
		}

		// BOM check
		if _, _, found := finalizer.GetBOMInfo(content); found {
			if _, changed, _ := finalizer.RemoveBOM(content); changed {
				allIssues = append(allIssues, Issue{
					File:          cleanFilePath,
					Severity:      SeverityMedium,
					Message:       "File begins with a Byte Order Mark (BOM)",
					Category:      CategoryFormat,
					SubCategory:   "bom",
					AutoFixable:   true,
					EstimatedTime: HumanReadableDuration(30 * time.Second),
				})
			}
		}

		// Line endings check (normalize to LF policy)
		if _, changed, _ := finalizer.NormalizeLineEndings(content, ""); changed {
			allIssues = append(allIssues, Issue{
				File:          cleanFilePath,
				Severity:      SeverityLow,
				Message:       "Inconsistent or CR/CRLF line endings (normalize to LF)",
				Category:      CategoryFormat,
				SubCategory:   "line-endings",
				AutoFixable:   true,
				EstimatedTime: HumanReadableDuration(20 * time.Second),
			})
		}

		// Use shared whitespace detection for consistency with format command
		options := finalizer.NormalizationOptions{
			TrimTrailingWhitespace:     true,  // Assessment always checks for trailing whitespace
			EnsureEOF:                  false, // EOF enforcement handled separately
			PreserveMarkdownHardBreaks: true,  // Preserve markdown hard breaks (2 trailing spaces)
		}

		if hasIssues, whitespaceIssues := finalizer.DetectWhitespaceIssues(content, options); hasIssues {
			for _, wsIssue := range whitespaceIssues {
				if wsIssue.Type == "trailing-whitespace" && len(wsIssue.LineNumbers) > 0 {
					allIssues = append(allIssues, Issue{
						File:          cleanFilePath,
						Line:          wsIssue.LineNumbers[0], // First affected line
						Severity:      SeverityLow,
						Message:       wsIssue.Description,
						Category:      CategoryFormat,
						SubCategory:   wsIssue.Type,
						AutoFixable:   true,
						EstimatedTime: HumanReadableDuration(15 * time.Second),
						LinesModified: wsIssue.LineNumbers,
						ChangeRelated: true,
					})
				}
			}
		}

		// EOF newline enforcement and multiple-EOF collapse (no trailing whitespace trimming here)
		if _, changed, _ := finalizer.NormalizeEOF(content, true, true, false, "", false); changed {
			logger.Debug(fmt.Sprintf("Creating EOF issue for file: '%s' (original: '%s')", cleanFilePath, filePath))

			// Determine line number for EOF issues
			lines := strings.Split(string(content), "\n")
			lineCount := len(lines)
			eofLine := lineCount
			if strings.HasSuffix(string(content), "\n\n") {
				eofLine = lineCount + 1 // Multiple newlines after last content line
			}

			allIssues = append(allIssues, Issue{
				File:          cleanFilePath,
				Line:          eofLine,
				Severity:      SeverityLow,
				Message:       "Missing or multiple trailing newlines at EOF (enforce single newline)",
				Category:      CategoryFormat,
				SubCategory:   "eof",
				AutoFixable:   true,
				EstimatedTime: HumanReadableDuration(10 * time.Second),
				ChangeRelated: true,
			})
		}
	}

	logger.Info(fmt.Sprintf("Format assessment completed: %d issues found across %d files", len(allIssues), len(supportedFiles)))

	return &AssessmentResult{
		CommandName:   r.commandName,
		Category:      CategoryFormat,
		Success:       true,
		ExecutionTime: HumanReadableDuration(time.Since(startTime)),
		Issues:        allIssues,
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
	// Estimate based on typical file counts and processing speed (normalization + gofmt)
	// Rough estimate: 100ms per file
	files, _ := r.findSupportedFiles(target, DefaultAssessmentConfig())
	estimatedMs := len(files) * 100
	if estimatedMs < 500 {
		estimatedMs = 500 // Minimum 500ms
	}
	return time.Duration(estimatedMs) * time.Millisecond
}

// IsAvailable implements AssessmentRunner.IsAvailable
func (r *FormatAssessmentRunner) IsAvailable() bool {
	// Normalization checks do not require external tools; category is available even if gofmt is missing.
	return true
}

// findSupportedFiles finds files supported by the finalizer (normalization) operations
func (r *FormatAssessmentRunner) findSupportedFiles(target string, config AssessmentConfig) ([]string, error) {
	var files []string

	err := filepath.Walk(target, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			dirName := filepath.Base(path)
			if dirName == ".git" || dirName == "vendor" || dirName == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}

		// Extension-based filter
		ext := strings.ToLower(filepath.Ext(path))
		if finalizer.IsSupportedExtension(ext) {
			// Apply include/exclude filters and ignore files
			if r.shouldIncludeFile(path, config) {
				if !r.matchesGoneatIgnore(path) {
					files = append(files, path)
				}
			}
		}

		return nil
	})

	return files, err
}

// shouldIncludeFile checks if a file should be included based on configuration
func (r *FormatAssessmentRunner) shouldIncludeFile(filePath string, config AssessmentConfig) bool {
	// Check .goneatignore patterns
	if r.matchesGoneatIgnore(filePath) {
		return false
	}

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

// matchesGoneatIgnore checks if a file path matches .goneatignore patterns
func (r *FormatAssessmentRunner) matchesGoneatIgnore(filePath string) bool {
	// Check repo-level .goneatignore
	if r.matchesIgnoreFile(filePath, ".goneatignore") {
		return true
	}

	// Check user-level .goneatignore
	if homeDir, err := os.UserHomeDir(); err == nil {
		userIgnorePath := filepath.Join(homeDir, ".goneatignore")
		if r.matchesIgnoreFile(filePath, userIgnorePath) {
			return true
		}
	}

	return false
}

// matchesIgnoreFile checks if a path matches patterns in an ignore file
func (r *FormatAssessmentRunner) matchesIgnoreFile(filePath, ignoreFilePath string) bool {
	// Validate ignore file path to prevent path traversal
	ignoreFilePath = filepath.Clean(ignoreFilePath)
	if strings.Contains(ignoreFilePath, "..") {
		return false
	}
	file, err := os.Open(ignoreFilePath)
	if err != nil {
		return false // File doesn't exist, no patterns to match
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			logger.Warn(fmt.Sprintf("Failed to close ignore file %s: %v", ignoreFilePath, closeErr))
		}
	}()

	var patterns []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}

	// Get relative path for pattern matching
	wd, err := os.Getwd()
	if err != nil {
		return false
	}

	relPath, err := filepath.Rel(wd, filePath)
	if err != nil {
		relPath = filePath
	}

	for _, pattern := range patterns {
		if r.matchesIgnorePattern(pattern, relPath) {
			return true
		}
	}

	return false
}

// matchesIgnorePattern performs gitignore-style pattern matching
func (r *FormatAssessmentRunner) matchesIgnorePattern(pattern, path string) bool {
	// Handle negation patterns (starting with !)
	if strings.HasPrefix(pattern, "!") {
		negatedPattern := strings.TrimPrefix(pattern, "!")
		return !r.matchesSimplePattern(negatedPattern, path)
	}

	return r.matchesSimplePattern(pattern, path)
}

// matchesSimplePattern performs basic pattern matching
func (r *FormatAssessmentRunner) matchesSimplePattern(pattern, path string) bool {
	// Handle glob patterns with *
	if strings.Contains(pattern, "*") {
		// Simple glob matching for patterns like *.log, test.*
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return true
		}
		// Also check full path
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}
	}

	// Handle directory patterns
	if strings.Contains(path, pattern) {
		return true
	}

	// Handle exact matches
	if filepath.Base(path) == pattern {
		return true
	}

	return false
}

// checkFormatting runs gofmt to check for formatting issues
func (r *FormatAssessmentRunner) checkFormatting(goFiles []string, config AssessmentConfig) ([]Issue, error) {
	var allIssues []Issue

	// Clean file paths to prevent path traversal
	for i, file := range goFiles {
		goFiles[i] = filepath.Clean(file)
	}

	// Run gofmt with -l flag to list files that need formatting
	args := append([]string{"-l"}, goFiles...)
	cmd := exec.CommandContext(context.Background(), "gofmt", args...) // #nosec G204

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
			Message:       "File needs formatting (run 'goneat format <file>' to fix)",
			Category:      CategoryFormat,
			SubCategory:   "whitespace",
			AutoFixable:   true,
			EstimatedTime: HumanReadableDuration(30 * time.Second), // Rough estimate for manual formatting
		}

		allIssues = append(allIssues, issue)
	}

	return allIssues, nil
}

// init registers the format assessment runner
func init() {
	RegisterAssessmentRunner(CategoryFormat, NewFormatAssessmentRunner())
}
