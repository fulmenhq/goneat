package assess

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

// This test executable doubles as a native, inert-by-default scanner fixture.
// It never invokes a real scanner, registry, package manager or network client.
func init() {
	mode := os.Getenv("GONEAT_SECURITY_PROCESS_FIXTURE")
	if !strings.HasPrefix(mode, "govuln-") && !strings.HasPrefix(mode, "gosec-") && !strings.HasPrefix(mode, "gitleaks-") && !strings.HasPrefix(mode, "audit-") && !strings.HasPrefix(mode, "deny-") {
		return
	}
	if os.Getenv("GONEAT_SECURITY_FIXTURE_ARGS") != "" {
		cwd, err := os.Getwd()
		if err != nil {
			os.Exit(99)
		}
		data, _ := json.Marshal(struct {
			PID  int      `json:"pid"`
			Args []string `json:"argv"`
			Cwd  string   `json:"cwd"`
		}{os.Getpid(), os.Args, cwd})
		if err := os.WriteFile(os.Getenv("GONEAT_SECURITY_FIXTURE_ARGS"), data, 0600); err != nil {
			os.Exit(99)
		}
	}
	if strings.HasPrefix(mode, "gosec-") {
		runGosecProcessFixture(mode)
		os.Exit(99)
	}
	if strings.HasPrefix(mode, "gitleaks-") {
		runGitleaksProcessFixture(mode)
		os.Exit(99)
	}
	if strings.HasPrefix(mode, "audit-") {
		runCargoAuditProcessFixture(mode)
		os.Exit(99)
	}
	if strings.HasPrefix(mode, "deny-") {
		runCargoDenyProcessFixture(mode)
		os.Exit(99)
	}
	fmt.Fprint(os.Stderr, "secret-bearing fixture stderr must not be logged")
	switch mode {
	case "govuln-empty":
		os.Exit(0)
	case "govuln-usage":
		os.Exit(2)
	case "govuln-clean":
		fmt.Println(govulnConfigFixture)
		os.Exit(0)
	case "govuln-load-error":
		fmt.Println(govulnConfigFixture)
		os.Exit(1)
	case "govuln-inventory", "govuln-inventory-exit1", "govuln-inventory-cancel", "govuln-imported-inventory":
		fmt.Println(govulnScopedConfigFixture)
		fmt.Println(govulnInventoryFixture)
		fmt.Println(govulnInertPackageWideAdvisory())
		fmt.Println(govulnModuleInventoryFixture)
		if mode == "govuln-imported-inventory" {
			fmt.Println(strings.Replace(govulnModuleInventoryFixture, `"version":"v0.56.0"`, `"version":"v0.56.0","package":"golang.org/x/crypto/openpgp"`, 1))
		}
		if mode == "govuln-inventory-exit1" {
			os.Exit(1)
		}
		if mode == "govuln-inventory-cancel" {
			securityFixtureReadyAndWait()
		}
		os.Exit(0)
	case "govuln-findings", "govuln-exit1", "govuln-exit3", "govuln-truncated", "govuln-cancel":
		fmt.Println(govulnConfigFixture)
		fmt.Println(govulnFindingFixture)
		if mode == "govuln-exit1" {
			os.Exit(1)
		}
		if mode == "govuln-exit3" {
			os.Exit(3)
		}
		if mode == "govuln-truncated" {
			fmt.Print(`{"progress":`)
			os.Exit(0)
		}
		if mode == "govuln-cancel" {
			// Publish readiness only after both JSON messages were written. The
			// parent cancels this process after readiness, not during startup.
			if err := os.WriteFile(os.Getenv("GONEAT_SECURITY_FIXTURE_READY"), []byte("ready"), 0600); err != nil {
				os.Exit(99)
			}
			time.Sleep(time.Minute)
		}
		os.Exit(0)
	default:
		os.Exit(99)
	}
}

func govulnInertPackageWideAdvisory() string {
	// Fixed official package identities, independent of the classifier's list.
	return `{"osv":{"id":"GO-2026-5932","affected":[{"package":{"name":"golang.org/x/crypto","ecosystem":"Go"},"ranges":[{"type":"SEMVER","events":[{"introduced":"0"}]}],"ecosystem_specific":{"imports":[{"path":"golang.org/x/crypto/openpgp"},{"path":"golang.org/x/crypto/openpgp/packet"},{"path":"golang.org/x/crypto/openpgp/armor"},{"path":"golang.org/x/crypto/openpgp/clearsign"},{"path":"golang.org/x/crypto/openpgp/errors"},{"path":"golang.org/x/crypto/openpgp/elgamal"},{"path":"golang.org/x/crypto/openpgp/s2k"}]}}]}}`
}

