package assess

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

const gosecStatsFixture = `"Stats":{"files":1,"lines":20,"nosec":0,"found":0},"GosecVersion":"fixture"`
const gosecCleanFixture = `{"Issues":[],"Golang errors":{},` + gosecStatsFixture + `}`
const gosecIssueFixture = `{"severity":"HIGH","details":"fixture finding","file":"a.go","line":"7","rule_id":"G101"}`
const gosecFindingFixture = `{"Issues":[` + gosecIssueFixture + `],"Golang errors":{},` + gosecStatsFixture + `}`
const gosecProcessingFixture = `{"Issues":[],"Golang errors":{"fixture":[{"line":1,"column":1,"error":"secret-bearing processing body"}]},` + gosecStatsFixture + `}`
const gosecSuppressedFixture = `{"Issues":[{"severity":"HIGH","details":"fixture","file":"a.go","line":"7","rule_id":"G101","nosec":true,"suppressions":[{"kind":"inSource","justification":"fixture reason"}]}],"Golang errors":{},` + gosecStatsFixture + `}`

func TestGosecReportCompletion(t *testing.T) {
	for _, tt := range []struct {
		name         string
		output       string
		issues       int
		suppressions int
		complete     bool
		failure      bool
	}{
		{"clean", gosecCleanFixture, 0, 0, true, false},
		{"findings", gosecFindingFixture, 1, 0, true, false},
		{"nested suppressions", gosecSuppressedFixture, 0, 1, true, false},
		{"processing error without findings", gosecProcessingFixture, 0, 0, true, true},
		{"processing error with findings", strings.Replace(gosecProcessingFixture, `"Issues":[]`, `"Issues":[`+gosecIssueFixture+`]`, 1), 1, 0, true, true},
		{"processing error with suppressions", strings.Replace(gosecProcessingFixture, `"Issues":[]`, `"Issues":[`+strings.TrimSuffix(strings.TrimPrefix(gosecSuppressedFixture, `{"Issues":[`), `],"Golang errors":{},`+gosecStatsFixture+`}`)+`]`, 1), 0, 1, true, true},
		{"empty report", "", 0, 0, false, true},
		{"clean nullable issue slice", strings.Replace(gosecCleanFixture, `"Issues":[]`, `"Issues":null`, 1), 0, 0, true, false},
		{"truncated issue array prefix retained", `{"Issues":[` + gosecIssueFixture + `,{`, 1, 0, false, true},
		{"invalid entry retains earlier finding", strings.Replace(gosecFindingFixture, gosecIssueFixture, gosecIssueFixture+`,{"file":false}`, 1), 1, 0, false, true},
		{"unrelated object", `{}`, 0, 0, false, true},
		{"issues only", `{"Issues":[` + gosecIssueFixture + `]}`, 1, 0, false, true},
		{"truncated tail retains finding", `{"Issues":[` + gosecIssueFixture + `],"Stats":`, 1, 0, false, true},
		{"trailing junk retains finding", gosecFindingFixture + "invalid", 1, 0, false, true},
		{"duplicate member", strings.TrimSuffix(gosecFindingFixture, "}") + `,"Golang errors":{}}`, 1, 0, false, true},
		{"missing metrics", strings.Replace(gosecFindingFixture, `"lines":20,`, "", 1), 1, 0, false, true},
		{"negative metrics", strings.Replace(gosecFindingFixture, `"lines":20`, `"lines":-1`, 1), 1, 0, false, true},
		{"unknown large integer untouched", strings.TrimSuffix(gosecCleanFixture, "}") + `,"future":{"integer":9007199254740993}}`, 0, 0, true, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			result := NewSecurityAssessmentRunner().parseGosecScanReport([]byte(tt.output))
			if result.complete != tt.complete || (result.err != nil) != tt.failure || len(result.issues) != tt.issues || len(result.suppressions) != tt.suppressions {
				t.Fatalf("report outcome wrong: %+v", result)
			}
			if result.err != nil && strings.Contains(result.err.Error(), "secret-bearing") {
				t.Fatalf("processing error text leaked: %v", result.err)
			}
			if len(result.suppressions) > 0 && result.suppressions[0].Reason != "fixture reason" {
				t.Fatalf("source suppression reason lost: %+v", result.suppressions)
			}
		})
	}
}

