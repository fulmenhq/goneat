package pathfinder

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// SafeWalker provides safe directory traversal with enhanced features
type SafeWalker struct {
	validator      *SafetyValidator
	maxConcurrency int
	maxDepth       int
}

// walkJob represents a job for concurrent directory walking
type walkJobStruct struct {
	path  string
	info  os.FileInfo
	depth int
}

// NewSafeWalker creates a new safe directory walker
func NewSafeWalker(validator *SafetyValidator) *SafeWalker {
	return &SafeWalker{
		validator:      validator,
		maxConcurrency: 10,
		maxDepth:       20,
	}
}

// WalkDirectory safely walks a directory tree with enhanced features
func (w *SafeWalker) WalkDirectory(basePath string, walkFunc WalkFunc, opts WalkOptions) error {
	// Validate base path
	if err := w.validator.ValidatePath(basePath); err != nil {
		return fmt.Errorf("base path validation failed: %w", err)
	}

	// Set default concurrency if not specified
	if opts.Concurrency <= 0 {
		opts.Concurrency = w.maxConcurrency
	}

	// Set default max depth if not specified
	if opts.MaxDepth <= 0 {
		opts.MaxDepth = w.maxDepth
	}

	// For concurrent walking, use a worker pool
	if opts.Concurrency > 1 {
		return w.walkConcurrent(basePath, walkFunc, opts)
	}

	// For single-threaded walking, use standard filepath.Walk
	return w.walkSequential(basePath, walkFunc, opts)
}

// walkSequential performs single-threaded directory walking
func (w *SafeWalker) walkSequential(basePath string, walkFunc WalkFunc, opts WalkOptions) error {
	var processed int64

	walkFuncWrapper := func(path string, info os.FileInfo, err error) error {
		// Handle walk errors
		if err != nil {
			if opts.ErrorHandler != nil {
				return opts.ErrorHandler(path, err)
			}
			return err
		}

		// Check depth limit
		if opts.MaxDepth > 0 {
			relPath, relErr := filepath.Rel(basePath, path)
			if relErr == nil {
				depth := strings.Count(relPath, string(filepath.Separator))
				if depth > opts.MaxDepth {
					if info.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}
		}

		// Skip directories if configured
		if info.IsDir() {
			for _, skipDir := range opts.SkipDirs {
				if strings.Contains(path, skipDir) {
					return filepath.SkipDir
				}
			}
		}

		// Skip files based on patterns
		if len(opts.SkipPatterns) > 0 {
			for _, pattern := range opts.SkipPatterns {
				if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
					if info.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}
		}

		// Validate path safety
		if err := w.validator.ValidatePath(path); err != nil {
			if opts.ErrorHandler != nil {
				return opts.ErrorHandler(path, err)
			}
			return err
		}

		// Call user walk function
		walkErr := walkFunc(path, w.convertFileInfo(info), nil)

		// Update progress
		atomic.AddInt64(&processed, 1)
		if opts.ProgressCallback != nil && atomic.LoadInt64(&processed)%100 == 0 {
			opts.ProgressCallback(int(processed), -1, path)
		}

		return walkErr
	}

	err := filepath.Walk(basePath, walkFuncWrapper)

	// Final progress callback
	if opts.ProgressCallback != nil {
		opts.ProgressCallback(int(atomic.LoadInt64(&processed)), int(atomic.LoadInt64(&processed)), "walk complete")
	}

	return err
}

// walkConcurrent performs concurrent directory walking using a worker pool
func (w *SafeWalker) walkConcurrent(basePath string, walkFunc WalkFunc, opts WalkOptions) error {
	type walkResult struct {
		err error
	}

	jobs := make(chan walkJobStruct, 1000)
	results := make(chan walkResult, opts.Concurrency)
	var processed int64
	var wg sync.WaitGroup

	// Start worker goroutines
	for i := 0; i < opts.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				// Process the file/directory
				err := w.processWalkJob(job, walkFunc, opts, &processed)
				results <- walkResult{err: err}
			}
		}()
	}

	// Start result collector
	go func() {
		wg.Wait()
		close(results)
	}()

	// Start directory traversal goroutine
	go func() {
		defer close(jobs)
		w.traverseDirectories(basePath, jobs, opts, 0)
	}()

	// Collect results and check for errors
	for result := range results {
		if result.err != nil {
			return result.err
		}
	}

	return nil
}

