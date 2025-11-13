// Package metadata provides tool release metadata fetching for cooling policy evaluation.
//
// # Overview
//
// The metadata package enables goneat to fetch release information (publish date, download counts)
// for external tools before installation. This supports cooling policy enforcement by ensuring
// tools meet minimum age and popularity thresholds.
//
// # Architecture
//
// The package uses a registry pattern with multiple metadata fetchers:
//
//   - Registry: Coordinates fetchers and provides caching
//   - Fetcher: Interface for fetching metadata from different sources
//   - GitHub: Implements fetcher for GitHub Releases API
//
// # Basic Usage
//
//	// Create registry with 24-hour cache TTL
//	reg := metadata.NewRegistry(24 * time.Hour)
//
//	// Register GitHub fetcher (optionally with auth token)
//	githubFetcher := metadata.NewGitHubFetcher(os.Getenv("GITHUB_TOKEN"), 30*time.Second)
//	reg.RegisterFetcher("github", githubFetcher)
//
//	// Fetch metadata for a tool
//	meta, err := reg.GetMetadata("anchore/syft", "v1.33.0")
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	fmt.Printf("Published: %s\n", meta.PublishDate)
//	fmt.Printf("Downloads: %d\n", meta.TotalDownloads)
//
// # Caching
//
// The registry automatically caches metadata to reduce API calls:
//
//   - First call: Fetches from API (e.g., GitHub)
//   - Subsequent calls: Returns cached data (within TTL)
//   - Cache miss: Refetches after TTL expires
//
// Monitor cache performance with CacheStats():
//
//	stats := reg.CacheStats()
//	fmt.Printf("Hits: %d, Misses: %d, Size: %d\n", stats.Hits, stats.Misses, stats.Size)
//
// # GitHub Rate Limits
//
// GitHub API rate limits:
//
//   - Unauthenticated: 60 requests/hour
//   - Authenticated (with token): 5000 requests/hour
//
// Provide a GitHub personal access token via environment variable or configuration:
//
//	token := os.Getenv("GITHUB_TOKEN")
//	fetcher := metadata.NewGitHubFetcher(token, 30*time.Second)
//
// # Manual Metadata
//
// For tools where API is unavailable or for testing, provide metadata manually:
//
//	publishDate := time.Date(2024, 11, 1, 10, 0, 0, 0, time.UTC)
//	meta := metadata.ManualMetadata("v1.33.0", publishDate)
//
// # Testing
//
// The package supports testing via injectable HTTP clients:
//
//	mock := registry.NewMockHTTPFetcher()
//	mock.AddResponse("https://api.github.com/repos/test/repo/releases/tags/v1.0.0", 200, jsonResponse)
//	fetcher := metadata.NewGitHubFetcherWithHTTP(mock, "", 30*time.Second)
//
// # Future Extensions
//
// The Fetcher interface supports additional metadata sources:
//
//   - Go modules via proxy.golang.org
//   - Package managers (Homebrew, Scoop, apt, etc.)
//   - Custom registries or APIs
//
// To add a new source, implement the Fetcher interface and register it:
//
//	type CustomFetcher struct { ... }
//	func (f *CustomFetcher) FetchMetadata(repo, version string) (*Metadata, error) { ... }
//	func (f *CustomFetcher) SupportsRepo(repo string) bool { ... }
//
//	reg.RegisterFetcher("custom", customFetcher)
package metadata
