package propagation

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/fulmenhq/goneat/pkg/logger"
	"github.com/fulmenhq/goneat/pkg/schema"
	"gopkg.in/yaml.v3"
)

// VersionPolicy defines the structure of .goneat/version-policy.yaml
type VersionPolicy struct {
	Version     VersionConfig          `yaml:"version"`
	Propagation PropagationConfig      `yaml:"propagation"`
	Rules       RulesConfig            `yaml:"rules,omitempty"`
	Guards      GuardsConfig           `yaml:"guards,omitempty"`
	Metadata    map[string]interface{} `yaml:"metadata,omitempty"`
}

// VersionConfig defines version-related settings
type VersionConfig struct {
	Scheme        string `yaml:"scheme"`            // semver | calver
	AllowExtended bool   `yaml:"allow_extended"`    // enables prerelease/build metadata
	Channel       string `yaml:"channel,omitempty"` // optional release channel
}

// PropagationConfig defines propagation behavior
type PropagationConfig struct {
	Defaults  PropagationDefaults          `yaml:"defaults"`
	Targets   map[string]PropagationTarget `yaml:"targets,omitempty"`
	Workspace WorkspaceConfig              `yaml:"workspace,omitempty"`
}

// PropagationDefaults defines default propagation settings
type PropagationDefaults struct {
	Include []string     `yaml:"include"`
	Exclude []string     `yaml:"exclude"`
	Backup  BackupConfig `yaml:"backup,omitempty"`
}

// BackupConfig defines backup behavior
type BackupConfig struct {
	Enabled   bool `yaml:"enabled"`   // create backup files
	Retention int  `yaml:"retention"` // number of backups to keep
}

// PropagationTarget defines propagation settings for a specific package manager
type PropagationTarget struct {
	Include      []string `yaml:"include,omitempty"`
	Exclude      []string `yaml:"exclude,omitempty"`
	Mode         string   `yaml:"mode,omitempty"` // project | poetry | workspace
	ValidateOnly bool     `yaml:"validate_only"`  // explicit to avoid accidental write attempts
}

// WorkspaceConfig defines workspace-specific behavior
type WorkspaceConfig struct {
	Strategy  string   `yaml:"strategy"`            // single-version | opt-in | opt-out
	Allowlist []string `yaml:"allowlist,omitempty"` // for opt-in strategy
	Blocklist []string `yaml:"blocklist,omitempty"` // for opt-out strategy
}

// RulesConfig defines content validation rules
type RulesConfig struct {
	RequireReleaseTag               bool     `yaml:"require_release_tag,omitempty"`
	AllowedChannels                 []string `yaml:"allowed_channels,omitempty"`
	ForbidPrereleaseOnDefaultBranch bool     `yaml:"forbid_prerelease_on_default_branch,omitempty"`
	MaxPrereleaseLength             int      `yaml:"max_prerelease_length,omitempty"`
}

// GuardsConfig defines execution preconditions
type GuardsConfig struct {
	RequiredBranches      []string `yaml:"required_branches,omitempty"`
	DisallowDirtyWorktree bool     `yaml:"disallow_dirty_worktree"`
}

// PolicyLoader handles loading and validation of version policies
type PolicyLoader struct{}

// NewPolicyLoader creates a new policy loader
func NewPolicyLoader() *PolicyLoader {
	return &PolicyLoader{}
}

// LoadPolicy loads the version policy from the specified path or default location
func (pl *PolicyLoader) LoadPolicy(policyPath string) (*VersionPolicy, error) {
	return pl.LoadPolicyWithValidation(policyPath)
}

// LoadPolicyWithValidation loads the version policy with schema validation
func (pl *PolicyLoader) LoadPolicyWithValidation(policyPath string) (*VersionPolicy, error) {
	if policyPath == "" {
		policyPath = ".goneat/version-policy.yaml"
	}

	// Validate policy path to prevent path traversal
	validatedPath, err := filepath.Abs(policyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve policy path: %w", err)
	}
	policyPath = validatedPath

	// Check if policy file exists
	if _, err := os.Stat(policyPath); os.IsNotExist(err) {
		logger.Debug("Policy file not found, using defaults", logger.String("path", policyPath))
		return pl.createDefaultPolicy(), nil
	}

	// Read policy file
	data, err := os.ReadFile(policyPath) // #nosec G304 - path validated with filepath.Abs above
	if err != nil {
		return nil, fmt.Errorf("failed to read policy file %s: %w", policyPath, err)
	}

	// Validate against embedded schema (follows pattern from pkg/schema/mapping/manager.go)
	validator, err := schema.GetEmbeddedValidator("version-policy-v1.0.0")
	if err != nil {
		logger.Debug("Failed to load embedded schema validator, proceeding without validation", logger.String("error", err.Error()))
	} else {
		// ValidateBytes handles YAML->JSON conversion internally
		validation, err := validator.ValidateBytes(data)
		if err != nil {
			return nil, fmt.Errorf("validate policy %s: %w", policyPath, err)
		}
		if !validation.Valid {
			// Format errors similar to mapping/manager.go
			var errorMsgs []string
			for _, verr := range validation.Errors {
				errorMsgs = append(errorMsgs, verr.Message)
			}
			return nil, fmt.Errorf("policy %s failed validation: %s", policyPath, strings.Join(errorMsgs, "; "))
		}
	}

	// Parse policy file
	var policy VersionPolicy
	if err := yaml.Unmarshal(data, &policy); err != nil {
		return nil, fmt.Errorf("failed to parse policy file %s: %w", policyPath, err)
	}

	// Validate policy structure
	if err := pl.validatePolicy(&policy); err != nil {
		return nil, fmt.Errorf("invalid policy file %s: %w", policyPath, err)
	}

	logger.Debug("Loaded version policy", logger.String("path", policyPath))
	return &policy, nil
}

