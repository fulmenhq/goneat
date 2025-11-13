package metadata

import (
	"fmt"
	"testing"
	"time"

	"github.com/fulmenhq/goneat/pkg/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRegistry(t *testing.T) {
	reg := NewRegistry(24 * time.Hour)
	require.NotNil(t, reg)

	stats := reg.CacheStats()
	assert.Equal(t, 0, stats.Hits)
	assert.Equal(t, 0, stats.Misses)
	assert.Equal(t, 0, stats.Size)
}

func TestRegistry_RegisterFetcher(t *testing.T) {
	reg := NewRegistry(24 * time.Hour)
	mock := registry.NewMockHTTPFetcher()
	fetcher := NewGitHubFetcherWithHTTP(mock, "", 30*time.Second)

	reg.RegisterFetcher("github", fetcher)

	// Verify fetcher is registered by attempting to fetch
	mock.AddResponse(
		"https://api.github.com/repos/test/repo/releases/tags/v1.0.0",
		200,
		`{
			"tag_name": "v1.0.0",
			"published_at": "2024-11-01T10:00:00Z",
			"assets": []
		}`,
	)

	meta, err := reg.GetMetadata("test/repo", "v1.0.0")
	require.NoError(t, err)
	assert.NotNil(t, meta)
}

func TestRegistry_GetMetadata_CacheHit(t *testing.T) {
	reg := NewRegistry(1 * time.Hour)
	mock := registry.NewMockHTTPFetcher()
	fetcher := NewGitHubFetcherWithHTTP(mock, "", 30*time.Second)
	reg.RegisterFetcher("github", fetcher)

	mock.AddResponse(
		"https://api.github.com/repos/anchore/syft/releases/tags/v1.33.0",
		200,
		`{
			"tag_name": "v1.33.0",
			"published_at": "2024-11-01T10:00:00Z",
			"assets": [{"name": "test.tar.gz", "download_count": 1000}]
		}`,
	)

	// First call - cache miss
	meta1, err := reg.GetMetadata("anchore/syft", "v1.33.0")
	require.NoError(t, err)
	assert.Equal(t, "github", meta1.Source)

	stats1 := reg.CacheStats()
	assert.Equal(t, 0, stats1.Hits)
	assert.Equal(t, 1, stats1.Misses)
	assert.Equal(t, 1, stats1.Size)

	// Second call - cache hit
	meta2, err := reg.GetMetadata("anchore/syft", "v1.33.0")
	require.NoError(t, err)
	assert.Equal(t, "cache", meta2.Source)

	stats2 := reg.CacheStats()
	assert.Equal(t, 1, stats2.Hits)
	assert.Equal(t, 1, stats2.Misses)
	assert.Equal(t, 1, stats2.Size)

	// Verify data is same
	assert.Equal(t, meta1.Version, meta2.Version)
	assert.Equal(t, meta1.PublishDate, meta2.PublishDate)
	assert.Equal(t, meta1.TotalDownloads, meta2.TotalDownloads)
}

func TestRegistry_GetMetadata_CacheExpiry(t *testing.T) {
	// Short TTL for testing
	reg := NewRegistry(100 * time.Millisecond)
	mock := registry.NewMockHTTPFetcher()
	fetcher := NewGitHubFetcherWithHTTP(mock, "", 30*time.Second)
	reg.RegisterFetcher("github", fetcher)

	response := `{
		"tag_name": "v1.0.0",
		"published_at": "2024-11-01T10:00:00Z",
		"assets": []
	}`

	mock.AddResponse(
		"https://api.github.com/repos/test/tool/releases/tags/v1.0.0",
		200,
		response,
	)

	// First call
	meta1, err := reg.GetMetadata("test/tool", "v1.0.0")
	require.NoError(t, err)
	assert.Equal(t, "github", meta1.Source)

	// Wait for cache to expire
	time.Sleep(150 * time.Millisecond)

	// Re-add mock response for refetch
	mock.AddResponse(
		"https://api.github.com/repos/test/tool/releases/tags/v1.0.0",
		200,
		response,
	)

	// Second call after expiry - should refetch
	meta2, err := reg.GetMetadata("test/tool", "v1.0.0")
	require.NoError(t, err)
	assert.Equal(t, "github", meta2.Source) // Fresh fetch, not from cache

	stats := reg.CacheStats()
	assert.Equal(t, 0, stats.Hits)   // No cache hits due to expiry
	assert.Equal(t, 2, stats.Misses) // Two misses (initial + expired)
}

