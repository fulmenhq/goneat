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
	"regexp"
	"strings"
	"time"

	"github.com/fulmenhq/goneat/pkg/logger"
)

// StaticAnalysisAssessmentRunner implements AssessmentRunner for static analysis tools like go vet
type StaticAnalysisAssessmentRunner struct {
	commandName string
	toolName    string
}

// NewStaticAnalysisAssessmentRunner creates a new static analysis assessment runner
func NewStaticAnalysisAssessmentRunner() *StaticAnalysisAssessmentRunner {
	return &StaticAnalysisAssessmentRunner{
		commandName: "static-analysis",
		toolName:    "go vet",
	}
}

// Assess implements AssessmentRunner.Assess
func (r *StaticAnalysisAssessmentRunner) Assess(ctx context.Context, target string, config AssessmentConfig) (*AssessmentResult, error) {
	startTime := time.Now()

	logger.Info(fmt.Sprintf("Running %s assessment on %s", r.toolName, target))

	// Check if go vet is available
	if !r.IsAvailable() {
		return &AssessmentResult{
			CommandName:   r.commandName,
			Category:      CategoryStaticAnalysis,
			Success:       false,
			ExecutionTime: time.Since(startTime),
			Error:         fmt.Sprintf("%s command not found in PATH", r.toolName),
		}, nil
	}

	// Find Go files to assess
	goFiles, err := r.findGoFiles(target, config)
	if err != nil {
		return &AssessmentResult{
			CommandName:   r.commandName,
			Category:      CategoryStaticAnalysis,
			Success:       false,
			ExecutionTime: time.Since(startTime),
			Error:         fmt.Sprintf("failed to find Go files: %v", err),
		}, nil
	}

	if len(goFiles) == 0 {
		logger.Info("No Go files found for static analysis assessment")
		return &AssessmentResult{
			CommandName:   r.commandName,
			Category:      CategoryStaticAnalysis,
			Success:       true,
			ExecutionTime: time.Since(startTime),
			Issues:        []Issue{},
		}, nil
	}

	// Run go vet to check for static analysis issues
	issues, err := r.checkStaticAnalysis(goFiles, config)
	if err != nil {
		return &AssessmentResult{
			CommandName:   r.commandName,
			Category:      CategoryStaticAnalysis,
			Success:       false,
			ExecutionTime: time.Since(startTime),
			Error:         fmt.Sprintf("static analysis check failed: %v", err),
		}, nil
	}

	logger.Info(fmt.Sprintf("%s assessment completed: %d issues found in %d files", r.toolName, len(issues), len(goFiles)))

	return &AssessmentResult{
		CommandName:   r.commandName,
		Category:      CategoryStaticAnalysis,
		Success:       true,
		ExecutionTime: time.Since(startTime),
		Issues:        issues,
	}, nil
}

// CanRunInParallel implements AssessmentRunner.CanRunInParallel
func (r *StaticAnalysisAssessmentRunner) CanRunInParallel() bool {
	return true // Static analysis can run in parallel on different files
}

// GetCategory implements AssessmentRunner.GetCategory
func (r *StaticAnalysisAssessmentRunner) GetCategory() AssessmentCategory {
	return CategoryStaticAnalysis
}

// GetEstimatedTime implements AssessmentRunner.GetEstimatedTime
func (r *StaticAnalysisAssessmentRunner) GetEstimatedTime(target string) time.Duration {
	// Estimate based on typical file counts and processing speed
	// Rough estimate: 200ms per file for static analysis
	goFiles, _ := r.findGoFiles(target, DefaultAssessmentConfig())
	estimatedMs := len(goFiles) * 200
	if estimatedMs < 500 {
		estimatedMs = 500 // Minimum 500ms
	}
	return time.Duration(estimatedMs) * time.Millisecond
}

// IsAvailable implements AssessmentRunner.IsAvailable
func (r *StaticAnalysisAssessmentRunner) IsAvailable() bool {
	_, err := exec.LookPath("go")
	return err == nil
}

