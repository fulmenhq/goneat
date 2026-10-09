package assess

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const cargoAuditHeaderFixture = `"database":{"advisory-count":100,"last-commit":null,"last-updated":null},"lockfile":{"dependency-count":2},"settings":{"target_arch":null,"target_os":null,"severity":null,"ignore":[],"informational_warnings":[]}`
const cargoAuditVulnerabilityFixture = `{"advisory":{"id":"RUSTSEC-2026-0001","title":"fixture advisory","severity":"high","url":"https://example.org/advisory"},"package":{"name":"fixture","version":"1.0.0"}}`
const cargoAuditWarningFixture = `{"kind":"unmaintained","package":{"name":"fixture","version":"1.0.0"},"advisory":null,"affected":null,"versions":null,"future":9007199254740993}`
const cargoAuditCleanFixture = `{` + cargoAuditHeaderFixture + `,"vulnerabilities":{"found":false,"count":0,"list":[]},"warnings":{}}`
const cargoAuditFindingFixture = `{` + cargoAuditHeaderFixture + `,"vulnerabilities":{"found":true,"count":1,"list":[` + cargoAuditVulnerabilityFixture + `]},"warnings":{}}`
const cargoAuditWarningsFixture = `{` + cargoAuditHeaderFixture + `,"vulnerabilities":{"found":false,"count":0,"list":[]},"warnings":{"unmaintained":[` + cargoAuditWarningFixture + `]}}`

func TestCargoAuditReportCompletion(t *testing.T) {
	for _, tt := range []struct {
		name     string
		report   string
		exit     int
		findings int
		warnings int
		failure  bool
	}{
		{"clean", cargoAuditCleanFixture, 0, 0, 0, false},
		{"findings", cargoAuditFindingFixture, 1, 1, 0, false},
		{"warnings complete exit1", cargoAuditWarningsFixture, 1, 0, 1, false},
		{"warnings clean exit0", cargoAuditWarningsFixture, 0, 0, 1, false},
		{"modern setting lists", strings.Replace(strings.Replace(cargoAuditWarningsFixture, `"target_arch":null`, `"target_arch":[]`, 1), `"target_os":null`, `"target_os":[]`, 1), 1, 0, 1, false},
		{"empty project exit1 unaccounted", cargoAuditCleanFixture, 1, 0, 0, true},
		{"audit error exit2 retains findings", cargoAuditFindingFixture, 2, 1, 0, true},
		{"unknown exit retains warnings", cargoAuditWarningsFixture, 126, 0, 1, true},
		{"missing report offline", "", 1, 0, 0, true},
		{"vulnerabilities only incomplete", `{"vulnerabilities":{"found":true,"count":1,"list":[` + cargoAuditVulnerabilityFixture + `]}}`, 1, 1, 0, true},
		{"truncated list preserves earlier finding", `{` + cargoAuditHeaderFixture + `,"vulnerabilities":{"found":true,"count":2,"list":[` + cargoAuditVulnerabilityFixture + `,{`, 1, 1, 0, true},
		{"truncated warning list preserves warning", `{` + cargoAuditHeaderFixture + `,"warnings":{"unmaintained":[` + cargoAuditWarningFixture + `,{`, 1, 0, 1, true},
		{"missing setting", strings.Replace(cargoAuditFindingFixture, `"ignore":[],`, "", 1), 1, 1, 0, true},
		{"wrong found flag", strings.Replace(cargoAuditFindingFixture, `"found":true`, `"found":false`, 1), 1, 1, 0, true},
		{"wrong count", strings.Replace(cargoAuditFindingFixture, `"count":1`, `"count":2`, 1), 1, 1, 0, true},
		{"duplicate section", strings.TrimSuffix(cargoAuditFindingFixture, "}") + `,"warnings":{}}`, 1, 1, 0, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			report := parseCargoAuditReport(strings.NewReader(tt.report))
			err := cargoAuditCompletionError(report, tt.exit, nil)
			warnings := 0
			for _, entries := range report.warnings {
				warnings += len(entries)
			}
			if len(report.vulnerabilities) != tt.findings || warnings != tt.warnings || (err != nil) != tt.failure {
				t.Fatalf("wrong report completion: findings=%+v warnings=%+v err=%v", report.vulnerabilities, report.warnings, err)
			}
			if warnings > 0 && !strings.Contains(string(report.warnings["unmaintained"][0]), "9007199254740993") {
				t.Fatal("warning fields or numeric precision lost")
			}
		})
	}
}

