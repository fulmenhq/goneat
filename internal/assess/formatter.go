/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package assess

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/aymerick/raymond"
)

// OutputFormat represents the format for assessment output
type OutputFormat string

const (
	FormatMarkdown OutputFormat = "markdown"
	FormatJSON     OutputFormat = "json"
	FormatHTML     OutputFormat = "html"
	FormatBoth     OutputFormat = "both"
	// Concise is a short, colorized summary ideal for hook logs
	FormatConcise OutputFormat = "concise"
)

// Formatter handles formatting assessment reports
type Formatter struct {
	format     OutputFormat
	targetPath string
}

// NewFormatter creates a new report formatter
func NewFormatter(format OutputFormat) *Formatter {
	return &Formatter{format: format}
}

// SetTargetPath sets the target path for project information extraction
func (f *Formatter) SetTargetPath(targetPath string) {
	f.targetPath = targetPath
}

// FormatReport formats an assessment report according to the configured format
func (f *Formatter) FormatReport(report *AssessmentReport) (string, error) {
	switch f.format {
	case FormatConcise:
		return f.formatConcise(report), nil
	case FormatMarkdown:
		return f.formatMarkdown(report), nil
	case FormatJSON:
		return f.formatJSON(report)
	case FormatHTML:
		return f.formatHTML(report), nil
	case FormatBoth:
		markdown := f.formatMarkdown(report)
		jsonStr, err := f.formatJSON(report)
		if err != nil {
			return "", err
		}
		return markdown + "\n\n---\n\n" + jsonStr, nil
	default:
		return "", fmt.Errorf("unsupported format: %s", f.format)
	}
}

// formatConcise prints a short, colorized summary suitable for hook logs
func (f *Formatter) formatConcise(report *AssessmentReport) string {
	color := func(code string, s string) string {
		if os.Getenv("NO_COLOR") != "" {
			return s
		}
		return "\x1b[" + code + "m" + s + "\x1b[0m"
	}
	bold := func(s string) string { return color("1", s) }
	green := func(s string) string { return color("32", s) }
	yellow := func(s string) string { return color("33", s) }
	red := func(s string) string { return color("31", s) }

	var sb strings.Builder

	// Header line with health and timing
	health := int(report.Summary.OverallHealth * 100)
	healthStr := fmt.Sprintf("%d%%", health)
	if health >= 90 {
		healthStr = green(healthStr)
	} else if health >= 75 {
		healthStr = yellow(healthStr)
	} else {
		healthStr = red(healthStr)
	}
	fmt.Fprintf(&sb, "%s %s | total issues: %d | time: %s\n",
		bold("Assessment"), fmt.Sprintf("health=%s", healthStr), report.Summary.TotalIssues, report.Metadata.ExecutionTime)

	// Fail-on context if present in commands_run hint
	// We don't have explicit fail-on in metadata yet; detect from commands or print default marker
	failOn := os.Getenv("GONEAT_SECURITY_FAIL_ON")
	if strings.TrimSpace(failOn) == "" {
		// best-effort default display; actual enforcement happens in caller
		failOn = "configured"
	}
	fmt.Fprintf(&sb, " - Fail-on: %s\n", failOn)

	// One line per category included
	ordered := f.getOrderedCategories(report.Categories)
	for _, cat := range ordered {
		res := report.Categories[cat]

		// Derive status from issue counts to avoid confusing transient runner warnings
		var statusStr string
		if res.IssueCount > 0 {
			statusStr = yellow(fmt.Sprintf("%d issue(s)", res.IssueCount))
		} else if res.Status == "error" && strings.TrimSpace(res.Error) != "" {
			statusStr = red("error")
		} else {
			statusStr = green("ok")
		}

		fmt.Fprintf(&sb, " - %s: %s (est %s)\n", titleCase(cat), statusStr, f.formatDuration(time.Duration(res.EstimatedTime)))

		// Show top-N affected files when there are issues
		if res.IssueCount > 0 {
			unique := make(map[string]struct{})
			files := make([]string, 0, len(res.Issues))
			for _, iss := range res.Issues {
				if iss.File == "" {
					continue
				}
				if _, seen := unique[iss.File]; !seen {
					unique[iss.File] = struct{}{}
					files = append(files, iss.File)
				}
			}
			// If the only entry is the repository sentinel, prefer metrics-provided file list
			if (len(files) == 1 && files[0] == "repository") || len(files) == 0 {
				if sample := extractSampleFiles(res.Metrics); len(sample) > 0 {
					files = sample
				}
			}
			const maxShow = 5
			shown := files
			if len(files) > maxShow {
				shown = files[:maxShow]
			}
			if len(shown) > 0 {
				more := ""
				if len(files) > len(shown) {
					more = fmt.Sprintf(" (+%d more)", len(files)-len(shown))
				}
				fmt.Fprintf(&sb, "   files: %s%s\n", strings.Join(shown, ", "), more)
			} else if len(res.Issues) > 0 {
				// Fallback: show first issue message if no file paths were captured
				msg := strings.TrimSpace(res.Issues[0].Message)
				if msg != "" {
					fmt.Fprintf(&sb, "   note: %s\n", msg)
				}
			}
		}

		// Headline metrics for category (e.g., security sharding)
		if res.Metrics != nil {
			if shards, ok := res.Metrics["gosec_shards"]; ok {
				line := fmt.Sprintf("   shards: %v", shards)
				if pool, ok2 := res.Metrics["gosec_pool_size"]; ok2 {
					line = fmt.Sprintf("%s (pool=%v)", line, pool)
				}
				sb.WriteString(line + "\n")
			}
		}

		if res.Status == "error" && strings.TrimSpace(res.Error) != "" {
			fmt.Fprintf(&sb, "   %s %s\n", red("!"), res.Error)
		}
	}

	// Footer pass/fail
	if report.Summary.TotalIssues == 0 {
		sb.WriteString(green("✅ Hook validation passed"))
	} else {
		sb.WriteString(yellow("⚠️ Issues detected - see details above or run with --verbose"))
	}

	return sb.String()
}

