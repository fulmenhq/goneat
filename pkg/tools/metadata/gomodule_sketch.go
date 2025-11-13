package metadata

// This file is a DESIGN SKETCH for Phase 2+
// It demonstrates how a Go Module fetcher would integrate with the metadata system
// by reusing existing pkg/registry/client.go infrastructure.
//
// DO NOT USE IN PRODUCTION - This is for architecture validation only.

/*
import (
	"fmt"
	"strings"
	"time"

	"github.com/fulmenhq/goneat/pkg/registry"
)

// GoModuleFetcher fetches metadata from Go proxy (proxy.golang.org)
// Reuses existing pkg/registry/client.go for Go module metadata
type GoModuleFetcher struct {
	registryClient registry.Client
	timeout        time.Duration
}

// NewGoModuleFetcher creates a Go module metadata fetcher
// Reuses the existing registry.GoClient with appropriate TTL
func NewGoModuleFetcher(timeout time.Duration) *GoModuleFetcher {
	// Reuse existing Go proxy client from pkg/registry
	// This validates that our Fetcher interface has the right error semantics
	// for ecosystems beyond GitHub releases
	registryClient := registry.NewGoClient(24 * time.Hour)

	return &GoModuleFetcher{
		registryClient: registryClient,
		timeout:        timeout,
	}
}

// SupportsRepo returns true for Go module paths
func (f *GoModuleFetcher) SupportsRepo(repo string) bool {
	// Go modules use domain-based paths:
	// - github.com/golangci/golangci-lint/cmd/golangci-lint
	// - golang.org/x/tools/cmd/goimports
	// - gopkg.in/yaml.v3

	// Basic heuristic: contains domain + path (at least 2 slashes)
	if !strings.Contains(repo, "/") {
		return false
	}

	// Check for common Go module prefixes
	prefixes := []string{
		"github.com/",
		"golang.org/",
		"gopkg.in/",
		"go.uber.org/",
	}

	for _, prefix := range prefixes {
		if strings.HasPrefix(repo, prefix) {
			return true
		}
	}

	// Could also check for "go.mod" presence or module proxy availability
	return false
}

// FetchMetadata fetches module info from Go proxy
func (f *GoModuleFetcher) FetchMetadata(repo, version string) (*Metadata, error) {
	// Reuse existing registry client - validates error semantics
	meta, err := f.registryClient.GetMetadata(repo, version)
	if err != nil {
		// Transform registry errors into metadata errors
		// This validates our error handling strategy works across ecosystems
		return nil, &NetworkError{
			Source:  "go-proxy",
			URL:     fmt.Sprintf("https://proxy.golang.org/%s/@v/%s.info", repo, version),
			Wrapped: err,
		}
	}

	// Convert registry.Metadata to tools.Metadata
	// Both have PublishDate, but registry doesn't have download counts
	return &Metadata{
		Version:         version,
		PublishDate:     meta.PublishDate,
		TotalDownloads:  -1, // Go proxy doesn't provide download stats
		RecentDownloads: -1, // Not available
		Source:          "go-proxy",
	}, nil
}

// Integration validation notes:
//
// ✅ Error semantics: registry.Client.GetMetadata() returns generic errors
//    Our NetworkError wrapper provides consistent error handling across fetchers
//
// ✅ Caching: registry.GoClient has its own cache (24h TTL)
//    Our metadata.DefaultRegistry adds a second cache layer
//    Question: Is double-caching acceptable, or should we bypass registry cache?
//
// ✅ Metadata mapping: registry.Metadata has PublishDate but not downloads
//    We handle unavailable data consistently (TotalDownloads = -1)
//
// ⚠️  Repo format: Go modules use full import paths (github.com/owner/repo/cmd/tool)
//    GitHub uses "owner/repo" format
//    Need consistent repo normalization across fetchers
//
// ⚠️  Version format: Go modules use "v1.2.3" (always with 'v')
//    GitHub may or may not have 'v' prefix
//    Normalization handled in individual fetchers (good!)
//
// Future considerations:
// - Should GoModuleFetcher be in pkg/tools/metadata or pkg/registry?
// - Do we want to expose module proxy errors directly or wrap them?
// - How do we handle replace directives in go.mod (probably not relevant for tools)?
*/