func TestGovulnMetadataFanInSurvivesFailure(t *testing.T) {
	installSecurityProcessFixture(t, "govulncheck")
	for _, tt := range []struct {
		mode       string
		failed     bool
		severities []IssueSeverity
	}{
		{"govuln-inventory", false, []IssueSeverity{SeverityInfo}},
		{"govuln-inventory-exit1", true, []IssueSeverity{SeverityHigh}},
		{"govuln-imported-inventory", false, []IssueSeverity{SeverityInfo, SeverityHigh}},
	} {
		t.Run(tt.mode, func(t *testing.T) {
			t.Setenv("GONEAT_SECURITY_PROCESS_FIXTURE", tt.mode)
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module fixture\n"), 0600); err != nil {
				t.Fatal(err)
			}
			cfg := DefaultAssessmentConfig()
			cfg.SecurityTools = []string{"govulncheck"}
			cfg.EnableCode, cfg.EnableSecrets = false, false
			cfg.Timeout = 10 * time.Second
			result, err := NewSecurityAssessmentRunner().Assess(t.Context(), root, cfg)
			if err != nil || result.Success == tt.failed || (result.Error != "") != tt.failed || result.SkipReason != "" || len(result.Issues) != len(tt.severities) {
				t.Fatalf("fan-in: %+v %v", result, err)
			}
			for i, want := range tt.severities {
				if result.Issues[i].Severity != want {
					t.Fatalf("grade lost: %+v", result.Issues)
				}
			}
			reports, ok := result.Metrics["tool_reports"].(map[string]map[string]interface{})
			if !ok || reports["govulncheck"]["execution_complete"] == tt.failed {
				t.Fatalf("completion missing: %+v", result.Metrics)
			}
			admissions := result.Metrics["tool_admissions"].([]securityToolAdmission)
			state := "completed_findings"
			if tt.failed {
				state = "failed_execution"
			}
			found := false
			for _, a := range admissions {
				if a.Tool == "govulncheck" {
					found = true
					if a.State != state {
						t.Fatalf("admission: %+v", a)
					}
				}
			}
			if !found {
				t.Fatal("missing admission")
			}
			data, e := json.Marshal(result)
			if e != nil || strings.Contains(string(data), "secret-bearing") || !strings.Contains(string(data), "GO-2026-5932") {
				t.Fatalf("metadata/privacy: %s %v", data, e)
			}
		})
	}
}

func TestGovulnCancellationAfterInventoryCannotPass(t *testing.T) {
	installSecurityProcessFixture(t, "govulncheck")
	t.Setenv("GONEAT_SECURITY_PROCESS_FIXTURE", "govuln-inventory-cancel")
	root := t.TempDir()
	ready := filepath.Join(root, "ready")
	t.Setenv("GONEAT_SECURITY_FIXTURE_READY", ready)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan bool, 1)
	go func() {
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		timer := time.NewTimer(9 * time.Second)
		defer timer.Stop()
		for {
			select {
			case <-timer.C:
				cancel()
				done <- false
				return
			case <-ticker.C:
				if _, err := os.Stat(ready); err == nil {
					cancel()
					done <- true
					return
				}
			case <-ctx.Done():
				done <- false
				return
			}
		}
	}()
	cfg := DefaultAssessmentConfig()
	cfg.Timeout = 10 * time.Second
	issues, metadata, err := NewSecurityAssessmentRunner().runGovulncheckWithMetadata(ctx, root, cfg)
	if !<-done || !errors.Is(err, context.Canceled) || len(issues) != 1 || issues[0].Severity != SeverityHigh || metadata["execution_complete"] != false {
		t.Fatalf("cancellation inventory: %+v %+v %v", issues, metadata, err)
	}
}

