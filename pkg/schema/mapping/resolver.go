package mapping

import (
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// Resolution contains the outcome of resolving a file against the manifest.
type Resolution struct {
	SchemaID   string
	Source     SchemaSource
	Rule       *MappingRule
	Excluded   bool
	Reason     string
	Confidence float64
}

// Metrics captures aggregate statistics while resolving mappings.
type Metrics struct {
	FilesEvaluated int
	Mapped         int
	Unmapped       int
	Excluded       int
}

// Resolver applies manifest rules to file paths.
type Resolver struct {
	manifest Manifest
	metrics  Metrics
}

// NewResolver constructs a resolver over the provided manifest.
func NewResolver(manifest Manifest) *Resolver {
	return &Resolver{manifest: manifest.Clone()}
}

// Metrics returns a copy of the resolver metrics collected so far.
func (r *Resolver) Metrics() Metrics {
	return r.metrics
}

// Resolve returns the schema resolution for the given relative path.
// The path should be repository-relative using platform separators.
func (r *Resolver) Resolve(relPath string) (Resolution, bool) {
	r.metrics.FilesEvaluated++

	norm := filepath.ToSlash(strings.TrimPrefix(relPath, "./"))

	if res, ok := r.applyExclusions(norm); ok {
		if res.Excluded {
			r.metrics.Excluded++
		}
		return res, true
	}

	for i := len(r.manifest.Mappings) - 1; i >= 0; i-- {
		// iterate overlay-first since repository manifest rules were appended last
		rule := r.manifest.Mappings[i]
		if !matchPattern(rule.Pattern, norm) {
			continue
		}

		if rule.SchemaID == "" {
			// For now we do not implement inference methods.
			continue
		}

		res := Resolution{
			SchemaID:   rule.SchemaID,
			Source:     rule.Source,
			Rule:       &rule,
			Confidence: 1.0,
		}
		r.metrics.Mapped++
		return res, true
	}

	r.metrics.Unmapped++
	return Resolution{}, false
}

func (r *Resolver) applyExclusions(path string) (Resolution, bool) {
	for i := len(r.manifest.Exclusions) - 1; i >= 0; i-- {
		rule := r.manifest.Exclusions[i]
		if rule.Pattern != "" && !matchPattern(rule.Pattern, path) {
			continue
		}

		// Content-based exclusions not yet implemented (future work).

		if rule.Action == ExclusionSkip || rule.Action == "" {
			return Resolution{Excluded: true, Reason: rule.Reason, Rule: nil}, true
		}
		if rule.Action == ExclusionRetryInference {
			return Resolution{Excluded: true, Reason: rule.Reason, Rule: nil}, true
		}
	}
	return Resolution{}, false
}

func matchPattern(pattern, path string) bool {
	if pattern == "" {
		return false
	}
	pat := filepath.ToSlash(pattern)
	matched, err := doublestar.Match(pat, path)
	if err != nil {
		return false
	}
	if matched {
		return true
	}
	// Also attempt match against basename for convenience
	return filepath.Base(path) == filepath.Base(pat)
}