func TestCargoAuditReadFailureRetainsFindings(t *testing.T) {
	cause := errors.New("fixture report reader failed")
	report := parseCargoAuditReport(&securityFaultReader{input: strings.NewReader(cargoAuditFindingFixture), err: cause})
	if len(report.vulnerabilities) != 1 || !errors.Is(report.err, cause) {
		t.Fatalf("prior finding or read failure lost: %+v", report)
	}
}

func runCargoAuditProcessFixture(mode string) {
	if len(os.Args) >= 3 && os.Args[1] == "audit" && os.Args[2] == "--version" {
		fmt.Print("cargo-audit 0.22.2")
		os.Exit(0)
	}
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Print("cargo 1.92.0")
		os.Exit(0)
	}
	if len(os.Args) != 3 || os.Args[1] != "audit" || os.Args[2] != "--json" {
		os.Exit(99)
	}
	report, exit := cargoAuditCleanFixture, 0
	switch mode {
	case "audit-clean":
	case "audit-findings":
		report, exit = cargoAuditFindingFixture, 1
	case "audit-cancel":
		report = strings.Replace(cargoAuditFindingFixture, `"warnings":{}`, `"warnings":{"unmaintained":[`+cargoAuditWarningFixture+`]}`, 1)
	case "audit-warnings":
		report, exit = cargoAuditWarningsFixture, 1
	case "audit-unaccounted":
		exit = 1
	case "audit-error":
		report, exit = cargoAuditFindingFixture, 2
	case "audit-offline":
		report, exit = "", 1
	case "audit-truncated":
		report, exit = `{`+cargoAuditHeaderFixture+`,"vulnerabilities":{"found":true,"count":2,"list":[`+cargoAuditVulnerabilityFixture+`,{`, 1
	default:
		os.Exit(99)
	}
	fmt.Fprint(os.Stderr, "secret-bearing audit stderr must not be echoed")
	fmt.Print(report)
	if mode == "audit-cancel" {
		securityFixtureReadyAndWait()
	}
	os.Exit(exit)
}

