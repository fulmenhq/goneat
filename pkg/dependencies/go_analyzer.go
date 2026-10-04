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
	"github.com/fulmenhq/goneat/pkg/safeio"
	"github.com/fulmenhq/goneat/pkg/schema"

	"gopkg.in/yaml.v3"
)

// GoAnalyzer implements Analyzer for Go dependencies.
type GoAnalyzer struct {
	collect func(context.Context) (map[string]*License, bool, error)
}

type goListModule struct {
	Path    string        `json:"Path"`
	Version string        `json:"Version"`
	Dir     string        `json:"Dir"`
	Replace *goListModule `json:"Replace"`
}

type goListPackage struct {
	ImportPath                                                                                              string        `json:"ImportPath"`
	Standard                                                                                                bool          `json:"Standard"`
	Module                                                                                                  *goListModule `json:"Module"`
	Dir                                                                                                     string        `json:"Dir"`
	Goroot                                                                                                  bool          `json:"Goroot"`
	GoFiles, CgoFiles, CFiles, CXXFiles, MFiles, HFiles, FFiles, SFiles, SwigFiles, SwigCXXFiles, SysoFiles []string
	TestGoFiles, XTestGoFiles                                                                               []string
}

func (p goListPackage) hasSourceFiles() bool {
	for _, files := range [][]string{p.GoFiles, p.CgoFiles, p.CFiles, p.CXXFiles, p.MFiles, p.HFiles, p.FFiles, p.SFiles, p.SwigFiles, p.SwigCXXFiles, p.SysoFiles} {
		if len(files) > 0 {
			return true
		}
	}
	return false
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
		collector := a.collect
		if collector == nil {
			collector = collectLicenses
		}
		licenseMap, degraded, licErr := collector(ctx)
		licenseIssues, licensePassed := applyGoLicenseInventory(deps, licenseMap, degraded, licErr)
		issues = append(issues, licenseIssues...)
		passed = passed && licensePassed
	}

	var licenseCfg *config.LicensePolicyConfig

	if cfg.PolicyPath != "" {
		policyData, err := os.ReadFile(cfg.PolicyPath)
		if err == nil {
			if err := yaml.Unmarshal(policyData, &policyConfig); err == nil {
				if _, ok := policyConfig["version"]; !ok {
					policyConfig["version"] = "v1"
				}
				if result, vErr := schema.Validate(policyConfig, "dependencies-policy-v1.0.0"); vErr != nil {
					logger.Warn("dependencies: policy schema validation error", logger.Err(vErr))
					policyConfig = nil
				} else if !result.Valid {
					logger.Warn("dependencies: policy failed schema validation")
					policyConfig = nil
				}
				if checkLicenses {
					licenseCfg, _ = policy.ParseLicenseConfig(policyConfig)
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
							recordCoolingResult(&issues, &passed, dep, coolingResult)
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
	if checkLicenses {
		// A fallback is diagnostic evidence only, not a policy-quality inventory.
		if !hasLicenseCollectionError(issues) {
			licenseIssues, licensePassed := evaluateForbiddenLicenses(deps, licenseCfg, time.Now())
			issues = append(issues, licenseIssues...)
			passed = passed && licensePassed
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

func applyGoLicenseInventory(deps []Dependency, inventory map[string]*License, degraded bool, collectionErr error) ([]Issue, bool) {
	var problems []string
	if collectionErr != nil {
		problems = append(problems, collectionErr.Error())
	}
	if degraded {
		problems = append(problems, "license collection is degraded; module-directory fallback is not full assurance")
	}
	if len(inventory) == 0 || len(deps) == 0 {
		problems = append(problems, "license inventory is empty")
	}
	for i := range deps {
		dep := &deps[i]
		if dep.Metadata == nil {
			dep.Metadata = map[string]interface{}{}
		}
		if lic, ok := inventory[dep.Name+"@"+dep.Version]; ok {
			dep.License = lic
			dep.Metadata["license_detection"] = "go_licenses"
			continue
		}
		problems = append(problems, fmt.Sprintf("license inventory has no coverage for %s@%s", dep.Name, dep.Version))
		// Diagnostic fallback only. It can never clear a collection error.
		moduleDir, _ := dep.Metadata["module_dir"].(string)
		if moduleDir == "" {
			continue
		}
		lp := findLicenseFile(moduleDir)
		if lp == "" {
			continue
		}
		if data, err := safeio.ReadFileContained(moduleDir, lp); err == nil {
			licenseType := detectLicenseType(string(data))
			dep.License = &License{Name: filepath.Base(lp), Type: licenseType, URL: getLicenseURL(licenseType)}
			dep.Metadata["license_path"] = lp
			dep.Metadata["license_detection"] = "module_dir"
		}
	}
	if len(problems) == 0 {
		return nil, true
	}
	return []Issue{{Type: "license_error", Severity: "critical", Message: "License collection failed: " + strings.Join(problems, "; ")}}, false
}

func hasLicenseCollectionError(issues []Issue) bool {
	for _, issue := range issues {
		if issue.Type == "license_error" {
			return true
		}
	}
	return false
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
