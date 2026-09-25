package assess

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// setupFakeRustfmtRepo creates a crate plus a fake cargo. fmt --version
// succeeds unless fmtMissing; `fmt --all -- --check -l` prints the lines in
// checkStdout, writes checkStderr and exits checkExit.
func setupFakeRustfmtRepo(t *testing.T, fmtMissing bool, checkStdout []string, checkStderr string, checkExit int) string {
	t.Helper()
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "Cargo.toml"), []byte("[package]\nname = \"demo\"\nversion = \"0.1.0\"\nedition = \"2021\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "src", "lib.rs"), []byte("pub fn hi(){}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdoutFile := filepath.Join(repo, "fmt-stdout.txt")
	stderrFile := filepath.Join(repo, "fmt-stderr.txt")
	var lines []string
	for _, l := range checkStdout {
		lines = append(lines, strings.ReplaceAll(l, "$REPO", repo))
	}
	if err := os.WriteFile(stdoutFile, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stderrFile, []byte(checkStderr), 0o644); err != nil {
		t.Fatal(err)
	}
	versionExit := "0"
	if fmtMissing {
		versionExit = "1"
	}
	script := "#!/usr/bin/env bash\n" +
		"echo \"$*\" >> \"" + filepath.Join(repo, "args.txt") + "\"\n" +
		"args=(\"$@\")\n" +
		"if [[ \"${args[0]}\" == +* ]]; then args=(\"${args[@]:1}\"); fi\n" +
		"if [[ \"${args[0]}\" == \"--version\" ]]; then echo 'cargo 1.98.0'; exit 0; fi\n" +
		"if [[ \"${args[0]}\" == \"fmt\" && \"${args[1]}\" == \"--version\" ]]; then\n" +
		"  if [[ " + versionExit + " != 0 ]]; then echo \"error: 'cargo-fmt' is not installed for the toolchain\" >&2; fi\n" +
		"  echo 'rustfmt 1.9.0'; exit " + versionExit + "\n" +
		"fi\n" +
		"if [[ \"${args[0]}\" == \"fmt\" && \" ${args[*]} \" == *\" --check \"* ]]; then\n" +
		"  cat \"" + stdoutFile + "\"; cat \"" + stderrFile + "\" >&2; exit " + itoaSmall(checkExit) + "\n" +
		"fi\n" +
		"exit 0\n"
	writeFakeCargo(t, repo, script)
	return repo
}

func itoaSmall(n int) string { return strconv.Itoa(n) }

func TestRunCargoFmt_Classification(t *testing.T) {
	cases := []struct {
		name      string
		missing   bool
		stdout    []string
		stderr    string
		exit      int
		wantFiles int
		wantErr   string
	}{
		{name: "clean", wantFiles: 0},
		{name: "drift lists files", stdout: []string{"$REPO/src/lib.rs"}, exit: 1, wantFiles: 1},
		{name: "drift with nightly-option warning", stdout: []string{"$REPO/src/lib.rs"}, stderr: "Warning: can't set `imports_granularity = Crate`, unstable features are only available in nightly channel.", exit: 1, wantFiles: 1},
		{name: "parse error fails", stderr: "error: expected expression, found `}`", exit: 1, wantErr: "expected expression"},
		{name: "parse error alongside drift fails", stdout: []string{"$REPO/src/lib.rs"}, stderr: "error: expected expression", exit: 1, wantErr: "expected expression"},
		{name: "unexpected exit fails", exit: 2, wantErr: "exited 2"},
		{name: "rustfmt missing", missing: true, wantErr: "rustfmt unavailable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := setupFakeRustfmtRepo(t, tc.missing, tc.stdout, tc.stderr, tc.exit)
			res, err := RunCargoFmt(RustFormatRequest{Root: repo, Check: true})
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
				}
				if tc.missing && !errors.Is(err, ErrRustfmtUnavailable) {
					t.Fatalf("missing rustfmt must wrap ErrRustfmtUnavailable, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(res.Files) != tc.wantFiles {
				t.Fatalf("expected %d files, got %v", tc.wantFiles, res.Files)
			}
		})
	}
}

