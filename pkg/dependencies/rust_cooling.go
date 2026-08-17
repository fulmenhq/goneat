/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package dependencies

import (
	"fmt"
	"os"
	"time"

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

// applyRustCooling runs cooling.Checker over enumerated crates.
// min_downloads_recent is forced to 0: crates.io version download counts are
// lifetime totals, not a 30-day window, and must not footgun a fresh version.
func applyRustCooling(deps []Dependency, policyPath string) ([]Issue, bool) {
	if policyPath == "" {
		return []Issue{{
			Type:     "configuration",
			Severity: "high",
			Message:  "Rust cooling requested but no policy path was provided",
		}}, false
	}

	policyConfig, err := loadPolicyConfig(policyPath)
	if err != nil {
		return []Issue{{
			Type:     "configuration",
			Severity: "high",
			Message:  fmt.Sprintf("Rust cooling could not load policy %s: %v", policyPath, err),
		}}, false
	}

	coolCfg, err := policy.ParseCoolingConfig(policyConfig)
	if err != nil || coolCfg == nil || !coolCfg.Enabled {
		return nil, true
	}

	// Do not apply crates.io per-version lifetime downloads as "recent".
	coolCfg.MinDownloadsRecent = 0

	checker := cooling.NewChecker(*coolCfg)
	issues := make([]Issue, 0)
	passed := true

	for i := range deps {
		dep := &deps[i]
		result, err := checker.Check(dep)
		if err != nil {
			continue
		}
		if result.Passed {
			continue
		}
		for _, violation := range result.Violations {
			message := violation.Message
			if violation.Type != "" {
				message = fmt.Sprintf("[%s] %s", violation.Type, violation.Message)
			}
			issues = append(issues, Issue{
				Type:       string(violation.Type),
				Severity:   string(violation.Severity),
				Message:    message,
				Dependency: dep,
			})
			passed = false
		}
	}

	return issues, passed
}
