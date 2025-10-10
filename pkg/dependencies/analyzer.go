package dependencies

import (
	"context"
	"time"

	"github.com/fulmenhq/goneat/pkg/config"
)

// Language represents a programming language
type Language string

const (
	LanguageGo         Language = "go"
	LanguageTypeScript Language = "typescript"
	LanguagePython     Language = "python"
	LanguageRust       Language = "rust"
	LanguageCSharp     Language = "csharp"
)

// Module represents a dependency module
type Module struct {
	Name     string
	Version  string
	Language Language
}

// License represents a software license
type License struct {
	Name string
	URL  string
	Type string // e.g., MIT, Apache-2.0
}

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
	Config     *config.DependenciesConfig // Thread config for overrides
}

// AnalysisResult holds the result of analysis
type AnalysisResult struct {
	Dependencies []Dependency
	Issues       []Issue
	Passed       bool
	Duration     time.Duration
}

// Dependency represents an analyzed dependency
type Dependency struct {
	Module
	License  *License
	Metadata map[string]interface{}
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
