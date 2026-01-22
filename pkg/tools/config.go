package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fulmenhq/goneat/pkg/schema"
	"github.com/fulmenhq/goneat/pkg/versioning"
	"gopkg.in/yaml.v3"
)

// Config represents the complete tools configuration (scopes + tool definitions).
type Config struct {
	Scopes map[string]Scope `yaml:"scopes" json:"scopes"`
	Tools  map[string]Tool  `yaml:"tools" json:"tools"`
}

// Scope represents a logical grouping of tools.
type Scope struct {
	Description string   `yaml:"description" json:"description"`
	Tools       []string `yaml:"tools" json:"tools"`
	Replace     bool     `yaml:"replace,omitempty" json:"replace,omitempty"`
}

// Tool represents a single tool definition.
type Tool struct {
	Name              string              `yaml:"name" json:"name"`
	Description       string              `yaml:"description" json:"description"`
	Kind              string              `yaml:"kind" json:"kind"`
	DetectCommand     string              `yaml:"detect_command" json:"detect_command"`
	Install           *InstallConfig      `yaml:"install,omitempty" json:"install,omitempty"`                 // v1.1.0+: structured installation
	InstallPackage    string              `yaml:"install_package,omitempty" json:"install_package,omitempty"` // Go package for "go" kind tools
	VersionArgs       []string            `yaml:"version_args,omitempty" json:"version_args,omitempty"`
	CheckArgs         []string            `yaml:"check_args,omitempty" json:"check_args,omitempty"`
	Platforms         []string            `yaml:"platforms,omitempty" json:"platforms,omitempty"`
	InstallCommands   map[string]string   `yaml:"install_commands,omitempty" json:"install_commands,omitempty"` // v1.0.0 legacy
	InstallerPriority map[string][]string `yaml:"installer_priority,omitempty" json:"installer_priority,omitempty"`
	VersionScheme     string              `yaml:"version_scheme,omitempty" json:"version_scheme,omitempty"`
	// MinVersion is a deprecated alias for MinimumVersion.
	//
	// Kept for backwards compatibility with configs that used min_version.
	// Prefer MinimumVersion going forward.
	MinVersion         string            `yaml:"min_version,omitempty" json:"min_version,omitempty"`
	MinimumVersion     string            `yaml:"minimum_version,omitempty" json:"minimum_version,omitempty"`
	RecommendedVersion string            `yaml:"recommended_version,omitempty" json:"recommended_version,omitempty"`
	DisallowedVersions []string          `yaml:"disallowed_versions,omitempty" json:"disallowed_versions,omitempty"`
	Artifacts          *ArtifactManifest `yaml:"artifacts,omitempty" json:"artifacts,omitempty"`
	Cooling            *CoolingConfig    `yaml:"cooling,omitempty" json:"cooling,omitempty"` // v1.2.0: optional tool-specific cooling policy override
}

// InstallConfig defines structured installation methods (v1.1.0+).
// Supports package managers (brew, scoop), downloads, and scripts.
type InstallConfig struct {
	Type           string                 `yaml:"type" json:"type"` // package_manager, download, script
	PackageManager *PackageManagerInstall `yaml:"package_manager,omitempty" json:"package_manager,omitempty"`
	// Future: Download, Script configs
}

// PackageManagerInstall defines installation via package managers (brew, scoop, etc.).
type PackageManagerInstall struct {
	Manager     string   `yaml:"manager" json:"manager"`                               // brew, scoop
	Tap         string   `yaml:"tap,omitempty" json:"tap,omitempty"`                   // Homebrew tap (brew-only)
	Bucket      string   `yaml:"bucket,omitempty" json:"bucket,omitempty"`             // Scoop bucket (scoop-only)
	Package     string   `yaml:"package" json:"package"`                               // Package name
	PackageType string   `yaml:"package_type,omitempty" json:"package_type,omitempty"` // formula|cask (brew-only, default: formula)
	Flags       []string `yaml:"flags,omitempty" json:"flags,omitempty"`               // Additional CLI flags
	Destination string   `yaml:"destination,omitempty" json:"destination,omitempty"`   // Symlink destination
	BinName     string   `yaml:"bin_name,omitempty" json:"bin_name,omitempty"`         // Binary name override
}

