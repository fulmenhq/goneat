package work

import (
	"bufio"
	"crypto/md5"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/3leaps/goneat/pkg/logger"
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
}

// Planner handles work planning and manifest generation
type Planner struct {
	config         PlannerConfig
	ignorePatterns []string
}

// NewPlanner creates a new work planner
func NewPlanner(config PlannerConfig) *Planner {
	planner := &Planner{config: config}
	planner.loadIgnorePatterns()
	return planner
}

// loadIgnorePatterns reads ignore patterns from .goneatignore files
func (p *Planner) loadIgnorePatterns() {
	var patterns []string

	// Load repo-level .goneatignore
	if repoIgnore, err := p.readIgnoreFile(".goneatignore"); err == nil {
		patterns = append(patterns, repoIgnore...)
	}

	// Load user-level .goneatignore
	if homeDir, err := os.UserHomeDir(); err == nil {
		userIgnorePath := filepath.Join(homeDir, ".goneatignore")
		if userIgnore, err := p.readIgnoreFile(userIgnorePath); err == nil {
			patterns = append(patterns, userIgnore...)
		}
	}

	p.ignorePatterns = patterns
}

// readIgnoreFile reads patterns from an ignore file
func (p *Planner) readIgnoreFile(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			logger.Warn(fmt.Sprintf("Failed to close ignore file %s: %v", path, closeErr))
		}
	}()

	var patterns []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}

	return patterns, scanner.Err()
}

// matchesIgnorePattern checks if a path matches any ignore pattern
func (p *Planner) matchesIgnorePattern(path string) bool {
	// Get relative path from working directory for pattern matching
	wd, err := os.Getwd()
	if err != nil {
		return false
	}

	relPath, err := filepath.Rel(wd, path)
	if err != nil {
		relPath = path
	}

	for _, pattern := range p.ignorePatterns {
		if p.matchesPattern(pattern, relPath) {
			return true
		}
	}

	return false
}

// matchesPattern checks if a path matches a gitignore-style pattern
func (p *Planner) matchesPattern(pattern, path string) bool {
	// Handle directory patterns (ending with /)
	if strings.HasSuffix(pattern, "/") {
		// If path is a directory and matches the pattern, ignore it
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return strings.Contains(path, strings.TrimSuffix(pattern, "/"))
		}
		return false
	}

	// Handle negation patterns (starting with !)
	if strings.HasPrefix(pattern, "!") {
		negatedPattern := strings.TrimPrefix(pattern, "!")
		return !p.matchesSimplePattern(negatedPattern, path)
	}

	return p.matchesSimplePattern(pattern, path)
}

// matchesSimplePattern performs basic gitignore-style pattern matching
func (p *Planner) matchesSimplePattern(pattern, path string) bool {
	// Handle glob patterns with *
	if strings.Contains(pattern, "*") {
		// Simple glob matching for patterns like *.log, test.*
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return true
		}
		// Also check full path
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}
	}

	// Handle directory patterns
	if strings.Contains(path, pattern) {
		return true
	}

	// Handle exact matches
	if filepath.Base(path) == pattern {
		return true
	}

	return false
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

			// Skip directories if we have depth limits
			if d.IsDir() {
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

	// Check .goneatignore patterns first
	if p.matchesIgnorePattern(path) {
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
			return false
		}
	}

	// Check include patterns
	if len(p.config.IncludePatterns) > 0 {
		matched := false
		for _, pattern := range p.config.IncludePatterns {
			if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check exclude patterns
	for _, pattern := range p.config.ExcludePatterns {
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return false
		}
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
	if len(files) <= 1 {
		return files, nil
	}

	// Sort by path length to process shorter paths first
	sort.Slice(files, func(i, j int) bool {
		return len(files[i]) < len(files[j])
	})

	var filtered []string
	var redundant []string
	seen := make(map[string]bool)

	for _, file := range files {
		dir := filepath.Dir(file)
		if !seen[dir] {
			filtered = append(filtered, file)
			seen[dir] = true
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
		stat, err := os.Stat(file)
		if err != nil {
			continue
		}

		contentType := p.getContentType(strings.ToLower(filepath.Ext(file)))
		size := stat.Size()

		// Generate unique ID
		id := fmt.Sprintf("%x", md5.Sum([]byte(file)))

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
