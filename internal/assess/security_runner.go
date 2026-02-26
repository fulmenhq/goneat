/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package assess

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"runtime"

	"github.com/fulmenhq/goneat/pkg/logger"
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
// directory patterns suitable for gosec's -exclude-dir option
func parseIgnorePatternsForGosec(moduleRoot string) []string {
	var excludeDirs []string

	// Common ignore files to check
	ignoreFiles := []string{
		filepath.Join(moduleRoot, ".goneatignore"),
		filepath.Join(moduleRoot, ".gitignore"),
		filepath.Join(moduleRoot, ".goneat", "ignore"),
	}

	// Directory patterns that should be excluded (converted to relative paths for gosec)
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

	// Check if ignore files exist and add their directory patterns
	for _, ignoreFile := range ignoreFiles {
		// #nosec G304 -- ignoreFile from controlled list of known ignore files (.goneatignore, .gitignore, .goneat/ignore)
		if file, err := os.Open(ignoreFile); err == nil {
			defer func() {
				_ = file.Close() // Ignore close errors for ignore file parsing - defensive programming
			}()
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}

				// Convert patterns ending with / to directory exclusions
				if strings.HasSuffix(line, "/") || strings.Contains(line, "*/") {
					// Remove trailing slash and wildcards for gosec
					cleanPattern := strings.TrimSuffix(line, "/")
					cleanPattern = strings.ReplaceAll(cleanPattern, "*/", "")
					if cleanPattern != "" && !sliceContainsString(excludeDirs, cleanPattern) {
						excludeDirs = append(excludeDirs, cleanPattern)
					}
				}
			}
		}
	}

	// Add common directory patterns if not already present
	for _, pattern := range dirPatterns {
		if !sliceContainsString(excludeDirs, pattern) {
			excludeDirs = append(excludeDirs, pattern)
		}
	}

	return excludeDirs
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
	}
	adapters := GetSecurityToolRegistry().SelectAdapters(config, r, moduleRoot)
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
			if withSupp, ok := tool.(SecurityToolWithSuppressions); ok && config.TrackSuppressions {
				iss, supps, err := withSupp.RunWithSuppressions(ctx)
				resultsCh <- res{issues: iss, suppressions: supps, err: err, name: tool.Name()}
			} else {
				iss, err := tool.Run(ctx)
				resultsCh <- res{issues: iss, err: err, name: tool.Name()}
			}
		}()
	}
	for i := 0; i < len(adapters); i++ {
		rres := <-resultsCh
		if rres.err != nil {
			// Provide actionable guidance on tool setup
			var hint string
			switch rres.name {
			case "gosec":
				hint = " (install/update with: go install github.com/securego/gosec/v2/cmd/gosec@latest)"
			case "govulncheck":
				hint = " (install/update with: go install golang.org/x/vuln/cmd/govulncheck@latest)"
			}
			logger.Warn(fmt.Sprintf("%s scan failed: %v%s", rres.name, rres.err, hint))
			continue
		}
		issues = append(issues, rres.issues...)
		allSuppressions = append(allSuppressions, rres.suppressions...)
	}

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
		Success:       true,
		ExecutionTime: HumanReadableDuration(time.Since(start)),
		Issues:        issues,
		Metrics:       metrics,
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

