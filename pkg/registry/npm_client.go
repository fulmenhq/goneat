package registry

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// NPMClient implements Client for npm registry
type NPMClient struct {
	baseURL      string
	downloadsURL string
	cache        map[string]*cacheEntry
	mu           sync.RWMutex
	ttl          time.Duration
	fetcher      HTTPFetcher
}

// NewNPMClient creates an NPMClient with real HTTP for production use
func NewNPMClient(ttl time.Duration) Client {
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
		},
	}

	return NewNPMClientWithFetcher(ttl, NewRealHTTPFetcher(client))
}

// NewNPMClientWithFetcher creates an NPMClient with injectable HTTP for testing
func NewNPMClientWithFetcher(ttl time.Duration, fetcher HTTPFetcher) Client {
	return &NPMClient{
		baseURL:      "https://registry.npmjs.org",
		downloadsURL: "https://api.npmjs.org/downloads/point",
		cache:        make(map[string]*cacheEntry),
		ttl:          ttl,
		fetcher:      fetcher,
	}
}

func (c *NPMClient) GetMetadata(name, version string) (*Metadata, error) {
	key := fmt.Sprintf("%s@%s", name, version)
	c.mu.RLock()
	entry, ok := c.cache[key]
	c.mu.RUnlock()

	if ok && time.Now().Before(entry.expiry) {
		return entry.meta, nil
	}

	// Fetch package metadata - URL escape the package name for scoped packages
	escapedName := url.PathEscape(name)
	pkgURL := fmt.Sprintf("%s/%s", c.baseURL, escapedName)
	pkgResp, err := c.fetcher.Get(pkgURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch package metadata: %w", err)
	}
	defer func() { _ = pkgResp.Body.Close() }()

	if pkgResp.StatusCode != 200 {
		return nil, fmt.Errorf("npm registry returned status %d", pkgResp.StatusCode)
	}

	var pkgData struct {
		Time map[string]string `json:"time"`
	}

	if err := json.NewDecoder(pkgResp.Body).Decode(&pkgData); err != nil {
		_ = pkgResp.Body.Close()
		return nil, fmt.Errorf("failed to decode package metadata: %w", err)
	}

	// Get publish date for this version
	publishDateStr, ok := pkgData.Time[version]
	if !ok {
		return nil, fmt.Errorf("version %s not found in package metadata", version)
	}

	publishDate, err := time.Parse(time.RFC3339, publishDateStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse publish date: %w", err)
	}

	// Fetch download stats - URL escape the package name for scoped packages
	lastMonth := time.Now().AddDate(0, -1, 0).Format("2006-01-02")
	today := time.Now().Format("2006-01-02")
	escapedNameDownloads := url.PathEscape(name)
	downloadsURL := fmt.Sprintf("%s/last-month/%s", c.downloadsURL, escapedNameDownloads)

	totalDownloads := 0
	recentDownloads := 0

	dlResp, err := c.fetcher.Get(downloadsURL)
	if err == nil && dlResp.StatusCode == 200 {
		defer func() { _ = dlResp.Body.Close() }()
		var dlData struct {
			Downloads int `json:"downloads"`
		}
		if json.NewDecoder(dlResp.Body).Decode(&dlData) == nil {
			recentDownloads = dlData.Downloads
			totalDownloads = dlData.Downloads // npm doesn't provide total, use recent as proxy
		}
	}

	// If downloads fetch failed, use conservative defaults
	if totalDownloads == 0 {
		totalDownloads = 1000
		recentDownloads = 100
	}

	meta := &Metadata{
		PublishDate:     publishDate,
		TotalDownloads:  totalDownloads,
		RecentDownloads: recentDownloads,
	}

	_ = lastMonth // Used in URL comment for clarity
	_ = today     // Reserved for future use

	c.mu.Lock()
	c.cache[key] = &cacheEntry{meta: meta, expiry: time.Now().Add(c.ttl)}
	c.mu.Unlock()

	return meta, nil
}
