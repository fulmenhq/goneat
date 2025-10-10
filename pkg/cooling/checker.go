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

	// Check if exception applies
	if c.isException(dep.Name) {
		return &CheckResult{
			Passed:      true,
			IsException: true,
		}, nil
	}

	var violations []Violation

	// Get age from metadata
	ageDays, ok := dep.Metadata["age_days"].(int)
	if !ok {
		// If no age data, assume it passes (conservative)
		return &CheckResult{Passed: true}, nil
	}

	// Age validation
	if ageDays < c.config.MinAgeDays {
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

	result := &CheckResult{
		Passed:     len(violations) == 0,
		Violations: violations,
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
