/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package assess

import (
	"encoding/json"
	"fmt"
	"time"
)

// HumanReadableDuration wraps time.Duration to provide human-readable JSON serialization
type HumanReadableDuration time.Duration

// MarshalJSON implements json.Marshaler for human-readable duration output
func (d HumanReadableDuration) MarshalJSON() ([]byte, error) {
	duration := time.Duration(d)
	if duration < time.Minute {
		return json.Marshal(fmt.Sprintf("%.0fs", duration.Seconds()))
	} else if duration < time.Hour {
		return json.Marshal(fmt.Sprintf("%.0fm", duration.Minutes()))
	} else {
		return json.Marshal(fmt.Sprintf("%.1fh", duration.Hours()))
	}
}

// UnmarshalJSON implements json.Unmarshaler for human-readable duration input
func (d *HumanReadableDuration) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	// Parse the human-readable string back to duration
	// Simple parsing for common formats
	if len(s) > 0 {
		switch s[len(s)-1] {
		case 's':
			if val, err := time.ParseDuration(s); err == nil {
				*d = HumanReadableDuration(val)
				return nil
			}
		case 'm':
			if val, err := time.ParseDuration(s); err == nil {
				*d = HumanReadableDuration(val)
				return nil
			}
		case 'h':
			if val, err := time.ParseDuration(s); err == nil {
				*d = HumanReadableDuration(val)
				return nil
			}
		}
	}

	// Fallback to parsing as duration string
	if val, err := time.ParseDuration(s); err == nil {
		*d = HumanReadableDuration(val)
		return nil
	}

	return fmt.Errorf("invalid duration format: %s", s)
}

// Duration returns the underlying time.Duration
func (d HumanReadableDuration) Duration() time.Duration {
	return time.Duration(d)
}

// String returns the human-readable string representation
func (d HumanReadableDuration) String() string {
	duration := time.Duration(d)
	if duration < time.Minute {
		return fmt.Sprintf("%.0fs", duration.Seconds())
	} else if duration < time.Hour {
		return fmt.Sprintf("%.0fm", duration.Minutes())
	} else {
		return fmt.Sprintf("%.1fh", duration.Hours())
	}
}

// AssessmentCategory represents the type of code assessment
type AssessmentCategory string

const (
	CategoryFormat         AssessmentCategory = "format"
	CategoryLint           AssessmentCategory = "lint"
	CategoryStaticAnalysis AssessmentCategory = "static-analysis"
	CategorySecurity       AssessmentCategory = "security"
	CategoryPerformance    AssessmentCategory = "performance"
	CategorySchema         AssessmentCategory = "schema"
	CategoryDates          AssessmentCategory = "dates"
	CategoryTools          AssessmentCategory = "tools"
	CategoryMaturity       AssessmentCategory = "maturity"
	CategoryRepoStatus     AssessmentCategory = "repo-status"
	CategoryDependencies   AssessmentCategory = "dependencies"
)

// IssueSeverity represents the severity level of an assessment issue
type IssueSeverity string

const (
	SeverityCritical IssueSeverity = "critical"
	SeverityHigh     IssueSeverity = "high"
	SeverityMedium   IssueSeverity = "medium"
	SeverityLow      IssueSeverity = "low"
	SeverityInfo     IssueSeverity = "info"
)

// Issue represents a single assessment finding
type Issue struct {
	File          string                `json:"file"`
	Line          int                   `json:"line,omitempty"`
	Column        int                   `json:"column,omitempty"`
	Severity      IssueSeverity         `json:"severity"`
	Message       string                `json:"message"`
	Category      AssessmentCategory    `json:"category"`
	SubCategory   string                `json:"sub_category,omitempty"`
	AutoFixable   bool                  `json:"auto_fixable"`
	EstimatedTime HumanReadableDuration `json:"estimated_time,omitempty"`
	ChangeRelated bool                  `json:"change_related,omitempty"`
	LinesModified []int                 `json:"lines_modified,omitempty"`
}

// CategoryResult represents the assessment results for a specific category
type CategoryResult struct {
	Category          AssessmentCategory     `json:"category"`
	Priority          int                    `json:"priority"`
	Issues            []Issue                `json:"issues"`
	IssueCount        int                    `json:"issue_count"`
	EstimatedTime     HumanReadableDuration  `json:"estimated_time"`
	Parallelizable    bool                   `json:"parallelizable"`
	Status            string                 `json:"status"` // "success", "error", "skipped", "issues"
	Error             string                 `json:"error,omitempty"`
	Metrics           map[string]interface{} `json:"metrics,omitempty"`
	SuppressionReport *SuppressionReport     `json:"suppression_report,omitempty"`
}

