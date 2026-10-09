package assess

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type securityFixtureTool struct {
	name         string
	available    bool
	applicable   bool
	issues       []Issue
	suppressions []Suppression
	err          error
}

func (f *securityFixtureTool) Name() string       { return f.name }
func (f *securityFixtureTool) IsAvailable() bool  { return f.available }
func (f *securityFixtureTool) IsApplicable() bool { return f.applicable }
func (f *securityFixtureTool) Run(context.Context) ([]Issue, error) {
	return f.issues, f.err
}
func (f *securityFixtureTool) RunWithSuppressions(context.Context) ([]Issue, []Suppression, error) {
	return f.issues, f.suppressions, f.err
}

func useSecurityFixtures(t *testing.T, fixtures ...*securityFixtureTool) {
	t.Helper()
	old := securityRegistry
	securityRegistry = &SecurityToolRegistry{}
	t.Cleanup(func() { securityRegistry = old })
	for _, fixture := range fixtures {
		tool := fixture
		RegisterSecurityTool(tool.name, "code", func(*SecurityAssessmentRunner, string, AssessmentConfig) SecurityTool { return tool })
	}
}

func TestSecurityAdmissionStates(t *testing.T) {
	for _, tt := range []struct {
		name       string
		explicit   bool
		applicable bool
		disabled   bool
		state      string
		failure    bool
	}{
		{"automatic unavailable", false, true, false, "unavailable", false},
		{"explicit unavailable", true, true, false, "unavailable", true},
		{"explicit inapplicable", true, false, false, "inapplicable", false},
		{"explicit disabled", true, true, true, "not_selected", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			useSecurityFixtures(t, &securityFixtureTool{name: "fixture", applicable: tt.applicable})
			cfg := DefaultAssessmentConfig()
			if tt.explicit {
				cfg.SecurityTools = []string{"fixture"}
			}
			if tt.disabled {
				cfg.EnableCode = false
				cfg.EnableVuln = true
			}
			result, err := NewSecurityAssessmentRunner().Assess(context.Background(), t.TempDir(), cfg)
			if err != nil || result == nil || result.Success == tt.failure {
				t.Fatalf("wrong assessment outcome: %+v transport=%v", result, err)
			}
			if tt.failure {
				if result.Error == "" || result.SkipReason != "" {
					t.Fatalf("explicit missing tool must error, not skip: %+v", result)
				}
			} else if result.Error != "" || result.SkipReason == "" {
				t.Fatalf("automatic absence/inapplicability must be a labeled skip: %+v", result)
			}
			admissions := result.Metrics["tool_admissions"].([]securityToolAdmission)
			if len(admissions) != 1 || admissions[0].State != tt.state || admissions[0].Reason == "" {
				t.Fatalf("admission reason lost: %+v", admissions)
			}
			if result.Metrics["tools_started"] != 0 || result.Metrics["tools_completed"] != 0 {
				t.Fatalf("an unstarted tool must not appear completed: %+v", result.Metrics)
			}
		})
	}
}

