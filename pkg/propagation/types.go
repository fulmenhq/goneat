// Package propagation implements version propagation from VERSION file to package manager files
package propagation

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/fulmenhq/goneat/pkg/logger"
	"github.com/fulmenhq/goneat/pkg/pathfinder"
	git "github.com/go-git/go-git/v5"
)

// PackageManager defines the interface for package manager implementations
type PackageManager interface {
	// Name returns the name of the package manager (e.g., "package.json", "pyproject.toml")
	Name() string

	// Detect scans the given root directory and returns paths to files this manager can handle
	Detect(root string) ([]string, error)

	// ExtractVersion reads the version from the specified file
	ExtractVersion(file string) (string, error)

	// UpdateVersion updates the version in the specified file
	UpdateVersion(file, version string) error

	// ValidateVersion checks if the version in the file matches the expected version
	ValidateVersion(file, version string) error
}

// Registry manages available package managers
type Registry struct {
	managers map[string]PackageManager
}

// NewRegistry creates a new package manager registry
func NewRegistry() *Registry {
	return &Registry{
		managers: make(map[string]PackageManager),
	}
}

// Register adds a package manager to the registry
func (r *Registry) Register(manager PackageManager) {
	r.managers[manager.Name()] = manager
}

// Get returns a package manager by name
func (r *Registry) Get(name string) (PackageManager, bool) {
	manager, exists := r.managers[name]
	return manager, exists
}

// List returns all registered package managers
func (r *Registry) List() []PackageManager {
	var managers []PackageManager
	for _, manager := range r.managers {
		managers = append(managers, manager)
	}
	return managers
}

// Propagator handles version propagation operations
type Propagator struct {
	registry *Registry
	policy   *PolicyLoader
	engine   *pathfinder.DiscoveryEngine
}

// Note: Uses pkg/logger for logging operations

// PropagateOptions configures propagation behavior
type PropagateOptions struct {
	DryRun       bool     // Preview changes without making them
	Force        bool     // Overwrite files without confirmation
	Targets      []string // Specific files or package managers to target
	Exclude      []string // Files to exclude from propagation
	Backup       bool     // Create backup files before changes
	ValidateOnly bool     // Only validate current version consistency
	PolicyPath   string   // Optional override for policy file path
}

// PropagationResult contains the outcome of a propagation operation
type PropagationResult struct {
	Success   bool
	Processed int
	Errors    []PropagationError
	Changes   []FileChange
	Duration  time.Duration
}

// PropagationError represents an error during propagation
type PropagationError struct {
	File    string
	Error   error
	Message string
}

// FileChange represents a change made to a file
type FileChange struct {
	File       string
	OldVersion string
	NewVersion string
	BackupPath string // Empty if no backup created
}

// NewPropagator creates a new version propagator
func NewPropagator(registry *Registry) *Propagator {
	// Initialize pathfinder components for pattern matching (dogfooding our own library)
	validator := pathfinder.NewSafetyValidator()
	validator.SetAllowSymlinks(true)
	engine := pathfinder.NewDiscoveryEngine(validator)

	return &Propagator{
		registry: registry,
		policy:   NewPolicyLoader(),
		engine:   engine,
	}
}

// filterManagersByPolicy filters package managers based on policy include/exclude rules
func (p *Propagator) filterManagersByPolicy(managers []PackageManager, policy *VersionPolicy) []PackageManager {
	filtered := make([]PackageManager, 0, len(managers))

	for _, manager := range managers {
		managerName := manager.Name()

		// Check if this manager type is in the default includes
		included := false
		for _, include := range policy.Propagation.Defaults.Include {
			if include == managerName {
				included = true
				break
			}
		}

		// If not in defaults, check if there are target-specific rules that include it
		if !included {
			if target, exists := policy.Propagation.Targets[managerName]; exists && len(target.Include) > 0 {
				included = true
			}
		}

		// Target-specific excludes can override includes
		if target, exists := policy.Propagation.Targets[managerName]; exists {
			for _, exclude := range target.Exclude {
				if exclude == managerName {
					included = false
					break
				}
			}
		}

		if included {
			filtered = append(filtered, manager)
		}
	}

	return filtered
}