func TestGovulnExistingScopeArgvUnchanged(t *testing.T) {
	installSecurityProcessFixture(t, "govulncheck")
	t.Setenv("GONEAT_SECURITY_PROCESS_FIXTURE", "govuln-inventory")
	root := t.TempDir()
	receipt := filepath.Join(root, "child.json")
	t.Setenv("GONEAT_SECURITY_FIXTURE_ARGS", receipt)
	cfg := DefaultAssessmentConfig()
	cfg.Timeout = 10 * time.Second
	issues, _, err := NewSecurityAssessmentRunner().runGovulncheckWithMetadata(t.Context(), root, cfg)
	if err != nil || len(issues) != 1 || issues[0].Severity != SeverityInfo {
		t.Fatalf("inventory: %+v %v", issues, err)
	}
	b, e := os.ReadFile(receipt)
	if e != nil {
		t.Fatal(e)
	}
	var child struct {
		Args []string `json:"argv"`
		PID  int      `json:"pid"`
	}
	if json.Unmarshal(b, &child) != nil || child.PID <= 0 || !reflect.DeepEqual(child.Args[1:], []string{"-json", "./..."}) {
		t.Fatalf("production argv changed: %s", b)
	}
}

func TestGovulnRootScopeOmitsOnlyNonGoDirectories(t *testing.T) {
	installSecurityProcessFixture(t, "govulncheck")
	t.Setenv("GONEAT_SECURITY_PROCESS_FIXTURE", "govuln-clean")
	for _, tt := range []struct {
		name     string
		selected []string
		patterns []string
	}{
		{"mixed", []string{"Makefile", "docs/ops/guide.md", "cmd/source.go", "scripts/control.py"}, []string{"./...", "./cmd", "./scripts"}},
		{"late_root_preserves_order", []string{"docs/ops/guide.md", "cmd/source.go", "Makefile", "scripts/control.py"}, []string{"./cmd", "./...", "./scripts"}},
		{"metadata_only_retains_existing_anchor", []string{"Makefile", "docs/ops/guide.md"}, []string{"./..."}},
		{"without_root_unchanged", []string{"docs/ops/guide.md", "cmd/source.go"}, []string{"./docs/ops", "./cmd"}},
		{"directory_go_entry_conservative", []string{"Makefile", "odd/guide.md"}, []string{"./...", "./odd"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			for _, name := range []string{"docs/ops/guide.md", "cmd/source.go", "scripts/control.py", "scripts/control_test.go"} {
				path := filepath.Join(root, name)
				if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte("fixture"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.MkdirAll(filepath.Join(root, "odd", "entry.go"), 0700); err != nil {
				t.Fatal(err)
			}
			receipt := filepath.Join(root, "argv.json")
			t.Setenv("GONEAT_SECURITY_FIXTURE_ARGS", receipt)
			cfg := DefaultAssessmentConfig()
			cfg.Timeout = 10 * time.Second
			for _, name := range tt.selected {
				cfg.IncludeFiles = append(cfg.IncludeFiles, filepath.Join(root, name))
			}
			_, _, err := NewSecurityAssessmentRunner().runGovulncheckWithMetadata(t.Context(), root, cfg)
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(receipt)
			if err != nil {
				t.Fatal(err)
			}
			var child struct {
				Args []string `json:"argv"`
				PID  int      `json:"pid"`
			}
			if err := json.Unmarshal(data, &child); err != nil {
				t.Fatal(err)
			}
			want := append([]string{"-json"}, tt.patterns...)
			if child.PID <= 0 || !reflect.DeepEqual(child.Args[1:], want) {
				t.Fatalf("argv: got %v want %v", child.Args, want)
			}
		})
	}
}

func TestGovulnRootScopeRetainsSymlinkGoEntry(t *testing.T) {
	installSecurityProcessFixture(t, "govulncheck")
	t.Setenv("GONEAT_SECURITY_PROCESS_FIXTURE", "govuln-clean")
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "linked"), 0700); err != nil {
		t.Fatal(err)
	}
	// A dangling Go symlink must remain the Go loader's error, not become
	// successful absence evidence in the directory-pattern optimization.
	if err := os.Symlink("absent.go", filepath.Join(root, "linked", "source.go")); err != nil {
		t.Fatal(err)
	}
	receipt := filepath.Join(root, "argv.json")
	t.Setenv("GONEAT_SECURITY_FIXTURE_ARGS", receipt)
	cfg := DefaultAssessmentConfig()
	cfg.IncludeFiles = []string{filepath.Join(root, "Makefile"), filepath.Join(root, "linked", "readme.md")}
	_, _, err := NewSecurityAssessmentRunner().runGovulncheckWithMetadata(t.Context(), root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(receipt)
	if err != nil {
		t.Fatal(err)
	}
	var child struct {
		Args []string `json:"argv"`
	}
	if err := json.Unmarshal(data, &child); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(child.Args[1:], []string{"-json", "./...", "./linked"}) {
		t.Fatalf("symlink Go omitted: %v", child.Args)
	}
}

func TestGovulnRootScopeMappingAndReadErrorsFailClosed(t *testing.T) {
	installSecurityProcessFixture(t, "govulncheck")
	t.Setenv("GONEAT_SECURITY_PROCESS_FIXTURE", "govuln-clean")
	for _, name := range []string{"missing_directory", "all_mappings_before_reads", "canceled_before_read"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			receipt := filepath.Join(root, "argv.json")
			t.Setenv("GONEAT_SECURITY_FIXTURE_ARGS", receipt)
			cfg := DefaultAssessmentConfig()
			cfg.IncludeFiles = []string{filepath.Join(root, "Makefile"), filepath.Join(root, "missing", "readme.md")}
			ctx := t.Context()
			if name == "all_mappings_before_reads" {
				cfg.IncludeFiles = append(cfg.IncludeFiles, filepath.Join(t.TempDir(), "outside", "source.go"))
			}
			if name == "canceled_before_read" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			issues, metadata, err := NewSecurityAssessmentRunner().runGovulncheckWithMetadata(ctx, root, cfg)
			if err == nil || len(issues) != 0 || metadata != nil {
				t.Fatalf("scope error became success: %+v %+v %v", issues, metadata, err)
			}
			if name == "all_mappings_before_reads" && !strings.Contains(err.Error(), "outside module root") {
				t.Fatalf("mapping error hidden by read error: %v", err)
			}
			if name == "missing_directory" && !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("read error lost: %v", err)
			}
			if name == "canceled_before_read" && !errors.Is(err, context.Canceled) {
				t.Fatalf("context error lost: %v", err)
			}
			if _, statErr := os.Stat(receipt); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("child launched for invalid scope: %v", statErr)
			}
		})
	}
}

