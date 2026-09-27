package assess

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
)

func TestResolveYamllintTargets_DefaultPatterns(t *testing.T) {
	tdir := t.TempDir()
	paths := []string{
		".github/workflows/build.yml",
		".github/workflows/deploy.yaml",
		"docs/workflows/ignored.yaml",
	}
	for _, p := range paths {
		full := filepath.Join(tdir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir failed: %v", err)
		}
		if err := os.WriteFile(full, []byte("name: test"), 0o644); err != nil {
			t.Fatalf("write failed: %v", err)
		}
	}

	files, err := resolveYamllintTargets(tdir, DefaultAssessmentConfig(), nil)
	if err != nil {
		t.Fatalf("resolveYamllintTargets error: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d (%v)", len(files), files)
	}
}

func TestResolveYamllintTargets_WithOverrides(t *testing.T) {
	tdir := t.TempDir()
	files := []string{"workflows/root.yaml", "workflows/skip.yaml"}
	for _, p := range files {
		full := filepath.Join(tdir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir failed: %v", err)
		}
		if err := os.WriteFile(full, []byte("name: test"), 0o644); err != nil {
			t.Fatalf("write failed: %v", err)
		}
	}
	cfg := &yamllintOverrides{
		Paths:  []string{"workflows/*.yaml"},
		Ignore: []string{"**/skip.yaml"},
	}
	result, err := resolveYamllintTargets(tdir, DefaultAssessmentConfig(), cfg)
	if err != nil {
		t.Fatalf("resolve error: %v", err)
	}
	if len(result) != 1 || result[0] != "workflows/root.yaml" {
		t.Fatalf("unexpected result: %v", result)
	}
}

func TestParseYamllintOutput(t *testing.T) {
	output := "env.yaml:4:1: [warning] missing document start \"---\" (document-start)\n" +
		"env.yaml:10:5: [error] wrong indentation (indentation)"
	issues := parseYamllintOutput(output, ".")
	if len(issues) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(issues))
	}
	if issues[0].Severity != SeverityMedium {
		t.Fatalf("expected warning severity, got %v", issues[0].Severity)
	}
	if issues[1].Severity != SeverityHigh {
		t.Fatalf("expected error severity, got %v", issues[1].Severity)
	}
}

// yamllint --strict exits 2 when it reports only warnings; those are
// completed findings, not a failure to lint.
func TestRunYamllint_ExitCodes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a shell-script stand-in for yamllint")
	}
	repo := t.TempDir()
	wf := filepath.Join(repo, ".github", "workflows")
	if err := os.MkdirAll(wf, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wf, "ci.yml"), []byte("on: push\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	const finding = ".github/workflows/ci.yml:1:1: [warning] missing document start \"---\" (document-start)"
	fake := func(code int, out, errOut string) string {
		p := filepath.Join(t.TempDir(), "yamllint")
		script := "#!/bin/sh\necho '" + out + "'\n"
		if errOut != "" {
			script += "echo '" + errOut + "' >&2\n"
		}
		script += "exit " + strconv.Itoa(code) + "\n"
		if err := os.WriteFile(p, []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
		return p
	}
	strictOff := false
	r := NewLintAssessmentRunner()
	cases := []struct {
		name      string
		code      int
		out       string
		errOut    string
		overrides *assessOverrides
		wantErr   bool
	}{
		{"strict warnings", 2, finding, "", nil, false},
		{"errors", 1, finding, "", nil, false},
		{"exit 2 without strict", 2, finding, "", &assessOverrides{Lint: &lintOverrides{Yamllint: &yamllintOverrides{Strict: &strictOff}}}, true},
		{"could not run", 255, finding, "", nil, true},
		{"strict exit 2 with empty report", 2, "", "", nil, true},
		{"strict exit 2 with malformed report", 2, "Traceback: something broke", "", nil, true},
		{"exit 1 with malformed report", 1, "invalid config: no such rule", "", nil, true},
		{"finding plus failure on stderr", 2, finding, "yamllint: failed to load configuration: unknown rule", nil, true},
		{"finding plus unparsed stdout line", 1, finding + "\nerror: could not read .yamllint", "", nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GONEAT_YAMLLINT_BIN", fake(tc.code, tc.out, tc.errOut))
			issues, err := r.runYamllintAssessment(repo, DefaultAssessmentConfig(), tc.overrides)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("exit %d must be an error", tc.code)
				}
				return
			}
			if err != nil || len(issues) != 1 {
				t.Fatalf("exit %d must report the finding, got issues=%v err=%v", tc.code, issues, err)
			}
		})
	}
}

// A missing shfmt is an optional tool: shell lint is skipped, not failed.
func TestRunShfmt_MissingToolSkips(t *testing.T) {
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "run.sh"), []byte("#!/bin/sh\necho hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	cfg := DefaultAssessmentConfig()
	cfg.LintShellEnabled = true
	issues, err := NewLintAssessmentRunner().runShfmtAssessment(repo, cfg, nil)
	if err != nil || len(issues) != 0 {
		t.Fatalf("missing shfmt must skip, got issues=%v err=%v", issues, err)
	}
}
