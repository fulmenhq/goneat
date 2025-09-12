/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package assess

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/fulmenhq/goneat/internal/gitctx"
	"github.com/fulmenhq/goneat/pkg/buildinfo"
	"github.com/fulmenhq/goneat/pkg/logger"
)

// AssessmentEngine orchestrates the assessment process
type AssessmentEngine struct {
	priorityManager *PriorityManager
	runnerRegistry  *AssessmentRunnerRegistry
}

// NewAssessmentEngine creates a new assessment engine
func NewAssessmentEngine() *AssessmentEngine {
	return &AssessmentEngine{
		priorityManager: NewPriorityManager(),
		runnerRegistry:  GetAssessmentRunnerRegistry(),
	}
}

// RunAssessment executes a comprehensive assessment of the target
func (e *AssessmentEngine) RunAssessment(ctx context.Context, target string, config AssessmentConfig) (*AssessmentReport, error) {
	startTime := time.Now()

	// Collect git change context (best-effort)
	var changeCtx *gitctx.ChangeContext
	var modifiedAbs map[string]struct{}
	var modifiedLinesAbs map[string][]int
	if ctx2, _, lines, _ := gitctx.CollectWithLines(target); ctx2 != nil {
		changeCtx = ctx2
		// Build absolute path set for quick lookups
		modifiedAbs = make(map[string]struct{}, len(ctx2.ModifiedFiles))
		for _, p := range ctx2.ModifiedFiles {
			abs := p
			if !filepath.IsAbs(abs) {
				abs = filepath.Join(target, p)
			}
			if a2, err := filepath.Abs(abs); err == nil {
				modifiedAbs[a2] = struct{}{}
			}
		}
		// Build absolute lines map
		if len(lines) > 0 {
			modifiedLinesAbs = make(map[string][]int, len(lines))
			for rel, lns := range lines {
				abs := rel
				if !filepath.IsAbs(abs) {
					abs = filepath.Join(target, rel)
				}
				if a2, err := filepath.Abs(abs); err == nil {
					modifiedLinesAbs[a2] = append(modifiedLinesAbs[a2], lns...)
				}
			}
		}
	}

	// Parse custom priorities if provided
	if config.PriorityString != "" {
		if err := e.priorityManager.ParsePriorityString(config.PriorityString); err != nil {
			return nil, fmt.Errorf("failed to parse priority string: %w", err)
		}
	}

	// Get available categories and order them by priority
	availableCategories := e.runnerRegistry.GetAvailableCategories()
	orderedCategories := e.priorityManager.GetOrderedCategories(availableCategories)

	// If specific categories were requested, filter accordingly
	if len(config.SelectedCategories) > 0 {
		allowed := make(map[string]bool)
		for _, c := range config.SelectedCategories {
			allowed[strings.TrimSpace(c)] = true
		}
		var filtered []AssessmentCategory
		for _, c := range orderedCategories {
			if allowed[string(c)] {
				filtered = append(filtered, c)
			}
		}
		orderedCategories = filtered
	}

	// Determine worker count based on flags and CPU cores
	var workerCount int
	if config.Concurrency > 0 {
		workerCount = config.Concurrency
	} else {
		percent := config.ConcurrencyPercent
		if percent <= 0 {
			percent = 50
		}
		cores := runtime.NumCPU()
		workerCount = (cores * percent) / 100
		if workerCount < 1 {
			workerCount = 1
		}
	}

	logger.Info(fmt.Sprintf("Starting assessment of %s with %d categories (workers=%d)", target, len(orderedCategories), workerCount))

	// Run assessments for each category (with optional concurrency)
	categoryResults := make(map[string]CategoryResult)
	var allIssues []Issue
	var commandsRun []string
	// Track per-category runtimes
	catRuntime := make(map[AssessmentCategory]time.Duration)

	type job struct {
		category AssessmentCategory
		priority int
	}

	if workerCount == 1 {
		// Sequential fallback
		for _, category := range orderedCategories {
			runner, exists := e.runnerRegistry.GetRunner(category)
			if !exists {
				logger.Warn(fmt.Sprintf("No runner found for category: %s", category))
				continue
			}

			logger.Info(fmt.Sprintf("Running %s assessment...", category))
			runStart := time.Now()
			// Apply per-category timeout based on global config
			rctx := ctx
			var cancel context.CancelFunc
			if config.Timeout > 0 {
				rctx, cancel = context.WithTimeout(ctx, config.Timeout)
			}
			result, err := runner.Assess(rctx, target, config)
			if cancel != nil {
				cancel()
			}
			runDur := time.Since(runStart)
			catRuntime[category] = runDur
			if result != nil {
				commandsRun = append(commandsRun, result.CommandName)
			}

			cr := CategoryResult{
				Category:       category,
				Priority:       e.priorityManager.GetPriority(category),
				Parallelizable: runner.CanRunInParallel(),
			}
			if err != nil {
				cr.Status = "error"
				cr.Error = err.Error()
				logger.Error(fmt.Sprintf("%s assessment failed after %v: %v", category, runDur, err))
			} else if result.Success {
				cr.Status = "success"
				cr.Issues = e.annotateIssuesWithChange(result.Issues, target, modifiedAbs, modifiedLinesAbs)
				cr.IssueCount = len(result.Issues)
				cr.EstimatedTime = HumanReadableDuration(e.estimateCategoryTime(result.Issues))
				if result.Metrics != nil {
					cr.Metrics = result.Metrics
					// Extract suppression report if present
					if suppressions, ok := result.Metrics["_suppressions"].([]Suppression); ok && len(suppressions) > 0 {
						cr.SuppressionReport = &SuppressionReport{
							Suppressions: suppressions,
							Summary:      GenerateSummary(suppressions),
						}
						// Remove internal key from metrics
						delete(result.Metrics, "_suppressions")
					}
				}
				allIssues = append(allIssues, cr.Issues...)
				logger.Info(fmt.Sprintf("%s assessment completed in %v: %d issues found", category, runDur, len(cr.Issues)))
			} else {
				// Map non-success without error to a consistent status.
				if result.Error != "" {
					cr.Status = "error"
					cr.Error = result.Error
				} else {
					cr.Status = "skipped"
				}
				logger.Debug(fmt.Sprintf("%s assessment non-success without error after %v (status=%s)", category, runDur, cr.Status))
			}
			categoryResults[string(category)] = cr
		}
	} else {
		// Concurrent execution with worker pool
		jobs := make(chan job)
		var wg sync.WaitGroup
		var mu sync.Mutex

		for w := 0; w < workerCount; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := range jobs {
					runner, exists := e.runnerRegistry.GetRunner(j.category)
					if !exists {
						logger.Warn(fmt.Sprintf("No runner found for category: %s", j.category))
						continue
					}

					logger.Info(fmt.Sprintf("Running %s assessment...", j.category))
					runStart := time.Now()
					// Apply per-category timeout based on global config
					rctx := ctx
					var cancel context.CancelFunc
					if config.Timeout > 0 {
						rctx, cancel = context.WithTimeout(ctx, config.Timeout)
					}
					result, err := runner.Assess(rctx, target, config)
					if cancel != nil {
						cancel()
					}
					runDur := time.Since(runStart)

					cr := CategoryResult{
						Category:       j.category,
						Priority:       j.priority,
						Parallelizable: runner.CanRunInParallel(),
					}
					if err != nil {
						cr.Status = "error"
						cr.Error = err.Error()
						logger.Error(fmt.Sprintf("%s assessment failed after %v: %v", j.category, runDur, err))
					} else if result.Success {
						cr.Status = "success"
						cr.Issues = e.annotateIssuesWithChange(result.Issues, target, modifiedAbs, modifiedLinesAbs)
						cr.IssueCount = len(result.Issues)
						cr.EstimatedTime = HumanReadableDuration(e.estimateCategoryTime(result.Issues))
						if result.Metrics != nil {
							cr.Metrics = result.Metrics
							// Extract suppression report if present
							if suppressions, ok := result.Metrics["_suppressions"].([]Suppression); ok && len(suppressions) > 0 {
								cr.SuppressionReport = &SuppressionReport{
									Suppressions: suppressions,
									Summary:      GenerateSummary(suppressions),
								}
								// Remove internal key from metrics
								delete(result.Metrics, "_suppressions")
							}
						}
						logger.Info(fmt.Sprintf("%s assessment completed in %v: %d issues found", j.category, runDur, len(cr.Issues)))
					} else {
						if result.Error != "" {
							cr.Status = "error"
							cr.Error = result.Error
						} else {
							cr.Status = "skipped"
						}
						logger.Debug(fmt.Sprintf("%s assessment non-success without error after %v (status=%s)", j.category, runDur, cr.Status))
					}

					mu.Lock()
					catRuntime[j.category] = runDur
					if result != nil {
						commandsRun = append(commandsRun, result.CommandName)
						if len(cr.Issues) > 0 {
							allIssues = append(allIssues, cr.Issues...)
						}
					}
					categoryResults[string(j.category)] = cr
					mu.Unlock()
				}
			}()
		}

		for _, category := range orderedCategories {
			jobs <- job{category: category, priority: e.priorityManager.GetPriority(category)}
		}
		close(jobs)
		wg.Wait()
	}

	// Generate workflow plan
	workflow := e.generateWorkflowPlan(categoryResults, allIssues)

	// Calculate summary statistics
	summary := e.calculateSummary(categoryResults, allIssues, time.Duration(workflow.TotalTime))

	// Create the final report
	report := &AssessmentReport{
		Metadata: ReportMetadata{
			GeneratedAt:   time.Now(),
			Tool:          "goneat",
			Version:       buildinfo.BinaryVersion, // Dynamic from ldflags
			Target:        target,
			ExecutionTime: HumanReadableDuration(time.Since(startTime)),
			CommandsRun:   commandsRun,
			FailOn:        string(config.FailOnSeverity),
		},
		Summary:    summary,
		Categories: categoryResults,
		Workflow:   workflow,
	}

	// Attach change context if present
	if changeCtx != nil {
		report.Metadata.ChangeContext = &ChangeContext{
			ModifiedFiles: changeCtx.ModifiedFiles,
			TotalChanges:  changeCtx.TotalChanges,
			ChangeScope:   changeCtx.ChangeScope,
			GitSHA:        changeCtx.GitSHA,
			Branch:        changeCtx.Branch,
		}
	}

	// Add extended workplan if requested
	if config.Extended {
		report.Workplan = e.generateExtendedWorkplan(
			target,
			config,
			orderedCategories,
			categoryResults,
			catRuntime,
			workerCount,
			time.Since(startTime),
		)
	}

	// Log summary with concurrency and per-category runtimes
	logger.Info(fmt.Sprintf("Concurrency summary: workers=%d, categories=%d", workerCount, len(orderedCategories)))
	for _, c := range orderedCategories {
		if d, ok := catRuntime[c]; ok {
			logger.Info(fmt.Sprintf("Runtime: %-16s %v", c, d))
		}
	}
	logger.Info(fmt.Sprintf("Assessment completed in %v: %d total issues, estimated fix time: %v",
		report.Metadata.ExecutionTime, summary.TotalIssues, summary.EstimatedTime))

	return report, nil
}