func installSecurityProcessFixture(t *testing.T, tool string) (string, string) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	name := tool
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(dir, name)
	if err := os.Link(executable, path); err != nil {
		source, err := os.Open(executable)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = source.Close() }()
		out, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
		if err != nil {
			t.Fatal(err)
		}
		_, copyErr := io.Copy(out, source)
		closeErr := out.Close()
		if copyErr != nil || closeErr != nil {
			t.Fatalf("fixture copy: %v / %v", copyErr, closeErr)
		}
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.New()
	_, hashErr := io.Copy(hash, file)
	closeErr := file.Close()
	if hashErr != nil || closeErr != nil {
		t.Fatalf("fixture hash: %v / %v", hashErr, closeErr)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return path, fmt.Sprintf("%x", hash.Sum(nil))
}

func TestGovulnProcessCompletion(t *testing.T) {
	path, digest := installSecurityProcessFixture(t, "govulncheck")
	root := t.TempDir()
	for _, tt := range []struct {
		mode     string
		findings int
		failure  bool
	}{
		{"govuln-clean", 0, false},
		{"govuln-findings", 1, false},
		{"govuln-load-error", 0, true},
		{"govuln-exit1", 1, true},
		{"govuln-exit3", 1, true},
		{"govuln-usage", 0, true},
		{"govuln-empty", 0, true},
		{"govuln-truncated", 1, true},
		{"govuln-cancel", 1, true},
	} {
		t.Run(tt.mode, func(t *testing.T) {
			t.Setenv("GONEAT_SECURITY_PROCESS_FIXTURE", tt.mode)
			receipt := filepath.Join(t.TempDir(), "child.json")
			t.Setenv("GONEAT_SECURITY_FIXTURE_ARGS", receipt)
			cfg := DefaultAssessmentConfig()
			cfg.Timeout = 10 * time.Second
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var readyDone chan bool
			if tt.mode == "govuln-cancel" {
				ready := filepath.Join(t.TempDir(), "ready")
				t.Setenv("GONEAT_SECURITY_FIXTURE_READY", ready)
				readyDone = make(chan bool, 1)
				go func() {
					ticker := time.NewTicker(10 * time.Millisecond)
					defer ticker.Stop()
					deadline := time.NewTimer(9 * time.Second)
					defer deadline.Stop()
					for {
						select {
						case <-ctx.Done():
							readyDone <- false
							return
						case <-deadline.C:
							cancel()
							readyDone <- false
							return
						case <-ticker.C:
							if data, err := os.ReadFile(ready); err == nil && string(data) == "ready" {
								cancel()
								readyDone <- true
								return
							}
						}
					}
				}()
			}
			issues, err := NewSecurityAssessmentRunner().runGovulncheck(ctx, root, cfg)
			cancel()
			if readyDone != nil && !<-readyDone {
				t.Fatal("fixture never reached post-report readiness before cancellation")
			}
			if (err != nil) != tt.failure || len(issues) != tt.findings {
				t.Fatalf("wrong native fixture outcome: issues=%+v err=%v", issues, err)
			}
			if err != nil && strings.Contains(err.Error(), "secret-bearing") {
				t.Fatalf("raw stderr leaked: %v", err)
			}
			data, err := os.ReadFile(receipt)
			if err != nil {
				t.Fatal(err)
			}
			var child struct {
				PID  int      `json:"pid"`
				Args []string `json:"argv"`
			}
			if json.Unmarshal(data, &child) != nil || child.PID <= 0 || len(child.Args) < 2 || child.Args[1] != "-json" {
				t.Fatalf("wrong invocation: %s", data)
			}
			t.Logf("native fixture path=%s pathname_sha256=%s pid=%d argv=%q; synthetic scanner, not live tool or immutable process-image proof", path, digest, child.PID, child.Args)
		})
	}
}