// findGoFiles finds all Go files in the target directory
func (r *StaticAnalysisAssessmentRunner) findGoFiles(target string, config AssessmentConfig) ([]string, error) {
	var goFiles []string

	err := filepath.Walk(target, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			// Skip common directories we don't want to analyze
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
func (r *StaticAnalysisAssessmentRunner) shouldIncludeFile(filePath string, config AssessmentConfig) bool {
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

// checkStaticAnalysis runs go vet to check for static analysis issues
func (r *StaticAnalysisAssessmentRunner) checkStaticAnalysis(goFiles []string, config AssessmentConfig) ([]Issue, error) {
	var allIssues []Issue

	// Run go vet on the target directory (go vet works on packages, not individual files)
	targetDir := filepath.Dir(goFiles[0])
	if len(goFiles) == 0 {
		return allIssues, nil
	}

	// Find the Go module root to run vet from the correct context
	moduleRoot, err := r.findModuleRoot(targetDir)
	if err != nil {
		// If we can't find module root, just use the target directory
		moduleRoot = targetDir
	}

	args := []string{"vet", "./..."}
	cmd := exec.CommandContext(context.Background(), "go", args...)
	cmd.Dir = moduleRoot

	output, err := cmd.CombinedOutput()
	exitCode := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	}

	// Parse go vet output
	outputStr := string(output)
	if outputStr != "" {
		issues, parseErr := r.parseVetOutput(outputStr, targetDir)
		if parseErr != nil {
			return nil, fmt.Errorf("failed to parse vet output: %v", parseErr)
		}
		allIssues = append(allIssues, issues...)
	}

	// go vet returns non-zero exit code when issues are found, but we still want to return the issues
	if err != nil && exitCode != 0 && len(allIssues) == 0 {
		return nil, fmt.Errorf("go vet failed with exit code %d: %s", exitCode, outputStr)
	}

	return allIssues, nil
}

// findModuleRoot finds the Go module root directory
func (r *StaticAnalysisAssessmentRunner) findModuleRoot(startDir string) (string, error) {
	currentDir := startDir

	for {
		if _, err := os.Stat(filepath.Join(currentDir, "go.mod")); err == nil {
			return currentDir, nil
		}

		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			// Reached filesystem root
			break
		}
		currentDir = parentDir
	}

	return "", fmt.Errorf("go.mod not found")
}

// parseVetOutput parses go vet output and converts it to assessment issues
func (r *StaticAnalysisAssessmentRunner) parseVetOutput(output, targetDir string) ([]Issue, error) {
	var issues []Issue

	// go vet output format: file:line:col: message
	// Example: main.go:10:2: printf: non-constant format string
	vetLineRegex := regexp.MustCompile(`^([^:]+):(\d+):(\d+):\s*(.+)$`)

	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		matches := vetLineRegex.FindStringSubmatch(line)
		if len(matches) != 5 {
			// Skip lines that don't match the expected format
			continue
		}

		filePath := matches[1]
		lineNum := 0
		colNum := 0

		// Parse line and column numbers (ignore parse errors)
		if lineStr := matches[2]; lineStr != "" {
			_, _ = fmt.Sscanf(lineStr, "%d", &lineNum)
		}
		if colStr := matches[3]; colStr != "" {
			_, _ = fmt.Sscanf(colStr, "%d", &colNum)
		}

		message := matches[4]

		// Determine severity based on the type of issue
		severity := r.determineVetIssueSeverity(message)

		issue := Issue{
			File:          filePath,
			Line:          lineNum,
			Column:        colNum,
			Severity:      severity,
			Message:       fmt.Sprintf("%s: %s", r.toolName, message),
			Category:      CategoryStaticAnalysis,
			SubCategory:   r.categorizeVetIssue(message),
			AutoFixable:   false, // go vet issues typically require manual fixes
			EstimatedTime: r.estimateVetFixTime(message),
		}

		issues = append(issues, issue)
	}

	return issues, nil
}

// determineVetIssueSeverity determines the severity of a go vet issue
func (r *StaticAnalysisAssessmentRunner) determineVetIssueSeverity(message string) IssueSeverity {
	// High severity issues that could cause runtime problems
	highSeverityKeywords := []string{
		"nil pointer",
		"race condition",
		"deadlock",
		"unreachable code",
		"unused variable",
		"unused import",
	}

	// Medium severity issues that are code quality problems
	mediumSeverityKeywords := []string{
		"printf",
		"format string",
		"composite literal",
		"field tag",
		"method set",
	}

	messageLower := strings.ToLower(message)

	for _, keyword := range highSeverityKeywords {
		if strings.Contains(messageLower, keyword) {
			return SeverityHigh
		}
	}

	for _, keyword := range mediumSeverityKeywords {
		if strings.Contains(messageLower, keyword) {
			return SeverityMedium
		}
	}

	return SeverityLow
}

// categorizeVetIssue categorizes a go vet issue
func (r *StaticAnalysisAssessmentRunner) categorizeVetIssue(message string) string {
	messageLower := strings.ToLower(message)

	if strings.Contains(messageLower, "printf") || strings.Contains(messageLower, "format") {
		return "printf"
	}
	if strings.Contains(messageLower, "composite literal") {
		return "composite-literal"
	}
	if strings.Contains(messageLower, "field tag") {
		return "struct-tag"
	}
	if strings.Contains(messageLower, "method set") {
		return "method-set"
	}
	if strings.Contains(messageLower, "nil pointer") {
		return "nil-pointer"
	}
	if strings.Contains(messageLower, "race") {
		return "race-condition"
	}

	return "general"
}

// estimateVetFixTime estimates the time to fix a go vet issue
func (r *StaticAnalysisAssessmentRunner) estimateVetFixTime(message string) time.Duration {
	// Simple estimation based on issue type
	messageLower := strings.ToLower(message)

	if strings.Contains(messageLower, "unused") {
		return 1 * time.Minute // Quick fixes
	}
	if strings.Contains(messageLower, "printf") || strings.Contains(messageLower, "format") {
		return 3 * time.Minute // May require understanding format strings
	}
	if strings.Contains(messageLower, "composite literal") {
		return 5 * time.Minute // May require struct changes
	}

	return 2 * time.Minute // Default estimate
}

// init registers the static analysis assessment runner
func init() {
	RegisterAssessmentRunner(CategoryStaticAnalysis, NewStaticAnalysisAssessmentRunner())
}
