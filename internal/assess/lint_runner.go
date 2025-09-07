/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package assess

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

    "github.com/fulmenhq/goneat/pkg/logger"
)

// LintAssessmentRunner implements AssessmentRunner for linting tools like golangci-lint
type LintAssessmentRunner struct {
	commandName string
	toolName    string
	config      LintConfig
}

// LintConfig contains configuration for lint assessment
type LintConfig struct {
	EnabledLinters  []string      `json:"enabled_linters"`
	DisabledLinters []string      `json:"disabled_linters"`
	Timeout         time.Duration `json:"timeout"`
	MaxIssues       int           `json:"max_issues"`
	Format          string        `json:"format"` // "json" or "text"
	Mode            LintMode      `json:"mode"`   // "check", "fix", or "no-op"
}

// LintMode represents the operation mode for linting
type LintMode string

const (
	LintModeCheck LintMode = "check" // Report issues without fixing
	LintModeFix   LintMode = "fix"   // Report and attempt to fix issues
	LintModeNoOp  LintMode = "no-op" // Assessment only
)

// DefaultLintConfig returns default lint configuration
func DefaultLintConfig() LintConfig {
	return LintConfig{
		EnabledLinters:  []string{}, // Use golangci-lint defaults
		DisabledLinters: []string{}, // Use golangci-lint defaults
		Timeout:         5 * time.Minute,
		MaxIssues:       1000,          // Reasonable limit
		Format:          "json",        // Prefer structured output
		Mode:            LintModeCheck, // Default to check mode
	}
}

// NewLintAssessmentRunner creates a new lint assessment runner
func NewLintAssessmentRunner() *LintAssessmentRunner {
	return &LintAssessmentRunner{
		commandName: "lint",
		toolName:    "golangci-lint",
		config:      DefaultLintConfig(),
	}
}

// Assess implements AssessmentRunner.Assess
func (r *LintAssessmentRunner) Assess(ctx context.Context, target string, config AssessmentConfig) (*AssessmentResult, error) {
	startTime := time.Now()

	modeDescription := r.getModeDescription(config.Mode)
	logger.Info(fmt.Sprintf("Running %s assessment on %s (%s)", r.toolName, target, modeDescription))

	// Check if golangci-lint is available
	if !r.IsAvailable() {
		return &AssessmentResult{
			CommandName:   r.commandName,
			Category:      CategoryLint,
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
			Category:      CategoryLint,
			Success:       false,
			ExecutionTime: time.Since(startTime),
			Error:         fmt.Sprintf("failed to find Go files: %v", err),
		}, nil
	}

	if len(goFiles) == 0 {
		logger.Info("No Go files found for lint assessment")
		return &AssessmentResult{
			CommandName:   r.commandName,
			Category:      CategoryLint,
			Success:       true,
			ExecutionTime: time.Since(startTime),
			Issues:        []Issue{},
		}, nil
	}

	// Run golangci-lint based on mode
	var issues []Issue
	var runErr error

	switch config.Mode {
	case AssessmentModeNoOp:
		// No-op mode: just log what would be done
		logger.Info(fmt.Sprintf("[NO-OP] Would run %s on %d files", r.toolName, len(goFiles)))
		issues = []Issue{} // No issues to report in no-op mode

	case AssessmentModeCheck:
		// Check mode: run linting and report issues
		issues, runErr = r.runGolangCILintCheck(target, config)

	case AssessmentModeFix:
		// Fix mode: run linting with auto-fix
		issues, runErr = r.runGolangCILintFix(target, config)

	default:
		return &AssessmentResult{
			CommandName:   r.commandName,
			Category:      CategoryLint,
			Success:       false,
			ExecutionTime: time.Since(startTime),
			Error:         fmt.Sprintf("unsupported assessment mode: %s", config.Mode),
		}, nil
	}

	if runErr != nil {
		return &AssessmentResult{
			CommandName:   r.commandName,
			Category:      CategoryLint,
			Success:       false,
			ExecutionTime: time.Since(startTime),
			Error:         fmt.Sprintf("lint operation failed: %v", runErr),
		}, nil
	}

	modeStr := r.getModeString(config.Mode)
	logger.Info(fmt.Sprintf("%s %s completed: %d issues found in %d files", r.toolName, modeStr, len(issues), len(goFiles)))

	return &AssessmentResult{
		CommandName:   r.commandName,
		Category:      CategoryLint,
		Success:       true,
		ExecutionTime: time.Since(startTime),
		Issues:        issues,
	}, nil
}

// runGolangCILintCheck runs golangci-lint in check mode (report issues)
func (r *LintAssessmentRunner) runGolangCILintCheck(target string, config AssessmentConfig) ([]Issue, error) {
	return r.runGolangCILintWithMode(target, config, false)
}