// annotateIssuesWithChange marks issues that relate to modified files; best-effort path normalization.
func (e *AssessmentEngine) annotateIssuesWithChange(issues []Issue, target string, modifiedAbs map[string]struct{}, modifiedLinesAbs map[string][]int) []Issue {
	if len(issues) == 0 || len(modifiedAbs) == 0 {
		return issues
	}
	out := make([]Issue, len(issues))
	for i, is := range issues {
		file := is.File
		abs := file
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(target, file)
		}
		if a2, err := filepath.Abs(abs); err == nil {
			if _, ok := modifiedAbs[a2]; ok {
				is.ChangeRelated = true
				if lns, ok2 := modifiedLinesAbs[a2]; ok2 && len(lns) > 0 {
					is.LinesModified = lns
				}
			}
		}
		out[i] = is
	}
	return out
}

// estimateCategoryTime estimates the time to fix issues in a category
func (e *AssessmentEngine) estimateCategoryTime(issues []Issue) time.Duration {
	var totalTime time.Duration

	for _, issue := range issues {
		if issue.EstimatedTime > 0 {
			totalTime += time.Duration(issue.EstimatedTime)
		} else {
			// Default time estimates based on severity
			switch issue.Severity {
			case SeverityCritical:
				totalTime += 30 * time.Minute
			case SeverityHigh:
				totalTime += 15 * time.Minute
			case SeverityMedium:
				totalTime += 5 * time.Minute
			case SeverityLow:
				totalTime += 2 * time.Minute
			default:
				totalTime += 1 * time.Minute
			}
		}
	}

	return totalTime
}

