package dependencies

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fulmenhq/goneat/pkg/config"
	"github.com/fulmenhq/goneat/pkg/cooling"
	"github.com/fulmenhq/goneat/pkg/dependencies/policy"
	"github.com/fulmenhq/goneat/pkg/logger"
	"github.com/fulmenhq/goneat/pkg/registry"
	"github.com/google/go-licenses/licenses"
	"gopkg.in/yaml.v3"
)

// GoAnalyzer implements Analyzer for Go dependencies.
type GoAnalyzer struct{}

type goListModule struct {
	Path    string `json:"Path"`
	Version string `json:"Version"`
	Dir     string `json:"Dir"`
}

type goListPackage struct {
	ImportPath string        `json:"ImportPath"`
	Standard   bool          `json:"Standard"`
	Module     *goListModule `json:"Module"`
}

type goListMainModule struct {
	Path string `json:"Path"`
	Dir  string `json:"Dir"`
}

func NewGoAnalyzer() Analyzer {
	return &GoAnalyzer{}
}

func (a *GoAnalyzer) Analyze(ctx context.Context, target string, cfg AnalysisConfig) (*AnalysisResult, error) {
	start := time.Now()

	// Default to legacy behavior if flags are not provided.
	checkLicenses := cfg.CheckLicenses
	checkCooling := cfg.CheckCooling
	if !checkLicenses && !checkCooling {
		checkLicenses = true
		checkCooling = true
	}

	goModPath := filepath.Join(target, "go.mod")
	if _, err := os.Stat(goModPath); err != nil {
		return nil, errors.New("no go.mod found in target directory")
	}

	originalDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}

	absTarget, err := filepath.Abs(target)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve target path: %w", err)
	}

	if err := os.Chdir(absTarget); err != nil {
		return nil, fmt.Errorf("failed to change to target directory: %w", err)
	}
	defer func() { _ = os.Chdir(originalDir) }()

	mainMod, _ := loadMainModule(ctx)
	modules, err := discoverModules(ctx)
	if err != nil {
		return nil, err
	}

	// Create registry client for cooling metadata
	registryClient := registry.NewGoClient(24 * time.Hour)

	deps := make([]Dependency, 0, len(modules))
	for _, mod := range modules {
		dep := Dependency{
			Module: Module{
				Name:     mod.Path,
				Version:  mod.Version,
				Language: LanguageGo,
			},
			License:  nil,
			Metadata: map[string]interface{}{"module_dir": mod.Dir},
		}

		if mainMod.Path != "" && mod.Path == mainMod.Path {
			dep.Metadata["is_local"] = true
			dep.Metadata["age_days"] = 0
		} else if mod.Version != "" {
			if metadata, err := registryClient.GetMetadata(mod.Path, mod.Version); err == nil {
				ageDays := int(time.Since(metadata.PublishDate).Hours() / 24)
				dep.Metadata["age_days"] = ageDays
				dep.Metadata["publish_date"] = metadata.PublishDate
				dep.Metadata["total_downloads"] = metadata.TotalDownloads
				dep.Metadata["recent_downloads"] = metadata.RecentDownloads
			} else {
				dep.Metadata["age_days"] = 365
				dep.Metadata["registry_error"] = err.Error()
				dep.Metadata["age_unknown"] = true
			}
		} else {
			dep.Metadata["age_days"] = 0
			dep.Metadata["version_unknown"] = true
		}

		deps = append(deps, dep)
	}

	issues := make([]Issue, 0)
	passed := true
	var policyConfig map[string]interface{}

	// Ensure slices are non-nil for schema compliance
	if deps == nil {
		deps = make([]Dependency, 0)
	}

	if checkLicenses {
		licenseMap, degraded, licErr := collectLicenses(ctx)
		if licErr != nil {
			logger.Warn("dependencies: license detection failed", logger.Err(licErr))
			issues = append(issues, Issue{Type: "license", Severity: "medium", Message: fmt.Sprintf("License detection degraded: %v", licErr), Dependency: nil})
			degraded = true
		}

		if degraded {
			// Best-effort: scan module dirs for license-file presence
			for i := range deps {
				dep := &deps[i]
				if dep.Metadata == nil {
					dep.Metadata = map[string]interface{}{}
				}
				if dep.License != nil {
					continue
				}
				moduleDir, _ := dep.Metadata["module_dir"].(string)
				if moduleDir == "" {
					continue
				}
				lp := findLicenseFile(moduleDir)
				if lp == "" {
					continue
				}
				licenseType := "Unknown"
				if data, err := os.ReadFile(lp); err == nil {
					licenseType = detectLicenseType(string(data))
				}
				dep.License = &License{Name: filepath.Base(lp), Type: licenseType, URL: getLicenseURL(licenseType)}
				dep.Metadata["license_path"] = lp
				dep.Metadata["license_detection"] = "module_dir"
			}
		} else {
			for i := range deps {
				dep := &deps[i]
				k := dep.Name + "@" + dep.Version
				if lic, ok := licenseMap[k]; ok {
					dep.License = lic
					if dep.Metadata == nil {
						dep.Metadata = map[string]interface{}{}
					}
					dep.Metadata["license_detection"] = "go_licenses"
				}
			}
		}
	}

	if cfg.PolicyPath != "" {
		policyData, err := os.ReadFile(cfg.PolicyPath)
		if err == nil {
			if err := yaml.Unmarshal(policyData, &policyConfig); err == nil {
				if checkLicenses {
					if licensesConfig, ok := policyConfig["licenses"].(map[string]interface{}); ok {
						if forbidden, ok := licensesConfig["forbidden"].([]interface{}); ok {
							for i := range deps {
								dep := &deps[i]
								if dep.License == nil {
									continue
								}
								for _, forbiddenLicense := range forbidden {
									if dep.License.Type == forbiddenLicense.(string) {
										issues = append(issues, Issue{Type: "license", Severity: "critical", Message: fmt.Sprintf("Package %s uses forbidden license: %s", dep.Name, dep.License.Type), Dependency: dep})
										passed = false
									}
								}
							}
						}
					}
				}

				if checkCooling {
					if coolCfg, err := policy.ParseCoolingConfig(policyConfig); err == nil && coolCfg != nil && coolCfg.Enabled {
						coolingChecker := cooling.NewChecker(*coolCfg)
						for i := range deps {
							dep := &deps[i]
							coolingResult, err := coolingChecker.Check(dep)
							if err != nil {
								continue
							}
							if !coolingResult.Passed {
								for _, violation := range coolingResult.Violations {
									message := violation.Message
									if violation.Type != "" {
										message = fmt.Sprintf("[%s] %s", violation.Type, violation.Message)
									}
									issues = append(issues, Issue{Type: string(violation.Type), Severity: string(violation.Severity), Message: message, Dependency: dep})
									passed = false
								}
							}
						}
					}
				}
			}
		}

		engine := policy.NewOPAEngine()
		if err := engine.LoadPolicy(cfg.PolicyPath); err == nil {
			input := map[string]interface{}{"dependencies": deps, "policy": policyConfig}
			if result, err := engine.Evaluate(ctx, input); err == nil {
				if denials, ok := result["data.goneat.dependencies.deny"].([]interface{}); ok {
					for _, denial := range denials {
						if msg, ok := denial.(string); ok {
							issues = append(issues, Issue{Type: "policy", Severity: "critical", Message: msg, Dependency: nil})
							passed = false
						}
					}
				}
			}
		}
	}

	return &AnalysisResult{Dependencies: deps, Issues: issues, Passed: passed, Duration: time.Since(start)}, nil
}

