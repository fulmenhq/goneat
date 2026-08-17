package registry

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// cratesIOMinInterval is the crates.io crawler-policy maximum: 1 request/sec.
// https://crates.io/data-access
const cratesIOMinInterval = time.Second

// cratesIOUserAgent identifies goneat and how to reach us (required + recommended).
const cratesIOUserAgent = "goneat (https://github.com/fulmenhq/goneat; hello@3leaps.net)"

type crateVersionMeta struct {
	createdAt time.Time
	downloads int
}

type crateRecord struct {
	totalDownloads int
	versions       map[string]crateVersionMeta
	expiry         time.Time
}

// CratesClient implements Client for crates.io registry
type CratesClient struct {
	baseURL     string
	cache       map[string]*crateRecord // keyed by crate name
	mu          sync.RWMutex
	ttl         time.Duration
	fetcher     HTTPFetcher
	minInterval time.Duration
	lastFetch   time.Time
	rateMu      sync.Mutex
}

// NewCratesClient creates a CratesClient with real HTTP and 1 req/s throttling.
func NewCratesClient(ttl time.Duration) Client {
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
		},
	}

	return newCratesClient(ttl, NewRealHTTPFetcher(client), cratesIOMinInterval)
}

// NewCratesClientWithFetcher creates a CratesClient with injectable HTTP for testing.
// Tests use minInterval=0 so they do not sleep.
func NewCratesClientWithFetcher(ttl time.Duration, fetcher HTTPFetcher) Client {
	return newCratesClient(ttl, fetcher, 0)
}

func newCratesClient(ttl time.Duration, fetcher HTTPFetcher, minInterval time.Duration) *CratesClient {
	return &CratesClient{
		baseURL:     "https://crates.io/api/v1",
		cache:       make(map[string]*crateRecord),
		ttl:         ttl,
		fetcher:     fetcher,
		minInterval: minInterval,
	}
}

func (c *CratesClient) GetMetadata(name, version string) (*Metadata, error) {
	if rec := c.cachedCrate(name); rec != nil {
		return metadataFromCrate(rec, version)
	}

	rec, err := c.fetchCrate(name)
	if err != nil {
		return nil, err
	}
	return metadataFromCrate(rec, version)
}

func (c *CratesClient) cachedCrate(name string) *crateRecord {
	c.mu.RLock()
	defer c.mu.RUnlock()
	rec, ok := c.cache[name]
	if !ok || rec == nil || time.Now().After(rec.expiry) {
		return nil
	}
	return rec
}

func (c *CratesClient) fetchCrate(name string) (*crateRecord, error) {
	c.throttle()

	crateURL := fmt.Sprintf("%s/crates/%s", c.baseURL, name)
	req, err := http.NewRequest("GET", crateURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", cratesIOUserAgent)

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
		return nil, fmt.Errorf("failed to decode crate metadata: %w", err)
	}

	rec := &crateRecord{
		totalDownloads: crateData.Crate.Downloads,
		versions:       make(map[string]crateVersionMeta, len(crateData.Versions)),
		expiry:         time.Now().Add(c.ttl),
	}
	for _, v := range crateData.Versions {
		rec.versions[v.Num] = crateVersionMeta{createdAt: v.CreatedAt, downloads: v.Downloads}
	}

	c.mu.Lock()
	c.cache[name] = rec
	c.mu.Unlock()
	return rec, nil
}

func (c *CratesClient) throttle() {
	if c.minInterval <= 0 {
		return
	}
	c.rateMu.Lock()
	defer c.rateMu.Unlock()
	if !c.lastFetch.IsZero() {
		if wait := c.minInterval - time.Since(c.lastFetch); wait > 0 {
			time.Sleep(wait)
		}
	}
	c.lastFetch = time.Now()
}

func metadataFromCrate(rec *crateRecord, version string) (*Metadata, error) {
	v, ok := rec.versions[version]
	if !ok {
		return nil, fmt.Errorf("version %s not found in crate metadata", version)
	}
	// RecentDownloads stays the crates.io per-version lifetime count.
	// Rust cooling must not treat this as a 30-day window.
	return &Metadata{
		PublishDate:     v.createdAt,
		TotalDownloads:  rec.totalDownloads,
		RecentDownloads: v.downloads,
	}, nil
}
