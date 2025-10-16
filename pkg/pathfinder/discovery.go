package pathfinder

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// DiscoveryEngine provides advanced file discovery with pattern matching
type DiscoveryEngine struct {
	validator *SafetyValidator
}

// NewDiscoveryEngine creates a new file discovery engine
func NewDiscoveryEngine(validator *SafetyValidator) *DiscoveryEngine {
	return &DiscoveryEngine{
		validator: validator,
	}
}

// DiscoverFiles finds files matching the given criteria
func (d *DiscoveryEngine) DiscoverFiles(basePath string, opts DiscoveryOptions) ([]string, error) {
	// Validate base path
	if err := d.validator.ValidatePath(basePath); err != nil {
		return nil, fmt.Errorf("base path validation failed: %w", err)
	}

	var files []string
	var totalSize int64
	var fileCount int

	// Walk the directory tree
	walkFunc := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Handle walk errors based on options
			if opts.ErrorHandler != nil {
				return opts.ErrorHandler(path, err)
			}
			return err
		}

		// Skip symlinks unless explicitly enabled
		if !opts.FollowSymlinks && info.Mode()&os.ModeSymlink != 0 {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		relForDepth, relErr := filepath.Rel(basePath, path)
		if relErr != nil {
			return relErr
		}

		// Enforce max depth if provided (0 or negative means unlimited)
		if opts.MaxDepth > 0 && relForDepth != "." {
			depth := calculateDepth(relForDepth, info.IsDir())
			if info.IsDir() && depth >= opts.MaxDepth {
				return filepath.SkipDir
			}
			if !info.IsDir() && depth > opts.MaxDepth {
				return nil
			}
		}

		// Skip directories if configured
		if info.IsDir() {
			// Check if directory should be skipped
			for _, skipDir := range opts.SkipDirs {
				if strings.Contains(path, skipDir) {
					return filepath.SkipDir
				}
			}
			return nil
		}

		relPathNormalized := filepath.ToSlash(relForDepth)

		// Apply file filters
		if !d.matchesFilters(path, relPathNormalized, info, opts) {
			return nil
		}

		// Convert to relative path
		relPath := filepath.ToSlash(relForDepth)
		files = append(files, relPath)
		totalSize += info.Size()
		fileCount++

		// Progress callback
		if opts.ProgressCallback != nil && fileCount%100 == 0 {
			opts.ProgressCallback(fileCount, -1, path)
		}

		return nil
	}

	// Perform the walk
	if err := filepath.Walk(basePath, walkFunc); err != nil {
		return nil, fmt.Errorf("discovery walk failed: %w", err)
	}

	// Sort results for consistent output
	sort.Strings(files)

	// Final progress callback
	if opts.ProgressCallback != nil {
		opts.ProgressCallback(fileCount, fileCount, "discovery complete")
	}

	return files, nil
}

// matchesFilters checks if a file matches all the configured filters
func (d *DiscoveryEngine) matchesFilters(absPath string, relPath string, info os.FileInfo, opts DiscoveryOptions) bool {
	// Include patterns (must match at least one if specified)
	if len(opts.IncludePatterns) > 0 {
		if !d.MatchesAnyPattern(absPath, opts.IncludePatterns) && !d.MatchesAnyPattern(relPath, opts.IncludePatterns) {
			return false
		}
	}

	// Exclude patterns (must not match any)
	if len(opts.ExcludePatterns) > 0 {
		if d.MatchesAnyPattern(absPath, opts.ExcludePatterns) || d.MatchesAnyPattern(relPath, opts.ExcludePatterns) {
			return false
		}
	}

	// Skip patterns
	if len(opts.SkipPatterns) > 0 {
		if d.MatchesAnyPattern(absPath, opts.SkipPatterns) || d.MatchesAnyPattern(relPath, opts.SkipPatterns) {
			return false
		}
	}

	// File size limits
	if opts.FileSizeLimits.Min != nil && info.Size() < *opts.FileSizeLimits.Min {
		return false
	}
	if opts.FileSizeLimits.Max != nil && info.Size() > *opts.FileSizeLimits.Max {
		return false
	}

	// Modification time range
	if opts.ModTimeRange.After != nil && info.ModTime().Before(*opts.ModTimeRange.After) {
		return false
	}
	if opts.ModTimeRange.Before != nil && info.ModTime().After(*opts.ModTimeRange.Before) {
		return false
	}

	// Hidden files filter
	if !opts.IncludeHidden && d.isHiddenFile(absPath) {
		return false
	}

	return true
}