// WriteReport writes a formatted report to the given writer
func (f *Formatter) WriteReport(w io.Writer, report *AssessmentReport) error {
	output, err := f.FormatReport(report)
	if err != nil {
		return err
	}

	_, err = w.Write([]byte(output))
	return err
}

// formatMarkdown creates a markdown-formatted assessment report
func (f *Formatter) formatMarkdown(report *AssessmentReport) string {
	var sb strings.Builder

	// Header
	sb.WriteString("# Codebase Assessment Report\n\n")
	fmt.Fprintf(&sb, "**Generated:** %s\n", report.Metadata.GeneratedAt.Format(time.RFC3339))
	fmt.Fprintf(&sb, "**Tool:** %s\n", report.Metadata.Tool)
	fmt.Fprintf(&sb, "**Version:** %s\n", report.Metadata.Version)
	fmt.Fprintf(&sb, "**Target:** %s\n", report.Metadata.Target)
	fmt.Fprintf(&sb, "**Execution Time:** %v\n\n", report.Metadata.ExecutionTime)

	// Executive Summary
	sb.WriteString("## Executive Summary\n\n")
	fmt.Fprintf(&sb, "- **Overall Health:** %s\n", f.formatHealthScore(report.Summary.OverallHealth))
	fmt.Fprintf(&sb, "- **Critical Issues:** %d\n", report.Summary.CriticalIssues)
	fmt.Fprintf(&sb, "- **Total Issues:** %d\n", report.Summary.TotalIssues)
	fmt.Fprintf(&sb, "- **Estimated Fix Time:** %s\n", f.formatDuration(time.Duration(report.Summary.EstimatedTime)))
	fmt.Fprintf(&sb, "- **Parallelizable Tasks:** %d groups identified\n", report.Summary.ParallelGroups)
	fmt.Fprintf(&sb, "- **Categories with Issues:** %d\n\n", report.Summary.CategoriesWithIssues)

	// Assessment Results by Category
	sb.WriteString("## Assessment Results\n\n")

	// Sort categories by priority
	orderedCategories := f.getOrderedCategories(report.Categories)

	for _, category := range orderedCategories {
		result := report.Categories[category]
		if result.Status == "skipped" {
			continue
		}

		// Category header
		statusEmoji := f.getStatusEmoji(result.Status)
		fmt.Fprintf(&sb, "### %s %s Issues (Priority: %d)\n\n", statusEmoji, titleCase(category), result.Priority)

		if result.Status == "error" {
			fmt.Fprintf(&sb, "**Status:** Error - %s\n\n", result.Error)
			continue
		}

		fmt.Fprintf(&sb, "**Status:** %d issues found\n", result.IssueCount)
		fmt.Fprintf(&sb, "**Estimated Time:** %s\n", f.formatDuration(time.Duration(result.EstimatedTime)))
		fmt.Fprintf(&sb, "**Parallelizable:** %s\n\n", f.formatBool(result.Parallelizable))

		if metricsBlock := f.formatCategoryMetricsMarkdown(category, result.Metrics); metricsBlock != "" {
			sb.WriteString(metricsBlock)
			sb.WriteString("\n")
		}

		if result.IssueCount > 0 {
			// Issues table (with optional cap for readability; JSON remains full SSOT)
			sb.WriteString("| File | Line | Severity | Message | Auto-fixable |\n")
			sb.WriteString("|------|------|----------|---------|--------------|\n")

			maxToShow := len(result.Issues)
			// optional: respect ENV cap for non-JSON output
			if capStr := strings.TrimSpace(os.Getenv("GONEAT_MAX_ISSUES_DISPLAY")); capStr != "" {
				if n, err := strconv.Atoi(capStr); err == nil && n > 0 && n < maxToShow {
					maxToShow = n
				}
			}

			for i, issue := range result.Issues {
				if i >= maxToShow {
					break
				}
				line := ""
				if issue.Line > 0 {
					line = fmt.Sprintf("%d", issue.Line)
				}
				fmt.Fprintf(&sb, "| %s | %s | %s | %s | %s |\n",
					issue.File, line, issue.Severity, issue.Message, f.formatBool(issue.AutoFixable))
			}
			if maxToShow < len(result.Issues) {
				fmt.Fprintf(&sb, "\n_Showing %d of %d issues. Use --format json for full details._\n", maxToShow, len(result.Issues))
			}
			sb.WriteString("\n")

			// If sentinel-only file reference, add an affected files summary from metrics for clarity
			onlySentinel := false
			if len(result.Issues) > 0 {
				onlySentinel = true
				for _, is := range result.Issues {
					if strings.TrimSpace(is.File) != "repository" && strings.TrimSpace(is.File) != "" {
						onlySentinel = false
						break
					}
				}
			}
			if onlySentinel {
				if sample := extractSampleFiles(result.Metrics); len(sample) > 0 {
					shown := sample
					const maxShow = 10
					if len(shown) > maxShow {
						shown = shown[:maxShow]
					}
					more := ""
					if len(sample) > len(shown) {
						more = fmt.Sprintf(" (+%d more)", len(sample)-len(shown))
					}
					fmt.Fprintf(&sb, "**Affected files:** %s%s\n\n", strings.Join(shown, ", "), more)
				}
			}
		}
	}

	// Recommended Workflow
	if len(report.Workflow.Phases) > 0 {
		sb.WriteString("## Recommended Workflow\n\n")

		// Build map for quick lookup of parallel groups by name
		groupByName := map[string]ParallelGroup{}
		for _, g := range report.Workflow.ParallelGroups {
			groupByName[g.Name] = g
		}
		// Prepare git-state samples once for potential sentinel expansion
		var gitStateSamples []string
		if cr, ok := report.Categories[string(CategoryRepoStatus)]; ok {
			if s := extractSampleFiles(cr.Metrics); len(s) > 0 {
				gitStateSamples = append(gitStateSamples, s...)
			}
		}
		if cr, ok := report.Categories[string(CategoryMaturity)]; ok {
			if s := extractSampleFiles(cr.Metrics); len(s) > 0 {
				gitStateSamples = append(gitStateSamples, s...)
			}
		}

		for i, phase := range report.Workflow.Phases {
			fmt.Fprintf(&sb, "### Phase %d: %s\n\n", i+1, phase.Description)
			fmt.Fprintf(&sb, "**Estimated Time:** %s\n", f.formatDuration(time.Duration(phase.EstimatedTime)))
			fmt.Fprintf(&sb, "**Categories:** %s\n", f.formatCategories(phase.Categories))

			if len(phase.ParallelGroups) > 0 {
				fmt.Fprintf(&sb, "**Parallel Groups:** %s\n", strings.Join(phase.ParallelGroups, ", "))
				// List files for each group (expand sentinel using git-state samples)
				for _, name := range phase.ParallelGroups {
					if g, ok := groupByName[name]; ok {
						files := g.Files
						if (len(files) == 1 && (files[0] == "repository" || files[0] == ".git")) && len(gitStateSamples) > 0 {
							const maxShow = 10
							shown := gitStateSamples
							if len(shown) > maxShow {
								shown = shown[:maxShow]
							}
							more := ""
							if len(gitStateSamples) > len(shown) {
								more = fmt.Sprintf(" (+%d more)", len(gitStateSamples)-len(shown))
							}
							fmt.Fprintf(&sb, "- %s files: %s%s\n", name, strings.Join(shown, ", "), more)
						} else if len(files) > 0 {
							fmt.Fprintf(&sb, "- %s files: %s\n", name, strings.Join(files, ", "))
						}
					}
				}
			}
			sb.WriteString("\n")
		}

		fmt.Fprintf(&sb, "**Total Estimated Time:** %s\n\n", f.formatDuration(time.Duration(report.Workflow.TotalTime)))
	}

	// Parallelization Opportunities
	if len(report.Workflow.ParallelGroups) > 0 {
		sb.WriteString("## Parallelization Opportunities\n\n")

		// Build a helper list of sample files from metrics for git-state categories
		var gitStateSamples []string
		if cr, ok := report.Categories[string(CategoryRepoStatus)]; ok {
			if s := extractSampleFiles(cr.Metrics); len(s) > 0 {
				gitStateSamples = append(gitStateSamples, s...)
			}
		}
		if cr, ok := report.Categories[string(CategoryMaturity)]; ok {
			if s := extractSampleFiles(cr.Metrics); len(s) > 0 {
				gitStateSamples = append(gitStateSamples, s...)
			}
		}

		for _, group := range report.Workflow.ParallelGroups {
			fmt.Fprintf(&sb, "### %s\n", group.Name)
			fmt.Fprintf(&sb, "**Description:** %s\n", group.Description)
			// If group files are sentinel-only, expand with git-state samples for clarity
			files := group.Files
			if (len(files) == 1 && (files[0] == "repository" || files[0] == ".git")) && len(gitStateSamples) > 0 {
				const maxShow = 20
				shown := gitStateSamples
				if len(shown) > maxShow {
					shown = shown[:maxShow]
				}
				more := ""
				if len(gitStateSamples) > len(shown) {
					more = fmt.Sprintf(" (+%d more)", len(gitStateSamples)-len(shown))
				}
				fmt.Fprintf(&sb, "**Files:** %s%s\n", strings.Join(shown, ", "), more)
			} else {
				fmt.Fprintf(&sb, "**Files:** %s\n", strings.Join(files, ", "))
			}
			fmt.Fprintf(&sb, "**Categories:** %s\n", f.formatCategories(group.Categories))
			fmt.Fprintf(&sb, "**Issues:** %d\n", group.IssueCount)
			fmt.Fprintf(&sb, "**Estimated Time:** %s\n\n", f.formatDuration(time.Duration(group.EstimatedTime)))
		}
	}

	// Extended Workplan (if --extended was used)
	if report.Workplan != nil {
		sb.WriteString("## Extended Workplan\n\n")
		sb.WriteString("### File Discovery\n\n")
		fmt.Fprintf(&sb, "- **Files Discovered:** %d\n", report.Workplan.FilesDiscovered)
		fmt.Fprintf(&sb, "- **Files Included:** %d\n", report.Workplan.FilesIncluded)
		fmt.Fprintf(&sb, "- **Files Excluded:** %d\n", report.Workplan.FilesExcluded)

		if len(report.Workplan.ExclusionReasons) > 0 {
			sb.WriteString("- **Exclusion Reasons:**\n")
			for reason, count := range report.Workplan.ExclusionReasons {
				fmt.Fprintf(&sb, "  - %s: %d files\n", reason, count)
			}
		}

		sb.WriteString("\n### Category Planning\n\n")
		fmt.Fprintf(&sb, "- **Categories Planned:** %s\n", strings.Join(report.Workplan.CategoriesPlanned, ", "))
		if len(report.Workplan.CategoriesSkipped) > 0 {
			fmt.Fprintf(&sb, "- **Categories Skipped:** %s\n", strings.Join(report.Workplan.CategoriesSkipped, ", "))
			if len(report.Workplan.SkipReasons) > 0 {
				sb.WriteString("- **Skip Reasons:**\n")
				for category, reason := range report.Workplan.SkipReasons {
					fmt.Fprintf(&sb, "  - %s: %s\n", category, reason)
				}
			}
		}

		sb.WriteString("\n### Discovery Patterns\n\n")
		if len(report.Workplan.DiscoveryPatterns.Include) > 0 {
			fmt.Fprintf(&sb, "- **Include Patterns:** %s\n", strings.Join(report.Workplan.DiscoveryPatterns.Include, ", "))
		}
		if len(report.Workplan.DiscoveryPatterns.Exclude) > 0 {
			fmt.Fprintf(&sb, "- **Exclude Patterns:** %s\n", strings.Join(report.Workplan.DiscoveryPatterns.Exclude, ", "))
		}
		if len(report.Workplan.DiscoveryPatterns.ForceInclude) > 0 {
			fmt.Fprintf(&sb, "- **Force Include Patterns:** %s\n", strings.Join(report.Workplan.DiscoveryPatterns.ForceInclude, ", "))
		}

		sb.WriteString("\n### Execution Summary\n\n")
		fmt.Fprintf(&sb, "- **Worker Count:** %d\n", report.Workplan.ExecutionSummary.WorkerCount)
		fmt.Fprintf(&sb, "- **Total Runtime:** %s\n", f.formatDuration(time.Duration(report.Workplan.ExecutionSummary.TotalRuntime)))

		if len(report.Workplan.ExecutionSummary.CategoryRuntimes) > 0 {
			sb.WriteString("- **Category Runtimes:**\n")
			for category, duration := range report.Workplan.ExecutionSummary.CategoryRuntimes {
				fmt.Fprintf(&sb, "  - %s: %s\n", category, f.formatDuration(time.Duration(duration)))
			}
		}

		if len(report.Workplan.FileList) > 0 && len(report.Workplan.FileList) <= 20 {
			sb.WriteString("\n### Files Processed\n\n")
			for _, file := range report.Workplan.FileList {
				fmt.Fprintf(&sb, "- %s\n", file)
			}
		} else if len(report.Workplan.FileList) > 20 {
			fmt.Fprintf(&sb, "\n### Files Processed\n\n_Showing first 20 of %d files processed. Use --format json for complete list._\n\n", len(report.Workplan.FileList))
			for _, file := range report.Workplan.FileList[:20] {
				fmt.Fprintf(&sb, "- %s\n", file)
			}
		}
		sb.WriteString("\n")
	}

	// Footer
	sb.WriteString("---\n\n")
	sb.WriteString("*Report generated by goneat assess*")

	return sb.String()
}

