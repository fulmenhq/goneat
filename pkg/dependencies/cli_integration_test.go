//go:build integration
// +build integration

package dependencies

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCLIIntegration_ForbiddenLicense(t *testing.T) {
	// Ensure goneat binary is built
	goneatBinary := "../../dist/goneat"
	if _, err := os.Stat(goneatBinary); err != nil {
		t.Skip("goneat binary not found, run 'make build' first")
	}

	// Create temporary policy file forbidding MIT licenses
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "forbidden-mit.yaml")
	policyContent := `version: v1
licenses:
  forbidden:
    - MIT
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0644); err != nil {
		t.Fatalf("Failed to create policy file: %v", err)
	}

	// Run goneat dependencies command
	cmd := exec.Command(goneatBinary, "dependencies", "--licenses", "--policy", policyPath, "../..")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		// Command should fail due to policy violations
		t.Logf("Command failed as expected (policy violation)")
	}

	// Parse JSON output from stdout only
	var result AnalysisResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nStdout: %s\nStderr: %s", err, stdout.String(), stderr.String())
	}

	// Verify that Passed is false (MIT licenses are forbidden)
	if result.Passed {
		t.Error("Expected Passed=false when MIT licenses are forbidden")
	}

	// Verify we have license issues
	licenseIssues := 0
	for _, issue := range result.Issues {
		if issue.Type == "license" {
			licenseIssues++
		}
	}

	if licenseIssues == 0 {
		t.Error("Expected license issues but found none")
	}

	t.Logf("Successfully detected %d MIT license violations", licenseIssues)
}

func TestCLIIntegration_CoolingPolicy(t *testing.T) {
	// Ensure goneat binary is built
	goneatBinary := "../../dist/goneat"
	if _, err := os.Stat(goneatBinary); err != nil {
		t.Skip("goneat binary not found, run 'make build' first")
	}

	// Create temporary policy file with aggressive cooling policy
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "cooling-strict.yaml")
	policyContent := `version: v1
cooling:
  enabled: true
  min_age_days: 1000  # Very aggressive to trigger violations
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0644); err != nil {
		t.Fatalf("Failed to create policy file: %v", err)
	}

	// Run goneat dependencies command
	cmd := exec.Command(goneatBinary, "dependencies", "--licenses", "--policy", policyPath, "../..")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		t.Logf("Command failed as expected (cooling violations)")
	}

	// Parse JSON output from stdout only
	var result AnalysisResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nStdout: %s\nStderr: %s", err, stdout.String(), stderr.String())
	}

	// Verify that Passed is false (packages are too young)
	if result.Passed {
		t.Error("Expected Passed=false with 1000-day cooling policy")
	}

	// Verify we have cooling issues
	coolingIssues := 0
	for _, issue := range result.Issues {
		if issue.Type == "cooling" {
			coolingIssues++
		}
	}

	if coolingIssues == 0 {
		t.Error("Expected cooling policy violations but found none")
	}

	t.Logf("Successfully detected %d cooling policy violations", coolingIssues)
}

func TestCLIIntegration_ValidPolicy(t *testing.T) {
	// Ensure goneat binary is built
	goneatBinary := "../../dist/goneat"
	if _, err := os.Stat(goneatBinary); err != nil {
		t.Skip("goneat binary not found, run 'make build' first")
	}

	// Create temporary policy file that allows all our licenses
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "permissive.yaml")
	policyContent := `version: v1
licenses:
  forbidden:
    - AGPL-3.0  # We don't use this
cooling:
  enabled: false  # Disable cooling for this test
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0644); err != nil {
		t.Fatalf("Failed to create policy file: %v", err)
	}

	// Run goneat dependencies command
	cmd := exec.Command(goneatBinary, "dependencies", "--licenses", "--policy", policyPath, "../..")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		t.Fatalf("Command failed unexpectedly: %v\nStdout: %s\nStderr: %s", err, stdout.String(), stderr.String())
	}

	// Parse JSON output from stdout only
	var result AnalysisResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nStdout: %s\nStderr: %s", err, stdout.String(), stderr.String())
	}

	// Verify that Passed is true (permissive policy)
	if !result.Passed {
		t.Errorf("Expected Passed=true with permissive policy, got %d issues", len(result.Issues))
		for i, issue := range result.Issues {
			if i < 5 { // Show first 5 issues
				t.Logf("Issue %d: %s - %s", i+1, issue.Type, issue.Message)
			}
		}
	}

	// Verify we detected dependencies
	if len(result.Dependencies) == 0 {
		t.Error("Expected dependencies but found none")
	}

	t.Logf("Successfully analyzed %d dependencies with permissive policy", len(result.Dependencies))
}

func TestCLIIntegration_NoPolicy(t *testing.T) {
	// Ensure goneat binary is built
	goneatBinary := "../../dist/goneat"
	if _, err := os.Stat(goneatBinary); err != nil {
		t.Skip("goneat binary not found, run 'make build' first")
	}

	// Run goneat dependencies command without policy
	cmd := exec.Command(goneatBinary, "dependencies", "--licenses", "../..")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		t.Fatalf("Command failed: %v\nStdout: %s\nStderr: %s", err, stdout.String(), stderr.String())
	}

	// Parse JSON output from stdout only
	var result AnalysisResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nStdout: %s\nStderr: %s", err, stdout.String(), stderr.String())
	}

	// Without policy, should pass (no violations)
	if !result.Passed {
		t.Error("Expected Passed=true when no policy is specified")
	}

	// Should still detect dependencies
	if len(result.Dependencies) == 0 {
		t.Error("Expected dependencies but found none")
	}

	// Verify license detection worked
	knownLicenses := 0
	for _, dep := range result.Dependencies {
		if dep.License != nil && dep.License.Type != "Unknown" {
			knownLicenses++
		}
	}

	if knownLicenses == 0 {
		t.Error("Expected some licenses to be detected")
	}

	detectionRate := float64(knownLicenses) / float64(len(result.Dependencies)) * 100
	t.Logf("License detection rate: %.1f%% (%d/%d)", detectionRate, knownLicenses, len(result.Dependencies))

	if detectionRate < 90.0 {
		t.Errorf("License detection rate %.1f%% is below 90%% threshold", detectionRate)
	}
}
