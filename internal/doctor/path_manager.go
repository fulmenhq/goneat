package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/fulmenhq/goneat/pkg/logger"
	"github.com/fulmenhq/goneat/pkg/tools"
)

// PathManager handles PATH detection and extension for package manager shims
type PathManager struct {
	originalPATH string
	additions    []string
}

// NewPathManager creates a new PATH manager
func NewPathManager() *PathManager {
	return &PathManager{
		originalPATH: os.Getenv("PATH"),
		additions:    make([]string, 0),
	}
}

// GetShimPath returns the shim directory path for a given package manager
//
// LIMITATION (v0.3.10): Shim paths are hardcoded for known package managers.
// This does NOT read from foundation-package-managers.yaml config.
// Scope limited to: mise, bun, scoop, go-install, brew (well-known standard paths).
//
// See docs/sop/adding-package-manager-sop.md for the complete checklist when
// adding a new package manager - GetShimPath is one of several required touchpoints.
//
// TODO (v0.4.x): Read shim paths from config to support:
// - Custom shim locations (user-configured package manager install dirs)
// - New package managers without code changes
// - Platform-specific shim path variations
func GetShimPath(packageManager string) string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	// v0.3.7 scope: hardcoded well-known shim paths only
	switch packageManager {
	case "mise":
		// mise installs to ~/.local/share/mise/shims
		return filepath.Join(homeDir, ".local", "share", "mise", "shims")
	case "bun":
		// bun installs to ~/.bun/bin
		return filepath.Join(homeDir, ".bun", "bin")
	case "scoop":
		// scoop installs to ~/scoop/shims
		return filepath.Join(homeDir, "scoop", "shims")
	case "go-install":
		// go install uses GOBIN or ~/go/bin
		if goBin := os.Getenv("GOBIN"); goBin != "" {
			return goBin
		}
		return filepath.Join(homeDir, "go", "bin")
	case "brew":
		// brew location varies by platform - use DetectBrew to find it
		_, brewPath, err := tools.DetectBrew()
		if err == nil && brewPath != "" {
			return filepath.Dir(brewPath) // Return bin directory, not brew binary path
		}
		return ""
	default:
		return ""
	}
}

// GetRequiredPATHAdditions checks installed package managers and returns
// paths that should be added to PATH for tools to be accessible
func GetRequiredPATHAdditions(config *PackageManagersConfig) []string {
	if config == nil {
		return nil
	}

	var additions []string
	pathSet := make(map[string]bool)
	platform := runtime.GOOS

	for _, pm := range config.PackageManagers {
		if !pm.SupportsPlatform(platform) {
			continue
		}

		// Check if package manager is installed
		installed, _ := DetectPackageManager(&pm)
		if !installed {
			continue
		}

		// brew installs typically export PATH themselves, but CI runs benefit from explicit additions.
		if pm.Name == "brew" {
			if _, brewPath, err := tools.DetectBrew(); err == nil && brewPath != "" {
				binDir := filepath.Dir(brewPath)
				if _, err := os.Stat(binDir); err == nil && !pathSet[binDir] {
					additions = append(additions, binDir)
					pathSet[binDir] = true
					logger.Debug(fmt.Sprintf("Detected brew bin directory for PATH: %s", binDir))
				}
			}
		}

		// Check if package manager requires PATH update
		if !pm.RequiresPathUpdate {
			continue
		}

		// Get shim path for this package manager
		shimPath := GetShimPath(pm.Name)
		if shimPath == "" {
			continue
		}

		// Verify directory exists
		if _, err := os.Stat(shimPath); err == nil && !pathSet[shimPath] {
			additions = append(additions, shimPath)
			pathSet[shimPath] = true
			logger.Debug(fmt.Sprintf("Found package manager shim directory: %s (%s)", shimPath, pm.Name))
		}
	}

	return additions
}

// AddToSessionPATH temporarily extends PATH for goneat's subprocess execution
// This allows tools installed to shim directories to be immediately accessible
// without requiring user to update their shell profile
func (pm *PathManager) AddToSessionPATH(paths ...string) {
	if len(paths) == 0 {
		return
	}

	// Filter out paths already in PATH
	currentPaths := strings.Split(pm.originalPATH, string(os.PathListSeparator))
	pathSet := make(map[string]bool)
	for _, p := range currentPaths {
		pathSet[p] = true
	}

	for _, path := range paths {
		if pathSet[path] {
			logger.Debug(fmt.Sprintf("Path already in PATH, skipping: %s", path))
			continue
		}
		pm.additions = append(pm.additions, path)
		pathSet[path] = true
	}

	if len(pm.additions) == 0 {
		return
	}

	// Build new PATH with additions at the front
	newPATH := strings.Join(append(pm.additions, currentPaths...), string(os.PathListSeparator))
	if err := os.Setenv("PATH", newPATH); err != nil {
		logger.Warn(fmt.Sprintf("Failed to set PATH environment variable: %v", err))
		return
	}

	logger.Debug(fmt.Sprintf("Extended PATH for session with %d directories", len(pm.additions)))
	for _, path := range pm.additions {
		logger.Debug(fmt.Sprintf("  + %s", path))
	}
}