// checkGuards validates execution preconditions defined in the policy
func (p *Propagator) checkGuards(policy *VersionPolicy) error {
	guards := policy.Guards

	// Check required branches
	if len(guards.RequiredBranches) > 0 {
		currentBranch, err := p.getCurrentBranch()
		if err != nil {
			return fmt.Errorf("failed to get current branch: %w", err)
		}

		allowed := false
		for _, pattern := range guards.RequiredBranches {
			if matched, err := filepath.Match(pattern, currentBranch); err == nil && matched {
				allowed = true
				break
			}
		}

		if !allowed {
			return fmt.Errorf("current branch '%s' not in required branches: %v", currentBranch, guards.RequiredBranches)
		}
	}

	// Check dirty worktree
	if guards.DisallowDirtyWorktree {
		if dirty, err := p.isWorktreeDirty(); err != nil {
			return fmt.Errorf("failed to check worktree status: %w", err)
		} else if dirty {
			return fmt.Errorf("worktree has uncommitted changes (dirty worktree not allowed)")
		}
	}

	return nil
}

// getCurrentBranch returns the current git branch name using go-git
func (p *Propagator) getCurrentBranch() (string, error) {
	// Open repository at current directory (follows pattern from internal/gitctx/gitctx.go)
	repo, err := git.PlainOpenWithOptions(".", &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		// Not a git repository or can't access it
		return "", fmt.Errorf("failed to open git repository: %w", err)
	}

	head, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD reference: %w", err)
	}

	// Return short branch name (e.g., "main" instead of "refs/heads/main")
	return head.Name().Short(), nil
}

