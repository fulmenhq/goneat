package ssot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/fulmenhq/goneat/pkg/logger"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"gopkg.in/yaml.v3"
)

// Provenance represents aggregate SSOT sync metadata
type Provenance struct {
	Schema      SchemaDescriptor `json:"schema" yaml:"schema"`
	GeneratedAt time.Time        `json:"generated_at" yaml:"generated_at"`
	Sources     []SourceMetadata `json:"sources" yaml:"sources"`
}

// SourceMetadata represents metadata for a single SSOT source
type SourceMetadata struct {
	Name          string            `json:"name" yaml:"name"`
	Slug          string            `json:"slug" yaml:"slug"`
	Method        string            `json:"method" yaml:"method"` // local_path, git_ref, git_tag, archive
	RepoURL       string            `json:"repo_url,omitempty" yaml:"repo_url,omitempty"`
	LocalPath     string            `json:"local_path,omitempty" yaml:"local_path,omitempty"`
	Ref           string            `json:"ref,omitempty" yaml:"ref,omitempty"`
	Commit        string            `json:"commit,omitempty" yaml:"commit,omitempty"`
	Dirty         bool              `json:"dirty,omitempty" yaml:"dirty,omitempty"`
	DirtyReason   string            `json:"dirty_reason,omitempty" yaml:"dirty_reason,omitempty"`
	ForcedRemote  bool              `json:"forced_remote,omitempty" yaml:"forced_remote,omitempty"` // v0.3.4+: indicates force-remote was used
	ForcedBy      string            `json:"forced_by,omitempty" yaml:"forced_by,omitempty"`         // v0.3.4+: "flag", "env", "config"
	VersionFile   string            `json:"version_file,omitempty" yaml:"version_file,omitempty"`
	Version       string            `json:"version,omitempty" yaml:"version,omitempty"`
	VersionSource string            `json:"version_source,omitempty" yaml:"version_source,omitempty"`
	Outputs       map[string]string `json:"outputs,omitempty" yaml:"outputs,omitempty"`
}

// SchemaDescriptor describes the schema version for metadata
type SchemaDescriptor struct {
	Name    string `json:"name" yaml:"name"`
	Version string `json:"version" yaml:"version"`
	URL     string `json:"url" yaml:"url"`
}

// ProvenanceConfig controls metadata generation behavior
type ProvenanceConfig struct {
	Enabled         *bool  `yaml:"enabled,omitempty"`           // Enable metadata generation (default: true when nil)
	OutputPath      string `yaml:"output,omitempty"`            // Aggregate manifest path (default: .goneat/ssot/provenance.json)
	MirrorPerSource *bool  `yaml:"mirror_per_source,omitempty"` // Write per-source mirrors (default: true when nil)
	PerSourceFormat string `yaml:"per_source_format,omitempty"` // Mirror format: yaml, json (default: yaml)
}

// SourceMetadataConfig provides per-source metadata configuration
type SourceMetadataConfig struct {
	VersionFile string `yaml:"version_file,omitempty"` // Version file name (default: VERSION)
	MirrorPath  string `yaml:"mirror_path,omitempty"`  // Custom mirror path (overrides slug-based default)
}