// runGolangCILintFix runs golangci-lint in fix mode (apply fixes)
func (r *LintAssessmentRunner) runGolangCILintFix(target string, config AssessmentConfig) ([]Issue, error) {
	return r.runGolangCILintWithMode(target, config, true)
}

// runGolangCILintWithMode runs golangci-lint with the specified mode
func (r *LintAssessmentRunner) runGolangCILintWithMode(target string, config AssessmentConfig, fixMode bool) ([]Issue, error) {
	// Clean paths to prevent path traversal issues
	target = filepath.Clean(target)
	includeFiles := make([]string, len(config.IncludeFiles))
	for i, file := range config.IncludeFiles {
		includeFiles[i] = filepath.Clean(file)
	}
	// Build command arguments
	args := []string{"run", "--timeout", r.config.Timeout.String()}

	// Add fix flag if in fix mode
	if fixMode {
		args = append(args, "--fix")
	}

	// Add output format (only for check mode, fix mode doesn't produce structured output)
	if !fixMode && r.config.Format == "json" {
		args = append(args, "--output.json.path", "stdout")
	}

	// Add enabled linters
	for _, linter := range r.config.EnabledLinters {
		args = append(args, "--enable", linter)
	}

	// Add disabled linters
	for _, linter := range r.config.DisabledLinters {
		args = append(args, "--disable", linter)
	}

	// Create command with context
	ctx, cancel := context.WithTimeout(context.Background(), r.config.Timeout)
	defer cancel()

	// Add target path(s): Prefer restricting to included files if provided
	var cmd *exec.Cmd
	if len(includeFiles) > 0 {
		// Filter to only .go files before passing to golangci-lint
		goFiles := make([]string, 0, len(includeFiles))
		for _, file := range includeFiles {
			if strings.HasSuffix(strings.ToLower(file), ".go") {
				goFiles = append(goFiles, file)
			}
		}

		if len(goFiles) > 0 {
			// Append only Go files (golangci-lint requires .go files for named file mode)
			args = append(args, goFiles...)
			cmd = exec.CommandContext(ctx, "golangci-lint", args...) // #nosec G204
			cmd.Dir = target
		} else {
			// No Go files in include list, fall back to directory mode
			args = append(args, "./...")
			cmd = exec.CommandContext(ctx, "golangci-lint", args...) // #nosec G204
			cmd.Dir = target
		}
	} else if info, err := os.Stat(target); err == nil && !info.IsDir() {
		// Target is a single file - only proceed if it's a .go file
		if strings.HasSuffix(strings.ToLower(target), ".go") {
			args = append(args, target)
			cmd = exec.CommandContext(ctx, "golangci-lint", args...) // #nosec G204
		} else {
			// Non-Go file, return empty result (no issues to lint)
			return []Issue{}, nil
		}
	} else {
		// Target is a directory; analyze all
		args = append(args, "./...")
		cmd = exec.CommandContext(ctx, "golangci-lint", args...) // #nosec G204
		cmd.Dir = target
	}

	// Execute command
	output, err := cmd.CombinedOutput()

	// Debug logging removed - JSON parsing now works correctly

	// golangci-lint returns non-zero exit code when issues are found
	// This is expected behavior, not an error for check mode
	if err != nil {
		exitCode := 0
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
		// Exit code 1 means issues found (normal for check mode), other codes are actual errors
		if exitCode != 1 {
			return nil, fmt.Errorf("golangci-lint execution failed: %v, output: %s", err, string(output))
		}
	}

	// For fix mode, we don't parse issues (golangci-lint doesn't provide structured output when fixing)
	if fixMode {
		logger.Info(fmt.Sprintf("%s applied fixes to target: %s", r.toolName, target))
		return []Issue{}, nil
	}

	// Parse output for check mode
	if r.config.Format == "json" {
		return r.parseLintJSONOutput(output)
	}
	return r.parseLintTextOutput(output)
}

// getModeDescription returns a human-readable description for the assessment mode
func (r *LintAssessmentRunner) getModeDescription(mode AssessmentMode) string {
	switch mode {
	case AssessmentModeNoOp:
		return "assessment only (no changes)"
	case AssessmentModeCheck:
		return "check and report issues"
	case AssessmentModeFix:
		return "check and fix issues automatically"
	default:
		return "assessment"
	}
}

// getModeString returns a human-readable string for the assessment mode
func (r *LintAssessmentRunner) getModeString(mode AssessmentMode) string {
	switch mode {
	case AssessmentModeNoOp:
		return "no-op assessment"
	case AssessmentModeCheck:
		return "check"
	case AssessmentModeFix:
		return "fix"
	default:
		return "assessment"
	}
}

