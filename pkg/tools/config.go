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
	Name               string              `yaml:"name" json:"name"`
	Description        string              `yaml:"description" json:"description"`
	Kind               string              `yaml:"kind" json:"kind"`
	DetectCommand      string              `yaml:"detect_command" json:"detect_command"`
	InstallPackage     string              `yaml:"install_package,omitempty" json:"install_package,omitempty"`
	VersionArgs        []string            `yaml:"version_args,omitempty" json:"version_args,omitempty"`
	CheckArgs          []string            `yaml:"check_args,omitempty" json:"check_args,omitempty"`
	Platforms          []string            `yaml:"platforms,omitempty" json:"platforms,omitempty"`
	InstallCommands    map[string]string   `yaml:"install_commands,omitempty" json:"install_commands,omitempty"`
	InstallerPriority  map[string][]string `yaml:"installer_priority,omitempty" json:"installer_priority,omitempty"`
	VersionScheme      string              `yaml:"version_scheme,omitempty" json:"version_scheme,omitempty"`
	MinimumVersion     string              `yaml:"minimum_version,omitempty" json:"minimum_version,omitempty"`
	RecommendedVersion string              `yaml:"recommended_version,omitempty" json:"recommended_version,omitempty"`
	DisallowedVersions []string            `yaml:"disallowed_versions,omitempty" json:"disallowed_versions,omitempty"`
	Artifacts          *ArtifactManifest   `yaml:"artifacts,omitempty" json:"artifacts,omitempty"`
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

// ParseConfig parses YAML bytes into a Config structure.
func ParseConfig(data []byte) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("yaml parse error: %w", err)
	}
	return &cfg, nil
}

// ValidateBytes validates raw YAML/JSON content against the tools schema.
func ValidateBytes(data []byte) error {
	var cfg interface{}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("yaml parse error: %w", err)
	}
	res, err := schema.Validate(cfg, "tools-config-v1.0.0")
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
	if t.VersionScheme == "" && t.MinimumVersion == "" && t.RecommendedVersion == "" && len(t.DisallowedVersions) == 0 {
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
	policy := versioning.Policy{
		Scheme:             scheme,
		MinimumVersion:     strings.TrimSpace(t.MinimumVersion),
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