func runGosecProcessFixture(mode string) {
	if mode == "gosec-discovery" {
		runGosecDiscoveryProcessFixture()
		return
	}
	exit := 0
	output := gosecCleanFixture
	switch mode {
	case "gosec-clean":
	case "gosec-findings":
		output, exit = gosecFindingFixture, 1
	case "gosec-cancel":
		output = strings.Replace(gosecSuppressedFixture, `"Issues":[`, `"Issues":[`+gosecIssueFixture+`,`, 1)
	case "gosec-suppressed":
		output = gosecSuppressedFixture
	case "gosec-processing":
		output, exit = gosecProcessingFixture, 1
	case "gosec-processing-findings":
		output, exit = strings.Replace(gosecProcessingFixture, `"Issues":[]`, `"Issues":[`+gosecIssueFixture+`]`, 1), 1
	case "gosec-unexpected":
		output, exit = gosecFindingFixture, 2
	case "gosec-empty":
		output = ""
	case "gosec-zero-packages":
		os.Exit(0)
	case "gosec-malformed-package-mapping":
		fmt.Println(".") // Deliberately violates go list's absolute-directory output.
		os.Exit(0)
	case "gosec-retry-valid":
		marker := os.Getenv("GONEAT_SECURITY_FIXTURE_READY")
		if _, err := os.Stat(marker); os.IsNotExist(err) {
			if err := os.WriteFile(marker, []byte("attempt"), 0600); err != nil {
				os.Exit(99)
			}
			output = `{"Issues":[` + gosecIssueFixture + `],"Stats":`
		}
	default:
		os.Exit(99)
	}
	fmt.Print(output)
	if mode == "gosec-cancel" {
		securityFixtureReadyAndWait()
	}
	os.Exit(exit)
}

func TestGosecProcessCompletion(t *testing.T) {
	path, digest := installSecurityProcessFixture(t, "gosec")
	for _, tt := range []struct {
		mode         string
		track        bool
		issues       int
		suppressions int
		failure      bool
	}{
		{"gosec-clean", false, 0, 0, false},
		{"gosec-findings", false, 1, 0, false},
		{"gosec-suppressed", true, 0, 1, false},
		{"gosec-processing", false, 0, 0, true},
		{"gosec-processing-findings", true, 1, 0, true},
		{"gosec-unexpected", false, 1, 0, true},
		{"gosec-empty", false, 0, 0, true},
		{"gosec-retry-valid", false, 1, 0, true},
	} {
		t.Run(tt.mode, func(t *testing.T) {
			t.Setenv("GONEAT_SECURITY_PROCESS_FIXTURE", tt.mode)
			root := t.TempDir()
			receipt := filepath.Join(root, "child.json")
			t.Setenv("GONEAT_SECURITY_FIXTURE_ARGS", receipt)
			t.Setenv("GONEAT_SECURITY_FIXTURE_READY", filepath.Join(root, "attempt"))
			cfg := DefaultAssessmentConfig()
			cfg.Timeout = 10 * time.Second
			cfg.IncludeFiles = []string{"a.go"}
			cfg.TrackSuppressions = tt.track
			issues, suppressions, err := NewSecurityAssessmentRunner().runGosec(context.Background(), root, cfg)
			if (err != nil) != tt.failure || len(issues) != tt.issues || len(suppressions) != tt.suppressions {
				t.Fatalf("native fixture outcome wrong: issues=%+v suppressions=%+v err=%v", issues, suppressions, err)
			}
			data, readErr := os.ReadFile(receipt)
			if readErr != nil {
				t.Fatal(readErr)
			}
			text := string(data)
			if !strings.Contains(text, "-fmt=json") || strings.Contains(text, "-quiet") || strings.Contains(text, "-no-fail") || strings.Contains(text, "-terse") || strings.Contains(text, "-out") || strings.Contains(text, "-track-suppressions") != tt.track {
				t.Fatalf("invocation contract changed: %s", text)
			}
			t.Logf("native synthetic fixture path=%s pathname_sha256=%s child=%s; not live tool or immutable process-image proof", path, digest, data)
		})
	}
}

func TestGosecCancellationBeforeScheduling(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cfg := DefaultAssessmentConfig()
	cfg.IncludeFiles = []string{"a.go", "other/b.go"}
	issues, suppressions, err := NewSecurityAssessmentRunner().runGosec(ctx, t.TempDir(), cfg)
	if len(issues) != 0 || len(suppressions) != 0 || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled shards cannot complete clean: issues=%+v suppressions=%+v err=%v", issues, suppressions, err)
	}
}

