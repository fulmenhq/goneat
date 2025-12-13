// Package ignore provides gitignore-based file filtering using go-git
package ignore

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5/osfs"
	gitignore "github.com/go-git/go-git/v5/plumbing/format/gitignore"
)

// Matcher provides gitignore-based file filtering
type Matcher struct {
	matcher  gitignore.Matcher
	repoRoot string
}

// NewMatcher creates a matcher with layered ignore files:
// 1. .gitignore and related git ignore files (foundation)
// 2. .goneatignore (repo overrides)
// 3. ~/.goneat/.goneatignore (user overrides)
func NewMatcher(repoRoot string) (*Matcher, error) {
	absRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		absRoot = repoRoot
	}
	fs := osfs.New(absRoot)

	// Load patterns with layered approach
	var allPatterns []gitignore.Pattern

	// Add default patterns that should always be ignored (highest priority)
	defaultPatterns := []string{".git/**", "node_modules/**", ".scratchpad/**"}
	for _, pattern := range defaultPatterns {
		allPatterns = append(allPatterns, gitignore.ParsePattern(pattern, nil))
	}

	// Layer 1: Load standard gitignore patterns (foundation)
	// ReadPatterns with nil reads .gitignore, global excludes, and .git/info/exclude
	if gitPatterns, err := gitignore.ReadPatterns(fs, nil); err == nil {
		allPatterns = append(allPatterns, gitPatterns...)
	}

	// Layer 2: Manually read .goneatignore patterns (repo overrides)
	if goneatPatterns, err := readIgnoreFile(filepath.Join(absRoot, ".goneatignore")); err == nil {
		for _, pattern := range goneatPatterns {
			allPatterns = append(allPatterns, gitignore.ParsePattern(pattern, nil))
		}
	}

	// Layer 3: Manually read user-level ignore patterns (user overrides)
	if homeDir, err := os.UserHomeDir(); err == nil {
		// Legacy location: ~/.goneatignore
		legacyIgnorePath := filepath.Join(homeDir, ".goneatignore")
		if userPatterns, err := readIgnoreFile(legacyIgnorePath); err == nil {
			for _, pattern := range userPatterns {
				allPatterns = append(allPatterns, gitignore.ParsePattern(pattern, nil))
			}
		}

		// Preferred location: ~/.goneat/.goneatignore
		userIgnorePath := filepath.Join(homeDir, ".goneat", ".goneatignore")
		if userPatterns, err := readIgnoreFile(userIgnorePath); err == nil {
			for _, pattern := range userPatterns {
				allPatterns = append(allPatterns, gitignore.ParsePattern(pattern, nil))
			}
		}
	}

	return &Matcher{
		matcher:  gitignore.NewMatcher(allPatterns),
		repoRoot: absRoot,
	}, nil
}

// readIgnoreFile reads patterns from a text file (like .goneatignore)
func readIgnoreFile(path string) ([]string, error) {
	// Only allow reading known ignore files in controlled locations
	cleaned := filepath.Clean(path)
	// Allowlist: files ending with .goneatignore or test-ignore (for testing)
	allowed := strings.HasSuffix(cleaned, ".goneatignore") || strings.HasSuffix(cleaned, "test-ignore")
	if !allowed {
		return nil, fmt.Errorf("disallowed ignore file path: %s", cleaned)
	}
	content, err := os.ReadFile(cleaned) // #nosec G304 -- path cleaned and allowlisted
	if err != nil {
		return nil, err
	}

	var patterns []string
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}

	return patterns, nil
}

// IsIgnoredRel checks if a repo-root-relative path should be ignored.
//
// relPath must be a repo-root relative path ("foo/bar.txt"), using either OS
// separators or forward slashes. It is normalized internally.
func (m *Matcher) IsIgnoredRel(relPath string) bool {
	relPath = filepath.ToSlash(relPath)
	pathParts := splitPath(relPath)
	if len(pathParts) == 0 {
		return false
	}
	return m.matcher.Match(pathParts, false)
}

// IsIgnored checks if a file path should be ignored.
//
// This method is best-effort and attempts to convert the provided path into a
// repo-root-relative path for matching. Prefer IsIgnoredRel when you already
// have a repo-relative path.
func (m *Matcher) IsIgnored(path string) bool {
	cleaned := filepath.Clean(path)

	relPath, err := filepath.Rel(m.repoRoot, cleaned)
	if err != nil {
		// Fallback to working directory relative conversion.
		wd, wdErr := os.Getwd()
		if wdErr != nil {
			return false
		}
		relPath, err = filepath.Rel(wd, cleaned)
		if err != nil {
			relPath = cleaned
		}
	}

	return m.IsIgnoredRel(relPath)
}

// IsIgnoredDir checks if a directory should be ignored (and thus skipped during traversal)
// IsIgnoredDirRel checks if a repo-root-relative directory should be ignored.
func (m *Matcher) IsIgnoredDirRel(relPath string) bool {
	relPath = filepath.ToSlash(relPath)
	pathParts := splitPath(relPath)
	if len(pathParts) == 0 {
		return false
	}
	return m.matcher.Match(pathParts, true)
}

// IsIgnoredDir checks if a directory should be ignored (and thus skipped during traversal).
func (m *Matcher) IsIgnoredDir(path string) bool {
	cleaned := filepath.Clean(path)

	relPath, err := filepath.Rel(m.repoRoot, cleaned)
	if err != nil {
		wd, wdErr := os.Getwd()
		if wdErr != nil {
			return false
		}
		relPath, err = filepath.Rel(wd, cleaned)
		if err != nil {
			relPath = cleaned
		}
	}

	return m.IsIgnoredDirRel(relPath)
}

// splitPath converts a slash-separated path into components for go-git matching
func splitPath(path string) []string {
	if path == "" || path == "." {
		return []string{}
	}

	// Remove leading slash if present
	path = strings.TrimPrefix(path, "/")

	// Split on forward slashes
	parts := strings.Split(path, "/")

	// Remove empty components
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" && part != "." {
			result = append(result, part)
		}
	}

	return result
}
