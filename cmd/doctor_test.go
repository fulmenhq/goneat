package cmd

import (
	"bytes"
	"strings"
	"testing"
)

// helper to execute the doctor tools subcommand with given args and capture error/output
func execDoctorTools(t *testing.T, args []string) (string, error) {
	t.Helper()

	// Execute via the real root command path: goneat doctor tools ...
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)
	rootArgs := append([]string{"doctor", "tools"}, args...)
	rootCmd.SetArgs(rootArgs)

	// Do not prompt in tests; we only exercise validation and printing paths
	err := rootCmd.Execute()
	return buf.String(), err
}

func TestDoctorTools_UnknownSingle(t *testing.T) {
	t.Skip("Skipping unknown tool validation in end-to-end Cobra path; behavior covered by internal catalog tests")
}

func TestDoctorTools_UnknownMultiple(t *testing.T) {
	t.Skip("Skipping unknown tool validation in end-to-end Cobra path; behavior covered by internal catalog tests")
}

func TestDoctorTools_KnownNames(t *testing.T) {
	// This test only asserts that the known tool names are accepted by flag parsing layer.
	// We do not assert presence/absence in PATH to keep tests environment-agnostic.
	out, err := execDoctorTools(t, []string{"--tools", "gosec,govulncheck,gitleaks", "--print-instructions"})
	// The command may return nil or an error depending on environment (missing tools cause non-zero).
	// Accept either, but ensure it did not error due to "unknown tool(s)".
	if err != nil && strings.Contains(err.Error(), "unknown tool(s)") {
		t.Fatalf("did not expect unknown tools error for known names; out=%q err=%v", out, err)
	}
}

func TestDoctorTools_KnownNames_Format(t *testing.T) {
	// Ensure format tool names are accepted now that doctor supports format scope.
	out, err := execDoctorTools(t, []string{"--tools", "goimports,gofmt", "--print-instructions"})
	if err != nil && strings.Contains(err.Error(), "unknown tool(s)") {
		t.Fatalf("did not expect unknown tools error for format tools; out=%q err=%v", out, err)
	}
}

// TestDoctorTools_PlatformFiltering tests that platform-specific tools are correctly filtered
// This test addresses a historical bug where Windows-only tools like "scoop" were reported
// as missing on macOS/Linux, causing false failures in multi-platform CI/CD pipelines.
// Platform-specific tools should be silently skipped on incompatible platforms.
func TestDoctorTools_PlatformFiltering(t *testing.T) {
	// Create a temporary config with platform-specific tools
	// Note: We can't easily create a temp config file in this test structure,
	// so we'll test by ensuring that built-in platform-specific tools don't cause failures

	// Test that known platform-specific tools are handled correctly
	// The actual tools checked depend on the embedded default configuration
	// We verify that the command doesn't fail due to platform filtering issues

	// Run doctor tools with a scope - should not fail due to platform-specific tools
	_, err := execDoctorTools(t, []string{"--scope", "security"})

	// The command may fail if tools are missing, but should NOT fail due to
	// platform-specific tools being checked on incompatible platforms
	// If the bug exists, we'd see errors like "1 tool(s) missing" for Windows tools on macOS

	if err != nil {
		errMsg := err.Error()
		// Check for specific bug symptoms that indicate platform filtering is broken
		if strings.Contains(strings.ToLower(errMsg), "scoop") {
			t.Errorf("Platform filtering bug detected: scoop (Windows-only) should not be checked on non-Windows platforms. Error: %v", err)
		}
		// Note: Other errors (like actual missing security tools) are acceptable for this test
		// We're specifically checking that platform-incompatible tools don't cause issues
	}

	t.Logf("Platform filtering test completed successfully")
}
