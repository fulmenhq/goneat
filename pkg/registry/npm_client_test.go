package registry

import (
	"errors"
	"os"
	"testing"
	"time"
)

func TestNPMClient_GetMetadata_Mock(t *testing.T) {
	// Load fixtures
	pkgFixture, err := os.ReadFile("testdata/npm_lodash.json")
	if err != nil {
		t.Fatalf("Failed to load package fixture: %v", err)
	}
	dlFixture, err := os.ReadFile("testdata/npm_lodash_downloads.json")
	if err != nil {
		t.Fatalf("Failed to load downloads fixture: %v", err)
	}

	// Create mock fetcher
	mock := NewMockHTTPFetcher()
	mock.AddResponse("https://registry.npmjs.org/lodash", 200, string(pkgFixture))
	mock.AddResponse("https://api.npmjs.org/downloads/point/last-month/lodash", 200, string(dlFixture))

	// Create client with mock
	client := NewNPMClientWithFetcher(24*time.Hour, mock)

	// Test GetMetadata
	meta, err := client.GetMetadata("lodash", "4.17.21")
	if err != nil {
		t.Fatalf("GetMetadata failed: %v", err)
	}

	// Verify metadata
	if meta == nil {
		t.Fatal("Expected metadata, got nil")
	}

	// Verify publish date from fixture
	expectedTime := time.Date(2021, 2, 20, 18, 55, 43, 207000000, time.UTC)
	if !meta.PublishDate.Equal(expectedTime) {
		t.Errorf("Expected PublishDate %v, got %v", expectedTime, meta.PublishDate)
	}

	// Verify downloads from fixture
	if meta.TotalDownloads != 45000000 {
		t.Errorf("Expected TotalDownloads 45000000, got %d", meta.TotalDownloads)
	}
	if meta.RecentDownloads != 45000000 {
		t.Errorf("Expected RecentDownloads 45000000, got %d", meta.RecentDownloads)
	}
}

func TestNPMClient_GetMetadata_Error(t *testing.T) {
	mock := NewMockHTTPFetcher()
	mock.AddError("https://registry.npmjs.org/nonexistent", errors.New("network timeout"))

	client := NewNPMClientWithFetcher(24*time.Hour, mock)

	_, err := client.GetMetadata("nonexistent", "1.0.0")
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
}

func TestNPMClient_GetMetadata_404(t *testing.T) {
	mock := NewMockHTTPFetcher()
	mock.AddResponse("https://registry.npmjs.org/nonexistent", 404, "Not Found")

	client := NewNPMClientWithFetcher(24*time.Hour, mock)

	_, err := client.GetMetadata("nonexistent", "1.0.0")
	if err == nil {
		t.Fatal("Expected error for 404 response")
	}
}

func TestNPMClient_GetMetadata_VersionNotFound(t *testing.T) {
	pkgFixture, _ := os.ReadFile("testdata/npm_lodash.json")
	mock := NewMockHTTPFetcher()
	mock.AddResponse("https://registry.npmjs.org/lodash", 200, string(pkgFixture))

	client := NewNPMClientWithFetcher(24*time.Hour, mock)

	_, err := client.GetMetadata("lodash", "99.99.99")
	if err == nil {
		t.Fatal("Expected error for nonexistent version")
	}
}