func TestGosecZeroShardsIsNotCompletion(t *testing.T) {
	installSecurityProcessFixture(t, "go")
	t.Setenv("GONEAT_SECURITY_PROCESS_FIXTURE", "gosec-zero-packages")
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module fixture\n"), 0600); err != nil {
		t.Fatal(err)
	}
	issues, suppressions, err := NewSecurityAssessmentRunner().runGosec(context.Background(), root, DefaultAssessmentConfig())
	if len(issues) != 0 || len(suppressions) != 0 || err == nil || !strings.Contains(err.Error(), "did not execute") || lastShardCount != 0 {
		t.Fatalf("zero executed shards cannot claim completion: issues=%+v suppressions=%+v err=%v", issues, suppressions, err)
	}
}

func TestGosecDiscoveryRelativeAbsoluteSelection(t *testing.T) {
	fixturePath, digest := installSecurityProcessFixture(t, "gosec")
	t.Setenv("GONEAT_SECURITY_PROCESS_FIXTURE", "gosec-clean")
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		"go.mod":        "module fixture\n",
		"a.go":          "package fixture\n",
		"visible/a.go":  "package visible\n",
		"ignored/a.go":  "package ignored\n",
		".goneatignore": "ignored/\n",
	} {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(root)
	for _, mode := range []string{"default", "no-ignore", "force-include"} {
		t.Run(mode, func(t *testing.T) {
			cfg := DefaultAssessmentConfig()
			cfg.Timeout = 10 * time.Second
			cfg.NoIgnore = mode == "no-ignore"
			if mode == "force-include" {
				cfg.ForceInclude = []string{"ignored/**"}
			}
			expected := []string{".", "visible"}
			if mode != "default" {
				expected = []string{".", "ignored", "visible"}
			}
			var previous []string
			for _, scope := range []string{".", root} {
				runner := NewSecurityAssessmentRunner()
				packages, err := runner.listGoPackageDirs(scope, scope, cfg)
				if err != nil {
					t.Fatal(err)
				}
				var selected []string
				for _, pkg := range packages {
					rel, err := filepath.Rel(root, pkg)
					if err != nil {
						t.Fatal(err)
					}
					selected = append(selected, filepath.ToSlash(rel))
				}
				sort.Strings(selected)
				if !reflect.DeepEqual(selected, expected) || (previous != nil && !reflect.DeepEqual(previous, selected)) {
					t.Fatalf("scope %q changed selection: got=%v expected=%v previous=%v", scope, selected, expected, previous)
				}
				previous = selected
				issues, suppressions, err := runner.runGosec(t.Context(), scope, cfg)
				if err != nil || len(issues) != 0 || len(suppressions) != 0 || lastShardCount != len(expected) {
					t.Fatalf("scope %q did not execute selected synthetic shards: issues=%+v suppressions=%+v shards=%d err=%v", scope, issues, suppressions, lastShardCount, err)
				}
			}
			t.Logf("real local go-list discovery, synthetic gosec path=%s pathname_sha256=%s; not live scanner or immutable process-image proof", fixturePath, digest)
		})
	}
}

