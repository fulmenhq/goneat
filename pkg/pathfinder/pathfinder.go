// Package pathfinder provides unified path safety, file discovery, and traversal operations
// for goneat with support for both local and remote file systems.
package pathfinder

import (
	"fmt"
)

import (
	"context"
	"io"
	"time"
)

// PathOperation represents different types of path operations for audit logging
type PathOperation string

const (
	OpOpen      PathOperation = "open"       // File open operations
	OpList      PathOperation = "list"       // Directory listing
	OpWalk      PathOperation = "walk"       // Directory traversal
	OpValidate  PathOperation = "validate"   // Path validation
	OpDiscover  PathOperation = "discover"   // File discovery
	OpCacheHit  PathOperation = "cache_hit"  // Cache hit events
	OpCacheMiss PathOperation = "cache_miss" // Cache miss events
	OpDenied    PathOperation = "denied"     // Access denied events
)

// OperationResult represents the result of a path operation
type OperationResult struct {
	Status  string `json:"status"`  // success, failure, denied, rate_limited
	Code    int    `json:"code"`    // HTTP-like status code
	Message string `json:"message"` // Human-readable message
}

// AuditRecord represents a structured audit entry that matches JSON schema
type AuditRecord struct {
	ID            string            `json:"id"`
	Timestamp     time.Time         `json:"timestamp"`
	Operation     PathOperation     `json:"operation"`
	Path          string            `json:"path"`
	SourceLoader  string            `json:"source_loader"`
	Constraint    string            `json:"constraint"`
	Result        OperationResult   `json:"result"`
	Duration      time.Duration     `json:"duration_ms"`
	UserContext   map[string]string `json:"user_context,omitempty"`
	SecurityFlags []string          `json:"security_flags,omitempty"`
	ErrorDetails  *ErrorDetail      `json:"error,omitempty"`
}

// ErrorDetail provides structured error information
type ErrorDetail struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

// SourceLoader provides unified interface for file sources
type SourceLoader interface {
	Open(path string) (io.ReadCloser, error)
	ListFiles(basePath string, include, exclude []string) ([]string, error)
	SourceType() string
	SourceDescription() string
	Validate() error
	// Audit support
	SetAuditLogger(logger AuditLogger)
}

// PathFinder provides the main library interface
type PathFinder interface {
	SafeJoin(base, path string) (string, error)
	ValidatePath(path string) error
	DiscoverFiles(basePath string, opts DiscoveryOptions) ([]string, error)
	WalkDirectory(basePath string, walker WalkFunc, opts WalkOptions) error
	CreateLoader(sourceType string, config LoaderConfig) (SourceLoader, error)
	// Audit trail access
	GetAuditTrail(query AuditQuery) ([]AuditRecord, error)
	EnableAudit(config AuditConfig) error
}

// LoaderCreator is a factory function for creating SourceLoaders
type LoaderCreator func(config LoaderConfig) (SourceLoader, error)

// loaderRegistry holds registered loader creators to avoid import cycles
var loaderRegistry = make(map[string]LoaderCreator)

// RegisterLoader allows external packages to register loader factories
// This breaks potential import cycles by allowing loaders to self-register via init()
func RegisterLoader(name string, creator LoaderCreator) {
	loaderRegistry[name] = creator
}

// pathfinderImpl is the central implementation of the PathFinder interface
// *(Added by Arch Eagle: Central orchestrator for all pathfinder components)*
type pathfinderImpl struct {
	validator     *SafetyValidator
	auditLogger   AuditLogger
	discovery     *DiscoveryEngine
	walker        *SafeWalker
	defaultLoader string
}

// NewPathFinder creates a new PathFinder instance with default configuration
// *(Added by Arch Eagle: Factory function for unified library access)*
func NewPathFinder() PathFinder {
	validator := NewSafetyValidator()
	discovery := NewDiscoveryEngine(validator)
	walker := NewSafeWalker(validator)

	impl := &pathfinderImpl{
		validator:     validator,
		discovery:     discovery,
		walker:        walker,
		defaultLoader: "local",
	}

	return impl
}

// SafeJoin delegates to the safety validator
func (p *pathfinderImpl) SafeJoin(base, path string) (string, error) {
	return p.validator.SafeJoin(base, path)
}

// ValidatePath delegates to the safety validator
func (p *pathfinderImpl) ValidatePath(path string) error {
	return p.validator.ValidatePath(path)
}

// DiscoverFiles delegates to the discovery engine
func (p *pathfinderImpl) DiscoverFiles(basePath string, opts DiscoveryOptions) ([]string, error) {
	return p.discovery.DiscoverFiles(basePath, opts)
}

// WalkDirectory delegates to the safe walker
func (p *pathfinderImpl) WalkDirectory(basePath string, walker WalkFunc, opts WalkOptions) error {
	return p.walker.WalkDirectory(basePath, walker, opts)
}

// CreateLoader creates a loader based on the source type using the registry
// *(Added by Arch Eagle: Uses registry to support extensible loaders without cycles)*
func (p *pathfinderImpl) CreateLoader(sourceType string, config LoaderConfig) (SourceLoader, error) {
	if creator, exists := loaderRegistry[sourceType]; exists {
		loader, err := creator(config)
		if err != nil {
			return nil, err
		}
		// Set audit logger if enabled
		if p.auditLogger != nil {
			loader.SetAuditLogger(p.auditLogger)
		}
		return loader, nil
	}
	if sourceType == "" {
		sourceType = p.defaultLoader
		return p.CreateLoader(sourceType, config)
	}
	return nil, fmt.Errorf("unsupported source type: %s", sourceType)
}