// formatJSON creates a JSON-formatted assessment report
func (f *Formatter) formatJSON(report *AssessmentReport) (string, error) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return string(data), nil
}

// Helper methods for formatting

func (f *Formatter) formatHealthScore(score float64) string {
	percentage := int(score * 100)
	if percentage >= 90 {
		return fmt.Sprintf("🟢 Excellent (%d%%)", percentage)
	} else if percentage >= 75 {
		return fmt.Sprintf("🟡 Good (%d%%)", percentage)
	} else if percentage >= 60 {
		return fmt.Sprintf("🟠 Fair (%d%%)", percentage)
	} else {
		return fmt.Sprintf("🔴 Poor (%d%%)", percentage)
	}
}

func (f *Formatter) formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0f seconds", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.0f minutes", d.Minutes())
	} else {
		return fmt.Sprintf("%.1f hours", d.Hours())
	}
}

func (f *Formatter) formatBool(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}

func (f *Formatter) formatCategoryMetricsMarkdown(category string, metrics map[string]interface{}) string {
	if metrics == nil {
		return ""
	}
	if strings.TrimSpace(category) != string(CategorySchema) {
		return ""
	}

	var lines []string

	if n, ok := asInt(metrics["schema_candidate_files"]); ok {
		lines = append(lines, fmt.Sprintf("- **Candidate files:** %d", n))
	}
	if n, ok := asInt(metrics["schema_validated_files"]); ok {
		lines = append(lines, fmt.Sprintf("- **Schemas validated:** %d", n))
	}
	if n, ok := asInt(metrics["schema_validation_workers"]); ok {
		lines = append(lines, fmt.Sprintf("- **Workers:** %d", n))
	}
	if s, ok := asString(metrics["schema_validation_duration"]); ok {
		lines = append(lines, fmt.Sprintf("- **Validation time:** %s", s))
	}
	if v, ok := asFloat(metrics["schema_validation_files_per_sec"]); ok {
		lines = append(lines, fmt.Sprintf("- **Throughput:** %.2f files/sec", v))
	}
	if b, ok := asBool(metrics["schema_meta_validation_enabled"]); ok {
		lines = append(lines, fmt.Sprintf("- **Meta-validation:** %s", f.formatBool(b)))
	}
	if n, ok := asInt(metrics["schema_meta_validators_compiled"]); ok {
		lines = append(lines, fmt.Sprintf("- **Meta-schema drafts compiled:** %d", n))
	}

	// Schema mapping headline metrics (if enabled)
	if n, ok := asInt(metrics["schema_mapping_files_evaluated"]); ok {
		lines = append(lines, fmt.Sprintf("- **Mapping files evaluated:** %d", n))
	}
	if n, ok := asInt(metrics["schema_mapping_mapped"]); ok {
		lines = append(lines, fmt.Sprintf("- **Mapping resolved:** %d", n))
	}
	if n, ok := asInt(metrics["schema_mapping_unmapped"]); ok {
		lines = append(lines, fmt.Sprintf("- **Mapping unmapped:** %d", n))
	}
	if v, ok := asFloat(metrics["schema_mapping_detection_rate"]); ok {
		lines = append(lines, fmt.Sprintf("- **Mapping detection rate:** %.0f%%", v*100))
	}

	if len(lines) == 0 {
		return ""
	}

	return "**Metrics:**\n" + strings.Join(lines, "\n") + "\n"
}

