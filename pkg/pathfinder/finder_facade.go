package pathfinder

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/fulmenhq/goneat/pkg/schema/signature"
)

const schemaPeekLimit = 4096

// PathResult represents a discovered path along with logical mapping information.
type PathResult struct {
	// RelativePath is the path relative to the query root (normalized with forward slashes).
	RelativePath string `json:"relative_path"`
	// SourcePath is the absolute or loader-native path used for reading the resource.
	SourcePath string `json:"source_path"`
	// LogicalPath is the consumer-facing path after optional transforms (defaults to RelativePath).
	LogicalPath string `json:"logical_path"`
	// LoaderType identifies the underlying loader used to resolve the path.
	LoaderType string `json:"loader_type"`
	// Metadata contains optional provider-specific information (size, etag, etc.).
	Metadata map[string]any `json:"metadata,omitempty"`
}

// PathTransform allows callers to remap path metadata (e.g., flatten directories).
type PathTransform func(result PathResult) PathResult

// FinderConfig holds default settings for the FinderFacade.
type FinderConfig struct {
	MaxWorkers   int
	CacheEnabled bool
	CacheTTL     time.Duration
	Constraint   PathConstraint
	LoaderType   string
}

// FindQuery specifies the parameters for discovery.
type FindQuery struct {
	Root                  string
	Include               []string
	Exclude               []string
	SkipDirs              []string
	MaxDepth              int
	FollowSymlinks        bool
	Workers               int
	Context               context.Context
	Stream                bool
	Transform             PathTransform
	SchemaMode            bool
	SchemaIDs             []string
	SchemaCategories      []string
	IncludeSchemaMetadata bool
}

// FinderFacade provides a simplified API on top of the full PathFinder interface.
type FinderFacade struct {
	pf     PathFinder
	config FinderConfig
}

// NewFinderFacade constructs a FinderFacade with sane defaults.
func NewFinderFacade(pf PathFinder, cfg FinderConfig) *FinderFacade {
	facade := &FinderFacade{
		pf:     pf,
		config: cfg,
	}
	if facade.config.MaxWorkers <= 0 {
		facade.config.MaxWorkers = runtime.NumCPU()
		if facade.config.MaxWorkers < 1 {
			facade.config.MaxWorkers = 1
		}
	}
	if facade.config.LoaderType == "" {
		facade.config.LoaderType = "local"
	}
	return facade
}

// Find returns all matching paths using the simplified facade semantics.
func (f *FinderFacade) Find(query FindQuery) ([]PathResult, error) {
	ctx := ensureContext(query.Context)
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if err := f.pf.ValidatePath(query.Root); err != nil {
		return nil, err
	}

	var detector *signature.Detector
	var detectOpts signature.DetectOptions
	includeSchemaMetadata := query.IncludeSchemaMetadata

	if query.SchemaMode {
		manifest, err := signature.LoadDefaultManifest()
		if err != nil {
			return nil, err
		}
		if detector, err = signature.NewDetector(manifest); err != nil {
			return nil, err
		}
		if len(query.Include) == 0 {
			if includes := buildSchemaIncludePatterns(manifest); len(includes) > 0 {
				query.Include = includes
			}
		}
		detectOpts = buildDetectOptions(query)
	}

	opts := f.buildDiscoveryOptions(query)
	files, err := f.pf.DiscoverFiles(query.Root, opts)
	if err != nil {
		return nil, err
	}

	results := make([]PathResult, 0, len(files))
	transform := query.Transform
	if transform == nil {
		transform = passthroughTransform
	}

	for _, rel := range files {
		normalizedRel := toSlash(rel)
		absPath := filepath.Clean(filepath.Join(query.Root, rel))
		result := PathResult{
			RelativePath: normalizedRel,
			SourcePath:   toSlash(absPath),
			LogicalPath:  normalizedRel,
			LoaderType:   f.effectiveLoaderType(query),
		}

		if query.SchemaMode && detector != nil {
			snippet, err := readSnippet(absPath, schemaPeekLimit)
			if err != nil {
				continue
			}
			match, ok := detector.Detect(absPath, snippet, detectOpts)
			if !ok {
				continue
			}
			if result.Metadata == nil {
				result.Metadata = make(map[string]any)
			}
			result.Metadata["schema"] = buildSchemaMetadata(match, includeSchemaMetadata)
		}

		result = transform(result)
		results = append(results, result)
	}

	return results, ctx.Err()
}