// MatchesAnyPattern checks if path matches any of the given patterns
// This uses cross-platform pattern normalization to handle ./ and .\ prefixes correctly
func (d *DiscoveryEngine) MatchesAnyPattern(path string, patterns []string) bool {
	for _, pattern := range patterns {
		// Check if original pattern started with ./ or .\ (indicates "root only" intent)
		isRootOnly := strings.HasPrefix(pattern, "./") || strings.HasPrefix(pattern, ".\\")

		// Normalize pattern to handle cross-platform path conventions:
		// Step 1: Convert all backslashes to forward slashes (Windows -> Unix style)
		//   This handles both ".\file" (Windows) and "./file" (Unix) consistently
		normalizedPattern := strings.ReplaceAll(pattern, "\\", "/")

		// Step 2: Use filepath.Clean to remove redundant separators and resolve . and ..
		//   filepath.Clean("./pyproject.toml") -> "pyproject.toml"
		//   filepath.Clean("apps/../pyproject.toml") -> "pyproject.toml"
		//   filepath.Clean(".//foo") -> "foo"
		normalizedPattern = filepath.Clean(normalizedPattern)

		// Step 3: Convert to forward slashes for consistent glob matching
		//   (doublestar uses Unix-style paths regardless of OS)
		normalizedPattern = filepath.ToSlash(normalizedPattern)

		// Use doublestar for glob pattern matching
		if matched, err := doublestar.Match(normalizedPattern, path); err == nil && matched {
			return true
		}

		// Also try with just the filename for simple patterns
		// BUT: if the original pattern started with ./ or .\, don't do basename matching
		// because that indicates the user wants to match the root file only
		if !isRootOnly && !strings.Contains(normalizedPattern, "/") {
			filename := filepath.Base(path)
			if matched, err := doublestar.Match(normalizedPattern, filename); err == nil && matched {
				return true
			}
		}
	}
	return false
}

// isHiddenFile checks if a file is hidden (starts with . or is in a hidden directory)
func (d *DiscoveryEngine) isHiddenFile(path string) bool {
	parts := strings.Split(path, string(filepath.Separator))
	for _, part := range parts {
		if strings.HasPrefix(part, ".") && len(part) > 1 {
			return true
		}
	}
	return false
}

func calculateDepth(relPath string, isDir bool) int {
	if relPath == "" || relPath == "." {
		return 0
	}
	normalized := filepath.ToSlash(relPath)
	normalized = strings.Trim(normalized, "/")
	if normalized == "" {
		return 0
	}
	segments := strings.Split(normalized, "/")
	if !isDir && len(segments) > 0 {
		return len(segments) - 1
	}
	return len(segments)
}

// FindFilesByType finds files of specific types using common patterns
func (d *DiscoveryEngine) FindFilesByType(basePath, fileType string) ([]string, error) {
	var patterns []string

	switch strings.ToLower(fileType) {
	case "go":
		patterns = []string{"*.go"}
	case "javascript", "js":
		patterns = []string{"*.js", "*.jsx", "*.ts", "*.tsx"}
	case "python", "py":
		patterns = []string{"*.py"}
	case "java":
		patterns = []string{"*.java"}
	case "config":
		patterns = []string{"*.json", "*.yaml", "*.yml", "*.toml", "*.ini", "*.cfg"}
	case "docs":
		patterns = []string{"*.md", "*.txt", "README*", "CHANGELOG*"}
	case "images":
		patterns = []string{"*.jpg", "*.jpeg", "*.png", "*.gif", "*.svg", "*.webp"}
	default:
		return nil, fmt.Errorf("unsupported file type: %s", fileType)
	}

	opts := DiscoveryOptions{
		IncludePatterns: patterns,
		IncludeHidden:   false,
	}

	return d.DiscoverFiles(basePath, opts)
}

