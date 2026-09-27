package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/fulmenhq/goneat/internal/assess"
)

// invalidAssessRepo returns a directory whose .goneat/assess.yaml has an
// invalid known value (typecheck.enabled) beside a valid sibling.
func invalidAssessRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".goneat"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := "version: 1\nlint:\n  yamllint:\n    enabled: false\ntypecheck:\n  enabled: \"yes please\"\n"
	if err := os.WriteFile(filepath.Join(repo, ".goneat", "assess.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	return repo
}

// useTypecheckAndFakeFormat registers the real typecheck runner (which loads
// assess.yaml) and a fake format runner (which does not).
func useTypecheckAndFakeFormat(t *testing.T) {
	t.Helper()
	original := assess.GetAssessmentRunnerRegistry()
	registry := assess.ResetRegistryForTesting()
	registry.RegisterRunner(assess.CategoryTypecheck, assess.NewTypecheckAssessmentRunner())
	registry.RegisterRunner(assess.CategoryFormat, &cliFakeRunner{})
	t.Cleanup(func() {
		assess.RestoreRegistry(original)
		assessMode, assessNoOp, assessCheck, assessFix = "", false, false, false
		assessHook, assessHookManifest = "", ".goneat/hooks.yaml"
	})
	assessMode, assessNoOp, assessCheck, assessFix = "check", false, false, false
}

func TestAssessCLI_InvalidAssessConfigFailsWithJSONDiagnostic(t *testing.T) {
	useTypecheckAndFakeFormat(t)
	repo := invalidAssessRepo(t)

	var stdout, stderr bytes.Buffer
	cmd := &cobra.Command{Use: "assess", RunE: runAssess}
	setupAssessCommandFlags(cmd)
	cmd.SilenceUsage, cmd.SilenceErrors = true, true
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--categories", "typecheck,format", "--format", "json", "--fail-on", "critical", "--concurrency", "1", repo})
	if err := cmd.ExecuteContext(context.Background()); err == nil {
		t.Fatalf("invalid assess.yaml must fail the command")
	}

	var report assess.AssessmentReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout must be JSON: %v\n%s", err, stdout.String())
	}
	tc := report.Categories[string(assess.CategoryTypecheck)]
	if tc.Status != "error" || !strings.Contains(tc.Error, filepath.Join(".goneat", "assess.yaml")) || !strings.Contains(tc.Error, "typecheck") {
		t.Fatalf("typecheck must report the config path and key, got status=%q error=%q", tc.Status, tc.Error)
	}
	if f := report.Categories[string(assess.CategoryFormat)]; f.Status == "error" {
		t.Fatalf("a category that does not read assess.yaml must be unaffected, got %+v", f)
	}
}

