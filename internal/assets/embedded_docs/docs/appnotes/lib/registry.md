---
title: "Registry Package"
description: "Multi-language package registry API clients with mockable HTTP transport"
library: "pkg/registry"
status: "Wave 2 Phase 1 Complete"
version: "v0.3.0"
last_updated: "2025-10-10"
tags:
  - "registry"
  - "api-client"
  - "testing"
  - "mockability"
  - "http"
---

# Registry Package

## Purpose

The `pkg/registry` package provides unified access to package registry APIs across multiple languages for fetching package metadata, including publish dates and download statistics. The package is designed with **mockability as a first-class concern** to enable fast, deterministic testing without hitting real APIs.

## Key Features

- **Multi-language support**: Go, npm, PyPI, crates.io, NuGet
- **Mockable HTTP transport**: Interface-based design for testing
- **Built-in caching**: 24-hour TTL with thread-safe access
- **TLS 1.2+ enforcement**: Security-first HTTP configuration
- **Conservative timeouts**: 30-second request timeouts
- **Failure resilience**: Graceful handling of API errors

## Architecture

### Package Structure

```
pkg/registry/
├── transport.go         # HTTPFetcher interface + implementations
├── client.go            # GoClient + factory function
├── npm_client.go        # NPM registry client
├── pypi_client.go       # PyPI registry client
├── crates_client.go     # crates.io registry client
├── nuget_client.go      # NuGet registry client
├── *_test.go            # Unit tests with mocks
└── testdata/            # Test fixtures (JSON responses)
```

### Core Abstraction: HTTPFetcher

The **HTTPFetcher interface** enables dependency injection for testing:

```go
type HTTPFetcher interface {
    Get(url string) (*http.Response, error)
    Do(req *http.Request) (*http.Response, error)
}
```

#### Implementations

1. **RealHTTPFetcher**: Production HTTP client wrapper
2. **MockHTTPFetcher**: In-memory response simulator for testing

## Installation

```go
import "github.com/fulmenhq/goneat/pkg/registry"
```

## Basic Usage

### Production Use

```go
// Create client with real HTTP
client := registry.NewGoClient(24 * time.Hour)

// Fetch metadata
metadata, err := client.GetMetadata("github.com/spf13/cobra", "v1.8.0")
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Published: %s\n", metadata.PublishDate)
fmt.Printf("Downloads: %d\n", metadata.TotalDownloads)
```

### Multi-Language Support

```go
// Factory creates appropriate client for language
goClient := registry.NewClient("go", 24*time.Hour)
npmClient := registry.NewClient("npm", 24*time.Hour)
pypiClient := registry.NewClient("python", 24*time.Hour)
cratesClient := registry.NewClient("rust", 24*time.Hour)
nugetClient := registry.NewClient("csharp", 24*time.Hour)
```

## Testing with Mocks

### Creating Mock HTTP Responses

```go
func TestGoClient_GetMetadata_Mock(t *testing.T) {
    // Load test fixture
    fixtureData, _ := os.ReadFile("testdata/go_proxy_cobra_v1.8.0.json")

    // Create mock fetcher
    mock := registry.NewMockHTTPFetcher()
    mock.AddResponse(
        "https://proxy.golang.org/github.com/spf13/cobra/@v/v1.8.0.info",
        200,
        string(fixtureData),
    )

    // Create client with mock
    client := registry.NewGoClientWithFetcher(24*time.Hour, mock)

    // Test without hitting real API
    meta, err := client.GetMetadata("github.com/spf13/cobra", "v1.8.0")
    assert.NoError(t, err)
    assert.Equal(t, expectedDate, meta.PublishDate)
}
```

### Simulating Errors

```go
func TestGoClient_NetworkError(t *testing.T) {
    mock := registry.NewMockHTTPFetcher()
    mock.AddError(
        "https://proxy.golang.org/invalid/package/@v/v1.0.0.info",
        errors.New("network timeout"),
    )

    client := registry.NewGoClientWithFetcher(24*time.Hour, mock)

    _, err := client.GetMetadata("invalid/package", "v1.0.0")
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "network timeout")
}
```

### Test Fixtures

Place JSON fixtures in `testdata/`:

```
testdata/
├── go_proxy_cobra_v1.8.0.json
├── npm_lodash.json
├── npm_lodash_downloads.json
├── pypi_requests_2.31.0.json
├── crates_serde_1.0.195.json
├── nuget_service_index.json
└── nuget_newtonsoft_json.json
```

## Client Implementations

### Go Client

**Registry**: `https://proxy.golang.org`

```go
type GoClient struct {
    cache   map[string]*cacheEntry
    mu      sync.RWMutex
    ttl     time.Duration
    fetcher HTTPFetcher
}
```

**API Endpoints**:

- Module info: `https://proxy.golang.org/{module}/@v/{version}.info`

**Note**: Go proxy doesn't provide download stats; uses conservative defaults (1000/100).

### npm Client

**Registry**: `https://registry.npmjs.org`