// ArtifactManifest defines trusted artifacts with SHA256 verification for supply-chain integrity.
type ArtifactManifest struct {
	DefaultVersion string                      `yaml:"default_version" json:"default_version"`
	Versions       map[string]VersionArtifacts `yaml:"versions" json:"versions"`
}

// VersionArtifacts holds platform-specific artifacts for a single version.
type VersionArtifacts struct {
	DarwinAMD64  *Artifact `yaml:"darwin_amd64,omitempty" json:"darwin_amd64,omitempty"`
	DarwinARM64  *Artifact `yaml:"darwin_arm64,omitempty" json:"darwin_arm64,omitempty"`
	LinuxAMD64   *Artifact `yaml:"linux_amd64,omitempty" json:"linux_amd64,omitempty"`
	LinuxARM64   *Artifact `yaml:"linux_arm64,omitempty" json:"linux_arm64,omitempty"`
	WindowsAMD64 *Artifact `yaml:"windows_amd64,omitempty" json:"windows_amd64,omitempty"`
}

// Artifact represents a single downloadable artifact with integrity verification.
type Artifact struct {
	URL         string `yaml:"url" json:"url"`
	SHA256      string `yaml:"sha256" json:"sha256"`
	ExtractPath string `yaml:"extract_path,omitempty" json:"extract_path,omitempty"`
}

