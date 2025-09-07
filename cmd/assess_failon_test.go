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
