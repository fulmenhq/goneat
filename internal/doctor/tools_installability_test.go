package doctor

import (
	"runtime"
	"testing"
)

// installerKinds mirrors the internal installerKindLookup for validation purposes.
var installerKinds = map[string]bool{
	"bun":        true,
	"mise":       true,
	"brew":       true,
	"scoop":      true,
	"winget":     true,
	"pacman":     true,
	"apt":        true,
	"apt-get":    true,
	"dnf":        true,
	"yum":        true,
	"npm":        true,
	"go-install": true,
	"manual":     true,
	"builtin":    true,
}

// TestDefaultConfigInstallability ensures the repo's default tools.yaml declares at least one installer
// for every supported platform per tool, and that Go tools include go-install in their priorities.
func TestDefaultConfigInstallability(t *testing.T) {
	t.Parallel()
	cfg, err := LoadToolsConfig()
	if err != nil {
		t.Fatalf("failed to load tools config: %v", err)
	}

	platforms := []string{"darwin", "linux", "windows"}

	for name, tool := range cfg.Tools {
		targetPlatforms := tool.Platforms
		if len(targetPlatforms) == 0 {
			targetPlatforms = platforms
		}

		for _, platform := range targetPlatforms {
			priorities := installerPrioritiesForPlatform(tool, platform)
			if len(priorities) == 0 {
				t.Fatalf("tool %s missing installer priority for platform %s (kind=%s)", name, platform, tool.Kind)
			}

			for _, p := range priorities {
				if !installerKinds[p] {
					t.Fatalf("tool %s uses unknown installer kind %q on platform %s", name, p, platform)
				}
			}

			if tool.Kind == "go" {
				if !contains(priorities, "go-install") {
					t.Fatalf("go tool %s missing go-install in priorities for platform %s", name, platform)
				}
			}
		}
	}

	// Sanity check that current platform is covered for all foundation tools.
	current := runtime.GOOS
	for _, toolName := range cfg.Scopes["foundation"].Tools {
		tool := cfg.Tools[toolName]
		priorities := installerPrioritiesForPlatform(tool, current)
		if len(priorities) == 0 {
			t.Fatalf("foundation tool %s missing installer priority for current platform %s", toolName, current)
		}
	}
}

func installerPrioritiesForPlatform(tool ToolConfig, platform string) []string {
	var priorities []string
	if p, ok := tool.InstallerPriority[platform]; ok {
		priorities = append(priorities, p...)
	}
	if p, ok := tool.InstallerPriority["all"]; ok {
		priorities = append(priorities, p...)
	}
	return priorities
}

func contains(list []string, target string) bool {
	for _, item := range list {
		if item == target {
			return true
		}
	}
	return false
}
