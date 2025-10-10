package registry

import (
	"errors"
	"os"
	"testing"
	"time"
)

func TestNuGetClient_GetMetadata_Mock(t *testing.T) {
	// Load fixtures
	indexFixture, err := os.ReadFile("testdata/nuget_service_index.json")
	if err != nil {
		t.Fatalf("Failed to load service index fixture: %v", err)
	}
	pkgFixture, err := os.ReadFile("testdata/nuget_newtonsoft_json.json")
	if err != nil {
		t.Fatalf("Failed to load package fixture: %v", err)
	}

	// Create mock fetcher
	mock := NewMockHTTPFetcher()
	mock.AddResponse("https://api.nuget.org/v3/index.json", 200, string(indexFixture))
	mock.AddResponse("https://api.nuget.org/v3-flatcontainer/Newtonsoft.Json/index.json", 200, string(pkgFixture))

	// Create client with mock
	client := NewNuGetClientWithFetcher(24*time.Hour, mock)

	// Test GetMetadata
	meta, err := client.GetMetadata("Newtonsoft.Json", "13.0.3")
	if err != nil {
		t.Fatalf("GetMetadata failed: %v", err)
	}

	// Verify metadata
	if meta == nil {
		t.Fatal("Expected metadata, got nil")
	}

	// Verify publish date from fixture
	expectedTime := time.Date(2023, 3, 8, 21, 40, 0, 0, time.UTC)
	if !meta.PublishDate.Equal(expectedTime) {
		t.Errorf("Expected PublishDate %v, got %v", expectedTime, meta.PublishDate)
	}

	// Verify conservative defaults for downloads
	if meta.TotalDownloads != 1000 {
		t.Errorf("Expected TotalDownloads 1000, got %d", meta.TotalDownloads)
	}
}

func TestNuGetClient_GetMetadata_ServiceIndexError(t *testing.T) {
	mock := NewMockHTTPFetcher()
	mock.AddError("https://api.nuget.org/v3/index.json", errors.New("service unavailable"))

	client := NewNuGetClientWithFetcher(24*time.Hour, mock)

	_, err := client.GetMetadata("Newtonsoft.Json", "13.0.3")
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
}

func TestNuGetClient_GetMetadata_PackageError(t *testing.T) {
	indexFixture, _ := os.ReadFile("testdata/nuget_service_index.json")
	mock := NewMockHTTPFetcher()
	mock.AddResponse("https://api.nuget.org/v3/index.json", 200, string(indexFixture))
	mock.AddError("https://api.nuget.org/v3-flatcontainer/Nonexistent/index.json", errors.New("not found"))

	client := NewNuGetClientWithFetcher(24*time.Hour, mock)

	_, err := client.GetMetadata("Nonexistent", "1.0.0")
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
}

func TestNuGetClient_GetMetadata_404(t *testing.T) {
	indexFixture, _ := os.ReadFile("testdata/nuget_service_index.json")
	mock := NewMockHTTPFetcher()
	mock.AddResponse("https://api.nuget.org/v3/index.json", 200, string(indexFixture))
	mock.AddResponse("https://api.nuget.org/v3-flatcontainer/Nonexistent/index.json", 404, "Not Found")

	client := NewNuGetClientWithFetcher(24*time.Hour, mock)

	_, err := client.GetMetadata("Nonexistent", "1.0.0")
	if err == nil {
		t.Fatal("Expected error for 404 response")
	}
}

func TestNuGetClient_GetMetadata_VersionNotFound(t *testing.T) {
	indexFixture, _ := os.ReadFile("testdata/nuget_service_index.json")
	pkgFixture, _ := os.ReadFile("testdata/nuget_newtonsoft_json.json")
	mock := NewMockHTTPFetcher()
	mock.AddResponse("https://api.nuget.org/v3/index.json", 200, string(indexFixture))
	mock.AddResponse("https://api.nuget.org/v3-flatcontainer/Newtonsoft.Json/index.json", 200, string(pkgFixture))

	client := NewNuGetClientWithFetcher(24*time.Hour, mock)

	_, err := client.GetMetadata("Newtonsoft.Json", "99.99.99")
	if err == nil {
		t.Fatal("Expected error for nonexistent version")
	}
}
