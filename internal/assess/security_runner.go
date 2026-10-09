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
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"runtime"

	"github.com/fulmenhq/goneat/pkg/ignore"
	"github.com/fulmenhq/goneat/pkg/logger"
	"github.com/fulmenhq/goneat/pkg/pattern"
)

// SecurityAssessmentRunner implements AssessmentRunner for vulnerability/code security scanners
type SecurityAssessmentRunner struct {
	commandName string
}

// NewSecurityAssessmentRunner creates a new security assessment runner
func NewSecurityAssessmentRunner() *SecurityAssessmentRunner {
	return &SecurityAssessmentRunner{commandName: "security"}
}

// parseIgnorePatternsForGosec parses .goneatignore and .gitignore files to extract
// directory patterns suitable for gosec's -exclude-dir option.
// Patterns are converted from glob to regex syntax and validated before use.
func parseIgnorePatternsForGosec(moduleRoot string) []string {
	var rawPatterns []string

	ignoreFiles := []string{
		filepath.Join(moduleRoot, ".goneatignore"),
		filepath.Join(moduleRoot, ".gitignore"),
		filepath.Join(moduleRoot, ".goneat", "ignore"),
	}

	dirPatterns := []string{
		"node_modules",
		"vendor",
		"dist",
		"build",
		"bin",
		".git",
		".goneat",
		"testdata",
		"fixtures",
		"coverage",
		"logs",
		"tmp",
		"temp",
	}

	for _, ignoreFile := range ignoreFiles {
		// #nosec G304 -- ignoreFile from controlled list of known ignore files (.goneatignore, .gitignore, .goneat/ignore)
		if file, err := os.Open(ignoreFile); err == nil {
			defer func() {
				_ = file.Close()
			}()
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}

				if strings.HasSuffix(line, "/") || strings.Contains(line, "*/") {
					if !sliceContainsString(rawPatterns, line) {
						rawPatterns = append(rawPatterns, line)
					}
				}
			}
		}
	}

	for _, pattern_ := range dirPatterns {
		if !sliceContainsString(rawPatterns, pattern_) {
			rawPatterns = append(rawPatterns, pattern_)
		}
	}

	regexes, decisions := pattern.ToGosecExcludeRegexDecisions(rawPatterns)
	skipCount := 0
	var skipReasons []string
	for _, d := range decisions {
		if d.Accepted {
			logger.Debug(fmt.Sprintf("gosec exclude pattern converted: raw=%q normalized=%q regex=%q",
				d.Raw, d.Normalized, d.Regex))
			continue
		}
		skipCount++
		skipReasons = append(skipReasons, d.Reason)
		logger.Debug(fmt.Sprintf("gosec exclude pattern skipped: raw=%q normalized=%q reason=%s",
			d.Raw, d.Normalized, d.Reason))
	}
	if skipCount > 0 {
		logger.Warn(fmt.Sprintf("gosec exclude conversion completed with skips: total=%d converted=%d skipped=%d reasons=%v",
			len(rawPatterns), len(regexes), skipCount, skipReasons))
	}

	return regexes
}

