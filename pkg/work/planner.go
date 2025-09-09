package work

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/fulmenhq/goneat/pkg/ignore"
	"github.com/fulmenhq/goneat/pkg/logger"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// WorkItem represents a single file to be processed
type WorkItem struct {
	ID            string                 `json:"id"`
	Path          string                 `json:"path"`
	ContentType   string                 `json:"content_type"`
	Size          int64                  `json:"size"`
	Priority      int                    `json:"priority"`
	EstimatedTime float64                `json:"estimated_time"`
	Dependencies  []string               `json:"dependencies"`
	Metadata      map[string]interface{} `json:"metadata"`
}

// WorkGroup represents a logical grouping of work items
type WorkGroup struct {
	ID                         string   `json:"id"`
	Name                       string   `json:"name"`
	Strategy                   string   `json:"strategy"`
	WorkItemIDs                []string `json:"work_item_ids"`
	EstimatedTotalTime         float64  `json:"estimated_total_time"`
	RecommendedParallelization int      `json:"recommended_parallelization"`
}

// Plan represents the complete execution plan
type Plan struct {
	Command           string    `json:"command"`
	Timestamp         time.Time `json:"timestamp"`
	WorkingDirectory  string    `json:"working_directory"`
	TotalFiles        int       `json:"total_files"`
	FilteredFiles     int       `json:"filtered_files"`
	RedundantPaths    []string  `json:"redundant_paths"`
	ExecutionStrategy string    `json:"execution_strategy"`
}

// Statistics provides statistical information about the work plan
type Statistics struct {
	FilesByType            map[string]int     `json:"files_by_type"`
	SizeDistribution       SizeStats          `json:"size_distribution"`
	PathDepthDistribution  map[int]int        `json:"path_depth_distribution"`
	EstimatedExecutionTime ExecutionEstimates `json:"estimated_execution_time"`
}

// SizeStats provides file size statistics
type SizeStats struct {
	MinSize   int64   `json:"min_size"`
	MaxSize   int64   `json:"max_size"`
	AvgSize   float64 `json:"avg_size"`
	TotalSize int64   `json:"total_size"`
}

// ExecutionEstimates provides time estimates for different strategies
type ExecutionEstimates struct {
	Sequential float64 `json:"sequential"`
	Parallel2  float64 `json:"parallel_2"`
	Parallel4  float64 `json:"parallel_4"`
	Parallel8  float64 `json:"parallel_8"`
}

// WorkManifest represents the complete work plan
type WorkManifest struct {
	Plan       Plan        `json:"plan"`
	WorkItems  []WorkItem  `json:"work_items"`
	Groups     []WorkGroup `json:"groups"`
	Statistics Statistics  `json:"statistics"`
}

// PlannerConfig configures the work planner
type PlannerConfig struct {
	Command            string
	Paths              []string
	IncludePatterns    []string
	ExcludePatterns    []string
	MaxDepth           int
	ContentTypes       []string
	ExecutionStrategy  string
	GroupBySize        bool
	GroupByContentType bool
	IgnoreFile         string // Path to ignore file (e.g., .goneatignore)
	EnableFinalizer    bool   // Whether finalizer operations are enabled
	Verbose            bool   // Enable verbose logging for skipped files
	// Ignore overrides
	NoIgnore             bool     // Disable .goneatignore/.gitignore matching entirely
	ForceIncludePatterns []string // Paths/globs to force-include even if ignored
	IncludeConfigDirs    bool     // Include common configuration directories (.claude, .vscode, etc.)
}

// Planner handles work planning and manifest generation
type Planner struct {
	config        PlannerConfig
	ignoreMatcher *ignore.Matcher
}

// NewPlanner creates a new work planner
func NewPlanner(config PlannerConfig) *Planner {
	planner := &Planner{config: config}

	// Initialize ignore matcher
	if config.NoIgnore {
		planner.ignoreMatcher = nil
	} else if matcher, err := ignore.NewMatcher("."); err != nil {
		logger.Warn(fmt.Sprintf("Failed to initialize ignore matcher: %v", err))
		planner.ignoreMatcher = nil
	} else {
		planner.ignoreMatcher = matcher
	}

	return planner
}

// matchesIgnorePattern checks if a path matches any ignore pattern using go-git
func (p *Planner) matchesIgnorePattern(path string) bool {
	if p.ignoreMatcher == nil {
		return false
	}
	return p.ignoreMatcher.IsIgnored(path)
}

