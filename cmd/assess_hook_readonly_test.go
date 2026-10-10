package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fulmenhq/goneat/internal/assess"
	"github.com/spf13/cobra"
)

type hookModeProbe struct {
	mode     assess.AssessmentMode
	readOnly bool
	shellFix bool
}

func (p *hookModeProbe) Assess(_ context.Context, _ string, cfg assess.AssessmentConfig) (*assess.AssessmentResult, error) {
	p.mode = cfg.Mode
	p.readOnly = cfg.HookReadOnly
	p.shellFix = cfg.LintShellFix
	return &assess.AssessmentResult{CommandName: "probe", Category: assess.CategoryLint, Success: true}, nil
}
func (*hookModeProbe) CanRunInParallel() bool                 { return false }
func (*hookModeProbe) GetCategory() assess.AssessmentCategory { return assess.CategoryLint }
func (*hookModeProbe) GetEstimatedTime(string) time.Duration  { return time.Millisecond }
func (*hookModeProbe) IsAvailable() bool                      { return true }

func TestHookCheckOnlyArgs(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want string
	}{
		{name: "empty", want: "--check"},
		{name: "keeps other flags", in: []string{"--fail-on", "high"}, want: "--fail-on high --check"},
		{name: "replaces false", in: []string{"--check=false", "file.go"}, want: "file.go --check"},
		{name: "one check at end", in: []string{"--check", "--check=false"}, want: "--check"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := stringsJoin(hookCheckOnlyArgs(tc.in))
			if got != tc.want {
				t.Fatalf("hookCheckOnlyArgs() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestHookReportOnlyDependencyArgs(t *testing.T) {
	got := stringsJoin(hookReportOnlyDependencyArgs([]string{
		"--licenses", "--output", "README.md", "--sbom", "--sbom-output=sbom/out.json", "--vuln", "--fail-on", "high",
	}))
	want := "--licenses --fail-on high"
	if got != want {
		t.Fatalf("hookReportOnlyDependencyArgs() = %q, want %q", got, want)
	}
}

func TestAssessHook_FixRequestStaysCheck(t *testing.T) {
	original := assess.GetAssessmentRunnerRegistry()
	registry := assess.ResetRegistryForTesting()
	probe := &hookModeProbe{}
	registry.RegisterRunner(assess.CategoryLint, probe)
	t.Cleanup(func() {
		assess.RestoreRegistry(original)
		assessMode, assessNoOp, assessCheck, assessFix = "", false, false, false
		assessHook, assessHookManifest = "", ".goneat/hooks.yaml"
		assessLintShellFix = false
	})

	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".goneat"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := "version: \"1.0.0\"\nhooks:\n  pre-push:\n    - command: \"assess\"\n      args: [\"--categories\", \"lint\", \"--fail-on\", \"critical\"]\n      priority: 10\n      timeout: \"45s\"\n"
	if err := os.WriteFile(filepath.Join(repo, ".goneat", "hooks.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(repo)

	var stdout, stderr bytes.Buffer
	cmd := &cobra.Command{Use: "assess", RunE: runAssess}
	setupAssessCommandFlags(cmd)
	cmd.SilenceUsage, cmd.SilenceErrors = true, true
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--hook", "pre-push", "--hook-manifest", ".goneat/hooks.yaml", "--fix", "--lint-shell-fix", "--format", "json", "--concurrency", "1"})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("hook assess: %v\n%s", err, stderr.String())
	}
	if probe.mode != assess.AssessmentModeCheck || !probe.readOnly || probe.shellFix {
		t.Fatalf("hook delivered mode=%q readOnly=%v shellFix=%v", probe.mode, probe.readOnly, probe.shellFix)
	}
}

func stringsJoin(args []string) string {
	out := ""
	for i, arg := range args {
		if i > 0 {
			out += " "
		}
		out += arg
	}
	return out
}
