package assess

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fulmenhq/goneat/internal/maturity"
	git "github.com/go-git/go-git/v5"
)

// MaturityRunner implements AssessmentRunner for maturity validation
type MaturityRunner struct{}

// NewMaturityRunner creates a new maturity runner
func NewMaturityRunner() *MaturityRunner {
	return &MaturityRunner{}
}

// Assess runs the maturity assessment
func (r *MaturityRunner) Assess(ctx context.Context, target string, config AssessmentConfig) (*AssessmentResult, error) {
	startTime := time.Now()

	var issues []Issue
	var dirtyFilesList []string

	// Check if RELEASE_PHASE and LIFECYCLE_PHASE files exist
	releasePhaseFile := filepath.Join(target, "RELEASE_PHASE")
	lifecyclePhaseFile := filepath.Join(target, "LIFECYCLE_PHASE")

	// Check RELEASE_PHASE
	// #nosec G304 -- releasePhaseFile constructed from controlled target and hardcoded filename
	releasePhaseContent, err := os.ReadFile(releasePhaseFile)
	if err != nil {
		issues = append(issues, Issue{
			File:     "RELEASE_PHASE",
			Line:     0,
			Severity: SeverityHigh,
			Message:  "RELEASE_PHASE file not found - required for maturity validation",
			Category: r.GetCategory(),
		})
	} else {
		releasePhaseStr := strings.TrimSpace(string(releasePhaseContent))
		releasePhase := maturity.ReleasePhase(releasePhaseStr)
		if !releasePhase.IsValid() {
			issues = append(issues, Issue{
				File:     "RELEASE_PHASE",
				Line:     0,
				Severity: SeverityHigh,
				Message:  fmt.Sprintf("Invalid RELEASE_PHASE: '%s' (valid: dev, rc, release, hotfix)", releasePhaseStr),
				Category: r.GetCategory(),
			})
		}
	}

	// Check LIFECYCLE_PHASE
	// #nosec G304 -- lifecyclePhaseFile constructed from controlled target and hardcoded filename
	lifecyclePhaseContent, err := os.ReadFile(lifecyclePhaseFile)
	if err != nil {
		issues = append(issues, Issue{
			File:     "LIFECYCLE_PHASE",
			Line:     0,
			Severity: SeverityHigh,
			Message:  "LIFECYCLE_PHASE file not found - required for maturity validation",
			Category: r.GetCategory(),
		})
	} else {
		lifecyclePhaseStr := strings.TrimSpace(string(lifecyclePhaseContent))
		lifecyclePhase := maturity.LifecyclePhase(lifecyclePhaseStr)
		if !lifecyclePhase.IsValid() {
			issues = append(issues, Issue{
				File:     "LIFECYCLE_PHASE",
				Line:     0,
				Severity: SeverityHigh,
				Message:  fmt.Sprintf("Invalid LIFECYCLE_PHASE: '%s' (valid: alpha, beta, ga, maintenance)", lifecyclePhaseStr),
				Category: r.GetCategory(),
			})
		}
	}

	// Check VERSION file consistency with phases
	versionFile := filepath.Join(target, "VERSION")
	// #nosec G304 -- versionFile constructed from controlled target and hardcoded filename
	versionContent, err := os.ReadFile(versionFile)
	if err != nil {
		issues = append(issues, Issue{
			File:     "VERSION",
			Line:     0,
			Severity: SeverityHigh,
			Message:  "VERSION file not found - required for maturity validation",
			Category: r.GetCategory(),
		})
	} else {
		versionStr := strings.TrimSpace(string(versionContent))

		// For release phase, version should not have suffixes
		if releasePhaseContent != nil {
			releasePhaseStr := strings.TrimSpace(string(releasePhaseContent))
			if releasePhaseStr == "release" {
				if strings.Contains(versionStr, "-") {
					issues = append(issues, Issue{
						File:     "VERSION",
						Line:     0,
						Severity: SeverityHigh,
						Message:  fmt.Sprintf("VERSION '%s' has suffix but RELEASE_PHASE is 'release' - production releases should not have suffixes", versionStr),
						Category: r.GetCategory(),
					})
				}
			}
		}

		// Git state check for strict phases
		if releasePhaseContent != nil {
			releasePhaseStr := strings.TrimSpace(string(releasePhaseContent))
			if releasePhaseStr == "rc" || releasePhaseStr == "release" || releasePhaseStr == "hotfix" {
				repo, err := git.PlainOpenWithOptions(target, &git.PlainOpenOptions{DetectDotGit: true})
				if err == nil {
					wt, err := repo.Worktree()
					if err == nil {
						st, err := wt.Status()
						if err == nil {
							dirty := false
							var dirtyFiles []string
							for path, fs := range st {
								// Skip untracked files (these don't block releases)
								if fs.Staging == git.Untracked {
									continue
								}
								if fs.Worktree != git.Unmodified || fs.Staging != git.Unmodified {
									dirty = true
									dirtyFiles = append(dirtyFiles, path)
								}
							}
							if dirty {
								issues = append(issues, Issue{
									File:        "repository",
									Line:        0,
									Severity:    SeverityHigh,
									SubCategory: "git-state",
									Message:     fmt.Sprintf("Dirty git state (uncommitted changes) not allowed in %s phase - commit or stash all changes. Files: %v", releasePhaseStr, dirtyFiles),
									Category:    r.GetCategory(),
								})
								// Attach dirty files for metrics later
								dirtyFilesList = append(dirtyFilesList, dirtyFiles...)
							}
						}
					}
				}
			}
		}
	}

	// Determine overall success
	success := len(issues) == 0

	// Build metrics
	metrics := map[string]interface{}{
		"phase_files_checked":   2,
		"version_files_checked": 1,
		"issues_found":          len(issues),
	}
	if len(dirtyFilesList) > 0 {
		metrics["git_dirty_files"] = dirtyFilesList
	}

	return &AssessmentResult{
		CommandName:   "maturity",
		Category:      r.GetCategory(),
		Success:       success,
		Issues:        issues,
		Metrics:       metrics,
		ExecutionTime: HumanReadableDuration(time.Since(startTime)),
	}, nil
}

// CanRunInParallel returns true - maturity checking can run in parallel
func (r *MaturityRunner) CanRunInParallel() bool {
	return true
}

// GetCategory returns the maturity category
func (r *MaturityRunner) GetCategory() AssessmentCategory {
	return CategoryMaturity
}

// GetEstimatedTime provides a rough time estimate for maturity checking
func (r *MaturityRunner) GetEstimatedTime(target string) time.Duration {
	// Maturity checking is fast (just file reads)
	return 1 * time.Second
}

// IsAvailable returns whether maturity checking is available
func (r *MaturityRunner) IsAvailable() bool {
	// Maturity checking is always available (just checks files)
	return true
}

// Register the maturity runner
func init() {
	RegisterAssessmentRunner(CategoryMaturity, NewMaturityRunner())
}
