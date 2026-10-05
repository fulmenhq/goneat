package cmd

import (
	"testing"

	"github.com/fulmenhq/goneat/internal/assess"
)

func makeReportWithSeverities(sevs []assess.IssueSeverity) *assess.AssessmentReport {
	issues := make([]assess.Issue, 0, len(sevs))
	for _, s := range sevs {
		issues = append(issues, assess.Issue{File: "x", Severity: s, Category: assess.CategorySecurity})
	}
	cat := assess.CategoryResult{Category: assess.CategorySecurity, Issues: issues, IssueCount: len(issues)}
	return &assess.AssessmentReport{Categories: map[string]assess.CategoryResult{string(assess.CategorySecurity): cat}}
}

func TestShouldFailDependencyGate(t *testing.T) {
	for _, tc := range []struct {
		name     string
		category assess.AssessmentCategory
		metric   interface{}
		wantFail bool
	}{
		{"failed_gate", assess.CategoryDependencies, false, true},
		{"passing_gate", assess.CategoryDependencies, true, false},
		{"missing_gate", assess.CategoryDependencies, nil, false},
		{"other_category", assess.CategoryLint, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			report := &assess.AssessmentReport{Categories: map[string]assess.CategoryResult{
				string(tc.category): {Category: tc.category, Status: "issues", Metrics: map[string]interface{}{"analysis_passed": tc.metric}, Issues: []assess.Issue{{Severity: assess.SeverityMedium}}},
			}}
			if got := shouldFail(report, assess.SeverityCritical); got != tc.wantFail {
				t.Fatalf("display threshold cleared dependency gate: got %t want %t", got, tc.wantFail)
			}
			if got := shouldFailHook(report, &HookConfig{FailOn: "critical"}); got != tc.wantFail {
				t.Fatalf("hook threshold cleared dependency gate: got %t want %t", got, tc.wantFail)
			}
		})
	}
}

func TestShouldFailThresholds(t *testing.T) {
	// With a HIGH issue present
	r := makeReportWithSeverities([]assess.IssueSeverity{assess.SeverityLow, assess.SeverityHigh})
	if !shouldFail(r, assess.SeverityMedium) {
		t.Fatalf("expected fail when fail-on=medium and high issue exists")
	}
	if !shouldFail(r, assess.SeverityHigh) {
		t.Fatalf("expected fail when fail-on=high and high issue exists")
	}
	if shouldFail(r, assess.SeverityCritical) {
		t.Fatalf("did not expect fail when fail-on=critical and only high/low issues exist")
	}

	// Only LOW issues
	r2 := makeReportWithSeverities([]assess.IssueSeverity{assess.SeverityLow, assess.SeverityLow})
	if shouldFail(r2, assess.SeverityMedium) {
		t.Fatalf("did not expect fail when fail-on=medium and only low issues exist")
	}
	if !shouldFail(r2, assess.SeverityLow) {
		t.Fatalf("expected fail when fail-on=low and low issues exist")
	}
}
