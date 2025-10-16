package managers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fulmenhq/goneat/pkg/logger"
	"github.com/pelletier/go-toml/v2"
)

// PythonManager handles pyproject.toml files
type PythonManager struct{}

// NewPythonManager creates a new Python package manager
func NewPythonManager() *PythonManager {
	return &PythonManager{}
}

// Name returns the name of this package manager
func (m *PythonManager) Name() string {
	return "pyproject.toml"
}

// Detect finds pyproject.toml files in the given root directory
func (m *PythonManager) Detect(root string) ([]string, error) {
	var files []string

	// Standard directories to skip during detection
	skipDirs := map[string]bool{
		".venv":         true,
		"venv":          true,
		".env":          true,
		"env":           true,
		".git":          true,
		".svn":          true,
		".hg":           true,
		"__pycache__":   true,
		".pytest_cache": true,
		".tox":          true,
		"dist":          true,
		"build":         true,
		"*.egg-info":    true,
	}

	// Find all pyproject.toml files, skipping standard exclusions
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories that should be excluded
		if info.IsDir() && skipDirs[info.Name()] {
			return filepath.SkipDir
		}

		if info.Name() == "pyproject.toml" {
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
		return nil, fmt.Errorf("failed to detect pyproject.toml files: %w", err)
	}

	logger.Debug("Detected pyproject.toml files", logger.Int("count", len(files)))
	return files, nil
}

// ExtractVersion reads the version from a pyproject.toml file
func (m *PythonManager) ExtractVersion(file string) (string, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return "", fmt.Errorf("failed to read pyproject.toml: %w", err)
	}

	var config map[string]interface{}
	if err := toml.Unmarshal(data, &config); err != nil {
		return "", fmt.Errorf("failed to parse pyproject.toml: %w", err)
	}

	// Try [project] section first (PEP 621)
	if project, ok := config["project"].(map[string]interface{}); ok {
		if version, ok := project["version"].(string); ok && version != "" {
			return version, nil
		}
	}

	// Try [tool.poetry] section as fallback
	if tool, ok := config["tool"].(map[string]interface{}); ok {
		if poetry, ok := tool["poetry"].(map[string]interface{}); ok {
			if version, ok := poetry["version"].(string); ok && version != "" {
				return version, nil
			}
		}
	}

	return "", fmt.Errorf("no version field found in [project] or [tool.poetry] sections")
}

// UpdateVersion updates the version in a pyproject.toml file
// NOTE: This implementation preserves file structure by using targeted text replacement
// rather than full TOML unmarshal/remarshal to avoid comment loss and reordering
func (m *PythonManager) UpdateVersion(file, version string) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read pyproject.toml: %w", err)
	}

	content := string(data)
	updated := false

	// Update [project] version field
	if newContent, fieldUpdated := m.updateTOMLField(content, "project", "version", version); fieldUpdated {
		content = newContent
		updated = true
	}

	// Update [tool.poetry] version field
	if newContent, fieldUpdated := m.updateTOMLField(content, "tool.poetry", "version", version); fieldUpdated {
		content = newContent
		updated = true
	}

	if !updated {
		return fmt.Errorf("no version field found to update in [project] or [tool.poetry] sections")
	}

	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write updated pyproject.toml: %w", err)
	}

	logger.Info("Updated pyproject.toml version", logger.String("file", file), logger.String("version", version))
	return nil
}

// updateTOMLField performs targeted text replacement for a TOML field
// This preserves comments, ordering, and formatting
func (m *PythonManager) updateTOMLField(content, section, field, newValue string) (string, bool) {
	// Find all matches and update them within the correct section
	lines := strings.Split(content, "\n")
	inSection := false
	sectionDepth := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Track section nesting
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			sectionName := strings.Trim(trimmed, "[]")
			if sectionName == section || strings.HasPrefix(sectionName, section+".") {
				inSection = true
				sectionDepth = strings.Count(sectionName, ".") + 1
			} else if inSection && strings.Count(sectionName, ".") < sectionDepth {
				// Exiting the section
				inSection = false
			}
		}

		// Only update if we're in the correct section and line contains the field
		if inSection && strings.Contains(line, field+" =") {
			// Preserve indentation and comments by capturing original formatting
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				// Preserve leading whitespace (indentation)
				leadingSpace := ""
				for _, r := range line {
					if r == ' ' || r == '\t' {
						leadingSpace += string(r)
					} else {
						break
					}
				}
				// Rebuild line with original indentation and new value
				lines[i] = leadingSpace + field + ` = "` + newValue + `"`
				return strings.Join(lines, "\n"), true
			}
		}
	}

	return content, false
}

// ValidateVersion checks if the version in the file matches the expected version
func (m *PythonManager) ValidateVersion(file, expectedVersion string) error {
	actualVersion, err := m.ExtractVersion(file)
	if err != nil {
		return err
	}

	if actualVersion != expectedVersion {
		return fmt.Errorf("version mismatch: expected %s, got %s", expectedVersion, actualVersion)
	}

	return nil
}
