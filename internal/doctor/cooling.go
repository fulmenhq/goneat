package doctor

import (
	"fmt"
	"io/fs"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/fulmenhq/goneat/internal/assets"
	"github.com/fulmenhq/goneat/pkg/config"
	"github.com/fulmenhq/goneat/pkg/cooling"
	"github.com/fulmenhq/goneat/pkg/dependencies/types"
	"github.com/fulmenhq/goneat/pkg/logger"
	pkgtools "github.com/fulmenhq/goneat/pkg/tools"
	"github.com/fulmenhq/goneat/pkg/tools/metadata"
)

// CoolingCheckResult contains the result of cooling policy validation for a tool
type CoolingCheckResult struct {
	Passed        bool
	Disabled      bool
	Violations    []cooling.Violation
	IsException   bool
	InGracePeriod bool
	Metadata      *metadata.Metadata
	Error         error
}

// CheckToolCoolingPolicy validates a tool against its effective cooling policy
// Accepts a shared metadata registry to enable caching across multiple tool checks
// Returns nil if cooling is disabled or check passes
func CheckToolCoolingPolicy(tool Tool, disableCooling bool, reg *metadata.Registry) *CoolingCheckResult {
	// Get effective cooling config (global + tool-specific + CLI flag)
	coolingConfig, err := tool.GetEffectiveCoolingConfig(disableCooling)
	if err != nil {
		logger.Warn(fmt.Sprintf("Failed to load cooling config for %s: %v", tool.Name, err))
		return &CoolingCheckResult{
			Passed:   true, // Conservative fallback: pass on config error
			Disabled: false,
			Error:    err,
		}
	}

	// If cooling is disabled, pass immediately
	if !coolingConfig.Enabled {
		return &CoolingCheckResult{
			Passed:   true,
			Disabled: true,
		}
	}

	// Fetch tool metadata using shared registry
	meta, err := fetchToolMetadata(tool, reg)
	if err != nil {
		// IMPORTANT: Unlike Phase 1, we now FAIL cooling checks when metadata is unavailable
		// Rationale: If we can't determine tool age, we cannot enforce cooling policy
		// Users can bypass with --no-cooling for offline/air-gapped environments
		logger.Warn(fmt.Sprintf("Cannot fetch metadata for %s: %v", tool.Name, err))
		return &CoolingCheckResult{
			Passed:   false,
			Disabled: false,
			Error:    err,
			Violations: []cooling.Violation{
				{
					Type:     "metadata_unavailable",
					Severity: cooling.SeverityHigh,
					Message:  fmt.Sprintf("Unable to determine release age: %v", err),
				},
			},
		}
	}

	// Convert metadata to dependency format for cooling checker
	dep := metadataToDependency(tool.Name, meta)

	// Run cooling policy check
	checker := cooling.NewChecker(configToChecker(*coolingConfig))
	checkResult, err := checker.Check(dep)
	if err != nil {
		logger.Warn(fmt.Sprintf("Cooling policy check error for %s: %v", tool.Name, err))
		return &CoolingCheckResult{
			Passed:   true, // Conservative fallback
			Disabled: false,
			Metadata: meta,
			Error:    err,
		}
	}

	return &CoolingCheckResult{
		Passed:        checkResult.Passed,
		Disabled:      false,
		Violations:    checkResult.Violations,
		IsException:   checkResult.IsException,
		InGracePeriod: checkResult.InGracePeriod,
		Metadata:      meta,
		Error:         nil,
	}
}

func inferRepoFromGoInstallPackage(installPackage string) string {
	// Examples:
	// - github.com/rhysd/actionlint/cmd/actionlint@latest -> rhysd/actionlint
	// - github.com/google/go-licenses@latest -> google/go-licenses
	installPackage = strings.TrimSpace(installPackage)
	if installPackage == "" {
		return ""
	}

	// Strip @version suffix
	if at := strings.Index(installPackage, "@"); at >= 0 {
		installPackage = installPackage[:at]
	}

	installPackage = strings.TrimPrefix(installPackage, "https://")
	installPackage = strings.TrimPrefix(installPackage, "http://")
	if !strings.HasPrefix(installPackage, "github.com/") {
		return ""
	}
	installPackage = strings.TrimPrefix(installPackage, "github.com/")

	parts := strings.Split(installPackage, "/")
	if len(parts) < 2 {
		return ""
	}
	owner := parts[0]
	repo := parts[1]
	if owner == "" || repo == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s", owner, repo)
}

