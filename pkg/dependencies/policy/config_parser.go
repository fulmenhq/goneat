package policy

import (
	"github.com/fulmenhq/goneat/pkg/config"
)

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

	// Parse optional fields with type-safe extraction
	if minAge, ok := coolingCfg["min_age_days"].(int); ok {
		cfg.MinAgeDays = minAge
	}
	if minDownloads, ok := coolingCfg["min_downloads"].(int); ok {
		cfg.MinDownloads = minDownloads
	}
	if minDownloadsRecent, ok := coolingCfg["min_downloads_recent"].(int); ok {
		cfg.MinDownloadsRecent = minDownloadsRecent
	}
	if alertOnly, ok := coolingCfg["alert_only"].(bool); ok {
		cfg.AlertOnly = alertOnly
	}
	if gracePeriod, ok := coolingCfg["grace_period_days"].(int); ok {
		cfg.GracePeriodDays = gracePeriod
	}

	// Parse exceptions array
	if exceptions, ok := coolingCfg["exceptions"].([]interface{}); ok {
		cfg.Exceptions = parseExceptions(exceptions)
	}

	return &cfg, nil
}

// parseExceptions extracts cooling exception rules from raw policy data
func parseExceptions(exceptions []interface{}) []config.CoolingException {
	var result []config.CoolingException

	for _, exc := range exceptions {
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
		if until, ok := excMap["until"].(string); ok {
			exception.Until = until
		}
		if approvedBy, ok := excMap["approved_by"].(string); ok {
			exception.ApprovedBy = approvedBy
		}

		// Only add if pattern is present (minimum requirement)
		if exception.Pattern != "" {
			result = append(result, exception)
		}
	}

	return result
}