// sliceContainsString checks if a slice contains a string
func sliceContainsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Assess implements AssessmentRunner.Assess
func (r *SecurityAssessmentRunner) Assess(ctx context.Context, target string, config AssessmentConfig) (*AssessmentResult, error) {
	start := time.Now()

	// Determine module root (for Go tools)
	moduleRoot, _ := r.findModuleRoot(target)
	if moduleRoot == "" {
		moduleRoot = target
	}

	var issues []Issue
	var allSuppressions []Suppression
	metrics := make(map[string]interface{})

	// Build adapters list via registry (respects enable/tools filters)
	type res struct {
		issues       []Issue
		suppressions []Suppression
		err          error
		name         string
		warnings     map[string][]json.RawMessage
		metadata     map[string]interface{}
	}
	adapters, admissions, toolErrors := GetSecurityToolRegistry().selectAdmissions(config, r, moduleRoot)
	metrics["tool_admissions"] = admissions
	metrics["tools_started"] = len(adapters)
	metrics["tools_admission_failed"] = len(toolErrors)
	metrics["tools_completed"] = 0
	metrics["tools_failed"] = 0
	skipped := 0
	for _, admission := range admissions {
		if admission.State != "selected" && (admission.State != "unavailable" || !admission.Explicit) {
			skipped++
		}
	}
	metrics["tools_skipped"] = skipped
	if len(adapters) == 0 && len(toolErrors) == 0 {
		const reason = "no applicable security tool found in PATH (gosec, govulncheck, gitleaks, cargo-audit, cargo-deny)"
		logger.Warn("security assessment skipped: " + reason)
		return &AssessmentResult{
			CommandName:   r.commandName,
			Category:      CategorySecurity,
			Success:       true,
			ExecutionTime: HumanReadableDuration(time.Since(start)),
			Issues:        []Issue{},
			SkipReason:    reason,
			Metrics:       metrics,
		}, nil
	}
	ranGosec := false
	for _, a := range adapters {
		if a.Name() == "gosec" {
			ranGosec = true
			break
		}
	}

	resultsCh := make(chan res, len(adapters))
	for _, a := range adapters {
		tool := a
		go func() {
			logger.Info(fmt.Sprintf("Running %s security tool", tool.Name()))
			if withMetadata, ok := tool.(securityToolWithMetadata); ok {
				iss, metadata, err := withMetadata.RunWithMetadata(ctx)
				resultsCh <- res{issues: iss, metadata: metadata, err: err, name: tool.Name()}
			} else if withWarnings, ok := tool.(securityToolWithWarnings); ok {
				iss, warnings, err := withWarnings.RunWithWarnings(ctx)
				resultsCh <- res{issues: iss, warnings: warnings, err: err, name: tool.Name()}
			} else if withSupp, ok := tool.(SecurityToolWithSuppressions); ok && config.TrackSuppressions {
				iss, supps, err := withSupp.RunWithSuppressions(ctx)
				resultsCh <- res{issues: iss, suppressions: supps, err: err, name: tool.Name()}
			} else {
				iss, err := tool.Run(ctx)
				resultsCh <- res{issues: iss, err: err, name: tool.Name()}
			}
		}()
	}
	results := make([]res, 0, len(adapters))
	for i := 0; i < len(adapters); i++ {
		results = append(results, <-resultsCh)
	}
	sort.Slice(results, func(i, j int) bool { return results[i].name < results[j].name })
	completed, failed := 0, 0
	toolWarnings := make(map[string]map[string][]json.RawMessage)
	toolReports := make(map[string]map[string]interface{})
	for _, rres := range results {
		// A result with an execution error can still contain trustworthy findings
		// and suppressions. Presentation filtering must not erase the error.
		issues = append(issues, rres.issues...)
		allSuppressions = append(allSuppressions, rres.suppressions...)
		if len(rres.warnings) > 0 {
			toolWarnings[rres.name] = rres.warnings
		}
		if rres.metadata != nil {
			toolReports[rres.name] = rres.metadata
		}
		state := "completed_clean"
		warningCount := 0
		for _, warnings := range rres.warnings {
			warningCount += len(warnings)
		}
		policyFindings, _ := rres.metadata["policy_findings_reported"].(bool)
		if len(rres.issues) > 0 || len(rres.suppressions) > 0 || warningCount > 0 || policyFindings {
			state = "completed_findings"
		}
		if rres.err != nil {
			state = "failed_execution"
			failed++
			toolErrors = append(toolErrors, fmt.Errorf("%s: %w", rres.name, rres.err))
		} else {
			completed++
		}
		for i := range admissions {
			if admissions[i].Tool == rres.name {
				admissions[i].State = state
			}
		}
	}
	metrics["tool_admissions"] = admissions
	metrics["tools_completed"] = completed
	metrics["tools_failed"] = failed
	if len(toolWarnings) > 0 {
		metrics["tool_warnings"] = toolWarnings
	}
	if len(toolReports) > 0 {
		metrics["tool_reports"] = toolReports
	}

	issues, allSuppressions = r.filterToAssessmentRoot(target, issues, allSuppressions)

	// Filter out known noise paths (e.g., test fixtures) from security scans
	issues = r.filterSecurityNoise(issues, config)

	// Apply repository security suppression policy (e.g., required git hook perms)
	var policySuppressions []Suppression
	issues, policySuppressions = r.applySecuritySuppressionPolicy(issues)

	// Basic metrics
	if ranGosec {
		metrics["gosec_shards"] = lastShardCount // set by runGosec
		metrics["gosec_pool_size"] = lastPoolSize
	}
	metrics["tools_started"] = len(adapters)

	// Add suppression metrics if tracking is enabled
	var suppSummary SuppressionSummary
	if config.TrackSuppressions && (len(allSuppressions) > 0 || len(policySuppressions) > 0) {
		// Optional Git enrichment (age/author) when inexpensive/available
		merged := append(allSuppressions, policySuppressions...)
		merged = EnrichWithGitInfo(merged)
		metrics["suppressions_found"] = len(merged)
		suppSummary = GenerateSummary(merged)
		metrics["suppression_summary"] = suppSummary
	}

	result := &AssessmentResult{
		CommandName:   r.commandName,
		Category:      CategorySecurity,
		Success:       len(toolErrors) == 0,
		ExecutionTime: HumanReadableDuration(time.Since(start)),
		Issues:        issues,
		Metrics:       metrics,
	}
	if len(toolErrors) > 0 {
		sort.Slice(toolErrors, func(i, j int) bool { return toolErrors[i].Error() < toolErrors[j].Error() })
		result.Error = errors.Join(toolErrors...).Error()
	}

	// Store suppressions for later use in CategoryResult
	if config.TrackSuppressions {
		result.Metrics["_suppressions"] = append(allSuppressions, policySuppressions...)
		// Also attach a structured suppression report to downstream category result
		// (engine will copy Metrics and can surface SuppressionReport in CategoryResult)
		// Here we only compute the summary; CategoryResult population happens in engine.
		_ = suppSummary
	}

	return result, nil
}

// CanRunInParallel implements AssessmentRunner.CanRunInParallel
func (r *SecurityAssessmentRunner) CanRunInParallel() bool { return false }

// GetCategory implements AssessmentRunner.GetCategory
func (r *SecurityAssessmentRunner) GetCategory() AssessmentCategory { return CategorySecurity }

// GetEstimatedTime implements AssessmentRunner.GetEstimatedTime
func (r *SecurityAssessmentRunner) GetEstimatedTime(target string) time.Duration {
	return 5 * time.Second
}

// IsAvailable implements AssessmentRunner.IsAvailable. Which security tools
// apply depends on the target (Go scanners, cargo-audit and cargo-deny for
// Cargo projects, gitleaks for secrets), so the category is always
// available and Assess reports a skip when none of them can run.
func (r *SecurityAssessmentRunner) IsAvailable() bool {
	return true
}

func (r *SecurityAssessmentRunner) toolAvailable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// applySecuritySuppressionPolicy filters issues that match documented repository policy exceptions
// and returns the filtered issues along with synthetic suppressions.
func (r *SecurityAssessmentRunner) applySecuritySuppressionPolicy(issues []Issue) ([]Issue, []Suppression) {
	var out []Issue
	var supps []Suppression
	for _, is := range issues {
		// Suppress gosec G302/G306 in cmd/hooks.go: git hooks must be executable (0700)
		if strings.HasSuffix(is.File, filepath.ToSlash("cmd/hooks.go")) {
			if strings.Contains(is.Message, "gosec(G302)") || strings.Contains(is.Message, "gosec(G306)") {
				rule := "G302"
				reason := "Git hooks require exec permissions (0700)"
				if strings.Contains(is.Message, "G306") {
					rule = "G306"
					reason = "Git hooks require exec permissions when writing hook files"
				}
				supps = append(supps, Suppression{
					Tool:     "gosec",
					RuleID:   rule,
					File:     is.File,
					Line:     is.Line,
					Reason:   reason,
					Severity: is.Severity,
					Syntax:   "#nosec " + rule + " - policy exception: git hooks executable",
				})
				continue // drop this issue
			}
		}
		out = append(out, is)
	}
	return out, supps
}

