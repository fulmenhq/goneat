package assess

import (
	"strings"
	"testing"
	"time"
)

func minimalReport() *AssessmentReport {
	return &AssessmentReport{
		Metadata: ReportMetadata{
			GeneratedAt:   time.Now(),
			Tool:          "goneat",
			Version:       "test",
			Target:        ".",
			ExecutionTime: 0,
			CommandsRun:   []string{"schema"},
		},
		Summary: ReportSummary{
			OverallHealth: 1,
			TotalIssues:   0,
		},
		Categories: map[string]CategoryResult{},
	}
}

func TestFormatter_Concise_Minimal(t *testing.T) {
	f := NewFormatter(FormatConcise)
	out, err := f.FormatReport(minimalReport())
	if err != nil {
		t.Fatalf("format concise failed: %v", err)
	}
	if out == "" || !containsAll(out, []string{"Assessment", "total issues"}) {
		t.Errorf("unexpected concise output: %s", out)
	}
}

func TestFormatter_Concise_ExecutionFooter(t *testing.T) {
	const passed = "✅ Hook validation passed"
	const warning = "⚠️ Issues detected - see details above or run with --verbose"
	const failed = "❌ Assessment execution failed"
	for _, tt := range []struct {
		name       string
		categories map[string]CategoryResult
		issues     int
		footer     string
	}{
		{"no categories", nil, 0, passed},
		{"success", map[string]CategoryResult{"security": {Status: "success"}}, 0, passed},
		{"success with findings", map[string]CategoryResult{"security": {Status: "success", IssueCount: 1}}, 1, warning},
		{"skipped", map[string]CategoryResult{"security": {Status: "skipped", Reason: "fixture skip"}}, 0, passed},
		{"non-error status with error text", map[string]CategoryResult{"security": {Status: "success", Error: "fixture note"}}, 0, passed},
		{"error without findings", map[string]CategoryResult{"security": {Status: "error", Error: "fixture failure"}}, 0, failed},
		{"error without text", map[string]CategoryResult{"security": {Status: "error"}}, 0, failed},
		{"error with findings", map[string]CategoryResult{"security": {Status: "error", Error: "fixture failure", IssueCount: 1}}, 1, failed},
		{"error with findings without text", map[string]CategoryResult{"security": {Status: "error", IssueCount: 1}}, 1, failed},
		{"mixed categories", map[string]CategoryResult{"security": {Status: "error", Priority: 2}, "format": {Status: "success", Priority: 1}}, 0, failed},
		{"mixed findings and error", map[string]CategoryResult{"security": {Status: "error", Priority: 1}, "format": {Status: "success", IssueCount: 1, Priority: 2}}, 1, failed},
	} {
		t.Run(tt.name, func(t *testing.T) {
			for _, noColor := range []string{"1", ""} {
				t.Run("NO_COLOR="+noColor, func(t *testing.T) {
					t.Setenv("NO_COLOR", noColor)
					report := minimalReport()
					report.Categories = make(map[string]CategoryResult, len(tt.categories))
					for name, category := range tt.categories {
						if category.IssueCount > 0 {
							category.Issues = []Issue{{File: "fixture.go", Severity: SeverityLow, Message: "fixture finding"}}
						}
						report.Categories[name] = category
					}
					report.Summary.TotalIssues = tt.issues
					out, err := NewFormatter(FormatConcise).FormatReport(report)
					if err != nil {
						t.Fatal(err)
					}
					footer := tt.footer
					if noColor == "" {
						code := "32"
						switch footer {
						case failed:
							code = "31"
						case warning:
							code = "33"
						}
						footer = "\x1b[" + code + "m" + footer + "\x1b[0m"
					}
					if !strings.HasSuffix(out, footer) {
						t.Fatalf("wrong footer: expected %q, got %q", footer, out)
					}
					for _, other := range []string{passed, warning, failed} {
						if other != tt.footer && strings.Contains(out, other) {
							t.Fatalf("conflicting footer %q: %q", other, out)
						}
					}
					if report.Summary.OverallHealth != 1 {
						t.Fatal("formatter changed issue-only health")
					}
					if tt.issues > 0 && !strings.Contains(out, "fixture.go") {
						t.Fatalf("finding file lost: %q", out)
					}
					for _, category := range tt.categories {
						if category.Status == "error" && category.Error != "" && !strings.Contains(out, category.Error) {
							t.Fatalf("category error detail lost: %q", out)
						}
					}
				})
			}
		})
	}
}

func containsAll(s string, parts []string) bool {
	for _, p := range parts {
		if !contains(s, p) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (len(sub) == 0 || (len(s) > 0 && (indexOf(s, sub) >= 0)))
}

func indexOf(s, sub string) int {
	// simple substring search
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
