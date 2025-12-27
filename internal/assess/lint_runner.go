/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package assess

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/fulmenhq/goneat/pkg/ignore"
	"github.com/fulmenhq/goneat/pkg/logger"
	"github.com/fulmenhq/goneat/pkg/versioning"
)

// LintAssessmentRunner implements AssessmentRunner for linting tools like golangci-lint
type LintAssessmentRunner struct {
	commandName string
	toolName    string
	config      LintConfig
}

type golangciLintMode int

const (
	golangciLintModeUnknown golangciLintMode = iota
	golangciLintModeV1
	golangciLintModeV2Early
	golangciLintModeV24Plus
)

type golangciLintEnvironment struct {
	mode      golangciLintMode
	raw       string
	version   *versioning.Version
	detectErr error
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
			ExecutionTime: HumanReadableDuration(time.Since(startTime)),
			Error:         fmt.Sprintf("%s command not found in PATH", r.toolName),
		}, nil
	}

	env := r.detectGolangciLintEnvironment()
	if env.detectErr != nil {
		logger.Warn(fmt.Sprintf("golangci-lint version detection failed: %v", env.detectErr))
	}

	// 🔧 Preflight: Verify golangci-lint configuration using detected version context
	if err := r.verifyGolangciConfig(target, env); err != nil {
		return &AssessmentResult{
			CommandName:   r.commandName,
			Category:      CategoryLint,
			Success:       false,
			ExecutionTime: HumanReadableDuration(time.Since(startTime)),
			Error:         err.Error(),
		}, nil
	}

	// Find Go files to assess
	goFiles, err := r.findGoFiles(target, config)
	if err != nil {
		return &AssessmentResult{
			CommandName:   r.commandName,
			Category:      CategoryLint,
			Success:       false,
			ExecutionTime: HumanReadableDuration(time.Since(startTime)),
			Error:         fmt.Sprintf("failed to find Go files: %v", err),
		}, nil
	}

	// Filter out files that match repo ignore patterns (gitignore + goneatignore)
	goFiles = r.filterFilesRespectingIgnores(goFiles, target, config)

	if len(goFiles) == 0 {
		logger.Info("No Go files found for lint assessment")
		return &AssessmentResult{
			CommandName:   r.commandName,
			Category:      CategoryLint,
			Success:       true,
			ExecutionTime: HumanReadableDuration(time.Since(startTime)),
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
		issues, runErr = r.runGolangCILintCheck(target, config, env)

	case AssessmentModeFix:
		// Fix mode: run linting with auto-fix
		issues, runErr = r.runGolangCILintFix(target, config, env)

	default:
		return &AssessmentResult{
			CommandName:   r.commandName,
			Category:      CategoryLint,
			Success:       false,
			ExecutionTime: HumanReadableDuration(time.Since(startTime)),
			Error:         fmt.Sprintf("unsupported assessment mode: %s", config.Mode),
		}, nil
	}

	if runErr != nil {
		return &AssessmentResult{
			CommandName:   r.commandName,
			Category:      CategoryLint,
			Success:       false,
			ExecutionTime: HumanReadableDuration(time.Since(startTime)),
			Error:         fmt.Sprintf("lint operation failed: %v", runErr),
		}, nil
	}

	overrides := loadAssessOverrides(target)

	yamlIssues, yamlErr := r.runYamllintAssessment(target, config, overrides)
	if yamlErr != nil {
		return &AssessmentResult{
			CommandName:   r.commandName,
			Category:      CategoryLint,
			Success:       false,
			ExecutionTime: HumanReadableDuration(time.Since(startTime)),
			Error:         fmt.Sprintf("yamllint failed: %v", yamlErr),
		}, nil
	}
	if len(yamlIssues) > 0 {
		issues = append(issues, yamlIssues...)
	}

	shfmtIssues, shfmtErr := r.runShfmtAssessment(target, config, overrides)
	if shfmtErr != nil {
		return &AssessmentResult{
			CommandName:   r.commandName,
			Category:      CategoryLint,
			Success:       false,
			ExecutionTime: HumanReadableDuration(time.Since(startTime)),
			Error:         fmt.Sprintf("shfmt failed: %v", shfmtErr),
		}, nil
	}
	issues = append(issues, shfmtIssues...)

	scIssues, scErr := r.runShellcheckAssessment(target, config, overrides)
	if scErr != nil {
		return &AssessmentResult{
			CommandName:   r.commandName,
			Category:      CategoryLint,
			Success:       false,
			ExecutionTime: HumanReadableDuration(time.Since(startTime)),
			Error:         fmt.Sprintf("shellcheck failed: %v", scErr),
		}, nil
	}
	issues = append(issues, scIssues...)

	actionIssues, actionErr := r.runActionlintAssessment(target, config, overrides)
	if actionErr != nil {
		return &AssessmentResult{
			CommandName:   r.commandName,
			Category:      CategoryLint,
			Success:       false,
			ExecutionTime: HumanReadableDuration(time.Since(startTime)),
			Error:         fmt.Sprintf("actionlint failed: %v", actionErr),
		}, nil
	}
	issues = append(issues, actionIssues...)

	makeIssues, makeErr := r.runCheckmakeAssessment(target, config, overrides)
	if makeErr != nil {
		return &AssessmentResult{
			CommandName:   r.commandName,
			Category:      CategoryLint,
			Success:       false,
			ExecutionTime: HumanReadableDuration(time.Since(startTime)),
			Error:         fmt.Sprintf("checkmake failed: %v", makeErr),
		}, nil
	}
	issues = append(issues, makeIssues...)

	modeStr := r.getModeString(config.Mode)
	logger.Info(fmt.Sprintf("%s %s completed: %d issues found in %d files", r.toolName, modeStr, len(issues), len(goFiles)))

	return &AssessmentResult{
		CommandName:   r.commandName,
		Category:      CategoryLint,
		Success:       true,
		ExecutionTime: HumanReadableDuration(time.Since(startTime)),
		Issues:        issues,
	}, nil
}