// IsAvailable implements AssessmentRunner.IsAvailable
func (r *SecurityAssessmentRunner) IsAvailable() bool {
	// Available if either gosec or govulncheck is in PATH
	return r.toolAvailable("gosec") || r.toolAvailable("govulncheck")
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
	if len(config.IncludeFiles) > 0 {
		dirs = r.uniqueDirs(config.IncludeFiles)
	} else {
		// Discover Go package directories across multi-module repos
		if moduleDirs, err := r.findModuleDirs(moduleRoot); err == nil && len(moduleDirs) > 0 {
			pkgSet := make(map[string]struct{})
			for _, mdir := range moduleDirs {
				if pkgs, err := r.listGoPackageDirs(mdir); err == nil {
					for _, p := range pkgs {
						// Convert to relative from moduleRoot if possible for nicer args
						rel := p
						if rel2, err2 := filepath.Rel(moduleRoot, p); err2 == nil {
							rel = rel2
						}
						// Honor .goneatignore patterns
						if r.pathIgnored(filepath.Join(moduleRoot, rel)) {
							continue
						}
						pkgSet[rel] = struct{}{}
					}
				}
			}
			for p := range pkgSet {
				dirs = append(dirs, p)
			}
		}
		if len(dirs) == 0 {
			// Fallback to single shard
			dirs = []string{"./..."}
		}
	}

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

	type shardResult struct {
		issues       []Issue
		suppressions []Suppression
		err          error
	}
	results := make(chan shardResult, len(dirs))

	// Simple bounded worker pool via semaphore
	sem := make(chan struct{}, workers)
	for _, d := range dirs {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		default:
		}
		sem <- struct{}{}
		go func(dirArg string) {
			defer func() { <-sem }()
			args := []string{"-quiet", "-fmt=json"}
			if config.TrackSuppressions {
				args = append(args, "-track-suppressions")
			}

			// Add directory exclusions based on .goneatignore and .gitignore patterns
			excludeDirs := parseIgnorePatternsForGosec(moduleRoot)
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
			if err != nil {
				// gosec returns non-zero when issues found; still parse JSON output if present
				logger.Debug(fmt.Sprintf("gosec(%s) returned error: %v", dirArg, err))
			}

			primary := stdout
			if len(bytes.TrimSpace(primary)) == 0 {
				primary = stderr
			}
			// Treat empty output as no issues without warning
			if len(bytes.TrimSpace(primary)) == 0 {
				results <- shardResult{issues: nil, err: nil}
				return
			}

			// Parse with retry on malformed non-empty output
			iss, supps, perr := r.parseGosecOutputWithSuppressions(primary)
			// Post-filter to included files when scoped (reduces noise from dir-level scans)
			if len(config.IncludeFiles) > 0 && len(iss) > 0 {
				var fIss []Issue
				for _, is := range iss {
					if pathMatchesAny(is.File, config.IncludeFiles) {
						fIss = append(fIss, is)
					}
				}
				iss = fIss
			}
			if perr != nil {
				// Exponential backoff retry (max 2 tries)
				backoff := 200 * time.Millisecond
				maxRetries := 2
				for i := 0; i < maxRetries; i++ {
					select {
					case <-ctx.Done():
						results <- shardResult{issues: nil, err: ctx.Err()}
						return
					case <-time.After(backoff):
					}
					stdout2, stderr2, _ := runOnce(rctx)
					primary2 := stdout2
					if len(bytes.TrimSpace(primary2)) == 0 {
						primary2 = stderr2
					}
					if len(bytes.TrimSpace(primary2)) == 0 {
						// No issues; consider success
						results <- shardResult{issues: nil, err: nil}
						return
					}
					iss2, supps2, perr2 := r.parseGosecOutputWithSuppressions(primary2)
					if perr2 == nil {
						results <- shardResult{issues: iss2, suppressions: supps2, err: nil}
						return
					}
					backoff *= 2
				}
				// If still failing after retries, report once
				if p := persistGosecParseFailure(dirArg, stdout, stderr); p != "" {
					logger.Warn(fmt.Sprintf("gosec(%s) parse failed after retries (debug: %s): %v", dirArg, p, perr))
				} else {
					logger.Warn(fmt.Sprintf("gosec(%s) parse failed after retries: %v", dirArg, perr))
				}
			}
			results <- shardResult{issues: iss, suppressions: supps, err: perr}
		}(d)
	}

	// Drain pool
	for i := 0; i < cap(sem); i++ {
		sem <- struct{}{}
	}

	// Collect
	var allIssues []Issue
	var allSuppressions []Suppression
	for i := 0; i < len(dirs); i++ {
		r := <-results
		if r.err != nil {
			logger.Warn(fmt.Sprintf("gosec parse failed: %v", r.err))
			continue
		}
		allIssues = append(allIssues, r.issues...)
		allSuppressions = append(allSuppressions, r.suppressions...)
	}
	close(results)
	return allIssues, allSuppressions, nil
}

// Package-level metrics (single-process assumption; not exported)
var (
	lastShardCount int
	lastPoolSize   int
)

// listGoPackageDirs returns absolute directories for all packages under moduleRoot.
func (r *SecurityAssessmentRunner) listGoPackageDirs(moduleRoot string) ([]string, error) {
	cmd := exec.Command("go", "list", "-f", "{{.Dir}}", "./...")
	cmd.Dir = moduleRoot
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var dirs []string
	for _, line := range lines {
		d := strings.TrimSpace(line)
		if d == "" {
			continue
		}
		// Skip vendor and node_modules as a safety measure
		if strings.Contains(d, string(filepath.Separator)+"vendor"+string(filepath.Separator)) || strings.Contains(d, string(filepath.Separator)+"node_modules"+string(filepath.Separator)) {
			continue
		}
		dirs = append(dirs, d)
	}
	return dirs, nil
}

