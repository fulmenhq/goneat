package assess

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func sampleReport() *AssessmentReport {
	return &AssessmentReport{
		Metadata: ReportMetadata{
			GeneratedAt:   time.Now(),
			Tool:          "goneat",
			Version:       "test",
			Target:        ".",
			ExecutionTime: HumanReadableDuration(123 * time.Millisecond),
		},
		Summary: ReportSummary{
			OverallHealth:        0.82,
			CriticalIssues:       1,
			TotalIssues:          3,
			EstimatedTime:        HumanReadableDuration(5 * time.Minute),
			ParallelGroups:       1,
			CategoriesWithIssues: 2,
		},
		Categories: map[string]CategoryResult{
			string(CategorySecurity): {
				Category:       CategorySecurity,
				Priority:       2,
				Parallelizable: false,
				Status:         "success",
				IssueCount:     1,
				EstimatedTime:  HumanReadableDuration(2 * time.Minute),
				Issues:         []Issue{{File: "a.go", Line: 1, Severity: SeverityHigh, Message: "x", Category: CategorySecurity}},
				Metrics:        map[string]interface{}{"gosec_shards": 7, "gosec_pool_size": 3},
			},
			string(CategoryFormat): {
				Category:       CategoryFormat,
				Priority:       1,
				Parallelizable: true,
				Status:         "success",
				IssueCount:     2,
				EstimatedTime:  HumanReadableDuration(3 * time.Minute),
				Issues:         []Issue{{File: "b.go", Line: 2, Severity: SeverityLow, Message: "fmt", Category: CategoryFormat}, {File: "c.go", Line: 3, Severity: SeverityLow, Message: "fmt2", Category: CategoryFormat}},
			},
		},
		Workflow: WorkflowPlan{Phases: []WorkflowPhase{{Name: "p1", Description: "d", EstimatedTime: HumanReadableDuration(5 * time.Minute), Categories: []AssessmentCategory{CategoryFormat, CategorySecurity}, Priority: 1}}},
	}
}

func TestFormatter_JSON(t *testing.T) {
	f := NewFormatter(FormatJSON)
	out, err := f.FormatReport(sampleReport())
	if err != nil {
		t.Fatalf("json format error: %v", err)
	}
	var rpt AssessmentReport
	if err := json.Unmarshal([]byte(out), &rpt); err != nil {
		t.Fatalf("invalid json output: %v", err)
	}
}

func TestFormatter_Markdown_AndMaxIssuesCap(t *testing.T) {
	t.Setenv("GONEAT_MAX_ISSUES_DISPLAY", "1")
	f := NewFormatter(FormatMarkdown)
	out := f.formatMarkdown(sampleReport())
	if !strings.HasPrefix(out, "# Codebase Assessment Report") {
		t.Fatalf("missing markdown header, got: %q", out[:50])
	}
	if !strings.Contains(out, "Showing 1 of 2 issues") {
		t.Fatalf("expected cap notice in markdown output")
	}
}

func TestFormatter_Concise_FailOnAndShardMetrics(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv("GONEAT_SECURITY_FAIL_ON", "critical")
	f := NewFormatter(FormatConcise)
	out := f.formatConcise(sampleReport())
	if !strings.Contains(out, "Fail-on: critical") {
		t.Fatalf("expected fail-on header from env, got: %s", out)
	}
	if !strings.Contains(out, "shards:") {
		t.Fatalf("expected security shard metrics line, got: %s", out)
	}
	// no ANSI color when NO_COLOR set
	if strings.Contains(out, "\x1b[") {
		t.Fatalf("expected no ANSI color codes when NO_COLOR set")
	}
}
