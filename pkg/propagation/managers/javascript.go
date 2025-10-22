package managers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/fulmenhq/goneat/pkg/logger"
)

// JavaScriptManager handles package.json files and workspaces
type JavaScriptManager struct{}

// NewJavaScriptManager creates a new JavaScript package manager
func NewJavaScriptManager() *JavaScriptManager {
	return &JavaScriptManager{}
}

// Name returns the name of this package manager
func (m *JavaScriptManager) Name() string {
	return "package.json"
}

// Detect finds package.json files in the given root directory
func (m *JavaScriptManager) Detect(root string) ([]string, error) {
	var files []string

	// Standard directories to skip during detection
	skipDirs := map[string]bool{
		"node_modules": true,
		".git":         true,
		".svn":         true,
		".hg":          true,
		"vendor":       true,
		"dist":         true,
		"build":        true,
		".next":        true,
		".nuxt":        true,
	}

	// Find all package.json files recursively, skipping standard exclusions
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories that should be excluded
		if info.IsDir() && skipDirs[info.Name()] {
			return filepath.SkipDir
		}

		if info.Name() == "package.json" {
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
		return nil, fmt.Errorf("failed to detect package.json files: %w", err)
	}

	logger.Debug("Detected package.json files", logger.Int("count", len(files)))
	return files, nil
}

// ExtractVersion reads the version from a package.json file
func (m *JavaScriptManager) ExtractVersion(file string) (string, error) {
	// Validate file path to prevent path traversal
	validatedPath, err := filepath.Abs(file)
	if err != nil {
		return "", fmt.Errorf("failed to resolve file path: %w", err)
	}
	file = validatedPath

	data, err := os.ReadFile(file) // #nosec G304 - path validated with filepath.Abs above
	if err != nil {
		return "", fmt.Errorf("failed to read package.json: %w", err)
	}

	var pkg struct {
		Version string `json:"version"`
	}

	if err := json.Unmarshal(data, &pkg); err != nil {
		return "", fmt.Errorf("failed to parse package.json: %w", err)
	}

	if pkg.Version == "" {
		return "", fmt.Errorf("no version field found in package.json")
	}

	return pkg.Version, nil
}

// UpdateVersion updates the version in a package.json file
func (m *JavaScriptManager) UpdateVersion(file, version string) error {
	// Validate file path to prevent path traversal
	validatedPath, err := filepath.Abs(file)
	if err != nil {
		return fmt.Errorf("failed to resolve file path: %w", err)
	}
	file = validatedPath

	data, err := os.ReadFile(file) // #nosec G304 - path validated with filepath.Abs above
	if err != nil {
		return fmt.Errorf("failed to read package.json: %w", err)
	}

	var pkg map[string]interface{}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return fmt.Errorf("failed to parse package.json: %w", err)
	}

	// Update version field
	pkg["version"] = version

	// Write back with proper formatting
	updatedData, err := json.MarshalIndent(pkg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal updated package.json: %w", err)
	}

	// Add newline at end
	updatedData = append(updatedData, '\n')

	if err := os.WriteFile(file, updatedData, 0600); err != nil {
		return fmt.Errorf("failed to write updated package.json: %w", err)
	}

	logger.Info("Updated package.json version", logger.String("file", file), logger.String("version", version))
	return nil
}

// ValidateVersion checks if the version in the file matches the expected version
func (m *JavaScriptManager) ValidateVersion(file, expectedVersion string) error {
	actualVersion, err := m.ExtractVersion(file)
	if err != nil {
		return err
	}

	if actualVersion != expectedVersion {
		return fmt.Errorf("version mismatch: expected %s, got %s", expectedVersion, actualVersion)
	}

	return nil
}
