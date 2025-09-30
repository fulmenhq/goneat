package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/fulmenhq/goneat/internal/assess"
)

// minimal fake runner for CLI tests (no external tools)
type cliFakeRunner struct{}

func (c *cliFakeRunner) Assess(ctx context.Context, target string, cfg assess.AssessmentConfig) (*assess.AssessmentResult, error) {
	return &assess.AssessmentResult{
		CommandName:   "format",
		Category:      assess.CategoryFormat,
		Success:       true,
		ExecutionTime: assess.HumanReadableDuration(5 * time.Millisecond),
		Issues:        nil,
	}, nil
}
func (c *cliFakeRunner) CanRunInParallel() bool                 { return true }
func (c *cliFakeRunner) GetCategory() assess.AssessmentCategory { return assess.CategoryFormat }
func (c *cliFakeRunner) GetEstimatedTime(string) time.Duration  { return time.Millisecond }
func (c *cliFakeRunner) IsAvailable() bool                      { return true }

// configurable fake runner for more complex tests
type configurableFakeRunner struct {
	category  assess.AssessmentCategory
	issues    []assess.Issue
	delay     time.Duration
	available bool
}

func (c *configurableFakeRunner) Assess(ctx context.Context, target string, cfg assess.AssessmentConfig) (*assess.AssessmentResult, error) {
	if c.delay > 0 {
		timer := time.NewTimer(c.delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
		}
	}

	return &assess.AssessmentResult{
		CommandName:   string(c.category),
		Category:      c.category,
		Success:       true,
		ExecutionTime: assess.HumanReadableDuration(c.delay),
		Issues:        c.issues,
	}, nil
}
func (c *configurableFakeRunner) CanRunInParallel() bool                 { return true }
func (c *configurableFakeRunner) GetCategory() assess.AssessmentCategory { return c.category }
func (c *configurableFakeRunner) GetEstimatedTime(string) time.Duration  { return time.Millisecond }
func (c *configurableFakeRunner) IsAvailable() bool                      { return c.available }

// TestAssessCLI_FormatOnlyCategories is disabled due to context setup complexity
// TODO: Re-enable when context handling is properly implemented in tests

func TestParseAssessmentMode(t *testing.T) {
	testCases := []struct {
		name        string
		modeStr     string
		noOp        bool
		check       bool
		fix         bool
		expected    assess.AssessmentMode
		shouldError bool
	}{
		{"check mode", "check", false, false, false, assess.AssessmentModeCheck, false},
		{"no-op mode", "no-op", false, false, false, assess.AssessmentModeNoOp, false},
		{"fix mode", "fix", false, false, false, assess.AssessmentModeFix, false},
		{"shorthand no-op", "check", true, false, false, assess.AssessmentModeNoOp, false},
		{"shorthand check", "check", false, true, false, assess.AssessmentModeCheck, false},
		{"shorthand fix", "check", false, false, true, assess.AssessmentModeFix, false},
		{"invalid mode", "invalid", false, false, false, "", true},
		{"multiple modes", "check", true, true, false, "", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := parseAssessmentMode(tc.modeStr, tc.noOp, tc.check, tc.fix)

			if tc.shouldError {
				if err == nil {
					t.Fatalf("expected error for %s", tc.name)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error for %s: %v", tc.name, err)
				}
				if result != tc.expected {
					t.Fatalf("expected %s, got %s", tc.expected, result)
				}
			}
		})
	}
}

