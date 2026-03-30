package dependencies

import (
	"testing"
	"time"

	"github.com/fulmenhq/goneat/pkg/config"
)

func TestEvaluateForbiddenLicenses(t *testing.T) {
	now := time.Date(2026, time.March, 30, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name           string
		licenseCfg     *config.LicensePolicyConfig
		wantIssues     int
		wantPassed     bool
		wantSuppressed bool
	}{
		{
			name: "valid exact exception suppresses forbidden finding",
			licenseCfg: &config.LicensePolicyConfig{
				Forbidden: []string{"GPL-3.0"},
				Exceptions: []config.LicenseException{{
					Package:      "github.com/hashicorp/go-cleanhttp",
					License:      "GPL-3.0",
					Reason:       "Verified upstream as MPL-2.0",
					ApprovedDate: "2026-03-01",
				}},
			},
			wantIssues:     0,
			wantPassed:     true,
			wantSuppressed: true,
		},
		{
			name: "expired exception does not suppress forbidden finding",
			licenseCfg: &config.LicensePolicyConfig{
				Forbidden: []string{"GPL-3.0"},
				Exceptions: []config.LicenseException{{
					Package: "github.com/hashicorp/go-cleanhttp",
					License: "GPL-3.0",
					Until:   "2026-03-01",
				}},
			},
			wantIssues: 1,
			wantPassed: false,
		},
		{
			name: "non matching exception does not suppress forbidden finding",
			licenseCfg: &config.LicensePolicyConfig{
				Forbidden: []string{"GPL-3.0"},
				Exceptions: []config.LicenseException{{
					Package: "github.com/hashicorp/go-retryablehttp",
					License: "GPL-3.0",
				}},
			},
			wantIssues: 1,
			wantPassed: false,
		},
		{
			name: "no exceptions configured reports forbidden finding",
			licenseCfg: &config.LicensePolicyConfig{
				Forbidden: []string{"GPL-3.0"},
			},
			wantIssues: 1,
			wantPassed: false,
		},
		{
			name: "future approved date is not active yet",
			licenseCfg: &config.LicensePolicyConfig{
				Forbidden: []string{"GPL-3.0"},
				Exceptions: []config.LicenseException{{
					Package:      "github.com/hashicorp/go-cleanhttp",
					License:      "GPL-3.0",
					ApprovedDate: "2026-04-15",
				}},
			},
			wantIssues: 1,
			wantPassed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := []Dependency{testForbiddenLicenseDependency()}

			issues, passed := evaluateForbiddenLicenses(deps, tt.licenseCfg, now)
			if len(issues) != tt.wantIssues {
				t.Fatalf("expected %d issues, got %d", tt.wantIssues, len(issues))
			}
			if passed != tt.wantPassed {
				t.Fatalf("expected passed=%t, got %t", tt.wantPassed, passed)
			}

			suppressed := matchesLicenseException(deps[0], tt.licenseCfg.Exceptions, now)
			if suppressed != tt.wantSuppressed {
				t.Fatalf("expected suppressed=%t, got %t", tt.wantSuppressed, suppressed)
			}
		})
	}
}

func testForbiddenLicenseDependency() Dependency {
	return Dependency{
		Module: Module{
			Name:     "github.com/hashicorp/go-cleanhttp",
			Version:  "v0.5.2",
			Language: LanguageGo,
		},
		License:  &License{Type: "GPL-3.0"},
		Metadata: map[string]interface{}{},
	}
}
