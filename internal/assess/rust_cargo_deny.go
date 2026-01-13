package assess

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/fulmenhq/goneat/pkg/dependencies"
	"github.com/fulmenhq/goneat/pkg/logger"
)

const cargoDenyMinVersion = "0.14.0"

type cargoDenyAdapter struct {
	runner     *SecurityAssessmentRunner
	moduleRoot string
	cfg        AssessmentConfig
}

func (c *cargoDenyAdapter) Name() string { return "cargo-deny" }

func (c *cargoDenyAdapter) IsAvailable() bool {
	if !IsCargoAvailable() {
		return false
	}
	project := DetectRustProject(c.moduleRoot)
	if project == nil || project.CargoTomlPath == "" {
		return false
	}
	presence := CheckRustToolPresence("cargo-deny", cargoDenyMinVersion)
	if presence.Present && !presence.MeetsMin && presence.Version != "" {
		logger.Warn(fmt.Sprintf("cargo-deny %s below minimum %s; results may be unreliable", presence.Version, cargoDenyMinVersion))
	}
	return presence.Present
}

// Run executes cargo-deny for security checks (advisories, sources).
// NOTE: cargo-deny outputs JSON to STDERR (not stdout) when using --format json.
// This is intentional per cargo-deny design - see pkg/dependencies/cargo_deny.go for details.
// The --format json flag must come BEFORE the check subcommand.
func (c *cargoDenyAdapter) Run(ctx context.Context) ([]Issue, error) {
	// Use the canonical cargo-deny implementation from pkg/dependencies
	// which correctly handles STDERR output and NDJSON parsing
	result, err := dependencies.RunCargoDeny(ctx, c.moduleRoot, []dependencies.CargoDenyCheckType{
		dependencies.CargoDenyCheckAdvisories,
		dependencies.CargoDenyCheckSources,
	}, c.cfg.Timeout)

	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}

	issues := make([]Issue, 0, len(result.Findings))
	for _, finding := range result.Findings {
		issues = append(issues, Issue{
			File:        filepath.ToSlash(result.ReportFile),
			Severity:    mapCargoDenyFindingSeverity(finding),
			Message:     finding.FormatMessage(),
			Category:    CategorySecurity,
			SubCategory: "rust:cargo-deny",
			AutoFixable: false,
		})
	}

	return issues, nil
}

// mapCargoDenyFindingSeverity maps cargo-deny finding to security severity
func mapCargoDenyFindingSeverity(f dependencies.CargoDenyFinding) IssueSeverity {
	switch f.SeverityLevel() {
	case 4:
		return SeverityCritical
	case 3:
		return SeverityHigh
	case 2:
		return SeverityMedium
	case 1:
		return SeverityLow
	default:
		return SeverityMedium
	}
}

func init() {
	RegisterSecurityTool("cargo-deny", "vuln", func(r *SecurityAssessmentRunner, moduleRoot string, cfg AssessmentConfig) SecurityTool {
		return &cargoDenyAdapter{runner: r, moduleRoot: moduleRoot, cfg: cfg}
	})
}

// RunCargoDenyDependencyChecks runs cargo-deny license and bans checks.
// This is called from the dependencies assessment category (not security).
// Returns issues with Category=CategoryDependencies and SubCategory=rust:cargo-deny.
//
// NOTE: cargo-deny outputs JSON to STDERR (not stdout) when using --format json.
// This is intentional per cargo-deny design. The --format json flag must come
// BEFORE the check subcommand. This was discovered during fulmen-toolbox testing
// and is documented here for maintainability.
// See: pkg/dependencies/cargo_deny.go for the canonical implementation.
func RunCargoDenyDependencyChecks(target string, timeout time.Duration) ([]Issue, error) {
	if !IsCargoAvailable() {
		return nil, nil
	}

	project := DetectRustProject(target)
	if project == nil || project.CargoTomlPath == "" {
		return nil, nil
	}

	presence := CheckRustToolPresence("cargo-deny", cargoDenyMinVersion)
	if !presence.Present {
		logger.Debug("cargo-deny not available, skipping Rust dependency checks")
		return nil, nil
	}
	if !presence.MeetsMin && presence.Version != "" {
		logger.Warn(fmt.Sprintf("cargo-deny %s below minimum %s; results may be unreliable", presence.Version, cargoDenyMinVersion))
	}

	// Use the canonical cargo-deny implementation from pkg/dependencies
	// which correctly handles:
	// 1. Command order: --format json BEFORE check subcommand
	// 2. STDERR output: cargo-deny outputs JSON to stderr, not stdout
	// 3. NDJSON parsing: diagnostic entries with type: "diagnostic" and fields
	// 4. Rich labels: file:line refs, license names, version context
	ctx := context.Background()
	result, err := dependencies.RunCargoDeny(ctx, target, []dependencies.CargoDenyCheckType{
		dependencies.CargoDenyCheckLicenses,
		dependencies.CargoDenyCheckBans,
	}, timeout)

	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}

	issues := make([]Issue, 0, len(result.Findings))
	for _, finding := range result.Findings {
		// Map finding type to subcategory
		subCategory := "rust:cargo-deny"
		if finding.IsLicenseFinding() {
			subCategory = "rust:cargo-deny:license"
		} else if finding.IsBanFinding() {
			subCategory = "rust:cargo-deny:bans"
		}

		issues = append(issues, Issue{
			File:          filepath.ToSlash(result.ReportFile),
			Severity:      mapCargoDenyDependencySeverity(finding),
			Message:       finding.FormatMessage(),
			Category:      CategoryDependencies,
			SubCategory:   subCategory,
			AutoFixable:   false,
			EstimatedTime: HumanReadableDuration(30 * time.Minute), // License/ban issues require manual review
		})
	}

	logger.Debug(fmt.Sprintf("cargo-deny dependency checks found %d issues", len(issues)))
	return issues, nil
}

// mapCargoDenyDependencySeverity maps cargo-deny severity for dependency issues.
// Per acceptance criteria:
// - License violations: high severity (supply chain/legal risk)
// - Bans violations: medium severity (policy enforcement)
// - Informational codes (e.g., "license-not-encountered"): low severity
func mapCargoDenyDependencySeverity(f dependencies.CargoDenyFinding) IssueSeverity {
	// Informational codes are low severity (not actual violations)
	// e.g., "license-not-encountered" means an allowed license wasn't used
	if dependencies.IsInformationalCode(f.Code) {
		return SeverityLow
	}

	// Bans are always medium per spec, regardless of cargo-deny's severity
	if f.IsBanFinding() {
		return SeverityMedium
	}

	// License violations are always high per spec
	if f.IsLicenseFinding() {
		return SeverityHigh
	}

	// For other types (shouldn't happen in dependencies context), use cargo-deny's severity
	switch f.SeverityLevel() {
	case 4:
		return SeverityCritical
	case 3:
		return SeverityHigh
	case 2:
		return SeverityMedium
	case 1:
		return SeverityLow
	default:
		return SeverityMedium
	}
}
