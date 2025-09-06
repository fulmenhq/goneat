/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package assess

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Suppression represents a security tool suppression
type Suppression struct {
	Tool     string                 `json:"tool"`
	RuleID   string                 `json:"rule_id,omitempty"`
	File     string                 `json:"file"`
	Line     int                    `json:"line"`
	Column   int                    `json:"column,omitempty"`
	Syntax   string                 `json:"syntax"`
	Reason   string                 `json:"reason,omitempty"`
	Severity IssueSeverity          `json:"severity,omitempty"`
	AgeDays  int                    `json:"age_days,omitempty"`
	Author   string                 `json:"author,omitempty"`
	Commit   string                 `json:"commit,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// SuppressionSummary provides statistics about suppressions
type SuppressionSummary struct {
	Total          int                 `json:"total"`
	ByTool         map[string]int      `json:"by_tool"`
	BySeverity     map[string]int      `json:"by_severity"`
	ByRule         map[string]int      `json:"by_rule"`
	ByRuleFiles    map[string][]string `json:"by_rule_files,omitempty"`
	ByFile         map[string]int      `json:"by_file,omitempty"`
	TopRules       []TopItem           `json:"top_rules,omitempty"`
	TopFiles       []TopItem           `json:"top_files,omitempty"`
	WithReason     int                 `json:"with_reason"`
	WithoutReason  int                 `json:"without_reason"`
	AverageAgeDays float64             `json:"average_age_days,omitempty"`
	OldestDays     int                 `json:"oldest_days,omitempty"`
	NewestDays     int                 `json:"newest_days,omitempty"`
}

// TopItem represents an aggregated item and its count
type TopItem struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// SuppressionReport contains all suppression information
type SuppressionReport struct {
	Suppressions     []Suppression      `json:"suppressions"`
	Summary          SuppressionSummary `json:"summary"`
	PolicyViolations []PolicyViolation  `json:"policy_violations,omitempty"`
}

// PolicyViolation represents a suppression that violates policy
type PolicyViolation struct {
	Suppression Suppression `json:"suppression"`
	Violations  []string    `json:"violations"`
}

// SuppressionParser extracts suppressions from source files
type SuppressionParser struct {
	// Tool-specific patterns for extracting suppressions
	patterns map[string][]*regexp.Regexp
}

// NewSuppressionParser creates a parser with default patterns
func NewSuppressionParser() *SuppressionParser {
	return &SuppressionParser{
		patterns: map[string][]*regexp.Regexp{
			"gosec": {
				// #nosec or // #nosec with optional rule and reason
				regexp.MustCompile(`(?m)^\s*(?://|/\*)\s*#nosec(?:\s+(G\d{3}))?(?:\s*[-–]\s*(.*))?`),
				regexp.MustCompile(`(?m)^\s*#nosec(?:\s+(G\d{3}))?(?:\s*[-–]\s*(.*))?`),
			},
			"bandit": {
				// # nosec or # nosec B104
				regexp.MustCompile(`(?m)^\s*#\s*nosec(?:\s+(B\d{3}))?(?:\s*[-–]\s*(.*))?`),
			},
			"semgrep": {
				// # nosemgrep or // nosemgrep
				regexp.MustCompile(`(?m)^\s*(?:#|//)\s*nosemgrep(?:\s*:\s*([^\s]+))?(?:\s*[-–]\s*(.*))?`),
			},
			"biome": {
				// // biome-ignore lint/category/rule: reason
				regexp.MustCompile(`(?m)^\s*//\s*biome-ignore\s+([^:]+)(?:\s*:\s*(.*))?`),
			},
			"eslint": {
				// // eslint-disable-next-line rule
				regexp.MustCompile(`(?m)^\s*//\s*eslint-disable-next-line(?:\s+([^\s]+))?(?:\s*--\s*(.*))?`),
				// /* eslint-disable rule */
				regexp.MustCompile(`(?m)^\s*/\*\s*eslint-disable(?:\s+([^\s]+))?\s*\*/`),
			},
			"ruff": {
				// # noqa or # noqa: F401
				regexp.MustCompile(`(?m)^\s*#\s*noqa(?:\s*:\s*([A-Z]\d{3}))?(?:\s*[-–]\s*(.*))?`),
			},
		},
	}
}

