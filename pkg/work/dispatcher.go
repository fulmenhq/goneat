package work

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/3leaps/goneat/pkg/logger"
)

// ExecutionResult represents the result of processing a work item
type ExecutionResult struct {
	WorkItemID string        `json:"work_item_id"`
	Success    bool          `json:"success"`
	Error      string        `json:"error,omitempty"`
	Duration   time.Duration `json:"duration"`
	Output     string        `json:"output,omitempty"`
}

// ExecutionSummary provides a summary of the execution
type ExecutionSummary struct {
	TotalItems    int           `json:"total_items"`
	Successful    int           `json:"successful"`
	Failed        int           `json:"failed"`
	TotalDuration time.Duration `json:"total_duration"`
	WorkerStats   WorkerStats   `json:"worker_stats"`
	GroupResults  []GroupResult `json:"group_results"`
}

// WorkerStats provides statistics about worker performance
type WorkerStats struct {
	TotalWorkers       int     `json:"total_workers"`
	ActiveWorkers      int     `json:"active_workers"`
	AverageUtilization float64 `json:"average_utilization"`
	PeakUtilization    int     `json:"peak_utilization"`
}

// GroupResult provides results for a specific work group
type GroupResult struct {
	GroupID       string        `json:"group_id"`
	ItemCount     int           `json:"item_count"`
	SuccessCount  int           `json:"success_count"`
	FailureCount  int           `json:"failure_count"`
	Duration      time.Duration `json:"duration"`
	ErrorMessages []string      `json:"error_messages,omitempty"`
}

// WorkItemProcessor defines the interface for processing work items
type WorkItemProcessor interface {
	ProcessWorkItem(ctx context.Context, item *WorkItem, dryRun bool, noOp bool) ExecutionResult
}

// DispatcherConfig configures the dispatcher
type DispatcherConfig struct {
	MaxWorkers       int
	DryRun           bool
	NoOp             bool
	ProgressCallback func(result ExecutionResult)
	Timeout          time.Duration
}

// Dispatcher handles parallel execution of work manifests
type Dispatcher struct {
	config    DispatcherConfig
	processor WorkItemProcessor
}

// NewDispatcher creates a new work dispatcher
func NewDispatcher(config DispatcherConfig, processor WorkItemProcessor) *Dispatcher {
	if config.MaxWorkers <= 0 {
		config.MaxWorkers = runtime.NumCPU()
	}
	if config.Timeout <= 0 {
		config.Timeout = 5 * time.Minute
	}

	return &Dispatcher{
		config:    config,
		processor: processor,
	}
}

// ExecuteManifest executes a work manifest
func (d *Dispatcher) ExecuteManifest(ctx context.Context, manifest *WorkManifest) (*ExecutionSummary, error) {
	logger.Info(fmt.Sprintf("Starting execution of %d work items with %d workers", len(manifest.WorkItems), d.config.MaxWorkers))

	startTime := time.Now()

	// Create work channels
	workChan := make(chan *WorkItem, len(manifest.WorkItems))
	resultChan := make(chan ExecutionResult, len(manifest.WorkItems))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < d.config.MaxWorkers; i++ {
		wg.Add(1)
		go d.worker(ctx, workChan, resultChan, &wg)
	}

	// Send work items to workers
	go func() {
		for _, item := range manifest.WorkItems {
			select {
			case workChan <- &item:
			case <-ctx.Done():
				close(workChan)
				return
			}
		}
		close(workChan)
	}()

	// Collect results
	results := make([]ExecutionResult, 0, len(manifest.WorkItems))
	groupResults := make(map[string]*GroupResult)

	// Create group result trackers
	for _, group := range manifest.Groups {
		groupResults[group.ID] = &GroupResult{
			GroupID:   group.ID,
			ItemCount: len(group.WorkItemIDs),
		}
	}

	// Wait for all workers to complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Process results
	for result := range resultChan {
		results = append(results, result)

		if d.config.ProgressCallback != nil {
			d.config.ProgressCallback(result)
		}

		// Update group results
		for _, group := range manifest.Groups {
			for _, itemID := range group.WorkItemIDs {
				if itemID == result.WorkItemID {
					groupResult := groupResults[group.ID]
					if result.Success {
						groupResult.SuccessCount++
					} else {
						groupResult.FailureCount++
						groupResult.ErrorMessages = append(groupResult.ErrorMessages, result.Error)
					}
					break
				}
			}
		}
	}

	// Convert group results map to slice
	groupResultsSlice := make([]GroupResult, 0, len(groupResults))
	for _, result := range groupResults {
		groupResultsSlice = append(groupResultsSlice, *result)
	}

	// Calculate summary
	totalDuration := time.Since(startTime)
	successful := 0
	failed := 0

	for _, result := range results {
		if result.Success {
			successful++
		} else {
			failed++
		}
	}

	summary := &ExecutionSummary{
		TotalItems:    len(results),
		Successful:    successful,
		Failed:        failed,
		TotalDuration: totalDuration,
		WorkerStats: WorkerStats{
			TotalWorkers:       d.config.MaxWorkers,
			ActiveWorkers:      d.config.MaxWorkers, // Simplified
			AverageUtilization: float64(len(results)) / float64(d.config.MaxWorkers) / totalDuration.Seconds(),
			PeakUtilization:    d.config.MaxWorkers,
		},
		GroupResults: groupResultsSlice,
	}

	logger.Info(fmt.Sprintf("Execution completed: %d successful, %d failed in %v", successful, failed, totalDuration))
	return summary, nil
}