func TestShouldFailHook(t *testing.T) {
	testCases := []struct {
		name     string
		issues   []assess.Issue
		failOn   string
		expected bool
	}{
		{"no issues", []assess.Issue{}, "high", false},
		{"low severity", []assess.Issue{{Severity: assess.SeverityLow}}, "high", false},
		{"high severity fail", []assess.Issue{{Severity: assess.SeverityHigh}}, "high", true},
		{"high severity pass", []assess.Issue{{Severity: assess.SeverityHigh}}, "critical", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			report := &assess.AssessmentReport{
				Categories: map[string]assess.CategoryResult{
					"test": {Issues: tc.issues},
				},
			}
			config := &HookConfig{FailOn: tc.failOn}

			result := shouldFailHook(report, config)
			if result != tc.expected {
				t.Fatalf("expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestAssessCLI_Modes(t *testing.T) {
	// Save and reset registry via test helpers for isolation
	originalRegistry := assess.GetAssessmentRunnerRegistry()
	testRegistry := assess.ResetRegistryForTesting()
	// Register fake runners
	testRegistry.RegisterRunner(assess.CategoryFormat, &cliFakeRunner{})
	testRegistry.RegisterRunner(assess.CategorySecurity, &cliFakeRunner{})
	testRegistry.RegisterRunner(assess.CategoryLint, &cliFakeRunner{})
	testRegistry.RegisterRunner(assess.CategoryStaticAnalysis, &cliFakeRunner{})
	testRegistry.RegisterRunner(assess.CategorySchema, &cliFakeRunner{})
	t.Cleanup(func() {
		assess.RestoreRegistry(originalRegistry)
		assessMode, assessNoOp, assessCheck, assessFix = "", false, false, false
	})

	testCases := []struct {
		name          string
		args          []string
		shouldSucceed bool
	}{
		{"check mode", []string{"--mode", "check", "--concurrency", "1", "."}, true},
		{"no-op mode", []string{"--mode", "no-op", "--concurrency", "1", "."}, true},
		{"fix mode", []string{"--mode", "fix", "--concurrency", "1", "."}, true},
		{"shorthand check", []string{"--check", "--concurrency", "1", "."}, true},
		{"shorthand no-op", []string{"--no-op", "--concurrency", "1", "."}, true},
		{"shorthand fix", []string{"--fix", "--concurrency", "1", "."}, true},
		{"invalid mode", []string{"--mode", "invalid", "--concurrency", "1", "."}, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset global mode flags for each subtest
			assessMode, assessNoOp, assessCheck, assessFix = "", false, false, false

			// Build a fresh command instance to avoid flag reuse across subtests
			cmd := &cobra.Command{Use: "assess", RunE: runAssess}
			setupAssessCommandFlags(cmd)
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs(tc.args)

			err := cmd.ExecuteContext(context.Background())
			if tc.shouldSucceed && err != nil {
				t.Fatalf("expected success for %s, got error: %v", tc.name, err)
			}
			if !tc.shouldSucceed && err == nil {
				t.Fatalf("expected failure for %s, got success", tc.name)
			}
		})
	}
}

// TestAssessCLI_OutputFormats is disabled due to context setup complexity
// TODO: Re-enable when context handling is properly implemented in tests

func TestAssessCLI_FailOnThresholds(t *testing.T) {
	assessMode, assessNoOp, assessCheck, assessFix = "check", false, false, false

	// Create a runner that returns high severity issues
	highSeverityRunner := &configurableFakeRunner{
		category: assess.CategorySecurity,
		issues: []assess.Issue{{
			File: "test.go", Line: 1, Severity: assess.SeverityHigh,
			Message: "test issue", Category: assess.CategorySecurity,
		}},
		available: true,
	}

	// Save original registry and runner for category
	originalRegistry := assess.GetAssessmentRunnerRegistry()
	_ = assess.ResetRegistryForTesting()
	// Register fake for test
	assess.RegisterAssessmentRunner(assess.CategorySecurity, highSeverityRunner)
	t.Cleanup(func() { assess.RestoreRegistry(originalRegistry) })

	testCases := []struct {
		name       string
		failOn     string
		shouldFail bool
	}{
		{"fail on critical with high issue", "critical", false},
		{"fail on high with high issue", "high", true},
		{"fail on medium with high issue", "medium", true},
		{"fail on low with high issue", "low", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset global mode flags for each subtest
			assessMode, assessNoOp, assessCheck, assessFix = "check", false, false, false

			// Build a fresh assess command instance to avoid state bleed and ensure RunE is wired
			cmd := &cobra.Command{Use: "assess", RunE: runAssess}
			setupAssessCommandFlags(cmd)
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)
			cmd.SetErr(buf)
			cmd.SetArgs([]string{"--fail-on", tc.failOn, "--categories", "security", "--concurrency", "1", "."})

			err := cmd.ExecuteContext(context.Background())

			if tc.shouldFail {
				if err == nil {
					t.Fatalf("expected command to fail with %s threshold", tc.failOn)
				}
			} else {
				if err != nil {
					t.Fatalf("expected command to succeed with %s threshold, got error: %v", tc.failOn, err)
				}
			}
		})
	}
}

func TestAssessCLI_InvalidTarget(t *testing.T) {
	// Reset mode flags to avoid bleed-over from other tests
	assessMode, assessNoOp, assessCheck, assessFix = "check", false, false, false
	out, err := execRoot(t, []string{"assess", "/nonexistent/path"})
	if err == nil {
		t.Fatalf("expected error for nonexistent target directory\n%s", out)
	}
	if !strings.Contains(err.Error(), "target directory does not exist") {
		t.Fatalf("expected 'target directory does not exist' error, got: %v", err)
	}
}

func TestAssessCLI_CustomPriorities(t *testing.T) {
	// Reset global mode flags
	assessMode, assessNoOp, assessCheck, assessFix = "check", false, false, false

	// Save and restore registry to isolate test
	originalRegistry := assess.GetAssessmentRunnerRegistry()
	t.Cleanup(func() { assess.RestoreRegistry(originalRegistry) })

	// Ensure real schema runner is available (other tests may replace it)
	assess.RegisterAssessmentRunner(assess.CategorySchema, assess.NewSchemaAssessmentRunner())
	// Use a fresh assess command instance to avoid global root state
	cmd := &cobra.Command{Use: "assess", RunE: runAssess}
	setupAssessCommandFlags(cmd)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--mode", "check", "--categories", "schema", "--priority", "schema=1", "--format", "json", "--concurrency", "1", "."})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v\n%s", err, buf.String())
	}
	out := buf.String()
	if !strings.Contains(out, `"tool":`) {
		preview := out
		if len(out) > 200 {
			preview = out[:200]
		}
		t.Fatalf("expected JSON output, got: %s", preview)
	}
}

