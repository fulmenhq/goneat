package pathfinder

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SafetyValidator provides comprehensive path validation and sanitization
type SafetyValidator struct {
	constraint    PathConstraint
	enforcement   EnforcementLevel
	maxDepth      int
	allowSymlinks bool
}

// NewSafetyValidator creates a new safety validator with default settings
func NewSafetyValidator() *SafetyValidator {
	return &SafetyValidator{
		enforcement:   EnforcementStrict,
		maxDepth:      20,
		allowSymlinks: false,
	}
}

// ValidatePath performs comprehensive path validation
func (s *SafetyValidator) ValidatePath(path string) error {
	if path == "" {
		return fmt.Errorf("path cannot be empty")
	}

	// Level 2: Traversal detection (check original path before cleaning)
	if err := s.detectTraversal(path); err != nil {
		return err
	}

	// Level 1: Basic cleaning and normalization
	cleanPath := s.cleanPath(path)

	// Level 2: Traversal detection
	if err := s.detectTraversal(path); err != nil {
		return err
	}

	// Level 3: Repository/workspace containment
	if s.constraint != nil {
		if !s.constraint.Contains(cleanPath) {
			return fmt.Errorf("path violates %s constraint", s.constraint.Type())
		}
	}

	// Level 4: Symlink validation
	if !s.allowSymlinks {
		if err := s.validateSymlinks(cleanPath); err != nil {
			return err
		}
	}

	// Level 5: Permission and accessibility checks
	if err := s.checkAccessibility(cleanPath); err != nil {
		return err
	}

	return nil
}

// SafeJoin safely joins path components with validation
func (s *SafetyValidator) SafeJoin(base, path string) (string, error) {
	if base == "" {
		return "", fmt.Errorf("base path cannot be empty")
	}

	// Validate base path
	if err := s.ValidatePath(base); err != nil {
		return "", fmt.Errorf("base path validation failed: %w", err)
	}

	// Validate the path component before joining (critical for security)
	if err := s.detectTraversal(path); err != nil {
		return "", fmt.Errorf("path component validation failed: %w", err)
	}

	// Join the paths
	joined := filepath.Join(base, path)

	// Validate the result
	if err := s.ValidatePath(joined); err != nil {
		return "", fmt.Errorf("joined path validation failed: %w", err)
	}

	return joined, nil
}

// cleanPath performs basic path cleaning and normalization
func (s *SafetyValidator) cleanPath(path string) string {
	// Use filepath.Clean for cross-platform compatibility
	cleanPath := filepath.Clean(path)

	// Convert to absolute path if relative
	if !filepath.IsAbs(cleanPath) {
		if absPath, err := filepath.Abs(cleanPath); err == nil {
			cleanPath = absPath
		}
	}

	return cleanPath
}

// detectTraversal checks for path traversal attempts
func (s *SafetyValidator) detectTraversal(path string) error {
	// Check for obvious traversal patterns
	if strings.Contains(path, "..") {
		// Always reject paths with .. for security, regardless of enforcement level
		// This is a critical security check that should never be bypassed
		return fmt.Errorf("path contains traversal sequences (..)")
	}

	return nil
}

// validateSymlinks checks for symlink-related security issues
func (s *SafetyValidator) validateSymlinks(path string) error {
	// Check if the path itself is a symlink
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlinks are not allowed")
		}
	}

	// Check parent directories for symlinks
	dir := filepath.Dir(path)
	for dir != "/" && dir != "." {
		if info, err := os.Lstat(dir); err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("parent directory contains symlink: %s", dir)
			}
		}
		dir = filepath.Dir(dir)
	}

	return nil
}

