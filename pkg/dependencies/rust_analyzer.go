/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package dependencies

import (
	"context"
	"fmt"
	"strings"
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

	collectionError := func(err error) {
		issues = append(issues, Issue{Type: "license_error", Severity: "critical", Message: err.Error()})
		passed = false
	}
	if checkLicenses && !cargoOK {
		collectionError(fmt.Errorf("rust license check requires cargo; install Rust: https://rustup.rs/"))
	} else if checkLicenses && !denyPresent {
		collectionError(fmt.Errorf("rust license check requires cargo-deny; install with cargo install cargo-deny"))
	}

	if checkCooling && !checkLicenses {
		dependencies = enumerateRustCratesForCooling(ctx, target, timeout)
	}

	if checkLicenses && cargoOK && denyPresent {
		licenseCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		snapshot, err := captureRustLicenseSnapshot(licenseCtx, project)
		if err != nil {
			collectionError(err)
		} else {
			defer snapshot.cleanup()
			listResult, listErr := runCargoDenyListWithSnapshot(licenseCtx, project.EffectiveRoot(), timeout, snapshot)
			if listErr != nil {
				collectionError(listErr)
			} else {
				// Preserve the license graph's source identities and exact denominator
				// when cooling is also requested. Cooling enriches this same set.
				dependencies = convertCratesToDependencies(listResult.Dependencies)
				for i := range dependencies {
					dependencies[i].Metadata["license_features"] = append([]string(nil), snapshot.Scope.Features...)
					dependencies[i].Metadata["license_all_features"] = snapshot.Scope.AllFeatures
					dependencies[i].Metadata["license_no_default_features"] = snapshot.Scope.NoDefaultFeatures
					if dependencies[i].License == nil {
						issues = append(issues, Issue{Type: "license", Severity: "high", Message: "Unresolved authoritative SPDX expression for " + listResult.Dependencies[i].PackageID, Dependency: &dependencies[i]})
						passed = false
					}
				}
			}
			result, checkErr := runCargoDenyWithSnapshot(licenseCtx, project, project.EffectiveRoot(), []CargoDenyCheckType{CargoDenyCheckLicenses, CargoDenyCheckBans}, timeout, snapshot)
			if checkErr != nil {
				collectionError(checkErr)
			} else if result != nil {
				if result.Failed {
					passed = false
					issues = append(issues, Issue{Type: "rust:cargo-deny", Severity: "high", Message: fmt.Sprintf("cargo-deny license/ban check exited %d", result.ExitCode)})
				}
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
		// Never reconstruct SPDX operators from flattened cargo-deny list IDs.
		var license *License
		if crate.Expression != "" {
			licenseType := crate.Expression
			license = &License{
				Name: licenseType,
				Type: licenseType,
			}
		}

		metadata := map[string]interface{}{"cargo_package_id": crate.PackageID, "license_scope": "all-workspace/all-targets/no-exclusions"}
		if source := crate.Source; source != "" {
			metadata["source"] = source
			metadata["is_local"] = strings.HasPrefix(source, "path+")
			if source == "registry+https://github.com/rust-lang/crates.io-index" || source == "registry+https://index.crates.io/" {
				metadata["registry"] = "crates.io"
			}
		}
		deps = append(deps, Dependency{
			Module: Module{
				Name:     crate.Name,
				Version:  crate.Version,
				Language: LanguageRust,
			},
			License:  license,
			Metadata: metadata,
		})
	}

	return deps
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