func TestRegistry_GetMetadata_NoFetcherAvailable(t *testing.T) {
	reg := NewRegistry(24 * time.Hour)

	// No fetchers registered
	meta, err := reg.GetMetadata("unknown.com/test/repo", "v1.0.0")
	require.Error(t, err)
	assert.Nil(t, meta)
	assert.Contains(t, err.Error(), "no fetcher available")
}

func TestRegistry_GetMetadata_FetcherError(t *testing.T) {
	reg := NewRegistry(24 * time.Hour)
	mock := registry.NewMockHTTPFetcher()
	fetcher := NewGitHubFetcherWithHTTP(mock, "", 30*time.Second)
	reg.RegisterFetcher("github", fetcher)

	// Mock error response
	mock.AddError(
		"https://api.github.com/repos/test/repo/releases/tags/v1.0.0",
		fmt.Errorf("network error"),
	)

	meta, err := reg.GetMetadata("test/repo", "v1.0.0")
	require.Error(t, err)
	assert.Nil(t, meta)
	assert.Contains(t, err.Error(), "failed to fetch metadata")
}

func TestRegistry_ClearCache(t *testing.T) {
	reg := NewRegistry(1 * time.Hour)
	mock := registry.NewMockHTTPFetcher()
	fetcher := NewGitHubFetcherWithHTTP(mock, "", 30*time.Second)
	reg.RegisterFetcher("github", fetcher)

	mock.AddResponse(
		"https://api.github.com/repos/test/tool/releases/tags/v1.0.0",
		200,
		`{
			"tag_name": "v1.0.0",
			"published_at": "2024-11-01T10:00:00Z",
			"assets": []
		}`,
	)

	// Populate cache
	_, err := reg.GetMetadata("test/tool", "v1.0.0")
	require.NoError(t, err)

	stats1 := reg.CacheStats()
	assert.Equal(t, 1, stats1.Size)
	assert.Equal(t, 1, stats1.Misses)

	// Clear cache
	reg.ClearCache()

	stats2 := reg.CacheStats()
	assert.Equal(t, 0, stats2.Size)
	assert.Equal(t, 0, stats2.Hits)
	assert.Equal(t, 0, stats2.Misses)
}

func TestRegistry_MultipleFetchers(t *testing.T) {
	reg := NewRegistry(24 * time.Hour)

	// Register GitHub fetcher
	mockGitHub := registry.NewMockHTTPFetcher()
	githubFetcher := NewGitHubFetcherWithHTTP(mockGitHub, "", 30*time.Second)
	reg.RegisterFetcher("github", githubFetcher)

	// Mock GitHub response
	mockGitHub.AddResponse(
		"https://api.github.com/repos/anchore/syft/releases/tags/v1.33.0",
		200,
		`{
			"tag_name": "v1.33.0",
			"published_at": "2024-11-01T10:00:00Z",
			"assets": []
		}`,
	)

	// Fetch from GitHub
	meta, err := reg.GetMetadata("anchore/syft", "v1.33.0")
	require.NoError(t, err)
	assert.Equal(t, "github", meta.Source)

	// Future: Could register additional fetchers for other sources
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	reg := NewRegistry(1 * time.Hour)
	mock := registry.NewMockHTTPFetcher()
	fetcher := NewGitHubFetcherWithHTTP(mock, "", 30*time.Second)
	reg.RegisterFetcher("github", fetcher)

	mock.AddResponse(
		"https://api.github.com/repos/test/tool/releases/tags/v1.0.0",
		200,
		`{
			"tag_name": "v1.0.0",
			"published_at": "2024-11-01T10:00:00Z",
			"assets": []
		}`,
	)

	// Prime the cache with first call
	_, err := reg.GetMetadata("test/tool", "v1.0.0")
	require.NoError(t, err)

	// Now do concurrent reads from cache (no HTTP calls)
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			meta, err := reg.GetMetadata("test/tool", "v1.0.0")
			assert.NoError(t, err)
			assert.Equal(t, "cache", meta.Source)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should have cache hits from concurrent access
	stats := reg.CacheStats()
	assert.Equal(t, 1, stats.Size)
	assert.Equal(t, 1, stats.Misses) // Initial fetch
	assert.Equal(t, 10, stats.Hits)  // All concurrent reads from cache
}

func TestManualMetadata(t *testing.T) {
	publishDate := time.Date(2024, 11, 1, 10, 0, 0, 0, time.UTC)
	meta := ManualMetadata("v1.33.0", publishDate)

	assert.Equal(t, "v1.33.0", meta.Version)
	assert.True(t, meta.PublishDate.Equal(publishDate))
	assert.Equal(t, -1, meta.TotalDownloads)
	assert.Equal(t, -1, meta.RecentDownloads)
	assert.Equal(t, "manual", meta.Source)
}
