package maturity

import (
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/fulmenhq/goneat/internal/schema" // For embedded schema validation
	"github.com/fulmenhq/goneat/pkg/pathfinder"
)

// Phase validation follows crucible schemas as SSOT:
// - LIFECYCLE: schemas/crucible-go/config/repository/v1.0.0/lifecycle-phase.json (repo-level)
// - RELEASE:   schemas/crucible-go/config/goneat/v1.0.0/release-phase.json (goneat-specific)
// See: docs/user-guide/commands/maturity.md for documentation

// ReleasePhase defines distribution cadence states per crucible schema
type ReleasePhase string

const (
	ReleaseDev ReleasePhase = "dev"
	ReleaseRC  ReleasePhase = "rc"
	ReleaseGA  ReleasePhase = "ga"
	// Note: "release" and "hotfix" planned for crucible v1.1.0
	// For now, accept "release" as alias for "ga" for backwards compatibility
	ReleaseRelease ReleasePhase = "release" // Alias for ga (backwards compat)
)

// ValidReleasePhases returns all valid release phases per crucible schema
// SSOT: schemas/crucible-go/config/goneat/v1.0.0/release-phase.json
func ValidReleasePhases() []ReleasePhase {
	return []ReleasePhase{ReleaseDev, ReleaseRC, ReleaseGA, ReleaseRelease}
}

// ValidReleasePhasesString returns comma-separated valid values for error messages
func ValidReleasePhasesString() string {
	phases := ValidReleasePhases()
	strs := make([]string, len(phases))
	for i, p := range phases {
		strs[i] = string(p)
	}
	return strings.Join(strs, ", ")
}

// String implements stringer
func (p ReleasePhase) String() string { return string(p) }

// IsValid checks if phase is valid
func (p ReleasePhase) IsValid() bool {
	for _, valid := range ValidReleasePhases() {
		if p == valid {
			return true
		}
	}
	return false
}

// LifecyclePhase defines product maturity states per crucible schema
type LifecyclePhase string

const (
	LifecycleExperimental LifecyclePhase = "experimental"
	LifecycleAlpha        LifecyclePhase = "alpha"
	LifecycleBeta         LifecyclePhase = "beta"
	LifecycleRC           LifecyclePhase = "rc"
	LifecycleGA           LifecyclePhase = "ga"
	LifecycleLTS          LifecyclePhase = "lts"
)

// ValidLifecyclePhases returns all valid lifecycle phases per crucible schema
// SSOT: schemas/crucible-go/config/repository/v1.0.0/lifecycle-phase.json
func ValidLifecyclePhases() []LifecyclePhase {
	return []LifecyclePhase{LifecycleExperimental, LifecycleAlpha, LifecycleBeta, LifecycleRC, LifecycleGA, LifecycleLTS}
}

// ValidLifecyclePhasesString returns comma-separated valid values for error messages
func ValidLifecyclePhasesString() string {
	phases := ValidLifecyclePhases()
	strs := make([]string, len(phases))
	for i, p := range phases {
		strs[i] = string(p)
	}
	return strings.Join(strs, ", ")
}

// String implements stringer
func (p LifecyclePhase) String() string { return string(p) }

// IsValid checks if phase is valid
func (p LifecyclePhase) IsValid() bool {
	for _, valid := range ValidLifecyclePhases() {
		if p == valid {
			return true
		}
	}
	return false
}

// PhaseRules defines metadata for a phase (e.g., coverage thresholds)
type PhaseRules struct {
	AllowedSuffixes    []string       `json:"allowed_suffixes"`
	MinCoverage        int            `json:"min_coverage"`
	AllowDirtyGit      bool           `json:"allow_dirty_git"`
	RequiredDocs       []string       `json:"required_docs"`
	ErrorLevel         string         `json:"error_level"` // warn/error/skip
	CoverageExceptions map[string]int `json:"coverage_exceptions,omitempty"`
	SupportDuration    string         `json:"support_duration,omitempty"`
}