// findModuleRoot finds the Go module root directory (best-effort)
func (r *SecurityAssessmentRunner) findModuleRoot(startDir string) (string, error) {
	current := startDir
	for {
		if _, err := os.Stat(filepath.Join(current, "go.mod")); err == nil {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return "", fmt.Errorf("go.mod not found")
}

// runGosec executes gosec and parses JSON output into issues.
// Phase B: For performance in larger repos, shard by directories and run with a bounded worker pool.
// Assumption: Single-process coordination; we honor assess's concurrency percent to size the pool.
func (r *SecurityAssessmentRunner) runGosec(ctx context.Context, moduleRoot string, config AssessmentConfig) ([]Issue, []Suppression, error) {
	// Build target directories
	var dirs []string
	var discoveryErrors []error
	if len(config.IncludeFiles) > 0 {
		dirs = r.uniqueDirs(config.IncludeFiles)
	} else {
		// Discover Go package directories across multi-module repos
		discoveredModules := false
		listedAnyModule := false
		commandDiscoveryFailed := false
		nonCommandDiscoveryFailed := false
		moduleDirs, discoveryErr := r.findModuleDirs(moduleRoot, config)
		if discoveryErr != nil {
			nonCommandDiscoveryFailed = true
			discoveryErrors = append(discoveryErrors, fmt.Errorf("gosec module discovery: %w", discoveryErr))
		}
		if discoveryErr == nil && len(moduleDirs) > 0 {
			discoveredModules = true
			pkgSet := make(map[string]struct{})
			for _, mdir := range moduleDirs {
				pkgs, err := r.listGoPackageDirs(moduleRoot, mdir, config)
				if err != nil {
					discoveryErrors = append(discoveryErrors, fmt.Errorf("gosec package discovery for %s: %w", mdir, err))
					var commandErr *gosecPackageDiscoveryCommandError
					if errors.As(err, &commandErr) {
						commandDiscoveryFailed = true
					} else {
						nonCommandDiscoveryFailed = true
					}
					continue
				}
				listedAnyModule = true
				for _, p := range pkgs {
					// Convert to relative from moduleRoot if possible for nicer args
					rel := p
					if rel2, err2 := filepath.Rel(moduleRoot, p); err2 == nil {
						rel = rel2
					}
					pkgSet[rel] = struct{}{}
				}
			}
			for p := range pkgSet {
				dirs = append(dirs, p)
			}
		}
		if len(dirs) == 0 && !nonCommandDiscoveryFailed &&
			(len(discoveryErrors) == 0 || commandDiscoveryFailed) && (!discoveredModules || !listedAnyModule) {
			// Retain the historical best-effort shard for command failures only.
			// Discovery errors remain failures even if this shard returns findings.
			dirs = []string{"./..."}
		}
	}
	sort.Strings(dirs)

	// Determine worker pool size from concurrency percent (default 80%)
	workers := config.Concurrency
	if workers <= 0 {
		// map percent to a minimum of 1, based on CPU cores
		cores := runtime.NumCPU()
		if config.ConcurrencyPercent > 0 {
			workers = (cores * config.ConcurrencyPercent) / 100
		}
		if workers < 1 {
			workers = 1
		}
	}
	if workers > len(dirs) {
		workers = len(dirs)
	}
	// expose shard/pool metrics via package-level variables (single-process assumption)
	lastShardCount = len(dirs)
	lastPoolSize = workers
	if len(dirs) == 0 {
		return nil, nil, errors.Join(append(discoveryErrors, errors.New("gosec did not execute any in-scope package shards"))...)
	}

	type shardResult struct {
		issues       []Issue
		suppressions []Suppression
		err          error
		dir          string
	}
	results := make(chan shardResult, len(dirs))
	excludeDirs := parseIgnorePatternsForGosec(moduleRoot)

	// Simple bounded worker pool via semaphore
	sem := make(chan struct{}, workers)
	for _, d := range dirs {
		select {
		case <-ctx.Done():
			results <- shardResult{dir: d, err: ctx.Err()}
			continue
		case sem <- struct{}{}:
		}
		go func(dirArg string) {
			defer func() { <-sem }()
			args := []string{"-fmt=json"}
			if config.TrackSuppressions {
				args = append(args, "-track-suppressions")
			}

			logger.Debug(fmt.Sprintf("gosec exclude dirs for %s: %v", dirArg, excludeDirs))
			for _, dir := range excludeDirs {
				args = append(args, "-exclude-dir", dir)
			}

			args = append(args, dirArg)

			runOnce := func(runCtx context.Context) ([]byte, []byte, error) {
				cmd := exec.CommandContext(runCtx, "gosec", args...)
				cmd.Dir = moduleRoot
				var stdout bytes.Buffer
				var stderr bytes.Buffer
				cmd.Stdout = &stdout
				cmd.Stderr = &stderr
				err := cmd.Run()
				return stdout.Bytes(), stderr.Bytes(), err
			}

			rctx, cancel := r.effectiveToolContext(ctx, config.Timeout, config.SecurityGosecTimeout)
			defer cancel()
			stdout, stderr, err := runOnce(rctx)
			primary := stdout
			if len(bytes.TrimSpace(primary)) == 0 {
				primary = stderr
			}
			report := r.parseGosecScanReport(primary)
			lastReport := report
			iss, supps := report.issues, report.suppressions
			// Keep the existing malformed-nonempty retry bound. Earlier terminal
			// failures and trustworthy evidence survive even a later valid report.
			var attemptErrors []error
			attemptErrors = append(attemptErrors, gosecExecutionError(report, err, rctx.Err()))
			if !report.complete && len(bytes.TrimSpace(primary)) > 0 && rctx.Err() == nil {
				backoff := 200 * time.Millisecond
				maxRetries := 2
				for i := 0; i < maxRetries; i++ {
					select {
					case <-rctx.Done():
						attemptErrors = append(attemptErrors, rctx.Err())
					case <-time.After(backoff):
					}
					if rctx.Err() != nil {
						break
					}
					stdout2, stderr2, retryErr := runOnce(rctx)
					primary2 := stdout2
					if len(bytes.TrimSpace(primary2)) == 0 {
						primary2 = stderr2
					}
					retryReport := r.parseGosecScanReport(primary2)
					lastReport = retryReport
					iss = append(iss, retryReport.issues...)
					supps = append(supps, retryReport.suppressions...)
					attemptErrors = append(attemptErrors, gosecExecutionError(retryReport, retryErr, rctx.Err()))
					if retryReport.complete || rctx.Err() != nil {
						break
					}
					backoff *= 2
				}
			}
			if !lastReport.complete && len(bytes.TrimSpace(primary)) > 0 && rctx.Err() == nil {
				if path := persistGosecParseFailure(dirArg, stdout, stderr); path != "" {
					logger.Warn(fmt.Sprintf("gosec(%s) incomplete report after bounded retries (debug: %s)", dirArg, path))
				}
			}
			iss, supps = uniqueGosecEvidence(iss, supps)
			// Presentation scoping applies to every attempt, never its errors.
			if len(config.IncludeFiles) > 0 {
				var filtered []Issue
				for _, issue := range iss {
					if pathMatchesAny(issue.File, config.IncludeFiles) {
						filtered = append(filtered, issue)
					}
				}
				iss = filtered
			}
			results <- shardResult{dir: dirArg, issues: iss, suppressions: supps, err: errors.Join(attemptErrors...)}
		}(d)
	}

	// Collect every scheduled shard, including canceled and failed shards.
	var shardResults []shardResult
	for range dirs {
		shardResults = append(shardResults, <-results)
	}
	sort.Slice(shardResults, func(i, j int) bool { return shardResults[i].dir < shardResults[j].dir })
	var allIssues []Issue
	var allSuppressions []Suppression
	for _, result := range shardResults {
		if result.err != nil {
			discoveryErrors = append(discoveryErrors, fmt.Errorf("gosec shard %s: %w", result.dir, result.err))
		}
		allIssues = append(allIssues, result.issues...)
		allSuppressions = append(allSuppressions, result.suppressions...)
	}
	close(results)
	return allIssues, allSuppressions, errors.Join(discoveryErrors...)
}

// Package-level metrics (single-process assumption; not exported)
var (
	lastShardCount int
	lastPoolSize   int
)

// gosecPackageDiscoveryCommandError identifies errors originating at the go-list
// command boundary, without classifying path-mapping or module-walk failures as
// reasons to launch a broader fallback. The original error remains inspectable.
type gosecPackageDiscoveryCommandError struct {
	err error
}

func (e *gosecPackageDiscoveryCommandError) Error() string { return e.err.Error() }
func (e *gosecPackageDiscoveryCommandError) Unwrap() error { return e.err }

// listGoPackageDirs returns absolute directories for all in-scope packages under moduleRoot.
func (r *SecurityAssessmentRunner) listGoPackageDirs(scopeRoot, moduleRoot string, config AssessmentConfig) ([]string, error) {
	// go list emits absolute directories even when assessment starts at ".".
	// Normalize the scope once, without changing successful containment/ignore
	// decisions or treating a mapping failure as an empty successful discovery.
	scopeRoot, err := filepath.Abs(scopeRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve gosec scope root: %w", err)
	}
	cmd := exec.Command("go", "list", "-f", "{{.Dir}}", "./...")
	cmd.Dir = moduleRoot
	out, err := cmd.Output()
	if err != nil {
		return nil, &gosecPackageDiscoveryCommandError{err: err}
	}
	matcher := r.newIgnoreMatcher(scopeRoot, config)
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var dirs []string
	for _, line := range lines {
		d := strings.TrimSpace(line)
		if d == "" {
			continue
		}
		rel, err := filepath.Rel(scopeRoot, d)
		if err != nil {
			return nil, fmt.Errorf("map gosec package directory to scope: %w", err)
		}
		rel = filepath.ToSlash(rel)
		if r.isIgnoredSecurityPath(rel, true, matcher, config) {
			continue
		}
		dirs = append(dirs, d)
	}
	return dirs, nil
}

// findModuleDirs finds all directories containing a go.mod starting from root (multi-module aware)
func (r *SecurityAssessmentRunner) findModuleDirs(root string, config AssessmentConfig) ([]string, error) {
	var dirs []string
	matcher := r.newIgnoreMatcher(root, config)
	// Always include root if it has go.mod or go.work references modules
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
		dirs = append(dirs, root)
	}

	// Walk and collect go.mod holders, pruning ignored directories before package discovery.
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			rel, relErr := filepath.Rel(root, path)
			if relErr == nil && rel != "." && r.isIgnoredSecurityPath(rel, true, matcher, config) {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Base(path) == "go.mod" {
			modDir := filepath.Dir(path)
			// Ignore the root since we already added it
			if modDir != root {
				dirs = append(dirs, modDir)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(dirs) == 0 {
		// Try to include root at minimum
		dirs = append(dirs, root)
	}
	return dirs, nil
}

func (r *SecurityAssessmentRunner) newIgnoreMatcher(root string, config AssessmentConfig) *ignore.Matcher {
	if config.NoIgnore {
		return nil
	}
	matcher, err := ignore.NewMatcher(root)
	if err != nil {
		logger.Warn(fmt.Sprintf("security ignore matcher unavailable: %v", err))
		return nil
	}
	return matcher
}

func (r *SecurityAssessmentRunner) isIgnoredSecurityPath(rel string, isDir bool, matcher *ignore.Matcher, config AssessmentConfig) bool {
	if config.NoIgnore || matcher == nil {
		return false
	}
	rel = filepath.ToSlash(rel)
	if matchesForceInclude(rel, config.ForceInclude) || forceIncludeDescendsFrom(rel, config.ForceInclude) {
		return false
	}
	if isDir {
		return matcher.IsIgnoredDirRel(rel)
	}
	return matcher.IsIgnoredRel(rel)
}

func forceIncludeDescendsFrom(rel string, patterns []string) bool {
	if len(patterns) == 0 {
		return false
	}
	rel = strings.TrimSuffix(filepath.ToSlash(rel), "/")
	if rel == "" || rel == "." {
		return true
	}
	for _, raw := range patterns {
		pat := filepath.ToSlash(strings.TrimSpace(raw))
		if pat == "" {
			continue
		}
		for strings.HasPrefix(pat, "./") {
			pat = strings.TrimPrefix(pat, "./")
		}
		pat = strings.TrimSuffix(pat, "/**")
		if strings.ContainsAny(pat, "*?[") {
			prefix := forceIncludeStaticDirPrefix(pat)
			if prefix != "" && (prefix == rel || strings.HasPrefix(prefix, rel+"/")) {
				return true
			}
			continue
		}
		if pat == rel || strings.HasPrefix(pat, rel+"/") {
			return true
		}
	}
	return false
}

func forceIncludeStaticDirPrefix(pattern string) string {
	pattern = strings.TrimPrefix(filepath.ToSlash(pattern), "./")
	wild := strings.IndexAny(pattern, "*?[")
	if wild < 0 {
		return strings.TrimSuffix(pattern, "/")
	}
	prefix := pattern[:wild]
	if prefix == "" {
		return ""
	}
	if strings.HasSuffix(prefix, "/") {
		return strings.TrimSuffix(prefix, "/")
	}
	slash := strings.LastIndex(prefix, "/")
	if slash < 0 {
		return ""
	}
	return strings.TrimSuffix(prefix[:slash], "/")
}

// parseGosecOutput converts gosec JSON to issues
func (r *SecurityAssessmentRunner) parseGosecOutput(output []byte) ([]Issue, error) {
	issues, _, err := r.parseGosecOutputWithSuppressions(output)
	return issues, err
}

// parseGosecOutputWithSuppressions converts gosec JSON to issues and tracks suppressions
func (r *SecurityAssessmentRunner) parseGosecOutputWithSuppressions(output []byte) ([]Issue, []Suppression, error) {
	type gosecIssue struct {
		Severity     string      `json:"severity"`
		Details      string      `json:"details"`
		File         string      `json:"file"`
		Code         string      `json:"code"`
		Line         interface{} `json:"line"` // tolerate string or number
		RuleID       string      `json:"rule_id"`
		NoSec        bool        `json:"nosec"`
		Suppressions []struct {
			Kind          string `json:"kind"`
			Justification string `json:"justification"`
		} `json:"suppressions"`
	}
	type gosecSuppression struct {
		RuleID string `json:"rule_id"`
		File   string `json:"file"`
		Line   int    `json:"line"`
		Column int    `json:"column"`
		Reason string `json:"justification,omitempty"`
	}
	type gosecReport struct {
		Issues       []gosecIssue       `json:"Issues"`
		Suppressions []gosecSuppression `json:"Suppressions,omitempty"`
	}

	var report gosecReport
	if err := json.Unmarshal(output, &report); err != nil {
		// Some versions print extra text around JSON; try to extract a valid JSON object
		cleaned := extractFirstJSONObjectBytes(output)
		if len(cleaned) == 0 {
			return nil, nil, fmt.Errorf("failed to parse gosec output: %v", err)
		}
		if uerr := json.Unmarshal(cleaned, &report); uerr != nil {
			return nil, nil, fmt.Errorf("failed to parse cleaned gosec output: %v", uerr)
		}
	}

	var issues []Issue
	var suppressions []Suppression
	for _, gi := range report.Issues {
		sev := strings.ToLower(strings.TrimSpace(gi.Severity))
		mapped := SeverityLow
		switch sev {
		case "critical":
			mapped = SeverityCritical
		case "high":
			mapped = SeverityHigh
		case "medium":
			mapped = SeverityMedium
		case "low":
			mapped = SeverityLow
		}
		// parse line flexibly
		lineNum := 0
		switch v := gi.Line.(type) {
		case float64:
			lineNum = int(v)
		case int:
			lineNum = v
		case string:
			if n, perr := fmt.Sscanf(v, "%d", &lineNum); n < 1 || perr != nil {
				lineNum = 0
			}
		default:
			lineNum = 0
		}
		if gi.NoSec || len(gi.Suppressions) > 0 {
			if len(gi.Suppressions) == 0 {
				suppressions = append(suppressions, Suppression{
					Tool: "gosec", RuleID: gi.RuleID, File: gi.File, Line: lineNum,
					Severity: mapped, Syntax: "#nosec " + gi.RuleID,
				})
			}
			for _, source := range gi.Suppressions {
				syntax := source.Kind
				if source.Kind == "inSource" {
					syntax = "#nosec " + gi.RuleID
				}
				suppressions = append(suppressions, Suppression{
					Tool: "gosec", RuleID: gi.RuleID, File: gi.File, Line: lineNum,
					Severity: mapped, Syntax: syntax, Reason: source.Justification,
					Metadata: map[string]interface{}{"kind": source.Kind},
				})
			}
			continue
		}

		issues = append(issues, Issue{
			File:        gi.File,
			Line:        lineNum,
			Severity:    mapped,
			Message:     fmt.Sprintf("gosec(%s): %s", gi.RuleID, gi.Details),
			Category:    CategorySecurity,
			SubCategory: "code",
			AutoFixable: false,
		})
	}

	// Convert gosec suppressions to our format
	for _, gs := range report.Suppressions {
		supp := Suppression{
			Tool:     "gosec",
			RuleID:   gs.RuleID,
			File:     gs.File,
			Line:     gs.Line,
			Column:   gs.Column,
			Reason:   gs.Reason,
			Severity: r.mapGosecSeverity(gs.RuleID), // Map based on rule
		}
		// Construct syntax from available info
		if gs.Reason != "" {
			supp.Syntax = fmt.Sprintf("#nosec %s - %s", gs.RuleID, gs.Reason)
		} else {
			supp.Syntax = fmt.Sprintf("#nosec %s", gs.RuleID)
		}
		suppressions = append(suppressions, supp)
	}

	return issues, suppressions, nil
}

// mapGosecSeverity estimates severity based on rule ID
func (r *SecurityAssessmentRunner) mapGosecSeverity(ruleID string) IssueSeverity {
	// Based on gosec rule categories
	switch {
	case strings.HasPrefix(ruleID, "G1"): // General
		return SeverityMedium
	case strings.HasPrefix(ruleID, "G2"): // SQL injection
		return SeverityHigh
	case strings.HasPrefix(ruleID, "G3"): // File/Path operations
		return SeverityMedium
	case strings.HasPrefix(ruleID, "G4"): // Crypto
		return SeverityHigh
	case strings.HasPrefix(ruleID, "G5"): // Blocklisted imports
		return SeverityMedium
	case strings.HasPrefix(ruleID, "G6"): // Memory/concurrency
		return SeverityLow
	default:
		return SeverityMedium
	}
}

// runGovulncheck executes govulncheck and parses JSON-lines output into issues
func (r *SecurityAssessmentRunner) runGovulncheck(ctx context.Context, moduleRoot string, config AssessmentConfig) ([]Issue, error) {
	issues, _, err := r.runGovulncheckWithMetadata(ctx, moduleRoot, config)
	return issues, err
}

func (r *SecurityAssessmentRunner) runGovulncheckWithMetadata(ctx context.Context, moduleRoot string, config AssessmentConfig) ([]Issue, map[string]interface{}, error) {
	// govulncheck emits a stream of potentially multiline JSON objects.
	// When scoped, prefer limiting to impacted package directories.
	args := []string{"-json"}
	if len(config.IncludeFiles) > 0 {
		// Package directories are local patterns, not bare import paths. Compare
		// against an absolute root without changing cmd.Dir or resolving symlinks.
		absModuleRoot, err := filepath.Abs(moduleRoot)
		if err != nil {
			return nil, nil, fmt.Errorf("resolve govulncheck module scope: %w", err)
		}
		dirs := r.uniqueDirs(config.IncludeFiles)
		type packageSelection struct {
			dir     string
			pattern string
		}
		var selections []packageSelection
		rootSelected := false
		for _, d := range dirs {
			// A selected root file denotes the module root under cmd.Dir, even
			// when the assessment caller is working in a different directory.
			if d == "." {
				selections = append(selections, packageSelection{dir: absModuleRoot, pattern: "./..."})
				rootSelected = true
				continue
			}
			absDir, err := filepath.Abs(d)
			if err != nil {
				return nil, nil, fmt.Errorf("resolve govulncheck package scope: %w", err)
			}
			rel, err := filepath.Rel(absModuleRoot, absDir)
			if err != nil {
				return nil, nil, fmt.Errorf("map govulncheck package scope: %w", err)
			}
			if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return nil, nil, errors.New("govulncheck included package is outside module root")
			}
			if rel == "." {
				selections = append(selections, packageSelection{dir: absDir, pattern: "./..."})
				rootSelected = true
			} else {
				selections = append(selections, packageSelection{dir: absDir, pattern: "./" + filepath.ToSlash(rel)})
			}
		}
		// Validate every mapping before omitting any directory. When ./... is
		// already selected, non-package directories add no Go coverage and would
		// make the producer reject a mixed documentation/code selection. Retain
		// any .go entry (including test files and symlinks) for the Go loader to
		// validate; never turn a read error into evidence of package absence.
		for _, selected := range selections {
			if rootSelected && selected.pattern != "./..." {
				if err := ctx.Err(); err != nil {
					return nil, nil, err
				}
				entries, err := os.ReadDir(selected.dir)
				if err != nil {
					return nil, nil, fmt.Errorf("read govulncheck selected directory: %w", err)
				}
				hasGo := false
				for _, entry := range entries {
					if strings.HasSuffix(entry.Name(), ".go") {
						hasGo = true
						break
					}
				}
				if !hasGo {
					continue
				}
			}
			args = append(args, selected.pattern)
		}
		if len(dirs) == 0 {
			args = append(args, "./...")
		}
	} else {
		args = append(args, "./...")
	}
	rctx, cancel := r.effectiveToolContext(ctx, config.Timeout, config.SecurityGovulncheckTimeout)
	defer cancel()
	// Parser failure must be able to stop a producer even when the caller has
	// configured no deadline. This adds cleanup cancellation, not a new timeout.
	rctx, stop := context.WithCancel(rctx)
	defer stop()
	cmd := exec.CommandContext(rctx, "govulncheck", args...)
	cmd.Dir = moduleRoot
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("start govulncheck: %w", err)
	}

	// Drain stderr without logging raw network/tool output or abandoning a reader.
	stderrDone := make(chan error, 1)
	go func() {
		_, err := io.Copy(io.Discard, stderr)
		stderrDone <- err
	}()
	report, parseErr := readGovulnStream(moduleRoot, stdout)
	if parseErr != nil {
		// Stop a malformed producer before draining/reaping it. The parser error
		// remains the primary cause, not a fabricated successful empty scan.
		stop()
	}
	_, drainErr := io.Copy(io.Discard, stdout)
	stderrErr := <-stderrDone
	waitErr := cmd.Wait()
	// In JSON mode findings are reported with exit 0, not text-mode exit 3.
	return report.finish(errors.Join(parseErr, drainErr, stderrErr, waitErr, rctx.Err()))
}

// parseGovulnEventLine parses a single govulncheck JSON event line into an Issue.
// Returns (nil, false) for non-finding or non-JSON lines.
func (r *SecurityAssessmentRunner) parseGovulnEventLine(moduleRoot, line string) (*Issue, bool) {
	var event map[string]json.RawMessage
	if json.Unmarshal([]byte(line), &event) != nil {
		return nil, false
	}
	iss, err := parseGovulnFinding(moduleRoot, event["finding"])
	return iss, err == nil && iss != nil
}

// runGitleaks executes gitleaks and parses JSON output into issues
func (r *SecurityAssessmentRunner) runGitleaks(ctx context.Context, moduleRoot string, config AssessmentConfig) ([]Issue, error) {
	// Clean and validate moduleRoot path to prevent path traversal
	moduleRoot = filepath.Clean(moduleRoot)
	// Prefer reporting to stdout in JSON; when scoped, limit source to nearest common ancestor
	source := moduleRoot
	if len(config.IncludeFiles) > 0 {
		if ca := nearestCommonAncestor(config.IncludeFiles); ca != "" {
			source = ca
		}
	}
	// A dedicated findings code separates a completed finding report from the
	// tool's partial-scan/report-write failure exit 1.
	args := []string{"detect", "--no-banner", "--report-format", "json", "--report-path", "-", "--source", source, "--exit-code", "42"}
	rctx, cancel := r.effectiveToolContext(ctx, config.Timeout, 0)
	defer cancel()
	rctx, stop := context.WithCancel(rctx)
	defer stop()
	cmd := exec.CommandContext(rctx, "gitleaks", args...) // #nosec G204
	cmd.Dir = moduleRoot

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start gitleaks: %w", err)
	}

	// Drain without echoing secret-bearing scanner output.
	stderrDone := make(chan error, 1)
	go func() {
		_, err := io.Copy(io.Discard, stderr)
		stderrDone <- err
	}()
	issues, parseErr := r.parseGitleaksStream(stdout)
	if parseErr != nil {
		stop()
	}
	_, drainErr := io.Copy(io.Discard, stdout)
	stderrErr := <-stderrDone
	waitErr := cmd.Wait()
	if waitErr != nil && parseErr == nil && drainErr == nil && stderrErr == nil && rctx.Err() == nil && len(issues) > 0 {
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) && exitErr.ExitCode() == 42 {
			waitErr = nil
		}
	} else if waitErr == nil && len(issues) > 0 {
		waitErr = errors.New("gitleaks returned its clean exit with a finding-bearing report")
	}

	// If scoped, post-filter to included files only
	if len(config.IncludeFiles) > 0 && len(issues) > 0 {
		var filtered []Issue
		for _, is := range issues {
			if pathMatchesAny(is.File, config.IncludeFiles) {
				filtered = append(filtered, is)
			}
		}
		issues = filtered
	}

	return issues, errors.Join(parseErr, drainErr, stderrErr, waitErr, rctx.Err())
}

