package ssot

// SyncConfig represents the complete sync consumer configuration
// This follows the crucible sync-consumer pattern from sync-consumers-guide.md
type SyncConfig struct {
	Version    string           `yaml:"version"`
	Sources    []Source         `yaml:"sources"`
	Strategy   Strategy         `yaml:"strategy"`
	Provenance ProvenanceConfig `yaml:"provenance,omitempty"`
	isLocal    bool             // Internal flag to track if local override was loaded
}

// Source defines a sync source (e.g., crucible repository)
type Source struct {
	Name         string               `yaml:"name"`                   // Source name (e.g., "crucible")
	Repo         string               `yaml:"repo"`                   // GitHub repo (e.g., "fulmenhq/crucible")
	Ref          string               `yaml:"ref"`                    // Git ref/branch/tag (e.g., "main")
	LocalPath    string               `yaml:"localPath"`              // Local filesystem path (overrides repo, for dev)
	ForceRemote  bool                 `yaml:"force_remote,omitempty"` // Disable local detection, always use remote (v0.3.4+)
	SyncPathBase string               `yaml:"sync_path_base"`         // Subpath within repo (e.g., "lang/go")
	Assets       []Asset              `yaml:"assets"`                 // Assets to sync
	Keys         []string             `yaml:"keys"`                   // Optional catalog keys
	Output       string               `yaml:"output"`                 // Optional destination root
	Metadata     SourceMetadataConfig `yaml:"metadata,omitempty"`     // Metadata generation config (v0.3.0+)
}

// Asset defines what files to sync and where
type Asset struct {
	Type       string   `yaml:"type"`                  // "doc", "schema", "config", etc.
	Mode       string   `yaml:"mode,omitempty"`        // "copy" (default) or "link"
	SourcePath string   `yaml:"source_path,omitempty"` // Subdirectory within sync_path_base (e.g., "docs")
	Paths      []string `yaml:"paths,omitempty"`       // Glob patterns relative to source_path (or sync_path_base if source_path empty)
	Link       string   `yaml:"link,omitempty"`        // Relative link target when mode=link
	Subdir     string   `yaml:"subdir"`                // Destination subdirectory or symlink path
	Tags       []string `yaml:"tags,omitempty"`        // Tags for filtering (e.g., "dev")
}

// Strategy defines sync behavior
type Strategy struct {
	OnConflict      string `yaml:"on_conflict"`      // "overwrite", "skip", "error"
	PruneStale      bool   `yaml:"prune_stale"`      // Remove files not in source
	VerifyChecksums bool   `yaml:"verify_checksums"` // Verify file integrity
}

// SyncOptions contains runtime options for sync operation
type SyncOptions struct {
	Config        *SyncConfig
	DryRun        bool
	Verbose       bool
	ForceRemoteBy string // Track how force-remote was activated: "flag", "env", "config", "" (v0.3.4+)
}

// SyncResult contains the results of a sync operation
type SyncResult struct {
	Sources      []string    // Successfully synced source names
	FilesCopied  int         // Number of files copied
	FilesRemoved int         // Number of files removed
	Errors       []error     // Any non-fatal errors encountered
	Metadata     *Provenance // Captured provenance metadata (v0.3.0+)
}

// ResolvedSource contains the resolved path to a source
type ResolvedSource struct {
	Name     string // Source name
	Path     string // Resolved filesystem path (sync_path_base applied)
	RepoRoot string // Repository root path (for metadata/version detection)
	IsLocal  bool   // Whether this is a local path (not cloned)
	IsCloned bool   // Whether this was cloned via go-git
}