func TestGovulnScopedPackageArguments(t *testing.T) {
	fixturePath, digest := installSecurityProcessFixture(t, "govulncheck")
	t.Setenv("GONEAT_SECURITY_PROCESS_FIXTURE", "govuln-clean")
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(base, "module")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	staged := []string{
		"cmd/security_failclosed_test.go",
		"internal/assess/lint_availability_test.go",
		"internal/assess/rust_cargo_audit.go",
		"internal/assess/rust_cargo_deny.go",
		"internal/assess/rust_clippy_failclosed_test.go",
		"internal/assess/security_cargo_audit.go",
		"internal/assess/security_cargo_audit_test.go",
		"internal/assess/security_cargo_deny_test.go",
		"internal/assess/security_failclosed_test.go",
		"internal/assess/security_gitleaks.go",
		"internal/assess/security_gitleaks_test.go",
		"internal/assess/security_gosec.go",
		"internal/assess/security_gosec_test.go",
		"internal/assess/security_govulncheck.go",
		"internal/assess/security_govulncheck_test.go",
		"internal/assess/security_parsing_test.go",
		"internal/assess/security_process_test.go",
		"internal/assess/security_registry.go",
		"internal/assess/security_report_json.go",
		"internal/assess/security_runner.go",
		"internal/assess/security_runner_test.go",
		"internal/assess/tool_failclosed_test.go",
		"pkg/dependencies/cargo_deny_security.go",
		"pkg/dependencies/cargo_deny_security_test.go",
	}
	stagedArgs := []string{"-json", "./cmd", "./internal/assess", "./pkg/dependencies"}
	absoluteStaged := make([]string, len(staged))
	for i, name := range staged {
		absoluteStaged[i] = filepath.Join(root, filepath.FromSlash(name))
	}
	for _, tt := range []struct {
		name       string
		cwd        string
		moduleRoot string
		files      []string
		argv       []string
	}{
		{"relative-root-staged", root, ".", staged, stagedArgs},
		{"absolute-root-staged", root, root, staged, stagedArgs},
		{"relative-root-absolute-files", root, ".", absoluteStaged, stagedArgs},
		{"absolute-root-absolute-files", root, root, absoluteStaged, stagedArgs},
		{"parent-cwd-relative-module", base, "module", []string{filepath.Join("module", "cmd", "a.go")}, []string{"-json", "./cmd"}},
		{"off-cwd-absolute-module-root-file", base, root, []string{"a.go"}, []string{"-json", "./..."}},
		{"parent-cwd-relative-module-root-file", base, "module", []string{"a.go"}, []string{"-json", "./..."}},
		{"off-cwd-cleaned-module-root-file", base, root, []string{"." + string(filepath.Separator) + "a.go"}, []string{"-json", "./..."}},
		{"relative-root-file", root, ".", []string{"a.go"}, []string{"-json", "./..."}},
		{"absolute-root-file", root, root, []string{filepath.Join(root, "a.go")}, []string{"-json", "./..."}},
		{"unscoped-relative", root, ".", nil, []string{"-json", "./..."}},
		{"unscoped-absolute", root, root, nil, []string{"-json", "./..."}},
		{"nested-exact-package", root, ".", []string{filepath.Join("selected", "nested", "a.go")}, []string{"-json", "./selected/nested"}},
		{"first-seen-order-dedup", root, ".", []string{filepath.Join("pkg", "z", "a.go"), filepath.Join("cmd", "a.go"), filepath.Join("pkg", "z", "b.go")}, []string{"-json", "./pkg/z", "./cmd"}},
		{"parent-like-contained-name", root, ".", []string{filepath.Join("..data", "a.go")}, []string{"-json", "./..data"}},
		{"directory-with-space", root, ".", []string{filepath.Join("selected dir", "a.go")}, []string{"-json", "./selected dir"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(tt.cwd)
			receipt := filepath.Join(t.TempDir(), "child.json")
			t.Setenv("GONEAT_SECURITY_FIXTURE_ARGS", receipt)
			cfg := DefaultAssessmentConfig()
			cfg.IncludeFiles = tt.files
			cfg.Timeout = 10 * time.Second
			issues, err := NewSecurityAssessmentRunner().runGovulncheck(t.Context(), tt.moduleRoot, cfg)
			if err != nil || len(issues) != 0 {
				t.Fatalf("synthetic scoped scan failed: issues=%+v err=%v", issues, err)
			}
			data, err := os.ReadFile(receipt)
			if err != nil {
				t.Fatal(err)
			}
			var child struct {
				PID  int      `json:"pid"`
				Args []string `json:"argv"`
				Cwd  string   `json:"cwd"`
			}
			if json.Unmarshal(data, &child) != nil || child.PID <= 0 || len(child.Args) == 0 || !reflect.DeepEqual(child.Args[1:], tt.argv) || child.Cwd != root {
				t.Fatalf("wrong local package invocation: got=%s want_argv=%q want_cwd=%q", data, tt.argv, root)
			}
			t.Logf("synthetic scanner path=%s pathname_sha256=%s pid=%d argv=%q cwd=%q; no live scanner or immutable process-image proof", fixturePath, digest, child.PID, child.Args, child.Cwd)
		})
	}
}