// introspectRepository inspects a Git repository for commit, ref, dirty state, and repo root
// Returns the repository root path which can be used to locate files like VERSION at the repo root
func introspectRepository(sourcePath string) (commit string, ref string, dirty bool, reason string, repoRoot string, err error) {
	repo, err := git.PlainOpenWithOptions(sourcePath, &git.PlainOpenOptions{
		DetectDotGit: true, // Walk up to find .git directory
	})
	if err != nil {
		// Not a git repository - not an error, just mark as non-git
		return "", "", true, "non-git", "", nil
	}

	head, err := repo.Head()
	if err != nil {
		return "", "", true, "no-head", "", fmt.Errorf("failed to get HEAD: %w", err)
	}

	commit = head.Hash().String()
	ref = head.Name().Short()

	worktree, err := repo.Worktree()
	if err != nil {
		return commit, ref, true, "no-worktree", "", nil
	}

	// Get repository root from worktree filesystem
	repoRoot = worktree.Filesystem.Root()

	status, err := worktree.Status()
	if err != nil {
		return commit, ref, true, "status-error", repoRoot, nil
	}

	// Check if worktree has uncommitted changes
	// go-git's Status() includes ALL untracked files (even gitignored ones)
	// We need to filter out gitignored files to match git's behavior

	// Check for uncommitted changes, respecting repository .gitignore
	// Design decision: We only check repository .gitignore, not global gitignore
	// Rationale documented in docs/architecture/decisions/0002-ssot-dirty-detection.md

	// Load gitignore patterns from repository
	patterns, err := gitignore.ReadPatterns(worktree.Filesystem, nil)
	if err != nil {
		logger.Debug(fmt.Sprintf("failed to read gitignore patterns: %v", err))
		patterns = []gitignore.Pattern{} // Continue with empty patterns if read fails
	}
	// Include external excludes (e.g., .git/info/exclude)
	patterns = append(patterns, worktree.Excludes...)
	matcher := gitignore.NewMatcher(patterns)

	for path, fileStatus := range status {
		// Handle untracked files - check if they're in repository .gitignore
		if fileStatus.Worktree == git.Untracked {
			pathParts := strings.Split(filepath.ToSlash(path), "/")
			isIgnored := matcher.Match(pathParts, false)
			if isIgnored {
				// File is in repository .gitignore, skip it
				continue
			}
			// File is untracked and NOT in repository .gitignore - repo is dirty
			dirty = true
			reason = "worktree-dirty"
			break
		}

		// For tracked files, any modification or staging means dirty
		if fileStatus.Staging != git.Unmodified || fileStatus.Worktree != git.Unmodified {
			dirty = true
			reason = "worktree-dirty"
			break
		}
	}

	return commit, ref, dirty, reason, repoRoot, nil
}

// detectVersion reads version from a file in the source directory
func detectVersion(sourcePath string, versionFile string) (version string, source string, err error) {
	if versionFile == "" {
		versionFile = "VERSION"
	}

	versionPath := filepath.Join(sourcePath, versionFile)
	// #nosec G304 -- versionPath is constructed from sourcePath (validated mirror config) and versionFile (config or default "VERSION")
	data, err := os.ReadFile(versionPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", "not-found", nil // Not an error
		}
		return "", "", fmt.Errorf("failed to read version file: %w", err)
	}

	version = strings.TrimSpace(string(data))
	source = versionFile
	return version, source, nil
}

// generateSlug creates a URL-safe slug from a source name
func generateSlug(name string) string {
	slug := strings.ToLower(name)
	slug = strings.ReplaceAll(slug, " ", "-")
	// Remove non-alphanumeric except hyphens
	reg := regexp.MustCompile(`[^a-z0-9-]+`)
	slug = reg.ReplaceAllString(slug, "")
	return slug
}

// captureSourceMetadata captures metadata for a single source
func captureSourceMetadata(source Source, resolved ResolvedSource, outputs map[string]string, opts SyncOptions) (*SourceMetadata, error) {
	slug := generateSlug(source.Name)

	// Introspect repository - returns repo root for finding VERSION file
	commit, detectedRef, dirty, dirtyReason, repoRoot, err := introspectRepository(resolved.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to introspect repo: %w", err)
	}

	// Use detected ref if source ref is empty
	ref := source.Ref
	if ref == "" {
		ref = detectedRef
	}

	// Detect version - use repo root if available, otherwise fall back to resolved path
	// This ensures we find VERSION at repo root even when sync_path_base is set
	versionSearchPath := resolved.Path
	if resolved.RepoRoot != "" {
		versionSearchPath = resolved.RepoRoot
	} else if repoRoot != "" {
		versionSearchPath = repoRoot
	}
	versionFile := "VERSION"
	if source.Metadata.VersionFile != "" {
		versionFile = source.Metadata.VersionFile
	}
	version, versionSource, err := detectVersion(versionSearchPath, versionFile)
	if err != nil {
		logger.Debug(fmt.Sprintf("version detection failed for %s: %v", source.Name, err))
	}

	// Determine method
	method := "local_path"
	if resolved.IsCloned {
		method = "git_ref"
	}
	if dirty && dirtyReason == "non-git" {
		method = "archive"
	}

	// Build repo URL
	repoURL := ""
	if source.Repo != "" {
		repoURL = fmt.Sprintf("https://github.com/%s", source.Repo)
	}

	// Determine force-remote metadata (v0.3.4+)
	forcedRemote := source.ForceRemote
	forcedBy := ""
	if forcedRemote {
		// Determine the source of force-remote
		if opts.ForceRemoteBy != "" {
			forcedBy = opts.ForceRemoteBy
		} else if os.Getenv("GONEAT_FORCE_REMOTE_SYNC") == "1" {
			forcedBy = "env"
		} else {
			forcedBy = "config"
		}
	}

	return &SourceMetadata{
		Name:          source.Name,
		Slug:          slug,
		Method:        method,
		RepoURL:       repoURL,
		LocalPath:     source.LocalPath,
		Ref:           ref, // Use computed ref (includes detectedRef fallback)
		Commit:        commit,
		Dirty:         dirty,
		DirtyReason:   dirtyReason,
		ForcedRemote:  forcedRemote,
		ForcedBy:      forcedBy,
		VersionFile:   versionFile,
		Version:       version,
		VersionSource: versionSource,
		Outputs:       outputs,
	}, nil
}