// matchesForceInclude checks if a path matches any force-include pattern
func (p *Planner) matchesForceInclude(path string) bool {
	if len(p.config.ForceIncludePatterns) == 0 {
		return false
	}
	// Normalize to forward slashes for matching
	rel := filepath.ToSlash(path)
	// Ensure relative from WD for consistency
	if wd, err := os.Getwd(); err == nil {
		if r, err := filepath.Rel(wd, path); err == nil {
			rel = filepath.ToSlash(r)
		}
	}
	for _, pat := range p.config.ForceIncludePatterns {
		pat = filepath.ToSlash(strings.TrimSpace(pat))
		if pat == "" {
			continue
		}
		// Directory recursive include: suffix '/**' or explicit directory path
		if strings.HasSuffix(pat, "/**") {
			prefix := strings.TrimSuffix(pat, "/**")
			if rel == prefix || strings.HasPrefix(rel, prefix+"/") {
				return true
			}
		} else {
			// Try full-path match first
			if ok, _ := pathMatch(pat, rel); ok {
				return true
			}
			// Also try basename match for simple globs like '*.yaml'
			base := filepath.Base(rel)
			if ok, _ := pathMatch(pat, base); ok {
				return true
			}
			// If pattern is a directory path, treat as recursive include
			if fi, err := os.Stat(pat); err == nil && fi.IsDir() {
				dir := filepath.ToSlash(pat)
				if rel == dir || strings.HasPrefix(rel, dir+"/") {
					return true
				}
			}
		}
	}
	return false
}

// dirHasForcedDescendant returns true if any force-include pattern lies under the given directory path.
func (p *Planner) dirHasForcedDescendant(dir string) bool {
	if len(p.config.ForceIncludePatterns) == 0 {
		return false
	}
	// Normalize directory path relative to WD with forward slashes
	rel := filepath.ToSlash(dir)
	if wd, err := os.Getwd(); err == nil {
		if r, err := filepath.Rel(wd, dir); err == nil {
			rel = filepath.ToSlash(r)
		}
	}
	if rel != "" && !strings.HasSuffix(rel, "/") {
		rel += "/"
	}
	for _, raw := range p.config.ForceIncludePatterns {
		pat := filepath.ToSlash(strings.TrimSpace(raw))
		if pat == "" {
			continue
		}
		// Derive anchor before any wildcard
		anchor := pat
		if i := strings.IndexAny(anchor, "*[?"); i >= 0 {
			anchor = anchor[:i]
		}
		anchor = strings.TrimSuffix(anchor, "**")
		anchor = strings.TrimSuffix(anchor, "*")
		anchor = strings.TrimSuffix(anchor, "?")
		anchor = strings.Trim(anchor, "/")
		if anchor == "" {
			continue
		}
		// Ensure anchor normalized as directory-like string
		a := anchor
		if !strings.HasSuffix(a, "/") {
			a += "/"
		}
		// If anchor is deeper than dir (i.e., inside this dir), keep descending
		if strings.HasPrefix(a, rel) {
			return true
		}
	}
	return false
}

// pathMatch performs glob-style matching on slash-separated paths
func pathMatch(pattern, name string) (bool, error) {
	// Use filepath.Match but with forward slashes; it treats path separator specially on current OS.
	// Patterns are already normalized to '/'. For safety, fall back to simple compare if pattern has no glob.
	if !strings.ContainsAny(pattern, "*?[") {
		return pattern == name, nil
	}
	// Try matching against forward-slash normalized name
	return filepath.Match(pattern, name)
}

// GenerateManifest generates a complete work manifest
func (p *Planner) GenerateManifest() (*WorkManifest, error) {
	logger.Info("Starting work plan generation")

	// Discover all files
	allFiles, err := p.discoverFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to discover files: %v", err)
	}

	// Eliminate redundancies
	filteredFiles, redundantPaths := p.eliminateRedundancies(allFiles)

	// Create work items
	workItems := p.createWorkItems(filteredFiles)

	// Create groups
	groups := p.createGroups(workItems)

	// Calculate statistics
	stats := p.calculateStatistics(workItems, filteredFiles)

	// Create plan
	plan := Plan{
		Command:           p.config.Command,
		Timestamp:         time.Now(),
		WorkingDirectory:  p.getWorkingDirectory(),
		TotalFiles:        len(allFiles),
		FilteredFiles:     len(filteredFiles),
		RedundantPaths:    redundantPaths,
		ExecutionStrategy: p.config.ExecutionStrategy,
	}

	manifest := &WorkManifest{
		Plan:       plan,
		WorkItems:  workItems,
		Groups:     groups,
		Statistics: stats,
	}

	logger.Info(fmt.Sprintf("Generated work manifest with %d work items in %d groups", len(workItems), len(groups)))
	return manifest, nil
}