func asInt(v interface{}) (int, bool) {
	switch t := v.(type) {
	case int:
		return t, true
	case int64:
		return int(t), true
	case float64:
		return int(t), true
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return int(i), true
		}
	case string:
		if i, err := strconv.Atoi(strings.TrimSpace(t)); err == nil {
			return i, true
		}
	}
	return 0, false
}

func asFloat(v interface{}) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case json.Number:
		if f, err := t.Float64(); err == nil {
			return f, true
		}
	case string:
		if f, err := strconv.ParseFloat(strings.TrimSpace(t), 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

func asBool(v interface{}) (bool, bool) {
	switch t := v.(type) {
	case bool:
		return t, true
	case string:
		s := strings.TrimSpace(strings.ToLower(t))
		if s == "true" || s == "yes" || s == "1" {
			return true, true
		}
		if s == "false" || s == "no" || s == "0" {
			return false, true
		}
	}
	return false, false
}

func asString(v interface{}) (string, bool) {
	switch t := v.(type) {
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return "", false
		}
		return s, true
	case fmt.Stringer:
		s := strings.TrimSpace(t.String())
		if s == "" {
			return "", false
		}
		return s, true
	}
	return "", false
}

func (f *Formatter) getStatusEmoji(status string) string {
	switch status {
	case "success":
		if status == "success" {
			return "✅"
		}
		return "⚠️"
	case "error":
		return "❌"
	case "skipped":
		return "⏭️"
	default:
		return "❓"
	}
}

