package doctor

import (
	"fmt"
	"io/fs"
	"os/exec"
	"runtime"
	"strings"

	"github.com/fulmenhq/goneat/internal/assets"
	"gopkg.in/yaml.v3"
)

// PackageManager represents a package manager with its capabilities and requirements
type PackageManager struct {
	Name                  string            `yaml:"name"`
	Description           string            `yaml:"description"`
	Platforms             []string          `yaml:"platforms"`
	RequiresSudo          interface{}       `yaml:"requires_sudo"` // bool or map[string]bool
	RequiresPathUpdate    bool              `yaml:"requires_path_update"`
	PathUpdateAutomatic   bool              `yaml:"path_update_automatic,omitempty"`
	PathActivation        map[string]string `yaml:"path_activation,omitempty"`
	RequiresPrerequisites []string          `yaml:"requires_prerequisites,omitempty"`
	SafeForLanguages      []string          `yaml:"safe_for_languages"`
	InstallMethod         string            `yaml:"install_method"`
	InstallCommand        interface{}       `yaml:"install_command,omitempty"` // string or map[string]string
	AutoInstallSafe       interface{}       `yaml:"auto_install_safe"`         // bool or map[string]bool
	DetectionCommand      string            `yaml:"detection_command,omitempty"`
	Priority              int               `yaml:"priority,omitempty"`
	Recommended           interface{}       `yaml:"recommended,omitempty"` // bool or map[string]bool
	Notes                 string            `yaml:"notes,omitempty"`

	// Runtime state
	Installed bool   `yaml:"-"`
	Version   string `yaml:"-"`
}

// PackageManagersConfig represents the foundation-package-managers.yaml structure
type PackageManagersConfig struct {
	Version           string                       `yaml:"version"`
	PackageManagers   []PackageManager             `yaml:"package_managers"`
	RepoTypeDetection map[string]RepoDetectionRule `yaml:"repo_type_detection,omitempty"`
	Recommendations   map[string]PlatformRec       `yaml:"recommendations,omitempty"`
}

// RepoDetectionRule defines files that indicate a repository type
type RepoDetectionRule struct {
	Files    []string `yaml:"files"`
	Priority int      `yaml:"priority"`
}

// PlatformRec contains platform-specific package manager recommendations
type PlatformRec struct {
	Primary  []string `yaml:"primary"`
	Fallback []string `yaml:"fallback,omitempty"`
	Avoid    []string `yaml:"avoid,omitempty"`
}

// LoadPackageManagersConfig loads foundation-package-managers.yaml from embedded assets
func LoadPackageManagersConfig() (*PackageManagersConfig, error) {
	configFS := assets.GetConfigFS()
	data, err := fs.ReadFile(configFS, "config/tools/foundation-package-managers.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to read package managers config: %w", err)
	}

	var config PackageManagersConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse package managers config: %w", err)
	}

	return &config, nil
}

// GetBoolForPlatform extracts a boolean value for the current platform from interface{}
// Handles both simple bool and platform-specific map[string]bool
func GetBoolForPlatform(value interface{}, platform string) bool {
	if value == nil {
		return false
	}

	switch v := value.(type) {
	case bool:
		return v
	case map[string]interface{}:
		if platformValue, ok := v[platform]; ok {
			if boolVal, ok := platformValue.(bool); ok {
				return boolVal
			}
		}
		return false
	default:
		return false
	}
}

// GetStringForPlatform extracts a string value for the current platform from interface{}
// Handles both simple string and platform-specific map[string]string
func GetStringForPlatform(value interface{}, platform string) string {
	if value == nil {
		return ""
	}

	switch v := value.(type) {
	case string:
		return v
	case map[string]interface{}:
		if platformValue, ok := v[platform]; ok {
			if strVal, ok := platformValue.(string); ok {
				return strVal
			}
		}
		return ""
	default:
		return ""
	}
}