// CanRunInParallel implements AssessmentRunner.CanRunInParallel
func (r *LintAssessmentRunner) CanRunInParallel() bool {
	return true // Lint checks can run in parallel on different files
}

// GetCategory implements AssessmentRunner.GetCategory
func (r *LintAssessmentRunner) GetCategory() AssessmentCategory {
	return CategoryLint
}

// GetEstimatedTime implements AssessmentRunner.GetEstimatedTime
func (r *LintAssessmentRunner) GetEstimatedTime(target string) time.Duration {
	// Estimate based on typical file counts and processing speed
	// Rough estimate: 500ms per file for comprehensive linting
	goFiles, _ := r.findGoFiles(target, DefaultAssessmentConfig())
	estimatedMs := len(goFiles) * 500
	if estimatedMs < 1000 {
		estimatedMs = 1000 // Minimum 1 second
	}
	if estimatedMs > 30000 {
		estimatedMs = 30000 // Maximum 30 seconds
	}
	return time.Duration(estimatedMs) * time.Millisecond
}

// IsAvailable implements AssessmentRunner.IsAvailable
func (r *LintAssessmentRunner) IsAvailable() bool {
	_, err := exec.LookPath("golangci-lint")
	return err == nil
}

// findGoFiles finds all Go files in the target directory
func (r *LintAssessmentRunner) findGoFiles(target string, config AssessmentConfig) ([]string, error) {
	// Reuse the static analysis runner's file finding logic
	saRunner := NewStaticAnalysisAssessmentRunner()
	return saRunner.findGoFiles(target, config)
}

// parseLintJSONOutput parses golangci-lint JSON output
func (r *LintAssessmentRunner) parseLintJSONOutput(output []byte) ([]Issue, error) {
	var issues []Issue

	// golangci-lint JSON format structure
	type LintIssue struct {
		FromLinter string `json:"FromLinter"`
		Text       string `json:"Text"`
		Pos        struct {
			Filename string `json:"Filename"`
			Line     int    `json:"Line"`
			Column   int    `json:"Column"`
		} `json:"Pos"`
	}

	type LintReport struct {
		Issues []LintIssue `json:"Issues"`
	}

	// Extract JSON part from golangci-lint output (it includes summary text after JSON)
	jsonStr := string(output)
	if idx := strings.Index(jsonStr, "}\n"); idx > 0 {
		// Find the end of the JSON object
		jsonStr = jsonStr[:idx+1]
	}

	var report LintReport
	if err := json.Unmarshal([]byte(jsonStr), &report); err != nil {
		// If JSON parsing fails, fall back to text parsing
		logger.Warn(fmt.Sprintf("JSON parsing failed, falling back to text parsing: %v", err))
		return r.parseLintTextOutput(output)
	}

	for _, lintIssue := range report.Issues {
		// Skip if we've reached the max issues limit
		if len(issues) >= r.config.MaxIssues {
			break
		}

		// Create assessment issue from lint issue
		severity := r.determineLintSeverity(lintIssue.FromLinter, lintIssue.Text)
		subCategory := r.categorizeLintIssue(lintIssue.FromLinter)

		issue := Issue{
			File:          lintIssue.Pos.Filename,
			Line:          lintIssue.Pos.Line,
			Column:        lintIssue.Pos.Column,
			Severity:      severity,
			Message:       fmt.Sprintf("%s: %s", r.toolName, lintIssue.Text),
			Category:      CategoryLint,
			SubCategory:   subCategory,
			AutoFixable:   r.isLintIssueAutoFixable(lintIssue.FromLinter),
			EstimatedTime: r.estimateLintFixTime(lintIssue.FromLinter, lintIssue.Text),
		}

		issues = append(issues, issue)
	}

	return issues, nil
}

// parseLintTextOutput parses golangci-lint text output
func (r *LintAssessmentRunner) parseLintTextOutput(output []byte) ([]Issue, error) {
	var issues []Issue

	outputStr := string(output)
	lines := strings.Split(outputStr, "\n")

	// Look for patterns like: file:line:col: linter: message
	fileLinePattern := regexp.MustCompile(`^([^:]+):(\d+):(\d+):\s*([^:]+):\s*(.+)$`)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Skip if we've reached the max issues limit
		if len(issues) >= r.config.MaxIssues {
			break
		}

		matches := fileLinePattern.FindStringSubmatch(line)
		if len(matches) >= 6 {
			filePath := matches[1]
			lineNumStr := matches[2]
			colNumStr := matches[3]
			linterName := matches[4]
			message := matches[5]

			// Parse line and column numbers
			lineNum, _ := strconv.Atoi(lineNumStr)
			colNum, _ := strconv.Atoi(colNumStr)

			// Create assessment issue
			severity := r.determineLintSeverity(linterName, message)
			subCategory := r.categorizeLintIssue(linterName)

			issue := Issue{
				File:          filePath,
				Line:          lineNum,
				Column:        colNum,
				Severity:      severity,
				Message:       fmt.Sprintf("%s: %s", r.toolName, message),
				Category:      CategoryLint,
				SubCategory:   subCategory,
				AutoFixable:   r.isLintIssueAutoFixable(linterName),
				EstimatedTime: r.estimateLintFixTime(linterName, message),
			}

			issues = append(issues, issue)
		}
	}

	return issues, nil
}

