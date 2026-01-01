package assess

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/fulmenhq/goneat/pkg/logger"
)

// RustProject represents a detected Rust project with workspace information
type RustProject struct {
	// CargoTomlPath is the path to the Cargo.toml file
	CargoTomlPath string
	// RootPath is the directory containing the Cargo.toml
	RootPath string
	// IsWorkspace indicates if this is a workspace root
	IsWorkspace bool
	// IsWorkspaceMember indicates if this is a workspace member (not root)
	IsWorkspaceMember bool
	// WorkspaceRootPath is the path to the workspace root (if member)
	WorkspaceRootPath string
}

// RustToolPresence represents the presence and version of a Rust tool
type RustToolPresence struct {
	Name       string
	Present    bool
	Version    string
	MeetsMin   bool
	MinVersion string
}

// WorkspacePattern matches [workspace] section in Cargo.toml
var WorkspacePattern = regexp.MustCompile(`(?m)^\s*\[workspace\]`)

// WorkspaceMemberPattern matches workspace = "..." in [package] section
var WorkspaceMemberPattern = regexp.MustCompile(`(?m)^\s*workspace\s*=`)

// DetectRustProject detects a Rust project at the given target path.
// Returns nil if no Rust project is detected.
func DetectRustProject(target string) *RustProject {
	// Primary detection: check for Cargo.toml
	cargoPath := filepath.Join(target, "Cargo.toml")
	if _, err := os.Stat(cargoPath); err == nil {
		return analyzeCargoToml(cargoPath, target)
	}

	// Check parent directories for Cargo.toml (we might be in a subdirectory)
	if project := findCargoInParents(target); project != nil {
		return project
	}

	// Secondary detection: check for .rs files (non-Cargo Rust, rare)
	if hasRustFiles(target) {
		logger.Debug(fmt.Sprintf("Rust files found without Cargo.toml at %s", target))
		return &RustProject{
			RootPath: target,
		}
	}

	return nil
}

// analyzeCargoToml analyzes a Cargo.toml to determine workspace status
func analyzeCargoToml(cargoPath, rootPath string) *RustProject {
	content, err := os.ReadFile(cargoPath) // #nosec G304 -- cargoPath derived from target directory
	if err != nil {
		logger.Debug(fmt.Sprintf("Failed to read Cargo.toml at %s: %v", cargoPath, err))
		return &RustProject{
			CargoTomlPath: cargoPath,
			RootPath:      rootPath,
		}
	}

	project := &RustProject{
		CargoTomlPath: cargoPath,
		RootPath:      rootPath,
	}

	// Check if this is a workspace root
	if WorkspacePattern.Match(content) {
		project.IsWorkspace = true
		return project
	}

	// Check if this is a workspace member
	if WorkspaceMemberPattern.Match(content) {
		project.IsWorkspaceMember = true
		// Find the workspace root
		if wsRoot := findWorkspaceRoot(rootPath); wsRoot != "" {
			project.WorkspaceRootPath = wsRoot
		}
	}

	return project
}

// findCargoInParents walks up the directory tree looking for Cargo.toml.
// Returns the first Cargo.toml found (standalone crate or workspace root).
// If a workspace member is found, continues walking to find the workspace root.
func findCargoInParents(startPath string) *RustProject {
	absPath, err := filepath.Abs(startPath)
	if err != nil {
		return nil
	}

	var firstProject *RustProject

	// Walk up to 10 levels to avoid infinite loops
	current := absPath
	for i := 0; i < 10; i++ {
		parent := filepath.Dir(current)
		if parent == current {
			// Reached filesystem root
			break
		}

		cargoPath := filepath.Join(parent, "Cargo.toml")
		if _, err := os.Stat(cargoPath); err == nil {
			project := analyzeCargoToml(cargoPath, parent)

			// If we found a workspace root, use it (best case)
			if project.IsWorkspace {
				return project
			}

			// If this is a standalone crate (not a workspace member), use it
			if !project.IsWorkspaceMember {
				return project
			}

			// If this is a workspace member, save it but keep looking for workspace root
			if firstProject == nil {
				firstProject = project
			}
		}

		current = parent
	}

	// If we found a workspace member but not the root, return the member
	// (EffectiveRoot will handle finding the workspace root at runtime)
	return firstProject
}