// discoverFiles finds all files matching the criteria
func (p *Planner) discoverFiles() ([]string, error) {
	var allFiles []string

	for _, basePath := range p.config.Paths {
		err := filepath.WalkDir(basePath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			// Skip directories if they match ignore patterns (unless overridden)
			if d.IsDir() {
				// Check if this is a configuration directory that should be included
				isConfigDir := p.config.IncludeConfigDirs && p.isConfigDirectory(path)

				if p.ignoreMatcher != nil && p.ignoreMatcher.IsIgnoredDir(path) && !p.config.NoIgnore && !isConfigDir {
					if p.config.Verbose {
						logger.Debug(fmt.Sprintf("Skipping directory %s: matches ignore pattern", path))
					}
					// If any force-include pattern targets this directory or a descendant, do not skip
					if p.matchesForceInclude(path) || p.dirHasForcedDescendant(path) {
						return nil
					}
					return filepath.SkipDir
				}

				// Skip directories if we have depth limits
				if p.config.MaxDepth > 0 {
					relPath, err := filepath.Rel(basePath, path)
					if err != nil {
						return err
					}
					depth := strings.Count(relPath, string(filepath.Separator))
					if depth > p.config.MaxDepth {
						return filepath.SkipDir
					}
				}
				return nil
			}

			// Check if file matches our criteria
			if p.shouldIncludeFile(path) {
				allFiles = append(allFiles, path)
			}

			return nil
		})

		if err != nil {
			return nil, err
		}
	}

	return allFiles, nil
}

// shouldIncludeFile determines if a file should be included
func (p *Planner) shouldIncludeFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))

	// Check .goneatignore patterns first (unless overridden or forced)
	if !p.config.NoIgnore && p.matchesIgnorePattern(path) && !p.matchesForceInclude(path) {
		if p.config.Verbose {
			logger.Debug(fmt.Sprintf("Skipping %s: matches ignore pattern", path))
		}
		return false
	}

	// Check content type filters
	if len(p.config.ContentTypes) > 0 {
		contentType := p.getContentType(ext)
		found := false
		for _, allowed := range p.config.ContentTypes {
			if allowed == contentType {
				found = true
				break
			}
		}
		if !found {
			if p.config.Verbose {
				logger.Debug(fmt.Sprintf("Skipping %s: content type '%s' not in allowed types %v", path, contentType, p.config.ContentTypes))
			}
			return false
		}
	}

	// Check include patterns
	if len(p.config.IncludePatterns) > 0 {
		matched := false
		for _, pattern := range p.config.IncludePatterns {
			if matched, _ = filepath.Match(pattern, filepath.Base(path)); matched {
				break
			}
		}
		if !matched {
			if p.config.Verbose {
				logger.Debug(fmt.Sprintf("Skipping %s: does not match include patterns %v", path, p.config.IncludePatterns))
			}
			return false
		}
	}

	// Check exclude patterns
	for _, pattern := range p.config.ExcludePatterns {
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			if p.config.Verbose {
				logger.Debug(fmt.Sprintf("Skipping %s: matches exclude pattern '%s'", path, pattern))
			}
			return false
		}
	}

	// File passed all filters - include it
	if p.config.Verbose {
		contentType := p.getContentType(ext)
		logger.Debug(fmt.Sprintf("Including %s (type: %s)", path, contentType))
	}

	return true
}

// getContentType determines content type from file extension
func (p *Planner) getContentType(ext string) string {
	switch ext {
	case ".go":
		return "go"
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return "json"
	case ".md", ".markdown":
		return "markdown"
	case ".txt":
		return "text"
	case ".sh":
		return "shell"
	case ".py":
		return "python"
	case ".js", ".jsx":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".html", ".htm":
		return "html"
	case ".css":
		return "css"
	case ".xml":
		return "xml"
	case ".toml":
		return "toml"
	case ".ini", ".cfg", ".conf":
		return "config"
	default:
		return "unknown"
	}
}

