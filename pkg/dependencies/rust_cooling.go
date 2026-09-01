/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package dependencies

import (
	"fmt"
	"os"
	"time"

	"github.com/fulmenhq/goneat/pkg/config"
	"github.com/fulmenhq/goneat/pkg/cooling"
	"github.com/fulmenhq/goneat/pkg/dependencies/policy"
	"github.com/fulmenhq/goneat/pkg/logger"
	"github.com/fulmenhq/goneat/pkg/registry"
	"gopkg.in/yaml.v3"
)

// attachCratesIOMetadata stamps crates.io publish age onto Rust dependencies.
//
// On registry success: age_days, publish_date, total_downloads.
// On failure: age_unknown + registry_error, and age_days is left unset so
// cooling.Checker fail-closes. recent_downloads is never set — crates.io
// "recent" is per-version lifetime and is not a 30-day activity signal.
func attachCratesIOMetadata(deps []Dependency, client registry.Client) {
	for i := range deps {
		dep := &deps[i]
		if dep.Metadata == nil {
			dep.Metadata = map[string]interface{}{}
		}
		if isLocal, ok := dep.Metadata["is_local"].(bool); ok && isLocal {
			if _, hasAge := dep.Metadata["age_days"]; !hasAge {
				dep.Metadata["age_days"] = 0
			}
			continue
		}
		// Only crates.io registry sources may be queried. A git/path/other-registry
		// crate that shares a public crates.io name+version must not inherit that age.
		if !isCratesIORegistry(dep.Metadata) {
			delete(dep.Metadata, "age_days")
			dep.Metadata["age_unknown"] = true
			if _, ok := dep.Metadata["registry_error"]; !ok {
				dep.Metadata["registry_error"] = "not a crates.io package"
			}
			continue
		}
		if client == nil {
			dep.Metadata["age_unknown"] = true
			dep.Metadata["registry_error"] = "crates.io client not configured"
			continue
		}
		if dep.Version == "" {
			dep.Metadata["version_unknown"] = true
			dep.Metadata["age_unknown"] = true
			continue
		}

		meta, err := client.GetMetadata(dep.Name, dep.Version)
		if err != nil {
			dep.Metadata["age_unknown"] = true
			dep.Metadata["registry_error"] = err.Error()
			logger.Debug(fmt.Sprintf("crates.io metadata failed for %s@%s: %v", dep.Name, dep.Version, err))
			continue
		}

		ageDays := int(time.Since(meta.PublishDate).Hours() / 24)
		dep.Metadata["age_days"] = ageDays
		dep.Metadata["publish_date"] = meta.PublishDate
		dep.Metadata["total_downloads"] = meta.TotalDownloads
		dep.Metadata["registry"] = "crates.io"
	}
}

func isCratesIORegistry(meta map[string]interface{}) bool {
	if meta == nil {
		return false
	}
	reg, _ := meta["registry"].(string)
	return reg == "crates.io"
}

func loadPolicyConfig(path string) (map[string]interface{}, error) {
	if path == "" {
		return nil, fmt.Errorf("no policy path")
	}
	data, err := os.ReadFile(path) // #nosec G304 -- policy path from AnalysisConfig
	if err != nil {
		return nil, err
	}
	var policyConfig map[string]interface{}
	if err := yaml.Unmarshal(data, &policyConfig); err != nil {
		return nil, err
	}
	if _, ok := policyConfig["version"]; !ok {
		policyConfig["version"] = "v1"
	}
	return policyConfig, nil
}

// defaultRustCoolingConfig is the estate 7-day age gate used when --cooling
// is on and no policy YAML exists. Age only: no min_downloads_recent.
func defaultRustCoolingConfig() config.CoolingConfig {
	return config.CoolingConfig{
		Enabled:            true,
		MinAgeDays:         7,
		MinDownloads:       0,
		MinDownloadsRecent: 0,
		AlertOnly:          false,
		GracePeriodDays:    0,
	}
}

func rustCoolingConfig(policyPath string) (config.CoolingConfig, error) {
	if policyPath == "" {
		return defaultRustCoolingConfig(), nil
	}
	policyConfig, err := loadPolicyConfig(policyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultRustCoolingConfig(), nil
		}
		return config.CoolingConfig{}, err
	}
	coolCfg, err := policy.ParseCoolingConfig(policyConfig)
	if err != nil {
		return config.CoolingConfig{}, err
	}
	if coolCfg == nil {
		// Policy file exists but has no cooling: section — do not invent one.
		return config.CoolingConfig{Enabled: false}, nil
	}
	return *coolCfg, nil
}

// applyRustCooling runs cooling.Checker over enumerated crates.
// Missing policy YAML applies the 7-day age default (not a configuration fail).
// min_downloads_recent is forced to 0: crates.io version download counts are
// lifetime totals, not a 30-day window, and must not footgun a fresh version.
func applyRustCooling(deps []Dependency, policyPath string) ([]Issue, bool) {
	coolCfg, err := rustCoolingConfig(policyPath)
	if err != nil {
		return []Issue{{
			Type:     "configuration",
			Severity: "high",
			Message:  fmt.Sprintf("Rust cooling could not load policy %s: %v", policyPath, err),
		}}, false
	}
	if !coolCfg.Enabled {
		return nil, true
	}

	coolCfg.MinDownloadsRecent = 0

	checker := cooling.NewChecker(coolCfg)
	issues := make([]Issue, 0)
	passed := true

	for i := range deps {
		dep := &deps[i]
		result, err := checker.Check(dep)
		if err != nil {
			continue
		}
		recordCoolingResult(&issues, &passed, dep, result)
	}

	return issues, passed
}

// recordCoolingResult always records violations (including grace/alert-only).
// The gate fails only when the checker did not pass.
func recordCoolingResult(issues *[]Issue, passed *bool, dep *Dependency, result *cooling.CheckResult) {
	if result == nil {
		return
	}
	for _, violation := range result.Violations {
		message := violation.Message
		if violation.Type != "" {
			message = fmt.Sprintf("[%s] %s", violation.Type, violation.Message)
		}
		if result.InGracePeriod {
			message = "[grace] " + message
		}
		*issues = append(*issues, Issue{
			Type:       string(violation.Type),
			Severity:   string(violation.Severity),
			Message:    message,
			Dependency: dep,
		})
	}
	if !result.Passed {
		*passed = false
	}
}
