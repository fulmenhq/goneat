package assess

import (
	"context"
	"time"
)

// fakeRunner is a lightweight AssessmentRunner for tests.
type fakeRunner struct {
	category    AssessmentCategory
	available   bool
	canParallel bool
	delay       time.Duration
	issues      []Issue
	metrics     map[string]interface{}
	returnError error
	commandName string
}

func (f *fakeRunner) Assess(ctx context.Context, target string, cfg AssessmentConfig) (*AssessmentResult, error) {
	// Simulate work and honor context cancellation/timeout
	timer := time.NewTimer(f.delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
	}
	if f.returnError != nil {
		return nil, f.returnError
	}
	return &AssessmentResult{
		CommandName:   ifEmpty(f.commandName, string(f.category)),
		Category:      f.category,
		Success:       true,
		ExecutionTime: f.delay,
		Issues:        append([]Issue(nil), f.issues...),
		Metrics:       f.metrics,
	}, nil
}

func (f *fakeRunner) CanRunInParallel() bool                       { return f.canParallel }
func (f *fakeRunner) GetCategory() AssessmentCategory              { return f.category }
func (f *fakeRunner) GetEstimatedTime(target string) time.Duration { return time.Second }
func (f *fakeRunner) IsAvailable() bool                            { return f.available }

func ifEmpty(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