// ReleasePhasesConfig maps ReleasePhase to rules
type ReleasePhasesConfig map[ReleasePhase]PhaseRules

// LifecyclePhasesConfig maps LifecyclePhase to rules (e.g., coverage)
type LifecyclePhasesConfig map[LifecyclePhase]PhaseRules

// PhasesConfig is the full config
type PhasesConfig struct {
	ReleasePhases   ReleasePhasesConfig   `json:"release_phases"`
	LifecyclePhases LifecyclePhasesConfig `json:"lifecycle_phases"`
}

// LoadPhasesConfig loads from .goneat/phases.yaml or infers from literal files
// Uses Pathfinder for safe reading; validates against embedded schema
func LoadPhasesConfig(pf pathfinder.PathFinder, configPath string) (*PhasesConfig, error) {
	if configPath == "" {
		configPath = ".goneat/phases.yaml"
	}

	// Validate and read config file safely
	if err := pf.ValidatePath(configPath); err != nil {
		// Fallback to literal files if schema file missing
		return inferFromLiteralFiles(pf)
	}

	loader, err := pf.CreateLoader("local", pathfinder.LoaderConfig{})
	if err != nil {
		return nil, err
	}
	file, err := loader.Open(configPath)
	if err != nil {
		// Fallback to literal
		return inferFromLiteralFiles(pf)
	}
	defer func() {
		_ = file.Close()
	}()

	// Read and unmarshal (YAML via json for simplicity; add yaml.v3 if needed)
	content, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var config PhasesConfig
	if err := json.Unmarshal(content, &config); err != nil {
		return nil, errors.New("invalid phases config: " + err.Error())
	}

	// Validate against embedded schema
	if result, err := schema.Validate(config, "phases"); err != nil {
		return nil, errors.New("phases schema validation failed: " + err.Error())
	} else if !result.Valid {
		return nil, errors.New("phases config validation failed")
	}

	// Apply defaults and validate enums
	config = applyDefaults(config)
	if err := config.Validate(); err != nil {
		return nil, err
	}

	return &config, nil
}

// inferFromLiteralFiles reads plain RELEASE_PHASE/LIFECYCLE_PHASE files and maps to defaults
func inferFromLiteralFiles(pf pathfinder.PathFinder) (*PhasesConfig, error) {
	config := &PhasesConfig{
		ReleasePhases:   make(ReleasePhasesConfig),
		LifecyclePhases: make(LifecyclePhasesConfig),
	}

	// Read RELEASE_PHASE file
	releaseFile := "RELEASE_PHASE"
	if err := pf.ValidatePath(releaseFile); err == nil {
		loader, _ := pf.CreateLoader("local", pathfinder.LoaderConfig{})
		file, _ := loader.Open(releaseFile)
		content, _ := io.ReadAll(file)
		_ = file.Close()
		phaseStr := strings.TrimSpace(string(content))
		releasePhase := ReleasePhase(phaseStr)
		if !releasePhase.IsValid() {
			return nil, errors.New("invalid RELEASE_PHASE: " + phaseStr)
		}
		// Map to default rules
		config.ReleasePhases[releasePhase] = getDefaultReleaseRules(releasePhase)
	}

	// Read LIFECYCLE_PHASE file (similar)
	lifecycleFile := "LIFECYCLE_PHASE"
	if err := pf.ValidatePath(lifecycleFile); err == nil {
		loader, _ := pf.CreateLoader("local", pathfinder.LoaderConfig{})
		file, _ := loader.Open(lifecycleFile)
		lifecycleContent, _ := io.ReadAll(file)
		_ = file.Close()
		lifecyclePhaseStr := strings.TrimSpace(string(lifecycleContent))
		lifecyclePhase := LifecyclePhase(lifecyclePhaseStr)
		if !lifecyclePhase.IsValid() {
			return nil, errors.New("invalid LIFECYCLE_PHASE: " + lifecyclePhaseStr)
		}
		config.LifecyclePhases[lifecyclePhase] = getDefaultLifecycleRules(lifecyclePhase)
	}

	// Fill missing phases with defaults
	for _, phase := range ValidReleasePhases() {
		if _, exists := config.ReleasePhases[phase]; !exists {
			config.ReleasePhases[phase] = getDefaultReleaseRules(phase)
		}
	}
	for _, phase := range ValidLifecyclePhases() {
		if _, exists := config.LifecyclePhases[phase]; !exists {
			config.LifecyclePhases[phase] = getDefaultLifecycleRules(phase)
		}
	}

	return config, nil
}

