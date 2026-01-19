package dependencies

import (
	"context"
	"time"

	"github.com/fulmenhq/goneat/pkg/config"
	"github.com/fulmenhq/goneat/pkg/dependencies/types"
)

// Re-export types for backward compatibility
type Language = types.Language
type Module = types.Module
type License = types.License
type Dependency = types.Dependency

const (
	LanguageGo         = types.LanguageGo
	LanguageTypeScript = types.LanguageTypeScript
	LanguagePython     = types.LanguagePython
	LanguageRust       = types.LanguageRust
	LanguageCSharp     = types.LanguageCSharp
)

// Policy represents a policy for dependency analysis
type Policy struct {
	ID          string
	Name        string
	Description string
	Rules       []Rule
}

// Rule represents a policy rule
type Rule struct {
	ID         string
	Type       string // license, cooling
	Conditions []Condition
	Action     string // allow, deny, warn
}

// Condition represents a rule condition
type Condition struct {
	Field    string
	Operator string
	Value    interface{}
}

// AnalysisConfig holds configuration for analysis
type AnalysisConfig struct {
	PolicyPath string
	EngineType string
	Languages  []Language
	Target     string

	// CheckLicenses controls whether license detection/policy is evaluated.
	// If both CheckLicenses and CheckCooling are false, analyzers may default to
	// legacy behavior (both enabled).
	CheckLicenses bool
	CheckCooling  bool

	Config *config.DependenciesConfig // Thread config for overrides
}

// AnalysisResult holds the result of analysis
type AnalysisResult struct {
	Dependencies    []Dependency
	Issues          []Issue
	Passed          bool
	Duration        time.Duration
	PackagesScanned int // Number of packages scanned (from SBOM, used for vuln scan)
}

// Issue represents an analysis issue
type Issue struct {
	Type       string
	Severity   string
	Message    string
	Dependency *Dependency
}

// PolicyResult represents policy evaluation result
type PolicyResult struct {
	Passed bool
	Reason string
}

// Analyzer defines the dependency analyzer interface
type Analyzer interface {
	Analyze(ctx context.Context, target string, config AnalysisConfig) (*AnalysisResult, error)
	DetectLanguages(target string) ([]Language, error)
}

// LanguageDetector defines language detection interface
type LanguageDetector interface {
	Detect(target string) (Language, bool, error)
	GetManifestFiles(target string) ([]string, error)
}

// LicenseDetector defines license detection interface
type LicenseDetector interface {
	DetectLicense(module Module) (*License, error)
	ValidateAgainstPolicy(license *License, policy Policy) (PolicyResult, error)
}
