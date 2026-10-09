package assess

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const cargoDenyDiagnosticFixture = `{"type":"diagnostic","fields":{"severity":"error","message":"fixture advisory","code":"vulnerability","advisory":{"id":"RUSTSEC-2026-0001","url":"https://example.org/advisory"}}}`

func cargoDenySummaryFixture(advisories, sources int) string {
	return fmt.Sprintf(`{"type":"summary","fields":{"advisories":{"errors":%d,"warnings":0,"notes":0,"helps":0},"sources":{"errors":%d,"warnings":0,"notes":0,"helps":0},"future":{"number":9007199254740993}}}`, advisories, sources)
}

func runCargoDenyProcessFixture(mode string) {
	if len(os.Args) == 3 && os.Args[1] == "deny" && os.Args[2] == "--version" {
		version := os.Getenv("GONEAT_SECURITY_FIXTURE_VERSION")
		if version == "" {
			version = "0.20.2"
		}
		fmt.Print("cargo-deny " + version)
		os.Exit(0)
	}
	if len(os.Args) != 7 || strings.Join(os.Args[1:], " ") != "deny --format json check advisories sources" {
		os.Exit(99)
	}
	summary := cargoDenySummaryFixture(0, 0)
	exit := 0
	diagnostic := ""
	switch mode {
	case "deny-clean":
	case "deny-advisories":
		summary, exit, diagnostic = cargoDenySummaryFixture(1, 0), 1, cargoDenyDiagnosticFixture
	case "deny-cancel":
		summary, exit, diagnostic = cargoDenySummaryFixture(1, 0), 1, cargoDenyDiagnosticFixture
	case "deny-sources":
		summary, exit = cargoDenySummaryFixture(0, 1), 8
	case "deny-both":
		summary, exit, diagnostic = cargoDenySummaryFixture(1, 1), 9, cargoDenyDiagnosticFixture
	case "deny-legacy-sources":
		summary, exit = cargoDenySummaryFixture(0, 1), 1
	case "deny-legacy-both":
		summary, exit, diagnostic = cargoDenySummaryFixture(1, 1), 1, cargoDenyDiagnosticFixture
	case "deny-unaccounted":
		exit = 1
	case "deny-modern-wrong-source":
		summary, exit = cargoDenySummaryFixture(0, 1), 1
	case "deny-clean-mismatch":
		summary = cargoDenySummaryFixture(1, 0)
		diagnostic = cargoDenyDiagnosticFixture
	case "deny-partial":
		summary = `{"type":"summary","fields":{"advisories":{"errors":1,"warnings":0,"notes":0,"helps":0}}}`
		exit = 1
		diagnostic = cargoDenyDiagnosticFixture
	case "deny-missing-advisories":
		summary = `{"type":"summary","fields":{"sources":{"errors":1,"warnings":0,"notes":0,"helps":0}}}`
		exit = 8
	case "deny-unrelated-bit":
		summary, exit, diagnostic = cargoDenySummaryFixture(1, 0), 3, cargoDenyDiagnosticFixture
	case "deny-truncated":
		summary = `{"type":"summary","fields":`
		exit = 1
		diagnostic = cargoDenyDiagnosticFixture
	case "deny-offline":
		summary = ""
		exit = 1
	default:
		os.Exit(99)
	}
	if diagnostic != "" {
		fmt.Fprintln(os.Stderr, diagnostic)
	}
	fmt.Fprint(os.Stderr, summary)
	if mode == "deny-cancel" {
		securityFixtureReadyAndWait()
	}
	os.Exit(exit)
}

