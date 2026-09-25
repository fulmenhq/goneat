package assess

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const (
	biomeOrdinary = `{"summary":{"errors":1},"diagnostics":[{"category":"lint/suspicious/noDebugger","severity":"error","message":"Unexpected debugger","location":{"path":"a.ts"}}]}`
	biomeFormat   = `{"summary":{"errors":1},"diagnostics":[{"category":"format","severity":"error","message":"File content differs","location":{"path":"a.ts"}}]}`
	biomeConfig   = `{"summary":{"errors":1},"diagnostics":[{"category":"configuration","severity":"error","message":"Unknown key","location":{"path":"biome.json"}}]}`
	biomeInternal = `{"summary":{"errors":1},"diagnostics":[{"category":"internalError/fs","severity":"error","message":"Cannot read file","location":{"path":"a.ts"}}]}`
	biomeEmpty    = `{"summary":{"errors":0},"diagnostics":[]}`
)

// Verbatim biome 2.4 stderr when every selected path is ignored by biome.json.
const biomeAllIgnoredStderr = "lint\n  × No files were processed in the specified paths.\n  i Check your biome.json or biome.jsonc to ensure the paths are not ignored by the configuration.\n  i These paths were provided but ignored:\n  - gen/skip.ts\n"

// setupFakeBiome writes a.ts and biome.json plus a fake biome that prints
// stdout, writes "biome: failure" to stderr when exit != 0, and exits.
func setupFakeBiome(t *testing.T, stdout string, exit int) string {
	t.Helper()
	return setupFakeBiomeStderr(t, stdout, exit, "biome: failure\n")
}