// isWorktreeDirty checks if the git worktree has uncommitted changes using go-git
func (p *Propagator) isWorktreeDirty() (bool, error) {
	// Open repository at current directory (follows pattern from internal/gitctx/gitctx.go)
	repo, err := git.PlainOpenWithOptions(".", &git.PlainOpenOptions{DetectDotGit: true})
	if err != nil {
		// Not a git repository or can't access it
		return false, fmt.Errorf("failed to open git repository: %w", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		return false, fmt.Errorf("failed to get worktree: %w", err)
	}

	st, err := wt.Status()
	if err != nil {
		return false, fmt.Errorf("failed to get worktree status: %w", err)
	}

	// IsClean returns true if there are no changes
	return !st.IsClean(), nil
}

// filterFilesByPolicy filters files based on policy include/exclude patterns and workspace strategy
func (p *Propagator) filterFilesByPolicy(files []string, managerName string, policy *VersionPolicy, explicitExcludes []string) []string {
	filtered := make([]string, 0, len(files))

	// Get target-specific rules for this manager
	var includePatterns []string
	var excludePatterns []string

	if target, exists := policy.Propagation.Targets[managerName]; exists {
		includePatterns = target.Include
		excludePatterns = target.Exclude
	}

	// If no target-specific includes, use defaults
	if len(includePatterns) == 0 {
		includePatterns = policy.Propagation.Defaults.Include
	}

	// Always apply default excludes
	excludePatterns = append(excludePatterns, policy.Propagation.Defaults.Exclude...)

	// Add explicit excludes from options
	excludePatterns = append(excludePatterns, explicitExcludes...)

	// Apply workspace strategy filtering
	files = p.applyWorkspaceStrategy(files, policy.Propagation.Workspace)

	for _, file := range files {
		// Normalize file path to use forward slashes for consistent matching
		normalizedFile := filepath.ToSlash(file)

		// Check include patterns using pathfinder (dogfooding with proper normalization)
		included := len(includePatterns) == 0 || p.engine.MatchesAnyPattern(normalizedFile, includePatterns)

		// Check exclude patterns
		if included && len(excludePatterns) > 0 {
			if p.engine.MatchesAnyPattern(normalizedFile, excludePatterns) {
				included = false
			}
		}

		if included {
			filtered = append(filtered, file)
		}
	}

	return filtered
}

// applyWorkspaceStrategy applies workspace strategy filtering to file list
func (p *Propagator) applyWorkspaceStrategy(files []string, workspace WorkspaceConfig) []string {
	switch workspace.Strategy {
	case "single-version":
		// Default behavior: allow all files (single version for entire workspace)
		return files
	case "opt-in":
		// Only allow files that match allowlist patterns
		if len(workspace.Allowlist) == 0 {
			return []string{} // No files allowed if no allowlist
		}
		filtered := make([]string, 0)
		for _, file := range files {
			normalizedFile := filepath.ToSlash(file)
			// Use pathfinder engine for consistent pattern matching
			if p.engine.MatchesAnyPattern(normalizedFile, workspace.Allowlist) {
				filtered = append(filtered, file)
			}
		}
		return filtered
	case "opt-out":
		// Allow all files except those matching blocklist
		if len(workspace.Blocklist) == 0 {
			return files // All files allowed if no blocklist
		}
		filtered := make([]string, 0)
		for _, file := range files {
			normalizedFile := filepath.ToSlash(file)
			// Use pathfinder engine for consistent pattern matching
			if !p.engine.MatchesAnyPattern(normalizedFile, workspace.Blocklist) {
				filtered = append(filtered, file)
			}
		}
		return filtered
	default:
		// Default to single-version behavior
		return files
	}
}

// Propagate executes version propagation according to the given options
func (p *Propagator) Propagate(ctx context.Context, version string, opts PropagateOptions) (*PropagationResult, error) {
	start := time.Now()

	policy, err := p.policy.LoadPolicy(opts.PolicyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load version policy: %w", err)
	}

	// Check guards (execution preconditions)
	if err := p.checkGuards(policy); err != nil {
		return nil, fmt.Errorf("guard check failed: %w", err)
	}

	logger.Debug("Propagation started", logger.String("version", version))

	targetManagers := p.registry.List()

	// Apply policy-driven filtering when no explicit targets specified
	if len(opts.Targets) == 0 {
		targetManagers = p.filterManagersByPolicy(targetManagers, policy)
	} else {
		// Explicit targets override policy
		targetMap := make(map[string]struct{}, len(opts.Targets))
		for _, target := range opts.Targets {
			targetMap[strings.ToLower(target)] = struct{}{}
		}

		filtered := make([]PackageManager, 0, len(targetManagers))
		for _, manager := range targetManagers {
			if _, ok := targetMap[strings.ToLower(manager.Name())]; ok {
				filtered = append(filtered, manager)
			}
		}
		targetManagers = filtered
	}

	logger.Debug("Managers selected for propagation", logger.Int("count", len(targetManagers)))

	result := &PropagationResult{
		Success:   true,
		Processed: 0,
	}

	for _, manager := range targetManagers {
		select {
		case <-ctx.Done():
			result.Success = false
			result.Errors = append(result.Errors, PropagationError{
				Message: fmt.Sprintf("propagation cancelled: %v", ctx.Err()),
			})
			result.Duration = time.Since(start)
			return result, ctx.Err()
		default:
		}

		logger.Debug("Processing manager", logger.String("manager", manager.Name()))

		// Detect files for this manager
		files, err := manager.Detect(".")
		if err != nil {
			result.Errors = append(result.Errors, PropagationError{
				Message: fmt.Sprintf("failed to detect files for %s: %v", manager.Name(), err),
			})
			result.Success = false
			continue
		}

		// Apply policy-based file filtering
		filteredFiles := p.filterFilesByPolicy(files, manager.Name(), policy, opts.Exclude)

		logger.Debug("Files detected and filtered",
			logger.String("manager", manager.Name()),
			logger.Int("detected", len(files)),
			logger.Int("included", len(filteredFiles)))

		// Check if this manager has validate_only set in policy
		managerValidateOnly := false
		if target, exists := policy.Propagation.Targets[manager.Name()]; exists && target.ValidateOnly {
			managerValidateOnly = true
		}

		// Process each filtered file
		for _, file := range filteredFiles {
			if opts.ValidateOnly || managerValidateOnly {
				// Only validate version consistency
				if err := manager.ValidateVersion(file, version); err != nil {
					result.Errors = append(result.Errors, PropagationError{
						File:    file,
						Error:   err,
						Message: fmt.Sprintf("version validation failed for %s", file),
					})
					result.Success = false
				}
			} else {
				// Extract current version for change tracking
				oldVersion, err := manager.ExtractVersion(file)
				if err != nil {
					result.Errors = append(result.Errors, PropagationError{
						File:    file,
						Error:   err,
						Message: fmt.Sprintf("failed to extract version from %s", file),
					})
					result.Success = false
					continue
				}

				// Skip if already at correct version
				if oldVersion == version {
					logger.Debug("File already at correct version", logger.String("file", file))
					continue
				}

				if !opts.DryRun {
					// Update version
					if err := manager.UpdateVersion(file, version); err != nil {
						result.Errors = append(result.Errors, PropagationError{
							File:    file,
							Error:   err,
							Message: fmt.Sprintf("failed to update version in %s", file),
						})
						result.Success = false
						continue
					}
				}

				// Record the change
				change := FileChange{
					File:       file,
					OldVersion: oldVersion,
					NewVersion: version,
				}
				result.Changes = append(result.Changes, change)
			}

			result.Processed++
		}
	}

	result.Duration = time.Since(start)
	return result, nil
}