// generateExtendedWorkplan creates detailed execution and discovery information
func (e *AssessmentEngine) generateExtendedWorkplan(
	target string,
	config AssessmentConfig,
	orderedCategories []AssessmentCategory,
	categoryResults map[string]CategoryResult,
	catRuntime map[AssessmentCategory]time.Duration,
	workerCount int,
	totalRuntime time.Duration,
) *ExtendedWorkplan {
	// Collect file list and discovery patterns from the target
	fileList := []string{}

	// For single file, just add the file
	if info, err := os.Stat(target); err == nil && !info.IsDir() {
		fileList = append(fileList, filepath.Base(target))
	}

	// Build discovery patterns
	discoveryPatterns := DiscoveryPatterns{
		Include:      config.IncludeFiles,
		Exclude:      config.ExcludeFiles,
		ForceInclude: config.ForceInclude,
	}

	// Count categories
	categoriesPlanned := []string{}
	categoriesSkipped := []string{}
	skipReasons := make(map[string]string)

	for _, category := range orderedCategories {
		categoriesPlanned = append(categoriesPlanned, string(category))
	}

	// Check for skipped categories by examining available categories vs selected
	availableCategories := e.runnerRegistry.GetAvailableCategories()
	for _, category := range availableCategories {
		categoryStr := string(category)
		found := false
		for _, planned := range categoriesPlanned {
			if planned == categoryStr {
				found = true
				break
			}
		}
		if !found {
			categoriesSkipped = append(categoriesSkipped, categoryStr)
			if len(config.SelectedCategories) > 0 {
				skipReasons[categoryStr] = "not in selected categories"
			} else {
				skipReasons[categoryStr] = "not available or filtered out"
			}
		}
	}

	// Build category runtimes map
	categoryRuntimes := make(map[string]HumanReadableDuration)
	for category, duration := range catRuntime {
		categoryRuntimes[string(category)] = HumanReadableDuration(duration)
	}

	return &ExtendedWorkplan{
		FilesDiscovered:   len(fileList), // Simple approximation
		FilesIncluded:     len(fileList),
		FilesExcluded:     0,                // Would need file discovery logic to calculate properly
		ExclusionReasons:  map[string]int{}, // Placeholder
		CategoriesPlanned: categoriesPlanned,
		CategoriesSkipped: categoriesSkipped,
		SkipReasons:       skipReasons,
		EstimatedDuration: HumanReadableDuration(totalRuntime),
		FileList:          fileList,
		DiscoveryPatterns: discoveryPatterns,
		ExecutionSummary: ExecutionSummary{
			WorkerCount:      workerCount,
			CategoryRuntimes: categoryRuntimes,
			TotalRuntime:     HumanReadableDuration(totalRuntime),
		},
	}
}