// findModuleDirs finds all directories containing a go.mod starting from root (multi-module aware)
func (r *SecurityAssessmentRunner) findModuleDirs(root string) ([]string, error) {
	var dirs []string
	// Always include root if it has go.mod or go.work references modules
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
		dirs = append(dirs, root)
	}

	// Walk and collect go.mod holders, skipping vendor/node_modules/.git
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			base := filepath.Base(path)
			if base == ".git" || base == "vendor" || base == "node_modules" {
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

// pathIgnored checks .goneatignore patterns using standardized config resolution
func (r *SecurityAssessmentRunner) pathIgnored(path string) bool {
	// Get ignore file locations using standardized resolver
	ignoreFiles := r.getIgnoreFiles(path)

	for _, ignoreFilePath := range ignoreFiles {
		if r.matchesIgnoreFile(path, ignoreFilePath) {
			return true
		}
	}
	return false
}

// getIgnoreFiles returns .goneatignore files in hierarchical order (Pattern 3: like .gitignore)
// Returns files from most specific (closest to target) to least specific (user global)
func (r *SecurityAssessmentRunner) getIgnoreFiles(targetPath string) []string {
	var ignoreFiles []string

	// Get the directory to start from (file's dir or target dir itself)
	startDir := targetPath
	if info, err := os.Stat(targetPath); err == nil && !info.IsDir() {
		startDir = filepath.Dir(targetPath)
	}

	// Convert to absolute path for consistent behavior
	if absDir, err := filepath.Abs(startDir); err == nil {
		startDir = absDir
	}

	// Walk up the directory hierarchy looking for .goneatignore files
	currentDir := startDir
	for {
		ignoreFile := filepath.Join(currentDir, ".goneatignore")
		if _, err := os.Stat(ignoreFile); err == nil {
			ignoreFiles = append(ignoreFiles, ignoreFile)
		}

		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			// Reached filesystem root
			break
		}
		currentDir = parentDir
	}

	// Add user-level global ignore (GONEAT_HOME/.goneatignore)
	if homeDir := os.Getenv("GONEAT_HOME"); homeDir != "" {
		userIgnore := filepath.Join(homeDir, ".goneatignore")
		if _, err := os.Stat(userIgnore); err == nil { // #nosec G703 - userIgnore from GONEAT_HOME env var, env-based paths are by design
			ignoreFiles = append(ignoreFiles, userIgnore)
		}
	} else if homeDir, err := os.UserHomeDir(); err == nil {
		// Standard location: ~/.goneat/.goneatignore
		userIgnore := filepath.Join(homeDir, ".goneat", ".goneatignore")
		if _, err := os.Stat(userIgnore); err == nil {
			ignoreFiles = append(ignoreFiles, userIgnore)
		}
	}

	return ignoreFiles
}

// matchesIgnoreFile checks if a path matches patterns in an ignore file
func (r *SecurityAssessmentRunner) matchesIgnoreFile(filePath, ignoreFilePath string) bool {
	// Validate ignore file path to prevent path traversal
	ignoreFilePath = filepath.Clean(ignoreFilePath)
	if strings.Contains(ignoreFilePath, "..") {
		return false
	}
	f, err := os.Open(ignoreFilePath)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	var patterns []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}
	// Relativize
	rel := filePath
	if wd, err := os.Getwd(); err == nil {
		if rp, err := filepath.Rel(wd, filePath); err == nil {
			rel = rp
		}
	}
	for _, p := range patterns {
		if matchIgnorePattern(p, rel) || matchIgnorePattern(p, filePath) {
			return true
		}
	}
	return false
}

// matchIgnorePattern performs simple glob and substring matching
func matchIgnorePattern(pattern, path string) bool {
	// negations not supported in this minimal helper
	pattern = strings.TrimPrefix(pattern, "!")
	if strings.Contains(pattern, "*") {
		if ok, _ := filepath.Match(pattern, filepath.Base(path)); ok {
			return true
		}
		if ok, _ := filepath.Match(pattern, path); ok {
			return true
		}
	}
	if strings.Contains(path, pattern) || filepath.Base(path) == pattern {
		return true
	}
	return false
}

// parseGosecOutput converts gosec JSON to issues
func (r *SecurityAssessmentRunner) parseGosecOutput(output []byte) ([]Issue, error) {
	issues, _, err := r.parseGosecOutputWithSuppressions(output)
	return issues, err
}