// eliminateRedundancies removes redundant paths
func (p *Planner) eliminateRedundancies(files []string) ([]string, []string) {
	// Only remove exact duplicate file paths while preserving order.
	if len(files) <= 1 {
		return files, nil
	}

	var filtered []string
	var redundant []string
	seen := make(map[string]bool, len(files))

	for _, file := range files {
		if !seen[file] {
			filtered = append(filtered, file)
			seen[file] = true
		} else {
			redundant = append(redundant, file)
		}
	}

	return filtered, redundant
}

// createWorkItems creates work items from file paths
func (p *Planner) createWorkItems(files []string) []WorkItem {
	var workItems []WorkItem

	for _, file := range files {
		// Validate that file path contains a path separator or extension
		// to catch corrupted paths like just "json"
		if !strings.Contains(file, string(filepath.Separator)) && !strings.Contains(file, ".") {
			logger.Debug(fmt.Sprintf("Skipping invalid file path: '%s'", file))
			continue
		}

		stat, err := os.Stat(file)
		if err != nil {
			logger.Debug(fmt.Sprintf("Skipping file that can't be stat'd: '%s': %v", file, err))
			continue
		}

		contentType := p.getContentType(strings.ToLower(filepath.Ext(file)))
		size := stat.Size()

		// Generate unique ID using SHA-256 for better security
		id := fmt.Sprintf("%x", sha256.Sum256([]byte(file)))

		// Estimate processing time based on size and content type
		estimatedTime := p.estimateProcessingTime(contentType, size)

		// Calculate priority (larger files get higher priority for parallelization)
		priority := int(size / 1024) // Priority based on KB
		if priority > 100 {
			priority = 100
		}

		workItem := WorkItem{
			ID:            id,
			Path:          file,
			ContentType:   contentType,
			Size:          size,
			Priority:      priority,
			EstimatedTime: estimatedTime,
			Dependencies:  []string{}, // No dependencies for now
			Metadata:      make(map[string]interface{}),
		}

		workItems = append(workItems, workItem)
	}

	return workItems
}

// estimateProcessingTime estimates processing time for a file
func (p *Planner) estimateProcessingTime(contentType string, size int64) float64 {
	// Base time per KB
	baseTimePerKB := map[string]float64{
		"go":       0.5, // Go formatting is fast
		"yaml":     1.0, // YAML parsing is more complex
		"json":     0.8, // JSON is relatively fast
		"markdown": 1.2, // Markdown can be complex
	}

	timePerKB := baseTimePerKB[contentType]
	if timePerKB == 0 {
		timePerKB = 1.0 // Default
	}

	kb := float64(size) / 1024
	return kb * timePerKB
}

// createGroups creates logical groups of work items
func (p *Planner) createGroups(workItems []WorkItem) []WorkGroup {
	var groups []WorkGroup

	if p.config.GroupByContentType {
		groups = append(groups, p.groupByContentType(workItems)...)
	}

	if p.config.GroupBySize {
		groups = append(groups, p.groupBySize(workItems)...)
	}

	// If no grouping specified, create a single group
	if len(groups) == 0 {
		totalTime := 0.0
		ids := make([]string, len(workItems))
		for i, item := range workItems {
			ids[i] = item.ID
			totalTime += item.EstimatedTime
		}

		groups = append(groups, WorkGroup{
			ID:                         "all",
			Name:                       "All Files",
			Strategy:                   "single_group",
			WorkItemIDs:                ids,
			EstimatedTotalTime:         totalTime,
			RecommendedParallelization: runtime.NumCPU(),
		})
	}

	return groups
}

// groupByContentType groups work items by content type
func (p *Planner) groupByContentType(workItems []WorkItem) []WorkGroup {
	groups := make(map[string][]WorkItem)

	for _, item := range workItems {
		groups[item.ContentType] = append(groups[item.ContentType], item)
	}

	var result []WorkGroup
	for contentType, items := range groups {
		totalTime := 0.0
		ids := make([]string, len(items))
		for i, item := range items {
			ids[i] = item.ID
			totalTime += item.EstimatedTime
		}

		c := cases.Title(language.Und)
		result = append(result, WorkGroup{
			ID:                         fmt.Sprintf("content_type_%s", contentType),
			Name:                       fmt.Sprintf("%s Files", c.String(contentType)),
			Strategy:                   "content_type_based",
			WorkItemIDs:                ids,
			EstimatedTotalTime:         totalTime,
			RecommendedParallelization: p.calculateParallelization(len(items), totalTime),
		})
	}

	return result
}

