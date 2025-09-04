package cmd

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/3leaps/goneat/internal/assess"
)

// minimal fake runner for CLI tests (no external tools)
type cliFakeRunner struct{}

func (c *cliFakeRunner) Assess(ctx context.Context, target string, cfg assess.AssessmentConfig) (*assess.AssessmentResult, error) {
	return &assess.AssessmentResult{
		CommandName:   "format",
		Category:      assess.CategoryFormat,
		Success:       true,
		ExecutionTime: 5 * time.Millisecond,
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
		ExecutionTime: c.delay,
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
	old := assess.GetAssessmentRunnerRegistry()
	assess.RegisterAssessmentRunner(assess.CategoryFormat, &cliFakeRunner{})
	t.Cleanup(func() { _ = old })

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
			buf := new(bytes.Buffer)
			assessCmd.SetOut(buf)
			assessCmd.SetErr(buf)
			assessCmd.SetArgs(tc.args)

			err := assessCmd.ExecuteContext(context.Background())
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
	old := assess.GetAssessmentRunnerRegistry()

	// Create a runner that returns high severity issues
	highSeverityRunner := &configurableFakeRunner{
		category: assess.CategorySecurity,
		issues: []assess.Issue{{
			File: "test.go", Line: 1, Severity: assess.SeverityHigh,
			Message: "test issue", Category: assess.CategorySecurity,
		}},
		available: true,
	}

	assess.RegisterAssessmentRunner(assess.CategorySecurity, highSeverityRunner)
	t.Cleanup(func() { _ = old })

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
			buf := new(bytes.Buffer)
			assessCmd.SetOut(buf)
			assessCmd.SetErr(buf)
			assessCmd.SetArgs([]string{"--fail-on", tc.failOn, "--categories", "security", "--concurrency", "1", "."})

			err := assessCmd.ExecuteContext(context.Background())

			if tc.shouldFail {
				// Should exit with code 1, which is returned as error
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
	buf := new(bytes.Buffer)
	assessCmd.SetOut(buf)
	assessCmd.SetErr(buf)
	assessCmd.SetArgs([]string{"/nonexistent/path"})

	err := assessCmd.ExecuteContext(context.Background())
	if err == nil {
		t.Fatalf("expected error for nonexistent target directory")
	}
	if !strings.Contains(err.Error(), "target directory does not exist") {
		t.Fatalf("expected 'target directory does not exist' error, got: %v", err)
	}
}

func TestAssessCLI_CustomPriorities(t *testing.T) {
	old := assess.GetAssessmentRunnerRegistry()
	assess.RegisterAssessmentRunner(assess.CategoryFormat, &cliFakeRunner{})
	assess.RegisterAssessmentRunner(assess.CategoryLint, &cliFakeRunner{})
	t.Cleanup(func() { _ = old })

	buf := new(bytes.Buffer)
	assessCmd.SetOut(buf)
	assessCmd.SetErr(buf)
	assessCmd.SetArgs([]string{"--priority", "lint=1,format=2", "--format", "json", "--concurrency", "1", "."})

	if err := assessCmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
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