// runGolangCILintCheck runs golangci-lint in check mode (report issues)
func (r *LintAssessmentRunner) runGolangCILintCheck(target string, config AssessmentConfig, env golangciLintEnvironment) ([]Issue, error) {
	return r.runGolangCILintWithMode(target, config, env, false)
}

// runGolangCILintFix runs golangci-lint in fix mode (apply fixes)
func (r *LintAssessmentRunner) runGolangCILintFix(target string, config AssessmentConfig, env golangciLintEnvironment) ([]Issue, error) {
	return r.runGolangCILintWithMode(target, config, env, true)
}

// runGolangCILintWithMode runs golangci-lint with the specified mode
func (r *LintAssessmentRunner) runShfmtAssessment(target string, config AssessmentConfig, overrides *assessOverrides) ([]Issue, error) {
	if !config.LintShellEnabled {
		return nil, nil
	}
	var ov *shellOverrides
	if overrides != nil && overrides.Lint != nil {
		ov = overrides.Lint.Shell
	}
	enabled := config.LintShellEnabled
	if ov != nil && ov.Shfmt != nil {
		enabled = boolWithDefault(ov.Shfmt.Enabled, enabled)
	}
	if !enabled {
		return nil, nil
	}

	paths := config.LintShellPaths
	exclude := append([]string{}, config.LintShellExclude...)
	if ov != nil {
		if len(ov.Paths) > 0 {
			paths = ov.Paths
		}
		exclude = append(exclude, ov.Ignore...)
		if ov.Shfmt != nil {
			if len(ov.Shfmt.Paths) > 0 {
				paths = ov.Shfmt.Paths
			}
			exclude = append(exclude, ov.Shfmt.Ignore...)
		}
	}

	files, err := collectFilesWithScope(target, paths, exclude, config)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, nil
	}

	args := []string{"-d"}
	if config.LintShellFix || (ov != nil && ov.Shfmt != nil && boolWithDefault(ov.Shfmt.Fix, false)) || config.Mode == AssessmentModeFix {
		args = []string{"-w"}
	}
	if ov != nil && ov.Shfmt != nil {
		args = append(args, sanitizeShfmtArgs(ov.Shfmt.Args)...)
	}
	args = append(args, files...)

	ctx, cancel := context.WithTimeout(context.Background(), r.config.Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "shfmt", args...) // #nosec G204
	cmd.Dir = target
	output, err := cmd.CombinedOutput()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			// shfmt returns non-zero when diffs exist; treat as issues
			if len(output) == 0 {
				return issuesFromFiles(files, "shfmt reported formatting differences"), nil
			}
			parsed := issuesFromShfmtOutput(string(output))
			if len(parsed) == 0 {
				return issuesFromFiles(files, "shfmt reported issues"), nil
			}
			return parsed, nil
		}
		return nil, fmt.Errorf("shfmt execution failed: %v", err)
	}
	if len(output) > 0 {
		parsed := issuesFromShfmtOutput(string(output))
		if len(parsed) == 0 {
			return issuesFromFiles(files, "shfmt reported issues"), nil
		}
		return parsed, nil
	}
	return nil, nil
}

