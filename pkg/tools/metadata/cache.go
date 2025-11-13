package metadata

import (
	"fmt"
	"sync"
	"time"
)

// Cache TTL Strategy:
//
// The metadata cache is intentionally separate from pkg/registry/client.go to prevent
// registry changes from invalidating tool metadata unexpectedly. This isolation ensures:
//   - Metadata TTL can differ from dependency registry TTL (24h for tools vs registry needs)
//   - Tool cooling policy remains stable even when dependency lookups refresh
//   - Clear separation of concerns: registry = Go modules, metadata = external tools
//
// Default TTL: 24 hours
//   - Rationale: Tool releases are infrequent (days/weeks between releases)
//   - Balance: Fresh enough for new releases, long enough to avoid rate limits
//   - Future fetchers: Adjust TTL based on ecosystem (e.g., npm may need shorter TTL)
//
// Staleness considerations:
//   - GitHub releases: Publish date is immutable once released
//   - Download counts: May lag by 1-2 hours, acceptable for cooling policy
//   - Version tags: Never change (semantic versioning guarantee)

// cacheEntry holds cached metadata with expiry time
type cacheEntry struct {
	metadata *Metadata
	expiry   time.Time
}

// DefaultRegistry implements Registry with caching
type DefaultRegistry struct {
	fetchers map[string]Fetcher
	cache    map[string]*cacheEntry
	mu       sync.RWMutex
	ttl      time.Duration
	stats    CacheStats
}

// NewRegistry creates a new metadata registry with caching
func NewRegistry(ttl time.Duration) Registry {
	return &DefaultRegistry{
		fetchers: make(map[string]Fetcher),
		cache:    make(map[string]*cacheEntry),
		ttl:      ttl,
		stats:    CacheStats{},
	}
}

// RegisterFetcher adds a metadata fetcher to the registry
func (r *DefaultRegistry) RegisterFetcher(name string, fetcher Fetcher) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.fetchers[name] = fetcher
}

// GetMetadata retrieves metadata for a tool, using cache if available
func (r *DefaultRegistry) GetMetadata(repo, version string) (*Metadata, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("%s@%s", repo, version)

	r.mu.RLock()
	entry, ok := r.cache[cacheKey]
	r.mu.RUnlock()

	if ok && time.Now().Before(entry.expiry) {
		r.mu.Lock()
		r.stats.Hits++
		r.mu.Unlock()

		// Return cached copy with updated source
		cached := *entry.metadata
		cached.Source = "cache"
		return &cached, nil
	}

	// Cache miss or expired
	r.mu.Lock()
	r.stats.Misses++
	r.mu.Unlock()

	// Find appropriate fetcher
	var fetcher Fetcher
	var fetcherName string

	r.mu.RLock()
	for name, f := range r.fetchers {
		if f.SupportsRepo(repo) {
			fetcher = f
			fetcherName = name
			break
		}
	}
	r.mu.RUnlock()

	if fetcher == nil {
		return nil, fmt.Errorf("no fetcher available for repository: %s", repo)
	}

	// Fetch metadata
	meta, err := fetcher.FetchMetadata(repo, version)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch metadata via %s: %w", fetcherName, err)
	}

	// Cache the result
	r.mu.Lock()
	r.cache[cacheKey] = &cacheEntry{
		metadata: meta,
		expiry:   time.Now().Add(r.ttl),
	}
	r.mu.Unlock()

	return meta, nil
}

// ClearCache removes all cached entries
func (r *DefaultRegistry) ClearCache() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.cache = make(map[string]*cacheEntry)
	r.stats = CacheStats{}
}

// CacheStats returns current cache performance statistics
func (r *DefaultRegistry) CacheStats() CacheStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return CacheStats{
		Hits:   r.stats.Hits,
		Misses: r.stats.Misses,
		Size:   len(r.cache),
	}
}

// ManualMetadata creates metadata from manually-provided values
// Useful when API is unavailable or for testing
func ManualMetadata(version string, publishDate time.Time) *Metadata {
	return &Metadata{
		Version:         version,
		PublishDate:     publishDate,
		TotalDownloads:  -1, // Unknown
		RecentDownloads: -1, // Unknown
		Source:          "manual",
	}
}