// GetAuditTrail delegates to the audit logger
func (p *pathfinderImpl) GetAuditTrail(query AuditQuery) ([]AuditRecord, error) {
	if p.auditLogger == nil {
		return []AuditRecord{}, fmt.Errorf("audit not enabled")
	}
	return p.auditLogger.Query(query)
}

// EnableAudit sets up and enables the audit system
func (p *pathfinderImpl) EnableAudit(config AuditConfig) error {
	if p.auditLogger == nil {
		p.auditLogger = NewAuditLogger()
	}
	return p.auditLogger.Configure(config)
}

// PathConstraint defines boundaries for path operations
type PathConstraint interface {
	Contains(path string) bool
	Root() string
	Type() ConstraintType
	EnforcementLevel() EnforcementLevel
}

// ConstraintType represents different types of path constraints
type ConstraintType string

const (
	ConstraintRepository ConstraintType = "repository"
	ConstraintWorkspace  ConstraintType = "workspace"
	ConstraintCloud      ConstraintType = "cloud"
)

// EnforcementLevel defines how strictly constraints are enforced
type EnforcementLevel string

const (
	EnforcementStrict     EnforcementLevel = "strict"
	EnforcementWarn       EnforcementLevel = "warn"
	EnforcementPermissive EnforcementLevel = "permissive"
)

// AuditLogger provides enterprise audit trail interface
type AuditLogger interface {
	LogOperation(record AuditRecord) error
	Query(constraints AuditQuery) ([]AuditRecord, error)
	Export(format ExportFormat) ([]byte, error)
	SetComplianceMode(mode ComplianceMode) error
	Configure(config AuditConfig) error
}

// ComplianceMode represents different compliance standards
type ComplianceMode string

const (
	ComplianceNone   ComplianceMode = "none"
	ComplianceHIPAA  ComplianceMode = "HIPAA"
	ComplianceSOC2   ComplianceMode = "SOC2"
	CompliancePCIDSS ComplianceMode = "PCI-DSS"
	ComplianceGDPR   ComplianceMode = "GDPR"
)

// ExportFormat represents different export formats for audit data
type ExportFormat string

const (
	ExportJSON   ExportFormat = "json"
	ExportCSV    ExportFormat = "csv"
	ExportSyslog ExportFormat = "syslog"
)

// DiscoveryOptions configures file discovery behavior
type DiscoveryOptions struct {
	IncludePatterns  []string
	ExcludePatterns  []string
	SkipPatterns     []string
	SkipDirs         []string
	MaxDepth         int
	FollowSymlinks   bool
	Concurrency      int
	IncludeHidden    bool
	FileSizeLimits   SizeRange
	ModTimeRange     TimeRange
	Constraint       PathConstraint
	ErrorHandler     ErrorHandlerFunc
	ProgressCallback ProgressFunc
}

// SizeRange defines file size limits
type SizeRange struct {
	Min *int64
	Max *int64
}

// TimeRange defines file modification time range
type TimeRange struct {
	After  *time.Time
	Before *time.Time
}

// WalkFunc is called for each file/directory during walking
type WalkFunc func(path string, info FileInfo, err error) error

// FileInfo extends os.FileInfo with additional pathfinder-specific information
type FileInfo interface {
	Name() string       // base name of the file
	Size() int64        // length in bytes for regular files; system-dependent for others
	Mode() FileMode     // file mode bits
	ModTime() time.Time // modification time
	IsDir() bool        // abbreviation for Mode().IsDir()
	Sys() interface{}   // underlying data source (can return nil)
}

// FileMode represents file mode bits
type FileMode uint32

// WalkOptions configures directory walking behavior
type WalkOptions struct {
	MaxDepth         int
	SkipDirs         []string
	SkipPatterns     []string
	FollowSymlinks   bool
	ErrorHandler     ErrorHandlerFunc
	ProgressCallback ProgressFunc
	Concurrency      int
}

// ErrorHandlerFunc handles errors during directory walking
type ErrorHandlerFunc func(path string, err error) error

// ProgressFunc reports progress during directory walking
type ProgressFunc func(processed int, total int, currentPath string)

// LoaderConfig configures source loader behavior
type LoaderConfig struct {
	Type       string                 `json:"type"`
	Enabled    bool                   `json:"enabled"`
	Config     map[string]interface{} `json:"config,omitempty"`
	AuditTrail bool                   `json:"audit_trail"`
}

// AuditQuery defines query parameters for audit trail retrieval
type AuditQuery struct {
	StartTime    *time.Time     `json:"start_time,omitempty"`
	EndTime      *time.Time     `json:"end_time,omitempty"`
	Operation    *PathOperation `json:"operation,omitempty"`
	Path         *string        `json:"path,omitempty"`
	SourceLoader *string        `json:"source_loader,omitempty"`
	Result       *string        `json:"result,omitempty"`
	Limit        int            `json:"limit,omitempty"`
	Offset       int            `json:"offset,omitempty"`
}

// AuditConfig configures audit trail behavior
type AuditConfig struct {
	Enabled        bool           `json:"enabled"`
	ComplianceMode ComplianceMode `json:"compliance_mode"`
	RetentionDays  int            `json:"retention_days"`
	ExportFormats  []ExportFormat `json:"export_formats"`
}

// Context-aware versions for future use
type (
	PathFinderWithContext interface {
		PathFinder
		SafeJoinWithContext(ctx context.Context, base, path string) (string, error)
		ValidatePathWithContext(ctx context.Context, path string) error
		DiscoverFilesWithContext(ctx context.Context, basePath string, opts DiscoveryOptions) ([]string, error)
		WalkDirectoryWithContext(ctx context.Context, basePath string, walker WalkFunc, opts WalkOptions) error
	}
)
