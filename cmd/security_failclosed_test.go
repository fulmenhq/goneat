package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fulmenhq/goneat/internal/assess"
	"github.com/spf13/cobra"
)

// These command-boundary controls consume an adapter-shaped failure result.
// Native process/parser controls live in internal/assess; no live scanner proof
// is claimed from this deterministic runner.
type securityFailureRunner struct{ keepFinding bool }

func (r *securityFailureRunner) Assess(ctx context.Context, target string, cfg assess.AssessmentConfig) (*assess.AssessmentResult, error) {
	var issues []assess.Issue
	if r.keepFinding {
		issues = []assess.Issue{{File: "a.go", Severity: assess.SeverityLow, Message: "retained low finding", Category: assess.CategorySecurity}}
	}
	return &assess.AssessmentResult{
		CommandName: "security", Category: assess.CategorySecurity, Success: false, Error: "fixture: partial scan", Issues: issues,
		Metrics: map[string]interface{}{
			"tool_warnings": map[string]json.RawMessage{"fixture": json.RawMessage(`{"future":9007199254740993}`)},
			"_suppressions": []assess.Suppression{{Tool: "fixture", File: "a.go", RuleID: "fixture", Reason: "retained reason"}},
		},
	}, nil
}

type govulnInventoryGateRunner struct {
	severity assess.IssueSeverity
	failed   bool
}

func (r *govulnInventoryGateRunner) Assess(context.Context, string, assess.AssessmentConfig) (*assess.AssessmentResult, error) {
	result := &assess.AssessmentResult{CommandName: "security", Category: assess.CategorySecurity, Success: !r.failed,
		Issues:  []assess.Issue{{File: "go.mod", Severity: r.severity, Message: "govulncheck: GO-2026-5932 in golang.org/x/crypto", Category: assess.CategorySecurity}},
		Metrics: map[string]interface{}{"tool_reports": map[string]interface{}{"govulncheck": map[string]interface{}{"finding": json.RawMessage(`{"osv":"GO-2026-5932","future":9007199254740993}`)}}},
	}
	if r.failed {
		result.Error = "govulncheck inventory classification incomplete"
	}
	return result, nil
}
func (*govulnInventoryGateRunner) CanRunInParallel() bool { return true }
func (*govulnInventoryGateRunner) GetCategory() assess.AssessmentCategory {
	return assess.CategorySecurity
}
func (*govulnInventoryGateRunner) GetEstimatedTime(string) time.Duration { return time.Millisecond }
func (*govulnInventoryGateRunner) IsAvailable() bool                     { return true }

// Adapter/process tests establish the grade. This command-boundary control
// establishes that inventory is visible and cannot waive an execution error.
func TestGovulnHighGateRetainsInventory(t *testing.T) {
	for _, hook := range []bool{false, true} {
		for _, tt := range []struct {
			name                string
			severity            assess.IssueSeverity
			failed, wantFailure bool
		}{
			{"inventory", assess.SeverityInfo, false, false},
			{"import", assess.SeverityHigh, false, true},
			{"called_init", assess.SeverityHigh, false, true},
			{"inventory_error", assess.SeverityInfo, true, true},
		} {
			t.Run(fmt.Sprintf("%s-hook%t", tt.name, hook), func(t *testing.T) {
				useSecurityFailureRunner(t, false)
				registry := assess.ResetRegistryForTesting()
				registry.RegisterRunner(assess.CategorySecurity, &govulnInventoryGateRunner{severity: tt.severity, failed: tt.failed})
				root := t.TempDir()
				args := []string{"--categories", "security", "--format", "json", "--fail-on", "high", root}
				if hook {
					if err := os.Mkdir(filepath.Join(root, ".goneat"), 0700); err != nil {
						t.Fatal(err)
					}
					manifest := `version: "1.0.0"
hooks:
  pre-push:
    - command: assess
      args: ["--categories", "security", "--fail-on", "high"]
      priority: 10
      timeout: "60s"
`
					if err := os.WriteFile(filepath.Join(root, ".goneat/hooks.yaml"), []byte(manifest), 0600); err != nil {
						t.Fatal(err)
					}
					t.Chdir(root)
					args = []string{"--hook", "pre-push", "--hook-manifest", ".goneat/hooks.yaml", "--format", "json"}
				}
				var out, stderr bytes.Buffer
				command := &cobra.Command{Use: "assess", RunE: runAssess}
				setupAssessCommandFlags(command)
				command.SilenceErrors, command.SilenceUsage = true, true
				command.SetOut(&out)
				command.SetErr(&stderr)
				command.SetArgs(args)
				err := command.ExecuteContext(t.Context())
				if (err != nil) != tt.wantFailure {
					t.Fatalf("wrong high gate: %v %s", err, out.String())
				}
				var report assess.AssessmentReport
				if err := json.Unmarshal(out.Bytes(), &report); err != nil {
					t.Fatalf("report: %v %s", err, out.String())
				}
				category := report.Categories[string(assess.CategorySecurity)]
				if len(category.Issues) != 1 || category.Issues[0].Severity != tt.severity || (category.Status == "error") != tt.failed || !strings.Contains(out.String(), "9007199254740993") {
					t.Fatalf("inventory/error/provenance lost: %s", out.String())
				}
			})
		}
	}
}
func (*securityFailureRunner) CanRunInParallel() bool                 { return true }
func (*securityFailureRunner) GetCategory() assess.AssessmentCategory { return assess.CategorySecurity }
func (*securityFailureRunner) GetEstimatedTime(string) time.Duration  { return time.Millisecond }
func (*securityFailureRunner) IsAvailable() bool                      { return true }

