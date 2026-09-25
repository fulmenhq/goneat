package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	formatpkg "github.com/fulmenhq/goneat/pkg/format"
	"github.com/spf13/cobra"
)

func setupFormatRustRepo(t *testing.T, fmtMissing bool, goneatYAML string) string {
	t.Helper()
	repo := t.TempDir()
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.WriteFile(filepath.Join(repo, "Cargo.toml"), []byte("[package]\nname = \"demo\"\nversion = \"0.1.0\"\nedition = \"2021\"\n"), 0o644))
	must(os.MkdirAll(filepath.Join(repo, "src"), 0o755))
	must(os.WriteFile(filepath.Join(repo, "src", "lib.rs"), []byte("pub fn hi(){}\n"), 0o644))
	if goneatYAML != "" {
		must(os.WriteFile(filepath.Join(repo, ".goneat.yaml"), []byte(goneatYAML), 0o644))
	}
	versionExit := "0"
	if fmtMissing {
		versionExit = "1"
	}
	binDir := filepath.Join(repo, "bin")
	must(os.MkdirAll(binDir, 0o755))
	script := "#!/usr/bin/env bash\n" +
		"echo \"$*\" >> \"" + filepath.Join(repo, "args.txt") + "\"\n" +
		"if [[ \"$1\" == +* ]]; then shift; fi\n" +
		"if [[ \"$1\" == \"--version\" ]]; then echo 'cargo 1.98.0'; exit 0; fi\n" +
		"if [[ \"$1\" == \"fmt\" && \"$2\" == \"--version\" ]]; then exit " + versionExit + "; fi\n" +
		"if [[ \"$1\" == \"fmt\" && \" $* \" == *\" --check \"* ]]; then echo \"$PWD/src/lib.rs\"; exit 1; fi\n" +
		"exit 0\n"
	// #nosec G306 -- test fake needs to be executable
	must(os.WriteFile(filepath.Join(binDir, "cargo"), []byte(script), 0o755))
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Chdir(repo)
	return repo
}

func newRustFormatCmd(t *testing.T, check, ignoreMissing bool, types []string) *cobra.Command {
	t.Helper()
	cmd := &cobra.Command{}
	cmd.Flags().StringSlice("files", []string{}, "")
	cmd.Flags().StringSlice("folders", []string{}, "")
	cmd.Flags().Bool("check", check, "")
	cmd.Flags().Bool("quiet", true, "")
	cmd.Flags().StringSlice("types", types, "")
	cmd.Flags().String("strategy", "sequential", "")
	cmd.Flags().Int("max-depth", -1, "")
	cmd.Flags().Bool("ignore-missing-tools", ignoreMissing, "")
	return cmd
}

func readArgs(t *testing.T, repo string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repo, "args.txt"))
	if err != nil {
		return ""
	}
	return string(data)
}

func TestFormatRust_CheckReportsDrift(t *testing.T) {
	repo := setupFormatRustRepo(t, false, "")
	err := RunFormat(newRustFormatCmd(t, true, false, []string{"rust"}), nil)
	if !errors.Is(err, formatpkg.ErrFormatDrift) {
		t.Fatalf("expected format drift, got %v", err)
	}
	if !strings.Contains(readArgs(t, repo), "fmt --all -- --check -l") {
		t.Fatalf("expected cargo fmt check invocation, got %q", readArgs(t, repo))
	}
}