func TestCargoAuditProcessCompletion(t *testing.T) {
	path, digest := installSecurityProcessFixture(t, "cargo")
	for _, tt := range []struct {
		mode     string
		findings int
		warnings int
		failure  bool
	}{
		{"audit-clean", 0, 0, false},
		{"audit-findings", 1, 0, false},
		{"audit-warnings", 0, 1, false},
		{"audit-unaccounted", 0, 0, true},
		{"audit-error", 1, 0, true},
		{"audit-offline", 0, 0, true},
		{"audit-truncated", 1, 0, true},
	} {
		t.Run(tt.mode, func(t *testing.T) {
			t.Setenv("GONEAT_SECURITY_PROCESS_FIXTURE", tt.mode)
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "Cargo.toml"), []byte("[package]\nname = \"fixture\"\nversion = \"1.0.0\"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			receipt := filepath.Join(root, "child.json")
			t.Setenv("GONEAT_SECURITY_FIXTURE_ARGS", receipt)
			cfg := DefaultAssessmentConfig()
			cfg.Timeout = 10 * time.Second
			issues, warnings, err := (&cargoAuditAdapter{moduleRoot: root, cfg: cfg}).RunWithWarnings(t.Context())
			warningCount := 0
			for _, entries := range warnings {
				warningCount += len(entries)
			}
			if len(issues) != tt.findings || warningCount != tt.warnings || (err != nil) != tt.failure {
				t.Fatalf("native report result wrong: issues=%+v warnings=%+v err=%v", issues, warnings, err)
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
			if json.Unmarshal(data, &child) != nil || child.PID <= 0 || strings.Join(child.Args[1:], " ") != "audit --json" {
				t.Fatalf("cargo-audit invocation changed: %s", data)
			}
			t.Logf("synthetic native fixture path=%s pathname_sha256=%s child=%s; not live audit or immutable process-image proof", path, digest, data)
		})
	}
}

type securityWarningFixtureTool struct {
	*securityFixtureTool
	warnings map[string][]json.RawMessage
}

func (f *securityWarningFixtureTool) RunWithWarnings(context.Context) ([]Issue, map[string][]json.RawMessage, error) {
	return f.issues, f.warnings, f.err
}

func TestSecurityWarningEvidenceSurvivesFailureAndEngine(t *testing.T) {
	for _, workers := range []int{1, 3} {
		for _, failed := range []bool{false, true} {
			t.Run(fmt.Sprintf("workers%d-failed%t", workers, failed), func(t *testing.T) {
				fixture := &securityWarningFixtureTool{
					securityFixtureTool: &securityFixtureTool{name: "fixture", available: true, applicable: true},
					warnings:            map[string][]json.RawMessage{"unmaintained": {json.RawMessage(cargoAuditWarningFixture)}},
				}
				if failed {
					fixture.err = context.Canceled
				}
				old := securityRegistry
				securityRegistry = &SecurityToolRegistry{}
				t.Cleanup(func() { securityRegistry = old })
				RegisterSecurityTool("fixture", "vuln", func(*SecurityAssessmentRunner, string, AssessmentConfig) SecurityTool { return fixture })
				cfg := DefaultAssessmentConfig()
				cfg.Concurrency = workers
				cfg.SelectedCategories = []string{string(CategorySecurity)}
				cfg.FailOnSeverity = IssueSeverity("none")
				registry := NewAssessmentRunnerRegistry()
				registry.RegisterRunner(CategorySecurity, NewSecurityAssessmentRunner())
				engine := &AssessmentEngine{runnerRegistry: registry, priorityManager: NewPriorityManager()}
				report, err := engine.RunAssessment(t.Context(), t.TempDir(), cfg)
				if err != nil {
					t.Fatal(err)
				}
				category := report.Categories[string(CategorySecurity)]
				if (category.Status == "error") != failed || len(category.Issues) != 0 || category.Reason != "" {
					t.Fatalf("warning metadata changed threshold or hid failure: %+v", category)
				}
				warnings := category.Metrics["tool_warnings"].(map[string]map[string][]json.RawMessage)["fixture"]["unmaintained"]
				if len(warnings) != 1 || string(warnings[0]) != cargoAuditWarningFixture {
					t.Fatalf("warning evidence lost: %s", warnings)
				}
				encoded, err := json.Marshal(report)
				if err != nil || !strings.Contains(string(encoded), "9007199254740993") {
					t.Fatalf("serialized warning precision lost: %s err=%v", encoded, err)
				}
			})
		}
	}
}

func TestCargoAuditCallerCancellationIsFailure(t *testing.T) {
	installSecurityProcessFixture(t, "cargo")
	t.Setenv("GONEAT_SECURITY_PROCESS_FIXTURE", "audit-clean")
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Cargo.toml"), []byte("[package]\nname = \"fixture\"\nversion = \"1.0.0\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	issues, _, err := (&cargoAuditAdapter{moduleRoot: root, cfg: DefaultAssessmentConfig()}).RunWithWarnings(ctx)
	if len(issues) != 0 || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled audit cannot complete clean: %+v err=%v", issues, err)
	}
}
