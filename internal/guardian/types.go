package guardian

import "time"

// Method represents the approval method to satisfy a guardian policy.
type Method string

const (
	// MethodBrowser indicates approval is requested through the interactive browser flow.
	MethodBrowser Method = "browser"
	// MethodGrant uses pre-generated tokens (grants) to satisfy the policy.
	MethodGrant Method = "grant"
)

// ConfigVersion defines the supported guardian policy schema version.
const ConfigVersion = "1.0.0"

// ConfigRoot is the root YAML structure stored on disk.
type ConfigRoot struct {
	Guardian GuardianConfig `yaml:"guardian"`
}

// GuardianConfig contains the guardian policy definition.
type GuardianConfig struct {
	Version      string                 `yaml:"version"`
	Defaults     PolicyDefaults         `yaml:"defaults"`
	Scopes       map[string]ScopePolicy `yaml:"scopes"`
	Security     SecuritySettings       `yaml:"security"`
	Integrations IntegrationSettings    `yaml:"integrations"`
}

// PolicyDefaults are global defaults applied to every scope/operation when values are omitted.
type PolicyDefaults struct {
	Method        Method `yaml:"method"`
	Expires       string `yaml:"expires"`
	RequireReason bool   `yaml:"require_reason"`
	AuditAll      bool   `yaml:"audit_all"`
}

// ScopePolicy configures operations within a scope (e.g., git, devops, sql).
type ScopePolicy struct {
	Description string                     `yaml:"description"`
	Operations  map[string]OperationPolicy `yaml:"operations"`
}

// OperationPolicy represents policy controls for a specific operation.
type OperationPolicy struct {
	Enabled       bool                `yaml:"enabled"`
	Method        Method              `yaml:"method"`
	Expires       string              `yaml:"expires"`
	RequireReason *bool               `yaml:"require_reason"`
	Risk          string              `yaml:"risk"`
	Conditions    map[string][]string `yaml:"conditions"`
}

// SecuritySettings configures security primitives for guardian.
type SecuritySettings struct {
	Encryption EncryptionSettings `yaml:"encryption"`
	Audit      AuditSettings      `yaml:"audit"`
	Browser    BrowserSettings    `yaml:"browser_approval"`
	Grants     GrantsSettings     `yaml:"grants"`
	Branding   BrandingSettings   `yaml:"branding"`
}

// EncryptionSettings describes encryption related configuration.
type EncryptionSettings struct {
	Enabled         bool   `yaml:"enabled"`
	Algorithm       string `yaml:"algorithm"`
	KeyRotationDays int    `yaml:"key_rotation_days"`
}

// AuditSettings configures audit logging.
type AuditSettings struct {
	Enabled        bool `yaml:"enabled"`
	RetentionDays  int  `yaml:"retention_days"`
	IncludeContext bool `yaml:"include_context"`
}

// BrowserSettings controls browser approval behaviour.
type BrowserSettings struct {
	TimeoutSeconds int   `yaml:"timeout_seconds"`
	PortRange      []int `yaml:"port_range"`
	LocalhostOnly  bool  `yaml:"localhost_only"`
	AutoOpen       bool  `yaml:"auto_open_browser"`
	ShowURL        bool  `yaml:"show_url_in_terminal"`
}

// GrantsSettings controls grant lifecycle.
type GrantsSettings struct {
	MaxDuration   string `yaml:"max_duration"`
	MaxConcurrent int    `yaml:"max_concurrent"`
	AutoCleanup   bool   `yaml:"auto_cleanup"`
}

// BrandingSettings configures optional UI theming.
type BrandingSettings struct {
	ProjectName   string `yaml:"project_name"`
	LogoPath      string `yaml:"logo_path"`
	CustomMessage string `yaml:"custom_message"`
}

// IntegrationSettings configures optional integration points.
type IntegrationSettings struct {
	Hooks HooksIntegration `yaml:"hooks"`
}

// HooksIntegration controls hook generator defaults for guardian.
type HooksIntegration struct {
	AutoInstall     bool `yaml:"auto_install"`
	BackupExisting  bool `yaml:"backup_existing"`
	VerifyIntegrity bool `yaml:"verify_integrity"`
}

// OperationContext carries contextual information for guardian checks.
type OperationContext struct {
	Branch string
	Remote string
	User   string
}

// ResolvedPolicy is the compiled policy after defaults are applied.
type ResolvedPolicy struct {
    Scope         string
    Operation     string
    Method        Method
    Expires       time.Duration
    RequireReason bool
    Risk          string
    Conditions    map[string][]string
    Raw           *OperationPolicy
}

// Grant represents a single-use approval artifact created after guardian approval succeeds.
type Grant struct {
    ID        string    `json:"id"`
    Scope     string    `json:"scope"`
    Operation string    `json:"operation"`
    Branch    string    `json:"branch,omitempty"`
    Remote    string    `json:"remote,omitempty"`
    User      string    `json:"user,omitempty"`
    IssuedAt  time.Time `json:"issued_at"`
    ExpiresAt time.Time `json:"expires_at"`
    Method    Method    `json:"method"`
    Nonce     string    `json:"nonce"`
}