func TestGovulnScopedOutsidePathFailsBeforeLaunch(t *testing.T) {
	installSecurityProcessFixture(t, "govulncheck")
	t.Setenv("GONEAT_SECURITY_PROCESS_FIXTURE", "govuln-clean")
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(base, "module")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	for _, outside := range []string{filepath.Join("..", "a.go"), filepath.Join("..", "outside", "a.go"), filepath.Join(base, "module-sibling", "a.go")} {
		t.Run(outside, func(t *testing.T) {
			for _, moduleRoot := range []string{".", root} {
				for _, includes := range [][]string{
					{filepath.Join("cmd", "a.go"), outside},
					{"a.go", outside},
					{outside, "a.go"},
				} {
					receipt := filepath.Join(t.TempDir(), "child.json")
					t.Setenv("GONEAT_SECURITY_FIXTURE_ARGS", receipt)
					cfg := DefaultAssessmentConfig()
					cfg.IncludeFiles = includes
					issues, err := NewSecurityAssessmentRunner().runGovulncheck(t.Context(), moduleRoot, cfg)
					if err == nil || !strings.Contains(err.Error(), "outside module root") || len(issues) != 0 {
						t.Fatalf("scope escape must fail, not scan valid remainder: issues=%+v err=%v", issues, err)
					}
					if _, err := os.Stat(receipt); !os.IsNotExist(err) {
						t.Fatalf("outside path launched a child or fallback: %v", err)
					}
				}
			}
		})
	}
}