// ParseFile extracts suppressions from a source file
func (p *SuppressionParser) ParseFile(filePath string) ([]Suppression, error) {
	// Validate file path to prevent path traversal
	filePath = filepath.Clean(filePath)
	if strings.Contains(filePath, "..") {
		return nil, fmt.Errorf("invalid file path: contains path traversal")
	}
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close() //nolint:errcheck // Close errors are typically ignored for files

	var suppressions []Suppression
	scanner := bufio.NewScanner(file)
	lineNum := 0

	// Determine which tools to check based on file extension
	ext := strings.ToLower(filepath.Ext(filePath))
	var toolsToCheck []string

	switch ext {
	case ".go":
		toolsToCheck = []string{"gosec"}
	case ".py":
		toolsToCheck = []string{"bandit", "ruff"}
	case ".js", ".jsx", ".ts", ".tsx":
		toolsToCheck = []string{"biome", "eslint", "semgrep"}
	case ".java":
		toolsToCheck = []string{"semgrep"}
	default:
		// Check all tools for unknown extensions
		for tool := range p.patterns {
			toolsToCheck = append(toolsToCheck, tool)
		}
	}

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		for _, tool := range toolsToCheck {
			patterns, ok := p.patterns[tool]
			if !ok {
				continue
			}

			for _, pattern := range patterns {
				if matches := pattern.FindStringSubmatch(line); matches != nil {
					supp := Suppression{
						Tool:   tool,
						File:   filePath,
						Line:   lineNum,
						Syntax: strings.TrimSpace(matches[0]),
					}

					// Extract rule ID and reason based on capture groups
					if len(matches) > 1 && matches[1] != "" {
						supp.RuleID = matches[1]
					}
					if len(matches) > 2 && matches[2] != "" {
						supp.Reason = strings.TrimSpace(matches[2])
					}

					suppressions = append(suppressions, supp)
					break // Only match once per line
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return suppressions, nil
}

// ParseDirectory recursively parses all files in a directory
func (p *SuppressionParser) ParseDirectory(dir string, includePatterns []string) ([]Suppression, error) {
	var allSuppressions []Suppression

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-source files
		if info.IsDir() {
			// Skip common non-source directories
			name := info.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" || name == ".idea" {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if file matches include patterns
		if len(includePatterns) > 0 {
			matched := false
			for _, pattern := range includePatterns {
				if matched, _ = filepath.Match(pattern, path); matched {
					break
				}
			}
			if !matched {
				return nil
			}
		}

		// Only parse known source file extensions
		ext := strings.ToLower(filepath.Ext(path))
		supportedExts := map[string]bool{
			".go": true, ".py": true, ".js": true, ".jsx": true,
			".ts": true, ".tsx": true, ".java": true, ".rb": true,
			".php": true, ".cs": true, ".cpp": true, ".c": true,
			".h": true, ".hpp": true, ".rs": true,
		}

		if !supportedExts[ext] {
			return nil
		}

		suppressions, err := p.ParseFile(path)
		if err != nil {
			// Log error but continue processing
			return nil
		}

		allSuppressions = append(allSuppressions, suppressions...)
		return nil
	})

	return allSuppressions, err
}

// GenerateSummary creates a summary from a list of suppressions
func GenerateSummary(suppressions []Suppression) SuppressionSummary {
	summary := SuppressionSummary{
		Total:       len(suppressions),
		ByTool:      make(map[string]int),
		BySeverity:  make(map[string]int),
		ByRule:      make(map[string]int),
		ByRuleFiles: make(map[string][]string),
		ByFile:      make(map[string]int),
	}

	// Initialize severity map
	for _, sev := range []string{"critical", "high", "medium", "low", "info"} {
		summary.BySeverity[sev] = 0
	}

	totalAge := 0
	ageCount := 0
	oldestAge := 0
	newestAge := int(^uint(0) >> 1) // Max int

	for _, supp := range suppressions {
		// Count by tool
		summary.ByTool[supp.Tool]++

		// Count by rule
		if supp.RuleID != "" {
			summary.ByRule[supp.RuleID]++
			// Append file to rule file list (deduplicated later)
			f := supp.File
			if f != "" {
				summary.ByRuleFiles[supp.RuleID] = append(summary.ByRuleFiles[supp.RuleID], f)
			}
		}

		// Count by file
		if supp.File != "" {
			summary.ByFile[supp.File]++
		}

		// Count by severity
		if supp.Severity != "" {
			summary.BySeverity[string(supp.Severity)]++
		}

		// Count with/without reason
		if supp.Reason != "" {
			summary.WithReason++
		} else {
			summary.WithoutReason++
		}

		// Calculate age statistics
		if supp.AgeDays > 0 {
			totalAge += supp.AgeDays
			ageCount++
			if supp.AgeDays > oldestAge {
				oldestAge = supp.AgeDays
			}
			if supp.AgeDays < newestAge {
				newestAge = supp.AgeDays
			}
		}
	}

	// Calculate average age
	if ageCount > 0 {
		summary.AverageAgeDays = float64(totalAge) / float64(ageCount)
		summary.OldestDays = oldestAge
		summary.NewestDays = newestAge
	}

	// Deduplicate file lists per rule
	for rule, files := range summary.ByRuleFiles {
		seen := make(map[string]struct{}, len(files))
		out := make([]string, 0, len(files))
		for _, f := range files {
			if _, ok := seen[f]; ok {
				continue
			}
			seen[f] = struct{}{}
			out = append(out, f)
		}
		summary.ByRuleFiles[rule] = out
	}

	// Deduplicate file lists per rule
	for rule, files := range summary.ByRuleFiles {
		seen := make(map[string]struct{}, len(files))
		out := make([]string, 0, len(files))
		for _, f := range files {
			if _, ok := seen[f]; ok {
				continue
			}
			seen[f] = struct{}{}
			out = append(out, f)
		}
		summary.ByRuleFiles[rule] = out
	}

	// Compute top summaries (top 5)
	summary.TopRules = topK(summary.ByRule, 5)
	summary.TopFiles = topK(summary.ByFile, 5)

	return summary
}

// topK returns the top k items by count from a map
func topK(m map[string]int, k int) []TopItem {
	if len(m) == 0 || k <= 0 {
		return nil
	}
	items := make([]TopItem, 0, len(m))
	for name, cnt := range m {
		items = append(items, TopItem{Name: name, Count: cnt})
	}
	// simple selection by sort (avoid importing sort if already in file?)
	// We already import sort in other files; add here
	// We'll implement a simple bubble for minimal deps, but in practice sort is fine.
	// Using sort.Slice for clarity.
	// Add import in header.
	return sortTop(items, k)
}

// sortTop sorts by Count desc and returns first k entries
func sortTop(items []TopItem, k int) []TopItem {
	// We'll rely on a tiny local sort to avoid an extra import line churn; use standard sort.
	// Note: We need to import "sort" at top of file.
	// The apply_patch will add the import automatically below.
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].Name < items[j].Name
		}
		return items[i].Count > items[j].Count
	})
	if k > len(items) {
		k = len(items)
	}
	return items[:k]
}

