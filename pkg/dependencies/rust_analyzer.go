/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package dependencies

import (
	"context"
	"fmt"
	"time"

	"github.com/fulmenhq/goneat/pkg/logger"
)

// RustAnalyzer implements Analyzer for Rust dependencies via cargo-deny.
type RustAnalyzer struct{}

// NewRustAnalyzer creates a new Rust dependency analyzer
func NewRustAnalyzer() Analyzer {
	return &RustAnalyzer{}
}

// Analyze implements Analyzer.Analyze for Rust.
// Uses cargo-deny to check license compliance and banned crates.
func (a *RustAnalyzer) Analyze(ctx context.Context, target string, cfg AnalysisConfig) (*AnalysisResult, error) {
	start := time.Now()

	// Check if this is a Rust project
	project := DetectRustProject(target)
	if project == nil {
		logger.Debug("No Rust project detected, skipping cargo-deny analysis")
		return &AnalysisResult{
			Dependencies: []Dependency{},
			Issues:       []Issue{},
			Passed:       true,
			Duration:     time.Since(start),
		}, nil
	}

	// Check if cargo is available
	if !IsCargoAvailable() {
		logger.Debug("cargo not available, skipping Rust dependency analysis")
		return &AnalysisResult{
			Dependencies: []Dependency{},
			Issues: []Issue{{
				Type:     "configuration",
				Severity: "info",
				Message:  "Rust project detected but cargo not available. Install Rust: https://rustup.rs/",
			}},
			Passed:   true,
			Duration: time.Since(start),
		}, nil
	}

	// Check if cargo-deny is available
	presence := CheckCargoDenyPresence()
	if !presence.Present {
		logger.Debug("cargo-deny not available, skipping Rust dependency analysis")
		return &AnalysisResult{
			Dependencies: []Dependency{},
			Issues: []Issue{{
				Type:     "configuration",
				Severity: "info",
				Message:  "Rust project detected but cargo-deny not installed.\n\nTo set up Rust dependency checking:\n  1. Install cargo-deny: cargo install cargo-deny\n  2. Initialize config:  cargo deny init\n  3. Learn more:         goneat docs show user-guide/rust/dependencies",
			}},
			Passed:   true,
			Duration: time.Since(start),
		}, nil
	}

	// Run cargo-deny for licenses and bans (5 minute default timeout)
	timeout := 5 * time.Minute

	result, err := RunCargoDeny(ctx, target, []CargoDenyCheckType{
		CargoDenyCheckLicenses,
		CargoDenyCheckBans,
	}, timeout)

	if err != nil {
		return nil, fmt.Errorf("cargo-deny analysis failed: %w", err)
	}

	if result == nil {
		return &AnalysisResult{
			Dependencies: []Dependency{},
			Issues:       []Issue{},
			Passed:       true,
			Duration:     time.Since(start),
		}, nil
	}

	// Convert findings to issues
	issues := make([]Issue, 0, len(result.Findings))
	for _, finding := range result.Findings {
		severity := mapFindingSeverityString(finding)
		issueType := "rust:cargo-deny"
		if finding.IsLicenseFinding() {
			issueType = "rust:cargo-deny:license"
		} else if finding.IsBanFinding() {
			issueType = "rust:cargo-deny:bans"
		}

		issues = append(issues, Issue{
			Type:     issueType,
			Severity: severity,
			Message:  finding.FormatMessage(),
		})
	}

	passed := true
	for _, issue := range issues {
		if issue.Severity == "high" || issue.Severity == "critical" {
			passed = false
			break
		}
	}

	logger.Debug(fmt.Sprintf("Rust dependency analysis found %d issues", len(issues)))

	return &AnalysisResult{
		Dependencies: []Dependency{}, // cargo-deny doesn't enumerate deps, just checks policies
		Issues:       issues,
		Passed:       passed,
		Duration:     result.Duration,
	}, nil
}

// mapFindingSeverityString maps cargo-deny finding severity to string severity.
// License violations are high severity (legal/supply chain risk).
// Bans are medium severity (policy enforcement).
// Informational codes (like "license-not-encountered") are low severity.
func mapFindingSeverityString(f CargoDenyFinding) string {
	// Informational codes are low severity (not actual violations)
	if IsInformationalCode(f.Code) {
		return "low"
	}

	// Actual license violations are high per spec
	if f.IsLicenseFinding() {
		return "high"
	}

	// Bans are medium per spec
	if f.IsBanFinding() {
		return "medium"
	}

	// For other types, map from cargo-deny severity
	switch f.SeverityLevel() {
	case 4:
		return "critical"
	case 3:
		return "high"
	case 2:
		return "medium"
	case 1:
		return "low"
	default:
		return "medium"
	}
}

// DetectLanguages implements Analyzer.DetectLanguages for Rust
func (a *RustAnalyzer) DetectLanguages(target string) ([]Language, error) {
	return []Language{LanguageRust}, nil
}
