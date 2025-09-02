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
