package cooling

import (
	"testing"
	"time"

	"github.com/fulmenhq/goneat/pkg/config"
	"github.com/fulmenhq/goneat/pkg/dependencies/types"
)

func TestChecker_Check_AgeViolation(t *testing.T) {
	cfg := config.CoolingConfig{
		Enabled:    true,
		MinAgeDays: 7,
	}
	checker := NewChecker(cfg)

	dep := &types.Dependency{
		Module: types.Module{
			Name:    "new-package",
			Version: "1.0.0",
		},
		Metadata: map[string]interface{}{
			"age_days": 3, // Too new
		},
	}

	result, err := checker.Check(dep)
	if err != nil {
		t.Fatal(err)
	}

	if result.Passed {
		t.Error("Should fail for package < 7 days old")
	}

	if len(result.Violations) != 1 {
		t.Errorf("Expected 1 violation, got %d", len(result.Violations))
	}

	if result.Violations[0].Type != AgeViolation {
		t.Errorf("Expected age violation, got %s", result.Violations[0].Type)
	}
}

func TestChecker_Check_PassesWhenOldEnough(t *testing.T) {
	cfg := config.CoolingConfig{
		Enabled:    true,
		MinAgeDays: 7,
	}
	checker := NewChecker(cfg)

	dep := &types.Dependency{
		Module: types.Module{
			Name:    "old-package",
			Version: "1.0.0",
		},
		Metadata: map[string]interface{}{
			"age_days": 30, // Old enough
		},
	}

	result, err := checker.Check(dep)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Passed {
		t.Error("Should pass for package >= 7 days old")
	}

	if len(result.Violations) != 0 {
		t.Errorf("Expected 0 violations, got %d", len(result.Violations))
	}
}

func TestChecker_Check_Exception(t *testing.T) {
	cfg := config.CoolingConfig{
		Enabled:    true,
		MinAgeDays: 7,
		Exceptions: []config.CoolingException{
			{
				Pattern: "@myorg/*",
				Reason:  "Internal packages",
			},
		},
	}
	checker := NewChecker(cfg)

	dep := &types.Dependency{
		Module: types.Module{
			Name:    "@myorg/new-package",
			Version: "1.0.0",
		},
		Metadata: map[string]interface{}{
			"age_days": 1, // Would normally fail
		},
	}

	result, err := checker.Check(dep)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Passed {
		t.Error("Should pass for exception pattern")
	}

	if !result.IsException {
		t.Error("Should mark as exception")
	}
}

func TestChecker_Check_GithubException(t *testing.T) {
	cfg := config.CoolingConfig{
		Enabled:    true,
		MinAgeDays: 7,
		Exceptions: []config.CoolingException{
			{
				Pattern: "github.com/spf13/*",
				Reason:  "Trusted maintainer",
			},
		},
	}
	checker := NewChecker(cfg)

	dep := &types.Dependency{
		Module: types.Module{
			Name:    "github.com/spf13/cobra",
			Version: "v1.8.0",
		},
		Metadata: map[string]interface{}{
			"age_days": 1,
		},
	}

	result, err := checker.Check(dep)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Passed {
		t.Error("Should pass for github.com exception pattern")
	}

	if !result.IsException {
		t.Error("Should mark as exception")
	}
}

func TestChecker_Check_TimeLimitedException_Expired(t *testing.T) {
	cfg := config.CoolingConfig{
		Enabled:    true,
		MinAgeDays: 7,
		Exceptions: []config.CoolingException{
			{
				Pattern: "temp-package",
				Until:   "2020-01-01", // Expired
			},
		},
	}
	checker := NewChecker(cfg)

	dep := &types.Dependency{
		Module: types.Module{
			Name:    "temp-package",
			Version: "1.0.0",
		},
		Metadata: map[string]interface{}{
			"age_days": 1,
		},
	}

	result, err := checker.Check(dep)
	if err != nil {
		t.Fatal(err)
	}

	if result.Passed {
		t.Error("Should fail when exception expired")
	}
}

func TestChecker_Check_TimeLimitedException_Valid(t *testing.T) {
	cfg := config.CoolingConfig{
		Enabled:    true,
		MinAgeDays: 7,
		Exceptions: []config.CoolingException{
			{
				Pattern: "temp-package",
				Until:   "2030-01-01", // Future date
			},
		},
	}
	checker := NewChecker(cfg)

	dep := &types.Dependency{
		Module: types.Module{
			Name:    "temp-package",
			Version: "1.0.0",
		},
		Metadata: map[string]interface{}{
			"age_days": 1,
		},
	}

	result, err := checker.Check(dep)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Passed {
		t.Error("Should pass when exception is valid")
	}

	if !result.IsException {
		t.Error("Should mark as exception")
	}
}