// generateWorkflowPlan creates a remediation workflow from assessment results
func (e *AssessmentEngine) generateWorkflowPlan(categoryResults map[string]CategoryResult, allIssues []Issue) WorkflowPlan {
	var phases []WorkflowPhase
	var parallelGroups []ParallelGroup

	// Group issues by category for phase creation
	categoryIssues := make(map[AssessmentCategory][]Issue)
	for _, issue := range allIssues {
		categoryIssues[issue.Category] = append(categoryIssues[issue.Category], issue)
	}

	// Create phases based on priority
	priorityOrder := []int{1, 2, 3, 4}
	for _, priority := range priorityOrder {
		var phaseCategories []AssessmentCategory
		var phaseTime time.Duration
		var phaseIssues []Issue

		// Collect categories for this priority
		for _, result := range categoryResults {
			if result.Priority == priority && result.IssueCount > 0 {
				phaseCategories = append(phaseCategories, result.Category)
				phaseTime += time.Duration(result.EstimatedTime)
				phaseIssues = append(phaseIssues, categoryIssues[result.Category]...)
			}
		}

		if len(phaseCategories) > 0 {
			phase := WorkflowPhase{
				Name:          fmt.Sprintf("Phase %d", priority),
				Description:   e.getPhaseDescription(priority, phaseCategories),
				EstimatedTime: HumanReadableDuration(phaseTime),
				Categories:    phaseCategories,
				Priority:      priority,
			}

			// Identify parallel groups within this phase
			phaseGroups := e.identifyParallelGroups(phaseIssues, phaseCategories)
			phase.ParallelGroups = make([]string, len(phaseGroups))
			for i, group := range phaseGroups {
				phase.ParallelGroups[i] = group.Name
			}
			parallelGroups = append(parallelGroups, phaseGroups...)

			phases = append(phases, phase)
		}
	}

	return WorkflowPlan{
		Phases:         phases,
		ParallelGroups: parallelGroups,
		TotalTime:      HumanReadableDuration(e.calculateTotalTime(phases)),
	}
}

