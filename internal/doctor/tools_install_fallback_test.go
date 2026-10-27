package doctor

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// TestInstallSystemTool_BrewErrorFallsBackToNonBrew fails if a stubbed brew
// installer error aborts the run before uv/pip/go-install fallbacks run.
func TestInstallSystemTool_BrewErrorFallsBackToNonBrew(t *testing.T) {
	t.Parallel()

	tool := Tool{
		Name:           "ruff-standin",
		Kind:           "python",
		DetectCommand:  "ruff-standin --version",
		InstallPackage: "ruff-standin",
		InstallerPriority: map[string][]string{
			"all": {"brew", "uv", "pip", "go-install"},
		},
		InstallCommands: map[string]string{
			"brew":       "brew install ruff-standin",
			"uv":         "uv tool install ruff-standin",
			"pip":        "pip install ruff-standin",
			"go-install": "go install example.com/ruff-standin@latest",
		},
	}

	if kinds := resolveInstallerPriority(tool, "linux"); !containsKind(kinds, installerUv) || !containsKind(kinds, installerPip) {
		t.Fatalf("installer priority dropped non-brew fallbacks: %v", kinds)
	}

	var ran []string
	available := func(kind installerKind) bool {
		switch kind {
		case installerBrew, installerUv, installerPip, installerGoInstall:
			return true
		default:
			return false
		}
	}
	run := func(command string) (string, int, error) {
		ran = append(ran, command)
		if strings.Contains(command, "brew ") {
			return "Error: Homebrew's master branch is no longer supported.", 1, errors.New("brew failed")
		}
		if strings.Contains(command, "uv tool install") {
			return "installed", 0, nil
		}
		return "unexpected installer " + command, 1, fmt.Errorf("unexpected installer %q", command)
	}

	status := installSystemToolWith(tool, run, available)
	if len(ran) < 2 {
		t.Fatalf("expected brew then a non-brew fallback, ran %v", ran)
	}
	if !strings.Contains(ran[0], "brew install") {
		t.Fatalf("first installer = %q, want brew", ran[0])
	}
	if !strings.Contains(ran[1], "uv tool install") {
		t.Fatalf("fallback installer = %q, want uv after brew error; ran %v", ran[1], ran)
	}
	for _, cmd := range ran[1:] {
		if strings.Contains(cmd, "brew ") {
			t.Fatalf("brew was retried after failure: %v", ran)
		}
	}
	if !status.Installed {
		t.Fatalf("fallback install should mark Installed; status=%+v ran=%v", status, ran)
	}
}

func TestInstallSystemTool_BrewOnlyRequiredUnavailable(t *testing.T) {
	t.Parallel()

	tool := Tool{
		Name:          "ripgrep-standin",
		Kind:          "system",
		DetectCommand: "rg-standin --version",
		InstallerPriority: map[string][]string{
			"all": {"brew"},
		},
		InstallCommands: map[string]string{
			"brew": "brew install ripgrep-standin",
		},
	}

	var ran []string
	run := func(command string) (string, int, error) {
		ran = append(ran, command)
		return "brew bootstrap failed", 1, errors.New("brew failed")
	}
	available := func(kind installerKind) bool {
		return kind == installerBrew
	}

	status := installSystemToolWith(tool, run, available)
	if status.Error == nil {
		t.Fatal("required brew-only tool must not silently succeed when brew fails")
	}
	if !CountsTowardMissing(tool, status) {
		t.Fatal("required brew-only tool must still fail the doctor run")
	}
	blob := status.Error.Error() + "\n" + status.Instructions
	if !strings.Contains(blob, "brew") {
		t.Fatalf("missing brew hint in error/instructions: %s", blob)
	}

	unavailable := installSystemToolWith(tool, func(string) (string, int, error) {
		t.Fatal("brew-only unavailable tool should not execute an installer")
		return "", 0, nil
	}, func(installerKind) bool { return false })
	if unavailable.Error == nil {
		t.Fatal("required brew-only tool must error when brew is unavailable")
	}
	if !strings.Contains(unavailable.Error.Error(), "Homebrew is not installed") {
		t.Fatalf("want Homebrew unavailable hint, got %v", unavailable.Error)
	}
	if !CountsTowardMissing(tool, unavailable) {
		t.Fatal("required brew-only unavailable tool must count as missing")
	}
}

func TestInstallSystemTool_ReportOnlyDoesNotRunOrFail(t *testing.T) {
	t.Parallel()

	tool := Tool{
		Name:          "minisign-standin",
		Kind:          "system",
		DetectCommand: "minisign-standin -v",
		Platforms:     []string{"linux", "darwin", "windows"},
		InstallerPriority: map[string][]string{
			"all": {"manual"},
		},
		InstallCommands: map[string]string{
			"manual": "echo 'Ubuntu/Debian: sudo apt-get install -y minisign-standin' && exit 1",
			"brew":   "brew install minisign-standin",
		},
	}

	if !IsReportOnly(tool) {
		t.Fatal("manual-only tool should be report-only on this platform")
	}

	ran := 0
	status := installSystemToolWith(tool, func(command string) (string, int, error) {
		ran++
		return command, 1, errors.New("should not run")
	}, func(installerKind) bool { return true })
	if ran != 0 {
		t.Fatalf("report-only installer ran %d time(s)", ran)
	}
	if status.Error != nil {
		t.Fatalf("report-only must not fail install: %v", status.Error)
	}
	if status.Present || status.Installed {
		t.Fatalf("report-only must not claim success: %+v", status)
	}
	if status.Instructions == "" {
		t.Fatal("report-only must print an install hint")
	}
	if !strings.Contains(status.Instructions, "minisign-standin") {
		t.Fatalf("instructions missing tool hint: %s", status.Instructions)
	}
	if CountsTowardMissing(tool, status) {
		t.Fatal("report-only tools must not fail the doctor run")
	}
}

func TestLinuxMinisignReportOnlyDoesNotCountAsMissing(t *testing.T) {
	t.Parallel()
	cfg, err := LoadToolsConfig()
	if err != nil {
		t.Fatalf("LoadToolsConfig: %v", err)
	}
	ms, ok := cfg.Tools["minisign"]
	if !ok {
		t.Fatal("minisign missing from tools.yaml")
	}
	tool := Tool{
		Name:              ms.Name,
		Kind:              ms.Kind,
		DetectCommand:     ms.DetectCommand,
		Platforms:         ms.Platforms,
		InstallCommands:   ms.InstallCommands,
		InstallerPriority: ms.InstallerPriority,
	}
	if getCurrentPlatform() != "linux" {
		t.Skip("linux minisign report-only is the bootstrap regression")
	}
	if !IsReportOnly(tool) {
		t.Fatalf("linux minisign installer_priority = %v, want report-only manual", ms.InstallerPriority["linux"])
	}
	status := InstallTool(tool)
	if status.Error != nil {
		t.Fatalf("linux minisign report-only must not execute a failing installer: %v", status.Error)
	}
	if CountsTowardMissing(tool, status) {
		t.Fatal("linux minisign must not fail make bootstrap when absent")
	}
}

func containsKind(kinds []installerKind, want installerKind) bool {
	for _, kind := range kinds {
		if kind == want {
			return true
		}
	}
	return false
}