func (f *Formatter) formatCategories(categories []AssessmentCategory) string {
	var names []string
	for _, cat := range categories {
		names = append(names, string(cat))
	}
	return strings.Join(names, ", ")
}

func (f *Formatter) getOrderedCategories(categoryResults map[string]CategoryResult) []string {
	// Simple ordering by priority (lower number = higher priority)
	type categoryInfo struct {
		name     string
		priority int
	}

	var categories []categoryInfo
	for name, result := range categoryResults {
		categories = append(categories, categoryInfo{name: name, priority: result.Priority})
	}

	// Sort by priority
	for i := 0; i < len(categories)-1; i++ {
		for j := i + 1; j < len(categories); j++ {
			if categories[i].priority > categories[j].priority {
				categories[i], categories[j] = categories[j], categories[i]
			}
		}
	}

	var ordered []string
	for _, cat := range categories {
		ordered = append(ordered, cat.name)
	}

	return ordered
}

// titleCase converts a string to title case (first letter of each word capitalized)
func titleCase(s string) string {
	words := strings.Fields(s)
	for i, word := range words {
		if len(word) > 0 {
			runes := []rune(word)
			runes[0] = unicode.ToUpper(runes[0])
			words[i] = string(runes)
		}
	}
	return strings.Join(words, " ")
}

