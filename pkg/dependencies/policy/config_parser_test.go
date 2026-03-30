package policy

import (
	"testing"
)

func TestParseCoolingConfig_Enabled(t *testing.T) {
	policyData := map[string]interface{}{
		"cooling": map[string]interface{}{
			"enabled":              true,
			"min_age_days":         14,
			"min_downloads":        500,
			"min_downloads_recent": 50,
			"alert_only":           true,
			"grace_period_days":    7,
		},
	}

	cfg, err := ParseCoolingConfig(policyData)
	if err != nil {
		t.Fatalf("ParseCoolingConfig failed: %v", err)
	}

	if cfg == nil {
		t.Fatal("Expected config, got nil")
	}

	if !cfg.Enabled {
		t.Error("Expected Enabled=true")
	}
	if cfg.MinAgeDays != 14 {
		t.Errorf("Expected MinAgeDays=14, got %d", cfg.MinAgeDays)
	}
	if cfg.MinDownloads != 500 {
		t.Errorf("Expected MinDownloads=500, got %d", cfg.MinDownloads)
	}
	if cfg.MinDownloadsRecent != 50 {
		t.Errorf("Expected MinDownloadsRecent=50, got %d", cfg.MinDownloadsRecent)
	}
	if !cfg.AlertOnly {
		t.Error("Expected AlertOnly=true")
	}
	if cfg.GracePeriodDays != 7 {
		t.Errorf("Expected GracePeriodDays=7, got %d", cfg.GracePeriodDays)
	}
}

func TestParseLicenseConfig_WithPackageException(t *testing.T) {
	policyData := map[string]interface{}{
		"licenses": map[string]interface{}{
			"forbidden": []interface{}{"GPL-3.0", "AGPL-3.0"},
			"exceptions": []interface{}{
				map[string]interface{}{
					"package":       "github.com/hashicorp/go-cleanhttp",
					"license":       "GPL-3.0",
					"reason":        "Upstream license manually verified",
					"approved_by":   "@legal",
					"approved_date": "2026-03-30",
					"until":         "2026-06-30",
					"ticket":        "GNT-008",
				},
			},
		},
	}

	cfg, err := ParseLicenseConfig(policyData)
	if err != nil {
		t.Fatalf("ParseLicenseConfig failed: %v", err)
	}

	if cfg == nil {
		t.Fatal("Expected config, got nil")
	}

	if len(cfg.Forbidden) != 2 {
		t.Fatalf("Expected 2 forbidden licenses, got %d", len(cfg.Forbidden))
	}
	if len(cfg.Exceptions) != 1 {
		t.Fatalf("Expected 1 exception, got %d", len(cfg.Exceptions))
	}

	exc := cfg.Exceptions[0]
	if exc.Package != "github.com/hashicorp/go-cleanhttp" {
		t.Errorf("Expected package to be parsed, got %q", exc.Package)
	}
	if exc.License != "GPL-3.0" {
		t.Errorf("Expected license to be parsed, got %q", exc.License)
	}
	if exc.Until != "2026-06-30" {
		t.Errorf("Expected until to be parsed, got %q", exc.Until)
	}
	if exc.ApprovedDate != "2026-03-30" {
		t.Errorf("Expected approved_date to be parsed, got %q", exc.ApprovedDate)
	}
}

func TestParseLicenseConfig_WithNameAndLicensesException(t *testing.T) {
	policyData := map[string]interface{}{
		"licenses": map[string]interface{}{
			"exceptions": []interface{}{
				map[string]interface{}{
					"name":     "github.com/hashicorp/go-retryablehttp",
					"licenses": []interface{}{"GPL-3.0", "MPL-2.0"},
					"reason":   "Multiple acceptable observed classifier outputs",
				},
			},
		},
	}

	cfg, err := ParseLicenseConfig(policyData)
	if err != nil {
		t.Fatalf("ParseLicenseConfig failed: %v", err)
	}

	if cfg == nil {
		t.Fatal("Expected config, got nil")
	}
	if len(cfg.Exceptions) != 1 {
		t.Fatalf("Expected 1 exception, got %d", len(cfg.Exceptions))
	}

	exc := cfg.Exceptions[0]
	if exc.Name != "github.com/hashicorp/go-retryablehttp" {
		t.Errorf("Expected name to be parsed, got %q", exc.Name)
	}
	if len(exc.Licenses) != 2 {
		t.Fatalf("Expected 2 licenses, got %d", len(exc.Licenses))
	}
}

