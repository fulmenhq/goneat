package cooling

import (
	"testing"

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

	// Should pass conservatively when no metadata
	if !result.Passed {
		t.Error("Should pass when no age metadata available (conservative)")
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