func (r *LintAssessmentRunner) runShellcheckAssessment(target string, config AssessmentConfig, overrides *assessOverrides) ([]Issue, error) {
	enabled := config.LintShellcheckEnabled
	var ovShell *shellOverrides
	if overrides != nil && overrides.Lint != nil {
		ovShell = overrides.Lint.Shell
	}
	if ovShell != nil && ovShell.Shellcheck != nil {
		enabled = boolWithDefault(ovShell.Shellcheck.Enabled, enabled)
	}
	if !enabled {
		return nil, nil
	}

	bin := strings.TrimSpace(config.LintShellcheckPath)
	if bin == "" && ovShell != nil && ovShell.Shellcheck != nil {
		bin = strings.TrimSpace(ovShell.Shellcheck.Path)
	}
	if bin == "" {
		bin = "shellcheck"
	}
	bin = filepath.Clean(bin)
	if _, err := exec.LookPath(bin); err != nil {
		logger.Info("shellcheck not found; skipping shellcheck lint")
		return nil, nil
	}

	paths := config.LintShellPaths
	exclude := append([]string{}, config.LintShellExclude...)
	if ovShell != nil {
		if len(ovShell.Paths) > 0 {
			paths = ovShell.Paths
		}
		exclude = append(exclude, ovShell.Ignore...)
		if ovShell.Shellcheck != nil {
			if len(ovShell.Shellcheck.Paths) > 0 {
				paths = ovShell.Shellcheck.Paths
			}
			exclude = append(exclude, ovShell.Shellcheck.Ignore...)
		}
	}

	files, err := collectFilesWithScope(target, paths, exclude, config)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, nil
	}

	args := []string{"--format", "json"}
	args = append(args, files...)
	ctx, cancel := context.WithTimeout(context.Background(), r.config.Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...) // #nosec G204
	cmd.Dir = target
	output, err := cmd.CombinedOutput()
	if err != nil {
		// shellcheck returns non-zero when issues found; still parse output
		if len(output) == 0 {
			return nil, fmt.Errorf("shellcheck failed: %v", err)
		}
	}

	var scIssues []struct {
		File    string `json:"file"`
		Line    int    `json:"line"`
		Column  int    `json:"column"`
		Level   string `json:"level"`
		Message string `json:"message"`
	}
	if len(output) == 0 {
		return nil, nil
	}
	if jsonErr := json.Unmarshal(output, &scIssues); jsonErr != nil {
		return nil, fmt.Errorf("failed to parse shellcheck output: %v", jsonErr)
	}
	issues := make([]Issue, 0, len(scIssues))
	for _, iss := range scIssues {
		sev := SeverityMedium
		switch strings.ToLower(iss.Level) {
		case "error":
			sev = SeverityHigh
		case "warning":
			sev = SeverityMedium
		case "info", "style":
			sev = SeverityLow
		}
		issues = append(issues, Issue{
			File:        filepath.ToSlash(iss.File),
			Line:        iss.Line,
			Column:      iss.Column,
			Severity:    sev,
			Message:     iss.Message,
			Category:    CategoryLint,
			SubCategory: "shell:shellcheck",
		})
	}
	return issues, nil
}

func (r *LintAssessmentRunner) runActionlintAssessment(target string, config AssessmentConfig, overrides *assessOverrides) ([]Issue, error) {
	enabled := config.LintGHAEnabled
	var ov *githubActionsConfig
	if overrides != nil && overrides.Lint != nil {
		ov = overrides.Lint.GitHubActions
	}
	if ov != nil && ov.Actionlint != nil {
		enabled = boolWithDefault(ov.Actionlint.Enabled, enabled)
	}
	if !enabled {
		return nil, nil
	}
	paths := config.LintGHAPaths
	exclude := append([]string{}, config.LintGHAExclude...)
	if ov != nil && ov.Actionlint != nil {
		if len(ov.Actionlint.Paths) > 0 {
			paths = ov.Actionlint.Paths
		}
		exclude = append(exclude, ov.Actionlint.Ignore...)
	}

	files, err := collectFilesWithScope(target, paths, exclude, config)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, nil
	}

	bin := "actionlint"
	if _, err := exec.LookPath(bin); err != nil {
		logger.Info("actionlint not found; skipping GitHub Actions lint")
		return nil, nil
	}

	args := []string{"-format", "{{json .}}"}
	args = append(args, files...)
	ctx, cancel := context.WithTimeout(context.Background(), r.config.Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...) // #nosec G204
	cmd.Dir = target
	output, err := cmd.CombinedOutput()
	if err != nil && len(output) == 0 {
		return nil, fmt.Errorf("actionlint failed: %v", err)
	}
	issues, parseErr := parseActionlintOutput(output)
	if parseErr != nil {
		return nil, parseErr
	}
	return issues, nil
}

