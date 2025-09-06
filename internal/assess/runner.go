/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package assess

import (
	"context"
	"time"
)

// AssessmentRunner defines the interface that commands must implement to participate in assessments
type AssessmentRunner interface {
	// Assess runs the assessment and returns results
	Assess(ctx context.Context, target string, config AssessmentConfig) (*AssessmentResult, error)

	// CanRunInParallel returns whether this assessment can run in parallel with others
	CanRunInParallel() bool

	// GetCategory returns the assessment category this runner handles
	GetCategory() AssessmentCategory

	// GetEstimatedTime provides a rough time estimate for the assessment
	GetEstimatedTime(target string) time.Duration

	// IsAvailable returns whether this assessment runner is available (tools installed, etc.)
	IsAvailable() bool
}

// AssessmentMode represents the operation mode for assessments
type AssessmentMode string

const (
	AssessmentModeNoOp  AssessmentMode = "no-op" // Assessment only, no changes
	AssessmentModeCheck AssessmentMode = "check" // Report issues, no changes
	AssessmentModeFix   AssessmentMode = "fix"   // Report and fix issues
)

// AssessmentConfig contains configuration for running assessments
type AssessmentConfig struct {
    Mode               AssessmentMode `json:"mode"`                // Operation mode
    Verbose            bool           `json:"verbose"`             // Verbose output
    Timeout            time.Duration  `json:"timeout"`             // Assessment timeout
    IncludeFiles       []string       `json:"include_files"`       // Files to include
    ExcludeFiles       []string       `json:"exclude_files"`       // Files to exclude
    // Ignore behavior overrides
    NoIgnore           bool           `json:"no_ignore"`            // Disable .goneatignore/.gitignore during discovery
    ForceInclude       []string       `json:"force_include"`        // Globs/paths to force-include even if ignored
    PriorityString     string         `json:"priority_string"`     // Custom priority string
    FailOnSeverity     IssueSeverity  `json:"fail_on_severity"`    // Fail if issues at or above this severity
    SelectedCategories []string       `json:"selected_categories"` // If set, restrict assessment to these categories
	// Concurrency controls
	// If Concurrency > 0 it is used directly. Otherwise ConcurrencyPercent determines worker count
	// as a percentage of available CPU cores (1-100). Values <=0 default to 50.
	Concurrency        int `json:"concurrency"`
	ConcurrencyPercent int `json:"concurrency_percent"`

	// Security-specific controls (no schema change yet; ephemeral)
	SecurityTools []string `json:"security_tools,omitempty"`
	EnableVuln    bool     `json:"enable_vuln"`
	EnableCode    bool     `json:"enable_code"`
	EnableSecrets bool     `json:"enable_secrets"`

	// Suppression tracking
    TrackSuppressions bool `json:"track_suppressions,omitempty"`

    // Schema options (preview)
    SchemaEnableMeta bool `json:"schema_enable_meta,omitempty"`

    // Scoped discovery (limits traversal to include dirs and force-include anchors)
    Scope bool `json:"scope,omitempty"`

	// Security per-tool timeouts (optional)
	SecurityGosecTimeout       time.Duration `json:"security_timeout_gosec,omitempty"`
	SecurityGovulncheckTimeout time.Duration `json:"security_timeout_govulncheck,omitempty"`
}

// DefaultAssessmentConfig returns default assessment configuration
func DefaultAssessmentConfig() AssessmentConfig {
    return AssessmentConfig{
        Mode:               AssessmentModeCheck, // Default to check mode for safety
        Verbose:            false,
        Timeout:            5 * time.Minute,
        IncludeFiles:       []string{},
        ExcludeFiles:       []string{},
        NoIgnore:           false,
        ForceInclude:       []string{},
        PriorityString:     "",
        FailOnSeverity:     SeverityCritical,
        SelectedCategories: []string{},
        Concurrency:        0,
		ConcurrencyPercent: 50,
		// Security defaults
		SecurityTools: []string{},
		EnableVuln:    true,
		EnableCode:    true,
        EnableSecrets: false,
        // Per-tool timeouts default to 0 (inherit global)
        SecurityGosecTimeout:       0,
        SecurityGovulncheckTimeout: 0,
        // Schema
        SchemaEnableMeta: false,
        // Scoped discovery default off
        Scope: false,
    }
}

// AssessmentRunnerRegistry manages available assessment runners
type AssessmentRunnerRegistry struct {
	runners map[AssessmentCategory]AssessmentRunner
}

// NewAssessmentRunnerRegistry creates a new registry for assessment runners
func NewAssessmentRunnerRegistry() *AssessmentRunnerRegistry {
	return &AssessmentRunnerRegistry{
		runners: make(map[AssessmentCategory]AssessmentRunner),
	}
}

// RegisterRunner registers an assessment runner for a category
func (r *AssessmentRunnerRegistry) RegisterRunner(category AssessmentCategory, runner AssessmentRunner) {
	r.runners[category] = runner
}

// GetRunner returns the runner for a category
func (r *AssessmentRunnerRegistry) GetRunner(category AssessmentCategory) (AssessmentRunner, bool) {
	runner, exists := r.runners[category]
	return runner, exists
}

// GetAvailableCategories returns categories that have available runners
func (r *AssessmentRunnerRegistry) GetAvailableCategories() []AssessmentCategory {
	var categories []AssessmentCategory
	for category, runner := range r.runners {
		if runner.IsAvailable() {
			categories = append(categories, category)
		}
	}
	return categories
}

// GetAllCategories returns all registered categories (available or not)
func (r *AssessmentRunnerRegistry) GetAllCategories() []AssessmentCategory {
	var categories []AssessmentCategory
	for category := range r.runners {
		categories = append(categories, category)
	}
	return categories
}

// Global registry instance
var globalRunnerRegistry = NewAssessmentRunnerRegistry()

// RegisterAssessmentRunner registers a runner globally
func RegisterAssessmentRunner(category AssessmentCategory, runner AssessmentRunner) {
	globalRunnerRegistry.RegisterRunner(category, runner)
}

// GetAssessmentRunnerRegistry returns the global runner registry
func GetAssessmentRunnerRegistry() *AssessmentRunnerRegistry {
	return globalRunnerRegistry
}