func TestParseLicenseConfig_MalformedExceptions(t *testing.T) {
	policyData := map[string]interface{}{
		"licenses": map[string]interface{}{
			"exceptions": []interface{}{
				map[string]interface{}{
					"package": "github.com/valid/pkg",
					"license": "GPL-3.0",
				},
				map[string]interface{}{
					"package": "github.com/missing/license",
				},
				map[string]interface{}{
					"license": "GPL-3.0",
				},
				"invalid-type",
			},
		},
	}

	cfg, err := ParseLicenseConfig(policyData)
	if err != nil {
		t.Fatalf("ParseLicenseConfig failed: %v", err)
	}

	if cfg == nil {
		t.Fatal("Expected config, got nil")
	}
	if len(cfg.Exceptions) != 1 {
		t.Fatalf("Expected 1 valid exception, got %d", len(cfg.Exceptions))
	}
}

func TestParseCoolingConfig_Defaults(t *testing.T) {
	policyData := map[string]interface{}{
		"cooling": map[string]interface{}{
			"enabled": true,
			// All other fields omitted - should use defaults
		},
	}

	cfg, err := ParseCoolingConfig(policyData)
	if err != nil {
		t.Fatalf("ParseCoolingConfig failed: %v", err)
	}

	if cfg == nil {
		t.Fatal("Expected config, got nil")
	}

	// Verify defaults
	if cfg.MinAgeDays != 7 {
		t.Errorf("Expected default MinAgeDays=7, got %d", cfg.MinAgeDays)
	}
	if cfg.MinDownloads != 100 {
		t.Errorf("Expected default MinDownloads=100, got %d", cfg.MinDownloads)
	}
	if cfg.MinDownloadsRecent != 10 {
		t.Errorf("Expected default MinDownloadsRecent=10, got %d", cfg.MinDownloadsRecent)
	}
	if cfg.AlertOnly {
		t.Error("Expected default AlertOnly=false")
	}
	if cfg.GracePeriodDays != 3 {
		t.Errorf("Expected default GracePeriodDays=3, got %d", cfg.GracePeriodDays)
	}
}

func TestParseCoolingConfig_Disabled(t *testing.T) {
	policyData := map[string]interface{}{
		"cooling": map[string]interface{}{
			"enabled": false,
		},
	}

	cfg, err := ParseCoolingConfig(policyData)
	if err != nil {
		t.Fatalf("ParseCoolingConfig failed: %v", err)
	}

	if cfg == nil {
		t.Fatal("Expected config, got nil")
	}

	if cfg.Enabled {
		t.Error("Expected Enabled=false")
	}
}

func TestParseCoolingConfig_NoCoolingSection(t *testing.T) {
	policyData := map[string]interface{}{
		"licenses": map[string]interface{}{
			"forbidden": []interface{}{"GPL-3.0"},
		},
	}

	cfg, err := ParseCoolingConfig(policyData)
	if err != nil {
		t.Fatalf("ParseCoolingConfig failed: %v", err)
	}

	if cfg != nil {
		t.Error("Expected nil config when cooling section missing")
	}
}

