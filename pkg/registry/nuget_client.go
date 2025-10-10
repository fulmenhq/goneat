package registry

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// NuGetClient implements Client for NuGet registry
type NuGetClient struct {
	serviceIndexURL string
	cache           map[string]*cacheEntry
	mu              sync.RWMutex
	ttl             time.Duration
	fetcher         HTTPFetcher
	packageBaseURL  string // Cached from service index
}

// NewNuGetClient creates a NuGetClient with real HTTP for production use
func NewNuGetClient(ttl time.Duration) Client {
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
		},
	}

	return NewNuGetClientWithFetcher(ttl, NewRealHTTPFetcher(client))
}

// NewNuGetClientWithFetcher creates a NuGetClient with injectable HTTP for testing
func NewNuGetClientWithFetcher(ttl time.Duration, fetcher HTTPFetcher) Client {
	return &NuGetClient{
		serviceIndexURL: "https://api.nuget.org/v3/index.json",
		cache:           make(map[string]*cacheEntry),
		ttl:             ttl,
		fetcher:         fetcher,
	}
}

func (c *NuGetClient) GetMetadata(name, version string) (*Metadata, error) {
	key := fmt.Sprintf("%s@%s", name, version)
	c.mu.RLock()
	entry, ok := c.cache[key]
	c.mu.RUnlock()

	if ok && time.Now().Before(entry.expiry) {
		return entry.meta, nil
	}

	// Discover package base URL if not cached
	if c.packageBaseURL == "" {
		if err := c.discoverServiceIndex(); err != nil {
			return nil, fmt.Errorf("failed to discover service index: %w", err)
		}
	}

	// Fetch package metadata (trim trailing slash if present)
	baseURL := c.packageBaseURL
	if baseURL[len(baseURL)-1] == '/' {
		baseURL = baseURL[:len(baseURL)-1]
	}
	pkgURL := fmt.Sprintf("%s/%s/index.json", baseURL, name)
	pkgResp, err := c.fetcher.Get(pkgURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch package metadata: %w", err)
	}
	defer func() { _ = pkgResp.Body.Close() }()

	if pkgResp.StatusCode != 200 {
		return nil, fmt.Errorf("NuGet registry returned status %d", pkgResp.StatusCode)
	}

	var pkgData struct {
		Items []struct {
			CatalogEntry struct {
				Version   string    `json:"version"`
				Published time.Time `json:"published"`
			} `json:"catalogEntry"`
		} `json:"items"`
	}

	if err := json.NewDecoder(pkgResp.Body).Decode(&pkgData); err != nil {
		_ = pkgResp.Body.Close()
		return nil, fmt.Errorf("failed to decode package metadata: %w", err)
	}

	// Find the requested version
	var publishDate time.Time
	found := false

	for _, item := range pkgData.Items {
		if item.CatalogEntry.Version == version {
			publishDate = item.CatalogEntry.Published
			found = true
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("version %s not found in package metadata", version)
	}

	// NuGet doesn't provide download stats via V3 API
	// Use conservative defaults
	totalDownloads := 1000
	recentDownloads := 100

	meta := &Metadata{
		PublishDate:     publishDate,
		TotalDownloads:  totalDownloads,
		RecentDownloads: recentDownloads,
	}

	c.mu.Lock()
	c.cache[key] = &cacheEntry{meta: meta, expiry: time.Now().Add(c.ttl)}
	c.mu.Unlock()

	return meta, nil
}

func (c *NuGetClient) discoverServiceIndex() error {
	resp, err := c.fetcher.Get(c.serviceIndexURL)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		return fmt.Errorf("service index returned status %d", resp.StatusCode)
	}

	var indexData struct {
		Resources []struct {
			Type string `json:"@type"`
			ID   string `json:"@id"`
		} `json:"resources"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&indexData); err != nil {
		_ = resp.Body.Close()
		return err
	}

	// Find PackageBaseAddress resource
	for _, res := range indexData.Resources {
		if res.Type == "PackageBaseAddress/3.0.0" {
			c.packageBaseURL = res.ID
			return nil
		}
	}

	return fmt.Errorf("PackageBaseAddress resource not found in service index")
}