func parseActionlintOutput(output []byte) ([]Issue, error) {
	trimmed := bytes.TrimSpace(output)
	if len(trimmed) == 0 {
		return nil, nil
	}

	type actionlintResult struct {
		Message  string `json:"message"`
		Filepath string `json:"filepath"`
		Line     int    `json:"line"`
		Column   int    `json:"column"`
		Level    string `json:"level"`
		Kind     string `json:"kind"`
	}

	var parsed []actionlintResult
	if err := json.Unmarshal(trimmed, &parsed); err != nil {
		// Fallback: try line-delimited JSON objects
		lines := bytes.Split(trimmed, []byte("\n"))
		for _, ln := range lines {
			ln = bytes.TrimSpace(ln)
			if len(ln) == 0 {
				continue
			}
			var item actionlintResult
			if jsonErr := json.Unmarshal(ln, &item); jsonErr != nil {
				return nil, fmt.Errorf("failed to parse actionlint output: %v", err)
			}
			parsed = append(parsed, item)
		}
	}

	if len(parsed) == 0 {
		return nil, nil
	}
	issues := make([]Issue, 0, len(parsed))
	for _, iss := range parsed {
		sev := SeverityMedium
		if strings.EqualFold(iss.Level, "error") {
			sev = SeverityHigh
		}
		issues = append(issues, Issue{
			File:        filepath.ToSlash(iss.Filepath),
			Line:        iss.Line,
			Column:      iss.Column,
			Severity:    sev,
			Message:     iss.Message,
			Category:    CategoryLint,
			SubCategory: "gha:actionlint",
		})
	}
	return issues, nil
}

func writeCheckmakeConfig(cfg *checkmakeConfigOptions) (string, error) {
	if cfg == nil {
		return "", nil
	}
	if cfg.MaxBodyLength == nil && len(cfg.MinPhonyTargets) == 0 {
		return "", nil
	}

	var b strings.Builder
	if cfg.MaxBodyLength != nil {
		fmt.Fprintf(&b, "[maxbodylength]\nmaxBodyLength=%d\n\n", *cfg.MaxBodyLength)
	}
	if len(cfg.MinPhonyTargets) > 0 {
		cleanTargets := make([]string, 0, len(cfg.MinPhonyTargets))
		for _, t := range cfg.MinPhonyTargets {
			t = strings.TrimSpace(t)
			if t == "" {
				continue
			}
			cleanTargets = append(cleanTargets, t)
		}
		if len(cleanTargets) > 0 {
			fmt.Fprintf(&b, "[minphony]\nrequired=%s\n\n", strings.Join(cleanTargets, ","))
		}
	}

	contents := strings.TrimSpace(b.String()) + "\n"
	if strings.TrimSpace(contents) == "" {
		return "", nil
	}

	f, err := os.CreateTemp("", "goneat-checkmake-*.ini")
	if err != nil {
		return "", fmt.Errorf("failed to create temp checkmake config: %w", err)
	}
	defer func() { _ = f.Close() }()

	if err := f.Chmod(0600); err != nil {
		return "", fmt.Errorf("failed to chmod temp checkmake config: %w", err)
	}
	if _, err := f.WriteString(contents); err != nil {
		return "", fmt.Errorf("failed to write temp checkmake config: %w", err)
	}

	return f.Name(), nil
}