// formatHTML creates a comprehensive HTML-formatted assessment report with transparency features
func (f *Formatter) formatHTML(report *AssessmentReport) string {
	// Always generate JSON first, then convert to HTML
	jsonData, err := f.formatJSON(report)
	if err != nil {
		return fmt.Sprintf("Error generating JSON: %v", err)
	}

	// Parse JSON back to structured data for HTML processing
	var jsonReport AssessmentReport
	if err := json.Unmarshal([]byte(jsonData), &jsonReport); err != nil {
		return fmt.Sprintf("Error parsing JSON: %v", err)
	}

	return f.generateHTMLFromJSON(&jsonReport)
}

// generateHTMLFromJSON creates HTML from structured JSON data using Handlebars templates
func (f *Formatter) generateHTMLFromJSON(report *AssessmentReport) string {
	// Extract project information
	projectName, version, displayPath := f.extractProjectInfo(f.targetPath)

	// Group issues by file (expand repository/git-state sentinel using metrics when available)
	fileGroupsMap := make(map[string][]TemplateIssue)
	for _, category := range report.Categories {
		for _, issue := range category.Issues {
			// Expand sentinel to actual files when metrics provide them
			if (issue.File == "repository" || issue.File == ".git") && (category.Metrics != nil) {
				if sample := extractSampleFiles(category.Metrics); len(sample) > 0 {
					for _, fn := range sample {
						templateIssue := TemplateIssue{
							Line:     issue.Line,
							Severity: string(issue.Severity),
							Category: string(issue.Category),
							Message:  issue.Message,
						}
						fileGroupsMap[fn] = append(fileGroupsMap[fn], templateIssue)
					}
					continue
				}
			}

			templateIssue := TemplateIssue{
				Line:     issue.Line,
				Severity: string(issue.Severity),
				Category: string(issue.Category),
				Message:  issue.Message,
			}
			fileGroupsMap[issue.File] = append(fileGroupsMap[issue.File], templateIssue)
		}
	}

	// Convert to FileIssues array
	var fileGroups []FileIssues
	for filename, issues := range fileGroupsMap {
		fileGroups = append(fileGroups, FileIssues{
			Filename: filename,
			Issues:   issues,
			Count:    len(issues),
		})
	}

	// Prepare template data
	templateData := TemplateData{
		Project: ProjectInfo{
			Name:        projectName,
			Version:     version,
			DisplayPath: displayPath,
		},
		Metadata: TemplateMetadata{
			Version:       report.Metadata.Version,
			GeneratedAt:   report.Metadata.GeneratedAt.Format(time.RFC3339),
			ExecutionTime: report.Metadata.ExecutionTime.String(),
		},
		Summary: TemplateSummary{
			HealthPercent:        fmt.Sprintf("%.0f", report.Summary.OverallHealth*100),
			TotalIssues:          report.Summary.TotalIssues,
			CriticalIssues:       report.Summary.CriticalIssues,
			EstimatedTimeMinutes: fmt.Sprintf("%.0f", time.Duration(report.Summary.EstimatedTime).Minutes()),
		},
		FileGroups: fileGroups,
	}

	// Load and render Handlebars template
	// Get the directory of the current executable
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Sprintf("Error getting executable path: %v", err)
	}
	execDir := filepath.Dir(execPath)

	// Allow explicit override via environment variable
	if envPath := os.Getenv("GONEAT_TEMPLATE_PATH"); strings.TrimSpace(envPath) != "" {
		envPath = filepath.Clean(envPath)                     // Path validation for G304
		if content, err := os.ReadFile(envPath); err == nil { // #nosec G703 - envPath from GONEAT_TEMPLATE_PATH env var, filepath.Clean applied above
			return renderHandlebars(string(content), templateData)
		}
	}

	// Try common locations relative to the executable
	candidatePaths := []string{
		filepath.Join(execDir, "templates", "report.html"),               // dist/templates/report.html
		filepath.Join(filepath.Dir(execDir), "templates", "report.html"), // ../templates/report.html (repo root)
		filepath.Join(execDir, "report.html"),                            // dist/report.html
		filepath.Join("templates", "report.html"),                        // cwd/templates/report.html
	}
	var templateContent []byte
	var readErr error
	for _, p := range candidatePaths {
		p = filepath.Clean(p) // Path validation for G304
		if content, err := os.ReadFile(p); err == nil {
			templateContent = content
			readErr = nil
			break
		} else {
			readErr = err
		}
	}
	if templateContent == nil {
		return fmt.Sprintf("Error loading template: %v (tried: %s)", readErr, strings.Join(candidatePaths, ", "))
	}

	// Render template
	return renderHandlebars(string(templateContent), templateData)
}