// CoolingConfig defines package cooling policy for supply chain security.
// Tool-specific cooling config can override global defaults from dependencies.yaml.
type CoolingConfig struct {
	Enabled            bool               `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	MinAgeDays         int                `yaml:"min_age_days,omitempty" json:"min_age_days,omitempty"`
	MinDownloads       int                `yaml:"min_downloads,omitempty" json:"min_downloads,omitempty"`
	MinDownloadsRecent int                `yaml:"min_downloads_recent,omitempty" json:"min_downloads_recent,omitempty"`
	Exceptions         []CoolingException `yaml:"exceptions,omitempty" json:"exceptions,omitempty"`
	AlertOnly          bool               `yaml:"alert_only,omitempty" json:"alert_only,omitempty"`
	GracePeriodDays    int                `yaml:"grace_period_days,omitempty" json:"grace_period_days,omitempty"`
}

// CoolingException represents a trusted package that can bypass cooling period.
type CoolingException struct {
	Pattern    string `yaml:"pattern,omitempty" json:"pattern,omitempty"`
	Reason     string `yaml:"reason,omitempty" json:"reason,omitempty"`
	Until      string `yaml:"until,omitempty" json:"until,omitempty"`
	ApprovedBy string `yaml:"approved_by,omitempty" json:"approved_by,omitempty"`
}

// ParseConfig parses YAML bytes into a Config structure.
func ParseConfig(data []byte) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("yaml parse error: %w", err)
	}
	return &cfg, nil
}

// ValidateBytes validates raw YAML/JSON content against the tools schema.
// Validates against v1.1.0 schema which is backward compatible with v1.0.0.
func ValidateBytes(data []byte) error {
	var cfg interface{}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("yaml parse error: %w", err)
	}
	// v1.1.0 is backward compatible with v1.0.0 (all new fields are optional)
	// so we validate against v1.1.0 for all configs
	res, err := schema.Validate(cfg, "tools-config-v1.1.0")
	if err != nil {
		return fmt.Errorf("schema validation error: %w", err)
	}
	if !res.Valid {
		details := make([]string, 0, len(res.Errors))
		for _, e := range res.Errors {
			details = append(details, fmt.Sprintf("%s: %s", e.Path, e.Message))
		}
		return fmt.Errorf("invalid config:\n%s", joinLines(details))
	}
	return nil
}

// ValidateFile validates a configuration file on disk.
func ValidateFile(path string) error {
	if path == "" {
		return fmt.Errorf("config path cannot be empty")
	}
	clean := filepath.Clean(path)
	if containsTraversal(clean) {
		return fmt.Errorf("config path contains invalid path traversal")
	}
	data, err := os.ReadFile(clean)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}
	return ValidateBytes(data)
}

// Merge merges another configuration into the receiver (mutating in place).
// Later definitions win when conflicts arise.
func (c *Config) Merge(other *Config) {
	if other == nil {
		return
	}
	if c.Scopes == nil {
		c.Scopes = make(map[string]Scope)
	}
	for name, scope := range other.Scopes {
		existing, exists := c.Scopes[name]
		if !exists || scope.Replace {
			scope.Replace = false
			c.Scopes[name] = scope
			continue
		}

		merged := existing
		if scope.Description != "" {
			merged.Description = scope.Description
		}
		merged.Tools = appendUnique(existing.Tools, scope.Tools)
		merged.Replace = false
		c.Scopes[name] = merged
	}
	if c.Tools == nil {
		c.Tools = make(map[string]Tool)
	}
	for name, tool := range other.Tools {
		c.Tools[name] = tool
	}
}

// GetToolsForScope resolves concrete tool definitions for a given scope.
func (c *Config) GetToolsForScope(scopeName string) ([]Tool, error) {
	scope, ok := c.Scopes[scopeName]
	if !ok {
		return nil, fmt.Errorf("scope '%s' not found", scopeName)
	}
	tools := make([]Tool, 0, len(scope.Tools))
	for _, toolName := range scope.Tools {
		toolDef, exists := c.Tools[toolName]
		if !exists {
			return nil, fmt.Errorf("tool '%s' referenced in scope '%s' but not defined", toolName, scopeName)
		}
		tools = append(tools, toolDef)
	}
	return tools, nil
}

// GetTool returns a tool definition by name.
func (c *Config) GetTool(name string) (Tool, bool) {
	t, ok := c.Tools[name]
	return t, ok
}

// GetAllScopes lists defined scope names (unordered).
func (c *Config) GetAllScopes() []string {
	scopes := make([]string, 0, len(c.Scopes))
	for name := range c.Scopes {
		scopes = append(scopes, name)
	}
	return scopes
}

// VersionPolicy converts the tool's version fields into a reusable policy object.
func (t Tool) VersionPolicy() (versioning.Policy, error) {
	if strings.TrimSpace(t.MinVersion) != "" && strings.TrimSpace(t.MinimumVersion) != "" {
		return versioning.Policy{}, fmt.Errorf("both min_version and minimum_version are set; use only minimum_version")
	}
	if t.VersionScheme == "" && t.MinVersion == "" && t.MinimumVersion == "" && t.RecommendedVersion == "" && len(t.DisallowedVersions) == 0 {
		return versioning.Policy{Scheme: versioning.SchemeLexical}, nil
	}
	scheme, err := parseScheme(t.VersionScheme)
	if err != nil {
		return versioning.Policy{}, err
	}
	cleanDisallowed := make([]string, 0, len(t.DisallowedVersions))
	for _, v := range t.DisallowedVersions {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			cleanDisallowed = append(cleanDisallowed, trimmed)
		}
	}
	minVersion := strings.TrimSpace(t.MinimumVersion)
	if minVersion == "" {
		minVersion = strings.TrimSpace(t.MinVersion)
	}
	policy := versioning.Policy{
		Scheme:             scheme,
		MinimumVersion:     minVersion,
		RecommendedVersion: strings.TrimSpace(t.RecommendedVersion),
		DisallowedVersions: cleanDisallowed,
	}
	return policy, nil
}

func appendUnique(base, extras []string) []string {
	if len(extras) == 0 {
		return append([]string(nil), base...)
	}
	seen := make(map[string]struct{}, len(base))
	result := append([]string(nil), base...)
	for _, item := range base {
		seen[item] = struct{}{}
	}
	for _, item := range extras {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}

func parseScheme(raw string) (versioning.Scheme, error) {
	cleaned := strings.TrimSpace(strings.ToLower(raw))
	if cleaned == "" {
		return versioning.SchemeLexical, nil
	}
	switch cleaned {
	case string(versioning.SchemeSemverLegacy), string(versioning.SchemeSemverFull):
		return versioning.SchemeSemverFull, nil
	case string(versioning.SchemeSemverCompact):
		return versioning.SchemeSemverCompact, nil
	case string(versioning.SchemeCalver):
		return versioning.SchemeCalver, nil
	case string(versioning.SchemeLexical):
		return versioning.SchemeLexical, nil
	default:
		return "", fmt.Errorf("unsupported version scheme: %s", raw)
	}
}

func joinLines(lines []string) string {
	return strings.Join(lines, "\n")
}

func containsTraversal(path string) bool {
	cleaned := filepath.Clean(path)
	if cleaned == ".." {
		return true
	}
	if strings.HasPrefix(cleaned, "../") || strings.HasPrefix(cleaned, "..\\") {
		return true
	}
	sep := string(filepath.Separator)
	needle := sep + ".." + sep
	return strings.Contains(cleaned, needle)
}

// LoadGlobalCoolingConfig loads the global cooling policy from .goneat/dependencies.yaml.
// Returns nil if the file doesn't exist or cooling section is not configured.
func LoadGlobalCoolingConfig() (*CoolingConfig, error) {
	dependenciesPath := ".goneat/dependencies.yaml"
	if _, err := os.Stat(dependenciesPath); os.IsNotExist(err) {
		return nil, nil // No global config - OK
	}

	data, err := os.ReadFile(dependenciesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", dependenciesPath, err)
	}

	// Parse YAML to extract just the cooling section
	var doc struct {
		Cooling *CoolingConfig `yaml:"cooling"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", dependenciesPath, err)
	}

	return doc.Cooling, nil
}

