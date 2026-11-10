package policy

import (
	"errors"
	"fmt"
	"time"

	"github.com/fulmenhq/goneat/pkg/config"
)

// ParseLicenseConfig extracts and validates license policy configuration from raw policy data.
func ParseLicenseConfig(policyData map[string]interface{}) (*config.LicensePolicyConfig, error) {
	licenseCfg, ok := policyData["licenses"].(map[string]interface{})
	if !ok {
		return nil, nil
	}

	cfg := config.LicensePolicyConfig{
		Forbidden: parseStringArray(licenseCfg["forbidden"]),
		Allowed:   parseStringArray(licenseCfg["allowed"]),
		Allow:     parseStringArray(licenseCfg["allow"]),
	}

	if exceptions, ok := licenseCfg["exceptions"].([]interface{}); ok {
		cfg.Exceptions = parseLicenseExceptions(exceptions)
	}

	return &cfg, nil
}

// ParseCoolingConfig extracts and validates cooling policy configuration from raw policy data.
// It applies sensible defaults for any missing values.
func ParseCoolingConfig(policyData map[string]interface{}) (*config.CoolingConfig, error) {
	coolingCfg, ok := policyData["cooling"].(map[string]interface{})
	if !ok {
		return nil, nil // No cooling config present
	}

	enabled, _ := coolingCfg["enabled"].(bool)
	if !enabled {
		return &config.CoolingConfig{Enabled: false}, nil
	}

	// Build config with defaults
	cfg := config.CoolingConfig{
		Enabled:            true,
		MinAgeDays:         7,   // default: 1 week
		MinDownloads:       100, // default: minimal popularity
		MinDownloadsRecent: 10,  // default: recent activity
		AlertOnly:          false,
		GracePeriodDays:    3,
	}

	// Parse optional fields with type-safe extraction. Invalid values are
	// collected and reported; the default stays in place for that field.
	var errs []error
	for _, field := range []struct {
		key string
		dst *int
	}{
		{"min_age_days", &cfg.MinAgeDays},
		{"min_downloads", &cfg.MinDownloads},
		{"min_downloads_recent", &cfg.MinDownloadsRecent},
		{"grace_period_days", &cfg.GracePeriodDays},
	} {
		raw, present := coolingCfg[field.key]
		if !present || raw == nil {
			continue
		}
		n, err := readInt(raw)
		if err != nil {
			errs = append(errs, fmt.Errorf("cooling.%s: %w", field.key, err))
			continue
		}
		*field.dst = n
	}
	if alertOnly, ok := coolingCfg["alert_only"].(bool); ok {
		cfg.AlertOnly = alertOnly
	}

	// Parse exceptions array
	if exceptions, ok := coolingCfg["exceptions"].([]interface{}); ok {
		var excErrs []error
		cfg.Exceptions, excErrs = parseExceptions(exceptions)
		errs = append(errs, excErrs...)
	}

	// cfg is always returned alongside any error so callers can keep running
	// the remaining checks. Exceptions that failed to parse are not in cfg.
	return &cfg, errors.Join(errs...)
}

// parseExceptions extracts cooling exception rules from raw policy data.
// An exception with an invalid until is dropped (it never applies) and
// reported in the returned errors, so a bad date fails closed.
func parseExceptions(exceptions []interface{}) ([]config.CoolingException, []error) {
	var result []config.CoolingException
	var errs []error

	for i, exc := range exceptions {
		excMap, ok := exc.(map[string]interface{})
		if !ok {
			continue
		}

		exception := config.CoolingException{}
		if pattern, ok := excMap["pattern"].(string); ok {
			exception.Pattern = pattern
		}
		if reason, ok := excMap["reason"].(string); ok {
			exception.Reason = reason
		}
		if approvedBy, ok := excMap["approved_by"].(string); ok {
			exception.ApprovedBy = approvedBy
		}

		// Only add if pattern is present (minimum requirement)
		if exception.Pattern == "" {
			continue
		}

		// An absent until means no expiry. A present until (including null)
		// must be a valid date.
		if rawUntil, present := excMap["until"]; present {
			until, err := readUntil(rawUntil)
			if err != nil {
				errs = append(errs, fmt.Errorf("cooling.exceptions[%d] (pattern %q) ignored: %w", i, exception.Pattern, err))
				continue
			}
			exception.Until = until
		}

		result = append(result, exception)
	}

	return result, errs
}

func parseLicenseExceptions(exceptions []interface{}) []config.LicenseException {
	var result []config.LicenseException

	for _, exc := range exceptions {
		excMap, ok := exc.(map[string]interface{})
		if !ok {
			continue
		}

		exception := config.LicenseException{
			Package:      readString(excMap["package"]),
			Name:         readString(excMap["name"]),
			License:      readString(excMap["license"]),
			Licenses:     parseStringArray(excMap["licenses"]),
			Reason:       readString(excMap["reason"]),
			Until:        readString(excMap["until"]),
			ApprovedBy:   readString(excMap["approved_by"]),
			ApprovedDate: readString(excMap["approved_date"]),
			Ticket:       readString(excMap["ticket"]),
		}

		if (exception.Package == "" && exception.Name == "") || (exception.License == "" && len(exception.Licenses) == 0) {
			continue
		}

		result = append(result, exception)
	}

	return result
}

func parseStringArray(value interface{}) []string {
	switch v := value.(type) {
	case []string:
		return append([]string(nil), v...)
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s := readString(item); s != "" {
				result = append(result, s)
			}
		}
		return result
	default:
		return nil
	}
}

func readString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case time.Time:
		return v.Format("2006-01-02")
	default:
		return ""
	}
}