func TestRustFormatAssessment_ReportsAndHonorsConfig(t *testing.T) {
	repo := setupFakeRustfmtRepo(t, false, []string{"$REPO/src/lib.rs"}, "", 1)
	t.Chdir(repo)

	issues, err := runRustFormatAssessment(repo, checkCfg())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 1 || issues[0].SubCategory != "rust:rustfmt" || !issues[0].AutoFixable {
		t.Fatalf("expected one auto-fixable rust:rustfmt issue, got %+v", issues)
	}

	if err := os.WriteFile(filepath.Join(repo, ".goneat.yaml"), []byte("format:\n  rust:\n    enabled: false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(repo, "args.txt")); err != nil {
		t.Fatal(err)
	}
	issues, err = runRustFormatAssessment(repo, checkCfg())
	if err != nil || len(issues) != 0 {
		t.Fatalf("format.rust.enabled: false must skip, got issues=%v err=%v", issues, err)
	}
	if _, statErr := os.Stat(filepath.Join(repo, "args.txt")); statErr == nil {
		t.Fatalf("cargo must not run when format.rust.enabled is false")
	}
}

func TestRustFormatAssessment_MissingRustfmtFailsClosed(t *testing.T) {
	repo := setupFakeRustfmtRepo(t, true, nil, "", 0)
	t.Chdir(repo)
	_, err := runRustFormatAssessment(repo, checkCfg())
	if !errors.Is(err, ErrRustfmtUnavailable) || !strings.Contains(err.Error(), "format.rust.enabled: false") {
		t.Fatalf("in-scope Rust with missing rustfmt must fail with the opt-out hint, got %v", err)
	}

	// Opt-out: format.rust.enabled: false skips Rust formatting.
	if err := os.WriteFile(filepath.Join(repo, ".goneat.yaml"), []byte("format:\n  rust:\n    enabled: false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if issues, err := runRustFormatAssessment(repo, checkCfg()); err != nil || len(issues) != 0 {
		t.Fatalf("format.rust.enabled: false must skip, got issues=%v err=%v", issues, err)
	}

	// A toolchain named in config that lacks rustfmt fails.
	if err := os.WriteFile(filepath.Join(repo, ".goneat.yaml"), []byte("format:\n  rust:\n    toolchain: nightly\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runRustFormatAssessment(repo, checkCfg()); err == nil || !strings.Contains(err.Error(), `toolchain "nightly"`) {
		t.Fatalf("expected configured-toolchain failure, got %v", err)
	}
}

func TestRustFormatAssessment_MissingCargoFailsClosed(t *testing.T) {
	repo := setupFakeRustfmtRepo(t, false, nil, "", 0)
	t.Chdir(repo)
	t.Setenv("PATH", t.TempDir()) // no cargo on PATH
	if _, err := runRustFormatAssessment(repo, checkCfg()); !errors.Is(err, ErrRustfmtUnavailable) {
		t.Fatalf("in-scope Rust without cargo must fail closed, got %v", err)
	}
}

func TestRustFormatAssessment_NoRustInScopeDoesNotRequireRustfmt(t *testing.T) {
	repo := setupFakeRustfmtRepo(t, true, nil, "", 0)
	t.Chdir(repo)
	cfg := checkCfg()
	cfg.IncludeFiles = []string{"README.md"}
	if issues, err := runRustFormatAssessment(repo, cfg); err != nil || len(issues) != 0 {
		t.Fatalf("no .rs in scope must not require rustfmt, got issues=%v err=%v", issues, err)
	}
	nonRust := t.TempDir()
	if issues, err := runRustFormatAssessment(nonRust, checkCfg()); err != nil || len(issues) != 0 {
		t.Fatalf("non-Cargo target must not require rustfmt, got issues=%v err=%v", issues, err)
	}
}

func TestRustFormatAssessment_ScopedToIncludeFiles(t *testing.T) {
	repo := setupFakeRustfmtRepo(t, false, []string{"$REPO/src/lib.rs", "$REPO/src/other.rs"}, "", 1)
	t.Chdir(repo)
	cfg := checkCfg()
	cfg.IncludeFiles = []string{"src/lib.rs"}
	issues, err := runRustFormatAssessment(repo, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 1 || !strings.HasSuffix(issues[0].File, "src/lib.rs") {
		t.Fatalf("expected only the selected file, got %+v", issues)
	}
	cfg.IncludeFiles = []string{"README.md"}
	if issues, _ := runRustFormatAssessment(repo, cfg); len(issues) != 0 {
		t.Fatalf("no .rs file in scope must skip rustfmt, got %+v", issues)
	}
}

// assess <target> run from another working directory must use the target's
// .goneat.yaml, not the working directory's.
func TestRustFormatAssessment_UsesTargetProjectConfig(t *testing.T) {
	repo := setupFakeRustfmtRepo(t, false, []string{"$REPO/src/lib.rs"}, "", 1)
	elsewhere := t.TempDir()
	if err := os.WriteFile(filepath.Join(elsewhere, ".goneat.yaml"), []byte("format:\n  rust:\n    enabled: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(elsewhere)

	if err := os.WriteFile(filepath.Join(repo, ".goneat.yaml"), []byte("format:\n  rust:\n    enabled: false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if issues, err := runRustFormatAssessment(repo, checkCfg()); err != nil || len(issues) != 0 {
		t.Fatalf("target's format.rust.enabled: false must apply from another cwd, got issues=%v err=%v", issues, err)
	}

	if err := os.WriteFile(filepath.Join(repo, ".goneat.yaml"), []byte("format:\n  rust:\n    toolchain: stable\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runRustFormatAssessment(repo, checkCfg()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	args, _ := os.ReadFile(filepath.Join(repo, "args.txt"))
	if !strings.Contains(string(args), "+stable fmt --all -- --check -l") {
		t.Fatalf("target's format.rust.toolchain must be used from another cwd, got %q", args)
	}
}

// An invalid .goneat.yaml at the target must fail the Rust format check with
// the file path, not silently use defaults.
func TestRustFormatAssessment_InvalidTargetProjectConfigFails(t *testing.T) {
	repo := setupFakeRustfmtRepo(t, false, nil, "", 0)
	t.Chdir(t.TempDir())
	for name, body := range map[string]string{
		"wrong type":    "format:\n  rust:\n    toolchain: [stable]\n",
		"unknown key":   "format:\n  rust:\n    toolchian: stable\n",
		"invalid token": "format:\n  rust:\n    toolchain: \"--config=x\"\n",
		"unparseable":   "format: [\n",
	} {
		t.Run(name, func(t *testing.T) {
			if err := os.WriteFile(filepath.Join(repo, ".goneat.yaml"), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := runRustFormatAssessment(repo, checkCfg())
			if err == nil {
				t.Fatalf("invalid project config must fail")
			}
			if !strings.Contains(err.Error(), ".goneat.yaml") {
				t.Fatalf("error must name the config file, got %v", err)
			}
		})
	}
}

// Without Rust in scope, the Rust check does not load (or judge) project config.
func TestRustFormatAssessment_InvalidConfigIgnoredWithoutRust(t *testing.T) {
	nonRust := t.TempDir()
	if err := os.WriteFile(filepath.Join(nonRust, ".goneat.yaml"), []byte("format:\n  rust:\n    toolchain: [stable]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if issues, err := runRustFormatAssessment(nonRust, checkCfg()); err != nil || len(issues) != 0 {
		t.Fatalf("non-Cargo target must not be affected, got issues=%v err=%v", issues, err)
	}
}