func TestGosecDiscoveryEmptyAndIgnoredScopesFail(t *testing.T) {
	for _, allIgnored := range []bool{false, true} {
		t.Run(fmt.Sprint(allIgnored), func(t *testing.T) {
			fixturePath, _ := installSecurityProcessFixture(t, "gosec")
			t.Setenv("GONEAT_SECURITY_PROCESS_FIXTURE", "gosec-clean")
			root, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module fixture\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if allIgnored {
				if err := os.Mkdir(filepath.Join(root, "ignored"), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "ignored", "a.go"), []byte("package ignored\n"), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, ".goneatignore"), []byte("ignored/\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			t.Chdir(root)
			receipt := filepath.Join(root, "scanner.json")
			t.Setenv("GONEAT_SECURITY_FIXTURE_ARGS", receipt)
			for _, scope := range []string{".", root} {
				issues, suppressions, err := NewSecurityAssessmentRunner().runGosec(t.Context(), scope, DefaultAssessmentConfig())
				if err == nil || !strings.Contains(err.Error(), "did not execute") || len(issues) != 0 || len(suppressions) != 0 || lastShardCount != 0 {
					t.Fatalf("empty/all-ignored scope %q cannot complete: issues=%+v suppressions=%+v shards=%d err=%v", scope, issues, suppressions, lastShardCount, err)
				}
				if _, err := os.Stat(receipt); !os.IsNotExist(err) {
					t.Fatalf("zero selected scope launched scanner %s: %v", fixturePath, err)
				}
			}
		})
	}
}

func TestGosecDiscoveryMappingErrorDoesNotLaunchFallback(t *testing.T) {
	installSecurityProcessFixture(t, "go")
	installSecurityProcessFixture(t, "gosec")
	t.Setenv("GONEAT_SECURITY_PROCESS_FIXTURE", "gosec-malformed-package-mapping")
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module fixture\n"), 0600); err != nil {
		t.Fatal(err)
	}
	receipt := filepath.Join(root, "child.json")
	t.Setenv("GONEAT_SECURITY_FIXTURE_ARGS", receipt)
	issues, suppressions, err := NewSecurityAssessmentRunner().runGosec(t.Context(), root, DefaultAssessmentConfig())
	if err == nil || !strings.Contains(err.Error(), "map gosec package directory") || len(issues) != 0 || len(suppressions) != 0 || lastShardCount != 0 {
		t.Fatalf("mapping error must fail without fallback shards: issues=%+v suppressions=%+v shards=%d err=%v", issues, suppressions, lastShardCount, err)
	}
	data, readErr := os.ReadFile(receipt)
	if readErr != nil {
		t.Fatal(readErr)
	}
	var child struct {
		PID  int      `json:"pid"`
		Args []string `json:"argv"`
	}
	if json.Unmarshal(data, &child) != nil || child.PID <= 0 || strings.Join(child.Args[1:], " ") != "list -f {{.Dir}} ./..." {
		t.Fatalf("mapping error launched fallback scanner instead of only discovery: %s", data)
	}
	t.Logf("synthetic malformed go-list child=%s; no scanner launch, not live or immutable process-image proof", data)
}

type gosecDiscoveryChild struct {
	Tool string
	PID  int
	Args []string
	Cwd  string
}

// This mode is dispatched by the existing inert process fixture. Its append-only
// receipts do not change the single-receipt protocol used by other controls.
func runGosecDiscoveryProcessFixture() {
	cwd, err := os.Getwd()
	if err != nil {
		os.Exit(99)
	}
	tool := "gosec"
	if len(os.Args) > 1 && os.Args[1] == "list" {
		tool = "go"
	}
	file, err := os.OpenFile(os.Getenv("GONEAT_GOSEC_DISCOVERY_CHILDREN"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		os.Exit(99)
	}
	encodeErr := json.NewEncoder(file).Encode(gosecDiscoveryChild{Tool: tool, PID: os.Getpid(), Args: os.Args, Cwd: cwd})
	closeErr := file.Close()
	if encodeErr != nil || closeErr != nil {
		os.Exit(99)
	}
	if tool == "go" {
		var modules map[string]string
		if json.Unmarshal([]byte(os.Getenv("GONEAT_GOSEC_DISCOVERY_MODULES")), &modules) != nil {
			os.Exit(99)
		}
		switch modules[cwd] {
		case "failure":
			os.Exit(17)
		case "mapping":
			fmt.Println(".")
		case "empty":
		case "ignored":
			fmt.Println(filepath.Join(os.Getenv("GONEAT_GOSEC_DISCOVERY_ROOT"), "ignored"))
		case "valid":
			fmt.Println(cwd)
		default:
			os.Exit(99)
		}
		os.Exit(0)
	}
	output := os.Getenv("GONEAT_GOSEC_DISCOVERY_OUTPUT")
	if marker := os.Getenv("GONEAT_GOSEC_DISCOVERY_RETRY"); marker != "" {
		if _, err := os.Stat(marker); os.IsNotExist(err) {
			if os.WriteFile(marker, []byte("attempt"), 0600) != nil {
				os.Exit(99)
			}
			output = `{"Issues":[` + gosecIssueFixture + `],"Stats":`
		}
	}
	fmt.Print(output)
	if os.Getenv("GONEAT_GOSEC_DISCOVERY_CANCEL") == "1" {
		securityFixtureReadyAndWait()
	}
	exit, err := strconv.Atoi(os.Getenv("GONEAT_GOSEC_DISCOVERY_EXIT"))
	if err != nil {
		os.Exit(99)
	}
	os.Exit(exit)
}

func setupGosecDiscoveryFixture(t *testing.T) (string, string) {
	t.Helper()
	installSecurityProcessFixture(t, "go")
	installSecurityProcessFixture(t, "gosec")
	t.Setenv("GONEAT_SECURITY_PROCESS_FIXTURE", "gosec-discovery")
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "go.mod"), "module fixture\n")
	receipt := filepath.Join(t.TempDir(), "children.jsonl")
	t.Setenv("GONEAT_GOSEC_DISCOVERY_CHILDREN", receipt)
	t.Setenv("GONEAT_GOSEC_DISCOVERY_ROOT", root)
	t.Setenv("GONEAT_GOSEC_DISCOVERY_OUTPUT", gosecCleanFixture)
	t.Setenv("GONEAT_GOSEC_DISCOVERY_EXIT", "0")
	setGosecDiscoveryModules(t, map[string]string{root: "failure"})
	return root, receipt
}