func setupFakeBiomeStderr(t *testing.T, stdout string, exit int, stderr string) string {
	t.Helper()
	repo := t.TempDir()
	for name, body := range map[string]string{"a.ts": "debugger;\n", "biome.json": "{}\n"} {
		if err := os.WriteFile(filepath.Join(repo, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	binDir := filepath.Join(repo, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(repo, "biome-stdout.json")
	if err := os.WriteFile(out, []byte(stdout), 0o644); err != nil {
		t.Fatal(err)
	}
	errFile := filepath.Join(repo, "biome-stderr.txt")
	if err := os.WriteFile(errFile, []byte(stderr), 0o644); err != nil {
		t.Fatal(err)
	}
	script := "#!/usr/bin/env bash\ncat \"" + out + "\"\n" +
		"if [[ " + strconv.Itoa(exit) + " != 0 ]]; then cat \"" + errFile + "\" >&2; fi\n" +
		"exit " + strconv.Itoa(exit) + "\n"
	// #nosec G306 -- test fake needs to be executable
	if err := os.WriteFile(filepath.Join(binDir, "biome"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return repo
}

func TestBiome_FailClosed(t *testing.T) {
	type call func(repo string) ([]Issue, error)
	lint := func(repo string) ([]Issue, error) { return runBiomeLint(repo, checkCfg(), []string{"a.ts"}) }
	lintIncremental := func(repo string) ([]Issue, error) {
		cfg := checkCfg()
		cfg.NewIssuesOnly = true
		return runBiomeLint(repo, cfg, []string{"a.ts"})
	}
	format := func(repo string) ([]Issue, error) { return runBiomeFormat(repo, checkCfg(), []string{"a.ts"}) }
	configCheck := func(repo string) ([]Issue, error) { return runBiomeConfigCheck(repo, checkCfg()) }

	paths := map[string]struct {
		fn       call
		ordinary string
	}{
		"lint":             {lint, biomeOrdinary},
		"lint-incremental": {lintIncremental, biomeOrdinary},
		"format-check":     {format, biomeFormat},
		"config-check":     {configCheck, biomeConfig},
	}
	for name, p := range paths {
		t.Run(name+"/nonzero with ordinary findings reports issues", func(t *testing.T) {
			issues, err := p.fn(setupFakeBiome(t, p.ordinary, 1))
			if err != nil || len(issues) != 1 {
				t.Fatalf("expected 1 issue and no error, got %d issues, err=%v", len(issues), err)
			}
		})
		t.Run(name+"/nonzero with internalError fails", func(t *testing.T) {
			_, err := p.fn(setupFakeBiome(t, biomeInternal, 1))
			if err == nil || !strings.Contains(err.Error(), "internalError/fs") {
				t.Fatalf("expected internalError failure, got %v", err)
			}
		})
		t.Run(name+"/nonzero with empty report fails", func(t *testing.T) {
			_, err := p.fn(setupFakeBiome(t, biomeEmpty, 1))
			if err == nil || !strings.Contains(err.Error(), "without completing") {
				t.Fatalf("expected did-not-complete failure, got %v", err)
			}
		})
		t.Run(name+"/nonzero with no output fails", func(t *testing.T) {
			if _, err := p.fn(setupFakeBiome(t, "", 1)); err == nil {
				t.Fatalf("expected failure for non-zero exit without a report")
			}
		})
		t.Run(name+"/exit 0 internalError fails", func(t *testing.T) {
			// biome exits 0 when one of several files is unreadable; that file was not checked.
			_, err := p.fn(setupFakeBiome(t, biomeInternal, 0))
			if err == nil || !strings.Contains(err.Error(), "could not check every file") {
				t.Fatalf("expected internalError failure at exit 0, got %v", err)
			}
		})
		t.Run(name+"/all selected paths ignored by biome config is a no-op", func(t *testing.T) {
			repo := setupFakeBiomeStderr(t, biomeEmpty, 1, biomeAllIgnoredStderr)
			issues, err := p.fn(repo)
			if err != nil || len(issues) != 0 {
				t.Fatalf("expected clean no-op, got %d issues, err=%v", len(issues), err)
			}
		})
	}

	t.Run("no files processed without the ignored marker fails", func(t *testing.T) {
		repo := setupFakeBiomeStderr(t, biomeEmpty, 1, "  × No files were processed in the specified paths.\n")
		if _, err := runBiomeLint(repo, checkCfg(), []string{"a.ts"}); err == nil {
			t.Fatalf("expected failure when biome processed nothing for another reason")
		}
	})

	t.Run("format --write nonzero fails", func(t *testing.T) {
		cfg := checkCfg()
		cfg.Mode = AssessmentModeFix
		if _, err := runBiomeFormat(setupFakeBiome(t, "", 1), cfg, []string{"a.ts"}); err == nil {
			t.Fatalf("expected biome format --write failure")
		}
	})
}

func TestCargoAudit_ReportShape(t *testing.T) {
	advisory := `{"vulnerabilities":{"found":true,"count":1,"list":[{"advisory":{"id":"RUSTSEC-2026-0001","title":"bad","severity":"high"},"package":{"name":"x","version":"1.0.0"}}]}}`
	cases := []struct {
		name    string
		stdout  string
		exit    int
		wantErr string
		want    int
	}{
		{name: "clean report", stdout: `{"vulnerabilities":{"found":false,"count":0,"list":[]}}`, want: 0},
		{name: "advisories with exit 1", stdout: advisory, exit: 1, want: 1},
		{name: "empty object with exit 1", stdout: `{}`, exit: 1, wantErr: "without a vulnerability report"},
		{name: "empty object with exit 0", stdout: `{}`, wantErr: "without a vulnerability report"},
		{name: "no advisories with exit 1", stdout: `{"vulnerabilities":{"found":false,"list":[]}}`, exit: 1, wantErr: "no advisories"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := t.TempDir()
			if err := os.WriteFile(filepath.Join(repo, "Cargo.toml"), []byte("[package]\nname = \"demo\"\nversion = \"0.1.0\"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			out := filepath.Join(repo, "audit.json")
			if err := os.WriteFile(out, []byte(tc.stdout), 0o644); err != nil {
				t.Fatal(err)
			}
			writeFakeCargo(t, repo, "#!/usr/bin/env bash\n"+
				"if [[ \"$1\" == \"audit\" && \"$2\" == \"--json\" ]]; then cat \""+out+"\"; echo 'audit stderr' >&2; exit "+strconv.Itoa(tc.exit)+"; fi\n"+
				"exit 0\n")
			issues, err := (&cargoAuditAdapter{moduleRoot: repo, cfg: checkCfg()}).Run(t.Context())
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil || len(issues) != tc.want {
				t.Fatalf("expected %d issues, got %d, err=%v", tc.want, len(issues), err)
			}
		})
	}
}