// renderHandlebars renders a Handlebars template string with helpers registered
func renderHandlebars(tpl string, data interface{}) string {
	// Register helper functions
	raymond.RegisterHelper("gt", func(a, b interface{}) bool {
		aVal, _ := strconv.Atoi(fmt.Sprintf("%v", a))
		bVal, _ := strconv.Atoi(fmt.Sprintf("%v", b))
		return aVal > bVal
	})
	out, err := raymond.Render(tpl, data)
	if err != nil {
		return fmt.Sprintf("Error rendering template: %v", err)
	}
	return out
}

// extractProjectInfo extracts project name, version, and formatted path
func (f *Formatter) extractProjectInfo(targetPath string) (projectName, version, displayPath string) {
	// Get absolute path
	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		absPath = targetPath
	}

	// Convert to user-friendly path (~/)
	homeDir, err := os.UserHomeDir()
	if err == nil && strings.HasPrefix(absPath, homeDir) {
		displayPath = "~" + strings.TrimPrefix(absPath, homeDir)
	} else {
		displayPath = absPath
	}

	// Extract project name from go.mod or directory name
	projectName = filepath.Base(absPath)

	// Try to read go.mod for project name
	gomodPath := filepath.Join(absPath, "go.mod")
	gomodPath = filepath.Clean(gomodPath) // Path validation for G304
	if content, err := os.ReadFile(gomodPath); err == nil {
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "module ") {
				// Extract module name, get the last part
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					moduleParts := strings.Split(parts[1], "/")
					last := moduleParts[len(moduleParts)-1]
					// If the last segment is a version suffix like v2, v3, use the previous segment
					if len(moduleParts) >= 2 && len(last) >= 2 && last[0] == 'v' {
						allDigits := true
						for i := 1; i < len(last); i++ {
							if last[i] < '0' || last[i] > '9' {
								allDigits = false
								break
							}
						}
						if allDigits {
							projectName = moduleParts[len(moduleParts)-2]
						} else {
							projectName = last
						}
					} else {
						projectName = last
					}
				}
				break
			}
		}
	}

	// Try to extract version from VERSION file
	version = "unknown"
	versionPath := filepath.Join(absPath, "VERSION")
	versionPath = filepath.Clean(versionPath) // Path validation for G304
	if content, err := os.ReadFile(versionPath); err == nil {
		version = strings.TrimSpace(string(content))
	}

	// Try to extract version from version.go or similar files
	versionFiles := []string{"version.go", "cmd/version/version.go", "pkg/version/version.go"}
	for _, vf := range versionFiles {
		vfPath := filepath.Join(absPath, vf)
		vfPath = filepath.Clean(vfPath) // Path validation for G304
		if content, err := os.ReadFile(vfPath); err == nil {
			// Simple regex to find version constants
			lines := strings.Split(string(content), "\n")
			for _, line := range lines {
				if strings.Contains(line, "Version") && (strings.Contains(line, "const") || strings.Contains(line, "var")) {
					parts := strings.Split(line, "=")
					if len(parts) >= 2 {
						version = strings.Trim(strings.TrimSpace(parts[1]), `"`)
						break
					}
				}
			}
			if version != "unknown" {
				break
			}
		}
	}

	return projectName, version, displayPath
}