// traverseDirectories recursively traverses directories and sends jobs to workers
func (w *SafeWalker) traverseDirectories(basePath string, jobs chan<- walkJobStruct, opts WalkOptions, depth int) {
	// Check depth limit
	if opts.MaxDepth > 0 && depth > opts.MaxDepth {
		return
	}

	entries, err := os.ReadDir(basePath)
	if err != nil {
		// Send error job
		jobs <- walkJobStruct{path: basePath, depth: depth}
		return
	}

	for _, entry := range entries {
		path := filepath.Join(basePath, entry.Name())

		// Check if directory should be skipped
		if entry.IsDir() {
			skip := false
			for _, skipDir := range opts.SkipDirs {
				if strings.Contains(path, skipDir) {
					skip = true
					break
				}
			}
			if skip {
				continue
			}
		}

		// Check skip patterns
		shouldSkip := false
		if len(opts.SkipPatterns) > 0 {
			for _, pattern := range opts.SkipPatterns {
				if matched, _ := filepath.Match(pattern, entry.Name()); matched {
					shouldSkip = true
					break
				}
			}
		}

		if shouldSkip {
			continue
		}

		// Get file info
		info, err := entry.Info()
		if err != nil {
			jobs <- walkJobStruct{path: path, depth: depth}
			continue
		}

		// Send job to worker
		jobs <- walkJobStruct{
			path:  path,
			info:  info,
			depth: depth,
		}

		// Recursively traverse subdirectories
		if entry.IsDir() {
			w.traverseDirectories(path, jobs, opts, depth+1)
		}
	}
}

// processWalkJob processes a single walk job
func (w *SafeWalker) processWalkJob(job walkJobStruct, walkFunc WalkFunc, opts WalkOptions, processed *int64) error {
	// Validate path safety
	if err := w.validator.ValidatePath(job.path); err != nil {
		if opts.ErrorHandler != nil {
			return opts.ErrorHandler(job.path, err)
		}
		return err
	}

	// Call user walk function
	walkErr := walkFunc(job.path, w.convertFileInfo(job.info), nil)

	// Update progress
	atomic.AddInt64(processed, 1)
	if opts.ProgressCallback != nil && atomic.LoadInt64(processed)%100 == 0 {
		opts.ProgressCallback(int(atomic.LoadInt64(processed)), -1, job.path)
	}

	return walkErr
}

// convertFileInfo converts os.FileInfo to pathfinder.FileInfo
func (w *SafeWalker) convertFileInfo(info os.FileInfo) FileInfo {
	return &fileInfoAdapter{FileInfo: info}
}

// fileInfoAdapter adapts os.FileInfo to pathfinder.FileInfo
type fileInfoAdapter struct {
	os.FileInfo
}

func (f *fileInfoAdapter) Mode() FileMode {
	return FileMode(f.FileInfo.Mode())
}

func (f *fileInfoAdapter) Sys() interface{} {
	return f.FileInfo.Sys()
}

// SetMaxConcurrency sets the maximum concurrency for concurrent walking
func (w *SafeWalker) SetMaxConcurrency(concurrency int) {
	if concurrency > 0 {
		w.maxConcurrency = concurrency
	}
}

// SetMaxDepth sets the maximum directory depth for walking
func (w *SafeWalker) SetMaxDepth(depth int) {
	if depth > 0 {
		w.maxDepth = depth
	}
}

// WalkStats provides statistics about a directory walk
type WalkStats struct {
	Path        string        `json:"path"`
	TotalFiles  int64         `json:"total_files"`
	TotalDirs   int64         `json:"total_dirs"`
	TotalSize   int64         `json:"total_size"`
	Duration    time.Duration `json:"duration"`
	Errors      []string      `json:"errors,omitempty"`
	SkippedDirs []string      `json:"skipped_dirs,omitempty"`
}

// CollectWalkStats collects statistics during a directory walk
func (w *SafeWalker) CollectWalkStats(basePath string, opts WalkOptions) (*WalkStats, error) {
	stats := &WalkStats{
		Path: basePath,
	}
	start := time.Now()

	walkFunc := func(path string, info FileInfo, err error) error {
		if err != nil {
			stats.Errors = append(stats.Errors, fmt.Sprintf("%s: %v", path, err))
			return nil
		}

		if info.IsDir() {
			stats.TotalDirs++
		} else {
			stats.TotalFiles++
			stats.TotalSize += info.Size()
		}

		return nil
	}

	err := w.WalkDirectory(basePath, walkFunc, opts)
	stats.Duration = time.Since(start)

	if err != nil {
		return stats, err
	}

	return stats, nil
}
