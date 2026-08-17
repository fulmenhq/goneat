package registry

import (
	"errors"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestCratesClient_GetMetadata_Mock(t *testing.T) {
	// Load fixture
	crateFixture, err := os.ReadFile("testdata/crates_serde_1.0.195.json")
	if err != nil {
		t.Fatalf("Failed to load fixture: %v", err)
	}

	// Create mock fetcher
	mock := NewMockHTTPFetcher()
	mock.AddResponse("https://crates.io/api/v1/crates/serde", 200, string(crateFixture))

	// Create client with mock
	client := NewCratesClientWithFetcher(24*time.Hour, mock)

	// Test GetMetadata
	meta, err := client.GetMetadata("serde", "1.0.195")
	if err != nil {
		t.Fatalf("GetMetadata failed: %v", err)
	}

	// Verify metadata
	if meta == nil {
		t.Fatal("Expected metadata, got nil")
	}

	// Verify publish date from fixture
	expectedTime := time.Date(2024, 1, 10, 8, 15, 0, 0, time.UTC)
	if !meta.PublishDate.Equal(expectedTime) {
		t.Errorf("Expected PublishDate %v, got %v", expectedTime, meta.PublishDate)
	}

	// Verify downloads from fixture
	if meta.TotalDownloads != 250000000 {
		t.Errorf("Expected TotalDownloads 250000000, got %d", meta.TotalDownloads)
	}
	if meta.RecentDownloads != 5000000 {
		t.Errorf("Expected RecentDownloads 5000000, got %d", meta.RecentDownloads)
	}
}

func TestCratesClient_GetMetadata_Error(t *testing.T) {
	mock := NewMockHTTPFetcher()
	mock.AddError("https://crates.io/api/v1/crates/nonexistent", errors.New("rate limit exceeded"))

	client := NewCratesClientWithFetcher(24*time.Hour, mock)

	_, err := client.GetMetadata("nonexistent", "1.0.0")
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
}

func TestCratesClient_GetMetadata_404(t *testing.T) {
	mock := NewMockHTTPFetcher()
	mock.AddResponse("https://crates.io/api/v1/crates/nonexistent", 404, "Not Found")

	client := NewCratesClientWithFetcher(24*time.Hour, mock)

	_, err := client.GetMetadata("nonexistent", "1.0.0")
	if err == nil {
		t.Fatal("Expected error for 404 response")
	}
}

func TestCratesClient_CrateCacheAvoidsSecondFetch(t *testing.T) {
	crateFixture, err := os.ReadFile("testdata/crates_serde_1.0.195.json")
	if err != nil {
		t.Fatal(err)
	}
	inner := NewMockHTTPFetcher()
	inner.AddResponse("https://crates.io/api/v1/crates/serde", 200, string(crateFixture))
	counting := &countingFetcher{inner: inner}

	client := newCratesClient(24*time.Hour, counting, 0)
	if _, err := client.GetMetadata("serde", "1.0.195"); err != nil {
		t.Fatal(err)
	}
	_, err = client.GetMetadata("serde", "99.99.99")
	if err == nil {
		t.Fatal("expected missing version error from cached crate payload")
	}
	if counting.n != 1 {
		t.Fatalf("expected 1 HTTP fetch for crate serde, got %d", counting.n)
	}
}

func TestCratesClient_RateLimitBetweenFetches(t *testing.T) {
	crateFixture, err := os.ReadFile("testdata/crates_serde_1.0.195.json")
	if err != nil {
		t.Fatal(err)
	}
	inner := NewMockHTTPFetcher()
	inner.AddResponse("https://crates.io/api/v1/crates/serde", 200, string(crateFixture))
	inner.AddResponse("https://crates.io/api/v1/crates/other", 200, string(crateFixture))
	counting := &countingFetcher{inner: inner}

	client := newCratesClient(24*time.Hour, counting, 40*time.Millisecond)
	start := time.Now()
	if _, err := client.GetMetadata("serde", "1.0.195"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.GetMetadata("other", "1.0.195"); err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(start)
	if counting.n != 2 {
		t.Fatalf("expected 2 HTTP fetches, got %d", counting.n)
	}
	if elapsed < 40*time.Millisecond {
		t.Fatalf("expected >= 40ms between crates.io fetches, got %s", elapsed)
	}
}

func TestCratesClient_ProductionIntervalIsOneSecond(t *testing.T) {
	if cratesIOMinInterval != time.Second {
		t.Fatalf("crates.io policy is 1 req/s, got %s", cratesIOMinInterval)
	}
}

type countingFetcher struct {
	inner HTTPFetcher
	n     int
}

func (c *countingFetcher) Get(url string) (*http.Response, error) {
	c.n++
	return c.inner.Get(url)
}

func (c *countingFetcher) Do(req *http.Request) (*http.Response, error) {
	c.n++
	return c.inner.Do(req)
}

func TestCratesClient_GetMetadata_VersionNotFound(t *testing.T) {
	crateFixture, _ := os.ReadFile("testdata/crates_serde_1.0.195.json")
	mock := NewMockHTTPFetcher()
	mock.AddResponse("https://crates.io/api/v1/crates/serde", 200, string(crateFixture))

	client := NewCratesClientWithFetcher(24*time.Hour, mock)

	_, err := client.GetMetadata("serde", "99.99.99")
	if err == nil {
		t.Fatal("Expected error for nonexistent version")
	}
}