func TestChecker_Check_DownloadViolation(t *testing.T) {
	cfg := config.CoolingConfig{
		Enabled:            true,
		MinAgeDays:         7,
		MinDownloads:       100,
		MinDownloadsRecent: 10,
	}
	checker := NewChecker(cfg)

	dep := &types.Dependency{
		Module: types.Module{
			Name:    "unpopular-package",
			Version: "1.0.0",
		},
		Metadata: map[string]interface{}{
			"age_days":         30, // Old enough
			"total_downloads":  50, // Too few
			"recent_downloads": 5,  // Too few
		},
	}

	result, err := checker.Check(dep)
	if err != nil {
		t.Fatal(err)
	}

	if result.Passed {
		t.Error("Should fail for insufficient downloads")
	}

	// Should have 2 download violations
	if len(result.Violations) != 2 {
		t.Errorf("Expected 2 violations, got %d", len(result.Violations))
	}

	for _, v := range result.Violations {
		if v.Type != DownloadViolation {
			t.Errorf("Expected download violation, got %s", v.Type)
		}
	}
}

func TestChecker_Check_DisabledCooling(t *testing.T) {
	cfg := config.CoolingConfig{
		Enabled:    false,
		MinAgeDays: 7,
	}
	checker := NewChecker(cfg)

	dep := &types.Dependency{
		Module: types.Module{
			Name:    "new-package",
			Version: "1.0.0",
		},
		Metadata: map[string]interface{}{
			"age_days": 1, // Would fail if enabled
		},
	}

	result, err := checker.Check(dep)
	if err != nil {
		t.Fatal(err)
	}

	if !result.Passed {
		t.Error("Should pass when cooling is disabled")
	}
}

func TestChecker_Check_NoMetadata(t *testing.T) {
	cfg := config.CoolingConfig{
		Enabled:    true,
		MinAgeDays: 7,
	}
	checker := NewChecker(cfg)

	dep := &types.Dependency{
		Module: types.Module{
			Name:    "package-without-metadata",
			Version: "1.0.0",
		},
		Metadata: map[string]interface{}{
			// No age_days
		},
	}

	result, err := checker.Check(dep)
	if err != nil {
		t.Fatal(err)
	}

	if result.Passed {
		t.Error("Should fail when age_days is missing (fail-closed)")
	}
	if len(result.Violations) != 1 {
		t.Fatalf("Expected 1 violation, got %d", len(result.Violations))
	}
	if result.Violations[0].Type != AgeViolation {
		t.Errorf("Expected age violation for missing age_days, got %s", result.Violations[0].Type)
	}
}

func TestChecker_Check_GithubPatternDoesNotMatchCrateName(t *testing.T) {
	cfg := config.CoolingConfig{
		Enabled:    true,
		MinAgeDays: 7,
		Exceptions: []config.CoolingException{
			{Pattern: "github.com/3leaps/*", Reason: "Go modules only"},
		},
	}
	checker := NewChecker(cfg)

	dep := &types.Dependency{
		Module: types.Module{
			Name:    "young-crate",
			Version: "0.1.0",
		},
		Metadata: map[string]interface{}{
			"age_days": 1,
		},
	}

	result, err := checker.Check(dep)
	if err != nil {
		t.Fatal(err)
	}
	if result.Passed || result.IsException {
		t.Error("github.com/3leaps/* must not silently pass a crates.io crate name")
	}
}

func TestChecker_Check_GraceDoesNotPassFarYoungPackage(t *testing.T) {
	cfg := config.CoolingConfig{
		Enabled:         true,
		MinAgeDays:      7,
		GracePeriodDays: 3,
	}
	checker := NewChecker(cfg)

	dep := &types.Dependency{
		Module: types.Module{Name: "uuid", Version: "1.24.1"},
		Metadata: map[string]interface{}{
			"age_days": 2,
		},
	}

	result, err := checker.Check(dep)
	if err != nil {
		t.Fatal(err)
	}
	if result.InGracePeriod {
		t.Error("age 2 + grace 3 < min_age 7 must not be in grace")
	}
	if result.Passed {
		t.Error("2-day package must fail min_age_days=7 even with grace_period_days=3")
	}
	if len(result.Violations) == 0 {
		t.Error("expected age_violation")
	}
}

