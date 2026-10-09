package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fulmenhq/goneat/internal/assess"
	"github.com/spf13/cobra"
)

type hookDeadlineFailureRunner struct{}

func (*hookDeadlineFailureRunner) Assess(ctx context.Context, _ string, _ assess.AssessmentConfig) (*assess.AssessmentResult, error) {
	if ctx.Err() != context.DeadlineExceeded {
		return nil, errors.New("fixture did not receive an expired context")
	}
	return &assess.AssessmentResult{
		CommandName: "fixture-lint", Category: assess.CategoryLint,
		Success: false, Error: "fixture collector failed to finish its report",
	}, nil
}
func (*hookDeadlineFailureRunner) CanRunInParallel() bool                 { return false }
func (*hookDeadlineFailureRunner) GetCategory() assess.AssessmentCategory { return assess.CategoryLint }
func (*hookDeadlineFailureRunner) GetEstimatedTime(string) time.Duration  { return time.Millisecond }
func (*hookDeadlineFailureRunner) IsAvailable() bool                      { return true }

func TestAssessHook_DeadlineRetainsErrorAndReport(t *testing.T) {
	original := assess.GetAssessmentRunnerRegistry()
	registry := assess.ResetRegistryForTesting()
	registry.RegisterRunner(assess.CategoryLint, &hookDeadlineFailureRunner{})
	t.Cleanup(func() {
		assess.RestoreRegistry(original)
		assessMode, assessNoOp, assessCheck, assessFix = "", false, false, false
		assessHook, assessHookManifest = "", ".goneat/hooks.yaml"
	})
	assessMode, assessNoOp, assessCheck, assessFix = "check", false, false, false
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".goneat"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := "version: \"1.0.0\"\nhooks:\n  pre-push:\n    - command: \"assess\"\n      args: [\"--categories\", \"lint\", \"--fail-on\", \"critical\"]\n      priority: 10\n      timeout: \"45s\"\n"
	if err := os.WriteFile(filepath.Join(repo, ".goneat", "hooks.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo)
	for _, format := range []string{"concise", "json"} {
		t.Run(format, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			cmd := &cobra.Command{Use: "assess", RunE: runAssess}
			setupAssessCommandFlags(cmd)
			cmd.SilenceUsage, cmd.SilenceErrors = true, true
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			cmd.SetArgs([]string{"--hook", "pre-push", "--hook-manifest", ".goneat/hooks.yaml", "--format", format, "--concurrency", "1"})
			ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
			defer cancel()
			err := cmd.ExecuteContext(ctx)
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("hook must retain deadline error identity: %v", err)
			}
			for _, message := range []string{"command timed out after 45s", "assessment found issues requiring attention"} {
				if !strings.Contains(err.Error(), message) {
					t.Errorf("returned error lost %q: %v", message, err)
				}
			}
			if !strings.Contains(stdout.String(), "fixture collector failed to finish its report") {
				t.Fatalf("original report diagnostic lost: %s", stdout.String())
			}
			if format == "json" {
				var report assess.AssessmentReport
				if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
					t.Fatalf("stdout is not one valid JSON report: %v\n%s", err, stdout.String())
				}
				if report.Categories["lint"].Status != "error" {
					t.Fatalf("category failure status lost: %+v", report.Categories["lint"])
				}
				if strings.Contains(stdout.String(), "command timed out after") {
					t.Fatal("hook timeout diagnostic contaminated report JSON")
				}
			}
		})
	}
}
