package integration

import (
	"os/exec"
	"strings"
	"testing"
)

// TestSecurity_Gitleaks_Smoke runs goneat security with gitleaks when available.
// Skips if gitleaks binary is not present in PATH.
func TestSecurity_Gitleaks_Smoke(t *testing.T) {
	if _, err := exec.LookPath("gitleaks"); err != nil {
		t.Skip("gitleaks not installed; skipping smoke test")
	}

	env := NewTestEnv(t)
	// Create a benign file; we only smoke test execution and JSON output shape
	env.WriteFile("README.md", "# test repo\n")

	goneatPath := env.findGoneatBinary()
	if goneatPath == "" {
		t.Skip("goneat binary not found; skipping")
	}

	// Run: goneat security --enable secrets --tools gitleaks --format json
	cmd := exec.Command(goneatPath, "security", "--enable", "secrets", "--tools", "gitleaks", "--format", "json")
	cmd.Dir = env.Dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("goneat security with gitleaks failed: %v\nOutput: %s", err, string(out))
	}
	s := strings.TrimSpace(string(out))
	if !strings.HasPrefix(s, "{") || !strings.Contains(s, "\"categories\"") {
		t.Fatalf("expected JSON output containing categories; got: %s", s)
	}
}