func TestCargoDenySecurityProcessCompletion(t *testing.T) {
	path, digest := installSecurityProcessFixture(t, "cargo")
	for _, tt := range []struct {
		mode, version  string
		findings, exit int
		failure        bool
	}{
		{"deny-clean", "0.20.2", 0, 0, false},
		{"deny-advisories", "0.20.2", 1, 1, false},
		{"deny-sources", "0.20.2", 0, 8, false},
		{"deny-both", "0.20.2", 1, 9, false},
		{"deny-legacy-sources", "0.14.0", 0, 1, false},
		{"deny-legacy-both", "0.14.0", 1, 1, false},
		{"deny-unaccounted", "0.14.0", 0, 1, true},
		{"deny-clean-mismatch", "0.14.0", 1, 0, true},
		{"deny-modern-wrong-source", "0.20.2", 0, 1, true},
		{"deny-partial", "0.14.0", 1, 1, true},
		{"deny-partial", "0.20.2", 1, 1, true},
		{"deny-missing-advisories", "0.20.2", 0, 8, true},
		{"deny-unrelated-bit", "0.20.2", 1, 3, true},
		{"deny-truncated", "0.20.2", 1, 1, true},
		{"deny-offline", "0.20.2", 0, 1, true},
	} {
		t.Run(tt.mode+"-"+tt.version, func(t *testing.T) {
			t.Setenv("GONEAT_SECURITY_PROCESS_FIXTURE", tt.mode)
			t.Setenv("GONEAT_SECURITY_FIXTURE_VERSION", tt.version)
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "Cargo.toml"), []byte("[package]\nname=\"fixture\"\nversion=\"1.0.0\"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			receipt := filepath.Join(root, "child.json")
			t.Setenv("GONEAT_SECURITY_FIXTURE_ARGS", receipt)
			cfg := DefaultAssessmentConfig()
			cfg.Timeout = 10 * time.Second
			issues, metadata, err := (&cargoDenyAdapter{moduleRoot: root, cfg: cfg}).RunWithMetadata(t.Context())
			if len(issues) != tt.findings || (err != nil) != tt.failure || metadata == nil || metadata["complete"] == tt.failure || metadata["exit_code"] != tt.exit || metadata["version"] != tt.version {
				t.Fatalf("wrong strict adapter outcome: issues=%+v metadata=%+v err=%v", issues, metadata, err)
			}
			data, err := os.ReadFile(receipt)
			if err != nil {
				t.Fatal(err)
			}
			var child struct {
				PID  int      `json:"pid"`
				Args []string `json:"argv"`
			}
			if json.Unmarshal(data, &child) != nil || child.PID <= 0 || strings.Join(child.Args[1:], " ") != "deny --format json check advisories sources" {
				t.Fatalf("check invocation changed: %s", data)
			}
			t.Logf("synthetic native fixture path=%s pathname_sha256=%s pid=%d argv=%q; not live tool or immutable process-image proof", path, digest, child.PID, child.Args)
		})
	}
}

type securityMetadataFixtureTool struct {
	*securityFixtureTool
	metadata map[string]interface{}
}

func (f *securityMetadataFixtureTool) RunWithMetadata(ctx context.Context) ([]Issue, map[string]interface{}, error) {
	return f.issues, f.metadata, f.err
}

func TestSecuritySummaryEvidenceIsNotInventedFinding(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(fmt.Sprint(failed), func(t *testing.T) {
			fixture := &securityMetadataFixtureTool{securityFixtureTool: &securityFixtureTool{name: "fixture", applicable: true, available: true}, metadata: map[string]interface{}{"complete": !failed, "policy_findings_reported": !failed, "summary": json.RawMessage(cargoDenySummaryFixture(0, 1))}}
			if failed {
				fixture.err = fmt.Errorf("summary/exit mismatch")
			}
			old := securityRegistry
			securityRegistry = &SecurityToolRegistry{}
			t.Cleanup(func() { securityRegistry = old })
			RegisterSecurityTool("fixture", "vuln", func(*SecurityAssessmentRunner, string, AssessmentConfig) SecurityTool { return fixture })
			result, err := NewSecurityAssessmentRunner().Assess(t.Context(), t.TempDir(), DefaultAssessmentConfig())
			if err != nil || result.Success == failed || result.SkipReason != "" || len(result.Issues) != 0 {
				t.Fatalf("summary-only result invented advisory or hid error: %+v err=%v", result, err)
			}
			state := result.Metrics["tool_admissions"].([]securityToolAdmission)[0].State
			if (!failed && state != "completed_findings") || (failed && state != "failed_execution") {
				t.Fatalf("completion state wrong: %s", state)
			}
			encoded, err := json.Marshal(result)
			if err != nil || !strings.Contains(string(encoded), "9007199254740993") {
				t.Fatalf("summary unknown/numeric fields lost: %s err=%v", encoded, err)
			}
		})
	}
}
