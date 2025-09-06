package work

import (
	"context"
	"runtime"
	"testing"
	"time"
)

// mockProcessor implements WorkItemProcessor for testing
type mockProcessor struct {
	processingTime time.Duration
	shouldError    bool
	errorMessage   string
}

func (m *mockProcessor) ProcessWorkItem(ctx context.Context, item *WorkItem, dryRun bool, noOp bool) ExecutionResult {
	if m.processingTime > 0 {
		time.Sleep(m.processingTime)
	}
	
	if m.shouldError {
		return ExecutionResult{
			WorkItemID: item.ID,
			Success:    false,
			Error:      m.errorMessage,
			Duration:   m.processingTime,
		}
	}
	
	return ExecutionResult{
		WorkItemID: item.ID,
		Success:    true,
		Duration:   m.processingTime,
		Output:     "mock processing completed",
	}
}

func TestNewDispatcher(t *testing.T) {
	processor := &mockProcessor{}
	
	testCases := []struct {
		name           string
		config         DispatcherConfig
		expectedWorkers int
		expectedTimeout time.Duration
	}{
		{
			name: "default config",
			config: DispatcherConfig{},
			expectedWorkers: runtime.NumCPU(),
			expectedTimeout: 5 * time.Minute,
		},
		{
			name: "custom config",
			config: DispatcherConfig{
				MaxWorkers: 8,
				Timeout:    30 * time.Second,
			},
			expectedWorkers: 8,
			expectedTimeout: 30 * time.Second,
		},
		{
			name: "zero workers defaults to CPU count",
			config: DispatcherConfig{
				MaxWorkers: 0,
			},
			expectedWorkers: runtime.NumCPU(),
			expectedTimeout: 5 * time.Minute,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dispatcher := NewDispatcher(tc.config, processor)
			
			if dispatcher == nil {
				t.Fatal("expected dispatcher to be created, got nil")
			}
			
			if dispatcher.config.MaxWorkers != tc.expectedWorkers {
				t.Errorf("expected MaxWorkers %d, got %d", tc.expectedWorkers, dispatcher.config.MaxWorkers)
			}
			
			if dispatcher.config.Timeout != tc.expectedTimeout {
				t.Errorf("expected Timeout %v, got %v", tc.expectedTimeout, dispatcher.config.Timeout)
			}
			
			if dispatcher.processor != processor {
				t.Error("expected processor to be set correctly")
			}
		})
	}
}

func TestGetRecommendedWorkerCount(t *testing.T) {
	testCases := []struct {
		name     string
		manifest *WorkManifest
		expected int
	}{
		{
			name: "no groups defaults to CPU count",
			manifest: &WorkManifest{
				Groups: []WorkGroup{},
			},
			expected: runtime.NumCPU(),
		},
		{
			name: "single group with recommendation",
			manifest: &WorkManifest{
				Groups: []WorkGroup{
					{RecommendedParallelization: 4},
				},
			},
			expected: 4,
		},
		{
			name: "multiple groups returns max recommendation",
			manifest: &WorkManifest{
				Groups: []WorkGroup{
					{RecommendedParallelization: 2},
					{RecommendedParallelization: 6},
					{RecommendedParallelization: 4},
				},
			},
			expected: 6,
		},
		{
			name: "zero recommendations defaults to CPU count",
			manifest: &WorkManifest{
				Groups: []WorkGroup{
					{RecommendedParallelization: 0},
					{RecommendedParallelization: 0},
				},
			},
			expected: runtime.NumCPU(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := GetRecommendedWorkerCount(tc.manifest)
			
			if result != tc.expected {
				t.Errorf("expected %d, got %d", tc.expected, result)
			}
		})
	}
}

