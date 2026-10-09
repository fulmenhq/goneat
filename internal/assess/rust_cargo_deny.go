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

func (c *cargoDenyAdapter) IsApplicable() bool {
	project := DetectRustProject(c.moduleRoot)
	return project != nil && project.CargoTomlPath != ""
}

func (c *cargoDenyAdapter) IsAvailable() bool {
	if !IsCargoAvailable() {
		return false
	}
	if !c.IsApplicable() {
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
	issues, _, err := c.RunWithMetadata(ctx)
	return issues, err
}

func (c *cargoDenyAdapter) RunWithMetadata(ctx context.Context) ([]Issue, map[string]interface{}, error) {
	// Use the canonical cargo-deny implementation from pkg/dependencies
	// which correctly handles STDERR output and NDJSON parsing
	result, err := dependencies.RunCargoDenySecurity(ctx, c.moduleRoot, c.cfg.Timeout)
	if result == nil {
		if err != nil {
			return nil, nil, err
		}
		return nil, nil, fmt.Errorf("cargo-deny did not return a security report")
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

	// Completion counters are evidence, not invented advisories or a new
	// severity mapping. Keep unknown summary members at their original precision.
	metadata := map[string]interface{}{
		"complete": result.Complete, "exit_code": result.ExitCode, "version": result.Version,
		"summary": result.Summary, "policy_findings_reported": result.Complete && result.ExitCode != 0,
	}
	return issues, metadata, err
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
	project := DetectRustProject(target)
	if project == nil || project.CargoTomlPath == "" {
		return nil, nil
	}
	ctx := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	// Standalone callers use the same inventory and result contract as the
	// dependencies command. The assessment runner does not call this twice.
	result, err := dependencies.NewRustAnalyzer().Analyze(ctx, target, dependencies.AnalysisConfig{CheckLicenses: true})
	if err != nil {
		return nil, err
	}
	issues := NewDependenciesRunner().convertToAssessmentIssues(result)
	for i := range issues {
		issues[i].File = filepath.ToSlash(project.CargoTomlPath)
	}
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