func TestChecker_Check_GraceNearThreshold(t *testing.T) {
	cfg := config.CoolingConfig{
		Enabled:         true,
		MinAgeDays:      7,
		GracePeriodDays: 3,
	}
	checker := NewChecker(cfg)

	dep := &types.Dependency{
		Module: types.Module{Name: "almost-aged", Version: "1.0.0"},
		Metadata: map[string]interface{}{
			"age_days": 5, // 5+3 >= 7
		},
	}

	result, err := checker.Check(dep)
	if err != nil {
		t.Fatal(err)
	}
	if !result.InGracePeriod {
		t.Error("age 5 + grace 3 >= min_age 7 should be in grace")
	}
	if !result.Passed {
		t.Error("near-threshold grace should not fail the gate")
	}
	if len(result.Violations) == 0 {
		t.Error("grace must still record the age_violation (not drop it)")
	}
}

func TestChecker_Check_FiveDaysNoGraceFails(t *testing.T) {
	cfg := config.CoolingConfig{
		Enabled:         true,
		MinAgeDays:      7,
		GracePeriodDays: 0,
	}
	checker := NewChecker(cfg)

	dep := &types.Dependency{
		Module: types.Module{Name: "num-integer", Version: "0.1.47"},
		Metadata: map[string]interface{}{
			"age_days": 5,
		},
	}

	result, err := checker.Check(dep)
	if err != nil {
		t.Fatal(err)
	}
	if result.Passed || result.InGracePeriod {
		t.Error("5-day crate with no grace must age_violation-fail")
	}
}

func TestChecker_Check_CrateNamePrefixException(t *testing.T) {
	cfg := config.CoolingConfig{
		Enabled:    true,
		MinAgeDays: 7,
		Exceptions: []config.CoolingException{
			{Pattern: "3leaps-*", Reason: "Estate crate prefix"},
		},
	}
	checker := NewChecker(cfg)

	dep := &types.Dependency{
		Module: types.Module{
			Name:    "3leaps-internal",
			Version: "0.1.0",
		},
		Metadata: map[string]interface{}{
			"age_days": 1,
		},
	}

	result, err := checker.Check(dep)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Passed || !result.IsException {
		t.Error("3leaps-* should except estate crate names")
	}
}

func TestChecker_matchesPattern_Wildcard(t *testing.T) {
	checker := NewChecker(config.CoolingConfig{})

	tests := []struct {
		pkgName  string
		pattern  string
		expected bool
	}{
		{"@myorg/package", "@myorg/*", true},
		{"@myorg/sub/package", "@myorg/*", true}, // Prefix matching handles nested paths
		{"github.com/spf13/cobra", "github.com/spf13/*", true},
		{"github.com/spf13/viper", "github.com/spf13/*", true},
		{"github.com/other/package", "github.com/spf13/*", false},
		{"exact-match", "exact-match", true},
		{"not-match", "exact-match", false},
		{"3leaps-sysprims", "3leaps-*", true},
		{"lanyte-core", "lanyte-*", true},
		{"fulmen-cli", "fulmen-*", true},
		{"birchton-util", "birchton-*", true},
		{"sysprims", "sysprims", true},
		{"serde", "github.com/3leaps/*", false},
		{"3leaps-foo", "github.com/3leaps/*", false},
		{"serde", "github.com/fulmenhq/*", false},
	}

	for _, tt := range tests {
		t.Run(tt.pkgName+"_"+tt.pattern, func(t *testing.T) {
			result := checker.matchesPattern(tt.pkgName, tt.pattern)
			if result != tt.expected {
				t.Errorf("matchesPattern(%q, %q) = %v, want %v",
					tt.pkgName, tt.pattern, result, tt.expected)
			}
		})
	}
}