func setGosecDiscoveryModules(t *testing.T, modules map[string]string) {
	t.Helper()
	data, err := json.Marshal(modules)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GONEAT_GOSEC_DISCOVERY_MODULES", string(data))
}

func gosecDiscoveryChildren(t *testing.T, path string) []gosecDiscoveryChild {
	t.Helper()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var children []gosecDiscoveryChild
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var child gosecDiscoveryChild
		if err := json.Unmarshal([]byte(line), &child); err != nil || child.PID <= 0 || len(child.Args) < 2 || child.Cwd == "" {
			t.Fatalf("invalid synthetic child receipt %q: %v", line, err)
		}
		if child.Tool == "go" && !reflect.DeepEqual(child.Args[1:], []string{"list", "-f", "{{.Dir}}", "./..."}) {
			t.Fatalf("unexpected discovery arguments: %+v", child)
		}
		children = append(children, child)
	}
	t.Logf("synthetic children=%s; not live scanners or immutable process-image proof", data)
	return children
}

func assertGosecDiscoveryScans(t *testing.T, children []gosecDiscoveryChild, root string, targets []string, track bool) {
	t.Helper()
	var expected []string
	if len(targets) > 0 {
		expected = []string{"-fmt=json"}
		if track {
			expected = append(expected, "-track-suppressions")
		}
		for _, exclusion := range parseIgnorePatternsForGosec(root) {
			expected = append(expected, "-exclude-dir", exclusion)
		}
	}
	var observed []string
	for _, child := range children {
		if child.Tool != "gosec" {
			continue
		}
		if child.Cwd != root || !reflect.DeepEqual(child.Args[1:len(child.Args)-1], expected) {
			t.Fatalf("scanner flags/exclusions/cwd changed: %+v, expected prefix=%q cwd=%q", child, expected, root)
		}
		observed = append(observed, child.Args[len(child.Args)-1])
	}
	if !reflect.DeepEqual(observed, targets) {
		t.Fatalf("fallback widened or dropped scan targets: got=%q want=%q", observed, targets)
	}
}

func gosecDiscoveryEvidenceFixture() string {
	return strings.Replace(gosecSuppressedFixture, `"Issues":[`, `"Issues":[`+gosecIssueFixture+`,`, 1)
}