func (r *SecurityAssessmentRunner) mapGitleaksFinding(m map[string]interface{}) *Issue {
	// Gitleaks JSON has varied schemas depending on version/config; best-effort mapping
	file := getString(m, []string{"File", "file"})
	desc := getString(m, []string{"Description", "description", "Rule", "RuleID", "rule"})
	line := getInt(m, []string{"StartLine", "Line", "line"})
	if file == "" && desc == "" {
		return nil
	}
	return &Issue{
		File:        file,
		Line:        line,
		Severity:    SeverityHigh,
		Message:     fmt.Sprintf("gitleaks: %s", strings.TrimSpace(desc)),
		Category:    CategorySecurity,
		SubCategory: "secrets",
		AutoFixable: false,
	}
}

// filterSecurityNoise removes issues from paths that are expected to contain intentionally vulnerable fixtures
func (r *SecurityAssessmentRunner) filterSecurityNoise(in []Issue, cfg AssessmentConfig) []Issue {
	if len(in) == 0 {
		return in
	}
	if !cfg.SecurityExcludeFixtures {
		return in
	}
	var out []Issue
	patterns := cfg.SecurityFixturePatterns
	if len(patterns) == 0 {
		patterns = []string{"tests/fixtures/", "test-fixtures/"}
	}
	for _, is := range in {
		p := strings.ToLower(filepath.ToSlash(is.File))
		excluded := false
		for _, pat := range patterns {
			if strings.Contains(p, strings.ToLower(pat)) {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}
		out = append(out, is)
	}
	return out
}

func (r *SecurityAssessmentRunner) filterToAssessmentRoot(target string, issues []Issue, suppressions []Suppression) ([]Issue, []Suppression) {
	root := filepath.Clean(target)
	if absRoot, err := filepath.Abs(root); err == nil {
		root = absRoot
	}

	filteredIssues := make([]Issue, 0, len(issues))
	droppedIssues := 0
	var droppedIssueFiles []string
	for _, issue := range issues {
		if !pathWithinAssessmentRoot(root, issue.File) {
			droppedIssues++
			if strings.TrimSpace(issue.File) != "" {
				droppedIssueFiles = append(droppedIssueFiles, issue.File)
			}
			continue
		}
		filteredIssues = append(filteredIssues, issue)
	}

	filteredSuppressions := make([]Suppression, 0, len(suppressions))
	droppedSuppressions := 0
	var droppedSuppressionFiles []string
	for _, suppression := range suppressions {
		if !pathWithinAssessmentRoot(root, suppression.File) {
			droppedSuppressions++
			if strings.TrimSpace(suppression.File) != "" {
				droppedSuppressionFiles = append(droppedSuppressionFiles, suppression.File)
			}
			continue
		}
		filteredSuppressions = append(filteredSuppressions, suppression)
	}

	if droppedIssues > 0 || droppedSuppressions > 0 {
		logger.Warn(fmt.Sprintf("security: dropped %d issue(s) and %d suppression(s) outside assessment root %s", droppedIssues, droppedSuppressions, root))
		if len(droppedIssueFiles) > 0 {
			logger.Debug(fmt.Sprintf("security: dropped out-of-scope issue files: %v", droppedIssueFiles))
		}
		if len(droppedSuppressionFiles) > 0 {
			logger.Debug(fmt.Sprintf("security: dropped out-of-scope suppression files: %v", droppedSuppressionFiles))
		}
	}

	return filteredIssues, filteredSuppressions
}

func pathWithinAssessmentRoot(root, candidate string) bool {
	if strings.TrimSpace(candidate) == "" {
		return false
	}

	cleanCandidate := filepath.Clean(candidate)
	if filepath.IsAbs(cleanCandidate) {
		rel, err := filepath.Rel(root, cleanCandidate)
		if err != nil {
			return false
		}
		return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
	}

	return cleanCandidate != ".." && !strings.HasPrefix(cleanCandidate, ".."+string(os.PathSeparator))
}

// pathMatchesAny checks if path contains any of the include anchors (substring match, absolute-safe)
func pathMatchesAny(path string, includes []string) bool {
	p := filepath.Clean(path)
	ap := p
	if !filepath.IsAbs(ap) {
		if a2, err := filepath.Abs(ap); err == nil {
			ap = a2
		}
	}
	for _, inc := range includes {
		inc = filepath.Clean(inc)
		if strings.Contains(p, inc) {
			return true
		}
		if a2, err := filepath.Abs(inc); err == nil && strings.Contains(ap, a2) {
			return true
		}
	}
	return false
}

// nearestCommonAncestor returns the closest common directory for the given file list
func nearestCommonAncestor(files []string) string {
	if len(files) == 0 {
		return ""
	}
	// Convert to absolute directories
	dirs := make([]string, 0, len(files))
	for _, f := range files {
		d := filepath.Dir(f)
		if !filepath.IsAbs(d) {
			if a2, err := filepath.Abs(d); err == nil {
				d = a2
			}
		}
		dirs = append(dirs, filepath.Clean(d))
	}
	// Initialize with first
	common := dirs[0]
	for _, d := range dirs[1:] {
		for !strings.HasPrefix(d+string(os.PathSeparator), common+string(os.PathSeparator)) && common != string(os.PathSeparator) {
			common = filepath.Dir(common)
		}
	}
	return common
}

func getString(m map[string]interface{}, keys []string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
}

func getInt(m map[string]interface{}, keys []string) int {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch t := v.(type) {
			case float64:
				return int(t)
			case json.Number:
				if n, err := strconv.Atoi(t.String()); err == nil {
					return n
				}
			case int:
				return t
			case string:
				var n int
				if _, err := fmt.Sscanf(t, "%d", &n); err == nil {
					return n
				}
			}
		}
	}
	return 0
}