// parseGosecOutputWithSuppressions converts gosec JSON to issues and tracks suppressions
func (r *SecurityAssessmentRunner) parseGosecOutputWithSuppressions(output []byte) ([]Issue, []Suppression, error) {
	type gosecIssue struct {
		Severity string      `json:"severity"`
		Details  string      `json:"details"`
		File     string      `json:"file"`
		Code     string      `json:"code"`
		Line     interface{} `json:"line"` // tolerate string or number
		RuleID   string      `json:"rule_id"`
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
	var suppressions []Suppression
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
	// govulncheck emits a JSON event stream; capture and parse line-wise
	// When scoped, prefer limiting to impacted package directories.
	args := []string{"-json"}
	if len(config.IncludeFiles) > 0 {
		// Build unique package dirs relative to module root
		dirs := r.uniqueDirs(config.IncludeFiles)
		for _, d := range dirs {
			// Normalize path relative to moduleRoot if possible
			rel := d
			if abs, err := filepath.Abs(d); err == nil {
				if strings.HasPrefix(abs, moduleRoot) {
					if r2, err2 := filepath.Rel(moduleRoot, abs); err2 == nil {
						rel = r2
					}
				}
			}
			if rel == "." {
				rel = "./..."
			}
			args = append(args, rel)
		}
		if len(dirs) == 0 {
			args = append(args, "./...")
		}
	} else {
		args = append(args, "./...")
	}
	rctx, cancel := r.effectiveToolContext(ctx, config.Timeout, config.SecurityGovulncheckTimeout)
	defer cancel()
	cmd := exec.CommandContext(rctx, "govulncheck", args...)
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
		return nil, fmt.Errorf("start govulncheck: %w", err)
	}

	// Consume stderr (avoid blocking); log at debug level
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			logger.Debug("govulncheck: " + scanner.Text())
		}
	}()

	var issues []Issue
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if iss, ok := r.parseGovulnEventLine(moduleRoot, line); ok {
			issues = append(issues, *iss)
		}
	}
	if err := scanner.Err(); err != nil {
		logger.Warn(fmt.Sprintf("govulncheck scan read error: %v", err))
	}
	if err := cmd.Wait(); err != nil {
		// Non-zero exit is possible when vulns found; not an error for our purposes
		logger.Debug(fmt.Sprintf("govulncheck exited: %v", err))
	}
	return issues, nil
}

// parseGovulnEventLine parses a single govulncheck JSON event line into an Issue.
// Returns (nil, false) for non-finding or non-JSON lines.
func (r *SecurityAssessmentRunner) parseGovulnEventLine(moduleRoot, line string) (*Issue, bool) {
	type gvFinding struct {
		Type    string `json:"type"`
		Finding struct {
			OSV    string `json:"osv"`
			Module struct {
				Path string `json:"path"`
			} `json:"module"`
			Package struct {
				Path string `json:"path"`
			} `json:"package"`
		} `json:"finding"`
	}

	var evt gvFinding
	if err := json.Unmarshal([]byte(line), &evt); err != nil {
		return nil, false
	}
	if evt.Type != "finding" || evt.Finding.OSV == "" {
		return nil, false
	}
	iss := Issue{
		File:        filepath.Join(moduleRoot, "go.mod"),
		Line:        0,
		Severity:    SeverityHigh,
		Message:     fmt.Sprintf("govulncheck: %s in %s (%s)", evt.Finding.OSV, evt.Finding.Module.Path, evt.Finding.Package.Path),
		Category:    CategorySecurity,
		SubCategory: "vulnerability",
		AutoFixable: false,
	}
	return &iss, true
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
	args := []string{"detect", "--no-banner", "--report-format", "json", "--report-path", "-", "--source", source}
	rctx, cancel := r.effectiveToolContext(ctx, config.Timeout, 0)
	defer cancel()
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

	// Drain stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			logger.Debug("gitleaks: " + scanner.Text())
		}
	}()

	// gitleaks may output a JSON array or newline-delimited JSON
	data, _ := io.ReadAll(stdout)
	issues, perr := r.parseGitleaksOutput(data)
	if perr != nil {
		logger.Warn(fmt.Sprintf("gitleaks parse failed: %v", perr))
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

	if err := cmd.Wait(); err != nil {
		// non-zero may still indicate findings; not fatal
		logger.Debug(fmt.Sprintf("gitleaks exited: %v", err))
	}
	return issues, nil
}

// parseGitleaksOutput parses gitleaks JSON output
func (r *SecurityAssessmentRunner) parseGitleaksOutput(data []byte) ([]Issue, error) {
	// Try array form first
	var arr []map[string]interface{}
	if err := json.Unmarshal(data, &arr); err == nil {
		return r.mapGitleaksArray(arr), nil
	}
	// Try NDJSON line by line
	var issues []Issue
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var m map[string]interface{}
		if json.Unmarshal([]byte(line), &m) == nil {
			iss := r.mapGitleaksFinding(m)
			if iss != nil {
				issues = append(issues, *iss)
			}
		}
	}
	if len(issues) == 0 {
		return nil, fmt.Errorf("unrecognized gitleaks output")
	}
	return issues, nil
}

func (r *SecurityAssessmentRunner) mapGitleaksArray(arr []map[string]interface{}) []Issue {
	var issues []Issue
	for _, m := range arr {
		if iss := r.mapGitleaksFinding(m); iss != nil {
			issues = append(issues, *iss)
		}
	}
	return issues
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
	return context.WithTimeout(ctx, eff)
}

// init registers the security assessment runner
func init() {
	RegisterAssessmentRunner(CategorySecurity, NewSecurityAssessmentRunner())
}