func TestChecker_Check_CombinedViolations(t *testing.T) {
	cfg := config.CoolingConfig{
		Enabled:            true,
		MinAgeDays:         30,
		MinDownloads:       1000,
		MinDownloadsRecent: 100,
	}
	checker := NewChecker(cfg)

	dep := &types.Dependency{
		Module: types.Module{
			Name:    "problematic-package",
			Version: "1.0.0",
		},
		Metadata: map[string]interface{}{
			"age_days":         5,   // Too new
			"total_downloads":  500, // Too few
			"recent_downloads": 50,  // Too few
		},
	}

	result, err := checker.Check(dep)
	if err != nil {
		t.Fatal(err)
	}

	if result.Passed {
		t.Error("Should fail for multiple violations")
	}

	// Should have 3 violations: 1 age + 2 download
	if len(result.Violations) != 3 {
		t.Errorf("Expected 3 violations, got %d", len(result.Violations))
	}

	ageCount := 0
	downloadCount := 0
	for _, v := range result.Violations {
		switch v.Type {
		case AgeViolation:
			ageCount++
		case DownloadViolation:
			downloadCount++
		}
	}

	if ageCount != 1 {
		t.Errorf("Expected 1 age violation, got %d", ageCount)
	}

	if downloadCount != 2 {
		t.Errorf("Expected 2 download violations, got %d", downloadCount)
	}
}

func TestChecker_Check_InvalidUntilDoesNotExempt(t *testing.T) {
	cases := map[string]string{
		"garbage":    "next tuesday",
		"wrong form": "01/02/2030",
		"blank":      "  ",
	}
	for name, until := range cases {
		t.Run(name, func(t *testing.T) {
			checker := NewChecker(config.CoolingConfig{
				Enabled:    true,
				MinAgeDays: 7,
				Exceptions: []config.CoolingException{
					{Pattern: "temp-package", Until: until},
				},
			})
			dep := &types.Dependency{
				Module:   types.Module{Name: "temp-package", Version: "1.0.0"},
				Metadata: map[string]interface{}{"age_days": 1},
			}

			result, err := checker.Check(dep)
			if err != nil {
				t.Fatal(err)
			}
			if result.IsException {
				t.Fatal("invalid until must not exempt the package")
			}
			if result.Passed {
				t.Fatal("young package with an invalid exception must fail")
			}
			var reported bool
			for _, v := range result.Violations {
				if v.Type == ExceptionConfigViolation {
					reported = true
				}
			}
			if !reported {
				t.Fatalf("invalid exception must be reported, violations=%+v", result.Violations)
			}
		})
	}
}

func TestChecker_Check_InvalidUntilOnOtherPatternIsIgnored(t *testing.T) {
	checker := NewChecker(config.CoolingConfig{
		Enabled:    true,
		MinAgeDays: 7,
		Exceptions: []config.CoolingException{
			{Pattern: "other-package", Until: "garbage"},
			{Pattern: "temp-package", Until: "2099-12-31"},
		},
	})
	dep := &types.Dependency{
		Module:   types.Module{Name: "temp-package", Version: "1.0.0"},
		Metadata: map[string]interface{}{"age_days": 1},
	}
	result, err := checker.Check(dep)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Passed || !result.IsException {
		t.Fatalf("valid future exception should still apply, got %+v", result)
	}
}

func invalidExceptionDep(ageDays int) *types.Dependency {
	return &types.Dependency{
		Module:   types.Module{Name: "temp-package", Version: "1.0.0"},
		Metadata: map[string]interface{}{"age_days": ageDays},
	}
}

func invalidExceptionConfig(alertOnly bool, graceDays int) config.CoolingConfig {
	return config.CoolingConfig{
		Enabled:         true,
		MinAgeDays:      7,
		AlertOnly:       alertOnly,
		GracePeriodDays: graceDays,
		Exceptions: []config.CoolingException{
			{Pattern: "temp-package", Until: "next tuesday"},
		},
	}
}

func TestChecker_Check_InvalidUntilFailsEvenWhenAlertOnly(t *testing.T) {
	result, err := NewChecker(invalidExceptionConfig(true, 0)).Check(invalidExceptionDep(1))
	if err != nil {
		t.Fatal(err)
	}
	if result.Passed {
		t.Fatal("alert_only must not let an invalid exception pass the gate")
	}
	if result.IsException {
		t.Fatal("invalid until must not exempt")
	}
}

func TestChecker_Check_InvalidUntilFailsEvenWhenInGracePeriod(t *testing.T) {
	// 5 days old, min 7, grace 3: in grace, which alone would pass.
	result, err := NewChecker(invalidExceptionConfig(false, 3)).Check(invalidExceptionDep(5))
	if err != nil {
		t.Fatal(err)
	}
	if !result.InGracePeriod {
		t.Fatal("test premise: package should be inside the grace period")
	}
	if result.Passed {
		t.Fatal("grace period must not let an invalid exception pass the gate")
	}
}

