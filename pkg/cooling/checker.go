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
	now    func() time.Time // clock; tests replace it
}

// NewChecker creates a new cooling checker
func NewChecker(cfg config.CoolingConfig) *Checker {
	return &Checker{config: cfg, now: time.Now}
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
	// ExceptionConfigViolation reports a matching exception whose configuration
	// is invalid (for example a malformed until). The exception is not applied.
	ExceptionConfigViolation ViolationType = "exception_config_invalid"
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

	// Check if exception applies. An invalid exception never applies and is
	// reported as a violation so the bad configuration cannot go unnoticed.
	exempt, exceptionErrs := c.isException(dep.Name)
	if exempt {
		return &CheckResult{
			Passed:      true,
			IsException: true,
		}, nil
	}

	var violations []Violation
	for _, excErr := range exceptionErrs {
		violations = append(violations, Violation{
			Type:     ExceptionConfigViolation,
			Severity: SeverityHigh,
			Message: fmt.Sprintf("Cooling exception for package %s is invalid and was not applied: %v",
				dep.Name, excErr),
		})
	}

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
	// An invalid exception is a configuration error, not a cooling finding:
	// it fails the gate regardless of alert-only or grace. Age and download
	// violations are still evaluated under the normal settings above.
	if len(exceptionErrs) > 0 {
		passed = false
	}

	result := &CheckResult{
		Passed:        passed,
		Violations:    violations,
		InGracePeriod: inGracePeriod,
	}

	return result, nil
}

// clock returns the current time; a zero-value Checker uses the system clock.
func (c *Checker) clock() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

// isException reports whether pkgName is covered by a valid exception.
// An exception whose until is malformed does not apply. Every matching
// exception is evaluated: if any of them has an unparseable until, none
// exempts the package (fail closed) and each bad one is returned so the caller
// can report it. An exception applies strictly before its until date (00:00 UTC); expired
// exceptions are skipped silently.
func (c *Checker) isException(pkgName string) (bool, []error) {
	var errs []error
	exempt := false
	for _, exc := range c.config.Exceptions {
		if !c.matchesPattern(pkgName, exc.Pattern) {
			continue
		}
		if exc.Until == "" {
			exempt = true // No expiry: permanent exception
			continue
		}
		until, err := ParseExceptionUntil(exc.Until)
		if err != nil {
			errs = append(errs, fmt.Errorf("pattern %q: %w", exc.Pattern, err))
			continue
		}
		// until is exclusive: the exception stops applying at 00:00:00 UTC on
		// that date.
		if c.clock().UTC().Before(until) {
			exempt = true
		}
	}
	if len(errs) > 0 {
		return false, errs
	}
	return exempt, nil
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
