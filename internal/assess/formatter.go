/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package assess

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode"
)

// OutputFormat represents the format for assessment output
type OutputFormat string

const (
	FormatMarkdown OutputFormat = "markdown"
	FormatJSON     OutputFormat = "json"
	FormatBoth     OutputFormat = "both"
)

// Formatter handles formatting assessment reports
type Formatter struct {
	format OutputFormat
}

// NewFormatter creates a new report formatter
func NewFormatter(format OutputFormat) *Formatter {
	return &Formatter{format: format}
}

// FormatReport formats an assessment report according to the configured format
func (f *Formatter) FormatReport(report *AssessmentReport) (string, error) {
	switch f.format {
	case FormatMarkdown:
		return f.formatMarkdown(report), nil
	case FormatJSON:
		return f.formatJSON(report)
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
	sb.WriteString(fmt.Sprintf("**Generated:** %s\n", report.Metadata.GeneratedAt.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("**Tool:** %s\n", report.Metadata.Tool))
	sb.WriteString(fmt.Sprintf("**Version:** %s\n", report.Metadata.Version))
	sb.WriteString(fmt.Sprintf("**Target:** %s\n", report.Metadata.Target))
	sb.WriteString(fmt.Sprintf("**Execution Time:** %v\n\n", report.Metadata.ExecutionTime))

	// Executive Summary
	sb.WriteString("## Executive Summary\n\n")
	sb.WriteString(fmt.Sprintf("- **Overall Health:** %s\n", f.formatHealthScore(report.Summary.OverallHealth)))
	sb.WriteString(fmt.Sprintf("- **Critical Issues:** %d\n", report.Summary.CriticalIssues))
	sb.WriteString(fmt.Sprintf("- **Total Issues:** %d\n", report.Summary.TotalIssues))
	sb.WriteString(fmt.Sprintf("- **Estimated Fix Time:** %s\n", f.formatDuration(report.Summary.EstimatedTime)))
	sb.WriteString(fmt.Sprintf("- **Parallelizable Tasks:** %d groups identified\n", report.Summary.ParallelGroups))
	sb.WriteString(fmt.Sprintf("- **Categories with Issues:** %d\n\n", report.Summary.CategoriesWithIssues))

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
		sb.WriteString(fmt.Sprintf("### %s %s Issues (Priority: %d)\n\n", statusEmoji, titleCase(category), result.Priority))

		if result.Status == "error" {
			sb.WriteString(fmt.Sprintf("**Status:** Error - %s\n\n", result.Error))
			continue
		}

		sb.WriteString(fmt.Sprintf("**Status:** %d issues found\n", result.IssueCount))
		sb.WriteString(fmt.Sprintf("**Estimated Time:** %s\n", f.formatDuration(result.EstimatedTime)))
		sb.WriteString(fmt.Sprintf("**Parallelizable:** %s\n\n", f.formatBool(result.Parallelizable)))

		if result.IssueCount > 0 {
			// Issues table
			sb.WriteString("| File | Line | Severity | Message | Auto-fixable |\n")
			sb.WriteString("|------|------|----------|---------|--------------|\n")

			for _, issue := range result.Issues {
				line := ""
				if issue.Line > 0 {
					line = fmt.Sprintf("%d", issue.Line)
				}
				sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
					issue.File, line, issue.Severity, issue.Message, f.formatBool(issue.AutoFixable)))
			}
			sb.WriteString("\n")
		}
	}

	// Recommended Workflow
	if len(report.Workflow.Phases) > 0 {
		sb.WriteString("## Recommended Workflow\n\n")

		for i, phase := range report.Workflow.Phases {
			sb.WriteString(fmt.Sprintf("### Phase %d: %s\n\n", i+1, phase.Description))
			sb.WriteString(fmt.Sprintf("**Estimated Time:** %s\n", f.formatDuration(phase.EstimatedTime)))
			sb.WriteString(fmt.Sprintf("**Categories:** %s\n", f.formatCategories(phase.Categories)))

			if len(phase.ParallelGroups) > 0 {
				sb.WriteString(fmt.Sprintf("**Parallel Groups:** %s\n", strings.Join(phase.ParallelGroups, ", ")))
			}
			sb.WriteString("\n")
		}

		sb.WriteString(fmt.Sprintf("**Total Estimated Time:** %s\n\n", f.formatDuration(report.Workflow.TotalTime)))
	}

	// Parallelization Opportunities
	if len(report.Workflow.ParallelGroups) > 0 {
		sb.WriteString("## Parallelization Opportunities\n\n")

		for _, group := range report.Workflow.ParallelGroups {
			sb.WriteString(fmt.Sprintf("### %s\n", group.Name))
			sb.WriteString(fmt.Sprintf("**Description:** %s\n", group.Description))
			sb.WriteString(fmt.Sprintf("**Files:** %s\n", strings.Join(group.Files, ", ")))
			sb.WriteString(fmt.Sprintf("**Categories:** %s\n", f.formatCategories(group.Categories)))
			sb.WriteString(fmt.Sprintf("**Issues:** %d\n", group.IssueCount))
			sb.WriteString(fmt.Sprintf("**Estimated Time:** %s\n\n", f.formatDuration(group.EstimatedTime)))
		}
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
