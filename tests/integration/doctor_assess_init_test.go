package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func goneatBinaryPathForIntegration() string {
	candidates := []string{
		filepath.Join(".", "dist", "goneat"),
		filepath.Join("..", "..", "dist", "goneat"),
	}
	if runtime.GOOS == "windows" {
		for i := range candidates {
			candidates[i] += ".exe"
		}
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return candidates[0]
}

func TestDoctorAssessInitWorkflow(t *testing.T) {
	binPath := goneatBinaryPathForIntegration()
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		t.Skip("goneat binary not found, skipping integration test")
	}

	tmpDir := t.TempDir()
	cmd := exec.Command(binPath, "doctor", "assess", "init", "--language", "go", "--force")
	cmd.Dir = tmpDir

	output, err := cmd.CombinedOutput()
	outputStr := string(output)
	if err != nil {
		t.Fatalf("doctor assess init should not fail: %s", outputStr)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, ".goneat", "assess.yaml"))
	if err != nil {
		t.Fatalf("expected .goneat/assess.yaml to exist: %v", err)
	}

	if !strings.Contains(string(data), "# Repo type: go") {
		t.Fatalf("expected go template marker, got:\n%s", string(data))
	}
}