// FindByContent finds files containing specific content patterns
func (d *DiscoveryEngine) FindByContent(basePath string, contentPattern string, opts DiscoveryOptions) ([]string, error) {
	// First discover files by pattern
	files, err := d.DiscoverFiles(basePath, opts)
	if err != nil {
		return nil, err
	}

	var matchingFiles []string

	for _, file := range files {
		fullPath := filepath.Join(basePath, file)

		// Read file content
		content, err := os.ReadFile(fullPath) // #nosec G304 - path constructed from validated basePath and file list
		if err != nil {
			continue // Skip files we can't read
		}

		// Check if content matches pattern
		if strings.Contains(string(content), contentPattern) {
			matchingFiles = append(matchingFiles, file)
		}
	}

	return matchingFiles, nil
}

// GetDirectoryStats returns statistics about a directory
func (d *DiscoveryEngine) GetDirectoryStats(basePath string) (*DirectoryStats, error) {
	if err := d.validator.ValidatePath(basePath); err != nil {
		return nil, fmt.Errorf("base path validation failed: %w", err)
	}

	stats := &DirectoryStats{
		Path: basePath,
	}

	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			stats.DirectoryCount++
		} else {
			stats.FileCount++
			stats.TotalSize += info.Size()

			// Track file types
			ext := strings.ToLower(filepath.Ext(path))
			if ext != "" {
				stats.FileTypes[ext]++
			}

			// Track largest files
			if len(stats.LargestFiles) < 10 {
				stats.LargestFiles = append(stats.LargestFiles, FileSizeInfo{
					Path: path,
					Size: info.Size(),
				})
			} else {
				// Replace smallest in top 10
				minIdx := 0
				minSize := stats.LargestFiles[0].Size
				for i, file := range stats.LargestFiles {
					if file.Size < minSize {
						minSize = file.Size
						minIdx = i
					}
				}
				if info.Size() > minSize {
					stats.LargestFiles[minIdx] = FileSizeInfo{
						Path: path,
						Size: info.Size(),
					}
				}
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("directory stats collection failed: %w", err)
	}

	// Sort largest files
	sort.Slice(stats.LargestFiles, func(i, j int) bool {
		return stats.LargestFiles[i].Size > stats.LargestFiles[j].Size
	})

	return stats, nil
}

// DirectoryStats contains statistics about a directory
type DirectoryStats struct {
	Path           string           `json:"path"`
	FileCount      int64            `json:"file_count"`
	DirectoryCount int64            `json:"directory_count"`
	TotalSize      int64            `json:"total_size"`
	FileTypes      map[string]int64 `json:"file_types"`
	LargestFiles   []FileSizeInfo   `json:"largest_files"`
}

// FileSizeInfo contains file path and size information
type FileSizeInfo struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

// BatchDiscover performs discovery on multiple base paths concurrently
func (d *DiscoveryEngine) BatchDiscover(basePaths []string, opts DiscoveryOptions, concurrency int) (map[string][]string, error) {
	if concurrency <= 0 {
		concurrency = 1
	}

	type result struct {
		basePath string
		files    []string
		err      error
	}

	results := make(chan result, len(basePaths))
	semaphore := make(chan struct{}, concurrency)

	for _, basePath := range basePaths {
		go func(bp string) {
			semaphore <- struct{}{}        // Acquire
			defer func() { <-semaphore }() // Release

			files, err := d.DiscoverFiles(bp, opts)
			results <- result{basePath: bp, files: files, err: err}
		}(basePath)
	}

	// Collect results
	batchResults := make(map[string][]string)
	for i := 0; i < len(basePaths); i++ {
		res := <-results
		if res.err != nil {
			return nil, fmt.Errorf("discovery failed for %s: %w", res.basePath, res.err)
		}
		batchResults[res.basePath] = res.files
	}

	return batchResults, nil
}
