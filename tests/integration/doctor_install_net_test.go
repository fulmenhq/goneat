package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestDoctorInstall_Gitleaks_Network attempts to install gitleaks via doctor when
// GONEAT_INTEGRATION_NET=1 is set. Skips if Go toolchain not available.
func TestDoctorInstall_Gitleaks_Network(t *testing.T) {
	if os.Getenv("GONEAT_INTEGRATION_NET") != "1" {
		t.Skip("networked doctor install tests disabled (set GONEAT_INTEGRATION_NET=1 to enable)")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("Go toolchain not found; skipping doctor install")
	}

	env := NewTestEnv(t)
	goneatPath := env.findGoneatBinary()
	if goneatPath == "" {
		t.Skip("goneat binary not found; skipping")
	}

	// First, print instructions to confirm tool is recognized
	cmd1 := exec.Command(goneatPath, "doctor", "tools", "--tools", "gitleaks", "--print-instructions")
	cmd1.Dir = env.Dir
	_, _ = cmd1.CombinedOutput() // best-effort

	// Attempt install (non-interactive)
	cmd := exec.Command(goneatPath, "doctor", "tools", "--tools", "gitleaks", "--install", "--yes")
	cmd.Dir = env.Dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("doctor install failed: %v\nOutput:\n%s", err, string(out))
	}

	// Validate presence either in PATH or typical Go bin locations
	if _, err := exec.LookPath("gitleaks"); err != nil {
		// Look in GOBIN or GOPATH/bin
		if bin := os.Getenv("GOBIN"); bin != "" {
			if _, statErr := os.Stat(filepath.Join(bin, exe("gitleaks"))); statErr == nil {
				return
			}
		}
		if gp, _ := exec.Command("go", "env", "GOPATH").Output(); len(gp) > 0 {
			root := strings.TrimSpace(string(gp))
			if _, statErr := os.Stat(filepath.Join(root, "bin", exe("gitleaks"))); statErr == nil {
				return
			}
		}
		t.Fatalf("gitleaks not found after install")
	}
}

func exe(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

// TestDoctorInstall_Gosec_Network attempts to install gosec via doctor when network tests are enabled.
func TestDoctorInstall_Gosec_Network(t *testing.T) {
	if os.Getenv("GONEAT_INTEGRATION_NET") != "1" {
		t.Skip("networked doctor install tests disabled (set GONEAT_INTEGRATION_NET=1 to enable)")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("Go toolchain not found; skipping doctor install")
	}
	env := NewTestEnv(t)
	goneatPath := env.findGoneatBinary()
	if goneatPath == "" {
		t.Skip("goneat binary not found; skipping")
	}
	cmd := exec.Command(goneatPath, "doctor", "tools", "--tools", "gosec", "--install", "--yes")
	cmd.Dir = env.Dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("doctor install failed for gosec: %v\nOutput:\n%s", err, string(out))
	}
	if _, err := exec.LookPath("gosec"); err != nil {
		if bin := os.Getenv("GOBIN"); bin != "" {
			if _, statErr := os.Stat(filepath.Join(bin, exe("gosec"))); statErr == nil {
				return
			}
		}
		if gp, _ := exec.Command("go", "env", "GOPATH").Output(); len(gp) > 0 {
			root := strings.TrimSpace(string(gp))
			if _, statErr := os.Stat(filepath.Join(root, "bin", exe("gosec"))); statErr == nil {
				return
			}
		}
		t.Fatalf("gosec not found after install")
	}
}

// TestDoctorInstall_Govulncheck_Network attempts to install govulncheck via doctor when network tests are enabled.
func TestDoctorInstall_Govulncheck_Network(t *testing.T) {
	if os.Getenv("GONEAT_INTEGRATION_NET") != "1" {
		t.Skip("networked doctor install tests disabled (set GONEAT_INTEGRATION_NET=1 to enable)")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("Go toolchain not found; skipping doctor install")
	}
	env := NewTestEnv(t)
	goneatPath := env.findGoneatBinary()
	if goneatPath == "" {
		t.Skip("goneat binary not found; skipping")
	}
	cmd := exec.Command(goneatPath, "doctor", "tools", "--tools", "govulncheck", "--install", "--yes")
	cmd.Dir = env.Dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("doctor install failed for govulncheck: %v\nOutput:\n%s", err, string(out))
	}
	if _, err := exec.LookPath("govulncheck"); err != nil {
		if bin := os.Getenv("GOBIN"); bin != "" {
			if _, statErr := os.Stat(filepath.Join(bin, exe("govulncheck"))); statErr == nil {
				return
			}
		}
		if gp, _ := exec.Command("go", "env", "GOPATH").Output(); len(gp) > 0 {
			root := strings.TrimSpace(string(gp))
			if _, statErr := os.Stat(filepath.Join(root, "bin", exe("govulncheck"))); statErr == nil {
				return
			}
		}
		t.Fatalf("govulncheck not found after install")
	}
}