func TestChecker_Check_InvalidUntilFailsWhenOldEnoughToPass(t *testing.T) {
	// Without the exception this package passes cooling; the bad config still fails.
	result, err := NewChecker(invalidExceptionConfig(false, 0)).Check(invalidExceptionDep(30))
	if err != nil {
		t.Fatal(err)
	}
	if result.Passed {
		t.Fatal("an invalid exception must fail the gate even when age passes")
	}
	if len(result.Violations) != 1 || result.Violations[0].Type != ExceptionConfigViolation {
		t.Fatalf("expected only the config violation, got %+v", result.Violations)
	}
}

func TestChecker_Check_AlertOnlyAndGraceStillApplyWithoutInvalidException(t *testing.T) {
	alert := invalidExceptionConfig(true, 0)
	alert.Exceptions = nil
	if result, err := NewChecker(alert).Check(invalidExceptionDep(1)); err != nil || !result.Passed {
		t.Fatalf("alert_only without an invalid exception should still pass: %+v %v", result, err)
	}

	grace := invalidExceptionConfig(false, 3)
	grace.Exceptions = nil
	if result, err := NewChecker(grace).Check(invalidExceptionDep(5)); err != nil || !result.Passed {
		t.Fatalf("grace without an invalid exception should still pass: %+v %v", result, err)
	}
}

// A valid matching exception must not hide a malformed matching one, in either order.
func TestChecker_Check_ValidDuplicateDoesNotHideInvalidException(t *testing.T) {
	orders := map[string][]config.CoolingException{
		"invalid first": {
			{Pattern: "temp-package", Until: "garbage"},
			{Pattern: "temp-*", Until: "2099-12-31"},
		},
		"valid first": {
			{Pattern: "temp-*", Until: "2099-12-31"},
			{Pattern: "temp-package", Until: "garbage"},
		},
		"permanent first": {
			{Pattern: "temp-*"},
			{Pattern: "temp-package", Until: "garbage"},
		},
	}
	for name, exceptions := range orders {
		t.Run(name, func(t *testing.T) {
			checker := NewChecker(config.CoolingConfig{
				Enabled:         true,
				MinAgeDays:      7,
				AlertOnly:       true,
				GracePeriodDays: 3,
				Exceptions:      exceptions,
			})
			result, err := checker.Check(invalidExceptionDep(10))
			if err != nil {
				t.Fatal(err)
			}
			if result.Passed {
				t.Fatalf("a malformed matching exception must fail the gate, got %+v", result)
			}
			if result.IsException {
				t.Fatal("no exception may apply while a matching one is malformed")
			}
			var reported bool
			for _, v := range result.Violations {
				if v.Type == ExceptionConfigViolation {
					reported = true
				}
			}
			if !reported {
				t.Fatalf("the malformed exception must be reported, got %+v", result.Violations)
			}
		})
	}
}

// until is exclusive: the exception applies up to, but not including, 00:00:00Z on that date.
func TestChecker_Check_UntilIsExclusiveAtMidnightUTC(t *testing.T) {
	until := time.Date(2030, 6, 1, 0, 0, 0, 0, time.UTC)
	cases := map[string]struct {
		now        time.Time
		wantExempt bool
	}{
		"one second before": {until.Add(-time.Second), true},
		"exactly at":        {until, false},
		"one second after":  {until.Add(time.Second), false},
		"before, other zone": {
			// 23:59:59 UTC the day before, written in UTC+9
			until.Add(-time.Second).In(time.FixedZone("x", 9*3600)), true,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			checker := NewChecker(config.CoolingConfig{
				Enabled:    true,
				MinAgeDays: 7,
				Exceptions: []config.CoolingException{{Pattern: "temp-package", Until: "2030-06-01"}},
			})
			checker.now = func() time.Time { return tc.now }
			result, err := checker.Check(invalidExceptionDep(1))
			if err != nil {
				t.Fatal(err)
			}
			if result.IsException != tc.wantExempt || result.Passed != tc.wantExempt {
				t.Fatalf("now=%s: IsException=%v Passed=%v, want %v", tc.now, result.IsException, result.Passed, tc.wantExempt)
			}
		})
	}
}
