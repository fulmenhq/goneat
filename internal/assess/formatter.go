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
	sb.WriteString(fmt.Sprintf("%s %s | total issues: %d | time: %s\n",
		bold("Assessment"), fmt.Sprintf("health=%s", healthStr), report.Summary.TotalIssues, report.Metadata.ExecutionTime))

	// Fail-on context if present in commands_run hint
	// We don't have explicit fail-on in metadata yet; detect from commands or print default marker
	failOn := os.Getenv("GONEAT_SECURITY_FAIL_ON")
	if strings.TrimSpace(failOn) == "" {
		// best-effort default display; actual enforcement happens in caller
		failOn = "configured"
	}
	sb.WriteString(fmt.Sprintf(" - Fail-on: %s\n", failOn))

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

		sb.WriteString(fmt.Sprintf(" - %s: %s (est %s)\n", titleCase(cat), statusStr, f.formatDuration(res.EstimatedTime)))

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
				sb.WriteString(fmt.Sprintf("   files: %s%s\n", strings.Join(shown, ", "), more))
			} else if len(res.Issues) > 0 {
				// Fallback: show first issue message if no file paths were captured
				msg := strings.TrimSpace(res.Issues[0].Message)
				if msg != "" {
					sb.WriteString(fmt.Sprintf("   note: %s\n", msg))
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
			sb.WriteString(fmt.Sprintf("   %s %s\n", red("!"), res.Error))
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
				sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
					issue.File, line, issue.Severity, issue.Message, f.formatBool(issue.AutoFixable)))
			}
			if maxToShow < len(result.Issues) {
				sb.WriteString(fmt.Sprintf("\n_Showing %d of %d issues. Use --format json for full details._\n", maxToShow, len(result.Issues)))
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

	// Allow explicit override via environment variable
	if envPath := os.Getenv("GONEAT_TEMPLATE_PATH"); strings.TrimSpace(envPath) != "" {
		envPath = filepath.Clean(envPath) // Path validation for G304
		if content, err := os.ReadFile(envPath); err == nil {
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
