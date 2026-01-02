/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package assess

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/fulmenhq/goneat/pkg/config"
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

// SchemaMappingConfig configures config-to-schema mapping behavior for assessments.
type SchemaMappingConfig struct {
	Enabled       bool    `json:"enabled,omitempty"`
	ManifestPath  string  `json:"manifest_path,omitempty"`
	MinConfidence float64 `json:"min_confidence,omitempty"`
	Strict        bool    `json:"strict,omitempty"`
}

// AssessmentConfig contains configuration for running assessments
type AssessmentConfig struct {
	Mode         AssessmentMode `json:"mode"`          // Operation mode
	Verbose      bool           `json:"verbose"`       // Verbose output
	Timeout      time.Duration  `json:"timeout"`       // Assessment timeout
	IncludeFiles []string       `json:"include_files"` // Files to include
	ExcludeFiles []string       `json:"exclude_files"` // Files to exclude
	// Ignore behavior overrides
	NoIgnore           bool          `json:"no_ignore"`           // Disable .goneatignore/.gitignore during discovery
	ForceInclude       []string      `json:"force_include"`       // Globs/paths to force-include even if ignored
	PriorityString     string        `json:"priority_string"`     // Custom priority string
	FailOnSeverity     IssueSeverity `json:"fail_on_severity"`    // Fail if issues at or above this severity
	SelectedCategories []string      `json:"selected_categories"` // If set, restrict assessment to these categories
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
	SchemaEnableMeta    bool                `json:"schema_enable_meta,omitempty"`    // Enable meta-schema validation
	SchemaDrafts        []string            `json:"schema_drafts,omitempty"`         // Filter by specific drafts (e.g., ["draft-07", "2020-12"])
	SchemaPatterns      []string            `json:"schema_patterns,omitempty"`       // Custom glob patterns for schema files
	SchemaDiscoveryMode string              `json:"schema_discovery_mode,omitempty"` // Discovery mode: "schemas-dir" (default) or "all"
	SchemaMapping       SchemaMappingConfig `json:"schema_mapping,omitempty"`

	// Scoped discovery (limits traversal to include dirs and force-include anchors)
	Scope bool `json:"scope,omitempty"`

	// Lint package mode (force package-based linting instead of individual files)
	PackageMode bool `json:"package_mode,omitempty"`

	// Extended output with detailed workplan information
	Extended bool `json:"extended,omitempty"`

	// Security per-tool timeouts (optional)
	SecurityGosecTimeout       time.Duration `json:"security_timeout_gosec,omitempty"`
	SecurityGovulncheckTimeout time.Duration `json:"security_timeout_govulncheck,omitempty"`

	// Security results hygiene
	SecurityExcludeFixtures bool     `json:"security_exclude_fixtures,omitempty"`
	SecurityFixturePatterns []string `json:"security_fixture_patterns,omitempty"`

	// Lint new-only control (golangci-lint --new-from-rev)
	LintNewFromRev string `json:"lint_new_from_rev,omitempty"`

	// Incremental checking (cross-tool: golangci-lint, biome)
	// When NewIssuesOnly is true, only issues introduced since NewIssuesBase are reported
	NewIssuesOnly bool   `json:"new_issues_only,omitempty"`
	NewIssuesBase string `json:"new_issues_base,omitempty"`

	// Lint extensions (shell/make/GHA)
	LintShellEnabled      bool     `json:"lint_shell_enabled,omitempty"`
	LintShellFix          bool     `json:"lint_shell_fix,omitempty"`
	LintShellPaths        []string `json:"lint_shell_paths,omitempty"`
	LintShellExclude      []string `json:"lint_shell_exclude,omitempty"`
	LintShellcheckEnabled bool     `json:"lint_shellcheck_enabled,omitempty"`
	LintShellcheckPath    string   `json:"lint_shellcheck_path,omitempty"`
	LintGHAEnabled        bool     `json:"lint_gha_enabled,omitempty"`
	LintGHAPaths          []string `json:"lint_gha_paths,omitempty"`
	LintGHAExclude        []string `json:"lint_gha_exclude,omitempty"`
	LintMakeEnabled       bool     `json:"lint_make_enabled,omitempty"`
	LintMakePaths         []string `json:"lint_make_paths,omitempty"`
	LintMakeExclude       []string `json:"lint_make_exclude,omitempty"`
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
		// Exclude common fixture paths by default to reduce noise
		SecurityExcludeFixtures: true,
		SecurityFixturePatterns: []string{"tests/fixtures/", "test-fixtures/"},
		// Schema
		SchemaEnableMeta: false,
		SchemaMapping:    SchemaMappingConfig{},
		// Scoped discovery default off
		Scope: false,
		// Lint extensions defaults
		LintShellEnabled:      true,
		LintShellFix:          false,
		LintShellPaths:        []string{"**/*.sh", "scripts/**/*.sh"},
		LintShellExclude:      []string{"**/node_modules/**", "**/.git/**", "**/vendor/**", "**/*.orig", "**/testdata/**", ".plans/**"},
		LintShellcheckEnabled: false,
		LintShellcheckPath:    "",
		LintGHAEnabled:        true,
		LintGHAPaths:          []string{".github/workflows/**/*.yml", ".github/workflows/**/*.yaml"},
		LintGHAExclude:        []string{},
		LintMakeEnabled:       true,
		LintMakePaths:         []string{"Makefile", "**/Makefile"},
		LintMakeExclude:       []string{},
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

// ResetRegistryForTesting creates a fresh registry and sets it globally for test isolation
func ResetRegistryForTesting() *AssessmentRunnerRegistry {
	newReg := NewAssessmentRunnerRegistry()
	globalRunnerRegistry = newReg
	return newReg
}

// RestoreRegistry restores a previously saved registry for test teardown
func RestoreRegistry(saved *AssessmentRunnerRegistry) {
	globalRunnerRegistry = saved
}

// ConfigResolver provides standardized config file resolution for assessment runners
type ConfigResolver struct {
	workingDir string
}

// NewConfigResolver creates a config resolver for the given target
// For single files, uses the file's directory as working directory for config resolution
func NewConfigResolver(target string) *ConfigResolver {
	workingDir := target
	if info, err := os.Stat(target); err == nil && !info.IsDir() {
		// Target is a file - use its directory for config resolution
		workingDir = filepath.Dir(target)
	}

	// Ensure we have an absolute path for consistent behavior
	if absDir, err := filepath.Abs(workingDir); err == nil {
		workingDir = absDir
	}

	return &ConfigResolver{workingDir: workingDir}
}

// GetWorkingDir returns the working directory for config resolution
func (cr *ConfigResolver) GetWorkingDir() string {
	return cr.workingDir
}

// ResolveConfigFile finds category-specific config files using standardized search paths
// Search order:
// 1. .goneat/{category}.yaml (project-level)
// 2. GONEAT_HOME/config/{category}.yaml (user-level)
// 3. fallback to defaults
func (cr *ConfigResolver) ResolveConfigFile(category string) (string, bool) {
	// 1. Project-level config
	projectConfig := filepath.Join(cr.workingDir, ".goneat", category+".yaml")
	if info, err := os.Stat(projectConfig); err == nil && !info.IsDir() {
		return projectConfig, true
	}

	// 2. User-level config (GONEAT_HOME)
	if configDir, err := config.GetConfigDir(); err == nil {
		userConfig := filepath.Join(configDir, category+".yaml")
		if info, err := os.Stat(userConfig); err == nil && !info.IsDir() {
			return userConfig, true
		}
	}

	// 3. No config file found
	return "", false
}