func TestGosecCommandDiscoveryFailureFallbackRetainsEvidence(t *testing.T) {
	evidence := gosecDiscoveryEvidenceFixture()
	for _, tt := range []struct {
		name         string
		output       string
		exit         string
		issues       int
		suppressions int
		attempts     int
		scanError    bool
		retry        bool
	}{
		{"clean", gosecCleanFixture, "0", 0, 0, 1, false, false},
		{"finding", gosecFindingFixture, "1", 1, 0, 1, false, false},
		{"suppression", gosecSuppressedFixture, "0", 0, 1, 1, false, false},
		{"finding and suppression", evidence, "1", 1, 1, 1, false, false},
		{"processing error with evidence", strings.Replace(evidence, `"Golang errors":{}`, `"Golang errors":{"fixture":[{"error":"secret-bearing processing body"}]}`, 1), "1", 1, 1, 1, true, false},
		{"unexpected exit with evidence", evidence, "2", 1, 1, 1, true, false},
		{"absent report", "", "0", 0, 0, 1, true, false},
		{"truncated prefix then complete", evidence, "1", 1, 1, 2, true, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root, receipt := setupGosecDiscoveryFixture(t)
			writeTestFile(t, filepath.Join(root, ".goneatignore"), "ignored/\n")
			t.Setenv("GONEAT_GOSEC_DISCOVERY_OUTPUT", tt.output)
			t.Setenv("GONEAT_GOSEC_DISCOVERY_EXIT", tt.exit)
			if tt.retry {
				t.Setenv("GONEAT_GOSEC_DISCOVERY_RETRY", filepath.Join(root, "attempt"))
			}
			cfg := DefaultAssessmentConfig()
			cfg.Concurrency, cfg.TrackSuppressions = 1, true
			issues, suppressions, err := NewSecurityAssessmentRunner().runGosec(t.Context(), root, cfg)
			var discoveryErr *gosecPackageDiscoveryCommandError
			var exitErr *exec.ExitError
			if !errors.As(err, &discoveryErr) || !errors.As(discoveryErr, &exitErr) || exitErr.ExitCode() != 17 || len(issues) != tt.issues || len(suppressions) != tt.suppressions {
				t.Fatalf("original discovery error/evidence lost: issues=%+v suppressions=%+v err=%v", issues, suppressions, err)
			}
			if strings.Contains(err.Error(), "gosec shard") != tt.scanError || strings.Contains(err.Error(), "secret-bearing") {
				t.Fatalf("fallback processing/process errors lost or leaked: %v", err)
			}
			if lastShardCount != 1 || lastPoolSize != 1 {
				t.Fatalf("expected one fallback shard, got shards=%d pool=%d", lastShardCount, lastPoolSize)
			}
			if len(issues) > 0 && (issues[0].File != "a.go" || !strings.Contains(issues[0].Message, "fixture finding")) {
				t.Fatalf("trustworthy finding changed: %+v", issues)
			}
			if len(suppressions) > 0 && (suppressions[0].RuleID != "G101" || suppressions[0].Reason != "fixture reason") {
				t.Fatalf("trustworthy suppression changed: %+v", suppressions)
			}
			children := gosecDiscoveryChildren(t, receipt)
			if len(children) != 1+tt.attempts || children[0].Tool != "go" {
				t.Fatalf("unexpected discovery/retry child count: %+v", children)
			}
			targets := make([]string, tt.attempts)
			for i := range targets {
				targets[i] = "./..."
			}
			assertGosecDiscoveryScans(t, children, root, targets, true)
		})
	}
}

func TestGosecCommandDiscoveryFailureClassification(t *testing.T) {
	t.Run("nonzero command", func(t *testing.T) {
		root, receipt := setupGosecDiscoveryFixture(t)
		_, err := NewSecurityAssessmentRunner().listGoPackageDirs(root, root, DefaultAssessmentConfig())
		var commandErr *gosecPackageDiscoveryCommandError
		var exitErr *exec.ExitError
		joined := errors.Join(errors.New("other"), fmt.Errorf("wrapped: %w", err))
		if !errors.As(joined, &commandErr) || !errors.As(joined, &exitErr) || exitErr.ExitCode() != 17 || commandErr.Error() != errors.Unwrap(commandErr).Error() {
			t.Fatalf("command wrapper lost original exit: %v", joined)
		}
		if children := gosecDiscoveryChildren(t, receipt); len(children) != 1 || children[0].Tool != "go" {
			t.Fatalf("classification ran a scanner: %+v", children)
		}
	})
	t.Run("non-ExitError start failure", func(t *testing.T) {
		root, receipt := setupGosecDiscoveryFixture(t)
		// Restrict only this synthetic test's PATH to a directory with gosec but
		// no go. Never remove a real tool or modify the operator's environment.
		path, _ := installSecurityProcessFixture(t, "gosec")
		t.Setenv("PATH", filepath.Dir(path))
		runner := NewSecurityAssessmentRunner()
		_, err := runner.listGoPackageDirs(root, root, DefaultAssessmentConfig())
		var commandErr *gosecPackageDiscoveryCommandError
		var startErr *exec.Error
		var exitErr *exec.ExitError
		if !errors.As(err, &commandErr) || !errors.As(err, &startErr) || errors.As(err, &exitErr) || !errors.Is(err, exec.ErrNotFound) {
			t.Fatalf("start failure was lost or invented an ExitError: %v", err)
		}
		issues, suppressions, err := runner.runGosec(t.Context(), root, DefaultAssessmentConfig())
		if !errors.As(err, &commandErr) || !errors.As(err, &startErr) || errors.As(err, &exitErr) || len(issues) != 0 || len(suppressions) != 0 || lastShardCount != 1 {
			t.Fatalf("start failure fallback hid the command error: %v", err)
		}
		children := gosecDiscoveryChildren(t, receipt)
		if len(children) != 1 {
			t.Fatalf("missing-go case launched an unexpected child: %+v", children)
		}
		assertGosecDiscoveryScans(t, children, root, []string{"./..."}, false)
	})
	t.Run("representative command read error", func(t *testing.T) {
		original := io.ErrUnexpectedEOF
		err := &gosecPackageDiscoveryCommandError{err: original}
		var commandErr *gosecPackageDiscoveryCommandError
		if !errors.As(errors.Join(fmt.Errorf("wrapped: %w", err)), &commandErr) || !errors.Is(err, original) || errors.Unwrap(err) != original || err.Error() != original.Error() {
			t.Fatalf("representative read error was not preserved: %v", err)
		}
		// Classification coverage only, not a reproduced physical pipe fault.
	})
	t.Run("non-command errors remain distinct", func(t *testing.T) {
		root, receipt := setupGosecDiscoveryFixture(t)
		_, mappingErr := filepath.Rel(root, ".")
		_, walkErr := os.Stat(filepath.Join(root, "missing"))
		for _, original := range []error{context.Canceled, context.DeadlineExceeded, mappingErr, walkErr} {
			if original == nil {
				t.Fatal("missing independent non-command error fixture")
			}
			for _, err := range []error{original, fmt.Errorf("wrapped: %w", original), errors.Join(errors.New("other"), original)} {
				var commandErr *gosecPackageDiscoveryCommandError
				if errors.As(err, &commandErr) || !errors.Is(err, original) {
					t.Fatalf("independent non-command error became command-class: %v", err)
				}
			}
		}
		if len(gosecDiscoveryChildren(t, receipt)) != 0 {
			t.Fatal("non-command classification launched a child")
		}
	})
}