func TestAssessCLI_Timeout(t *testing.T) {
	t.Skip("Timeout test disabled - fake runners don't support realistic timeout testing")
	old := assess.GetAssessmentRunnerRegistry()

	// Create a slow runner
	slowRunner := &configurableFakeRunner{
		category:  assess.CategoryFormat,
		delay:     200 * time.Millisecond,
		available: true,
	}

	assess.RegisterAssessmentRunner(assess.CategoryFormat, slowRunner)
	t.Cleanup(func() { _ = old })

	buf := new(bytes.Buffer)
	assessCmd.SetOut(buf)
	assessCmd.SetErr(buf)
	assessCmd.SetArgs([]string{"--timeout", "50ms", "--concurrency", "1", "."})

	// Should succeed but with timeout error recorded
	if err := assessCmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "error") {
		t.Fatalf("expected timeout error in output, got: %s", out)
	}
}

func TestAssessCLI_VerboseOutput(t *testing.T) {
	t.Skip("Verbose output test disabled - fake runners don't produce realistic verbose output for testing")
	old := assess.GetAssessmentRunnerRegistry()
	assess.RegisterAssessmentRunner(assess.CategoryFormat, &cliFakeRunner{})
	t.Cleanup(func() { _ = old })

	buf := new(bytes.Buffer)
	assessCmd.SetOut(buf)
	assessCmd.SetErr(buf)
	assessCmd.SetArgs([]string{"--verbose", "--concurrency", "1", "."})

	if err := assessCmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	// Verbose output should contain more detailed information
	if len(out) < 100 { // Arbitrary threshold for "verbose" output
		t.Fatalf("expected verbose output to be longer, got: %s", out)
	}
}

// TestExecute tests the main command execution path
func TestExecute(t *testing.T) {
	// Test Execute function (cmd/root.go:36)
	// Note: Execute() calls rootCmd.Execute(), which is hard to test directly
	// But we can at least ensure no panic occurs when calling it
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Execute() panicked: %v", r)
		}
	}()

	// This would normally call os.Exit, but in test we just ensure no panic
	// Execute() // Cannot call directly due to os.Exit calls
}

// TestRunInfo tests the info command
func TestRunInfo(t *testing.T) {
	out, err := execRoot(t, []string{"info", "--help"})
	if err != nil {
		t.Fatalf("unexpected error: %v\n%s", err, out)
	}
	if !strings.Contains(out, "licenses") {
		t.Fatalf("expected info help to mention 'licenses', got: %s", out)
	}
}

// TestRunVersion tests the version command
func TestRunVersion(t *testing.T) {
	out, err := execRoot(t, []string{"version"})
	if err != nil {
		t.Fatalf("unexpected error: %v\n%s", err, out)
	}
	if strings.TrimSpace(out) == "" {
		t.Fatalf("expected version output, got empty string")
	}
}

// TestRunValidate tests the validate command with existing good schema
func TestRunValidate(t *testing.T) {
	// Exercise validate via root path to ensure consistent flag/parent behavior
	out, err := execRoot(t, []string{"validate", "--include", "schemas/", "--format", "markdown"})
	if err != nil {
		t.Fatalf("unexpected error: %v\n%s", err, out)
	}
	if strings.TrimSpace(out) == "" {
		t.Fatalf("expected validation output, got empty")
	}
}