// TemplateData represents the data structure for Handlebars templates
type TemplateData struct {
	Project    ProjectInfo      `json:"project"`
	Metadata   TemplateMetadata `json:"metadata"`
	Summary    TemplateSummary  `json:"summary"`
	FileGroups []FileIssues     `json:"fileGroups,omitempty"`
}

type ProjectInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	DisplayPath string `json:"displayPath"`
}

type TemplateMetadata struct {
	Version       string `json:"version"`
	GeneratedAt   string `json:"generatedAt"`
	ExecutionTime string `json:"executionTime"`
}

type TemplateSummary struct {
	HealthPercent        string `json:"healthPercent"`
	TotalIssues          int    `json:"totalIssues"`
	CriticalIssues       int    `json:"criticalIssues"`
	EstimatedTimeMinutes string `json:"estimatedTimeMinutes"`
}

type TemplateIssue struct {
	Line     int    `json:"line"`
	Severity string `json:"severity"`
	Category string `json:"category"`
	Message  string `json:"message"`
}

type FileIssues struct {
	Filename string          `json:"filename"`
	Issues   []TemplateIssue `json:"issues"`
	Count    int             `json:"count"`
}

// extractSampleFiles tries to extract a list of filenames from common metrics keys
func extractSampleFiles(metrics map[string]interface{}) []string {
	if metrics == nil {
		return nil
	}
	// Preferred keys in order
	keys := []string{"uncommitted_files", "git_dirty_files"}
	for _, k := range keys {
		if v, ok := metrics[k]; ok {
			switch vv := v.(type) {
			case []string:
				return vv
			case []interface{}:
				var out []string
				for _, it := range vv {
					if s, ok2 := it.(string); ok2 {
						out = append(out, s)
					}
				}
				if len(out) > 0 {
					return out
				}
			}
		}
	}
	return nil
}
