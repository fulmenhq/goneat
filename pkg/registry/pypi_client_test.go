package registry

import (
	"errors"
	"os"
	"testing"
	"time"
)

func TestPyPIClient_GetMetadata_Mock(t *testing.T) {
	// Load fixture
	pkgFixture, err := os.ReadFile("testdata/pypi_requests_2.31.0.json")
	if err != nil {
		t.Fatalf("Failed to load fixture: %v", err)
	}

	// Create mock fetcher
	mock := NewMockHTTPFetcher()
	mock.AddResponse("https://pypi.org/pypi/requests/2.31.0/json", 200, string(pkgFixture))

	// Create client with mock
	client := NewPyPIClientWithFetcher(24*time.Hour, mock)

	// Test GetMetadata
	meta, err := client.GetMetadata("requests", "2.31.0")
	if err != nil {
		t.Fatalf("GetMetadata failed: %v", err)
	}

	// Verify metadata
	if meta == nil {
		t.Fatal("Expected metadata, got nil")
	}

	// Verify publish date from fixture
	expectedTime := time.Date(2023, 5, 22, 13, 30, 0, 0, time.UTC)
	if !meta.PublishDate.Equal(expectedTime) {
		t.Errorf("Expected PublishDate %v, got %v", expectedTime, meta.PublishDate)
	}

	// Verify conservative defaults for downloads
	if meta.TotalDownloads != 1000 {
		t.Errorf("Expected TotalDownloads 1000, got %d", meta.TotalDownloads)
	}
}

func TestPyPIClient_GetMetadata_Error(t *testing.T) {
	mock := NewMockHTTPFetcher()
	mock.AddError("https://pypi.org/pypi/nonexistent/1.0.0/json", errors.New("connection refused"))

	client := NewPyPIClientWithFetcher(24*time.Hour, mock)

	_, err := client.GetMetadata("nonexistent", "1.0.0")
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
}

func TestPyPIClient_GetMetadata_404(t *testing.T) {
	mock := NewMockHTTPFetcher()
	mock.AddResponse("https://pypi.org/pypi/nonexistent/1.0.0/json", 404, "Not Found")

	client := NewPyPIClientWithFetcher(24*time.Hour, mock)

	_, err := client.GetMetadata("nonexistent", "1.0.0")
	if err == nil {
		t.Fatal("Expected error for 404 response")
	}
}