func TestParseCoolingConfig_WithExceptions(t *testing.T) {
	policyData := map[string]interface{}{
		"cooling": map[string]interface{}{
			"enabled":      true,
			"min_age_days": 7,
			"exceptions": []interface{}{
				map[string]interface{}{
					"pattern":     "@myorg/*",
					"reason":      "Internal packages",
					"approved_by": "@security-team",
				},
				map[string]interface{}{
					"pattern": "github.com/trusted/*",
					"reason":  "Trusted vendor",
					"until":   "2025-12-31",
				},
			},
		},
	}

	cfg, err := ParseCoolingConfig(policyData)
	if err != nil {
		t.Fatalf("ParseCoolingConfig failed: %v", err)
	}

	if cfg == nil {
		t.Fatal("Expected config, got nil")
	}

	if len(cfg.Exceptions) != 2 {
		t.Fatalf("Expected 2 exceptions, got %d", len(cfg.Exceptions))
	}

	// Check first exception
	exc1 := cfg.Exceptions[0]
	if exc1.Pattern != "@myorg/*" {
		t.Errorf("Expected pattern '@myorg/*', got '%s'", exc1.Pattern)
	}
	if exc1.Reason != "Internal packages" {
		t.Errorf("Expected reason 'Internal packages', got '%s'", exc1.Reason)
	}
	if exc1.ApprovedBy != "@security-team" {
		t.Errorf("Expected approved_by '@security-team', got '%s'", exc1.ApprovedBy)
	}

	// Check second exception
	exc2 := cfg.Exceptions[1]
	if exc2.Pattern != "github.com/trusted/*" {
		t.Errorf("Expected pattern 'github.com/trusted/*', got '%s'", exc2.Pattern)
	}
	if exc2.Until != "2025-12-31" {
		t.Errorf("Expected until '2025-12-31', got '%s'", exc2.Until)
	}
}

func TestParseCoolingConfig_MalformedExceptions(t *testing.T) {
	policyData := map[string]interface{}{
		"cooling": map[string]interface{}{
			"enabled": true,
			"exceptions": []interface{}{
				map[string]interface{}{
					"pattern": "@valid/*",
					"reason":  "Valid exception",
				},
				map[string]interface{}{
					// Missing pattern - should be skipped
					"reason": "Invalid - no pattern",
				},
				"invalid-string-type", // Invalid type - should be skipped
				map[string]interface{}{
					"pattern": "another-valid",
				},
			},
		},
	}

	cfg, err := ParseCoolingConfig(policyData)
	if err != nil {
		t.Fatalf("ParseCoolingConfig failed: %v", err)
	}

	if cfg == nil {
		t.Fatal("Expected config, got nil")
	}

	// Should only have 2 valid exceptions (skipped invalid ones)
	if len(cfg.Exceptions) != 2 {
		t.Errorf("Expected 2 valid exceptions, got %d", len(cfg.Exceptions))
	}

	if cfg.Exceptions[0].Pattern != "@valid/*" {
		t.Errorf("Expected first pattern '@valid/*', got '%s'", cfg.Exceptions[0].Pattern)
	}
	if cfg.Exceptions[1].Pattern != "another-valid" {
		t.Errorf("Expected second pattern 'another-valid', got '%s'", cfg.Exceptions[1].Pattern)
	}
}

func TestParseExceptions_EmptyArray(t *testing.T) {
	exceptions := []interface{}{}
	result := parseExceptions(exceptions)

	if len(result) != 0 {
		t.Errorf("Expected 0 exceptions, got %d", len(result))
	}
}

func TestParseExceptions_AllFieldsPopulated(t *testing.T) {
	exceptions := []interface{}{
		map[string]interface{}{
			"pattern":     "test-pattern",
			"reason":      "test-reason",
			"until":       "2025-12-31",
			"approved_by": "test-approver",
		},
	}

	result := parseExceptions(exceptions)

	if len(result) != 1 {
		t.Fatalf("Expected 1 exception, got %d", len(result))
	}

	exc := result[0]
	if exc.Pattern != "test-pattern" {
		t.Errorf("Expected pattern 'test-pattern', got '%s'", exc.Pattern)
	}
	if exc.Reason != "test-reason" {
		t.Errorf("Expected reason 'test-reason', got '%s'", exc.Reason)
	}
	if exc.Until != "2025-12-31" {
		t.Errorf("Expected until '2025-12-31', got '%s'", exc.Until)
	}
	if exc.ApprovedBy != "test-approver" {
		t.Errorf("Expected approved_by 'test-approver', got '%s'", exc.ApprovedBy)
	}
}