func (r *LintAssessmentRunner) runCheckmakeAssessment(target string, config AssessmentConfig, overrides *assessOverrides) ([]Issue, error) {
	enabled := config.LintMakeEnabled
	var ov *makeOverrides
	if overrides != nil && overrides.Lint != nil {
		ov = overrides.Lint.Make
	}
	if ov != nil && ov.Checkmake != nil {
		enabled = boolWithDefault(ov.Checkmake.Enabled, enabled)
	}
	if !enabled {
		return nil, nil
	}

	paths := config.LintMakePaths
	exclude := append([]string{}, config.LintMakeExclude...)
	if ov != nil {
		if len(ov.Paths) > 0 {
			paths = ov.Paths
		}
		exclude = append(exclude, ov.Ignore...)
	}

	files, err := collectFilesWithScope(target, paths, exclude, config)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, nil
	}

	bin := "checkmake"
	if _, err := exec.LookPath(bin); err != nil {
		logger.Info("checkmake not found; skipping Makefile lint")
		return nil, nil
	}

	var configPath string
	if ov != nil && ov.Checkmake != nil {
		configPath, err = writeCheckmakeConfig(ov.Checkmake.Config)
		if err != nil {
			return nil, err
		}
		if configPath != "" {
			defer func() { _ = os.Remove(configPath) }()
		}
	}

	issues := []Issue{}
	for _, f := range files {
		args := []string{}
		if configPath != "" {
			args = append(args, "--config", configPath)
		}
		args = append(args, f)
		ctx, cancel := context.WithTimeout(context.Background(), r.config.Timeout)
		cmd := exec.CommandContext(ctx, bin, args...) // #nosec G204
		cmd.Dir = target
		output, err := cmd.CombinedOutput()
		cancel()
		if err != nil {
			if len(output) == 0 {
				return nil, fmt.Errorf("checkmake failed on %s: %v", f, err)
			}
		}
		if len(output) == 0 {
			continue
		}
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) == "" {
				continue
			}
			issues = append(issues, Issue{
				File:        filepath.ToSlash(f),
				Severity:    SeverityMedium,
				Message:     line,
				Category:    CategoryLint,
				SubCategory: "make:checkmake",
			})
		}
	}
	return issues, nil
}

func collectFilesWithScope(root string, includes, excludes []string, config AssessmentConfig) ([]string, error) {
	if len(includes) == 0 {
		return nil, nil
	}

	var matcher *ignore.Matcher
	if !config.NoIgnore {
		m, err := ignore.NewMatcher(root)
		if err == nil {
			matcher = m
		}
	}

	files := make([]string, 0)
	seen := make(map[string]struct{})
	for _, pattern := range includes {
		patterns := []string{pattern}
		// Doublestar patterns like "**/Makefile" are expected to match a root-level
		// Makefile, but this is not consistently true across environments.
		// Add a compatibility fallback for the common "**/" prefix.
		if strings.HasPrefix(pattern, "**/") {
			patterns = append(patterns, strings.TrimPrefix(pattern, "**/"))
		}

		for _, pat := range patterns {
			absPattern := filepath.Join(root, filepath.FromSlash(pat))
			matches, err := doublestar.FilepathGlob(absPattern)
			if err != nil {
				return nil, fmt.Errorf("invalid pattern %q: %w", pat, err)
			}
			for _, match := range matches {
				info, err := os.Stat(match)
				if err != nil || info.IsDir() {
					continue
				}
				rel, err := filepath.Rel(root, match)
				if err != nil {
					continue
				}
				rel = filepath.ToSlash(rel)
				if strings.HasSuffix(rel, ".orig") {
					continue
				}

				if matcher != nil && matcher.IsIgnoredRel(rel) && !matchesForceInclude(rel, config.ForceInclude) {
					continue
				}
				if isExcluded(rel, excludes) {
					continue
				}
				if _, ok := seen[rel]; ok {
					continue
				}
				seen[rel] = struct{}{}
				files = append(files, rel)
			}
		}
	}

	sort.Strings(files)
	return files, nil
}

func matchesForceInclude(rel string, patterns []string) bool {
	if len(patterns) == 0 {
		return false
	}
	rel = filepath.ToSlash(rel)
	base := filepath.Base(rel)

	for _, raw := range patterns {
		pat := filepath.ToSlash(strings.TrimSpace(raw))
		if pat == "" {
			continue
		}
		for strings.HasPrefix(pat, "./") {
			pat = strings.TrimPrefix(pat, "./")
		}

		if strings.HasSuffix(pat, "/**") {
			prefix := strings.TrimSuffix(pat, "/**")
			if rel == prefix || strings.HasPrefix(rel, prefix+"/") {
				return true
			}
			continue
		}

		if ok, _ := doublestar.PathMatch(pat, rel); ok {
			return true
		}
		if ok, _ := doublestar.PathMatch(pat, base); ok {
			return true
		}
	}
	return false
}

func isExcluded(path string, excludes []string) bool {
	for _, ex := range excludes {
		if ok, _ := doublestar.PathMatch(ex, path); ok {
			return true
		}
	}
	return false
}