// worker processes work items from the work channel
func (d *Dispatcher) worker(ctx context.Context, workChan <-chan *WorkItem, resultChan chan<- ExecutionResult, wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		select {
		case item, ok := <-workChan:
			if !ok {
				return // Channel closed
			}

			startTime := time.Now()
			result := d.processor.ProcessWorkItem(ctx, item, d.config.DryRun, d.config.NoOp)
			result.Duration = time.Since(startTime)

			select {
			case resultChan <- result:
			case <-ctx.Done():
				return
			}

		case <-ctx.Done():
			return
		}
	}
}

// ExecuteGroup executes a specific work group
func (d *Dispatcher) ExecuteGroup(ctx context.Context, manifest *WorkManifest, groupID string) (*ExecutionSummary, error) {
	// Find the group
	var targetGroup *WorkGroup
	for _, group := range manifest.Groups {
		if group.ID == groupID {
			targetGroup = &group
			break
		}
	}

	if targetGroup == nil {
		return nil, fmt.Errorf("group %s not found", groupID)
	}

	// Create work items for this group
	groupItems := make([]WorkItem, 0, len(targetGroup.WorkItemIDs))
	for _, itemID := range targetGroup.WorkItemIDs {
		for _, item := range manifest.WorkItems {
			if item.ID == itemID {
				groupItems = append(groupItems, item)
				break
			}
		}
	}

	// Create a mini-manifest for this group
	groupManifest := &WorkManifest{
		Plan: Plan{
			Command:           manifest.Plan.Command,
			Timestamp:         time.Now(),
			WorkingDirectory:  manifest.Plan.WorkingDirectory,
			TotalFiles:        len(groupItems),
			FilteredFiles:     len(groupItems),
			ExecutionStrategy: "parallel",
		},
		WorkItems: groupItems,
		Groups:    []WorkGroup{*targetGroup},
	}

	return d.ExecuteManifest(ctx, groupManifest)
}

// GetRecommendedWorkerCount returns the recommended number of workers for a manifest
func GetRecommendedWorkerCount(manifest *WorkManifest) int {
	maxRecommended := 0
	for _, group := range manifest.Groups {
		if group.RecommendedParallelization > maxRecommended {
			maxRecommended = group.RecommendedParallelization
		}
	}

	if maxRecommended == 0 {
		return runtime.NumCPU()
	}

	return maxRecommended
}

// ValidateManifest validates a work manifest before execution
func ValidateManifest(manifest *WorkManifest) error {
	if len(manifest.WorkItems) == 0 {
		return fmt.Errorf("manifest contains no work items")
	}

	if len(manifest.Groups) == 0 {
		return fmt.Errorf("manifest contains no groups")
	}

	// Check that all work items in groups exist
	itemMap := make(map[string]bool)
	for _, item := range manifest.WorkItems {
		itemMap[item.ID] = true
	}

	for _, group := range manifest.Groups {
		for _, itemID := range group.WorkItemIDs {
			if !itemMap[itemID] {
				return fmt.Errorf("group %s references non-existent work item %s", group.ID, itemID)
			}
		}
	}

	return nil
}