func TestSecurityFailureRetainsFindingsAndSuppressions(t *testing.T) {
	root := t.TempDir()
	issue := Issue{File: filepath.Join(root, "a.go"), Severity: SeverityLow, Message: "retained finding", Category: CategorySecurity}
	supp := Suppression{Tool: "z-tool", File: issue.File, RuleID: "G101", Reason: "fixture"}
	useSecurityFixtures(t,
		&securityFixtureTool{name: "z-tool", available: true, applicable: true, issues: []Issue{issue}, suppressions: []Suppression{supp}, err: errors.New("partial scan")},
		&securityFixtureTool{name: "a-tool", available: true, applicable: true, err: errors.New("offline")},
		&securityFixtureTool{name: "clean-tool", available: true, applicable: true},
	)
	cfg := DefaultAssessmentConfig()
	cfg.TrackSuppressions = true
	cfg.FailOnSeverity = IssueSeverity("none")
	result, err := NewSecurityAssessmentRunner().Assess(context.Background(), root, cfg)
	if err != nil || result.Success || result.SkipReason != "" {
		t.Fatalf("expected retained error result and nil transport: %+v err=%v", result, err)
	}
	if result.Error != "a-tool: offline\nz-tool: partial scan" || len(result.Issues) != 1 || !reflect.DeepEqual(result.Issues[0], issue) {
		t.Fatalf("sorted errors or parsed findings lost: %+v", result)
	}
	if got := result.Metrics["_suppressions"].([]Suppression); len(got) != 1 || got[0].RuleID != supp.RuleID {
		t.Fatalf("failed tool suppression lost: %+v", got)
	}
	if result.Metrics["tools_started"] != 3 || result.Metrics["tools_completed"] != 1 || result.Metrics["tools_failed"] != 2 {
		t.Fatalf("started/completed/failed conflated: %+v", result.Metrics)
	}
}

func TestSecurityPresentationSuppressionCannotHideExecutionFailure(t *testing.T) {
	root := t.TempDir()
	issue := Issue{File: filepath.Join(root, "cmd", "hooks.go"), Severity: SeverityHigh, Message: "gosec(G302): policy fixture", Category: CategorySecurity}
	useSecurityFixtures(t, &securityFixtureTool{name: "gosec", available: true, applicable: true, issues: []Issue{issue}, err: context.DeadlineExceeded})
	cfg := DefaultAssessmentConfig()
	cfg.TrackSuppressions = true
	result, err := NewSecurityAssessmentRunner().Assess(context.Background(), root, cfg)
	if err != nil || result.Success || result.Error == "" || result.SkipReason != "" || len(result.Issues) != 0 {
		t.Fatalf("suppression must not hide execution failure: %+v transport=%v", result, err)
	}
	if got := result.Metrics["_suppressions"].([]Suppression); len(got) != 1 || got[0].RuleID != "G302" {
		t.Fatalf("existing policy suppression changed: %+v", got)
	}
}

func TestSecurityFailureResultSurvivesEnginePaths(t *testing.T) {
	for _, workers := range []int{1, 3} {
		t.Run(fmt.Sprint(workers), func(t *testing.T) {
			issue := Issue{File: "a.go", Severity: SeverityLow, Message: "retained", Category: CategorySecurity}
			supp := Suppression{Tool: "fixture", File: "a.go", RuleID: "fixture", Reason: "fixture"}
			registry := NewAssessmentRunnerRegistry()
			registry.RegisterRunner(CategorySecurity, &statusRunner{category: CategorySecurity, result: &AssessmentResult{
				CommandName: "security", Success: false, Error: "fixture: scan interrupted", Issues: []Issue{issue}, Metrics: map[string]interface{}{"_suppressions": []Suppression{supp}},
			}})
			engine := &AssessmentEngine{runnerRegistry: registry, priorityManager: NewPriorityManager()}
			cfg := DefaultAssessmentConfig()
			cfg.SelectedCategories = []string{string(CategorySecurity)}
			cfg.Concurrency = workers
			cfg.TrackSuppressions = true
			cfg.FailOnSeverity = IssueSeverity("none")
			report, err := engine.RunAssessment(context.Background(), t.TempDir(), cfg)
			if err != nil {
				t.Fatal(err)
			}
			got := report.Categories[string(CategorySecurity)]
			if got.Status != "error" || !strings.Contains(got.Error, "scan interrupted") || len(got.Issues) != 1 || !reflect.DeepEqual(got.Issues[0], issue) || got.Reason != "" {
				t.Fatalf("engine lost error or findings at workers=%d: %+v", workers, got)
			}
			if got.SuppressionReport == nil || len(got.SuppressionReport.Suppressions) != 1 {
				t.Fatalf("engine lost failed-result suppressions: %+v", got)
			}
		})
	}
}
