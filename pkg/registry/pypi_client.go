package registry

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// PyPIClient implements Client for PyPI registry
type PyPIClient struct {
	baseURL string
	cache   map[string]*cacheEntry
	mu      sync.RWMutex
	ttl     time.Duration
	fetcher HTTPFetcher
}

// NewPyPIClient creates a PyPIClient with real HTTP for production use
func NewPyPIClient(ttl time.Duration) Client {
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
		},
	}

	return NewPyPIClientWithFetcher(ttl, NewRealHTTPFetcher(client))
}

// NewPyPIClientWithFetcher creates a PyPIClient with injectable HTTP for testing
func NewPyPIClientWithFetcher(ttl time.Duration, fetcher HTTPFetcher) Client {
	return &PyPIClient{
		baseURL: "https://pypi.org/pypi",
		cache:   make(map[string]*cacheEntry),
		ttl:     ttl,
		fetcher: fetcher,
	}
}

func (c *PyPIClient) GetMetadata(name, version string) (*Metadata, error) {
	key := fmt.Sprintf("%s@%s", name, version)
	c.mu.RLock()
	entry, ok := c.cache[key]
	c.mu.RUnlock()

	if ok && time.Now().Before(entry.expiry) {
		return entry.meta, nil
	}

	// Fetch package metadata
	pkgURL := fmt.Sprintf("%s/%s/%s/json", c.baseURL, name, version)
	pkgResp, err := c.fetcher.Get(pkgURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch package metadata: %w", err)
	}
	defer pkgResp.Body.Close()

	if pkgResp.StatusCode != 200 {
		return nil, fmt.Errorf("PyPI registry returned status %d", pkgResp.StatusCode)
	}

	var pkgData struct {
		Info struct {
			Version string `json:"version"`
		} `json:"info"`
		Releases map[string][]struct {
			UploadTime string `json:"upload_time"`
			Downloads  int    `json:"downloads"` // Note: PyPI doesn't provide this anymore
		} `json:"releases"`
	}

	if err := json.NewDecoder(pkgResp.Body).Decode(&pkgData); err != nil {
		_ = pkgResp.Body.Close()
		return nil, fmt.Errorf("failed to decode package metadata: %w", err)
	}

	// Get publish date for this version
	releases, ok := pkgData.Releases[version]
	if !ok || len(releases) == 0 {
		return nil, fmt.Errorf("version %s not found in package metadata", version)
	}

	// Use the first release's upload time
	publishDate, err := time.Parse("2006-01-02T15:04:05", releases[0].UploadTime)
	if err != nil {
		return nil, fmt.Errorf("failed to parse publish date: %w", err)
	}

	// PyPI no longer provides download stats via JSON API
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