func TestGosecDiscoveryFailuresDoNotWidenScope(t *testing.T) {
	for _, tt := range []struct {
		name    string
		root    string
		nested  string
		targets []string
		command bool
	}{
		{"all commands failed", "failure", "failure", []string{"./..."}, true},
		{"command plus mapping", "failure", "mapping", nil, true},
		{"command plus successful empty", "failure", "empty", nil, true},
		{"command plus all ignored", "failure", "ignored", nil, true},
		{"command plus valid module", "failure", "valid", []string{"nested"}, true},
		{"mapping plus valid module", "mapping", "valid", []string{"nested"}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root, receipt := setupGosecDiscoveryFixture(t)
			nested := filepath.Join(root, "nested")
			writeTestFile(t, filepath.Join(nested, "go.mod"), "module nested\n")
			writeTestFile(t, filepath.Join(root, "ignored", "a.go"), "package ignored\n")
			writeTestFile(t, filepath.Join(root, ".goneatignore"), "ignored/\n")
			setGosecDiscoveryModules(t, map[string]string{root: tt.root, nested: tt.nested})
			t.Setenv("GONEAT_GOSEC_DISCOVERY_OUTPUT", gosecDiscoveryEvidenceFixture())
			t.Setenv("GONEAT_GOSEC_DISCOVERY_EXIT", "1")
			cfg := DefaultAssessmentConfig()
			cfg.Concurrency, cfg.TrackSuppressions = 1, true
			issues, suppressions, err := NewSecurityAssessmentRunner().runGosec(t.Context(), root, cfg)
			var commandErr *gosecPackageDiscoveryCommandError
			if err == nil || errors.As(err, &commandErr) != tt.command || len(issues) != len(tt.targets) || len(suppressions) != len(tt.targets) || lastShardCount != len(tt.targets) {
				t.Fatalf("mixed discovery lost errors/evidence or widened scope: issues=%+v suppressions=%+v shards=%d err=%v", issues, suppressions, lastShardCount, err)
			}
			if (tt.root == "mapping" || tt.nested == "mapping") && !strings.Contains(err.Error(), "map gosec package directory") {
				t.Fatalf("mapping error was dropped: %v", err)
			}
			children := gosecDiscoveryChildren(t, receipt)
			if len(children) != 2+len(tt.targets) {
				t.Fatalf("unexpected mixed-module child count: %+v", children)
			}
			assertGosecDiscoveryScans(t, children, root, tt.targets, true)
		})
	}
}