// groupBySize groups work items by file size
func (p *Planner) groupBySize(workItems []WorkItem) []WorkGroup {
	// Sort by size
	sort.Slice(workItems, func(i, j int) bool {
		return workItems[i].Size > workItems[j].Size // Largest first
	})

	// Create size-based groups
	var largeFiles, mediumFiles, smallFiles []WorkItem

	for _, item := range workItems {
		if item.Size > 100*1024 { // > 100KB
			largeFiles = append(largeFiles, item)
		} else if item.Size > 10*1024 { // > 10KB
			mediumFiles = append(mediumFiles, item)
		} else {
			smallFiles = append(smallFiles, item)
		}
	}

	var groups []WorkGroup

	if len(largeFiles) > 0 {
		groups = append(groups, p.createSizeGroup("large", "Large Files (>100KB)", largeFiles))
	}
	if len(mediumFiles) > 0 {
		groups = append(groups, p.createSizeGroup("medium", "Medium Files (10-100KB)", mediumFiles))
	}
	if len(smallFiles) > 0 {
		groups = append(groups, p.createSizeGroup("small", "Small Files (<10KB)", smallFiles))
	}

	return groups
}

// createSizeGroup creates a work group for size-based grouping
func (p *Planner) createSizeGroup(id, name string, items []WorkItem) WorkGroup {
	totalTime := 0.0
	ids := make([]string, len(items))
	for i, item := range items {
		ids[i] = item.ID
		totalTime += item.EstimatedTime
	}

	return WorkGroup{
		ID:                         id,
		Name:                       name,
		Strategy:                   "size_based",
		WorkItemIDs:                ids,
		EstimatedTotalTime:         totalTime,
		RecommendedParallelization: p.calculateParallelization(len(items), totalTime),
	}
}

// calculateParallelization calculates recommended parallelization level
func (p *Planner) calculateParallelization(itemCount int, totalTime float64) int {
	maxWorkers := runtime.NumCPU()

	// For small numbers of items, use fewer workers
	if itemCount < maxWorkers {
		return itemCount
	}

	// For large total time, use more workers
	if totalTime > 1000 { // > 1 second
		return maxWorkers
	}

	// Default to half the CPU count for balanced workloads
	return maxWorkers / 2
}

// calculateStatistics calculates statistical information
func (p *Planner) calculateStatistics(workItems []WorkItem, files []string) Statistics {
	stats := Statistics{
		FilesByType:           make(map[string]int),
		PathDepthDistribution: make(map[int]int),
	}

	var totalSize int64
	var minSize, maxSize int64 = -1, 0

	for _, item := range workItems {
		stats.FilesByType[item.ContentType]++

		totalSize += item.Size
		if minSize == -1 || item.Size < minSize {
			minSize = item.Size
		}
		if item.Size > maxSize {
			maxSize = item.Size
		}
	}

	if len(workItems) > 0 {
		stats.SizeDistribution = SizeStats{
			MinSize:   minSize,
			MaxSize:   maxSize,
			AvgSize:   float64(totalSize) / float64(len(workItems)),
			TotalSize: totalSize,
		}
	}

	// Calculate path depth distribution
	for _, file := range files {
		depth := strings.Count(file, string(filepath.Separator))
		stats.PathDepthDistribution[depth]++
	}

	// Estimate execution times
	totalTime := 0.0
	for _, item := range workItems {
		totalTime += item.EstimatedTime
	}

	stats.EstimatedExecutionTime = ExecutionEstimates{
		Sequential: totalTime,
		Parallel2:  totalTime / 2 * 1.1, // 10% overhead
		Parallel4:  totalTime / 4 * 1.2, // 20% overhead
		Parallel8:  totalTime / 8 * 1.3, // 30% overhead
	}

	return stats
}

// getWorkingDirectory returns the current working directory
func (p *Planner) getWorkingDirectory() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return wd
}

// isConfigDirectory checks if a directory is a common configuration directory
func (p *Planner) isConfigDirectory(path string) bool {
	baseName := filepath.Base(path)

	// Common configuration directories
	configDirs := []string{
		".claude",
		".vscode",
		".idea",
		".vs",
		".eclipse",
		".settings",
		".metadata",
		".project",
		".classpath",
		".config",
		".local",
		".cache",
		"node_modules",
		".git",
		".gitignore",
		".goneatignore",
		".github",
		".gitlab",
		".circleci",
		".travis",
		".jenkins",
		".ci",
		"docs",
		"packaging",
		"scripts",
		"templates",
	}

	for _, configDir := range configDirs {
		if baseName == configDir {
			return true
		}
	}

	return false
}