func (a *GoAnalyzer) DetectLanguages(target string) ([]Language, error) {
	detector := NewDetector(&config.DependenciesConfig{})
	lang, _, err := detector.Detect(target)
	if err != nil {
		return nil, err
	}
	if lang != "" {
		return []Language{lang}, nil
	}
	return []Language{}, nil
}

func loadMainModule(ctx context.Context) (goListMainModule, error) {
	cmd := exec.CommandContext(ctx, "go", "list", "-m", "-json")
	out, err := cmd.Output()
	if err != nil {
		return goListMainModule{}, err
	}
	var mod goListMainModule
	if err := json.Unmarshal(out, &mod); err != nil {
		return goListMainModule{}, err
	}
	return mod, nil
}

func discoverModules(ctx context.Context) ([]goListModule, error) {
	cmd := exec.CommandContext(ctx, "go", "list", "-deps", "-json", "./...")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	dec := json.NewDecoder(stdout)
	seen := map[string]goListModule{}
	for {
		var pkg goListPackage
		err := dec.Decode(&pkg)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			_ = cmd.Wait()
			return nil, fmt.Errorf("failed to decode go list output: %w", err)
		}
		if pkg.Standard || pkg.Module == nil {
			continue
		}
		key := pkg.Module.Path + "@" + pkg.Module.Version
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = *pkg.Module
	}

	if err := cmd.Wait(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return nil, fmt.Errorf("go list failed: %w: %s", err, msg)
		}
		return nil, err
	}

	mods := make([]goListModule, 0, len(seen))
	for _, m := range seen {
		mods = append(mods, m)
	}
	sort.Slice(mods, func(i, j int) bool {
		if mods[i].Path == mods[j].Path {
			return mods[i].Version < mods[j].Version
		}
		return mods[i].Path < mods[j].Path
	})
	return mods, nil
}

func collectLicenses(ctx context.Context) (map[string]*License, bool, error) {
	classifier, err := licenses.NewClassifier(0.9)
	if err != nil {
		return nil, true, err
	}

	libraries, err := licenses.Libraries(ctx, classifier, false, nil, "./...")
	if err != nil {
		if isStdlibModuleInfoError(err) {
			return nil, true, err
		}
		return nil, true, err
	}

	out := map[string]*License{}
	for _, lib := range libraries {
		licenseType := "Unknown"
		licenseName := filepath.Base(lib.LicensePath)
		if data, err := os.ReadFile(lib.LicensePath); err == nil {
			licenseType = detectLicenseType(string(data))
		}
		if licenseType == "Unknown" {
			licenseType = detectLicenseType(licenseName)
		}
		k := lib.Name() + "@" + lib.Version()
		out[k] = &License{Name: licenseName, Type: licenseType, URL: getLicenseURL(licenseType)}
	}

	return out, false, nil
}

func isStdlibModuleInfoError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "does not have module info") && strings.Contains(msg, "Non go modules projects")
}

func findLicenseFile(moduleDir string) string {
	candidates := []string{
		"LICENSE",
		"LICENSE.txt",
		"LICENSE.md",
		"COPYING",
		"COPYING.txt",
		"NOTICE",
		"NOTICE.txt",
	}
	for _, name := range candidates {
		p := filepath.Join(moduleDir, name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// Also allow LICENSE.* without over-walking the tree.
	matches, _ := filepath.Glob(filepath.Join(moduleDir, "LICENSE.*"))
	if len(matches) > 0 {
		sort.Strings(matches)
		return matches[0]
	}
	return ""
}
