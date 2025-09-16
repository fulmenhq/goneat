package loaders

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/fulmenhq/goneat/pkg/pathfinder"
)

// LocalLoader provides access to the local filesystem with enhanced safety and audit features
// *(Updated by Arch Eagle: Self-registration via init() to break import cycles)*
type LocalLoader struct {
	rootPath       string
	auditLogger    pathfinder.AuditLogger
	constraint     pathfinder.PathConstraint
	maxFileSize    int64
	followSymlinks bool
}

func init() {
	pathfinder.RegisterLoader("local", func(cfg pathfinder.LoaderConfig) (pathfinder.SourceLoader, error) {
		loader := NewLocalLoader("")
		return loader, nil
	})
}

// NewLocalLoader creates a new local filesystem loader
func NewLocalLoader(rootPath string) *LocalLoader {
	return &LocalLoader{
		rootPath:       rootPath,
		maxFileSize:    100 * 1024 * 1024, // 100MB default
		followSymlinks: false,             // Security default
	}
}

// Open opens a file for reading
func (l *LocalLoader) Open(path string) (io.ReadCloser, error) {
	start := time.Now()

	// Validate and clean the path
	cleanPath, err := l.validateAndCleanPath(path)
	if err != nil {
		l.logAudit(pathfinder.OpOpen, path, pathfinder.OperationResult{
			Status:  "denied",
			Code:    403,
			Message: err.Error(),
		}, time.Since(start))
		return nil, err
	}

	// Open the file
	file, err := os.Open(cleanPath) // #nosec G304 - path is validated by validateAndCleanPath
	if err != nil {
		result := pathfinder.OperationResult{
			Status:  "failure",
			Code:    500,
			Message: err.Error(),
		}
		l.logAudit(pathfinder.OpOpen, path, result, time.Since(start))
		return nil, &LoaderError{
			Type:    "OpenError",
			Message: "Failed to open file",
			Path:    path,
			Cause:   err,
		}
	}

	// Check file size if configured
	if l.maxFileSize > 0 {
		if info, err := file.Stat(); err == nil {
			if info.Size() > l.maxFileSize {
				if closeErr := file.Close(); closeErr != nil {
					// Log close error but continue with size limit error
					fmt.Printf("Warning: failed to close file: %v\n", closeErr)
				}
				result := pathfinder.OperationResult{
					Status:  "denied",
					Code:    413,
					Message: fmt.Sprintf("File size %d exceeds maximum %d", info.Size(), l.maxFileSize),
				}
				l.logAudit(pathfinder.OpOpen, path, result, time.Since(start))
				return nil, &LoaderError{
					Type:    "SizeLimitError",
					Message: "File exceeds size limit",
					Path:    path,
				}
			}
		}
	}

	l.logAudit(pathfinder.OpOpen, path, pathfinder.OperationResult{
		Status: "success",
		Code:   200,
	}, time.Since(start))

	return file, nil
}

// ListFiles lists files matching the given patterns
func (l *LocalLoader) ListFiles(basePath string, include, exclude []string) ([]string, error) {
	start := time.Now()

	// Validate base path
	cleanBase, err := l.validateAndCleanPath(basePath)
	if err != nil {
		l.logAudit(pathfinder.OpList, basePath, pathfinder.OperationResult{
			Status:  "denied",
			Code:    403,
			Message: err.Error(),
		}, time.Since(start))
		return nil, err
	}

	var files []string

	// Walk the directory tree
	err = filepath.WalkDir(cleanBase, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories unless they match include patterns
		if d.IsDir() && !l.shouldInclude(path, include, exclude) {
			return nil
		}

		// Check if file matches patterns
		if l.shouldInclude(path, include, exclude) {
			// Convert to relative path from base
			relPath, err := filepath.Rel(cleanBase, path)
			if err != nil {
				return err
			}

			// Normalize path separators for consistency
			relPath = filepath.ToSlash(relPath)
			files = append(files, relPath)
		}

		return nil
	})

	if err != nil {
		result := pathfinder.OperationResult{
			Status:  "failure",
			Code:    500,
			Message: err.Error(),
		}
		l.logAudit(pathfinder.OpList, basePath, result, time.Since(start))
		return nil, &LoaderError{
			Type:    "ListError",
			Message: "Failed to list files",
			Path:    basePath,
			Cause:   err,
		}
	}

	l.logAudit(pathfinder.OpList, basePath, pathfinder.OperationResult{
		Status:  "success",
		Code:    200,
		Message: fmt.Sprintf("Found %d files", len(files)),
	}, time.Since(start))

	return files, nil
}

