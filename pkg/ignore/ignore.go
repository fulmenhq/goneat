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
	matcher gitignore.Matcher
}

// NewMatcher creates a matcher with layered ignore files:
// 1. .gitignore and related git ignore files (foundation)
// 2. .goneatignore (repo overrides)
// 3. ~/.goneat/.goneatignore (user overrides)
func NewMatcher(repoRoot string) (*Matcher, error) {
	fs := osfs.New(repoRoot)

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
	if goneatPatterns, err := readIgnoreFile(filepath.Join(repoRoot, ".goneatignore")); err == nil {
		for _, pattern := range goneatPatterns {
			allPatterns = append(allPatterns, gitignore.ParsePattern(pattern, nil))
		}
	}

	// Layer 3: Manually read user-level ~/.goneat/.goneatignore patterns (user overrides)
	if homeDir, err := os.UserHomeDir(); err == nil {
		userIgnorePath := filepath.Join(homeDir, ".goneat", ".goneatignore")
		if userPatterns, err := readIgnoreFile(userIgnorePath); err == nil {
			for _, pattern := range userPatterns {
				allPatterns = append(allPatterns, gitignore.ParsePattern(pattern, nil))
			}
		}
	}

	return &Matcher{
		matcher: gitignore.NewMatcher(allPatterns),
	}, nil
}

// readIgnoreFile reads patterns from a text file (like .goneatignore)
func readIgnoreFile(path string) ([]string, error) {
    // Only allow reading known ignore files in controlled locations
    cleaned := filepath.Clean(path)
    // Allowlist: .goneatignore at repo root or under $HOME/.goneat/
    allowed := false
    if strings.HasSuffix(cleaned, string(os.PathSeparator)+".goneatignore") {
        allowed = true
    }
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

// IsIgnored checks if a file path should be ignored
func (m *Matcher) IsIgnored(path string) bool {
	// Convert path to relative path from working directory
	wd, err := os.Getwd()
	if err != nil {
		return false
	}

	relPath, err := filepath.Rel(wd, path)
	if err != nil {
		relPath = path
	}

	// Convert to forward slashes for gitignore matching
	relPath = filepath.ToSlash(relPath)

	// Split path into components for go-git matcher
	pathParts := splitPath(relPath)
	if len(pathParts) == 0 {
		return false
	}

	// Check if file matches ignore patterns
	return m.matcher.Match(pathParts, false)
}

// IsIgnoredDir checks if a directory should be ignored (and thus skipped during traversal)
func (m *Matcher) IsIgnoredDir(path string) bool {
	// Convert path to relative path from working directory
	wd, err := os.Getwd()
	if err != nil {
		return false
	}

	relPath, err := filepath.Rel(wd, path)
	if err != nil {
		relPath = path
	}

	// Convert to forward slashes for gitignore matching
	relPath = filepath.ToSlash(relPath)

	// Split path into components for go-git matcher
	pathParts := splitPath(relPath)
	if len(pathParts) == 0 {
		return false
	}

	// Check if directory matches ignore patterns
	return m.matcher.Match(pathParts, true) // isDir=true for directory matching
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
