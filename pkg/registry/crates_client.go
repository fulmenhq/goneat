package registry

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// CratesClient implements Client for crates.io registry
type CratesClient struct {
	baseURL string
	cache   map[string]*cacheEntry
	mu      sync.RWMutex
	ttl     time.Duration
	fetcher HTTPFetcher
}

// NewCratesClient creates a CratesClient with real HTTP for production use
func NewCratesClient(ttl time.Duration) Client {
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
		},
	}

	return NewCratesClientWithFetcher(ttl, NewRealHTTPFetcher(client))
}

// NewCratesClientWithFetcher creates a CratesClient with injectable HTTP for testing
func NewCratesClientWithFetcher(ttl time.Duration, fetcher HTTPFetcher) Client {
	return &CratesClient{
		baseURL: "https://crates.io/api/v1",
		cache:   make(map[string]*cacheEntry),
		ttl:     ttl,
		fetcher: fetcher,
	}
}

func (c *CratesClient) GetMetadata(name, version string) (*Metadata, error) {
	key := fmt.Sprintf("%s@%s", name, version)
	c.mu.RLock()
	entry, ok := c.cache[key]
	c.mu.RUnlock()

	if ok && time.Now().Before(entry.expiry) {
		return entry.meta, nil
	}

	// Fetch crate metadata
	crateURL := fmt.Sprintf("%s/crates/%s", c.baseURL, name)

	// Create request with User-Agent (required by crates.io)
	req, err := http.NewRequest("GET", crateURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "goneat/0.3.0 (https://github.com/fulmenhq/goneat)")

	crateResp, err := c.fetcher.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch crate metadata: %w", err)
	}
	defer func() { _ = crateResp.Body.Close() }()

	if crateResp.StatusCode != 200 {
		return nil, fmt.Errorf("crates.io registry returned status %d", crateResp.StatusCode)
	}

	var crateData struct {
		Crate struct {
			Downloads int `json:"downloads"`
		} `json:"crate"`
		Versions []struct {
			Num       string    `json:"num"`
			CreatedAt time.Time `json:"created_at"`
			Downloads int       `json:"downloads"`
		} `json:"versions"`
	}

	if err := json.NewDecoder(crateResp.Body).Decode(&crateData); err != nil {
		_ = crateResp.Body.Close()
		return nil, fmt.Errorf("failed to decode crate metadata: %w", err)
	}

	// Find the requested version
	var publishDate time.Time
	var versionDownloads int
	found := false

	for _, v := range crateData.Versions {
		if v.Num == version {
			publishDate = v.CreatedAt
			versionDownloads = v.Downloads
			found = true
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("version %s not found in crate metadata", version)
	}

	meta := &Metadata{
		PublishDate:     publishDate,
		TotalDownloads:  crateData.Crate.Downloads,
		RecentDownloads: versionDownloads,
	}

	c.mu.Lock()
	c.cache[key] = &cacheEntry{meta: meta, expiry: time.Now().Add(c.ttl)}
	c.mu.Unlock()

	return meta, nil
}