// findWorkspaceRoot finds the workspace root for a workspace member
func findWorkspaceRoot(memberPath string) string {
	absPath, err := filepath.Abs(memberPath)
	if err != nil {
		return ""
	}

	current := filepath.Dir(absPath)
	for i := 0; i < 10; i++ {
		parent := filepath.Dir(current)
		if parent == current {
			// Reached filesystem root (works on all platforms)
			break
		}

		cargoPath := filepath.Join(current, "Cargo.toml")
		// #nosec G304 -- cargoPath is filepath.Join(current, "Cargo.toml") where current walks up parent dirs
		if content, err := os.ReadFile(cargoPath); err == nil {
			if WorkspacePattern.Match(content) {
				return current
			}
		}

		current = parent
	}

	return ""
}

// hasRustFiles checks if the target directory contains any .rs files
func hasRustFiles(target string) bool {
	found := false
	// Quick check - just look for any .rs file, don't enumerate all
	_ = filepath.WalkDir(target, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		// Skip common non-source directories
		if d.IsDir() {
			name := d.Name()
			if name == "target" || name == ".git" || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".rs") {
			found = true
			return filepath.SkipAll // Stop walking once we find one
		}
		return nil
	})
	return found
}

// EffectiveRoot returns the path where Rust tools should be executed.
// For workspace members, this returns the workspace root.
// For standalone crates or workspace roots, this returns the project root.
func (p *RustProject) EffectiveRoot() string {
	if p.IsWorkspaceMember && p.WorkspaceRootPath != "" {
		return p.WorkspaceRootPath
	}
	return p.RootPath
}

// CheckRustToolPresence checks if a Rust tool is available and meets minimum version
func CheckRustToolPresence(tool, minVersion string) RustToolPresence {
	result := RustToolPresence{
		Name:       tool,
		MinVersion: minVersion,
	}

	// Determine the version command based on tool
	var args []string
	switch tool {
	case "cargo-deny":
		args = []string{"deny", "--version"}
	case "cargo-audit":
		args = []string{"audit", "--version"}
	case "cargo-clippy":
		args = []string{"clippy", "--version"}
	default:
		args = []string{tool, "--version"}
	}

	cmd := exec.Command("cargo", args...) // #nosec G204 -- args from controlled switch
	output, err := cmd.Output()
	if err != nil {
		return result
	}

	result.Present = true
	result.Version = parseVersionFromOutput(string(output))

	if minVersion != "" && result.Version != "" {
		result.MeetsMin = compareVersions(result.Version, minVersion) >= 0
	} else {
		result.MeetsMin = true // No minimum specified
	}

	return result
}

// parseVersionFromOutput extracts a version number from tool output
func parseVersionFromOutput(output string) string {
	// Common patterns: "cargo-deny 0.16.2" or "cargo-audit 0.21.0"
	// Also handles: "clippy 0.1.85 (abc123 2024-01-01)"
	versionPattern := regexp.MustCompile(`(\d+\.\d+\.\d+)`)
	matches := versionPattern.FindStringSubmatch(output)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

// compareVersions compares two semver versions.
// Returns: -1 if a < b, 0 if a == b, 1 if a > b
func compareVersions(a, b string) int {
	partsA := strings.Split(a, ".")
	partsB := strings.Split(b, ".")

	// Pad to same length
	for len(partsA) < 3 {
		partsA = append(partsA, "0")
	}
	for len(partsB) < 3 {
		partsB = append(partsB, "0")
	}

	for i := 0; i < 3; i++ {
		numA := parseVersionPart(partsA[i])
		numB := parseVersionPart(partsB[i])
		if numA < numB {
			return -1
		}
		if numA > numB {
			return 1
		}
	}
	return 0
}

// parseVersionPart parses a version part to an integer
func parseVersionPart(part string) int {
	// Handle pre-release suffixes like "0-rc1"
	if idx := strings.IndexAny(part, "-+"); idx != -1 {
		part = part[:idx]
	}
	var num int
	for _, c := range part {
		if c >= '0' && c <= '9' {
			num = num*10 + int(c-'0')
		} else {
			break
		}
	}
	return num
}

// IsCargoAvailable checks if the cargo command is available
func IsCargoAvailable() bool {
	_, err := exec.LookPath("cargo")
	return err == nil
}