func useSecurityFailureRunner(t *testing.T, keepFinding bool) {
	t.Helper()
	old := assess.GetAssessmentRunnerRegistry()
	registry := assess.ResetRegistryForTesting()
	registry.RegisterRunner(assess.CategorySecurity, &securityFailureRunner{keepFinding: keepFinding})
	t.Cleanup(func() {
		assess.RestoreRegistry(old)
		assessMode, assessNoOp, assessCheck, assessFix = "", false, false, false
		assessHook, assessHookManifest = "", ".goneat/hooks.yaml"
	})
	assessMode, assessNoOp, assessCheck, assessFix = "check", false, false, false
}

func TestSecurityFailureCLIAndHookJSON(t *testing.T) {
	for _, workers := range []int{1, 3} {
		for _, keepFinding := range []bool{false, true} {
			for _, hook := range []bool{false, true} {
				t.Run(fmt.Sprintf("workers%d-findings%t-hook%t", workers, keepFinding, hook), func(t *testing.T) {
					useSecurityFailureRunner(t, keepFinding)
					root := t.TempDir()
					args := []string{"--categories", "security", "--format", "json", "--fail-on", "critical", "--track-suppressions", "--concurrency", fmt.Sprint(workers), root}
					if hook {
						if err := os.Mkdir(filepath.Join(root, ".goneat"), 0700); err != nil {
							t.Fatal(err)
						}
						manifest := `version: "1.0.0"
hooks:
  pre-push:
    - command: assess
      args: ["--categories", "security", "--fail-on", "critical"]
      priority: 10
      timeout: "60s"
`
						if err := os.WriteFile(filepath.Join(root, ".goneat", "hooks.yaml"), []byte(manifest), 0600); err != nil {
							t.Fatal(err)
						}
						t.Chdir(root)
						args = []string{"--hook", "pre-push", "--hook-manifest", ".goneat/hooks.yaml", "--format", "json", "--track-suppressions", "--concurrency", fmt.Sprint(workers)}
					}
					var stdout, stderr bytes.Buffer
					command := &cobra.Command{Use: "assess", RunE: runAssess}
					setupAssessCommandFlags(command)
					command.SilenceErrors, command.SilenceUsage = true, true
					command.SetOut(&stdout)
					command.SetErr(&stderr)
					command.SetArgs(args)
					if err := command.ExecuteContext(t.Context()); err == nil {
						t.Fatal("security execution failure must fail direct and hook commands")
					}
					var report assess.AssessmentReport
					if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
						t.Fatalf("stdout must contain only complete JSON: %v\n%s", err, stdout.String())
					}
					category := report.Categories[string(assess.CategorySecurity)]
					wantFindings := 0
					if keepFinding {
						wantFindings = 1
					}
					if category.Status != "error" || !strings.Contains(category.Error, "partial scan") || category.Reason != "" || len(category.Issues) != wantFindings {
						t.Fatalf("command lost retained error/finding result: %+v", category)
					}
					if category.SuppressionReport == nil || len(category.SuppressionReport.Suppressions) != 1 || !strings.Contains(stdout.String(), "9007199254740993") {
						t.Fatalf("suppression/warning metadata or precision lost: %s", stdout.String())
					}
				})
			}
		}
	}
}
