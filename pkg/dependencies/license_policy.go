package dependencies

import (
	"fmt"
	"strings"
	"time"

	"github.com/fulmenhq/goneat/pkg/config"
)

func evaluateForbiddenLicenses(deps []Dependency, licenseCfg *config.LicensePolicyConfig, now time.Time) ([]Issue, bool) {
	if licenseCfg == nil {
		licenseCfg = &config.LicensePolicyConfig{}
	}

	forbidden := make(map[string]struct{}, len(licenseCfg.Forbidden))
	for _, license := range licenseCfg.Forbidden {
		license = strings.TrimSpace(license)
		if license != "" {
			forbidden[license] = struct{}{}
		}
	}

	issues := []Issue{}
	passed := true

	for i := range deps {
		dep := &deps[i]
		if matchesLicenseException(*dep, licenseCfg.Exceptions, now) {
			continue
		}
		licenseType := "Unknown"
		if dep.License != nil && strings.TrimSpace(dep.License.Type) != "" {
			licenseType = strings.TrimSpace(dep.License.Type)
		}
		for _, component := range strings.Split(licenseType, " AND ") {
			component = strings.TrimSpace(component)
			candidate := *dep
			candidate.License = &License{Type: component}
			if matchesLicenseException(candidate, licenseCfg.Exceptions, now) {
				continue
			}
			message := ""
			if component == "" || strings.EqualFold(component, "unknown") || strings.EqualFold(component, "NOASSERTION") {
				message = fmt.Sprintf("Package %s has an unresolved required license", dep.Name)
			} else if _, ok := forbidden[component]; ok {
				message = fmt.Sprintf("Package %s uses forbidden license: %s", dep.Name, component)
			}
			if message != "" {
				issues = append(issues, Issue{Type: "license", Severity: "critical", Message: message, Dependency: dep})
				passed = false
			}
		}
	}

	return issues, passed
}

func matchesLicenseException(dep Dependency, exceptions []config.LicenseException, now time.Time) bool {
	if dep.License == nil {
		return false
	}

	for _, exc := range exceptions {
		if !licenseExceptionActive(exc, now) {
			continue
		}
		if !licenseExceptionMatchesPackage(exc, dep.Name) {
			continue
		}
		if licenseExceptionMatchesLicense(exc, dep.License.Type) {
			return true
		}
	}

	return false
}

func licenseExceptionMatchesPackage(exc config.LicenseException, packageName string) bool {
	target := strings.TrimSpace(exc.Package)
	if target == "" {
		target = strings.TrimSpace(exc.Name)
	}

	return target != "" && strings.TrimSpace(packageName) == target
}

func licenseExceptionMatchesLicense(exc config.LicenseException, license string) bool {
	license = strings.TrimSpace(license)
	if license == "" {
		return false
	}

	if strings.TrimSpace(exc.License) == license {
		return true
	}

	for _, allowed := range exc.Licenses {
		if strings.TrimSpace(allowed) == license {
			return true
		}
	}

	return false
}

func licenseExceptionActive(exc config.LicenseException, now time.Time) bool {
	currentDate := policyDateOnly(now)

	if approvedDate, ok := parsePolicyDate(exc.ApprovedDate); ok && currentDate.Before(approvedDate) {
		return false
	}

	if untilDate, ok := parsePolicyDate(exc.Until); ok && currentDate.After(untilDate) {
		return false
	}

	return true
}

func parsePolicyDate(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}

	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, false
	}

	return policyDateOnly(parsed), true
}

func policyDateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
