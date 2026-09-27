package assess

import (
	"context"
	"fmt"
	"time"

	"github.com/fulmenhq/goneat/internal/gitrepo"
	"github.com/fulmenhq/goneat/pkg/logger"
)

// RepoStatusRunner implements AssessmentRunner for repository status validation
type RepoStatusRunner struct{}

// NewRepoStatusRunner creates a new repo status runner
func NewRepoStatusRunner() *RepoStatusRunner {
	return &RepoStatusRunner{}
}

// Assess runs the repository status assessment
func (r *RepoStatusRunner) Assess(ctx context.Context, target string, config AssessmentConfig) (*AssessmentResult, error) {
	startTime := time.Now()

	var issues []Issue
	var allUncommitted []string

	// Read working-tree status (linked worktrees included). A failure to read
	// status is reported as its own high finding, never as dirty-file metrics.
	st, err := gitrepo.WorktreeStatus(target)
	if err != nil {
		issues = append(issues, Issue{
			File:        "repository",
			Line:        0,
			Severity:    SeverityHigh,
			SubCategory: "git-status-error",
			Message:     fmt.Sprintf("Cannot read git repository state (%v); repository status is unknown", err),
			Category:    r.GetCategory(),
		})
	} else if uncommittedFiles := gitrepo.TrackedChanges(st); len(uncommittedFiles) > 0 {
		// Untracked files alone do not block releases.
		logger.Debug(fmt.Sprintf("repo-status: detected %d uncommitted files", len(uncommittedFiles)))
		allUncommitted = append(allUncommitted, uncommittedFiles...)
		issues = append(issues, Issue{
			File:        "repository",
			Line:        0,
			Severity:    SeverityHigh,
			SubCategory: "git-state",
			Message:     fmt.Sprintf("Repository has %d uncommitted changes (staged or unstaged) - commit all before pushing. Files: %v", len(uncommittedFiles), uncommittedFiles),
			Category:    r.GetCategory(),
		})
	}

	// Determine overall success
	success := len(issues) == 0

	// Build metrics (include sample of uncommitted files for UX)
	metrics := map[string]interface{}{
		"git_checks":   1,
		"issues_found": len(issues),
	}
	if len(allUncommitted) > 0 {
		metrics["uncommitted_files"] = allUncommitted
	}
	// Attach full list; formatter will truncate for display
	// Note: uncommittedFiles is only in scope when repository opened successfully.
	// To keep simple, recompute a minimal list here if issues exist and message included them is not easily parsed.
	// We leave metrics absent when we cannot enumerate.
	// (No-op here; issues construction already had uncommittedFiles in scope.)

	logger.Debug(fmt.Sprintf("repo-status: returning issues=%d success=%v", len(issues), success))

	return &AssessmentResult{
		CommandName:   "repo-status",
		Category:      r.GetCategory(),
		Success:       success,
		Issues:        issues,
		Metrics:       metrics,
		ExecutionTime: HumanReadableDuration(time.Since(startTime)),
	}, nil
}

// CanRunInParallel returns true - repo status checking can run in parallel
func (r *RepoStatusRunner) CanRunInParallel() bool {
	return true
}

// GetCategory returns the repo-status category
func (r *RepoStatusRunner) GetCategory() AssessmentCategory {
	return CategoryRepoStatus
}

// GetEstimatedTime provides a rough time estimate for repo status checking
func (r *RepoStatusRunner) GetEstimatedTime(target string) time.Duration {
	// Git status is typically fast
	return 500 * time.Millisecond
}

// IsAvailable returns whether repo status checking is available
func (r *RepoStatusRunner) IsAvailable() bool {
	// Always available since we use go-git which is embedded
	return true
}

// Register the repo-status runner
func init() {
	RegisterAssessmentRunner(CategoryRepoStatus, NewRepoStatusRunner())
}