// determineLintSeverity determines the severity of a lint issue based on linter and message
func (r *LintAssessmentRunner) determineLintSeverity(linterName, message string) IssueSeverity {
	linterName = strings.ToLower(linterName)
	messageLower := strings.ToLower(message)

	// High severity linters (critical issues)
	highSeverityLinters := []string{
		"errcheck", "govet", "staticcheck", "gosec", "ineffassign",
		"deadcode", "unused", "gosimple",
	}

	// Medium severity linters (code quality issues)
	mediumSeverityLinters := []string{
		"golint", "goimports", "misspell", "goconst", "gocyclo",
		"dupl", "lll", "maligned", "prealloc",
	}

	// Check linter name first
	for _, linter := range highSeverityLinters {
		if strings.Contains(linterName, linter) {
			return SeverityHigh
		}
	}

	for _, linter := range mediumSeverityLinters {
		if strings.Contains(linterName, linter) {
			return SeverityMedium
		}
	}

	// Check message content for severity indicators
	if strings.Contains(messageLower, "security") || strings.Contains(messageLower, "unsafe") {
		return SeverityHigh
	}
	if strings.Contains(messageLower, "error") || strings.Contains(messageLower, "bug") {
		return SeverityHigh
	}
	if strings.Contains(messageLower, "unused") || strings.Contains(messageLower, "dead") {
		return SeverityMedium
	}

	return SeverityLow
}

// categorizeLintIssue categorizes a lint issue by linter type
func (r *LintAssessmentRunner) categorizeLintIssue(linterName string) string {
	linterName = strings.ToLower(linterName)

	// Map linters to categories
	switch {
	case strings.Contains(linterName, "errcheck"):
		return "error-handling"
	case strings.Contains(linterName, "govet") || strings.Contains(linterName, "staticcheck"):
		return "correctness"
	case strings.Contains(linterName, "gosec") || strings.Contains(linterName, "ineffassign"):
		return "security"
	case strings.Contains(linterName, "goimports") || strings.Contains(linterName, "golint"):
		return "style"
	case strings.Contains(linterName, "gocyclo") || strings.Contains(linterName, "dupl"):
		return "complexity"
	case strings.Contains(linterName, "unused") || strings.Contains(linterName, "deadcode"):
		return "maintenance"
	default:
		return "general"
	}
}

// isLintIssueAutoFixable determines if a lint issue can be auto-fixed
func (r *LintAssessmentRunner) isLintIssueAutoFixable(linterName string) bool {
	// Linters that typically support auto-fixing
	autoFixableLinters := []string{
		"goimports", "gofmt", "goimports", "misspell",
		"golint", "whitespace", "goconst",
	}

	linterName = strings.ToLower(linterName)
	for _, linter := range autoFixableLinters {
		if strings.Contains(linterName, linter) {
			return true
		}
	}

	return false
}

// estimateLintFixTime estimates the time to fix a lint issue
func (r *LintAssessmentRunner) estimateLintFixTime(linterName, message string) time.Duration {
	// Base time estimates by linter type
	linterName = strings.ToLower(linterName)

	switch {
	case strings.Contains(linterName, "errcheck"):
		return 2 * time.Minute // Add error checking
	case strings.Contains(linterName, "goimports") || strings.Contains(linterName, "gofmt"):
		return 30 * time.Second // Usually quick formatting
	case strings.Contains(linterName, "govet") || strings.Contains(linterName, "staticcheck"):
		return 5 * time.Minute // May require code changes
	case strings.Contains(linterName, "unused"):
		return 1 * time.Minute // Usually just removal
	case strings.Contains(linterName, "golint") || strings.Contains(linterName, "misspell"):
		return 2 * time.Minute // Style/documentation fixes
	default:
		return 3 * time.Minute // Default estimate
	}
}

// init registers the lint assessment runner
func init() {
	RegisterAssessmentRunner(CategoryLint, NewLintAssessmentRunner())
}
