package signature

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fulmenhq/goneat/internal/assets"
	"github.com/fulmenhq/goneat/pkg/config"
	"gopkg.in/yaml.v3"
)

const (
	embeddedManifestPath       = "embedded_schemas/schemas/signatures/v1.0.0/schema-signatures.yaml"
	embeddedManifestSchemaPath = "embedded_schemas/schemas/signatures/v1.0.0/schema-signature-manifest.schema.yaml"
	defaultConfidenceThreshold = 0.6
)

// Manifest is the top-level structure for schema signatures.
type Manifest struct {
	Version    string      `yaml:"version"`
	Signatures []Signature `yaml:"signatures"`
}

// Signature describes a single schema signature definition.
type Signature struct {
	ID                  string         `yaml:"id"`
	Category            string         `yaml:"category"`
	Description         string         `yaml:"description,omitempty"`
	ConfidenceThreshold float64        `yaml:"confidence_threshold,omitempty"`
	Aliases             []string       `yaml:"aliases,omitempty"`
	FileExtensions      []string       `yaml:"file_extensions,omitempty"`
	Matchers            []Matcher      `yaml:"matchers"`
	Metadata            map[string]any `yaml:"metadata,omitempty"`
	source              string
}

// Matcher defines an individual detection heuristic.
type Matcher struct {
	Type       string  `yaml:"type"`
	Value      string  `yaml:"value,omitempty"`
	Pattern    string  `yaml:"pattern,omitempty"`
	Weight     float64 `yaml:"weight,omitempty"`
	IgnoreCase bool    `yaml:"ignore_case,omitempty"`
}

// LoadDefaultManifest returns the embedded manifest combined with user overrides.
func LoadDefaultManifest() (*Manifest, error) {
	base, err := loadEmbeddedManifest()
	if err != nil {
		return nil, err
	}

	overrides, err := loadUserManifests()
	if err != nil {
		return nil, err
	}

	for _, overlay := range overrides {
		mergeManifest(base, overlay)
	}

	normaliseManifest(base)
	return base, nil
}

func loadEmbeddedManifest() (*Manifest, error) {
	data, ok := assets.GetSchema(embeddedManifestPath)
	if !ok {
		return nil, fmt.Errorf("embedded signature manifest not found: %s", embeddedManifestPath)
	}

	manifest, err := parseManifest(data, embeddedManifestPath)
	if err != nil {
		return nil, err
	}
	manifest.Version = strings.TrimSpace(manifest.Version)
	return manifest, nil
}

func loadUserManifests() ([]*Manifest, error) {
	home, err := config.GetGoneatHome()
	if err != nil {
		// treat as no overrides when home not resolvable
		return nil, nil
	}

	var manifests []*Manifest

	configFile := filepath.Join(home, "config", "signatures.yaml")
	if m, err := loadManifestFromFile(configFile); err != nil {
		return nil, err
	} else if m != nil {
		manifests = append(manifests, m)
	}

	signaturesDir := filepath.Join(home, "signatures")
	if entries, err := os.ReadDir(signaturesDir); err == nil {
		var files []string
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if !isYAML(name) {
				continue
			}
			files = append(files, filepath.Join(signaturesDir, name))
		}
		sort.Strings(files)
		for _, f := range files {
			m, err := loadManifestFromFile(f)
			if err != nil {
				return nil, err
			}
			if m != nil {
				manifests = append(manifests, m)
			}
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}

	return manifests, nil
}

func loadManifestFromFile(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read signature manifest %s: %w", path, err)
	}
	manifest, err := parseManifest(data, path)
	if err != nil {
		return nil, err
	}
	return manifest, nil
}

func parseManifest(data []byte, source string) (*Manifest, error) {
	var manifest Manifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse signature manifest %s: %w", source, err)
	}
	for i := range manifest.Signatures {
		manifest.Signatures[i].source = source
	}
	return &manifest, nil
}

func mergeManifest(base, overlay *Manifest) {
	if overlay == nil {
		return
	}

	if overlay.Version != "" {
		base.Version = overlay.Version
	}

	if len(overlay.Signatures) == 0 {
		return
	}

	index := make(map[string]int, len(base.Signatures))
	for i, sig := range base.Signatures {
		index[strings.ToLower(sig.ID)] = i
	}

	for _, sig := range overlay.Signatures {
		key := strings.ToLower(sig.ID)
		sig.source = overlayVersionSource(overlay, sig)
		if idx, ok := index[key]; ok {
			base.Signatures[idx] = sig
		} else {
			base.Signatures = append(base.Signatures, sig)
			index[key] = len(base.Signatures) - 1
		}
	}
}

func overlayVersionSource(manifest *Manifest, sig Signature) string {
	if sig.source != "" {
		return sig.source
	}
	if manifest != nil && manifest.Version != "" {
		return fmt.Sprintf("%s@%s", manifest.Version, "override")
	}
	return "override"
}

func normaliseManifest(manifest *Manifest) {
	seen := make(map[string]struct{})
	var deduped []Signature
	for _, sig := range manifest.Signatures {
		id := strings.ToLower(strings.TrimSpace(sig.ID))
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			// last definition wins; skip earlier duplicates
			continue
		}
		seen[id] = struct{}{}

		if sig.ConfidenceThreshold <= 0 {
			sig.ConfidenceThreshold = defaultConfidenceThreshold
		}
		// normalise extensions to lower-case with leading dot
		if len(sig.FileExtensions) > 0 {
			normalised := make([]string, 0, len(sig.FileExtensions))
			extSeen := make(map[string]struct{})
			for _, ext := range sig.FileExtensions {
				ext = strings.TrimSpace(ext)
				if ext == "" {
					continue
				}
				if !strings.HasPrefix(ext, ".") {
					ext = "." + ext
				}
				ext = strings.ToLower(ext)
				if _, ok := extSeen[ext]; ok {
					continue
				}
				extSeen[ext] = struct{}{}
				normalised = append(normalised, ext)
			}
			sort.Strings(normalised)
			sig.FileExtensions = normalised
		}

		for i := range sig.Matchers {
			if sig.Matchers[i].Weight <= 0 {
				sig.Matchers[i].Weight = 1
			}
			sig.Matchers[i].Type = strings.ToLower(strings.TrimSpace(sig.Matchers[i].Type))
		}

		deduped = append(deduped, sig)
	}
	manifest.Signatures = deduped
}

func isYAML(name string) bool {
	lower := strings.ToLower(name)
	return strings.HasSuffix(lower, ".yaml") || strings.HasSuffix(lower, ".yml")
}

// Source returns the origin of the signature definition (embedded manifest or override file).
func (s Signature) Source() string {
	return s.source
}
