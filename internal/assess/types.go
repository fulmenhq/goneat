/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package assess

import (
	"time"
)

// AssessmentCategory represents the type of code assessment
type AssessmentCategory string

const (
	CategoryFormat         AssessmentCategory = "format"
	CategoryLint           AssessmentCategory = "lint"
	CategorySecurity       AssessmentCategory = "security"
	CategoryPerformance    AssessmentCategory = "performance"
	CategoryStaticAnalysis AssessmentCategory = "static-analysis"
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
	File          string             `json:"file"`
	Line          int                `json:"line,omitempty"`
	Column        int                `json:"column,omitempty"`
	Severity      IssueSeverity      `json:"severity"`
	Message       string             `json:"message"`
	Category      AssessmentCategory `json:"category"`
	SubCategory   string             `json:"sub_category,omitempty"`
	AutoFixable   bool               `json:"auto_fixable"`
	EstimatedTime time.Duration      `json:"estimated_time,omitempty"`
}

// CategoryResult represents the assessment results for a specific category
type CategoryResult struct {
	Category       AssessmentCategory `json:"category"`
	Priority       int                `json:"priority"`
	Issues         []Issue            `json:"issues"`
	IssueCount     int                `json:"issue_count"`
	EstimatedTime  time.Duration      `json:"estimated_time"`
	Parallelizable bool               `json:"parallelizable"`
	Status         string             `json:"status"` // "success", "error", "skipped"
	Error          string             `json:"error,omitempty"`
}

// AssessmentResult represents the complete result from running an assessment
type AssessmentResult struct {
	CommandName   string             `json:"command_name"`
	Category      AssessmentCategory `json:"category"`
	Success       bool               `json:"success"`
	ExecutionTime time.Duration      `json:"execution_time"`
	Issues        []Issue            `json:"issues"`
	Error         string             `json:"error,omitempty"`
}

// WorkflowPhase represents a phase in the remediation workflow
type WorkflowPhase struct {
	Name           string               `json:"name"`
	Description    string               `json:"description"`
	EstimatedTime  time.Duration        `json:"estimated_time"`
	Categories     []AssessmentCategory `json:"categories"`
	ParallelGroups []string             `json:"parallel_groups,omitempty"`
	Priority       int                  `json:"priority"`
}

// ParallelGroup represents a group of tasks that can be worked on in parallel
type ParallelGroup struct {
	Name          string               `json:"name"`
	Description   string               `json:"description"`
	Files         []string             `json:"files"`
	Categories    []AssessmentCategory `json:"categories"`
	EstimatedTime time.Duration        `json:"estimated_time"`
	IssueCount    int                  `json:"issue_count"`
}

// AssessmentReport represents the complete assessment report
type AssessmentReport struct {
	Metadata   ReportMetadata            `json:"metadata"`
	Summary    ReportSummary             `json:"summary"`
	Categories map[string]CategoryResult `json:"categories"`
	Workflow   WorkflowPlan              `json:"workflow"`
}

// ReportMetadata contains metadata about the assessment run
type ReportMetadata struct {
	GeneratedAt   time.Time     `json:"generated_at"`
	Tool          string        `json:"tool"`
	Version       string        `json:"version"`
	Target        string        `json:"target"`
	ExecutionTime time.Duration `json:"execution_time"`
	CommandsRun   []string      `json:"commands_run"`
}

// ReportSummary provides high-level assessment statistics
type ReportSummary struct {
	OverallHealth        float64       `json:"overall_health"` // 0.0 to 1.0
	CriticalIssues       int           `json:"critical_issues"`
	TotalIssues          int           `json:"total_issues"`
	EstimatedTime        time.Duration `json:"estimated_time"`
	ParallelGroups       int           `json:"parallel_groups"`
	CategoriesWithIssues int           `json:"categories_with_issues"`
}

// WorkflowPlan contains the recommended remediation workflow
type WorkflowPlan struct {
	Phases         []WorkflowPhase `json:"phases"`
	ParallelGroups []ParallelGroup `json:"parallel_groups"`
	TotalTime      time.Duration   `json:"total_time"`
}
