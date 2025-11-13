package metadata_test

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/fulmenhq/goneat/pkg/tools/metadata"
)

// ExampleNewRegistry demonstrates basic registry setup and usage
func ExampleNewRegistry() {
	// Create registry with 24-hour cache
	reg := metadata.NewRegistry(24 * time.Hour)

	// Get GitHub token from environment (optional but recommended)
	token := os.Getenv("GITHUB_TOKEN")

	// Create and register GitHub fetcher
	githubFetcher := metadata.NewGitHubFetcher(token, 30*time.Second)
	reg.RegisterFetcher("github", githubFetcher)

	// Fetch metadata for a tool
	meta, err := reg.GetMetadata("anchore/syft", "v1.33.0")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Tool: %s\n", meta.Version)
	fmt.Printf("Published: %s\n", meta.PublishDate.Format("2006-01-02"))
	fmt.Printf("Downloads: %d\n", meta.TotalDownloads)
	fmt.Printf("Source: %s\n", meta.Source)
}

// ExampleRegistry_GetMetadata demonstrates fetching metadata with caching
func ExampleRegistry_GetMetadata() {
	reg := metadata.NewRegistry(24 * time.Hour)
	githubFetcher := metadata.NewGitHubFetcher("", 30*time.Second)
	reg.RegisterFetcher("github", githubFetcher)

	// First call fetches from API
	meta1, _ := reg.GetMetadata("golangci/golangci-lint", "v2.4.0")
	fmt.Printf("First call source: %s\n", meta1.Source)

	// Second call uses cache
	meta2, _ := reg.GetMetadata("golangci/golangci-lint", "v2.4.0")
	fmt.Printf("Second call source: %s\n", meta2.Source)

	// Output:
	// First call source: github
	// Second call source: cache
}

// ExampleRegistry_CacheStats demonstrates monitoring cache performance
func ExampleRegistry_CacheStats() {
	reg := metadata.NewRegistry(24 * time.Hour)
	githubFetcher := metadata.NewGitHubFetcher("", 30*time.Second)
	reg.RegisterFetcher("github", githubFetcher)

	// Perform some fetches
	_, _ = reg.GetMetadata("anchore/syft", "v1.33.0")
	_, _ = reg.GetMetadata("anchore/syft", "v1.33.0") // Cache hit

	stats := reg.CacheStats()
	fmt.Printf("Cache hits: %d\n", stats.Hits)
	fmt.Printf("Cache misses: %d\n", stats.Misses)
	fmt.Printf("Cache size: %d\n", stats.Size)

	// Output:
	// Cache hits: 1
	// Cache misses: 1
	// Cache size: 1
}

// ExampleManualMetadata demonstrates creating metadata without API calls
func ExampleManualMetadata() {
	// For tools where API is unavailable or for testing
	publishDate := time.Date(2024, 11, 1, 10, 0, 0, 0, time.UTC)
	meta := metadata.ManualMetadata("v1.33.0", publishDate)

	fmt.Printf("Version: %s\n", meta.Version)
	fmt.Printf("Published: %s\n", meta.PublishDate.Format("2006-01-02"))
	fmt.Printf("Source: %s\n", meta.Source)

	// Output:
	// Version: v1.33.0
	// Published: 2024-11-01
	// Source: manual
}

// ExampleGitHubFetcher_SupportsRepo demonstrates repository format validation
func ExampleGitHubFetcher_SupportsRepo() {
	fetcher := metadata.NewGitHubFetcher("", 30*time.Second)

	repos := []string{
		"github.com/anchore/syft",
		"anchore/syft",
		"https://github.com/anchore/syft",
		"invalid",
	}

	for _, repo := range repos {
		supported := fetcher.SupportsRepo(repo)
		fmt.Printf("%s: %v\n", repo, supported)
	}

	// Output:
	// github.com/anchore/syft: true
	// anchore/syft: true
	// https://github.com/anchore/syft: true
	// invalid: false
}

// ExampleDefaultFetcherOptions demonstrates configuration options
func ExampleDefaultFetcherOptions() {
	opts := metadata.DefaultFetcherOptions()

	fmt.Printf("Cache TTL: %s\n", opts.CacheTTL)
	fmt.Printf("Timeout: %s\n", opts.Timeout)

	// Output:
	// Cache TTL: 24h0m0s
	// Timeout: 30s
}

// ExampleRegistry_ClearCache demonstrates clearing the cache
func ExampleRegistry_ClearCache() {
	reg := metadata.NewRegistry(24 * time.Hour)
	githubFetcher := metadata.NewGitHubFetcher("", 30*time.Second)
	reg.RegisterFetcher("github", githubFetcher)

	// Populate cache
	_, _ = reg.GetMetadata("anchore/syft", "v1.33.0")

	statsBefore := reg.CacheStats()
	fmt.Printf("Before clear - Size: %d\n", statsBefore.Size)

	// Clear cache
	reg.ClearCache()

	statsAfter := reg.CacheStats()
	fmt.Printf("After clear - Size: %d\n", statsAfter.Size)

	// Output:
	// Before clear - Size: 1
	// After clear - Size: 0
}
