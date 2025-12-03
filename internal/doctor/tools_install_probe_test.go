//go:build installprobe

package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestFoundationToolInstallabilityProbe performs a lightweight, best-effort probe that the
// declared installer priorities can actually resolve packages on the current platform.
//
// Enable with: GONEAT_INSTALL_PROBE=1 go test -tags=installprobe ./internal/doctor
//
// Notes:
// - Non-destructive: uses package-manager "info/show" style commands, never installs.
// - Skips when package manager is absent on PATH.
// - Only checks tools in the foundation scope that apply to the current platform.
func TestFoundationToolInstallabilityProbe(t *testing.T) {
	t.Parallel()
	if os.Getenv("GONEAT_INSTALL_PROBE") != "1" {
		t.Skip("set GONEAT_INSTALL_PROBE=1 to run installability probes")
	}

	cfg, err := LoadToolsConfig()
	if err != nil {
		t.Fatalf("failed to load tools config: %v", err)
	}

	scope, ok := cfg.Scopes["foundation"]
	if !ok {
		t.Fatalf("foundation scope missing from tools config")
	}

	platform := runtime.GOOS
	for _, toolName := range scope.Tools {
		tool := cfg.Tools[toolName]
		if !toolSupportsPlatform(tool, platform) {
			continue
		}

		// Skip Go-kind tools (would require network/download); they are covered by schema + go-install presence.
		if tool.Kind == "go" || tool.Kind == "bundled-go" {
			t.Logf("skip go-kind tool probe: %s", toolName)
			continue
		}

		priorities := installerPrioritiesForPlatform(tool, platform)
		if len(priorities) == 0 {
			t.Fatalf("tool %s has no installer priority for platform %s", toolName, platform)
		}

		probed := false
		for _, installer := range priorities {
			cmd := probeCommand(installer, tool.Name)
			if len(cmd) == 0 {
				continue
			}

			if _, err := exec.LookPath(cmd[0]); err != nil {
				t.Logf("skip %s via %s: %s not available on PATH", toolName, installer, cmd[0])
				continue
			}

			probed = true
			output, err := runWithTimeout(cmd, 20*time.Second)
			if err != nil {
				t.Fatalf("installer probe failed for %s via %s: %v\noutput:\n%s", toolName, installer, err, output)
			}
			t.Logf("installer probe passed for %s via %s", toolName, installer)
			break
		}

		if !probed {
			t.Logf("no installer probe executed for %s on %s (no supported installer available); ensure CI auto-installs required manager", toolName, platform)
		}
	}
}

func probeCommand(installer, pkg string) []string {
	switch strings.ToLower(strings.TrimSpace(installer)) {
	case "brew":
		return []string{"brew", "info", pkg}
	case "scoop":
		return []string{"scoop", "info", pkg}
	case "winget":
		return []string{"winget", "show", pkg}
	case "bun":
		// Bun uses npm semantics; `bun pm ls <pkg>` is a lightweight metadata call.
		return []string{"bun", "pm", "ls", pkg}
	default:
		return nil
	}
}

func runWithTimeout(cmd []string, timeout time.Duration) (string, error) {
	c := exec.Command(cmd[0], cmd[1:]...) // #nosec G204 - commands are internal/probe-only
	done := make(chan struct{})
	var output []byte
	var err error

	go func() {
		output, err = c.CombinedOutput()
		close(done)
	}()

	select {
	case <-done:
		return string(output), err
	case <-time.After(timeout):
		_ = c.Process.Kill()
		return string(output), fmt.Errorf("command timed out after %s", timeout)
	}
}

func toolSupportsPlatform(tool ToolConfig, platform string) bool {
	if len(tool.Platforms) == 0 {
		return true
	}
	for _, p := range tool.Platforms {
		if strings.EqualFold(strings.TrimSpace(p), platform) || strings.EqualFold(strings.TrimSpace(p), "all") {
			return true
		}
	}
	return false
}