// RequiresSudoOnPlatform checks if the package manager requires sudo on the given platform
func (pm *PackageManager) RequiresSudoOnPlatform(platform string) bool {
	return GetBoolForPlatform(pm.RequiresSudo, platform)
}

// IsAutoInstallSafeOnPlatform checks if the package manager can be safely auto-installed
func (pm *PackageManager) IsAutoInstallSafeOnPlatform(platform string) bool {
	return GetBoolForPlatform(pm.AutoInstallSafe, platform)
}

// IsRecommendedOnPlatform checks if the package manager is recommended on the given platform
func (pm *PackageManager) IsRecommendedOnPlatform(platform string) bool {
	return GetBoolForPlatform(pm.Recommended, platform)
}

// GetInstallCommandForPlatform gets the install command for the given platform
func (pm *PackageManager) GetInstallCommandForPlatform(platform string) string {
	return GetStringForPlatform(pm.InstallCommand, platform)
}

// SupportsPlatform checks if the package manager supports the given platform
func (pm *PackageManager) SupportsPlatform(platform string) bool {
	if len(pm.Platforms) == 0 {
		return true // No platform restriction
	}
	for _, p := range pm.Platforms {
		if p == platform {
			return true
		}
	}
	return false
}

// SupportsLanguage checks if the package manager can safely install tools for the given language
func (pm *PackageManager) SupportsLanguage(language string) bool {
	for _, lang := range pm.SafeForLanguages {
		if lang == "all" || lang == language {
			return true
		}
	}
	return false
}

// DetectPackageManager checks if a package manager is installed and gets its version
func DetectPackageManager(pm *PackageManager) (bool, string) {
	if pm.DetectionCommand == "" {
		return false, ""
	}

	// Split detection command (e.g., "bun --version")
	parts := strings.Fields(pm.DetectionCommand)
	if len(parts) == 0 {
		return false, ""
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, ""
	}

	version := strings.TrimSpace(string(output))
	// Extract just the first line for cleaner display
	if idx := strings.Index(version, "\n"); idx > 0 {
		version = version[:idx]
	}

	return true, version
}

// DetectAllPackageManagers detects which package managers are installed
func DetectAllPackageManagers(config *PackageManagersConfig) []PackageManager {
	result := make([]PackageManager, 0, len(config.PackageManagers))
	platform := runtime.GOOS

	for _, pm := range config.PackageManagers {
		if !pm.SupportsPlatform(platform) {
			continue
		}

		pmCopy := pm
		pmCopy.Installed, pmCopy.Version = DetectPackageManager(&pm)
		result = append(result, pmCopy)
	}

	return result
}

// GetRecommendedPackageManagers returns recommended package managers for the current platform
func GetRecommendedPackageManagers(config *PackageManagersConfig) []PackageManager {
	platform := runtime.GOOS
	result := make([]PackageManager, 0)

	for _, pm := range config.PackageManagers {
		if !pm.SupportsPlatform(platform) {
			continue
		}

		if pm.IsRecommendedOnPlatform(platform) {
			pmCopy := pm
			pmCopy.Installed, pmCopy.Version = DetectPackageManager(&pm)
			result = append(result, pmCopy)
		}
	}

	return result
}

// GetSafePackageManagersForLanguage returns safe package managers for a given language/repo type
func GetSafePackageManagersForLanguage(config *PackageManagersConfig, language string) []PackageManager {
	platform := runtime.GOOS
	result := make([]PackageManager, 0)

	for _, pm := range config.PackageManagers {
		if !pm.SupportsPlatform(platform) {
			continue
		}

		if !pm.SupportsLanguage(language) {
			continue
		}

		// Exclude package managers that require sudo on this platform
		if pm.RequiresSudoOnPlatform(platform) {
			continue
		}

		pmCopy := pm
		pmCopy.Installed, pmCopy.Version = DetectPackageManager(&pm)
		result = append(result, pmCopy)
	}

	return result
}