// buildProvenance creates an aggregate provenance manifest
func buildProvenance(sources []SourceMetadata) *Provenance {
	return &Provenance{
		Schema: SchemaDescriptor{
			Name:    "goneat.ssot.provenance",
			Version: "v1",
			URL:     "https://github.com/fulmenhq/crucible/schemas/content/ssot-provenance/v1.1.0/ssot-provenance.schema.json",
		},
		GeneratedAt: time.Now().UTC(),
		Sources:     sources,
	}
}

// writeAggregateProvenance writes the aggregate provenance manifest
func writeAggregateProvenance(provenance *Provenance, outputPath string, dryRun bool) error {
	if outputPath == "" {
		outputPath = ".goneat/ssot/provenance.json"
	}

	if dryRun {
		logger.Info(fmt.Sprintf("[DRY RUN] Would write aggregate provenance to %s", outputPath))
		return nil
	}

	// Ensure directory exists
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(provenance, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// Write with secure permissions
	if err := os.WriteFile(outputPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	sourceNames := make([]string, len(provenance.Sources))
	for i, s := range provenance.Sources {
		sourceNames[i] = s.Name
	}

	logger.Info(fmt.Sprintf("SSOT provenance recorded: %s (sources: %s)",
		outputPath, strings.Join(sourceNames, ", ")))

	return nil
}

// writePerSourceMirror writes a per-source metadata mirror
func writePerSourceMirror(source *SourceMetadata, format string, sourceConfig *Source, dryRun bool) error {
	if format == "" {
		format = "yaml"
	}

	// Use custom mirror path if provided, otherwise default to slug-based
	mirrorPath := ""
	if sourceConfig != nil && sourceConfig.Metadata.MirrorPath != "" {
		mirrorPath = sourceConfig.Metadata.MirrorPath
	} else {
		mirrorPath = fmt.Sprintf(".%s/metadata/metadata.%s", source.Slug, format)
	}

	if dryRun {
		logger.Info(fmt.Sprintf("[DRY RUN] Would write source mirror to %s", mirrorPath))
		return nil
	}

	// Ensure directory exists
	dir := filepath.Dir(mirrorPath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create single-source provenance wrapper
	singleSource := &Provenance{
		Schema: SchemaDescriptor{
			Name:    "goneat.ssot.source-metadata",
			Version: "v1",
			URL:     "https://github.com/fulmenhq/goneat/schemas/ssot/source-metadata.v1.1.0.json",
		},
		GeneratedAt: time.Now().UTC(),
		Sources:     []SourceMetadata{*source},
	}

	var data []byte
	var err error

	if format == "json" {
		data, err = json.MarshalIndent(singleSource, "", "  ")
	} else {
		data, err = yaml.Marshal(singleSource)
	}

	if err != nil {
		return fmt.Errorf("failed to marshal %s: %w", format, err)
	}

	if err := os.WriteFile(mirrorPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}
