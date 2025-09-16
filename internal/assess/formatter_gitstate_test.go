package assess

import (
	"os"
	"strings"
	"testing"
	"time"
)

func gitStateReport(sampleFiles []string) *AssessmentReport {
	cr := CategoryResult{
		Category:       CategoryRepoStatus,
		Priority:       1,
		Parallelizable: true,
		Status:         "issues",
		IssueCount:     1,
		EstimatedTime:  HumanReadableDuration(15 * time.Minute),
		Issues: []Issue{{
			File:        "repository",
			Severity:    SeverityHigh,
			Category:    CategoryRepoStatus,
			SubCategory: "git-state",
			Message:     "Dirty git state",
		}},
		Metrics: map[string]interface{}{
			"uncommitted_files": sampleFiles,
		},
	}

	return &AssessmentReport{
		Metadata: ReportMetadata{GeneratedAt: time.Now(), Tool: "goneat", Version: "test", Target: ".", ExecutionTime: 0},
		Summary:  ReportSummary{OverallHealth: 0.8, TotalIssues: 1},
		Categories: map[string]CategoryResult{
			string(CategoryRepoStatus): cr,
		},
	}
}

func TestFormatter_Concise_UsesMetricsFilesForGitState(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	rpt := gitStateReport([]string{"README.md", "CHANGELOG.md"})
	out, err := NewFormatter(FormatConcise).FormatReport(rpt)
	if err != nil {
		t.Fatalf("concise: %v", err)
	}
	if !strings.Contains(out, "README.md") || !strings.Contains(out, "CHANGELOG.md") {
		t.Fatalf("expected metrics-driven filenames in concise output, got: %s", out)
	}
}

func TestFormatter_Markdown_UsesMetricsFilesForGitState(t *testing.T) {
	rpt := gitStateReport([]string{"README.md", "CHANGELOG.md"})
	// Add a parallel group with sentinel to exercise expansion in markdown
	rpt.Workflow = WorkflowPlan{ParallelGroups: []ParallelGroup{{
		Name: "group_gitstate", Description: "Issues in repository", Files: []string{"repository"},
		Categories: []AssessmentCategory{CategoryRepoStatus}, IssueCount: 1, EstimatedTime: HumanReadableDuration(15 * time.Minute),
	}}}
	out := NewFormatter(FormatMarkdown).formatMarkdown(rpt)
	if !strings.Contains(out, "Affected files:") || !strings.Contains(out, "README.md") {
		t.Fatalf("expected affected files list in markdown output, got: %s", out)
	}
	if !strings.Contains(out, "Parallelization Opportunities") || !strings.Contains(out, "README.md") {
		t.Fatalf("expected parallelization section to list metrics-driven files, got: %s", out)
	}
}

func TestFormatter_HTML_UsesMetricsFilesForGitState(t *testing.T) {
	// Point template path to embedded test template to avoid file lookup variance
	t.Setenv("GONEAT_TEMPLATE_PATH", "../assets/embedded_templates/report.html")
	rpt := gitStateReport([]string{"README.md", "CHANGELOG.md"})
	out := NewFormatter(FormatHTML).formatHTML(rpt)
	if !strings.Contains(out, "README.md") {
		// include snippet for debugging
		_ = os.WriteFile("/tmp/html_out.html", []byte(out), 0600)
		t.Fatalf("expected html to include sample file name, got length=%d", len(out))
	}
}
