/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package dependencies

import (
	"context"
	"fmt"
	"time"

	"github.com/fulmenhq/goneat/pkg/logger"
	"github.com/fulmenhq/goneat/pkg/registry"
)

// RustAnalyzer implements Analyzer for Rust dependencies via cargo-deny
// (licenses/bans) and crates.io cooling.
type RustAnalyzer struct {
	cratesClient registry.Client
}

// NewRustAnalyzer creates a new Rust dependency analyzer
func NewRustAnalyzer() Analyzer {
	return &RustAnalyzer{}
}

// NewRustAnalyzerWithClient creates a Rust analyzer with an injectable
// crates.io client (tests).
func NewRustAnalyzerWithClient(client registry.Client) *RustAnalyzer {
	return &RustAnalyzer{cratesClient: client}
}

func (a *RustAnalyzer) client() registry.Client {
	if a != nil && a.cratesClient != nil {
		return a.cratesClient
	}
	return registry.NewCratesClient(24 * time.Hour)
}

// Analyze implements Analyzer.Analyze for Rust.
// License/ban policy stays on cargo-deny. Cooling enumerates via pkg/cargo
// (Cargo.lock or cargo metadata JSON) — not cargo-deny — then crates.io
// metadata + cooling.Checker.
func (a *RustAnalyzer) Analyze(ctx context.Context, target string, cfg AnalysisConfig) (*AnalysisResult, error) {
	start := time.Now()

	checkLicenses := cfg.CheckLicenses
	checkCooling := cfg.CheckCooling
	if !checkLicenses && !checkCooling {
		checkLicenses = true
		checkCooling = true
	}

	project := DetectRustProject(target)
	if project == nil {
		logger.Debug("No Rust project detected, skipping Rust dependency analysis")
		return &AnalysisResult{
			Dependencies: []Dependency{},
			Issues:       []Issue{},
			Passed:       true,
			Duration:     time.Since(start),
		}, nil
	}

	timeout := 5 * time.Minute
	issues := make([]Issue, 0)
	var dependencies []Dependency
	passed := true

	cargoOK := IsCargoAvailable()
	denyPresent := CheckCargoDenyPresence().Present

	if checkLicenses && !cargoOK {
		logger.Debug("cargo not available, skipping cargo-deny license analysis")
		issues = append(issues, Issue{
			Type:     "configuration",
			Severity: "info",
			Message:  "Rust project detected but cargo not available. Install Rust: https://rustup.rs/",
		})
	} else if checkLicenses && !denyPresent {
		logger.Debug("cargo-deny not available, skipping cargo-deny license analysis")
		issues = append(issues, Issue{
			Type:     "configuration",
			Severity: "info",
			Message:  "Rust project detected but cargo-deny not installed.\n\nTo set up Rust dependency checking:\n  1. Install cargo-deny: cargo install cargo-deny\n  2. Initialize config:  cargo deny init\n  3. Learn more:         goneat docs show user-guide/rust/dependencies",
		})
	}

	if checkCooling {
		dependencies = enumerateRustCratesForCooling(ctx, target, timeout)
	}

	if checkLicenses && cargoOK && denyPresent {
		listResult, listErr := RunCargoDenyList(ctx, target, timeout)
		if listErr != nil {
			logger.Warn(fmt.Sprintf("cargo deny list failed: %v", listErr))
		} else if listResult != nil {
			licensed := convertCratesToDependencies(listResult.Dependencies)
			if checkCooling && len(dependencies) > 0 {
				overlayLicenses(dependencies, licensed)
			} else if !checkCooling {
				dependencies = licensed
			}
		}

		result, err := RunCargoDeny(ctx, target, []CargoDenyCheckType{
			CargoDenyCheckLicenses,
			CargoDenyCheckBans,
		}, timeout)
		if err != nil {
			return nil, fmt.Errorf("cargo-deny analysis failed: %w", err)
		}
		if result != nil {
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
		}
	}

	if checkCooling {
		if len(dependencies) == 0 {
			issues = append(issues, Issue{
				Type:     "age_violation",
				Severity: "high",
				Message:  "Rust cooling requested but no crates could be enumerated (need Cargo.lock or cargo metadata JSON)",
			})
			passed = false
		} else {
			attachCratesIOMetadata(dependencies, a.client())
			coolIssues, coolPassed := applyRustCooling(dependencies, cfg.PolicyPath)
			issues = append(issues, coolIssues...)
			if !coolPassed {
				passed = false
			}
		}
	}

	// cargo-deny license/ban findings fail the run; cooling pass/fail is
	// already decided by applyRustCooling (grace/alert_only may list
	// high issues without failing the gate).
	for _, issue := range issues {
		if (issue.Type == "rust:cargo-deny:license" || issue.Type == "rust:cargo-deny") &&
			(issue.Severity == "high" || issue.Severity == "critical") {
			passed = false
			break
		}
	}

	if dependencies == nil {
		dependencies = []Dependency{}
	}

	logger.Debug(fmt.Sprintf("Rust dependency analysis found %d issues", len(issues)))

	return &AnalysisResult{
		Dependencies: dependencies,
		Issues:       issues,
		Passed:       passed,
		Duration:     time.Since(start),
	}, nil
}

// convertCratesToDependencies converts cargo deny list results to the unified Dependency format.
// This enables `goneat dependencies --licenses` to work for Rust projects with the same
// structured output as Go projects.
func convertCratesToDependencies(crates []CargoCrateLicense) []Dependency {
	deps := make([]Dependency, 0, len(crates))

	for _, crate := range crates {
		// Create license info - join multiple licenses with " OR " for SPDX-like expression
		var license *License
		if len(crate.Licenses) > 0 {
			// Use the first license as the primary, but store all in the Type field
			licenseType := crate.Licenses[0]
			if len(crate.Licenses) > 1 {
				licenseType = joinLicenses(crate.Licenses)
			}
			license = &License{
				Name: licenseType,
				Type: licenseType,
			}
		}

		deps = append(deps, Dependency{
			Module: Module{
				Name:     crate.Name,
				Version:  crate.Version,
				Language: LanguageRust,
			},
			License: license,
		})
	}

	return deps
}

// joinLicenses joins multiple licenses into an SPDX-like expression.
// Example: ["MIT", "Apache-2.0"] -> "MIT OR Apache-2.0"
func joinLicenses(licenses []string) string {
	if len(licenses) == 0 {
		return ""
	}
	if len(licenses) == 1 {
		return licenses[0]
	}
	return stringJoin(licenses, " OR ")
}

// stringJoin joins strings with a separator (avoiding import of strings just for Join)
func stringJoin(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += sep + parts[i]
	}
	return result
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