func TestGovulnScopedNonzeroRemainsCategoryFailure(t *testing.T) {
	installSecurityProcessFixture(t, "govulncheck")
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module fixture\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	for _, mode := range []string{"govuln-load-error", "govuln-exit1"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("GONEAT_SECURITY_PROCESS_FIXTURE", mode)
			receipt := filepath.Join(t.TempDir(), "child.json")
			t.Setenv("GONEAT_SECURITY_FIXTURE_ARGS", receipt)
			cfg := DefaultAssessmentConfig()
			cfg.IncludeFiles = []string{filepath.Join("cmd", "a.go")}
			cfg.SecurityTools = []string{"govulncheck"}
			cfg.EnableCode, cfg.EnableSecrets = false, false
			cfg.Timeout = 10 * time.Second
			result, err := NewSecurityAssessmentRunner().Assess(t.Context(), ".", cfg)
			findings := 0
			if mode == "govuln-exit1" {
				findings = 1
			}
			if err != nil || result.Success || !strings.Contains(result.Error, "exit status 1") || result.SkipReason != "" || len(result.Issues) != findings || strings.Contains(result.Error, "secret-bearing") {
				t.Fatalf("corrected local argv cannot clear scanner exit 1: result=%+v err=%v", result, err)
			}
			data, err := os.ReadFile(receipt)
			if err != nil {
				t.Fatal(err)
			}
			var child struct {
				PID  int      `json:"pid"`
				Args []string `json:"argv"`
				Cwd  string   `json:"cwd"`
			}
			if json.Unmarshal(data, &child) != nil || child.PID <= 0 || len(child.Args) == 0 || !reflect.DeepEqual(child.Args[1:], []string{"-json", "./cmd"}) || child.Cwd != root {
				t.Fatalf("wrong failed scoped invocation: %s", data)
			}
		})
	}
}

func TestGovulnProcessDeadlineBeforeStart(t *testing.T) {
	installSecurityProcessFixture(t, "govulncheck")
	t.Setenv("GONEAT_SECURITY_PROCESS_FIXTURE", "govuln-clean")
	cfg := DefaultAssessmentConfig()
	cfg.SecurityGovulncheckTimeout = time.Nanosecond
	issues, err := NewSecurityAssessmentRunner().runGovulncheck(context.Background(), t.TempDir(), cfg)
	if len(issues) != 0 || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("deadline must fail without inventing findings: issues=%+v err=%v", issues, err)
	}
}

func securityFixtureReadyAndWait() {
	if err := os.WriteFile(os.Getenv("GONEAT_SECURITY_FIXTURE_READY"), []byte("ready"), 0600); err != nil {
		os.Exit(99)
	}
	time.Sleep(time.Minute)
}

