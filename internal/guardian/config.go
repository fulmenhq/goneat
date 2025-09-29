package guardian

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fulmenhq/goneat/pkg/config"
	"gopkg.in/yaml.v3"
)

const (
	guardianDirName  = "guardian"
	configFileName   = "config.yaml"
	grantsDirName    = "grants"
	auditLogFileName = "audit.log"
)

var (
	errInvalidConfigVersion = errors.New("unsupported guardian config version")
)

// ConfigPath returns the absolute path to the guardian configuration file.
func ConfigPath() (string, error) {
	dir, err := ensureGuardianDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, configFileName), nil
}

// GrantsDir returns the directory where grant tokens reside.
func GrantsDir() (string, error) {
	dir, err := ensureGuardianDir()
	if err != nil {
		return "", err
	}
	grantsDir := filepath.Join(dir, grantsDirName)
	if err := os.MkdirAll(grantsDir, 0o700); err != nil {
		return "", fmt.Errorf("failed to create guardian grants directory: %w", err)
	}
	return grantsDir, nil
}

// AuditLogPath returns the path to the guardian audit log file.
func AuditLogPath() (string, error) {
	dir, err := ensureGuardianDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, auditLogFileName), nil
}

// ensureGuardianDir ensures the guardian home directory exists and returns it.
func ensureGuardianDir() (string, error) {
	home, err := config.EnsureGoneatHome()
	if err != nil {
		return "", err
	}

	guardianDir := filepath.Join(home, guardianDirName)
	if err := os.MkdirAll(guardianDir, 0o750); err != nil {
		return "", fmt.Errorf("failed to create guardian directory: %w", err)
	}
	return guardianDir, nil
}

// EnsureConfig ensures a default config file exists and returns the resolved configuration path.
func EnsureConfig() (string, error) {
	path, err := ConfigPath()
	if err != nil {
		return "", err
	}

	if _, statErr := os.Stat(path); statErr == nil {
		return path, nil
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return "", fmt.Errorf("failed to stat guardian config: %w", statErr)
	}

	if err := os.WriteFile(path, []byte(defaultConfigYAML), 0o600); err != nil {
		return "", fmt.Errorf("failed to write default guardian config: %w", err)
	}

	return path, nil
}

// LoadConfig loads guardian configuration from disk, creating defaults if necessary.
func LoadConfig() (*ConfigRoot, error) {
	path, err := EnsureConfig()
	if err != nil {
		return nil, err
	}

	raw, err := os.ReadFile(path) // #nosec G304 -- path from EnsureConfig in guardian directory
	if err != nil {
		return nil, fmt.Errorf("failed to read guardian config: %w", err)
	}

	var cfg ConfigRoot
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse guardian config: %w", err)
	}

	if cfg.Guardian.Version != "" && cfg.Guardian.Version != ConfigVersion {
		return nil, fmt.Errorf("%w: got %s, expected %s", errInvalidConfigVersion, cfg.Guardian.Version, ConfigVersion)
	}

	// Apply defaults if values omitted.
	cfg.applyDefaults()
	return &cfg, nil
}

func (cfg *ConfigRoot) applyDefaults() {
	if cfg.Guardian.Version == "" {
		cfg.Guardian.Version = ConfigVersion
	}
	if cfg.Guardian.Defaults.Method == "" {
		cfg.Guardian.Defaults.Method = MethodBrowser
	}
	if cfg.Guardian.Defaults.Expires == "" {
		cfg.Guardian.Defaults.Expires = "30m"
	}
	if cfg.Guardian.Scopes == nil {
		cfg.Guardian.Scopes = make(map[string]ScopePolicy)
	}
}

// ResolvePolicy returns the effective policy for a scope/operation combination.
func (cfg *ConfigRoot) ResolvePolicy(scope, operation string) (*ResolvedPolicy, bool, error) {
	sp, ok := cfg.Guardian.Scopes[scope]
	if !ok {
		return nil, false, nil
	}
	op, ok := sp.Operations[operation]
	if !ok {
		return nil, false, nil
	}
	if !op.Enabled {
		return nil, false, nil
	}

	method := op.Method
	if method == "" {
		method = cfg.Guardian.Defaults.Method
	}
	expires := op.Expires
	if expires == "" {
		expires = cfg.Guardian.Defaults.Expires
	}
	d, err := time.ParseDuration(expires)
	if err != nil {
		return nil, true, fmt.Errorf("invalid expires duration %q for %s.%s: %w", expires, scope, operation, err)
	}

	requireReason := cfg.Guardian.Defaults.RequireReason
	if op.RequireReason != nil {
		requireReason = *op.RequireReason
	}

	opCopy := op

	rp := &ResolvedPolicy{
		Scope:         scope,
		Operation:     operation,
		Method:        method,
		Expires:       d,
		RequireReason: requireReason,
		Risk:          op.Risk,
		Conditions:    op.Conditions,
		Raw:           &opCopy,
	}
	return rp, true, nil
}

// defaultConfigYAML contains the bootstrap configuration written on first run.
const defaultConfigYAML = `guardian:
  version: "1.0.0"
  defaults:
    method: "browser"
    expires: "30m"
    require_reason: false
    audit_all: true
  scopes:
    git:
      description: "Repository operations protection"
      operations:
        commit:
          enabled: true
          method: "browser"
          expires: "10m"
          require_reason: false
          risk: "high"
          conditions:
            branches: ["main", "master"]
        push:
          enabled: true
          method: "browser"
          expires: "15m"
          require_reason: true
          risk: "critical"
          conditions:
            branches: ["main", "master", "release/*"]
            remote_patterns: ["origin", "upstream"]
  security:
    encryption:
      enabled: true
      algorithm: "AES-256-GCM"
      key_rotation_days: 30
    audit:
      enabled: true
      retention_days: 365
      include_context: true
    browser_approval:
      timeout_seconds: 300
      port_range: [49152, 65535]
      localhost_only: true
      auto_open_browser: true
      show_url_in_terminal: true
    grants:
      max_duration: "4h"
      max_concurrent: 10
      auto_cleanup: true
    branding:
      project_name: ""
      logo_path: ""
      custom_message: ""
  integrations:
    hooks:
      auto_install: false
      backup_existing: true
      verify_integrity: true
`