// EnrichWithGitInfo adds git blame information to suppressions
func EnrichWithGitInfo(suppressions []Suppression) []Suppression {
	// This is a placeholder for git integration
	// In a real implementation, this would:
	// 1. Run git blame on each file/line
	// 2. Extract author and commit SHA
	// 3. Calculate age from commit date
	// 4. Add to suppression metadata

	// For now, just return suppressions as-is
	return suppressions
}

// CheckPolicyViolations checks suppressions against policy rules
func CheckPolicyViolations(suppressions []Suppression, policy SecurityPolicy) []PolicyViolation {
	var violations []PolicyViolation

	for _, supp := range suppressions {
		var issues []string

		// Check if reason is required
		if policy.RequiresReason(supp.Severity, supp.Tool) && supp.Reason == "" {
			issues = append(issues, fmt.Sprintf("Missing required reason for %s severity", supp.Severity))
		}

		// Check age limit
		if policy.MaxAgeDays > 0 && supp.AgeDays > policy.MaxAgeDays {
			issues = append(issues, fmt.Sprintf("Suppression older than %d days", policy.MaxAgeDays))
		}

		// Check if approval is required
		if approvers := policy.RequiresApproval(supp.Severity, supp.RuleID); len(approvers) > 0 {
			// In real implementation, would check git history for approval
			issues = append(issues, fmt.Sprintf("Requires approval from %s", strings.Join(approvers, ", ")))
		}

		// Check block patterns
		if policy.IsBlocked(supp.RuleID, supp.File) {
			issues = append(issues, fmt.Sprintf("Rule %s cannot be suppressed in %s", supp.RuleID, supp.File))
		}

		if len(issues) > 0 {
			violations = append(violations, PolicyViolation{
				Suppression: supp,
				Violations:  issues,
			})
		}
	}

	return violations
}

// SecurityPolicy represents security policy rules (simplified for this implementation)
type SecurityPolicy struct {
	MaxAgeDays            int
	RequireReasonSeverity []IssueSeverity
	RequireApprovalRules  map[string][]string // rule -> approvers
	BlockedPatterns       []BlockPattern
}

type BlockPattern struct {
	Rule        string
	FilePattern string
}

func (p SecurityPolicy) RequiresReason(severity IssueSeverity, tool string) bool {
	for _, sev := range p.RequireReasonSeverity {
		if sev == severity {
			return true
		}
	}
	return false
}

func (p SecurityPolicy) RequiresApproval(severity IssueSeverity, ruleID string) []string {
	if approvers, ok := p.RequireApprovalRules[ruleID]; ok {
		return approvers
	}
	return nil
}

func (p SecurityPolicy) IsBlocked(ruleID, file string) bool {
	for _, pattern := range p.BlockedPatterns {
		if pattern.Rule == ruleID {
			if matched, _ := filepath.Match(pattern.FilePattern, file); matched {
				return true
			}
		}
	}
	return false
}
