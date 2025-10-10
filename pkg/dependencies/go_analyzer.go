package dependencies

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fulmenhq/goneat/pkg/config"
	"github.com/fulmenhq/goneat/pkg/cooling"
	"github.com/fulmenhq/goneat/pkg/dependencies/policy"
	"github.com/fulmenhq/goneat/pkg/registry"
	"github.com/google/go-licenses/licenses"
	"gopkg.in/yaml.v3"
)

// GoAnalyzer implements Analyzer for Go dependencies
type GoAnalyzer struct{}

func NewGoAnalyzer() Analyzer {
	return &GoAnalyzer{}
}

func (a *GoAnalyzer) Analyze(ctx context.Context, target string, cfg AnalysisConfig) (*AnalysisResult, error) {
	start := time.Now()

	// Check for go.mod
	goModPath := filepath.Join(target, "go.mod")
	if _, err := os.Stat(goModPath); err != nil {
		return nil, errors.New("no go.mod found in target directory")
	}

	// Create classifier for license detection
	classifier, err := licenses.NewClassifier(0.9) // 90% confidence threshold
	if err != nil {
		return nil, err
	}

	// Run license check
	libraries, err := licenses.Libraries(ctx, classifier, false, nil, ".")
	if err != nil {
		return nil, err
	}

	// Create registry client for cooling metadata
	registryClient := registry.NewGoClient(24 * time.Hour)

	// Convert results to our structures
	var deps []Dependency
	for _, lib := range libraries {
		name := lib.Name()
		version := lib.Version()

		// Get license information - try to read file for better detection
		licenseType := "Unknown"
		licenseName := filepath.Base(lib.LicensePath)

		// Try to detect license from file content
		if data, err := os.ReadFile(lib.LicensePath); err == nil {
			licenseType = detectLicenseType(string(data))
		}
		if licenseType == "Unknown" {
			licenseType = detectLicenseType(licenseName)
		}

		dep := Dependency{
			Module: Module{
				Name:     name,
				Version:  version,
				Language: LanguageGo,
			},
			License: &License{
				Name: licenseName,
				Type: licenseType,
				URL:  getLicenseURL(licenseType),
			},
			Metadata: map[string]interface{}{
				"license_path": lib.LicensePath,
				"packages":     lib.Packages,
			},
		}

		// Get real cooling metadata from registry
		if version != "" {
			if metadata, err := registryClient.GetMetadata(name, version); err == nil {
				ageDays := int(time.Since(metadata.PublishDate).Hours() / 24)
				dep.Metadata["age_days"] = ageDays
				dep.Metadata["publish_date"] = metadata.PublishDate
				dep.Metadata["total_downloads"] = metadata.TotalDownloads
			} else {
				// Fallback if registry fails - mark as unknown
				dep.Metadata["age_days"] = 365 // Conservative fallback (assume old)
				dep.Metadata["registry_error"] = err.Error()
				dep.Metadata["age_unknown"] = true
			}
		} else {
			// No version means local package
			dep.Metadata["age_days"] = 0
		}

		deps = append(deps, dep)
	}

	// Load and apply policy if specified
	var issues []Issue
	passed := true
	var policyConfig map[string]interface{}

	if cfg.PolicyPath != "" {
		// Load policy file
		policyData, err := os.ReadFile(cfg.PolicyPath)
		if err == nil {
			// Parse policy for direct evaluation
			if err := yaml.Unmarshal(policyData, &policyConfig); err == nil {
				// Check forbidden licenses
				if licensesConfig, ok := policyConfig["licenses"].(map[string]interface{}); ok {
					if forbidden, ok := licensesConfig["forbidden"].([]interface{}); ok {
						for i := range deps {
							dep := &deps[i]
							for _, forbiddenLicense := range forbidden {
								if dep.License.Type == forbiddenLicense.(string) {
									issues = append(issues, Issue{
										Type:       "license",
										Severity:   "critical",
										Message:    fmt.Sprintf("Package %s uses forbidden license: %s", dep.Name, dep.License.Type),
										Dependency: dep,
									})
									passed = false
								}
							}
						}
					}
				}

				// Check cooling policy using proper checker
				if coolCfg, err := policy.ParseCoolingConfig(policyConfig); err == nil && coolCfg != nil && coolCfg.Enabled {
					coolingChecker := cooling.NewChecker(*coolCfg)

					for i := range deps {
						dep := &deps[i]
						coolingResult, err := coolingChecker.Check(dep)
						if err != nil {
							// Log error but continue
							continue
						}

						if !coolingResult.Passed {
							for _, violation := range coolingResult.Violations {
								issues = append(issues, Issue{
									Type:       string(violation.Type),
									Severity:   string(violation.Severity),
									Message:    violation.Message,
									Dependency: dep,
								})
								passed = false
							}
						}
					}
				}
			}
		}

		// Also use OPA engine for policy evaluation
		engine := policy.NewOPAEngine()
		if err := engine.LoadPolicy(cfg.PolicyPath); err == nil {
			input := map[string]interface{}{
				"dependencies": deps,
				"policy":       policyConfig,
			}

			if result, err := engine.Evaluate(ctx, input); err == nil {
				// Process OPA deny results
				if denials, ok := result["data.goneat.dependencies.deny"].([]interface{}); ok {
					for _, denial := range denials {
						if msg, ok := denial.(string); ok {
							issues = append(issues, Issue{
								Type:       "policy",
								Severity:   "critical",
								Message:    msg,
								Dependency: nil,
							})
							passed = false
						}
					}
				}
			}
		}
	}

	return &AnalysisResult{
		Dependencies: deps,
		Issues:       issues,
		Passed:       passed,
		Duration:     time.Since(start),
	}, nil
}

func (a *GoAnalyzer) DetectLanguages(target string) ([]Language, error) {
	detector := NewDetector(&config.DependenciesConfig{}) // Default config
	lang, _, err := detector.Detect(target)
	if err != nil {
		return nil, err
	}
	if lang != "" {
		return []Language{lang}, nil
	}
	return []Language{}, nil
}
