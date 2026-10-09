package assess

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Without golangci-lint the lint category still runs its other tools:
// clippy runs and reports a planted warning, and only Go lint is skipped,
// with a note.
func TestLint_RunsWithoutGolangciLint(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a shell-script stand-in for cargo")
	}
	repo := setupFakeClippyRepo(t, []string{clippyWarningLine, buildOK}, "", 0)
	if err := os.WriteFile(filepath.Join(repo, "go.mod"), []byte("module example.com/demo\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Only the fake cargo and system tools: golangci-lint is not reachable.
	t.Setenv("PATH", filepath.Join(repo, "bin")+string(os.PathListSeparator)+"/usr/bin:/bin")
	if golangciLintAvailable() {
		t.Skip("golangci-lint is installed in a system directory")
	}

	if !NewLintAssessmentRunner().IsAvailable() {
		t.Fatalf("lint must stay available without golangci-lint")
	}

	registry := NewAssessmentRunnerRegistry()
	registry.RegisterRunner(CategoryLint, NewLintAssessmentRunner())
	engine := &AssessmentEngine{runnerRegistry: registry, priorityManager: NewPriorityManager()}
	cfg := checkCfg()
	cfg.SelectedCategories = []string{string(CategoryLint)}
	cfg.Concurrency = 1
	report, err := engine.RunAssessment(context.Background(), repo, cfg)
	if err != nil {
		t.Fatal(err)
	}
	lint, ok := report.Categories[string(CategoryLint)]
	if !ok {
		t.Fatalf("lint category missing from report: %+v", report.Categories)
	}
	clippy := false
	for _, issue := range lint.Issues {
		if issue.SubCategory == "rust:clippy" {
			clippy = true
		}
	}
	if !clippy {
		t.Fatalf("clippy must run and report the planted warning, got status=%q issues=%+v error=%q", lint.Status, lint.Issues, lint.Error)
	}
	if len(lint.Notes) != 1 || !strings.Contains(lint.Notes[0], "golangci-lint not found") {
		t.Fatalf("expected a Go-lint-skipped note, got %v", lint.Notes)
	}
}

type unavailableRunner struct{ category AssessmentCategory }

func (u *unavailableRunner) Assess(context.Context, string, AssessmentConfig) (*AssessmentResult, error) {
	panic("an unavailable runner must not be run")
}
func (u *unavailableRunner) CanRunInParallel() bool                { return true }
func (u *unavailableRunner) GetCategory() AssessmentCategory       { return u.category }
func (u *unavailableRunner) GetEstimatedTime(string) time.Duration { return time.Millisecond }
func (u *unavailableRunner) IsAvailable() bool                     { return false }
func (u *unavailableRunner) UnavailableReason() string             { return "gosec not found" }

// A requested category that cannot run is reported as skipped with a
// reason; one that was not requested stays out of the report.
func TestEngine_RequestedUnavailableCategoryIsSkippedWithReason(t *testing.T) {
	registry := NewAssessmentRunnerRegistry()
	registry.RegisterRunner(CategorySecurity, &unavailableRunner{category: CategorySecurity})
	registry.RegisterRunner(CategoryStaticAnalysis, &unavailableRunner{category: CategoryStaticAnalysis})
	engine := &AssessmentEngine{runnerRegistry: registry, priorityManager: NewPriorityManager()}
	cfg := DefaultAssessmentConfig()
	cfg.SelectedCategories = []string{string(CategorySecurity)}
	report, err := engine.RunAssessment(context.Background(), t.TempDir(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	sec, ok := report.Categories[string(CategorySecurity)]
	if !ok || sec.Status != "skipped" || sec.Reason != "gosec not found" {
		t.Fatalf("requested unavailable category must be skipped with a reason, got %+v", report.Categories)
	}
	if _, ok := report.Categories[string(CategoryStaticAnalysis)]; ok {
		t.Fatalf("an unrequested unavailable category must not be reported")
	}
}

type countingRunner struct {
	category AssessmentCategory
	runs     int
}

func (c *countingRunner) Assess(context.Context, string, AssessmentConfig) (*AssessmentResult, error) {
	c.runs++
	return &AssessmentResult{CommandName: string(c.category), Category: c.category, Success: true}, nil
}
func (c *countingRunner) CanRunInParallel() bool                { return true }
func (c *countingRunner) GetCategory() AssessmentCategory       { return c.category }
func (c *countingRunner) GetEstimatedTime(string) time.Duration { return time.Millisecond }
func (c *countingRunner) IsAvailable() bool                     { return true }

// Requested names are validated before anything runs: an unknown name is an
// error naming the valid set, and a known sibling is not executed.
func TestEngine_UnknownRequestedCategory(t *testing.T) {
	lint := &countingRunner{category: CategoryLint}
	registry := NewAssessmentRunnerRegistry()
	registry.RegisterRunner(CategoryLint, lint)
	engine := &AssessmentEngine{runnerRegistry: registry, priorityManager: NewPriorityManager()}

	cases := []struct {
		name      string
		requested []string
		unknown   []string
		lintRuns  int
		skipped   string
	}{
		{"single unknown", []string{"lnt"}, []string{"lnt"}, 0, ""},
		{"mixed known and unknown", []string{"lint", "lnt", "sec", "lnt"}, []string{"lnt", "sec"}, 0, ""},
		{"accepted spellings", []string{" lint ", "", "lint"}, nil, 1, ""},
		{"defined category without a runner", []string{"lint", "performance"}, nil, 1, "performance"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lint.runs = 0
			cfg := DefaultAssessmentConfig()
			cfg.SelectedCategories = tc.requested
			report, err := engine.RunAssessment(context.Background(), t.TempDir(), cfg)
			if err != nil {
				t.Fatal(err)
			}
			if lint.runs != tc.lintRuns {
				t.Fatalf("lint ran %d times, want %d", lint.runs, tc.lintRuns)
			}
			for _, name := range tc.unknown {
				cr, ok := report.Categories[name]
				if !ok || cr.Status != "error" || !strings.Contains(cr.Error, "unknown assessment category \""+name+"\"") || !strings.Contains(cr.Error, "repo-status") {
					t.Fatalf("%q must be an error naming the valid set, got %+v", name, report.Categories)
				}
			}
			errors := 0
			for _, cr := range report.Categories {
				if cr.Status == "error" {
					errors++
				}
			}
			if errors != len(tc.unknown) {
				t.Fatalf("want %d error categories, got %+v", len(tc.unknown), report.Categories)
			}
			if tc.skipped != "" {
				if cr := report.Categories[tc.skipped]; cr.Status != "skipped" || cr.Reason == "" {
					t.Fatalf("%s must be skipped with a reason, got %+v", tc.skipped, cr)
				}
			}
		})
	}
}

// Security runs whatever applicable adapter is installed: a Cargo project
// with only cargo-audit (no gosec or govulncheck) is assessed, not skipped.
// With no applicable tool at all the category is skipped with a reason.
func TestSecurity_RunsApplicableAdapterWithoutGoScanners(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a shell-script stand-in for cargo")
	}
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "Cargo.toml"), []byte("[package]\nname = \"demo\"\nversion = \"0.1.0\"\nedition = \"2021\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(repo, "audit-ran")
	writeFakeCargo(t, repo, "#!/usr/bin/env bash\n"+
		"if [[ \"$1\" == \"audit\" && \"$2\" == \"--version\" ]]; then echo 'cargo-audit-audit 0.22.2'; exit 0; fi\n"+
		"if [[ \"$1\" == \"audit\" && \"$2\" == \"--json\" ]]; then touch '"+marker+"'; echo '"+cargoAuditCleanFixture+"'; exit 0; fi\n"+
		"exit 1\n")
	t.Setenv("PATH", filepath.Join(repo, "bin")+string(os.PathListSeparator)+"/usr/bin:/bin")
	for _, tool := range []string{"gosec", "govulncheck"} {
		if _, err := exec.LookPath(tool); err == nil {
			t.Skipf("%s is installed in a system directory", tool)
		}
	}

	run := func(target string) CategoryResult {
		t.Helper()
		registry := NewAssessmentRunnerRegistry()
		registry.RegisterRunner(CategorySecurity, NewSecurityAssessmentRunner())
		engine := &AssessmentEngine{runnerRegistry: registry, priorityManager: NewPriorityManager()}
		cfg := checkCfg()
		cfg.SelectedCategories = []string{string(CategorySecurity)}
		cfg.Concurrency = 1
		report, err := engine.RunAssessment(context.Background(), target, cfg)
		if err != nil {
			t.Fatal(err)
		}
		return report.Categories[string(CategorySecurity)]
	}

	if cr := run(repo); cr.Status == "skipped" || cr.Status == "error" {
		t.Fatalf("cargo-audit applies and must run, got %+v", cr)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("cargo audit was not executed")
	}

	cr := run(t.TempDir()) // no Cargo.toml, no Go scanners, no gitleaks
	if cr.Status != "skipped" || !strings.Contains(cr.Reason, "no applicable security tool") {
		t.Fatalf("with no applicable tool security must be skipped with a reason, got %+v", cr)
	}
}
