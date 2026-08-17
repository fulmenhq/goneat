package cooling

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/fulmenhq/goneat/pkg/config"
	"github.com/fulmenhq/goneat/pkg/dependencies/types"
)

// Checker validates packages against cooling policy
type Checker struct {
	config config.CoolingConfig
}

// NewChecker creates a new cooling checker
func NewChecker(cfg config.CoolingConfig) *Checker {
	return &Checker{config: cfg}
}

// Violation represents a cooling policy violation
type Violation struct {
	Type     ViolationType
	Severity Severity
	Message  string
	Actual   interface{}
	Expected interface{}
}

type ViolationType string

const (
	AgeViolation      ViolationType = "age_violation"
	DownloadViolation ViolationType = "download_violation"
)

type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
)

// CheckResult contains the result of cooling policy check
type CheckResult struct {
	Passed        bool
	Violations    []Violation
	IsException   bool
	InGracePeriod bool
}

// Check validates a dependency against cooling policy
func (c *Checker) Check(dep *types.Dependency) (*CheckResult, error) {
	// If cooling is disabled, pass all
	if !c.config.Enabled {
		return &CheckResult{Passed: true}, nil
	}

	// Skip local modules (no published version)
	if isLocal, ok := dep.Metadata["is_local"].(bool); ok && isLocal {
		return &CheckResult{Passed: true}, nil
	}

	// Check if exception applies
	if c.isException(dep.Name) {
		return &CheckResult{
			Passed:      true,
			IsException: true,
		}, nil
	}

	var violations []Violation

	// Age is required for cooling. Missing age_days is fail-closed (not a pass).
	// Go always stamps age_days (including a 365-day fallback on registry errors);
	// Rust leaves age_days unset when crates.io metadata is missing so this fires.
	ageDays, ok := ageDaysFromMetadata(dep.Metadata)
	if !ok {
		violations = append(violations, Violation{
			Type:     AgeViolation,
			Severity: SeverityHigh,
			Message: fmt.Sprintf("Package %s (%s) has unknown age; cooling requires age_days (missing metadata is not a pass)",
				dep.Name, dep.Version),
			Actual:   nil,
			Expected: c.config.MinAgeDays,
		})
	} else if ageDays < c.config.MinAgeDays {
		violations = append(violations, Violation{
			Type:     AgeViolation,
			Severity: SeverityHigh,
			Message: fmt.Sprintf("Package %s (%s) is only %d days old (minimum: %d days)",
				dep.Name, dep.Version, ageDays, c.config.MinAgeDays),
			Actual:   ageDays,
			Expected: c.config.MinAgeDays,
		})
	}

	// Download validation (if available)
	if totalDownloads, ok := dep.Metadata["total_downloads"].(int); ok {
		if totalDownloads < c.config.MinDownloads {
			violations = append(violations, Violation{
				Type:     DownloadViolation,
				Severity: SeverityMedium,
				Message: fmt.Sprintf("Package %s has %d total downloads (minimum: %d)",
					dep.Name, totalDownloads, c.config.MinDownloads),
				Actual:   totalDownloads,
				Expected: c.config.MinDownloads,
			})
		}
	}

	if recentDownloads, ok := dep.Metadata["recent_downloads"].(int); ok {
		if recentDownloads < c.config.MinDownloadsRecent {
			violations = append(violations, Violation{
				Type:     DownloadViolation,
				Severity: SeverityMedium,
				Message: fmt.Sprintf("Package %s has %d recent downloads (minimum: %d)",
					dep.Name, recentDownloads, c.config.MinDownloadsRecent),
				Actual:   recentDownloads,
				Expected: c.config.MinDownloadsRecent,
			})
		}
	}

	// Grace is slack against min_age_days: in grace only when
	// age + grace >= min_age (near the threshold). It is NOT
	// publish + min_age + grace (that made every young package pass).
	// A 2-day crate with min_age=7 and grace=3 still fails (2+3 < 7).
	inGracePeriod := false
	if c.config.GracePeriodDays > 0 && ok && ageDays < c.config.MinAgeDays {
		inGracePeriod = ageDays+c.config.GracePeriodDays >= c.config.MinAgeDays
	}

	// Determine if check passes
	passed := len(violations) == 0
	if !passed && (c.config.AlertOnly || inGracePeriod) {
		// Alert-only mode or grace period - don't fail the check
		passed = true
	}

	result := &CheckResult{
		Passed:        passed,
		Violations:    violations,
		InGracePeriod: inGracePeriod,
	}

	return result, nil
}

// isException checks if package matches exception patterns
func (c *Checker) isException(pkgName string) bool {
	for _, exc := range c.config.Exceptions {
		if c.matchesPattern(pkgName, exc.Pattern) {
			// Check if time-limited exception is still valid
			if exc.Until != "" {
				until, err := time.Parse("2006-01-02", exc.Until)
				if err == nil && time.Now().After(until) {
					continue // Exception expired
				}
			}
			return true
		}
	}
	return false
}

// matchesPattern implements glob-style pattern matching
func (c *Checker) matchesPattern(pkgName, pattern string) bool {
	// Support wildcards: * and ?
	matched, err := filepath.Match(pattern, pkgName)
	if err != nil {
		return false
	}
	if matched {
		return true
	}

	// Also support prefix matching for @org/* and github.com/org/* style patterns
	if strings.Contains(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		if strings.HasPrefix(pkgName, prefix+"/") {
			return true
		}
	}

	return false
}

// ageDaysFromMetadata reads age_days from dependency metadata.
// Accepts int/int64/float64 so JSON/YAML round-trips still count as present.
func ageDaysFromMetadata(meta map[string]interface{}) (int, bool) {
	if meta == nil {
		return 0, false
	}
	switch v := meta["age_days"].(type) {
	case int:
		return v, true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case float32:
		return int(v), true
	default:
		return 0, false
	}
}