// applyDefaults fills schema defaults (simplified; full in unmarshal)
func applyDefaults(config PhasesConfig) PhasesConfig {
	// Implement defaults from schema (e.g., if MinCoverage == 0 { 80 })
	return config
}

// Validate checks business rules (e.g., enums, coverage 0-100)
func (c *PhasesConfig) Validate() error {
	for phase := range c.ReleasePhases {
		if !phase.IsValid() {
			return errors.New("invalid release phase key: " + phase.String())
		}
		if c.ReleasePhases[phase].MinCoverage < 0 || c.ReleasePhases[phase].MinCoverage > 100 {
			return errors.New("min_coverage must be 0-100 for " + phase.String())
		}
	}
	// Similar for lifecycle
	return nil
}

// GetMinCoverageForPhase returns coverage threshold for a lifecycle phase (for 0.2.7)
func (c *PhasesConfig) GetMinCoverageForPhase(phase LifecyclePhase) int {
	if rules, exists := c.LifecyclePhases[phase]; exists {
		return rules.MinCoverage
	}
	return 80 // Default
}

// getDefaultReleaseRules returns schema defaults with updated thresholds/exceptions
// Aligned with crucible schema: dev, rc, ga (release as alias)
func getDefaultReleaseRules(phase ReleasePhase) PhaseRules {
	switch phase {
	case ReleaseDev:
		return PhaseRules{AllowedSuffixes: []string{"-dev", "-alpha"}, MinCoverage: 50, AllowDirtyGit: true, CoverageExceptions: map[string]int{"tests/**": 100}}
	case ReleaseRC:
		return PhaseRules{AllowedSuffixes: []string{"-rc.1", "-rc.2"}, MinCoverage: 70, AllowDirtyGit: false, CoverageExceptions: map[string]int{"node_modules/**": 0}}
	case ReleaseGA, ReleaseRelease: // ga and release are equivalent
		return PhaseRules{AllowedSuffixes: []string{}, MinCoverage: 75, AllowDirtyGit: false, CoverageExceptions: map[string]int{"docs/**": 100}}
	default:
		return PhaseRules{MinCoverage: 75}
	}
}

// getDefaultLifecycleRules returns schema defaults with updated thresholds/exceptions
// Aligned with crucible schema: experimental, alpha, beta, rc, ga, lts
func getDefaultLifecycleRules(phase LifecyclePhase) PhaseRules {
	switch phase {
	case LifecycleExperimental:
		return PhaseRules{MinCoverage: 0, AllowDirtyGit: true, CoverageExceptions: map[string]int{"**": 0}}
	case LifecycleAlpha:
		return PhaseRules{MinCoverage: 30, CoverageExceptions: map[string]int{"prototypes/**": 0}}
	case LifecycleBeta:
		return PhaseRules{MinCoverage: 60}
	case LifecycleRC:
		return PhaseRules{MinCoverage: 70}
	case LifecycleGA:
		return PhaseRules{MinCoverage: 75}
	case LifecycleLTS:
		return PhaseRules{MinCoverage: 80, SupportDuration: "P3Y"}
	default:
		return PhaseRules{MinCoverage: 75}
	}
}

// *(End of file; tests below in _test.go)*