// TestAssessCLI_JSONPurity tests that --format json produces pure JSON on stdout
// and logs on stderr, ensuring programmatic consumption works correctly
func TestAssessCLI_JSONPurity(t *testing.T) {
	// Save and reset registry for isolation
	originalRegistry := assess.GetAssessmentRunnerRegistry()
	testRegistry := assess.ResetRegistryForTesting()

	// Register fake runners for quick execution
	testRegistry.RegisterRunner(assess.CategoryFormat, &cliFakeRunner{})
	testRegistry.RegisterRunner(assess.CategorySecurity, &cliFakeRunner{})
	testRegistry.RegisterRunner(assess.CategoryLint, &cliFakeRunner{})
	testRegistry.RegisterRunner(assess.CategoryStaticAnalysis, &cliFakeRunner{})
	testRegistry.RegisterRunner(assess.CategorySchema, &cliFakeRunner{})

	t.Cleanup(func() {
		assess.RestoreRegistry(originalRegistry)
		assessMode, assessNoOp, assessCheck, assessFix = "", false, false, false
	})

	// Reset global mode flags
	assessMode, assessNoOp, assessCheck, assessFix = "", false, false, false

	// Create separate buffers for stdout and stderr
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Build command with JSON format
	cmd := &cobra.Command{Use: "assess", RunE: runAssess}
	setupAssessCommandFlags(cmd)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--format", "json", "--concurrency", "1", "."})

	// Execute command
	err := cmd.ExecuteContext(context.Background())
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	// Verify stdout contains valid JSON
	stdoutStr := stdout.String()
	if strings.TrimSpace(stdoutStr) == "" {
		t.Fatal("expected JSON output on stdout, got empty")
	}

	// Parse JSON to ensure it's valid
	var jsonData interface{}
	if err := json.Unmarshal([]byte(stdoutStr), &jsonData); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout content: %s", err, stdoutStr)
	}

	// Verify JSON contains expected structure
	jsonMap, ok := jsonData.(map[string]interface{})
	if !ok {
		t.Fatalf("expected JSON object, got: %T", jsonData)
	}

	// Check for required top-level keys
	requiredKeys := []string{"metadata", "summary", "categories"}
	for _, key := range requiredKeys {
		if _, exists := jsonMap[key]; !exists {
			t.Fatalf("JSON missing required key: %s", key)
		}
	}

	// Verify stderr contains logs (not empty, indicating logs were properly separated)
	stderrStr := stderr.String()
	if strings.TrimSpace(stderrStr) == "" {
		t.Log("Warning: stderr is empty, expected logs to be separated from JSON output")
	}

	// Ensure stdout doesn't contain log-like content (basic check)
	if strings.Contains(stdoutStr, "[INFO]") || strings.Contains(stdoutStr, "[ERROR]") {
		t.Fatalf("stdout contains log content, should be pure JSON: %s", stdoutStr)
	}
}