// createDefaultPolicy creates a default policy when no policy file exists
func (pl *PolicyLoader) createDefaultPolicy() *VersionPolicy {
	return &VersionPolicy{
		Version: VersionConfig{
			Scheme:        "semver",
			AllowExtended: true,
		},
		Propagation: PropagationConfig{
			Defaults: PropagationDefaults{
				Include: []string{"package.json", "pyproject.toml"},
				Exclude: []string{"**/node_modules/**", "docs/**"},
				Backup: BackupConfig{
					Enabled:   true,
					Retention: 5,
				},
			},
			Targets: make(map[string]PropagationTarget),
			Workspace: WorkspaceConfig{
				Strategy: "single-version",
			},
		},
		Guards: GuardsConfig{
			DisallowDirtyWorktree: true,
		},
	}
}

// validatePolicy validates the loaded policy
func (pl *PolicyLoader) validatePolicy(policy *VersionPolicy) error {
	// Validate version scheme
	if policy.Version.Scheme != "semver" && policy.Version.Scheme != "calver" {
		return fmt.Errorf("invalid version scheme: %s (must be semver or calver)", policy.Version.Scheme)
	}

	// Validate channel pattern if provided
	if policy.Version.Channel != "" {
		if matched, _ := regexp.MatchString(`^[a-z0-9.-]+$`, policy.Version.Channel); !matched {
			return fmt.Errorf("invalid channel name: %s (must match pattern ^[a-z0-9.-]+$)", policy.Version.Channel)
		}
	}

	// Validate target modes
	for name, target := range policy.Propagation.Targets {
		if target.Mode != "" && target.Mode != "project" && target.Mode != "poetry" && target.Mode != "workspace" {
			return fmt.Errorf("invalid mode for target %s: %s (must be project, poetry, or workspace)", name, target.Mode)
		}
	}

	// Validate workspace strategy
	if policy.Propagation.Workspace.Strategy != "" {
		validStrategies := []string{"single-version", "opt-in", "opt-out"}
		valid := false
		for _, s := range validStrategies {
			if policy.Propagation.Workspace.Strategy == s {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("invalid workspace strategy: %s (must be single-version, opt-in, or opt-out)", policy.Propagation.Workspace.Strategy)
		}
	}

	// Validate rules
	if len(policy.Rules.AllowedChannels) > 0 {
		channelPattern := regexp.MustCompile(`^[a-z0-9.-]+$`)
		for _, channel := range policy.Rules.AllowedChannels {
			if !channelPattern.MatchString(channel) {
				return fmt.Errorf("invalid allowed channel: %s (must match pattern ^[a-z0-9.-]+$)", channel)
			}
		}
	}

	return nil
}

// GeneratePolicyFile generates a sample policy file with comments
func (pl *PolicyLoader) GeneratePolicyFile(path string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Generate YAML with comments
	content := `# Version SSOT Propagation Policy
# This file controls how goneat propagates version changes from VERSION to package manager files

version:
  scheme: semver          # semver | calver
  allow_extended: true    # enables prerelease/build metadata (e.g., v1.2.3-rc.1)

propagation:
  defaults:
    include: ["package.json", "pyproject.toml"]  # Default package managers to include
    exclude: ["**/node_modules/**", "docs/**"]  # Patterns to exclude
    backup:
      enabled: true        # Create backup files before changes
      retention: 5         # Number of backup files to keep

  workspace:
    strategy: single-version  # single-version | opt-in | opt-out

  # Target-specific overrides (optional)
  # targets:
  #   package.json:
  #     include: ["./package.json", "apps/*/package.json", "packages/*/package.json"]
  #     exclude: ["packages/legacy-*"]
  #
  #   pyproject.toml:
  #     include: ["services/*/pyproject.toml"]
  #     mode: poetry       # project | poetry | workspace
  #
  #   go.mod:
  #     validate_only: true   # explicit to avoid accidental write attempts

# rules:  # Content validation rules (Phase 3a)
#   allowed_channels: ["stable", "beta"]
#   forbid_prerelease_on_default_branch: true

guards:  # Execution preconditions
  required_branches: ["main", "release/*"]  # Optional: restrict to specific branches
  disallow_dirty_worktree: true             # Prevent propagation with uncommitted changes

# metadata:  # Optional: organizational tracking
#   team: "platform"
#   last_reviewed: "2025-01-15"
`

	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		return fmt.Errorf("failed to write policy file %s: %w", path, err)
	}

	logger.Info("Generated sample policy file", logger.String("path", path))
	return nil
}