func sanitizeShfmtArgs(raw []string) []string {
	if len(raw) == 0 {
		return nil
	}

	out := make([]string, 0, len(raw))
	for _, arg := range raw {
		arg = strings.TrimSpace(arg)
		if arg == "" {
			continue
		}

		// We control diff/write mode explicitly. If a repo adds these,
		// drop them to avoid confusing behavior.
		switch arg {
		case "-d", "-w":
			logger.Warn(fmt.Sprintf("Ignoring shfmt arg %q (mode is controlled by goneat)", arg))
			continue
		}

		// Refuse path-y tokens from config; shfmt file targets are appended after args.
		if strings.Contains(arg, "/") || strings.Contains(arg, "\\") {
			logger.Warn(fmt.Sprintf("Ignoring shfmt arg %q (looks like a path)", arg))
			continue
		}

		out = append(out, arg)
	}
	return out
}

func issuesFromShfmtOutput(output string) []Issue {
	// shfmt output can be:
	// - unified diffs ("diff -u" / "--- <file>.orig")
	// - parse/validation errors ("file:line:col: message")
	//
	// We parse both formats into Issues to avoid silent skips.
	shfmtErrorPattern := regexp.MustCompile(`^([^:]+):(\d+):(\d+):\s+(.*)$`)

	issues := make([]Issue, 0)
	diffFiles := make(map[string]struct{})

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if match := shfmtErrorPattern.FindStringSubmatch(line); match != nil {
			ln, _ := strconv.Atoi(match[2])
			col, _ := strconv.Atoi(match[3])
			issues = append(issues, Issue{
				File:        filepath.ToSlash(match[1]),
				Line:        ln,
				Column:      col,
				Severity:    SeverityHigh,
				Message:     match[4],
				Category:    CategoryLint,
				SubCategory: "shell:shfmt",
			})
			continue
		}

		// Diff format can include:
		//   diff -u scripts/foo.sh.orig scripts/foo.sh
		if strings.HasPrefix(line, "diff -u ") {
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				name := strings.TrimSuffix(parts[len(parts)-1], ".orig")
				diffFiles[name] = struct{}{}
			}
			continue
		}

		// Or the classic unified header:
		//   --- scripts/foo.sh.orig
		if strings.HasPrefix(line, "--- ") {
			name := strings.TrimSpace(strings.TrimPrefix(line, "--- "))
			name = strings.TrimSuffix(name, ".orig")
			diffFiles[name] = struct{}{}
			continue
		}
	}

	if len(issues) > 0 {
		return issues
	}
	if len(diffFiles) == 0 {
		return nil
	}

	out := make([]Issue, 0, len(diffFiles))
	for f := range diffFiles {
		out = append(out, Issue{
			File:        filepath.ToSlash(f),
			Severity:    SeverityMedium,
			Message:     "shfmt reported formatting differences",
			Category:    CategoryLint,
			SubCategory: "shell:shfmt",
		})
	}
	return out
}

func issuesFromFiles(files []string, message string) []Issue {
	issues := make([]Issue, 0, len(files))
	for _, f := range files {
		issues = append(issues, Issue{
			File:        filepath.ToSlash(f),
			Severity:    SeverityMedium,
			Message:     message,
			Category:    CategoryLint,
			SubCategory: "shell:shfmt",
		})
	}
	return issues
}