// getPhaseDescription generates a description for a workflow phase
func (e *AssessmentEngine) getPhaseDescription(priority int, categories []AssessmentCategory) string {
	switch priority {
	case 1:
		return "Address format issues - quick wins, often auto-fixable"
	case 2:
		return "Fix security issues - critical problems that may block progress"
	case 3:
		return "Improve code quality - lint and style issues"
	case 4:
		return "Optimize performance - efficiency improvements"
	default:
		return "Address remaining issues"
	}
}

// identifyParallelGroups finds groups of issues that can be worked on in parallel
func (e *AssessmentEngine) identifyParallelGroups(issues []Issue, categories []AssessmentCategory) []ParallelGroup {
	// Group issues by file
	fileIssues := make(map[string][]Issue)
	for _, issue := range issues {
		fileIssues[issue.File] = append(fileIssues[issue.File], issue)
	}

	var groups []ParallelGroup
	groupIndex := 0

	// Simple parallelization: one group per file with multiple issues
	for file, fileIssues := range fileIssues {
		if len(fileIssues) > 1 {
			group := ParallelGroup{
				Name:          fmt.Sprintf("group_%d", groupIndex),
				Description:   fmt.Sprintf("Issues in %s", file),
				Files:         []string{file},
				Categories:    categories,
				EstimatedTime: HumanReadableDuration(e.estimateCategoryTime(fileIssues)),
				IssueCount:    len(fileIssues),
			}
			groups = append(groups, group)
			groupIndex++
		}
	}

	return groups
}

// calculateTotalTime sums up time from all phases
func (e *AssessmentEngine) calculateTotalTime(phases []WorkflowPhase) time.Duration {
	var total time.Duration
	for _, phase := range phases {
		total += time.Duration(phase.EstimatedTime)
	}
	return total
}

// calculateSummary generates summary statistics from assessment results
func (e *AssessmentEngine) calculateSummary(categoryResults map[string]CategoryResult, allIssues []Issue, totalTime time.Duration) ReportSummary {
	criticalCount := 0
	categoriesWithIssues := 0

	for _, issue := range allIssues {
		if issue.Severity == SeverityCritical {
			criticalCount++
		}
	}

	for _, result := range categoryResults {
		if result.IssueCount > 0 {
			categoriesWithIssues++
		}
	}

	// Calculate overall health (0.0 to 1.0, higher is better)
	overallHealth := 1.0
	if len(allIssues) > 0 {
		// Simple health calculation: reduce health based on issue count and severity
		severityPenalty := 0.0
		for _, issue := range allIssues {
			switch issue.Severity {
			case SeverityCritical:
				severityPenalty += 0.1
			case SeverityHigh:
				severityPenalty += 0.05
			case SeverityMedium:
				severityPenalty += 0.02
			case SeverityLow:
				severityPenalty += 0.01
			}
		}
		overallHealth = max(0.0, 1.0-severityPenalty)
	}

	return ReportSummary{
		OverallHealth:        overallHealth,
		CriticalIssues:       criticalCount,
		TotalIssues:          len(allIssues),
		EstimatedTime:        HumanReadableDuration(totalTime),
		ParallelGroups:       len(e.identifyParallelGroups(allIssues, []AssessmentCategory{})),
		CategoriesWithIssues: categoriesWithIssues,
	}
}

// max returns the maximum of two float64 values
func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
