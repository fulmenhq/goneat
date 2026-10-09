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

const gitleaksFindingFixture = `{"RuleID":"fixture","Description":"fixture secret","File":"a.go","StartLine":7,"Secret":"secret-bearing-report-field"}`

func TestGitleaksStreamCompletion(t *testing.T) {
	for _, tt := range []struct {
		name     string
		report   string
		findings int
		failure  bool
	}{
		{"clean", `[]`, 0, false},
		{"findings array", `[` + gitleaksFindingFixture + `]`, 1, false},
		{"NDJSON compatibility", gitleaksFindingFixture + "\n" + gitleaksFindingFixture, 2, false},
		{"truncated array retains finding", `[` + gitleaksFindingFixture + `,{`, 1, true},
		{"malformed NDJSON retains finding", gitleaksFindingFixture + "\ninvalid", 1, true},
		{"trailing data retains finding", `[` + gitleaksFindingFixture + `]invalid`, 1, true},
		{"empty", ``, 0, true},
		{"null", `null`, 0, true},
		{"invalid finding", `[{}]`, 0, true},
		{"unknown large number", strings.Replace(gitleaksFindingFixture, `"Secret":`, `"future":9007199254740993,"Secret":`, 1), 1, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			issues, err := NewSecurityAssessmentRunner().parseGitleaksOutput([]byte(tt.report))
			if len(issues) != tt.findings || (err != nil) != tt.failure {
				t.Fatalf("completion or finding preservation wrong: %+v err=%v", issues, err)
			}
			if len(issues) > 0 && (issues[0].Line != 7 || strings.Contains(issues[0].Message, "secret-bearing-report-field")) {
				t.Fatalf("numeric line lost or raw Secret exposed: %+v", issues)
			}
		})
	}
}

func TestGitleaksReadFailureRetainsFindings(t *testing.T) {
	cause := errors.New("fixture read failed")
	reader := &securityFaultReader{input: strings.NewReader(`[` + gitleaksFindingFixture + `]`), err: cause}
	issues, err := NewSecurityAssessmentRunner().parseGitleaksStream(reader)
	// A complete document followed by a failed reader is not a clean EOF.
	if len(issues) != 1 || !errors.Is(err, cause) {
		t.Fatalf("reader failure must not erase findings or complete clean: %+v err=%v", issues, err)
	}
}

func runGitleaksProcessFixture(mode string) {
	report := `[` + gitleaksFindingFixture + `]`
	exit := 42
	switch mode {
	case "gitleaks-clean":
		report, exit = `[]`, 0
	case "gitleaks-findings":
	case "gitleaks-cancel":
	case "gitleaks-partial":
		exit = 1
	case "gitleaks-unexpected":
		exit = 126
	case "gitleaks-truncated":
		report = `[` + gitleaksFindingFixture + `,{`
	case "gitleaks-absent":
		report, exit = "", 126
	case "gitleaks-clean-mismatch":
		exit = 0
	case "gitleaks-findings-mismatch":
		report = `[]`
	default:
		os.Exit(99)
	}
	fmt.Fprint(os.Stderr, "secret-bearing fixture stderr must not be logged")
	fmt.Print(report)
	if mode == "gitleaks-cancel" {
		securityFixtureReadyAndWait()
	}
	os.Exit(exit)
}

func TestGitleaksProcessCompletion(t *testing.T) {
	path, digest := installSecurityProcessFixture(t, "gitleaks")
	for _, tt := range []struct {
		mode     string
		findings int
		failure  bool
	}{
		{"gitleaks-clean", 0, false},
		{"gitleaks-findings", 1, false},
		{"gitleaks-partial", 1, true},
		{"gitleaks-unexpected", 1, true},
		{"gitleaks-truncated", 1, true},
		{"gitleaks-absent", 0, true},
		{"gitleaks-clean-mismatch", 1, true},
		{"gitleaks-findings-mismatch", 0, true},
	} {
		t.Run(tt.mode, func(t *testing.T) {
			t.Setenv("GONEAT_SECURITY_PROCESS_FIXTURE", tt.mode)
			root := t.TempDir()
			receipt := filepath.Join(root, "child.json")
			t.Setenv("GONEAT_SECURITY_FIXTURE_ARGS", receipt)
			cfg := DefaultAssessmentConfig()
			cfg.Timeout = 10 * time.Second
			issues, err := NewSecurityAssessmentRunner().runGitleaks(context.Background(), root, cfg)
			if len(issues) != tt.findings || (err != nil) != tt.failure {
				t.Fatalf("wrong process outcome: %+v err=%v", issues, err)
			}
			if err != nil && strings.Contains(err.Error(), "secret-bearing") {
				t.Fatalf("raw scanner output leaked: %v", err)
			}
			data, err := os.ReadFile(receipt)
			if err != nil {
				t.Fatal(err)
			}
			var child struct {
				PID  int      `json:"pid"`
				Args []string `json:"argv"`
			}
			if json.Unmarshal(data, &child) != nil || child.PID <= 0 || strings.Join(child.Args[1:], " ") != "detect --no-banner --report-format json --report-path - --source "+root+" --exit-code 42" {
				t.Fatalf("gitleaks invocation changed beyond dedicated findings code: %s", data)
			}
			t.Logf("synthetic native fixture path=%s pathname_sha256=%s pid=%d argv=%q; not live tool or immutable process-image proof", path, digest, child.PID, child.Args)
		})
	}
}

func TestGitleaksScopedPartialFailureCannotDisappear(t *testing.T) {
	installSecurityProcessFixture(t, "gitleaks")
	t.Setenv("GONEAT_SECURITY_PROCESS_FIXTURE", "gitleaks-partial")
	root := t.TempDir()
	cfg := DefaultAssessmentConfig()
	cfg.IncludeFiles = []string{filepath.Join(root, "other.go")}
	issues, err := NewSecurityAssessmentRunner().runGitleaks(context.Background(), root, cfg)
	if len(issues) != 0 || err == nil {
		t.Fatalf("post-filtered finding must not hide partial-scan exit1: %+v err=%v", issues, err)
	}
}