// FindStream returns channels for streaming discovery results.
// The returned error channel is closed after all items are produced; a non-nil value
// indicates either discovery failure or context cancellation.
func (f *FinderFacade) FindStream(query FindQuery) (<-chan PathResult, <-chan error) {
	resultCh := make(chan PathResult)
	errCh := make(chan error, 1)

	go func() {
		defer close(resultCh)
		defer close(errCh)

		results, err := f.Find(query)
		if err != nil {
			errCh <- err
			return
		}

		ctx := ensureContext(query.Context)
		for _, item := range results {
			select {
			case <-ctx.Done():
				errCh <- ctx.Err()
				return
			case resultCh <- item:
			}
		}
	}()

	return resultCh, errCh
}

func (f *FinderFacade) buildDiscoveryOptions(query FindQuery) DiscoveryOptions {
	effectiveWorkers := query.Workers
	if effectiveWorkers <= 0 {
		effectiveWorkers = f.config.MaxWorkers
	}
	if effectiveWorkers < 1 {
		effectiveWorkers = 1
	}

	opts := DiscoveryOptions{
		IncludePatterns:  query.Include,
		ExcludePatterns:  query.Exclude,
		SkipDirs:         query.SkipDirs,
		MaxDepth:         query.MaxDepth,
		FollowSymlinks:   query.FollowSymlinks,
		Constraint:       f.config.Constraint,
		Concurrency:      effectiveWorkers,
		IncludeHidden:    false,
		ProgressCallback: nil,
	}

	if opts.MaxDepth == 0 {
		opts.MaxDepth = -1
	}

	return opts
}

func (f *FinderFacade) effectiveLoaderType(query FindQuery) string {
	if f.config.LoaderType != "" {
		return f.config.LoaderType
	}
	return "local"
}

func ensureContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func passthroughTransform(result PathResult) PathResult {
	return result
}

func readSnippet(path string, limit int) ([]byte, error) {
	if limit <= 0 {
		limit = schemaPeekLimit
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	buf := make([]byte, limit)
	n, err := io.ReadFull(file, buf)
	if err != nil {
		if err == io.ErrUnexpectedEOF || err == io.EOF {
			if n <= 0 {
				return []byte{}, nil
			}
			return buf[:n], nil
		}
		return nil, err
	}
	return buf, nil
}

func buildDetectOptions(query FindQuery) signature.DetectOptions {
	opts := signature.DetectOptions{}
	if len(query.SchemaIDs) > 0 {
		opts.AllowedIDs = makeStringSet(query.SchemaIDs)
	}
	if len(query.SchemaCategories) > 0 {
		opts.AllowedCategories = makeStringSet(query.SchemaCategories)
	}
	return opts
}

func makeStringSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, v := range values {
		clean := strings.ToLower(strings.TrimSpace(v))
		if clean == "" {
			continue
		}
		set[clean] = struct{}{}
	}
	return set
}

func buildSchemaMetadata(match signature.Match, includeExtra bool) map[string]any {
	meta := map[string]any{
		"id":       match.Signature.ID,
		"category": match.Signature.Category,
		"score":    match.Score,
	}
	if desc := strings.TrimSpace(match.Signature.Description); desc != "" {
		meta["description"] = desc
	}
	if src := match.Signature.Source(); src != "" {
		meta["source"] = src
	}
	if includeExtra {
		if len(match.Signature.Metadata) > 0 {
			meta["signature_metadata"] = match.Signature.Metadata
		}
		if len(match.Matchers) > 0 {
			var matched []map[string]any
			for _, m := range match.Matchers {
				matched = append(matched, map[string]any{
					"type":   m.Type,
					"weight": m.Weight,
				})
				if m.Value != "" {
					matched[len(matched)-1]["value"] = m.Value
				}
				if m.Pattern != "" {
					matched[len(matched)-1]["pattern"] = m.Pattern
				}
			}
			meta["matched"] = matched
		}
	}
	return meta
}

func buildSchemaIncludePatterns(manifest *signature.Manifest) []string {
	if manifest == nil {
		return nil
	}
	seen := make(map[string]struct{})
	for _, sig := range manifest.Signatures {
		for _, ext := range sig.FileExtensions {
			clean := strings.TrimSpace(ext)
			if clean == "" {
				continue
			}
			clean = strings.ToLower(clean)
			clean = strings.TrimPrefix(clean, ".")
			if clean == "" {
				continue
			}
			seen[clean] = struct{}{}
		}
	}
	if len(seen) == 0 {
		return nil
	}
	patterns := make([]string, 0, len(seen))
	for ext := range seen {
		patterns = append(patterns, fmt.Sprintf("**/*.%s", ext))
	}
	sort.Strings(patterns)
	return patterns
}

func toSlash(p string) string {
	cleaned := filepath.Clean(p)
	return strings.TrimPrefix(path.Clean(filepath.ToSlash(cleaned)), "./")
}
