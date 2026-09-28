package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/fulmenhq/goneat/internal/assess"
)

type unavailableCLIRunner struct{ category assess.AssessmentCategory }

func (u *unavailableCLIRunner) Assess(context.Context, string, assess.AssessmentConfig) (*assess.AssessmentResult, error) {
	panic("an unavailable runner must not be run")
}
func (u *unavailableCLIRunner) CanRunInParallel() bool                 { return true }
func (u *unavailableCLIRunner) GetCategory() assess.AssessmentCategory { return u.category }
func (u *unavailableCLIRunner) GetEstimatedTime(string) time.Duration  { return time.Millisecond }
func (u *unavailableCLIRunner) IsAvailable() bool                      { return false }
func (u *unavailableCLIRunner) UnavailableReason() string {
	return "neither gosec nor govulncheck found in PATH"
}

func useUnavailableSecurity(t *testing.T) {
	t.Helper()
	original := assess.GetAssessmentRunnerRegistry()
	registry := assess.ResetRegistryForTesting()
	registry.RegisterRunner(assess.CategorySecurity, &unavailableCLIRunner{category: assess.CategorySecurity})
	registry.RegisterRunner(assess.CategoryFormat, &cliFakeRunner{})
	t.Cleanup(func() {
		assess.RestoreRegistry(original)
		assessMode, assessNoOp, assessCheck, assessFix = "", false, false, false
		assessHook, assessHookManifest = "", ".goneat/hooks.yaml"
	})
	assessMode, assessNoOp, assessCheck, assessFix = "check", false, false, false
}

func runAssessCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	cmd := &cobra.Command{Use: "assess", RunE: runAssess}
	setupAssessCommandFlags(cmd)
	cmd.SilenceUsage, cmd.SilenceErrors = true, true
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	err := cmd.ExecuteContext(context.Background())
	return stdout.String(), err
}

// A requested category that cannot run appears as skipped with its reason in
// JSON and human output, directly and through a hook. It is not an error.
func TestAssessCLI_RequestedUnavailableCategoryShowsSkipped(t *testing.T) {
	useUnavailableSecurity(t)
	repo := t.TempDir()

	out, err := runAssessCLI(t, "--categories", "security,format", "--format", "json", "--fail-on", "critical", "--concurrency", "1", repo)
	if err != nil {
		t.Fatalf("a skipped category must not fail the run: %v", err)
	}
	var report assess.AssessmentReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("stdout must be JSON: %v\n%s", err, out)
	}
	sec, ok := report.Categories[string(assess.CategorySecurity)]
	if !ok || sec.Status != "skipped" || !strings.Contains(sec.Reason, "gosec") {
		t.Fatalf("security must be skipped with a reason, got %+v", report.Categories)
	}

	out, err = runAssessCLI(t, "--categories", "security,format", "--format", "concise", "--fail-on", "critical", "--concurrency", "1", repo)
	if err != nil {
		t.Fatalf("concise run: %v", err)
	}
	if !strings.Contains(out, "skipped") || !strings.Contains(out, "not run: neither gosec nor govulncheck") {
		t.Fatalf("human output must show the skip and its reason, got:\n%s", out)
	}

	if err := os.MkdirAll(filepath.Join(repo, ".goneat"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := "version: \"1.0.0\"\nhooks:\n  pre-push:\n    - command: \"assess\"\n      args: [\"--categories\", \"security,format\", \"--fail-on\", \"high\"]\n      priority: 10\n      timeout: \"60s\"\n"
	if err := os.WriteFile(filepath.Join(repo, ".goneat", "hooks.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo)
	out, err = runAssessCLI(t, "--hook", "pre-push", "--hook-manifest", ".goneat/hooks.yaml", "--format", "json")
	if err != nil {
		t.Fatalf("hook with a skipped category must pass: %v", err)
	}
	if !strings.Contains(out, `"status": "skipped"`) || !strings.Contains(out, `"reason": "neither gosec nor govulncheck found in PATH"`) {
		t.Fatalf("hook JSON must carry the skip and its reason, got:\n%s", out)
	}
}

// An unknown category name fails direct and hook runs, in JSON and human
// output, and the known sibling in the same list does not run.
func TestAssessCLI_UnknownCategoryFails(t *testing.T) {
	useUnavailableSecurity(t)
	repo := t.TempDir()

	for _, format := range []string{"json", "concise"} {
		out, err := runAssessCLI(t, "--categories", "format,lnt", "--format", format, "--fail-on", "critical", "--concurrency", "1", repo)
		if err == nil {
			t.Fatalf("%s: an unknown category must fail the run, got:\n%s", format, out)
		}
		if !strings.Contains(out, `unknown assessment category \"lnt\"`) && !strings.Contains(out, `unknown assessment category "lnt"`) {
			t.Fatalf("%s: output must name the unknown category, got:\n%s", format, out)
		}
		if format == "json" {
			var report assess.AssessmentReport
			if err := json.Unmarshal([]byte(out), &report); err != nil {
				t.Fatalf("stdout must be JSON: %v\n%s", err, out)
			}
			if _, ran := report.Categories[string(assess.CategoryFormat)]; ran {
				t.Fatalf("the known sibling must not run, got %+v", report.Categories)
			}
		}
	}

	if err := os.MkdirAll(filepath.Join(repo, ".goneat"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := "version: \"1.0.0\"\nhooks:\n  pre-commit:\n    - command: \"assess\"\n      args: [\"--categories\", \"format,lnt\", \"--fail-on\", \"critical\"]\n      priority: 10\n      timeout: \"60s\"\n"
	if err := os.WriteFile(filepath.Join(repo, ".goneat", "hooks.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo)
	for _, format := range []string{"json", "concise"} {
		out, err := runAssessCLI(t, "--hook", "pre-commit", "--hook-manifest", ".goneat/hooks.yaml", "--format", format)
		if err == nil {
			t.Fatalf("%s: a hook naming an unknown category must fail, got:\n%s", format, out)
		}
		if !strings.Contains(out, "lnt") {
			t.Fatalf("%s: hook output must name the unknown category, got:\n%s", format, out)
		}
	}
}