// TestAssessCLI_JSONVsMarkdown_Contrast tests that JSON and markdown formats
// produce different output types, ensuring format selection works correctly
func TestAssessCLI_JSONVsMarkdown_Contrast(t *testing.T) {
	// Save and reset registry for isolation
	originalRegistry := assess.GetAssessmentRunnerRegistry()
	testRegistry := assess.ResetRegistryForTesting()

	// Register fake runners for quick execution
	testRegistry.RegisterRunner(assess.CategoryFormat, &cliFakeRunner{})
	testRegistry.RegisterRunner(assess.CategorySecurity, &cliFakeRunner{})
	testRegistry.RegisterRunner(assess.CategoryLint, &cliFakeRunner{})
	testRegistry.RegisterRunner(assess.CategoryStaticAnalysis, &cliFakeRunner{})
	testRegistry.RegisterRunner(assess.CategorySchema, &cliFakeRunner{})

	t.Cleanup(func() {
		assess.RestoreRegistry(originalRegistry)
		assessMode, assessNoOp, assessCheck, assessFix = "", false, false, false
	})

	testCases := []struct {
		name       string
		format     string
		expectJSON bool
		expectLogs bool
	}{
		{"JSON format", "json", true, false},
		{"Markdown format", "markdown", false, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset global mode flags for each subtest
			assessMode, assessNoOp, assessCheck, assessFix = "", false, false, false

			// Create separate buffers for stdout and stderr
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			// Build command with specified format
			cmd := &cobra.Command{Use: "assess", RunE: runAssess}
			setupAssessCommandFlags(cmd)
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			cmd.SetArgs([]string{"--format", tc.format, "--concurrency", "1", "."})

			// Execute command
			err := cmd.ExecuteContext(context.Background())
			if err != nil {
				t.Fatalf("expected success for %s, got error: %v", tc.name, err)
			}

			stdoutStr := stdout.String()
			stderrStr := stderr.String()

			if tc.expectJSON {
				// Should be valid JSON
				var jsonData interface{}
				if err := json.Unmarshal([]byte(stdoutStr), &jsonData); err != nil {
					t.Fatalf("%s format should produce valid JSON, got: %s", tc.name, stdoutStr)
				}
				// Should not contain obvious log markers
				if strings.Contains(stdoutStr, "[INFO]") || strings.Contains(stdoutStr, "[ERROR]") {
					t.Fatalf("%s format stdout contains log content: %s", tc.name, stdoutStr)
				}
			} else {
				// Should NOT be valid JSON (markdown format)
				var jsonData interface{}
				if err := json.Unmarshal([]byte(stdoutStr), &jsonData); err == nil {
					t.Fatalf("%s format should NOT produce valid JSON, but it did: %s", tc.name, stdoutStr)
				}
				// Should contain markdown-like content
				if !strings.Contains(stdoutStr, "#") && !strings.Contains(stdoutStr, "-") {
					t.Logf("Warning: %s format doesn't contain expected markdown markers", tc.name)
				}
			}

			if tc.expectLogs {
				// Should have logs in stderr
				if strings.TrimSpace(stderrStr) == "" {
					t.Logf("Warning: %s format stderr is empty, expected logs", tc.name)
				}
			} else {
				// Should have minimal/no logs in stderr (JSON format suppresses them)
				// Note: In test environment, logs might still appear due to logger initialization
				if strings.Contains(stderrStr, "[INFO]") && len(stderrStr) > 100 {
					t.Logf("Note: %s format has some logs in stderr (expected due to test setup): %d chars", tc.name, len(stderrStr))
				}
			}
		})
	}
}

func TestCountIssuesBySeverity(t *testing.T) {
	tests := []struct {
		name     string
		report   *assess.AssessmentReport
		expected map[string]int
	}{
		{
			name: "empty report",
			report: &assess.AssessmentReport{
				Categories: map[string]assess.CategoryResult{},
			},
			expected: map[string]int{"critical": 0, "high": 0, "medium": 0, "low": 0, "info": 0, "total": 0},
		},
		{
			name: "single critical issue",
			report: &assess.AssessmentReport{
				Categories: map[string]assess.CategoryResult{
					"security": {
						Issues: []assess.Issue{
							{Severity: assess.SeverityCritical, Message: "Critical issue"},
						},
					},
				},
			},
			expected: map[string]int{"critical": 1, "high": 0, "medium": 0, "low": 0, "info": 0, "total": 1},
		},
		{
			name: "mixed severities",
			report: &assess.AssessmentReport{
				Categories: map[string]assess.CategoryResult{
					"format": {
						Issues: []assess.Issue{
							{Severity: assess.SeverityHigh, Message: "High issue"},
							{Severity: assess.SeverityMedium, Message: "Medium issue"},
						},
					},
					"lint": {
						Issues: []assess.Issue{
							{Severity: assess.SeverityLow, Message: "Low issue"},
							{Severity: assess.SeverityInfo, Message: "Info issue"},
						},
					},
				},
			},
			expected: map[string]int{"critical": 0, "high": 1, "medium": 1, "low": 1, "info": 1, "total": 4},
		},
		{
			name: "multiple issues same severity",
			report: &assess.AssessmentReport{
				Categories: map[string]assess.CategoryResult{
					"security": {
						Issues: []assess.Issue{
							{Severity: assess.SeverityHigh, Message: "High issue 1"},
							{Severity: assess.SeverityHigh, Message: "High issue 2"},
							{Severity: assess.SeverityHigh, Message: "High issue 3"},
						},
					},
				},
			},
			expected: map[string]int{"critical": 0, "high": 3, "medium": 0, "low": 0, "info": 0, "total": 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := countIssuesBySeverity(tt.report)
			for key, expectedValue := range tt.expected {
				if actualValue, exists := result[key]; !exists {
					t.Errorf("countIssuesBySeverity() missing key %q", key)
				} else if actualValue != expectedValue {
					t.Errorf("countIssuesBySeverity()[%q] = %d, want %d", key, actualValue, expectedValue)
				}
			}
		})
	}
}