func TestGosecModuleWalkFailureDoesNotLaunchFallback(t *testing.T) {
	root, receipt := setupGosecDiscoveryFixture(t)
	issues, suppressions, err := NewSecurityAssessmentRunner().runGosec(t.Context(), filepath.Join(root, "missing"), DefaultAssessmentConfig())
	var commandErr *gosecPackageDiscoveryCommandError
	if err == nil || !strings.Contains(err.Error(), "module discovery") || errors.As(err, &commandErr) || len(issues) != 0 || len(suppressions) != 0 || lastShardCount != 0 || len(gosecDiscoveryChildren(t, receipt)) != 0 {
		t.Fatalf("walk failure launched a broad fallback or lost the error: %v", err)
	}
}

func TestGosecCommandDiscoveryFailureFallbackCancellation(t *testing.T) {
	for _, beforeScheduling := range []bool{true, false} {
		t.Run(fmt.Sprint(beforeScheduling), func(t *testing.T) {
			root, receipt := setupGosecDiscoveryFixture(t)
			t.Setenv("GONEAT_GOSEC_DISCOVERY_OUTPUT", gosecDiscoveryEvidenceFixture())
			t.Setenv("GONEAT_GOSEC_DISCOVERY_CANCEL", "1")
			ready := filepath.Join(root, "ready")
			t.Setenv("GONEAT_SECURITY_FIXTURE_READY", ready)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			var done chan bool
			if beforeScheduling {
				cancel()
			} else {
				done = make(chan bool, 1)
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
			}
			cfg := DefaultAssessmentConfig()
			cfg.Concurrency, cfg.TrackSuppressions, cfg.Timeout = 1, true, 20*time.Second
			issues, suppressions, err := NewSecurityAssessmentRunner().runGosec(ctx, root, cfg)
			cancel()
			if done != nil && !<-done {
				t.Fatal("fallback never reached post-report readiness")
			}
			var commandErr *gosecPackageDiscoveryCommandError
			if !errors.As(err, &commandErr) || !errors.Is(err, context.Canceled) {
				t.Fatalf("cancellation lost discovery/context errors: %v", err)
			}
			var targets []string
			if !beforeScheduling {
				targets = []string{"./..."}
			}
			if len(issues) != len(targets) || len(suppressions) != len(targets) {
				t.Fatalf("cancellation lost trustworthy evidence: issues=%+v suppressions=%+v err=%v", issues, suppressions, err)
			}
			children := gosecDiscoveryChildren(t, receipt)
			if len(children) != 1+len(targets) {
				t.Fatalf("canceled fallback launched unexpected children: %+v", children)
			}
			assertGosecDiscoveryScans(t, children, root, targets, true)
		})
	}
}

func TestGosecFallbackDiscoveryErrorSurvivesCategory(t *testing.T) {
	for _, tt := range []struct {
		name         string
		output       string
		issues       int
		suppressions int
	}{
		{"clean", gosecCleanFixture, 0, 0},
		{"finding", gosecFindingFixture, 1, 0},
		{"suppressed", gosecSuppressedFixture, 0, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root, receipt := setupGosecDiscoveryFixture(t)
			t.Setenv("GONEAT_GOSEC_DISCOVERY_OUTPUT", tt.output)
			cfg := DefaultAssessmentConfig()
			cfg.SecurityTools = []string{"gosec"}
			cfg.Concurrency, cfg.TrackSuppressions = 1, true
			cfg.FailOnSeverity = IssueSeverity("none")
			result, err := NewSecurityAssessmentRunner().Assess(t.Context(), root, cfg)
			if err != nil || result == nil || result.Success || !strings.Contains(result.Error, "package discovery") || result.SkipReason != "" || len(result.Issues) != tt.issues {
				t.Fatalf("actual gosec category cleared discovery failure: result=%+v transport=%v", result, err)
			}
			suppressions, _ := result.Metrics["_suppressions"].([]Suppression)
			if len(suppressions) != tt.suppressions || result.Metrics["tools_started"] != 1 || result.Metrics["tools_completed"] != 0 || result.Metrics["tools_failed"] != 1 {
				t.Fatalf("failed category lost suppressions or invented completion: %+v", result)
			}
			children := gosecDiscoveryChildren(t, receipt)
			if len(children) != 2 {
				t.Fatalf("category ran unselected scanners: %+v", children)
			}
			assertGosecDiscoveryScans(t, children, root, []string{"./..."}, true)
		})
	}
}