// extractFirstJSONObjectBytes extracts the first valid JSON object from noisy output.
// It ignores brace-delimited non-JSON fragments like "{0 packages}".
func extractFirstJSONObjectBytes(b []byte) []byte {
	for i := 0; i < len(b); i++ {
		if b[i] != '{' {
			continue
		}
		j := i + 1
		for j < len(b) {
			switch b[j] {
			case ' ', '\n', '\r', '\t':
				j++
				continue
			default:
				goto afterWS
			}
		}
		continue
	afterWS:
		if j >= len(b) || b[j] != '"' {
			continue
		}
		dec := json.NewDecoder(bytes.NewReader(b[i:]))
		var raw json.RawMessage
		if err := dec.Decode(&raw); err == nil && len(raw) > 0 {
			return raw
		}
	}
	return nil
}

func persistGosecParseFailure(dirArg string, stdout, stderr []byte) string {
	// Best-effort: write debug output to a temp file for diagnosis.
	safe := strings.NewReplacer(string(os.PathSeparator), "-", ":", "-", "..", "-").Replace(dirArg)
	if safe == "" {
		safe = "shard"
	}
	f, err := os.CreateTemp("", "goneat-gosec-"+safe+"-*.log")
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()
	_, _ = f.WriteString("# gosec parse failure debug\n")
	_, _ = f.WriteString("## stdout\n")
	_, _ = f.Write(stdout)
	_, _ = f.WriteString("\n\n## stderr\n")
	_, _ = f.Write(stderr)
	return f.Name()
}

// uniqueDirs returns unique directory paths for the given file list
func (r *SecurityAssessmentRunner) uniqueDirs(files []string) []string {
	seen := make(map[string]bool)
	var dirs []string
	for _, f := range files {
		d := filepath.Dir(f)
		if !seen[d] {
			seen[d] = true
			dirs = append(dirs, d)
		}
	}
	if len(dirs) == 0 {
		return []string{"./..."}
	}
	return dirs
}

// effectiveToolContext returns a context with timeout=min(global, per-tool) if any set, otherwise the original ctx
func (r *SecurityAssessmentRunner) effectiveToolContext(ctx context.Context, global, perTool time.Duration) (context.Context, context.CancelFunc) {
	eff := time.Duration(0)
	if global > 0 && perTool > 0 {
		if global < perTool {
			eff = global
		} else {
			eff = perTool
		}
	} else if global > 0 {
		eff = global
	} else if perTool > 0 {
		eff = perTool
	}
	if eff <= 0 {
		// No timeout configured; return original context with no-op cancel
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, eff) // #nosec G118 -- helper returns cancel func so callers can defer it at the call site
}

// init registers the security assessment runner
func init() {
	RegisterAssessmentRunner(CategorySecurity, NewSecurityAssessmentRunner())
}
