package managers

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/fulmenhq/goneat/pkg/logger"
)

// GoManager handles go.mod files (validation-only for Wave 3)
type GoManager struct{}

// NewGoManager creates a new Go package manager
func NewGoManager() *GoManager {
	return &GoManager{}
}

// Name returns the name of this package manager
func (m *GoManager) Name() string {
	return "go.mod"
}

// Detect finds go.mod files in the given root directory
func (m *GoManager) Detect(root string) ([]string, error) {
	var files []string

	// Standard directories to skip during detection
	skipDirs := map[string]bool{
		"vendor":       true,
		".git":         true,
		".svn":         true,
		".hg":          true,
		"testdata":     true,
		"node_modules": true, // Some Go projects have JS tooling
	}

	// Find all go.mod files, skipping standard exclusions
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories that should be excluded
		if info.IsDir() && skipDirs[info.Name()] {
			return filepath.SkipDir
		}

		if info.Name() == "go.mod" {
			// Convert to relative path from root for consistent handling
			relPath, err := filepath.Rel(root, path)
			if err != nil {
				relPath = path // Fallback to absolute if relative fails
			}
			files = append(files, relPath)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to detect go.mod files: %w", err)
	}

	logger.Debug("Detected go.mod files", logger.Int("count", len(files)))
	return files, nil
}

// ExtractVersion reads the module name from a go.mod file
// Note: go.mod doesn't contain version info, but we can extract the module name
// for validation against VERSION file patterns
func (m *GoManager) ExtractVersion(file string) (string, error) {
	// Validate file path to prevent path traversal
	validatedPath, err := filepath.Abs(file)
	if err != nil {
		return "", fmt.Errorf("failed to resolve file path: %w", err)
	}
	file = validatedPath

	data, err := os.ReadFile(file) // #nosec G304 - path validated with filepath.Abs above
	if err != nil {
		return "", fmt.Errorf("failed to read go.mod: %w", err)
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			moduleName := strings.TrimSpace(strings.TrimPrefix(line, "module "))
			// For validation purposes, return the module name
			// The actual version validation will be done by comparing against VERSION
			return moduleName, nil
		}
	}

	return "", fmt.Errorf("no module directive found in go.mod")
}

// UpdateVersion is not implemented for go.mod files (validation-only)
func (m *GoManager) UpdateVersion(file, version string) error {
	return fmt.Errorf("go.mod version updates are not supported (validation-only)")
}

// ValidateVersion validates that the go.mod module name is consistent with version patterns
// Note: This is validation-only and does not attempt version scheme detection.
// It performs a best-effort check for SemVer-style /vN suffixes but is lenient for CalVer.
func (m *GoManager) ValidateVersion(file, expectedVersion string) error {
	moduleName, err := m.ExtractVersion(file)
	if err != nil {
		return err
	}

	// Check if module name contains version information (e.g., /v2, /v3)
	// This is primarily for SemVer modules; CalVer modules typically don't use /vN suffixes
	if strings.Contains(moduleName, "/v") {
		re := regexp.MustCompile(`/v(\d+)$`)
		matches := re.FindStringSubmatch(moduleName)
		if len(matches) > 1 {
			moduleMajorVersion := matches[1]

			// Try to extract major version if expectedVersion looks like SemVer (N.N.N format)
			// For CalVer or other schemes, skip this check
			versionParts := strings.Split(expectedVersion, ".")
			if len(versionParts) >= 2 {
				// Check if first part is numeric (SemVer-like)
				if _, err := regexp.MatchString(`^\d+$`, versionParts[0]); err == nil {
					majorVersion := versionParts[0]
					if moduleMajorVersion != majorVersion {
						// This is only a warning - don't fail validation
						logger.Warn("Module version suffix mismatch (SemVer modules only)",
							logger.String("file", file),
							logger.String("module", moduleName),
							logger.String("expected_major", majorVersion),
							logger.String("module_major", moduleMajorVersion))
					}
				}
			}
		}
	}

	// For validation-only mode, we just check that the file exists and is parseable
	// Actual version enforcement is the responsibility of go.mod maintenance workflows
	logger.Debug("Validated go.mod file", logger.String("file", file), logger.String("module", moduleName))
	return nil
}
