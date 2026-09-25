package assess

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	clippyWarningLine = `{"reason":"compiler-message","message":{"message":"unneeded return statement","level":"warning","code":{"code":"clippy::needless_return"},"spans":[{"file_name":"src/lib.rs","line_start":3,"column_start":5,"is_primary":true}]}}`
	clippyErrorLine   = `{"reason":"compiler-message","message":{"message":"mismatched types","level":"error","code":{"code":"E0308"},"spans":[{"file_name":"src/lib.rs","line_start":7,"column_start":9,"is_primary":true}]}}`
	buildOK           = `{"reason":"build-finished","success":true}`
	buildFailed       = `{"reason":"build-finished","success":false}`
)

// setupFakeClippyRepo writes a single-crate repo and a fake cargo whose
// `clippy` run prints stdoutLines, writes stderr, and exits with exitCode.
// Every clippy argv is appended to args.txt, one invocation per line.
func setupFakeClippyRepo(t *testing.T, stdoutLines []string, stderr string, exitCode int) string {
	t.Helper()
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "Cargo.toml"), []byte("[package]\nname = \"demo\"\nversion = \"0.1.0\"\nedition = \"2021\"\n"), 0o644); err != nil {
		t.Fatalf("write Cargo.toml: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "src"), 0o755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "src", "lib.rs"), []byte("pub fn hi() {}\n"), 0o644); err != nil {
		t.Fatalf("write lib.rs: %v", err)
	}
	stdoutFile := filepath.Join(repo, "clippy-stdout.txt")
	if err := os.WriteFile(stdoutFile, []byte(strings.Join(stdoutLines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write stdout fixture: %v", err)
	}
	stderrFile := filepath.Join(repo, "clippy-stderr.txt")
	if err := os.WriteFile(stderrFile, []byte(stderr), 0o644); err != nil {
		t.Fatalf("write stderr fixture: %v", err)
	}
	script := "#!/usr/bin/env bash\n" +
		"args=(\"$@\")\n" +
		"if [[ \"${args[0]}\" == +* ]]; then args=(\"${args[@]:1}\"); fi\n" +
		"if [[ \"${args[0]}\" == \"--version\" ]]; then echo 'cargo 1.98.0'; exit 0; fi\n" +
		"if [[ \"${args[0]}\" == \"clippy\" && \"${args[1]:-}\" == \"--version\" ]]; then echo 'clippy 0.1.98'; exit 0; fi\n" +
		"if [[ \"${args[0]}\" == \"clippy\" ]]; then\n" +
		"  echo \"$*\" >> \"" + filepath.Join(repo, "args.txt") + "\"\n" +
		"  cat \"" + stdoutFile + "\"\n" +
		"  cat \"" + stderrFile + "\" >&2\n" +
		"  exit " + strconv.Itoa(exitCode) + "\n" +
		"fi\n" +
		"exit 0\n"
	writeFakeCargo(t, repo, script)
	return repo
}

func writeAssessYAML(t *testing.T, repo, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(repo, ".goneat"), 0o755); err != nil {
		t.Fatalf("mkdir .goneat: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".goneat", "assess.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("write assess.yaml: %v", err)
	}
}

func readClippyArgs(t *testing.T, repo string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repo, "args.txt"))
	if err != nil {
		t.Fatalf("read args.txt: %v", err)
	}
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

func checkCfg() AssessmentConfig {
	cfg := DefaultAssessmentConfig()
	cfg.Mode = AssessmentModeCheck
	return cfg
}

func TestClippy_FailClosed(t *testing.T) {
	cases := []struct {
		name       string
		stdout     []string
		stderr     string
		exit       int
		wantErr    string // substring; "" means no error
		wantIssues int
	}{
		{
			name:       "clean run with warnings passes",
			stdout:     []string{clippyWarningLine, buildOK},
			wantIssues: 1,
		},
		{
			name:    "broken manifest fails with stderr",
			stderr:  "error: failed to parse manifest at `Cargo.toml`\nCaused by:\n  invalid TOML value",
			exit:    101,
			wantErr: "failed to parse manifest",
		},
		{
			name:    "unresolvable dependency fails",
			stderr:  "error: failed to select a version for the requirement `nosuchcrate = \"^9\"`",
			exit:    101,
			wantErr: "failed to select a version",
		},
		{
			name:       "warning then later cargo failure fails and keeps diagnostics",
			stdout:     []string{clippyWarningLine, buildFailed},
			stderr:     "error: failed to run custom build command for `demo v0.1.0`",
			exit:       101,
			wantErr:    "before completing a lint run",
			wantIssues: 1,
		},
		{
			name:       "compile error fails and keeps diagnostics",
			stdout:     []string{clippyWarningLine, clippyErrorLine, buildFailed},
			stderr:     "error: could not compile `demo` (lib) due to 1 previous error",
			exit:       101,
			wantErr:    "compilation failed with 1 error diagnostic",
			wantIssues: 2,
		},
		{
			name:    "missing target std fails with rustup hint",
			stderr:  "error[E0463]: can't find crate for `core`\n  = note: the `x86_64-unknown-linux-gnu` target may not be installed",
			exit:    101,
			wantErr: "rustup target add",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := setupFakeClippyRepo(t, tc.stdout, tc.stderr, tc.exit)
			issues, err := runCargoClippyLint(repo, checkCfg())
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
			if len(issues) != tc.wantIssues {
				t.Fatalf("expected %d issues, got %d: %+v", tc.wantIssues, len(issues), issues)
			}
		})
	}
}