func TestAssessHook_InvalidAssessConfigFailsHook(t *testing.T) {
	useTypecheckAndFakeFormat(t)
	repo := invalidAssessRepo(t)
	manifest := "version: \"1.0.0\"\nhooks:\n  pre-commit:\n    - command: \"assess\"\n      args: [\"--categories\", \"typecheck,format\", \"--fail-on\", \"critical\"]\n      priority: 10\n      timeout: \"60s\"\n"
	if err := os.WriteFile(filepath.Join(repo, ".goneat", "hooks.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo)

	var out bytes.Buffer
	cmd := &cobra.Command{Use: "assess", RunE: runAssess}
	setupAssessCommandFlags(cmd)
	cmd.SilenceUsage, cmd.SilenceErrors = true, true
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--hook", "pre-commit", "--hook-manifest", ".goneat/hooks.yaml", "--format", "concise"})
	if err := cmd.ExecuteContext(context.Background()); err == nil {
		t.Fatalf("invalid assess.yaml must fail the hook, output:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "invalid assess configuration") {
		t.Fatalf("hook output must carry the config diagnostic, got:\n%s", out.String())
	}

	// JSON hook output; the cached config error keeps failing on repeat.
	for i := 0; i < 2; i++ {
		var stdout, stderr bytes.Buffer
		cmd = &cobra.Command{Use: "assess", RunE: runAssess}
		setupAssessCommandFlags(cmd)
		cmd.SilenceUsage, cmd.SilenceErrors = true, true
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs([]string{"--hook", "pre-commit", "--hook-manifest", ".goneat/hooks.yaml", "--format", "json"})
		if err := cmd.ExecuteContext(context.Background()); err == nil {
			t.Fatalf("json run %d: invalid assess.yaml must fail the hook", i)
		}
		if !strings.Contains(stdout.String(), `"status": "error"`) || !strings.Contains(stdout.String(), "invalid assess configuration") {
			t.Fatalf("json run %d: hook JSON must carry the error status and diagnostic, got:\n%s", i, stdout.String())
		}
	}
}

func TestShouldFailHook_CategoryError(t *testing.T) {
	report := &assess.AssessmentReport{Categories: map[string]assess.CategoryResult{
		"lint": {Status: "error", Error: "invalid assess configuration"},
	}}
	if !shouldFailHook(report, &HookConfig{FailOn: "critical"}) {
		t.Fatalf("a category error must fail the hook at any threshold")
	}
}

// erroringRunner stands in for a tool that could not complete (for example a
// Cargo/clippy execution failure), unrelated to assess.yaml.
type erroringRunner struct{ category assess.AssessmentCategory }

func (e *erroringRunner) Assess(ctx context.Context, target string, cfg assess.AssessmentConfig) (*assess.AssessmentResult, error) {
	return &assess.AssessmentResult{CommandName: string(e.category), Category: e.category, Success: false, Error: "cargo-clippy failed: cargo exited 101"}, nil
}
func (e *erroringRunner) CanRunInParallel() bool                 { return true }
func (e *erroringRunner) GetCategory() assess.AssessmentCategory { return e.category }
func (e *erroringRunner) GetEstimatedTime(string) time.Duration  { return time.Millisecond }
func (e *erroringRunner) IsAvailable() bool                      { return true }

func TestAssessHook_ToolExecutionErrorFailsHook(t *testing.T) {
	original := assess.GetAssessmentRunnerRegistry()
	registry := assess.ResetRegistryForTesting()
	registry.RegisterRunner(assess.CategoryLint, &erroringRunner{category: assess.CategoryLint})
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
	manifest := "version: \"1.0.0\"\nhooks:\n  pre-push:\n    - command: \"assess\"\n      args: [\"--categories\", \"lint\", \"--fail-on\", \"critical\"]\n      priority: 10\n      timeout: \"60s\"\n"
	if err := os.WriteFile(filepath.Join(repo, ".goneat", "hooks.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo)

	for _, format := range []string{"concise", "json"} {
		var out bytes.Buffer
		cmd := &cobra.Command{Use: "assess", RunE: runAssess}
		setupAssessCommandFlags(cmd)
		cmd.SilenceUsage, cmd.SilenceErrors = true, true
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"--hook", "pre-push", "--hook-manifest", ".goneat/hooks.yaml", "--format", format})
		if err := cmd.ExecuteContext(context.Background()); err == nil {
			t.Fatalf("%s: a category that could not run must fail the hook, output:\n%s", format, out.String())
		}
	}
}

func TestAssessCLI_InvalidAssessConfigFailsHumanOutput(t *testing.T) {
	useTypecheckAndFakeFormat(t)
	repo := invalidAssessRepo(t)

	var out bytes.Buffer
	cmd := &cobra.Command{Use: "assess", RunE: runAssess}
	setupAssessCommandFlags(cmd)
	cmd.SilenceUsage, cmd.SilenceErrors = true, true
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--categories", "typecheck,format", "--format", "concise", "--fail-on", "critical", "--concurrency", "1", repo})
	if err := cmd.ExecuteContext(context.Background()); err == nil {
		t.Fatalf("invalid assess.yaml must fail the command")
	}
	if !strings.Contains(out.String(), "invalid assess configuration") || !strings.Contains(out.String(), "typecheck") {
		t.Fatalf("human output must name the config problem, got:\n%s", out.String())
	}
}

// A dangling assess.yaml symlink fails direct and hook runs with the path.
func TestAssessCLI_DanglingAssessConfigSymlinkFails(t *testing.T) {
	useTypecheckAndFakeFormat(t)
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".goneat"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("nonexistent.yaml", filepath.Join(repo, ".goneat", "assess.yaml")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	manifest := "version: \"1.0.0\"\nhooks:\n  pre-commit:\n    - command: \"assess\"\n      args: [\"--categories\", \"typecheck,format\", \"--fail-on\", \"critical\"]\n      priority: 10\n      timeout: \"60s\"\n"
	if err := os.WriteFile(filepath.Join(repo, ".goneat", "hooks.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo)

	for _, args := range [][]string{
		{"--categories", "typecheck,format", "--format", "json", "--fail-on", "critical", "--concurrency", "1", "."},
		{"--hook", "pre-commit", "--hook-manifest", ".goneat/hooks.yaml", "--format", "json"},
	} {
		var stdout, stderr bytes.Buffer
		cmd := &cobra.Command{Use: "assess", RunE: runAssess}
		setupAssessCommandFlags(cmd)
		cmd.SilenceUsage, cmd.SilenceErrors = true, true
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs(args)
		if err := cmd.ExecuteContext(context.Background()); err == nil {
			t.Fatalf("%v: dangling assess.yaml must fail, output:\n%s", args, stdout.String())
		}
		if !strings.Contains(stdout.String(), filepath.Join(".goneat", "assess.yaml")) {
			t.Fatalf("%v: output must name the config path, got:\n%s", args, stdout.String())
		}
	}
}

// A yamllint run that reports a valid finding but also fails on stderr must
// fail the lint category, directly and through a hook.
func TestAssessCLI_YamllintMixedOutputFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a shell-script stand-in for yamllint")
	}
	original := assess.GetAssessmentRunnerRegistry()
	registry := assess.ResetRegistryForTesting()
	registry.RegisterRunner(assess.CategoryLint, assess.NewLintAssessmentRunner())
	t.Cleanup(func() {
		assess.RestoreRegistry(original)
		assessMode, assessNoOp, assessCheck, assessFix = "", false, false, false
		assessHook, assessHookManifest = "", ".goneat/hooks.yaml"
	})
	assessMode, assessNoOp, assessCheck, assessFix = "check", false, false, false

	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".github", "workflows"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, ".goneat"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".github", "workflows", "ci.yml"), []byte("on: push\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := "version: \"1.0.0\"\nhooks:\n  pre-commit:\n    - command: \"assess\"\n      args: [\"--categories\", \"lint\", \"--fail-on\", \"critical\"]\n      priority: 10\n      timeout: \"60s\"\n"
	if err := os.WriteFile(filepath.Join(repo, ".goneat", "hooks.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	fake := filepath.Join(t.TempDir(), "yamllint")
	script := "#!/bin/sh\necho '.github/workflows/ci.yml:1:1: [warning] missing document start \"---\" (document-start)'\necho 'yamllint: failed to load configuration: unknown rule' >&2\nexit 2\n"
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GONEAT_YAMLLINT_BIN", fake)
	t.Chdir(repo)

	for _, args := range [][]string{
		{"--categories", "lint", "--format", "json", "--fail-on", "critical", "--concurrency", "1", "."},
		{"--hook", "pre-commit", "--hook-manifest", ".goneat/hooks.yaml", "--format", "json"},
	} {
		var stdout, stderr bytes.Buffer
		cmd := &cobra.Command{Use: "assess", RunE: runAssess}
		setupAssessCommandFlags(cmd)
		cmd.SilenceUsage, cmd.SilenceErrors = true, true
		cmd.SetOut(&stdout)
		cmd.SetErr(&stderr)
		cmd.SetArgs(args)
		if err := cmd.ExecuteContext(context.Background()); err == nil {
			t.Fatalf("%v: mixed yamllint output must fail, got:\n%s", args, stdout.String())
		}
		if !strings.Contains(stdout.String(), "failed to load configuration") {
			t.Fatalf("%v: output must carry the yamllint failure, got:\n%s", args, stdout.String())
		}
	}
}