func TestFormatRust_FixRunsCargoFmt(t *testing.T) {
	repo := setupFormatRustRepo(t, false, "format:\n  rust:\n    enabled: true\n")
	if err := RunFormat(newRustFormatCmd(t, false, false, []string{"rust"}), nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(readArgs(t, repo), "fmt --all\n") {
		t.Fatalf("expected cargo fmt --all, got %q", readArgs(t, repo))
	}
}

func TestFormatRust_DisabledByProjectConfig(t *testing.T) {
	repo := setupFormatRustRepo(t, false, "format:\n  rust:\n    enabled: false\n")
	if err := RunFormat(newRustFormatCmd(t, true, false, []string{"rust"}), nil); err != nil {
		t.Fatalf("format.rust.enabled: false must skip Rust, got %v", err)
	}
	if strings.Contains(readArgs(t, repo), "fmt") {
		t.Fatalf("cargo fmt must not run when disabled, got %q", readArgs(t, repo))
	}
}

func TestFormatRust_MissingRustfmtFailsClosed(t *testing.T) {
	setupFormatRustRepo(t, true, "")
	err := RunFormat(newRustFormatCmd(t, true, false, []string{"rust"}), nil)
	if !errors.Is(err, formatpkg.ErrToolUnavailable) {
		t.Fatalf("expected tool-unavailable, got %v", err)
	}
	if err := RunFormat(newRustFormatCmd(t, true, true, []string{"rust"}), nil); err != nil {
		t.Fatalf("--ignore-missing-tools must skip Rust, got %v", err)
	}
}

func TestFormatRust_TypesFilterExcludesRust(t *testing.T) {
	repo := setupFormatRustRepo(t, false, "")
	_ = RunFormat(newRustFormatCmd(t, true, false, []string{"yaml"}), nil)
	if strings.Contains(readArgs(t, repo), "fmt") {
		t.Fatalf("--types without rust must not run cargo fmt, got %q", readArgs(t, repo))
	}
}

func TestFormatRust_IgnoreMissingToolsDoesNotCoverConfiguredToolchain(t *testing.T) {
	setupFormatRustRepo(t, true, "format:\n  rust:\n    toolchain: nightly\n")
	err := RunFormat(newRustFormatCmd(t, true, true, []string{"rust"}), nil)
	if !errors.Is(err, formatpkg.ErrToolUnavailable) {
		t.Fatalf("configured toolchain without rustfmt must fail even with --ignore-missing-tools, got %v", err)
	}
}

func setFlag(t *testing.T, cmd *cobra.Command, name, value string) {
	t.Helper()
	if cmd.Flags().Lookup(name) == nil {
		cmd.Flags().String(name, "", "")
	}
	if err := cmd.Flags().Set(name, value); err != nil {
		t.Fatalf("set %s: %v", name, err)
	}
}

// Explicit and staged .rs selections go to cargo fmt only, never to the
// per-file formatter (which has no Rust support and would reject the file).
func TestFormatRust_ExplicitFilesRouteToCargo(t *testing.T) {
	for _, strategy := range []string{"sequential", "parallel"} {
		t.Run(strategy+"/check", func(t *testing.T) {
			repo := setupFormatRustRepo(t, false, "")
			cmd := newRustFormatCmd(t, true, false, nil)
			setFlag(t, cmd, "strategy", strategy)
			if err := cmd.Flags().Set("files", "src/lib.rs"); err != nil {
				t.Fatal(err)
			}
			err := RunFormat(cmd, nil)
			if !errors.Is(err, formatpkg.ErrFormatDrift) {
				t.Fatalf("expected cargo fmt drift for --files src/lib.rs, got %v", err)
			}
			if err.Error() != "1 files need formatting" {
				t.Fatalf("selected .rs must only report cargo fmt drift (no per-file formatter errors), got %v", err)
			}
			if !strings.Contains(readArgs(t, repo), "fmt --all -- --check -l") {
				t.Fatalf("expected cargo fmt check, got %q", readArgs(t, repo))
			}
		})
		t.Run(strategy+"/fix", func(t *testing.T) {
			repo := setupFormatRustRepo(t, false, "")
			cmd := newRustFormatCmd(t, false, false, nil)
			setFlag(t, cmd, "strategy", strategy)
			if err := cmd.Flags().Set("files", "src/lib.rs"); err != nil {
				t.Fatal(err)
			}
			if err := RunFormat(cmd, nil); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(readArgs(t, repo), "fmt --all\n") {
				t.Fatalf("expected cargo fmt --all, got %q", readArgs(t, repo))
			}
		})
	}
}

func TestFormatRust_StagedRustRoutesToCargo(t *testing.T) {
	repo := setupFormatRustRepo(t, false, "")
	orig := formatStagedFiles
	formatStagedFiles = func() ([]string, error) { return []string{"src/lib.rs"}, nil }
	t.Cleanup(func() { formatStagedFiles = orig })

	for _, types := range [][]string{nil, {"rust"}} {
		cmd := newRustFormatCmd(t, true, false, types)
		cmd.Flags().Bool("staged-only", true, "")
		err := RunFormat(cmd, nil)
		if err == nil || err.Error() != "1 files need formatting" {
			t.Fatalf("types=%v: expected only cargo fmt drift for staged .rs, got %v", types, err)
		}
	}
	if n := strings.Count(readArgs(t, repo), "fmt --all -- --check -l"); n != 2 {
		t.Fatalf("expected two cargo fmt checks, got %d: %q", n, readArgs(t, repo))
	}
}

// With Rust disabled (or excluded by --types), selected .rs files are skipped:
// never handed to the per-file formatter and never passed to cargo fmt.
func TestFormatRust_SelectedRustSkippedWhenNotInScope(t *testing.T) {
	cases := map[string]struct {
		yaml  string
		types []string
	}{
		"disabled":           {yaml: "format:\n  rust:\n    enabled: false\n"},
		"types without rust": {types: []string{"yaml"}},
	}
	for name, tc := range cases {
		for _, strategy := range []string{"sequential", "parallel"} {
			t.Run(name+"/explicit/"+strategy, func(t *testing.T) {
				repo := setupFormatRustRepo(t, false, tc.yaml)
				cmd := newRustFormatCmd(t, true, false, tc.types)
				setFlag(t, cmd, "strategy", strategy)
				if err := cmd.Flags().Set("files", "src/lib.rs"); err != nil {
					t.Fatal(err)
				}
				if err := RunFormat(cmd, nil); err != nil {
					t.Fatalf("selected .rs must be skipped cleanly, got %v", err)
				}
				if strings.Contains(readArgs(t, repo), "fmt") {
					t.Fatalf("cargo fmt must not run, got %q", readArgs(t, repo))
				}
			})
		}
		t.Run(name+"/staged", func(t *testing.T) {
			repo := setupFormatRustRepo(t, false, tc.yaml)
			orig := formatStagedFiles
			formatStagedFiles = func() ([]string, error) { return []string{"src/lib.rs"}, nil }
			t.Cleanup(func() { formatStagedFiles = orig })
			cmd := newRustFormatCmd(t, true, false, tc.types)
			cmd.Flags().Bool("staged-only", true, "")
			if err := RunFormat(cmd, nil); err != nil {
				t.Fatalf("staged .rs must be skipped cleanly, got %v", err)
			}
			if strings.Contains(readArgs(t, repo), "fmt") {
				t.Fatalf("cargo fmt must not run, got %q", readArgs(t, repo))
			}
		})
	}
}

func TestFormatRust_SelectedRustOutsideCargoSkipped(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile(filepath.Join(dir, "loose.rs"), []byte("fn main(){}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := newRustFormatCmd(t, true, false, nil)
	if err := cmd.Flags().Set("files", "loose.rs"); err != nil {
		t.Fatal(err)
	}
	if err := RunFormat(cmd, nil); err != nil {
		t.Fatalf(".rs outside a Cargo project must be skipped cleanly, got %v", err)
	}
}