func TestClippy_LintRunnerKeepsDiagnosticsOnFailure(t *testing.T) {
	repo := setupFakeClippyRepo(t, []string{clippyWarningLine, buildFailed}, "error: failed to run custom build command", 101)
	cfg := checkCfg()
	cfg.SelectedCategories = []string{string(CategoryLint)}
	result, err := NewLintAssessmentRunner().Assess(t.Context(), repo, cfg)
	if err != nil {
		t.Fatalf("Assess: %v", err)
	}
	if result.Success {
		t.Fatalf("expected lint Success=false when cargo fails")
	}
	if !strings.Contains(result.Error, "cargo-clippy failed") {
		t.Fatalf("expected cargo-clippy error, got %q", result.Error)
	}
	found := false
	for _, issue := range result.Issues {
		if issue.SubCategory == "rust:clippy" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected clippy diagnostic to be preserved, got %+v", result.Issues)
	}
}

func TestClippy_DefaultArgsUnchanged(t *testing.T) {
	repo := setupFakeClippyRepo(t, []string{buildOK}, "", 0)
	if _, err := runCargoClippyLint(repo, checkCfg()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := readClippyArgs(t, repo); len(got) != 1 || got[0] != "clippy --message-format=json" {
		t.Fatalf("default invocation changed: %q", got)
	}
}

func TestClippy_StructuredConfig(t *testing.T) {
	repo := setupFakeClippyRepo(t, []string{clippyWarningLine, buildOK}, "", 0)
	writeAssessYAML(t, repo, `version: 1
lint:
  rust:
    clippy:
      toolchain: stable
      all_targets: true
      features: [async, serde/derive]
      no_default_features: true
      locked: true
      packages: [demo]
      targets: [x86_64-unknown-linux-gnu, aarch64-unknown-linux-gnu]
`)
	issues, err := runCargoClippyLint(repo, checkCfg())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Same warning from both target runs collapses to one issue.
	if len(issues) != 1 {
		t.Fatalf("expected duplicate warnings to collapse to 1 issue, got %d", len(issues))
	}
	got := readClippyArgs(t, repo)
	want := []string{
		"+stable clippy --message-format=json -p demo --all-targets --no-default-features --features async,serde/derive --locked --target x86_64-unknown-linux-gnu",
		"+stable clippy --message-format=json -p demo --all-targets --no-default-features --features async,serde/derive --locked --target aarch64-unknown-linux-gnu",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("unexpected invocations:\n got: %q\nwant: %q", got, want)
	}
}

func TestClippy_RejectsOptionShapedValues(t *testing.T) {
	cases := map[string]string{
		"toolchain": "      toolchain: \"--config=evil\"\n",
		"features":  "      features: [\"--manifest-path=/tmp/x\"]\n",
		"packages":  "      packages: [\"-Zunstable\"]\n",
		"targets":   "      targets: [\"x86 64\"]\n",
	}
	for field, body := range cases {
		t.Run(field, func(t *testing.T) {
			repo := setupFakeClippyRepo(t, []string{buildOK}, "", 0)
			writeAssessYAML(t, repo, "version: 1\nlint:\n  rust:\n    clippy:\n"+body)
			_, err := runCargoClippyLint(repo, checkCfg())
			if err == nil || !strings.Contains(err.Error(), "invalid lint.rust.clippy."+field) {
				t.Fatalf("expected invalid %s error, got %v", field, err)
			}
			if _, statErr := os.Stat(filepath.Join(repo, "args.txt")); statErr == nil {
				t.Fatalf("cargo clippy must not run with an invalid %s value", field)
			}
		})
	}
}

func TestClippy_MissingConfiguredToolchainFails(t *testing.T) {
	repo := setupFakeClippyRepo(t, []string{buildOK}, "", 0)
	// Replace the fake so any "+toolchain" invocation reports it is not installed.
	script := "#!/usr/bin/env bash\n" +
		"if [[ \"$1\" == +* ]]; then echo \"error: toolchain '${1#+}' is not installed\" >&2; exit 1; fi\n" +
		"if [[ \"$1\" == \"--version\" ]]; then echo 'cargo 1.98.0'; exit 0; fi\n" +
		"if [[ \"$1\" == \"clippy\" && \"${2:-}\" == \"--version\" ]]; then echo 'clippy 0.1.98'; exit 0; fi\n" +
		"exit 0\n"
	writeFakeCargo(t, repo, script)
	writeAssessYAML(t, repo, "version: 1\nlint:\n  rust:\n    clippy:\n      toolchain: nightly-2020-01-01\n")
	_, err := runCargoClippyLint(repo, checkCfg())
	if err == nil || !strings.Contains(err.Error(), "not available for configured toolchain") {
		t.Fatalf("expected missing-toolchain failure, got %v", err)
	}
}

func TestClippy_DisabledSkips(t *testing.T) {
	repo := setupFakeClippyRepo(t, []string{clippyErrorLine, buildFailed}, "boom", 101)
	writeAssessYAML(t, repo, "version: 1\nlint:\n  rust:\n    clippy:\n      enabled: false\n")
	issues, err := runCargoClippyLint(repo, checkCfg())
	if err != nil || len(issues) != 0 {
		t.Fatalf("expected disabled clippy to skip, got issues=%d err=%v", len(issues), err)
	}
}

// cargo audit exits non-zero with a JSON report when it finds advisories; a
// non-zero exit without a report means it did not run and must not pass.
func TestCargoAudit_NoReportFailsClosed(t *testing.T) {
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "Cargo.toml"), []byte("[package]\nname = \"demo\"\nversion = \"0.1.0\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeFakeCargo(t, repo, "#!/usr/bin/env bash\n"+
		"if [[ \"$1\" == \"audit\" && \"$2\" == \"--json\" ]]; then echo 'error: Couldn'\"'\"'t load Cargo.lock' >&2; exit 1; fi\n"+
		"exit 0\n")
	adapter := &cargoAuditAdapter{moduleRoot: repo, cfg: checkCfg()}
	_, err := adapter.Run(t.Context())
	if err == nil || !strings.Contains(err.Error(), "Cargo.lock") {
		t.Fatalf("expected did-not-run error with stderr, got %v", err)
	}
}

