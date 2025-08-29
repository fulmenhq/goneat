/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package assess

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/3leaps/goneat/pkg/logger"
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

	// Parse custom priorities if provided
	if config.PriorityString != "" {
		if err := e.priorityManager.ParsePriorityString(config.PriorityString); err != nil {
			return nil, fmt.Errorf("failed to parse priority string: %w", err)
		}
	}

	// Get available categories and order them by priority
	availableCategories := e.runnerRegistry.GetAvailableCategories()
	orderedCategories := e.priorityManager.GetOrderedCategories(availableCategories)

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
			result, err := runner.Assess(ctx, target, config)
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
				cr.Issues = result.Issues
				cr.IssueCount = len(result.Issues)
				cr.EstimatedTime = e.estimateCategoryTime(result.Issues)
				allIssues = append(allIssues, result.Issues...)
				logger.Info(fmt.Sprintf("%s assessment completed in %v: %d issues found", category, runDur, len(result.Issues)))
			} else {
				cr.Status = "failed"
				logger.Warn(fmt.Sprintf("%s assessment failed without error after %v", category, runDur))
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
					result, err := runner.Assess(ctx, target, config)
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
						cr.Issues = result.Issues
						cr.IssueCount = len(result.Issues)
						cr.EstimatedTime = e.estimateCategoryTime(result.Issues)
						logger.Info(fmt.Sprintf("%s assessment completed in %v: %d issues found", j.category, runDur, len(result.Issues)))
					} else {
						cr.Status = "failed"
						logger.Warn(fmt.Sprintf("%s assessment failed without error after %v", j.category, runDur))
					}

					mu.Lock()
					catRuntime[j.category] = runDur
					if result != nil {
						commandsRun = append(commandsRun, result.CommandName)
						if len(result.Issues) > 0 {
							allIssues = append(allIssues, result.Issues...)
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
	summary := e.calculateSummary(categoryResults, allIssues, workflow.TotalTime)

	// Create the final report
	report := &AssessmentReport{
		Metadata: ReportMetadata{
			GeneratedAt:   time.Now(),
			Tool:          "goneat",
			Version:       "1.0.0", // TODO: Get from version package
			Target:        target,
			ExecutionTime: time.Since(startTime),
			CommandsRun:   commandsRun,
		},
		Summary:    summary,
		Categories: categoryResults,
		Workflow:   workflow,
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

// estimateCategoryTime estimates the time to fix issues in a category
func (e *AssessmentEngine) estimateCategoryTime(issues []Issue) time.Duration {
	var totalTime time.Duration

	for _, issue := range issues {
		if issue.EstimatedTime > 0 {
			totalTime += issue.EstimatedTime
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
				phaseTime += result.EstimatedTime
				phaseIssues = append(phaseIssues, categoryIssues[result.Category]...)
			}
		}

		if len(phaseCategories) > 0 {
			phase := WorkflowPhase{
				Name:          fmt.Sprintf("Phase %d", priority),
				Description:   e.getPhaseDescription(priority, phaseCategories),
				EstimatedTime: phaseTime,
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
		TotalTime:      e.calculateTotalTime(phases),
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
				EstimatedTime: e.estimateCategoryTime(fileIssues),
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
		total += phase.EstimatedTime
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
		EstimatedTime:        totalTime,
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