```go
type NPMClient struct {
    baseURL      string
    downloadsURL string
    cache        map[string]*cacheEntry
    mu           sync.RWMutex
    ttl          time.Duration
    fetcher      HTTPFetcher
}
```

**API Endpoints**:

- Package metadata: `https://registry.npmjs.org/{package}`
- Downloads: `https://api.npmjs.org/downloads/point/last-month/{package}`

### PyPI Client

**Registry**: `https://pypi.org/pypi`

```go
type PyPIClient struct {
    baseURL string
    cache   map[string]*cacheEntry
    mu      sync.RWMutex
    ttl     time.Duration
    fetcher HTTPFetcher
}
```

**API Endpoints**:

- Package metadata: `https://pypi.org/pypi/{package}/{version}/json`

**Note**: PyPI no longer provides download stats via JSON API; uses conservative defaults.

### crates.io Client

**Registry**: `https://crates.io/api/v1`

```go
type CratesClient struct {
    baseURL string
    cache   map[string]*cacheEntry
    mu      sync.RWMutex
    ttl     time.Duration
    fetcher HTTPFetcher
}
```

**API Endpoints**:

- Crate metadata: `https://crates.io/api/v1/crates/{name}`

**Special Requirements**:

- Must include `User-Agent` header: `goneat/0.3.0 (https://github.com/fulmenhq/goneat)`

### NuGet Client

**Registry**: `https://api.nuget.org/v3`

```go
type NuGetClient struct {
    serviceIndexURL string
    packageBaseURL  string // Cached from service index
    cache           map[string]*cacheEntry
    mu              sync.RWMutex
    ttl             time.Duration
    fetcher         HTTPFetcher
}
```

**API Endpoints**:

- Service index: `https://api.nuget.org/v3/index.json`
- Package metadata: `{PackageBaseAddress}/{id}/index.json`

**Special Behavior**:

- Discovers `PackageBaseAddress` from service index on first request
- Caches service index URL for subsequent requests

**Note**: NuGet V3 API doesn't provide download stats; uses conservative defaults.

## API Reference

### Client Interface

```go
type Client interface {
    GetMetadata(name, version string) (*Metadata, error)
}
```

### Metadata Structure

```go
type Metadata struct {
    PublishDate     time.Time
    TotalDownloads  int
    RecentDownloads int
}
```

### HTTPFetcher Interface

```go
type HTTPFetcher interface {
    Get(url string) (*http.Response, error)
    Do(req *http.Request) (*http.Response, error)
}
```

### Mock HTTP Fetcher

```go
type MockHTTPFetcher struct {
    responses map[string]*http.Response
    errors    map[string]error
}

func NewMockHTTPFetcher() *MockHTTPFetcher
func (m *MockHTTPFetcher) AddResponse(url string, statusCode int, body string)
func (m *MockHTTPFetcher) AddError(url string, err error)
```

## Dual Constructor Pattern

Each client provides two constructors for flexibility:

### Production Constructor

```go
// Creates client with real HTTP
func NewGoClient(ttl time.Duration) Client {
    client := &http.Client{
        Timeout: 30 * time.Second,
        Transport: &http.Transport{
            TLSClientConfig: &tls.Config{
                MinVersion: tls.VersionTLS12,
            },
        },
    }
    return NewGoClientWithFetcher(ttl, NewRealHTTPFetcher(client))
}
```

### Testing Constructor

```go
// Creates client with injectable fetcher
func NewGoClientWithFetcher(ttl time.Duration, fetcher HTTPFetcher) Client {
    return &GoClient{
        cache:   make(map[string]*cacheEntry),
        ttl:     ttl,
        fetcher: fetcher,
    }
}
```

## Best Practices

### 1. Use Test Fixtures

Store real API responses as JSON fixtures:

```bash
# Capture real API response
curl https://registry.npmjs.org/lodash > testdata/npm_lodash.json
```

### 2. Test Both Mocked and Real APIs

```go
// Unit test with mock (fast, always run)
func TestNPMClient_GetMetadata_Mock(t *testing.T) { ... }

// Integration test with real API (slow, skip in CI)
func TestNPMClient_GetMetadata_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }
    // Test with real API...
}
```

### 3. Handle Rate Limits Gracefully

```go
metadata, err := client.GetMetadata(name, version)
if err != nil {
    if strings.Contains(err.Error(), "rate limit") {
        // Implement exponential backoff
        time.Sleep(backoff)
        // Retry...
    }
}
```

### 4. Use Appropriate Cache TTL

```go
// Development: short TTL for fresh data
devClient := registry.NewGoClient(1 * time.Hour)

// Production: longer TTL to reduce API calls
prodClient := registry.NewGoClient(24 * time.Hour)
```

## Anti-Patterns

### ❌ Don't Hit Real APIs in Unit Tests