func TestRunToolSplit_ReportsTimeout(t *testing.T) {
	_, err := runToolSplit(t.TempDir(), "sleep", []string{"5"}, 50*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("a killed/timed-out tool must be an error, got %v", err)
	}
}

func TestToolRun_FailedWithoutOutput(t *testing.T) {
	if !(toolRun{ExitCode: 1, Stdout: []byte("  \n")}).failedWithoutOutput() {
		t.Fatalf("non-zero exit with empty stdout must count as did-not-run")
	}
	if (toolRun{ExitCode: 1, Stdout: []byte(`{"diagnostics":[]}`)}).failedWithoutOutput() {
		t.Fatalf("non-zero exit with a report is findings, not did-not-run")
	}
}

func TestRustTokenValidation(t *testing.T) {
	accept := map[string][]string{
		"toolchain": {"stable", "nightly", "1.85.0", "nightly-2026-01-01", "nightly-x86_64-unknown-linux-gnu", "stable-aarch64-apple-darwin"},
		"targets":   {"x86_64-unknown-linux-gnu", "aarch64-pc-windows-msvc", "wasm32-unknown-unknown", "thumbv7em-none-eabihf"},
		"features":  {"async", "serde/derive", "full", "dep_name/feat-1"},
		"packages":  {"ipcprims-peer", "my_crate", "foo@1.2.3", "registry+https://github.com/rust-lang/crates.io-index#serde@1.0.0"},
	}
	reject := []string{"-Z", "--config=evil", "", " stable", "a b", "x;y", "$(id)", "a,b"}
	patterns := map[string]*regexpMatcher{
		"toolchain": {rustToolchainToken.MatchString},
		"targets":   {rustTargetToken.MatchString},
		"features":  {rustFeatureToken.MatchString},
		"packages":  {rustPackageToken.MatchString},
	}
	for field, match := range patterns {
		for _, v := range accept[field] {
			if field == "packages" && strings.Contains(v, "#") {
				// Full package-id specs with a source URL are out of scope; -p name[@version] is supported.
				continue
			}
			if !match.fn(v) {
				t.Errorf("%s: expected %q to be accepted", field, v)
			}
		}
		for _, v := range reject {
			if match.fn(v) {
				t.Errorf("%s: expected %q to be rejected", field, v)
			}
		}
	}

	cfg := clippyOverrides{Toolchain: "nightly-x86_64-unknown-linux-gnu"}
	if err := cfg.validate(); err != nil {
		t.Fatalf("host-qualified toolchain must be accepted: %v", err)
	}
}

type regexpMatcher struct{ fn func(string) bool }