func TestValidateManifest(t *testing.T) {
	testCases := []struct {
		name        string
		manifest    *WorkManifest
		shouldError bool
	}{
		{
			name: "empty work items",
			manifest: &WorkManifest{
				WorkItems: []WorkItem{},
				Groups:    []WorkGroup{},
			},
			shouldError: true,
		},
		{
			name: "empty groups",
			manifest: &WorkManifest{
				WorkItems: []WorkItem{
					{ID: "item1", Path: "test.go", ContentType: "go"},
				},
				Groups: []WorkGroup{},
			},
			shouldError: true,
		},
		{
			name: "valid manifest",
			manifest: &WorkManifest{
				WorkItems: []WorkItem{
					{
						ID:          "item1",
						Path:        "test.go",
						ContentType: "go",
					},
				},
				Groups: []WorkGroup{
					{
						ID:                         "group1",
						Name:                       "Go Files",
						Strategy:                   "parallel",
						WorkItemIDs:                []string{"item1"},
						EstimatedTotalTime:         1.0,
						RecommendedParallelization: 2,
					},
				},
			},
			shouldError: false,
		},
		{
			name: "group references nonexistent work item",
			manifest: &WorkManifest{
				WorkItems: []WorkItem{
					{ID: "item1", Path: "test.go", ContentType: "go"},
				},
				Groups: []WorkGroup{
					{
						ID:          "group1",
						WorkItemIDs: []string{"nonexistent"},
					},
				},
			},
			shouldError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateManifest(tc.manifest)
			
			if tc.shouldError && err == nil {
				t.Error("expected error, got nil")
			}
			
			if !tc.shouldError && err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}

func TestExecuteManifest(t *testing.T) {
	processor := &mockProcessor{processingTime: 10 * time.Millisecond}
	config := DispatcherConfig{
		MaxWorkers: 2,
		Timeout:    5 * time.Second,
	}
	dispatcher := NewDispatcher(config, processor)

	t.Run("successful execution", func(t *testing.T) {
		manifest := &WorkManifest{
			WorkItems: []WorkItem{
				{ID: "item1", Path: "test1.go", ContentType: "go"},
				{ID: "item2", Path: "test2.go", ContentType: "go"},
			},
			Groups: []WorkGroup{
				{
					ID:          "group1",
					Name:        "Go Files",
					Strategy:    "parallel",
					WorkItemIDs: []string{"item1", "item2"},
				},
			},
		}

		summary, err := dispatcher.ExecuteManifest(context.Background(), manifest)
		
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		
		if summary.TotalItems != 2 {
			t.Errorf("expected 2 total items, got %d", summary.TotalItems)
		}
		
		if summary.Successful != 2 {
			t.Errorf("expected 2 successful items, got %d", summary.Successful)
		}
		
		if summary.Failed != 0 {
			t.Errorf("expected 0 failed items, got %d", summary.Failed)
		}
	})

	t.Run("execution with errors", func(t *testing.T) {
		errorProcessor := &mockProcessor{
			shouldError:  true,
			errorMessage: "mock processing error",
		}
		errorDispatcher := NewDispatcher(config, errorProcessor)

		manifest := &WorkManifest{
			WorkItems: []WorkItem{
				{ID: "item1", Path: "test1.go", ContentType: "go"},
			},
			Groups: []WorkGroup{
				{
					ID:          "group1",
					Name:        "Go Files", 
					Strategy:    "parallel",
					WorkItemIDs: []string{"item1"},
				},
			},
		}

		summary, err := errorDispatcher.ExecuteManifest(context.Background(), manifest)
		
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		
		if summary.Failed != 1 {
			t.Errorf("expected 1 failed item, got %d", summary.Failed)
		}
		
		if summary.Successful != 0 {
			t.Errorf("expected 0 successful items, got %d", summary.Successful)
		}
	})

	t.Run("empty manifest executes without error", func(t *testing.T) {
		emptyManifest := &WorkManifest{
			WorkItems: []WorkItem{},
			Groups:    []WorkGroup{},
		}

		summary, err := dispatcher.ExecuteManifest(context.Background(), emptyManifest)
		
		// ExecuteManifest doesn't validate, it just executes what's given
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		
		if summary.TotalItems != 0 {
			t.Errorf("expected 0 total items, got %d", summary.TotalItems)
		}
	})
}

func TestExecuteGroup(t *testing.T) {
	processor := &mockProcessor{processingTime: 10 * time.Millisecond}
	config := DispatcherConfig{MaxWorkers: 2}
	dispatcher := NewDispatcher(config, processor)

	t.Run("successful group execution", func(t *testing.T) {
		manifest := &WorkManifest{
			WorkItems: []WorkItem{
				{ID: "item1", Path: "test1.go", ContentType: "go"},
				{ID: "item2", Path: "test2.go", ContentType: "go"},
			},
			Groups: []WorkGroup{
				{
					ID:          "test-group",
					Name:        "Test Group",
					Strategy:    "parallel",
					WorkItemIDs: []string{"item1", "item2"},
				},
			},
		}

		summary, err := dispatcher.ExecuteGroup(context.Background(), manifest, "test-group")
		
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		
		if summary.TotalItems != 2 {
			t.Errorf("expected 2 total items, got %d", summary.TotalItems)
		}
		
		if summary.Successful != 2 {
			t.Errorf("expected 2 successful items, got %d", summary.Successful)
		}
		
		if summary.Failed != 0 {
			t.Errorf("expected 0 failed items, got %d", summary.Failed)
		}
	})

	t.Run("nonexistent group", func(t *testing.T) {
		manifest := &WorkManifest{
			WorkItems: []WorkItem{},
			Groups:    []WorkGroup{},
		}

		_, err := dispatcher.ExecuteGroup(context.Background(), manifest, "nonexistent")
		
		if err == nil {
			t.Error("expected error for nonexistent group, got nil")
		}
	})
}