func TestSecurityProcessCancellationRetainsEvidence(t *testing.T) {
	for _, tool := range []string{"gosec", "govulncheck", "gitleaks", "cargo-audit", "cargo-deny"} {
		t.Run(tool, func(t *testing.T) {
			name := tool
			mode := tool + "-cancel"
			if tool == "govulncheck" {
				mode = "govuln-cancel"
			}
			if tool == "cargo-audit" {
				name, mode = "cargo", "audit-cancel"
			}
			if tool == "cargo-deny" {
				name, mode = "cargo", "deny-cancel"
			}
			path, digest := installSecurityProcessFixture(t, name)
			t.Setenv("GONEAT_SECURITY_PROCESS_FIXTURE", mode)
			root := t.TempDir()
			if name == "cargo" {
				if err := os.WriteFile(filepath.Join(root, "Cargo.toml"), []byte("[package]\nname=\"fixture\"\nversion=\"1.0.0\"\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			ready := filepath.Join(root, "ready")
			receipt := filepath.Join(root, "child.json")
			t.Setenv("GONEAT_SECURITY_FIXTURE_ARGS", receipt)
			t.Setenv("GONEAT_SECURITY_FIXTURE_READY", ready)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			done := make(chan bool, 1)
			go func() {
				ticker := time.NewTicker(10 * time.Millisecond)
				defer ticker.Stop()
				deadline := time.NewTimer(15 * time.Second)
				defer deadline.Stop()
				for {
					select {
					case <-ctx.Done():
						done <- false
						return
					case <-deadline.C:
						cancel()
						done <- false
						return
					case <-ticker.C:
						if data, err := os.ReadFile(ready); err == nil && string(data) == "ready" {
							cancel()
							done <- true
							return
						}
					}
				}
			}()
			cfg := DefaultAssessmentConfig()
			cfg.Timeout = 20 * time.Second
			cfg.IncludeFiles = []string{"a.go"}
			cfg.TrackSuppressions = true
			runner := NewSecurityAssessmentRunner()
			var issues []Issue
			var err error
			switch tool {
			case "gosec":
				var suppressions []Suppression
				issues, suppressions, err = runner.runGosec(ctx, root, cfg)
				if len(suppressions) != 1 {
					t.Fatalf("canceled shard suppression lost: %+v err=%v", suppressions, err)
				}
			case "govulncheck":
				issues, err = runner.runGovulncheck(ctx, root, cfg)
			case "gitleaks":
				issues, err = runner.runGitleaks(ctx, root, cfg)
			case "cargo-audit":
				var warnings map[string][]json.RawMessage
				issues, warnings, err = (&cargoAuditAdapter{moduleRoot: root, cfg: cfg}).RunWithWarnings(ctx)
				if len(warnings["unmaintained"]) != 1 {
					t.Fatalf("canceled audit warning lost: %+v err=%v", warnings, err)
				}
			case "cargo-deny":
				var metadata map[string]interface{}
				issues, metadata, err = (&cargoDenyAdapter{moduleRoot: root, cfg: cfg}).RunWithMetadata(ctx)
				if metadata["complete"] != false {
					t.Fatalf("canceled deny must not complete: %+v err=%v", metadata, err)
				}
			}
			cancel()
			if !<-done {
				t.Fatal("fixture did not reach post-report readiness")
			}
			if len(issues) != 1 || !errors.Is(err, context.Canceled) {
				t.Fatalf("canceled execution must retain prior finding and context error: %+v err=%v", issues, err)
			}
			data, readErr := os.ReadFile(receipt)
			if readErr != nil {
				t.Fatal(readErr)
			}
			var child struct {
				PID  int      `json:"pid"`
				Args []string `json:"argv"`
			}
			if json.Unmarshal(data, &child) != nil || child.PID <= 0 || len(child.Args) < 2 {
				t.Fatalf("missing native receipt: %s", data)
			}
			t.Logf("post-report cancellation native synthetic fixture path=%s pathname_sha256=%s pid=%d argv=%q; not live tool or immutable process-image proof", path, digest, child.PID, child.Args)
		})
	}
}

func TestSecurityProcessDeadlineIsFailure(t *testing.T) {
	for _, tool := range []string{"gosec", "gitleaks", "cargo-audit", "cargo-deny"} {
		t.Run(tool, func(t *testing.T) {
			name, mode := tool, tool+"-clean"
			if tool == "cargo-audit" {
				name, mode = "cargo", "audit-clean"
			}
			if tool == "cargo-deny" {
				name, mode = "cargo", "deny-clean"
			}
			installSecurityProcessFixture(t, name)
			t.Setenv("GONEAT_SECURITY_PROCESS_FIXTURE", mode)
			root := t.TempDir()
			if name == "cargo" {
				if err := os.WriteFile(filepath.Join(root, "Cargo.toml"), []byte("[package]\nname=\"fixture\"\nversion=\"1.0.0\"\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			cfg := DefaultAssessmentConfig()
			cfg.Timeout = time.Nanosecond
			cfg.IncludeFiles = []string{"a.go"}
			runner := NewSecurityAssessmentRunner()
			var issues []Issue
			var err error
			switch tool {
			case "gosec":
				issues, _, err = runner.runGosec(t.Context(), root, cfg)
			case "gitleaks":
				issues, err = runner.runGitleaks(t.Context(), root, cfg)
			case "cargo-audit":
				issues, err = (&cargoAuditAdapter{moduleRoot: root, cfg: cfg}).Run(t.Context())
			case "cargo-deny":
				issues, err = (&cargoDenyAdapter{moduleRoot: root, cfg: cfg}).Run(t.Context())
			}
			if len(issues) != 0 || !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("deadline before report must fail, not clean or skip: %+v err=%v", issues, err)
			}
		})
	}
}