func inferRepoFromPythonInstall(installCommands map[string]string) string {
	if len(installCommands) == 0 {
		return ""
	}

	packageName := ""
	for _, cmd := range installCommands {
		cmd = strings.TrimSpace(cmd)
		if cmd == "" {
			continue
		}

		// uv tool install yamllint
		if strings.Contains(cmd, "uv tool install ") {
			parts := strings.Fields(cmd)
			for i := 0; i < len(parts)-1; i++ {
				if parts[i] != "install" {
					continue
				}
				for j := i + 1; j < len(parts); j++ {
					if strings.HasPrefix(parts[j], "-") {
						continue
					}
					packageName = parts[j]
					break
				}
				break
			}
		}

		// pip install yamllint
		if packageName == "" && strings.Contains(cmd, "pip install ") {
			parts := strings.Fields(cmd)
			for i := 0; i < len(parts)-1; i++ {
				if parts[i] != "install" {
					continue
				}
				for j := i + 1; j < len(parts); j++ {
					if strings.HasPrefix(parts[j], "-") {
						continue
					}
					packageName = parts[j]
					break
				}
				break
			}
		}

		if packageName != "" {
			break
		}
	}

	packageName = strings.TrimSpace(packageName)
	packageName = strings.Trim(packageName, "\"'")
	if packageName == "" {
		return ""
	}
	return fmt.Sprintf("pypi:%s", packageName)
}

// fetchToolMetadata fetches release metadata for a tool using shared registry
// Attempts to determine repo from tool artifacts or name
// Uses GetLatestMetadata when version is unavailable
func fetchToolMetadata(tool Tool, reg *metadata.Registry) (*metadata.Metadata, error) {
	// Try to extract repo from artifacts
	var repo, version string
	if tool.Artifacts != nil {
		// For artifact-based tools, try to extract repo from artifact URLs
		// Example: "https://github.com/anchore/syft/releases/download/v1.33.0/syft_1.33.0_darwin_arm64.tar.gz"
		repo, version = extractRepoFromArtifacts(tool.Artifacts)
	}

	// Infer repo from install identity
	if repo == "" {
		if goRepo := inferRepoFromGoInstallPackage(tool.InstallPackage); goRepo != "" {
			repo = goRepo
		}
	}
	if repo == "" {
		if pypiRepo := inferRepoFromPythonInstall(tool.InstallCommands); pypiRepo != "" {
			repo = pypiRepo
		}
	}

	// If we couldn't infer, try common patterns based on tool name
	if repo == "" {
		repo = guessRepoFromToolName(tool.Name)
	}

	// If still no repo, we can't check cooling policy
	if repo == "" {
		return nil, fmt.Errorf("unable to determine repository for tool %s (no artifacts or known mapping)", tool.Name)
	}

	// If version is still empty, try to use recommended version from tool config
	if version == "" && tool.RecommendedVersion != "" {
		version = tool.RecommendedVersion
	}

	// Fetch metadata
	var meta *metadata.Metadata
	var err error

	if version == "" {
		// No version available - fetch latest release
		// This ensures we actually enforce cooling policy instead of silently bypassing
		logger.Debug(fmt.Sprintf("No version specified for %s, fetching latest release", tool.Name))
		meta, err = (*reg).GetLatestMetadata(repo)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch latest metadata for %s: %w (hint: add recommended_version to tool config)", repo, err)
		}
	} else {
		// Version specified - fetch specific release
		meta, err = (*reg).GetMetadata(repo, version)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch metadata for %s@%s: %w", repo, version, err)
		}
	}

	return meta, nil
}

// extractRepoFromArtifacts attempts to extract GitHub repo and version from artifact URLs
func extractRepoFromArtifacts(artifacts *pkgtools.ArtifactManifest) (string, string) {
	if artifacts == nil || len(artifacts.Versions) == 0 {
		return "", ""
	}

	// Get first version's first artifact URL
	for ver, verArtifacts := range artifacts.Versions {
		if verArtifacts.DarwinAMD64 != nil && verArtifacts.DarwinAMD64.URL != "" {
			repo := extractRepoFromURL(verArtifacts.DarwinAMD64.URL)
			if repo != "" {
				return repo, ver
			}
		}
		if verArtifacts.LinuxAMD64 != nil && verArtifacts.LinuxAMD64.URL != "" {
			repo := extractRepoFromURL(verArtifacts.LinuxAMD64.URL)
			if repo != "" {
				return repo, ver
			}
		}
	}

	return "", ""
}

// extractRepoFromURL extracts "owner/repo" from GitHub release URL
// Example: "https://github.com/anchore/syft/releases/download/v1.33.0/..." → "anchore/syft"
func extractRepoFromURL(url string) string {
	if !strings.Contains(url, "github.com") {
		return ""
	}

	// Split by "github.com/"
	parts := strings.Split(url, "github.com/")
	if len(parts) < 2 {
		return ""
	}

	// Get path after github.com/
	path := parts[1]

	// Split by "/" and take first two parts (owner/repo)
	pathParts := strings.Split(path, "/")
	if len(pathParts) < 2 {
		return ""
	}

	return fmt.Sprintf("%s/%s", pathParts[0], pathParts[1])
}

