package mapping

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/fulmenhq/goneat/pkg/safeio"
	"github.com/fulmenhq/goneat/pkg/schema"
	"gopkg.in/yaml.v3"
)

// DefaultManifestRelativePath is the repository-relative path for mapping manifests.
const DefaultManifestRelativePath = ".goneat/schema-mappings.yaml"

// Severity indicates diagnostic classifications during manifest loading.
type Severity string

const (
	SeverityInfo  Severity = "info"
	SeverityWarn  Severity = "warn"
	SeverityError Severity = "error"
)

// Diagnostic captures informational messages produced while loading manifests.
type Diagnostic struct {
	Severity Severity
	Message  string
	Source   string
}

// LoadOptions configures manifest resolution.
type LoadOptions struct {
	RepoRoot     string
	ManifestPath string
}

// LoadResult contains the resolved manifest state.
type LoadResult struct {
	Builtin        Manifest
	Repository     *Manifest
	RepositoryPath string
	Effective      Manifest
	Diagnostics    []Diagnostic
}

// Manager orchestrates manifest loading, validation, and composition.
type Manager struct {
	validator *schema.Validator
	builtin   Manifest
}

// NewManager constructs a Manager prepared for v1 manifests.
func NewManager() (*Manager, error) {
	validator, err := schema.GetEmbeddedValidator("schema-mapping-manifest-v1.0.0")
	if err != nil {
		return nil, fmt.Errorf("load embedded manifest validator: %w", err)
	}
	return &Manager{
		validator: validator,
		builtin:   BuiltinManifest(),
	}, nil
}

// Load composes the builtin manifest with an optional repository manifest.
func (m *Manager) Load(opts LoadOptions) (*LoadResult, error) {
	repoManifest, repoPath, diags, err := m.loadRepositoryManifest(opts)
	if err != nil {
		return nil, err
	}

	effective := m.builtin.Clone()
	if repoManifest != nil {
		effective = mergeManifests(effective, *repoManifest)
	}

	result := &LoadResult{
		Builtin:        m.builtin.Clone(),
		Repository:     repoManifest,
		RepositoryPath: repoPath,
		Effective:      effective,
		Diagnostics:    diags,
	}
	return result, nil
}

func (m *Manager) loadRepositoryManifest(opts LoadOptions) (*Manifest, string, []Diagnostic, error) {
	repoRoot := opts.RepoRoot
	if repoRoot == "" {
		repoRoot = "."
	}
	manifestRel := opts.ManifestPath
	if manifestRel == "" {
		manifestRel = DefaultManifestRelativePath
	}

	sanitizedRel, err := safeio.CleanUserPath(manifestRel)
	if err != nil {
		return nil, "", nil, fmt.Errorf("invalid manifest path %q: %w", manifestRel, err)
	}

	manifestPath := filepath.Join(repoRoot, sanitizedRel)
	if err := ensureWithinRepo(repoRoot, manifestPath); err != nil {
		return nil, "", nil, err
	}

	data, err := os.ReadFile(manifestPath) // #nosec G304 -- path sanitized by ensureWithinRepo above
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, "", []Diagnostic{{
				Severity: SeverityInfo,
				Message:  fmt.Sprintf("schema mapping manifest not found; using built-in defaults (%s)", manifestPath),
				Source:   manifestPath,
			}}, nil
		}
		return nil, "", nil, fmt.Errorf("read manifest %s: %w", manifestPath, err)
	}

	validation, err := m.validator.ValidateBytes(data)
	if err != nil {
		return nil, manifestPath, nil, fmt.Errorf("validate manifest %s: %w", manifestPath, err)
	}
	if !validation.Valid {
		return nil, manifestPath, nil, fmt.Errorf("manifest %s failed validation: %s", manifestPath, flattenValidationErrors(validation))
	}

	var manifest Manifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, manifestPath, nil, fmt.Errorf("parse manifest %s: %w", manifestPath, err)
	}

	if manifest.Version == "" {
		manifest.Version = ManifestVersionV1
	}
	if manifest.Version != ManifestVersionV1 {
		return nil, manifestPath, nil, fmt.Errorf("unsupported schema mapping manifest version %q (expected %s)", manifest.Version, ManifestVersionV1)
	}

	// Apply defaults relative to builtin configuration.
	manifest.Config = manifest.Config.WithDefaults(m.builtin.Config)

	return &manifest, manifestPath, nil, nil
}

func ensureWithinRepo(repoRoot, target string) error {
	repoAbs, err := filepath.Abs(repoRoot)
	if err != nil {
		return fmt.Errorf("resolve repo root: %w", err)
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return fmt.Errorf("resolve manifest path: %w", err)
	}
	if !strings.HasPrefix(targetAbs+string(os.PathSeparator), repoAbs+string(os.PathSeparator)) {
		return fmt.Errorf("manifest path %s escapes repository root %s", targetAbs, repoAbs)
	}
	return nil
}

func mergeManifests(base Manifest, overlay Manifest) Manifest {
	merged := base.Clone()
	merged.Config = merged.Config.Merge(overlay.Config)
	merged.Mappings = append(merged.Mappings, overlay.Mappings...)
	merged.Exclusions = append(merged.Exclusions, overlay.Exclusions...)
	merged.Overrides = append(merged.Overrides, overlay.Overrides...)
	return merged
}

func flattenValidationErrors(res *schema.Result) string {
	if res == nil || len(res.Errors) == 0 {
		return "unknown validation failure"
	}
	msgs := make([]string, 0, len(res.Errors))
	for _, err := range res.Errors {
		msg := err.Message
		if err.Context.SourceFile != "" {
			msg = fmt.Sprintf("%s (%s)", msg, err.Context.SourceFile)
		}
		msgs = append(msgs, msg)
	}
	return strings.Join(msgs, "; ")
}
