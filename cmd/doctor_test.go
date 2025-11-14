package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helper to execute the doctor tools subcommand with given args and capture error/output
func execDoctorTools(t *testing.T, args []string) (string, error) {
	t.Helper()

	// Reset package-level flag variables to prevent state pollution between tests
	// These are bound to flag vars and persist across test runs
	flagDoctorInstall = false
	flagDoctorAll = false
	flagDoctorTools = nil
	flagDoctorPrintInstructions = false
	flagDoctorYes = false
	flagDoctorScope = "security" // default value
	flagDoctorCheckUpdates = false
	flagDoctorInstallPkgMgr = false
	flagDoctorConfig = ""
	flagDoctorListScopes = false
	flagDoctorValidateConfig = false
	flagDoctorDryRun = false
	flagDoctorNoCooling = false

	// Create a fresh root command instance per test to prevent command tree pollution
	// This ensures each test runs with a clean command tree
	cmd := newRootCommand()

	// Register all subcommands (doctor, etc.)
	registerSubcommands(cmd)

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	// Execute via the command path: goneat doctor tools ...
	rootArgs := append([]string{"doctor", "tools"}, args...)
	cmd.SetArgs(rootArgs)

	// Do not prompt in tests; we only exercise validation and printing paths
	err := cmd.Execute()

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

// TestDoctorTools_ManualInstallerBootstrap tests that manual installer works for bootstrap scenarios
// This test addresses the bug where installerManual was never executed because isInstallerAvailable
// returned false. Manual installers are used for bootstrapping package managers (mise, scoop) via
// official install scripts.
func TestDoctorTools_ManualInstallerBootstrap(t *testing.T) {
	// Create a temporary tools config with a manual installer
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "tools-test-manual.yaml")

	// Write a simple config with a tool that uses manual installer
	// We use a fake tool that writes a marker file when "installed"
	markerFile := filepath.Join(tmpDir, "installed.marker")
	configContent := fmt.Sprintf(`
scopes:
  test-bootstrap:
    description: "Test bootstrap scope"
    tools: ["test-manual-tool"]

tools:
  test-manual-tool:
    name: "test-manual-tool"
    description: "Test tool for manual installer"
    kind: "system"
    detect_command: "test-manual-tool --version"
    platforms: ["linux", "darwin", "windows"]
    installer_priority:
      linux: ["manual"]
      darwin: ["manual"]
      windows: ["manual"]
    install_commands:
      manual: "echo 'Manual install executed' > %s"
`, markerFile)

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Run doctor tools with --dry-run to see what would be executed
	// (We use dry-run to avoid actually running install commands in tests)
	out, err := execDoctorTools(t, []string{"--config", configPath, "--scope", "test-bootstrap", "--dry-run"})

	// Command should succeed (or at least not panic)
	if err != nil && !strings.Contains(err.Error(), "missing") {
		t.Errorf("Manual installer bootstrap test failed unexpectedly: %v", err)
	}

	// Verify output contains dry-run results (not just scope listing)
	// Handle both JSON and text output formats
	hasDryRunOutput := strings.Contains(out, "would install") ||
		strings.Contains(out, "Dry run") ||
		strings.Contains(out, "would_install")

	if !hasDryRunOutput {
		t.Logf("Output: %s", out)
		t.Errorf("Expected dry-run output, but got scope listing. This suggests --dry-run flag was not processed.")
	}

	// Verify manual installer is mentioned in the output (text or JSON format)
	hasManualMention := strings.Contains(out, "Manual") ||
		strings.Contains(out, "manual")

	if hasDryRunOutput && !hasManualMention {
		t.Logf("Output: %s", out)
		t.Errorf("Expected output to mention 'Manual' installation, but it didn't.")
	}

	t.Logf("Manual installer bootstrap test completed. Output: %s", out)
}
