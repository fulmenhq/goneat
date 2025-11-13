package metadata

import (
	"time"
)

// Metadata contains tool release information for cooling policy evaluation
type Metadata struct {
	// Version is the tool version (e.g., "v1.33.0", "1.33.0")
	Version string

	// PublishDate is when this version was published/released
	PublishDate time.Time

	// TotalDownloads is the total number of downloads (if available)
	// Set to -1 if unavailable
	TotalDownloads int

	// RecentDownloads is the download count in a recent period (typically 30 days)
	// Set to -1 if unavailable
	//
	// TODO(Phase 2): RecentDownloads unavailable via GitHub REST API
	// GitHub's REST API only provides per-asset download counts (total lifetime downloads).
	// To get recent download stats, we would need:
	//   - GitHub Traffic API (requires repo push access, not viable for public tools)
	//   - Third-party analytics services (e.g., Sourcegraph, libraries.io)
	//   - Custom scraping/tracking (maintenance burden)
	// Recommendation: Leave as -1 for now, revisit if cooling policy requires recent activity.
	RecentDownloads int

	// Source indicates where metadata was fetched from (e.g., "github", "manual", "cache")
	Source string
}

// FetcherOptions configures metadata fetching behavior
type FetcherOptions struct {
	// CacheTTL is how long to cache metadata before refetching
	// Default: 24 hours
	CacheTTL time.Duration

	// GitHubToken is an optional GitHub personal access token for higher rate limits
	// If empty, uses unauthenticated requests (60 requests/hour)
	GitHubToken string

	// Timeout for HTTP requests
	// Default: 30 seconds
	Timeout time.Duration
}

// DefaultFetcherOptions returns sensible defaults
func DefaultFetcherOptions() FetcherOptions {
	return FetcherOptions{
		CacheTTL: 24 * time.Hour,
		Timeout:  30 * time.Second,
	}
}

// Fetcher retrieves metadata for tool releases
type Fetcher interface {
	// FetchMetadata fetches metadata for a specific tool version
	// Returns error if metadata cannot be fetched
	FetchMetadata(repo, version string) (*Metadata, error)

	// SupportsRepo returns true if this fetcher can handle the given repo format
	// Examples: "github.com/anchore/syft", "anchore/syft"
	SupportsRepo(repo string) bool
}

// Registry coordinates multiple metadata fetchers
type Registry interface {
	// GetMetadata attempts to fetch metadata using appropriate fetcher
	// Returns cached result if available and not expired
	GetMetadata(repo, version string) (*Metadata, error)

	// RegisterFetcher adds a new fetcher to the registry
	RegisterFetcher(name string, fetcher Fetcher)

	// ClearCache removes all cached entries
	ClearCache()

	// CacheStats returns cache hit/miss statistics for monitoring
	CacheStats() CacheStats
}

// CacheStats provides cache performance metrics
type CacheStats struct {
	Hits   int
	Misses int
	Size   int
}