// GetActivationInstructions returns shell-specific instructions for activating PATH additions
func (pm *PathManager) GetActivationInstructions(shell string) string {
	if len(pm.additions) == 0 {
		return ""
	}

	var lines []string

	switch shell {
	case "bash", "zsh", "sh":
		for _, path := range pm.additions {
			lines = append(lines, fmt.Sprintf("export PATH=\"%s:$PATH\"", path))
		}
	case "fish":
		for _, path := range pm.additions {
			lines = append(lines, fmt.Sprintf("set -gx PATH %s $PATH", path))
		}
	case "powershell", "pwsh":
		for _, path := range pm.additions {
			lines = append(lines, fmt.Sprintf("$env:PATH = \"%s;$env:PATH\"", path))
		}
	default:
		// Default to bash syntax
		for _, path := range pm.additions {
			lines = append(lines, fmt.Sprintf("export PATH=\"%s:$PATH\"", path))
		}
	}

	return strings.Join(lines, "\n")
}

// GetGitHubActionsInstructions returns GitHub Actions specific PATH update syntax
// Output is one path per line, suitable for: goneat doctor tools env --github >> $GITHUB_PATH
func (pm *PathManager) GetGitHubActionsInstructions() string {
	if len(pm.additions) == 0 {
		return ""
	}

	// Just output paths, one per line - GitHub Actions will append to $GITHUB_PATH
	return strings.Join(pm.additions, "\n")
}

// BuildPATHInstructions creates user-facing instructions for PATH setup
func BuildPATHInstructions(toolName, shimPath, packageManager string) string {
	var instructions strings.Builder

	instructions.WriteString(fmt.Sprintf("\n%s installed to: %s\n", toolName, shimPath))
	instructions.WriteString("But this directory is not in your PATH.\n\n")

	instructions.WriteString("🔧 For immediate use in this terminal:\n")
	if runtime.GOOS == "windows" {
		instructions.WriteString(fmt.Sprintf("  $env:PATH = \"%s;$env:PATH\"\n\n", shimPath))
	} else {
		instructions.WriteString(fmt.Sprintf("  export PATH=\"%s:$PATH\"\n\n", shimPath))
	}

	instructions.WriteString("📝 To persist across shell sessions:\n")
	if runtime.GOOS == "windows" {
		instructions.WriteString("  Add to your PowerShell profile or use Windows System Properties\n\n")
	} else {
		shellRC := "~/.bashrc"
		if shell := os.Getenv("SHELL"); strings.Contains(shell, "zsh") {
			shellRC = "~/.zshrc"
		}
		instructions.WriteString(fmt.Sprintf("  echo 'export PATH=\"%s:$PATH\"' >> %s\n\n", shimPath, shellRC))
	}

	instructions.WriteString("⚡ Or use goneat's helper:\n")
	instructions.WriteString("  goneat doctor tools env >> ")
	if runtime.GOOS == "windows" {
		instructions.WriteString("$PROFILE\n")
	} else {
		instructions.WriteString("~/.bashrc\n")
	}

	// Add package manager specific activation if available
	config, err := LoadPackageManagersConfig()
	if err == nil {
		for _, pm := range config.PackageManagers {
			if pm.Name == packageManager && len(pm.PathActivation) > 0 {
				instructions.WriteString("\n💡 Package manager activation (alternative):\n")
				for shell, cmd := range pm.PathActivation {
					instructions.WriteString(fmt.Sprintf("  %s: %s\n", shell, cmd))
				}
			}
		}
	}

	return instructions.String()
}

// DetectCurrentShell attempts to detect the user's current shell
func DetectCurrentShell() string {
	if runtime.GOOS == "windows" {
		// Check for PowerShell
		if os.Getenv("PSModulePath") != "" {
			return "powershell"
		}
		return "cmd"
	}

	// Unix-like systems
	shell := os.Getenv("SHELL")
	if shell == "" {
		return "sh"
	}

	// Extract shell name from path
	base := filepath.Base(shell)
	return base
}

// IsPathInPATH checks if a given path is already in the PATH environment variable
func IsPathInPATH(path string) bool {
	pathVar := os.Getenv("PATH")
	paths := strings.Split(pathVar, string(os.PathListSeparator))

	for _, p := range paths {
		if p == path {
			return true
		}
	}

	return false
}
