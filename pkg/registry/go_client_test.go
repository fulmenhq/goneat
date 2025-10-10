package registry

import (
	"errors"
	"os"
	"testing"
	"time"
)

func TestGoClient_GetMetadata_Mock(t *testing.T) {
	// Load fixture
	fixtureData, err := os.ReadFile("testdata/go_proxy_cobra_v1.8.0.json")
	if err != nil {
		t.Fatalf("Failed to load fixture: %v", err)
	}

	// Create mock fetcher
	mock := NewMockHTTPFetcher()
	mock.AddResponse(
		"https://proxy.golang.org/github.com/spf13/cobra/@v/v1.8.0.info",
		200,
		string(fixtureData),
	)

	// Create client with mock
	client := NewGoClientWithFetcher(24*time.Hour, mock)

	// Test GetMetadata
	meta, err := client.GetMetadata("github.com/spf13/cobra", "v1.8.0")
	if err != nil {
		t.Fatalf("GetMetadata failed: %v", err)
	}

	// Verify metadata
	if meta == nil {
		t.Fatal("Expected metadata, got nil")
	}

	// Verify publish date from fixture
	expectedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	if !meta.PublishDate.Equal(expectedTime) {
		t.Errorf("Expected PublishDate %v, got %v", expectedTime, meta.PublishDate)
	}

	// Verify downloads (hardcoded for Go proxy)
	if meta.TotalDownloads != 1000 {
		t.Errorf("Expected TotalDownloads 1000, got %d", meta.TotalDownloads)
	}
	if meta.RecentDownloads != 100 {
		t.Errorf("Expected RecentDownloads 100, got %d", meta.RecentDownloads)
	}
}

func TestGoClient_GetMetadata_Cache(t *testing.T) {
	mock := NewMockHTTPFetcher()
	fixtureData, _ := os.ReadFile("testdata/go_proxy_yaml_v3.json")
	mock.AddResponse(
		"https://proxy.golang.org/gopkg.in/yaml.v3/@v/v3.0.1.info",
		200,
		string(fixtureData),
	)

	client := NewGoClientWithFetcher(1*time.Hour, mock)

	// First call - should hit mock
	meta1, err := client.GetMetadata("gopkg.in/yaml.v3", "v3.0.1")
	if err != nil {
		t.Fatalf("First GetMetadata failed: %v", err)
	}

	// Second call - should hit cache (no network call)
	meta2, err := client.GetMetadata("gopkg.in/yaml.v3", "v3.0.1")
	if err != nil {
		t.Fatalf("Cached GetMetadata failed: %v", err)
	}

	// Verify both return same data
	if meta1.PublishDate != meta2.PublishDate {
		t.Error("Cache returned different publish date")
	}
}

func TestGoClient_GetMetadata_Error(t *testing.T) {
	mock := NewMockHTTPFetcher()
	mock.AddError(
		"https://proxy.golang.org/invalid/package/@v/v1.0.0.info",
		errors.New("network timeout"),
	)

	client := NewGoClientWithFetcher(24*time.Hour, mock)

	_, err := client.GetMetadata("invalid/package", "v1.0.0")
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if err.Error() != "failed to fetch module info: network timeout" {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestGoClient_GetMetadata_404(t *testing.T) {
	mock := NewMockHTTPFetcher()
	mock.AddResponse(
		"https://proxy.golang.org/nonexistent/package/@v/v1.0.0.info",
		404,
		"Not Found",
	)

	client := NewGoClientWithFetcher(24*time.Hour, mock)

	_, err := client.GetMetadata("nonexistent/package", "v1.0.0")
	if err == nil {
		t.Fatal("Expected error for 404 response")
	}
}

func TestGoClient_GetMetadata_InvalidJSON(t *testing.T) {
	mock := NewMockHTTPFetcher()
	mock.AddResponse(
		"https://proxy.golang.org/github.com/test/pkg/@v/v1.0.0.info",
		200,
		"invalid json {",
	)

	client := NewGoClientWithFetcher(24*time.Hour, mock)

	_, err := client.GetMetadata("github.com/test/pkg", "v1.0.0")
	if err == nil {
		t.Fatal("Expected error for invalid JSON")
	}
}
