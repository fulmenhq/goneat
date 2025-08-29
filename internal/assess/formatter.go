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

	// Group issues by file
	fileGroupsMap := make(map[string][]TemplateIssue)
	for _, category := range report.Categories {
		for _, issue := range category.Issues {
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
			EstimatedTimeMinutes: fmt.Sprintf("%.0f", report.Summary.EstimatedTime.Minutes()),
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
	templatePath := filepath.Join(execDir, "templates", "report.html")

	templateContent, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Sprintf("Error loading template: %v (path: %s)", err, templatePath)
	}

	// Register helper functions
	raymond.RegisterHelper("gt", func(a, b interface{}) bool {
		aVal, _ := strconv.Atoi(fmt.Sprintf("%v", a))
		bVal, _ := strconv.Atoi(fmt.Sprintf("%v", b))
		return aVal > bVal
	})

	// Render template
	result, err := raymond.Render(string(templateContent), templateData)
	if err != nil {
		return fmt.Sprintf("Error rendering template: %v", err)
	}

	return result
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
	if content, err := os.ReadFile(gomodPath); err == nil {
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "module ") {
				// Extract module name, get the last part
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					moduleParts := strings.Split(parts[1], "/")
					projectName = moduleParts[len(moduleParts)-1]
				}
				break
			}
		}
	}

	// Try to extract version from VERSION file
	version = "unknown"
	versionPath := filepath.Join(absPath, "VERSION")
	if content, err := os.ReadFile(versionPath); err == nil {
		version = strings.TrimSpace(string(content))
	}

	// Try to extract version from version.go or similar files
	versionFiles := []string{"version.go", "cmd/version/version.go", "pkg/version/version.go"}
	for _, vf := range versionFiles {
		vfPath := filepath.Join(absPath, vf)
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
