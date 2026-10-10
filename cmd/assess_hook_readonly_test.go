package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
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

func TestPushGateIsTheCheckOnlyAssess(t *testing.T) {
	root := findModuleRoot(t)
	makefile := string(mustRead(t, filepath.Join(root, "Makefile")))
	prepush := makefileRecipe(t, makefile, "prepush")
	precommit := makefileRecipe(t, makefile, "precommit")
	for _, recipe := range []string{prepush, precommit} {
		if strings.Contains(recipe, "release-check") || strings.Contains(recipe, "embed-assets") || strings.Contains(recipe, "build-all") {
			t.Fatalf("push gate recipe includes a write target:\n%s", recipe)
		}
	}
	if !strings.Contains(prepush, "assess --mode check --hook pre-push") {
		t.Fatalf("prepush recipe is not the check-only assess:\n%s", prepush)
	}
	if !strings.Contains(precommit, "assess --mode check --hook pre-commit") {
		t.Fatalf("precommit recipe is not the check-only assess:\n%s", precommit)
	}
	hook := string(mustRead(t, filepath.Join(root, "templates", "hooks", "bash", "pre-push.sh.tmpl")))
	if strings.Contains(hook, "make ") {
		t.Fatal("pre-push hook template calls make")
	}
	if !strings.Contains(hook, "assess --mode check --hook pre-push") {
		t.Fatal("pre-push hook template is not the check-only assess")
	}
	ci := string(mustRead(t, filepath.Join(root, ".github", "workflows", "ci.yml")))
	if !strings.Contains(ci, "make prepush") {
		t.Fatal("CI does not run make prepush")
	}
}

func findModuleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func makefileRecipe(t *testing.T, makefile, target string) string {
	t.Helper()
	prefix := target + ":"
	start := strings.Index(makefile, prefix)
	if start < 0 {
		t.Fatalf("makefile target %s not found", target)
	}
	rest := makefile[start+len(prefix):]
	end := strings.Index(rest, "\n\n")
	if end < 0 {
		end = len(rest)
	}
	return rest[:end]
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