// SourceType returns the loader type
func (l *LocalLoader) SourceType() string {
	return "local"
}

// SourceDescription returns a human-readable description
func (l *LocalLoader) SourceDescription() string {
	return fmt.Sprintf("Local filesystem loader (root: %s)", l.rootPath)
}

// Validate checks if the loader is properly configured
func (l *LocalLoader) Validate() error {
	if l.rootPath == "" {
		return fmt.Errorf("root path is required")
	}

	// Check if root path exists and is accessible
	info, err := os.Stat(l.rootPath)
	if err != nil {
		return fmt.Errorf("root path validation failed: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("root path must be a directory")
	}

	return nil
}

// SetAuditLogger sets the audit logger for this loader
func (l *LocalLoader) SetAuditLogger(logger pathfinder.AuditLogger) {
	l.auditLogger = logger
}

// SetConstraint sets the path constraint for this loader
func (l *LocalLoader) SetConstraint(constraint pathfinder.PathConstraint) {
	l.constraint = constraint
}

// SetMaxFileSize sets the maximum file size limit
func (l *LocalLoader) SetMaxFileSize(size int64) {
	l.maxFileSize = size
}

// SetFollowSymlinks configures symlink following behavior
func (l *LocalLoader) SetFollowSymlinks(follow bool) {
	l.followSymlinks = follow
}

// validateAndCleanPath validates and cleans a file path
func (l *LocalLoader) validateAndCleanPath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path cannot be empty")
	}

	// Clean the path
	cleanPath := filepath.Clean(path)

	// Convert to absolute path if relative
	if !filepath.IsAbs(cleanPath) {
		if l.rootPath != "" {
			cleanPath = filepath.Join(l.rootPath, cleanPath)
		} else {
			absPath, err := filepath.Abs(cleanPath)
			if err != nil {
				return "", fmt.Errorf("failed to get absolute path: %w", err)
			}
			cleanPath = absPath
		}
	}

	// Apply constraint if set
	if l.constraint != nil {
		if !l.constraint.Contains(cleanPath) {
			return "", fmt.Errorf("path violates constraint: %s", l.constraint.Type())
		}
	}

	// Check for path traversal attempts
	if strings.Contains(cleanPath, "..") {
		// More thorough check by resolving the path
		resolved, err := filepath.EvalSymlinks(cleanPath)
		if err != nil {
			// If we can't resolve symlinks, be conservative
			if !l.followSymlinks {
				return "", fmt.Errorf("path contains traversal sequences and symlinks are disabled")
			}
		} else {
			cleanPath = resolved
		}
	}

	// Final constraint check on resolved path
	if l.constraint != nil && !l.constraint.Contains(cleanPath) {
		return "", fmt.Errorf("resolved path violates constraint: %s", l.constraint.Type())
	}

	return cleanPath, nil
}

// shouldInclude checks if a path should be included based on patterns
func (l *LocalLoader) shouldInclude(path string, include, exclude []string) bool {
	// If no include patterns, include everything except excluded
	if len(include) == 0 {
		return !l.matchesAnyPattern(path, exclude)
	}

	// If include patterns exist, must match at least one and not match exclude
	return l.matchesAnyPattern(path, include) && !l.matchesAnyPattern(path, exclude)
}

// matchesAnyPattern checks if path matches any of the given patterns
func (l *LocalLoader) matchesAnyPattern(path string, patterns []string) bool {
	for _, pattern := range patterns {
		if matched, _ := doublestar.Match(pattern, path); matched {
			return true
		}
		// Also try with forward slashes for cross-platform compatibility
		if matched, _ := doublestar.Match(pattern, filepath.ToSlash(path)); matched {
			return true
		}
	}
	return false
}

// logAudit logs an audit event if audit logger is configured
func (l *LocalLoader) logAudit(operation pathfinder.PathOperation, path string, result pathfinder.OperationResult, duration time.Duration) {
	if l.auditLogger == nil {
		return
	}

	record := pathfinder.AuditRecord{
		Operation:    operation,
		Path:         path,
		SourceLoader: l.SourceType(),
		Result:       result,
		Duration:     duration,
		Timestamp:    time.Now(),
	}

	if l.constraint != nil {
		record.Constraint = string(l.constraint.Type())
	}

	// Log asynchronously to avoid blocking file operations
	go func() {
		_ = l.auditLogger.LogOperation(record)
	}()
}