```go
// Bad: network call in unit test
func TestClient(t *testing.T) {
    client := registry.NewGoClient(1*time.Hour) // Real HTTP!
    meta, _ := client.GetMetadata("pkg", "v1.0.0")
}

// Good: mock HTTP in unit test
func TestClient(t *testing.T) {
    mock := registry.NewMockHTTPFetcher()
    mock.AddResponse(url, 200, fixture)
    client := registry.NewGoClientWithFetcher(1*time.Hour, mock)
    meta, _ := client.GetMetadata("pkg", "v1.0.0")
}
```

### ❌ Don't Ignore defer Body.Close() Errors

```go
// Bad: unchecked Close()
resp, err := client.Get(url)
defer resp.Body.Close()

// Good: explicit close check
resp, err := client.Get(url)
if err != nil {
    return err
}
defer func() { _ = resp.Body.Close() }()
```

### ❌ Don't Share Clients Across Goroutines Without Care

```go
// Good: clients are thread-safe with internal mutex
client := registry.NewGoClient(ttl)

var wg sync.WaitGroup
for _, pkg := range packages {
    wg.Add(1)
    go func(p string) {
        defer wg.Done()
        meta, _ := client.GetMetadata(p, "v1.0.0") // Safe
    }(pkg)
}
wg.Wait()
```

## Error Handling

### Registry API Errors

```go
metadata, err := client.GetMetadata(name, version)
if err != nil {
    // Conservative fallback for cooling policy
    return &Metadata{
        PublishDate:     time.Now().Add(-365 * 24 * time.Hour),
        TotalDownloads:  1000,
        RecentDownloads: 100,
    }
}
```

### HTTP Status Codes

```go
if resp.StatusCode != 200 {
    switch resp.StatusCode {
    case 404:
        return nil, fmt.Errorf("package not found: %s", name)
    case 429:
        return nil, fmt.Errorf("rate limit exceeded")
    default:
        return nil, fmt.Errorf("registry returned status %d", resp.StatusCode)
    }
}
```

## Caching Behavior

### Cache Entry Structure

```go
type cacheEntry struct {
    meta   *Metadata
    expiry time.Time
}
```

### Cache Lookup Logic

```go
func (c *GoClient) GetMetadata(name, version string) (*Metadata, error) {
    key := fmt.Sprintf("%s@%s", name, version)

    // Check cache
    c.mu.RLock()
    entry, ok := c.cache[key]
    c.mu.RUnlock()

    if ok && time.Now().Before(entry.expiry) {
        return entry.meta, nil // Cache hit
    }

    // Fetch from API...
    // Store in cache with TTL
    c.mu.Lock()
    c.cache[key] = &cacheEntry{
        meta:   metadata,
        expiry: time.Now().Add(c.ttl),
    }
    c.mu.Unlock()

    return metadata, nil
}
```

## Security Considerations

### TLS Configuration

All clients enforce TLS 1.2 or higher:

```go
Transport: &http.Transport{
    TLSClientConfig: &tls.Config{
        MinVersion: tls.VersionTLS12,
    },
}
```

### Timeout Protection

30-second timeouts prevent hanging requests:

```go
client := &http.Client{
    Timeout: 30 * time.Second,
}
```

### Thread Safety

All clients use `sync.RWMutex` for cache access:

```go
c.mu.RLock()
entry, ok := c.cache[key]
c.mu.RUnlock()
```

## Performance Characteristics

### Cache Hit Rate

With 24-hour TTL and typical project analysis:

- **First run**: 0% cache hits, ~2s per 100 dependencies
- **Subsequent runs**: ~95% cache hits, ~50ms per 100 dependencies

### Concurrency

Clients are safe for concurrent use:

```go
// Process 100 packages concurrently
var wg sync.WaitGroup
for _, pkg := range packages {
    wg.Add(1)
    go func(p Package) {
        defer wg.Done()
        metadata, _ := client.GetMetadata(p.Name, p.Version)
        // Process metadata...
    }(pkg)
}
wg.Wait()
```

## CI/CD Testing Strategy

### Makefile Targets

```makefile
# Fast unit tests with mocks (always run)
test-unit:
	go test ./pkg/registry/... -short -v

# Slow integration tests with real APIs (nightly)
test-integration:
	go test ./pkg/registry/... -v -run Integration
```

### GitHub Actions

```yaml
# PR checks: unit tests only
- name: Unit Tests
  run: make test-unit

# Nightly: full integration tests
- name: Integration Tests
  run: make test-integration
  if: github.event_name == 'schedule'
```

## See Also

- [Dependencies Package](dependencies.md) - Consumer of registry clients
- [OPA v1 Migration ADR](../architecture/decisions/adr-0001-opa-v1-rego-v1-migration.md) - Related policy engine work
- Wave 2 Spec: `.plans/active/v0.3.0/wave-2-detailed-spec.md`

## References

- Go Proxy API: https://proxy.golang.org
- npm Registry API: https://github.com/npm/registry/blob/master/docs/REGISTRY-API.md
- PyPI JSON API: https://warehouse.pypa.io/api-reference/json.html
- crates.io API: https://crates.io/data-access
- NuGet Service Index: https://learn.microsoft.com/en-us/nuget/api/service-index