// checkAccessibility verifies file/directory accessibility
func (s *SafetyValidator) checkAccessibility(path string) error {
	// Check if path exists (but don't fail if it doesn't - we might be validating non-existent paths)
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Path doesn't exist, but that's OK for validation purposes
			// We'll still check the path structure for safety
			return nil
		}
		return fmt.Errorf("cannot access path: %w", err)
	}

	// Check depth limit for directories
	if info.IsDir() && s.maxDepth > 0 {
		depth := strings.Count(path, string(filepath.Separator))
		if depth > s.maxDepth {
			return fmt.Errorf("directory depth %d exceeds maximum %d", depth, s.maxDepth)
		}
	}

	return nil
}

// SetConstraint sets the path constraint for validation
func (s *SafetyValidator) SetConstraint(constraint PathConstraint) {
	s.constraint = constraint
}

// SetEnforcementLevel sets the enforcement level
func (s *SafetyValidator) SetEnforcementLevel(level EnforcementLevel) {
	s.enforcement = level
}

// SetMaxDepth sets the maximum directory depth
func (s *SafetyValidator) SetMaxDepth(depth int) {
	s.maxDepth = depth
}

// SetAllowSymlinks configures symlink handling
func (s *SafetyValidator) SetAllowSymlinks(allow bool) {
	s.allowSymlinks = allow
}

// RepositoryConstraint implements path containment for git repositories
type RepositoryConstraint struct {
	rootPath string
}

// NewRepositoryConstraint creates a repository-based constraint
func NewRepositoryConstraint(rootPath string) (*RepositoryConstraint, error) {
	absPath, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Verify it's actually a git repository
	gitDir := filepath.Join(absPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("not a git repository: %s", absPath)
	}

	return &RepositoryConstraint{rootPath: absPath}, nil
}

// Contains checks if the path is within the repository
func (c *RepositoryConstraint) Contains(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	relPath, err := filepath.Rel(c.rootPath, absPath)
	if err != nil {
		return false
	}

	// Check for traversal attempts
	return !strings.HasPrefix(relPath, "..")
}

// Root returns the constraint root path
func (c *RepositoryConstraint) Root() string {
	return c.rootPath
}

// Type returns the constraint type
func (c *RepositoryConstraint) Type() ConstraintType {
	return ConstraintRepository
}

// EnforcementLevel returns the enforcement level for this constraint
func (c *RepositoryConstraint) EnforcementLevel() EnforcementLevel {
	return EnforcementStrict
}

// WorkspaceConstraint implements path containment for workspace directories
type WorkspaceConstraint struct {
	rootPath        string
	maxDepth        int
	allowedPatterns []string
}

// NewWorkspaceConstraint creates a workspace-based constraint
func NewWorkspaceConstraint(rootPath string, maxDepth int, allowedPatterns []string) (*WorkspaceConstraint, error) {
	absPath, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	return &WorkspaceConstraint{
		rootPath:        absPath,
		maxDepth:        maxDepth,
		allowedPatterns: allowedPatterns,
	}, nil
}

// Contains checks if the path is within the workspace and matches allowed patterns
func (c *WorkspaceConstraint) Contains(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	relPath, err := filepath.Rel(c.rootPath, absPath)
	if err != nil {
		return false
	}

	// Check for traversal attempts
	if strings.HasPrefix(relPath, "..") {
		return false
	}

	// Check depth limit
	if c.maxDepth > 0 {
		depth := strings.Count(relPath, string(filepath.Separator))
		if depth > c.maxDepth {
			return false
		}
	}

	// Check allowed patterns
	if len(c.allowedPatterns) > 0 {
		matches := false
		for _, pattern := range c.allowedPatterns {
			if matched, _ := filepath.Match(pattern, relPath); matched {
				matches = true
				break
			}
		}
		if !matches {
			return false
		}
	}

	return true
}

// Root returns the constraint root path
func (c *WorkspaceConstraint) Root() string {
	return c.rootPath
}

// Type returns the constraint type
func (c *WorkspaceConstraint) Type() ConstraintType {
	return ConstraintWorkspace
}

// EnforcementLevel returns the enforcement level for this constraint
func (c *WorkspaceConstraint) EnforcementLevel() EnforcementLevel {
	return EnforcementStrict
}