// Package-level cache: parsed map cached at init to avoid reparsing YAML on every cooling check
var commonReposCache map[string]string

func init() {
	var err error
	commonReposCache, err = loadCommonToolsRepos()
	if err != nil {
		logger.Warn("Failed to load common tools repos config, using hardcoded fallback", logger.String("error", err.Error()))
		commonReposCache = getHardcodedRepos()
	}
}

// loadCommonToolsRepos loads the common tools repository mappings from embedded config
// Uses existing fs.ReadFile pattern (same as used for schemas/config elsewhere)
func loadCommonToolsRepos() (map[string]string, error) {
	// Load from embedded assets using existing pattern
	// Pattern: fs.ReadFile(assets.Config, ...) is used throughout goneat for embedded content
	data, err := fs.ReadFile(assets.Config, "embedded_config/config/tools/common-tools-repos.yaml")
	if err != nil {
		// Fallback to hardcoded map for backward compatibility during transition
		return getHardcodedRepos(), nil
	}

	var config struct {
		Version      string            `yaml:"version"`
		Repositories map[string]string `yaml:"repositories"`
	}

	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse common-tools-repos.yaml: %w", err)
	}

	return config.Repositories, nil
}

// getHardcodedRepos provides fallback for backward compatibility
// These are the same mappings as in config/tools/common-tools-repos.yaml
func getHardcodedRepos() map[string]string {
	return map[string]string{
		"cosign":        "sigstore/cosign",
		"gitleaks":      "zricethezav/gitleaks",
		"golangci-lint": "golangci/golangci-lint",
		"gosec":         "securego/gosec",
		"grype":         "anchore/grype",
		"jq":            "jqlang/jq",
		"prettier":      "prettier/prettier",
		"ripgrep":       "BurntSushi/ripgrep",
		"shellcheck":    "koalaman/shellcheck",
		"shfmt":         "mvdan/sh",
		"syft":          "anchore/syft",
		"trivy":         "aquasecurity/trivy",
		"yamlfmt":       "google/yamlfmt",
		"yq":            "mikefarah/yq",
	}
}

// guessRepoFromToolName attempts common GitHub repo patterns
func guessRepoFromToolName(toolName string) string {
	if repo, ok := commonReposCache[toolName]; ok {
		return repo
	}

	return ""
}

// metadataToDependency converts tool metadata to dependency format for cooling checker
func metadataToDependency(name string, meta *metadata.Metadata) *types.Dependency {
	ageDays := int(time.Since(meta.PublishDate).Hours() / 24)

	dep := &types.Dependency{
		Module: types.Module{
			Name:    name,
			Version: meta.Version,
		},
		Metadata: map[string]interface{}{
			"age_days":     ageDays,
			"publish_date": meta.PublishDate,
			"source":       meta.Source,
		},
	}

	// Add download counts if available
	if meta.TotalDownloads >= 0 {
		dep.Metadata["total_downloads"] = meta.TotalDownloads
	}
	if meta.RecentDownloads >= 0 {
		dep.Metadata["recent_downloads"] = meta.RecentDownloads
	}

	return dep
}

// configToChecker converts pkgtools.CoolingConfig to config.CoolingConfig
func configToChecker(tc pkgtools.CoolingConfig) config.CoolingConfig {
	// Convert exceptions
	exceptions := make([]config.CoolingException, len(tc.Exceptions))
	for i, ex := range tc.Exceptions {
		exceptions[i] = config.CoolingException{
			Pattern:    ex.Pattern,
			Reason:     ex.Reason,
			Until:      ex.Until,
			ApprovedBy: ex.ApprovedBy,
		}
	}

	return config.CoolingConfig{
		Enabled:            tc.Enabled,
		MinAgeDays:         tc.MinAgeDays,
		MinDownloads:       tc.MinDownloads,
		MinDownloadsRecent: tc.MinDownloadsRecent,
		Exceptions:         exceptions,
		AlertOnly:          tc.AlertOnly,
		GracePeriodDays:    tc.GracePeriodDays,
	}
}

// FormatCoolingViolation formats a cooling violation for user display
func FormatCoolingViolation(toolName string, result *CoolingCheckResult) string {
	if result.Passed {
		return ""
	}

	var msgs []string
	for _, v := range result.Violations {
		msgs = append(msgs, fmt.Sprintf("  • %s", v.Message))
	}

	header := fmt.Sprintf("❌ Cooling policy violation for %s:", toolName)
	if result.InGracePeriod {
		header = fmt.Sprintf("⚠️  Cooling policy violation for %s (grace period active):", toolName)
	}

	return fmt.Sprintf("%s\n%s", header, strings.Join(msgs, "\n"))
}