func (r *LintAssessmentRunner) runGolangCILintWithMode(target string, config AssessmentConfig, env golangciLintEnvironment, fixMode bool) ([]Issue, error) {
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
		outputArgs, expectedJSON := resolveGolangciOutputArgs(env)
		if expectedJSON {
			args = append(args, outputArgs...)
		} else {
			logger.Warn("golangci-lint JSON output unsupported for detected version; falling back to text parsing")
		}
	}

	// Limit to new issues only when requested
	if config.LintNewFromRev != "" {
		args = append(args, "--new-from-rev", config.LintNewFromRev)
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
			// Check if we should use package mode to avoid mixed-directory errors
			if r.shouldUsePackageMode(goFiles, config) {
				// Use package mode: convert file paths to package paths
				packages := r.detectPackagesFromFiles(goFiles)
				logger.Info(fmt.Sprintf("Using package mode for %d files from %d packages: %v", len(goFiles), len(packages), packages))

				// Convert package paths to ./pkg/... format
				for _, pkg := range packages {
					if pkg == "." {
						args = append(args, "./...")
					} else {
						args = append(args, fmt.Sprintf("./%s/...", pkg))
					}
				}
			} else {
				// Use individual file mode
				logger.Info(fmt.Sprintf("Using individual file mode for %d files", len(goFiles)))
				args = append(args, goFiles...)
			}
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
		// Exit code 1: issues found (normal for check mode)
		// Exit code 5: no go files to analyze (treat as no issues when running new-from-rev)
		if exitCode == 1 {
			// proceed to parse output for issues
		} else if exitCode == 5 && config.LintNewFromRev != "" {
			return []Issue{}, nil
		} else {
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
		// In golangci-lint v1.x, JSON output goes to stdout
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

// detectPackagesFromFiles extracts unique Go package paths from a list of Go files
func (r *LintAssessmentRunner) detectPackagesFromFiles(goFiles []string) []string {
	packages := make(map[string]bool)

	for _, file := range goFiles {
		// Extract package path by finding the directory containing go.mod or main.go
		// For files like "internal/assets/file.go", the package is "internal/assets"
		// For files like "cmd/main.go", the package is "cmd"

		// Get the directory of the file
		dir := filepath.Dir(file)

		// If the directory is "." or empty, it's the root package
		if dir == "." || dir == "" {
			packages["."] = true
			continue
		}

		// Check if this directory has a go.mod file (indicating a module root)
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			// This is a module root, add the module path
			packages[dir] = true
		} else {
			// Not a module root, add the directory as a package
			packages[dir] = true
		}
	}

	// Convert map to slice
	var result []string
	for pkg := range packages {
		result = append(result, pkg)
	}

	return result
}

// shouldUsePackageMode determines if we should use package mode instead of individual files
func (r *LintAssessmentRunner) shouldUsePackageMode(goFiles []string, config AssessmentConfig) bool {
	// Always prefer package mode when specific files are provided to avoid type-check
	// failures from missing sibling files (e.g., staged-only runs on a single file).
	if len(goFiles) > 0 {
		return true
	}

	// Fallback: honor explicit package-mode flag
	return config.PackageMode
}

// verifyGolangciConfig validates golangci-lint config file (Pattern 2: repo root only)
func (r *LintAssessmentRunner) verifyGolangciConfig(target string, env golangciLintEnvironment) error {
	if env.mode == golangciLintModeV1 {
		logger.Info("Skipping golangci-lint config verification: version < 2.0.0 does not support 'config verify'")
		return nil
	}
	if env.detectErr != nil {
		logger.Warn("Skipping golangci-lint config verification due to version detection failure")
		return nil
	}

	// Use standardized config resolver to find working directory
	// For single files, this resolves to the file's directory
	resolver := NewConfigResolver(target)
	workingDir := resolver.GetWorkingDir()

	// Try common golangci-lint config file names (repo root only)
	configNames := []string{".golangci.yml", ".golangci.yaml", ".golangci.toml", ".golangci.json"}
	var configPath string

	for _, name := range configNames {
		candidatePath := filepath.Join(workingDir, name)
		if _, err := os.Stat(candidatePath); err == nil {
			configPath = candidatePath
			break
		}
	}

	if configPath == "" {
		// No config file is OK - golangci-lint will use defaults
		return nil
	}

	// Run config verification in the working directory
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "golangci-lint", "config", "verify")
	cmd.Dir = workingDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("golangci-lint config validation failed: %v\nOutput: %s\n\nPlease check your .golangci.yml file against the golangci-lint v2.4.0 schema.\nFor migration help, see: https://golangci-lint.run/usage/configuration/", err, string(output))
	}

	return nil
}

func (r *LintAssessmentRunner) detectGolangciLintEnvironment() golangciLintEnvironment {
	cmd := exec.Command("golangci-lint", "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return golangciLintEnvironment{
			detectErr: fmt.Errorf("failed to execute golangci-lint --version: %w", err),
		}
	}

	rawOutput := string(output)
	versionToken := extractGolangciLintVersion(rawOutput)
	if versionToken == "" {
		return golangciLintEnvironment{
			detectErr: errors.New("unable to parse golangci-lint version output"),
			raw:       strings.TrimSpace(rawOutput),
		}
	}

	parsed, parseErr := versioning.ParseLenient(versionToken)
	if parseErr != nil {
		return golangciLintEnvironment{
			detectErr: fmt.Errorf("failed to parse golangci-lint version token '%s': %w", versionToken, parseErr),
			raw:       versionToken,
		}
	}

	mode := determineGolangciLintMode(parsed)
	switch mode {
	case golangciLintModeV1:
		logger.Warn("Detected golangci-lint < 2.0.0; using legacy compatibility mode. Please upgrade to v2.4.0+ for best results.")
	case golangciLintModeV2Early:
		logger.Info("Detected golangci-lint v2.0.x–v2.3.x; enabling transitional compatibility flags.")
	case golangciLintModeV24Plus:
		logger.Debug("Detected golangci-lint v2.4.0 or newer; using modern output capabilities.")
	}

	return golangciLintEnvironment{
		mode:    mode,
		raw:     versionToken,
		version: parsed,
	}
}

