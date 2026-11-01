package dependencies

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// goCoolingFixture writes a dependency-free Go module and a policy file.
// The only module is the main module, so no registry or network access is needed.
func goCoolingFixture(t *testing.T, policy string) (target, policyPath string) {
	t.Helper()
	target = t.TempDir()
	files := map[string]string{
		"go.mod":  "module example.com/coolingfixture\n\ngo 1.26\n",
		"main.go": "package main\n\nfunc main() {}\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(target, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	policyPath = filepath.Join(target, "dependencies.yaml")
	if err := os.WriteFile(policyPath, []byte(policy), 0o600); err != nil {
		t.Fatal(err)
	}
	return target, policyPath
}

func analyzeGoCooling(t *testing.T, policy string) *AnalysisResult {
	t.Helper()
	target, policyPath := goCoolingFixture(t, policy)
	result, err := NewGoAnalyzer().Analyze(context.Background(), target, AnalysisConfig{
		Target:       target,
		PolicyPath:   policyPath,
		CheckCooling: true,
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	return result
}

// A cooling config error must reach the gate through GoAnalyzer.Analyze as a
// high-severity configuration issue, not be dropped.
func TestGoAnalyzer_Analyze_CoolingConfigErrorFailsGate(t *testing.T) {
	cases := map[string]string{
		"malformed until":           `until: "next tuesday"`,
		"unquoted unpadded until":   `until: 2099-1-5`,
		"unquoted rfc3339 until":    `until: 2099-12-31T00:00:00Z`,
		"quoted unpadded until":     `until: "2099-1-5"`,
		"unquoted time-of-day date": `until: 2099-12-31 12:00:00`,
		// Non-scalar and null values fail schema validation, which used to skip
		// cooling and pass.
		"list until":    `until: ["2099-01-05"]`,
		"block list":    "until:\n        - 2099-01-05",
		"mapping until": `until: {date: 2099-01-05}`,
		"block mapping": "until:\n        date: 2099-01-05",
		"null until":    `until: null`,
		"integer until": `until: 20990105`,
		"boolean until": `until: true`,
	}
	for name, untilLine := range cases {
		t.Run(name, func(t *testing.T) {
			result := analyzeGoCooling(t, `version: v1
cooling:
  enabled: true
  min_age_days: 7
  exceptions:
    - pattern: example.com/some-module
      reason: test
      `+untilLine+`
`)
			if result.Passed {
				t.Fatalf("a cooling config error must fail the gate, issues=%v", result.Issues)
			}
			var found bool
			for _, issue := range result.Issues {
				if issue.Type == "configuration" && issue.Severity == "high" {
					found = true
				}
			}
			if !found {
				t.Fatalf("expected a high-severity configuration issue, issues=%v", result.Issues)
			}
		})
	}
}

// Control: a valid policy passes, so the failures above come from the config error.
func TestGoAnalyzer_Analyze_ValidCoolingPolicyPasses(t *testing.T) {
	result := analyzeGoCooling(t, `version: v1
cooling:
  enabled: true
  min_age_days: 7
  exceptions:
    - pattern: example.com/some-module
      reason: test
      until: 2099-01-05
`)
	if !result.Passed {
		t.Fatalf("a valid policy should pass, issues=%v", result.Issues)
	}
	for _, issue := range result.Issues {
		if issue.Type == "configuration" {
			t.Fatalf("unexpected configuration issue: %+v", issue)
		}
	}
}