// MergeCoolingConfig merges tool-specific cooling config with global defaults.
// Tool-specific settings override global defaults. If tool config is nil, returns global.
// If both are nil, returns default enabled config with 7-day cooling.
func MergeCoolingConfig(global, toolSpecific *CoolingConfig) *CoolingConfig {
	// If no tool-specific override, use global
	if toolSpecific == nil {
		if global != nil {
			return global
		}
		// Return sensible defaults if no configuration at all
		return &CoolingConfig{
			Enabled:            true,
			MinAgeDays:         7,
			MinDownloads:       100,
			MinDownloadsRecent: 10,
			AlertOnly:          false,
			GracePeriodDays:    3,
		}
	}

	// Tool-specific exists - merge with global as base
	merged := &CoolingConfig{}
	if global != nil {
		*merged = *global // Copy global as baseline
	} else {
		// No global config, use defaults as base
		merged.Enabled = true
		merged.MinAgeDays = 7
		merged.MinDownloads = 100
		merged.MinDownloadsRecent = 10
		merged.AlertOnly = false
		merged.GracePeriodDays = 3
	}

	// Override with tool-specific values
	// Note: For numeric fields, we treat 0 as "not set" (use global/default)
	// For boolean fields and other types, we always override if tool-specific exists

	// Numeric fields: only override if non-zero (0 means "use default")
	if toolSpecific.MinAgeDays != 0 {
		merged.MinAgeDays = toolSpecific.MinAgeDays
	}
	if toolSpecific.MinDownloads != 0 {
		merged.MinDownloads = toolSpecific.MinDownloads
	}
	if toolSpecific.MinDownloadsRecent != 0 {
		merged.MinDownloadsRecent = toolSpecific.MinDownloadsRecent
	}
	if toolSpecific.GracePeriodDays != 0 {
		merged.GracePeriodDays = toolSpecific.GracePeriodDays
	}

	// Arrays: override if specified
	if len(toolSpecific.Exceptions) > 0 {
		merged.Exceptions = toolSpecific.Exceptions
	}

	// Boolean fields: We need special handling since we can't distinguish false from unset.
	// Solution: If tool-specific config is provided, always apply its boolean values.
	// To disable cooling for a tool, explicitly set enabled: false in tool config.
	merged.Enabled = toolSpecific.Enabled
	merged.AlertOnly = toolSpecific.AlertOnly

	return merged
}

// GetEffectiveCoolingConfig returns the effective cooling configuration for a tool.
// It loads global config, merges with tool-specific overrides, and respects --no-cooling flag.
func (t *Tool) GetEffectiveCoolingConfig(disableCooling bool) (*CoolingConfig, error) {
	if disableCooling {
		return &CoolingConfig{Enabled: false}, nil
	}

	globalCooling, err := LoadGlobalCoolingConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load global cooling config: %w", err)
	}

	return MergeCoolingConfig(globalCooling, t.Cooling), nil
}