// AssessmentResult represents the complete result from running an assessment
type AssessmentResult struct {
	CommandName   string                 `json:"command_name"`
	Category      AssessmentCategory     `json:"category"`
	Success       bool                   `json:"success"`
	ExecutionTime HumanReadableDuration  `json:"execution_time"`
	Issues        []Issue                `json:"issues"`
	Error         string                 `json:"error,omitempty"`
	Metrics       map[string]interface{} `json:"metrics,omitempty"`
}

// WorkflowPhase represents a phase in the remediation workflow
type WorkflowPhase struct {
	Name           string                `json:"name"`
	Description    string                `json:"description"`
	EstimatedTime  HumanReadableDuration `json:"estimated_time"`
	Categories     []AssessmentCategory  `json:"categories"`
	ParallelGroups []string              `json:"parallel_groups,omitempty"`
	Priority       int                   `json:"priority"`
}

// ParallelGroup represents a group of tasks that can be worked on in parallel
type ParallelGroup struct {
	Name          string                `json:"name"`
	Description   string                `json:"description"`
	Files         []string              `json:"files"`
	Categories    []AssessmentCategory  `json:"categories"`
	EstimatedTime HumanReadableDuration `json:"estimated_time"`
	IssueCount    int                   `json:"issue_count"`
}

// AssessmentReport represents the complete assessment report
type AssessmentReport struct {
	Metadata   ReportMetadata            `json:"metadata"`
	Summary    ReportSummary             `json:"summary"`
	Categories map[string]CategoryResult `json:"categories"`
	Workflow   WorkflowPlan              `json:"workflow"`
	Workplan   *ExtendedWorkplan         `json:"workplan,omitempty"` // Only included when --extended is used
}

// ReportMetadata contains metadata about the assessment run
type ReportMetadata struct {
	GeneratedAt   time.Time             `json:"generated_at"`
	Tool          string                `json:"tool"`
	Version       string                `json:"version"`
	Target        string                `json:"target"`
	ExecutionTime HumanReadableDuration `json:"execution_time"`
	CommandsRun   []string              `json:"commands_run"`
	FailOn        string                `json:"fail_on,omitempty"`
	ChangeContext *ChangeContext        `json:"change_context,omitempty"`
}

// ReportSummary provides high-level assessment statistics
type ReportSummary struct {
	OverallHealth        float64               `json:"overall_health"` // 0.0 to 1.0
	CriticalIssues       int                   `json:"critical_issues"`
	TotalIssues          int                   `json:"total_issues"`
	EstimatedTime        HumanReadableDuration `json:"estimated_time"`
	ParallelGroups       int                   `json:"parallel_groups"`
	CategoriesWithIssues int                   `json:"categories_with_issues"`
}

// ChangeContext mirrors internal/gitctx.ChangeContext for report embedding
type ChangeContext struct {
	ModifiedFiles []string `json:"modified_files"`
	TotalChanges  int      `json:"total_changes"`
	ChangeScope   string   `json:"change_scope"`
	GitSHA        string   `json:"git_sha,omitempty"`
	Branch        string   `json:"branch,omitempty"`
}

// WorkflowPlan contains the recommended remediation workflow
type WorkflowPlan struct {
	Phases         []WorkflowPhase       `json:"phases"`
	ParallelGroups []ParallelGroup       `json:"parallel_groups"`
	TotalTime      HumanReadableDuration `json:"total_time"`
}

// ExtendedWorkplan provides detailed execution and discovery information
type ExtendedWorkplan struct {
	FilesDiscovered   int                   `json:"files_discovered"`
	FilesIncluded     int                   `json:"files_included"`
	FilesExcluded     int                   `json:"files_excluded"`
	ExclusionReasons  map[string]int        `json:"exclusion_reasons"`
	CategoriesPlanned []string              `json:"categories_planned"`
	CategoriesSkipped []string              `json:"categories_skipped"`
	SkipReasons       map[string]string     `json:"skip_reasons"`
	EstimatedDuration HumanReadableDuration `json:"estimated_duration"`
	FileList          []string              `json:"file_list"`
	DiscoveryPatterns DiscoveryPatterns     `json:"discovery_patterns"`
	ExecutionSummary  ExecutionSummary      `json:"execution_summary"`
}

// DiscoveryPatterns shows the patterns used for file discovery
type DiscoveryPatterns struct {
	Include      []string `json:"include"`
	Exclude      []string `json:"exclude"`
	ForceInclude []string `json:"force_include,omitempty"`
}

// ExecutionSummary provides execution details per category
type ExecutionSummary struct {
	WorkerCount      int                              `json:"worker_count"`
	CategoryRuntimes map[string]HumanReadableDuration `json:"category_runtimes"`
	TotalRuntime     HumanReadableDuration            `json:"total_runtime"`
}