func resolveGolangciOutputArgs(env golangciLintEnvironment) ([]string, bool) {
	switch env.mode {
	case golangciLintModeV24Plus:
		return []string{"--output.json.path", "stdout"}, true
	case golangciLintModeV2Early:
		return []string{"--out-format", "json"}, true
	case golangciLintModeV1:
		return []string{"--out-format", "json"}, true
	default:
		return []string{"--out-format", "json"}, true
	}
}

var golangciLintVersionPattern = regexp.MustCompile(`(?i)v?\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?`)

func extractGolangciLintVersion(output string) string {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return ""
	}

	match := golangciLintVersionPattern.FindString(trimmed)
	return strings.TrimSpace(match)
}

func determineGolangciLintMode(v *versioning.Version) golangciLintMode {
	if v == nil {
		return golangciLintModeUnknown
	}

	versionStr := v.String()
	cmpV2, err := versioning.Compare(versioning.SchemeSemverFull, versionStr, "2.0.0")
	if err != nil {
		return golangciLintModeUnknown
	}
	if cmpV2 == versioning.ComparisonLess {
		return golangciLintModeV1
	}
	cmpV24, err := versioning.Compare(versioning.SchemeSemverFull, versionStr, "2.4.0")
	if err != nil {
		return golangciLintModeUnknown
	}
	if cmpV24 == versioning.ComparisonLess {
		return golangciLintModeV2Early
	}
	return golangciLintModeV24Plus
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

	// Find the start of JSON by looking for the opening brace
	jsonStart := strings.Index(jsonStr, "{")
	if jsonStart == -1 {
		// No JSON found, fall back to text parsing
		logger.Warn("No JSON found in golangci-lint output, falling back to text parsing")
		return r.parseLintTextOutput(output)
	}

	// Find the end of the JSON object by looking for the closing brace of the root object
	// The JSON structure is: {"Issues":[...],"Report":{...}}
	// We need to find the matching closing brace for the root object
	// Ignore braces inside strings
	braceCount := 0
	jsonEnd := -1
	inString := false
	escaped := false

	for i := jsonStart; i < len(jsonStr); i++ {
		char := jsonStr[i]

		if escaped {
			escaped = false
			continue
		}

		if char == '\\' {
			escaped = true
			continue
		}

		if char == '"' {
			inString = !inString
			continue
		}

		if !inString {
			if char == '{' {
				braceCount++
			} else if char == '}' {
				braceCount--
				if braceCount == 0 {
					jsonEnd = i
					break
				}
			}
		}
	}

	if jsonEnd > jsonStart {
		jsonStr = jsonStr[jsonStart : jsonEnd+1]
	} else {
		// Malformed JSON, fall back to text parsing
		logger.Warn("Malformed JSON in golangci-lint output, falling back to text parsing")
		return r.parseLintTextOutput(output)
	}

	var report LintReport
	if err := json.Unmarshal([]byte(jsonStr), &report); err != nil {
		// If JSON parsing fails, fall back to text parsing
		logger.Warn(fmt.Sprintf("JSON parsing failed, falling back to text parsing: %v", err))
		logger.Debug(fmt.Sprintf("Failed JSON string: %q", jsonStr))
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
			EstimatedTime: HumanReadableDuration(r.estimateLintFixTime(lintIssue.FromLinter, lintIssue.Text)),
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
				EstimatedTime: HumanReadableDuration(r.estimateLintFixTime(linterName, message)),
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

// filterFilesRespectingIgnores filters files to respect .goneatignore patterns
func (r *LintAssessmentRunner) filterFilesRespectingIgnores(files []string, target string, config AssessmentConfig) []string {
	if len(files) == 0 || config.NoIgnore {
		return files
	}
	matcher, err := ignore.NewMatcher(target)
	if err != nil {
		return files
	}

	filtered := make([]string, 0, len(files))
	for _, file := range files {
		rel := file
		if filepath.IsAbs(file) {
			if r, err := filepath.Rel(target, file); err == nil {
				rel = r
			}
		}
		rel = filepath.ToSlash(rel)
		if matcher.IsIgnoredRel(rel) && !matchesForceInclude(rel, config.ForceInclude) {
			continue
		}
		filtered = append(filtered, file)
	}
	return filtered
}

// init registers the lint assessment runner
func init() {
	RegisterAssessmentRunner(CategoryLint, &LintAssessmentRunner{
		commandName: "golangci-lint",
		toolName:    "golangci-lint",
		config:      DefaultLintConfig(),
	})
}
