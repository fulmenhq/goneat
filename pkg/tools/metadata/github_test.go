package metadata

import (
	"errors"
	"testing"
	"time"

	"github.com/fulmenhq/goneat/pkg/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitHubFetcher_SupportsRepo(t *testing.T) {
	fetcher := NewGitHubFetcher("", 30*time.Second)

	tests := []struct {
		name     string
		repo     string
		expected bool
	}{
		{
			name:     "github.com prefix",
			repo:     "github.com/anchore/syft",
			expected: true,
		},
		{
			name:     "owner/repo format",
			repo:     "anchore/syft",
			expected: true,
		},
		{
			name:     "https github.com",
			repo:     "https://github.com/anchore/syft",
			expected: true,
		},
		{
			name:     "invalid single part",
			repo:     "syft",
			expected: false,
		},
		{
			name:     "invalid too many parts",
			repo:     "github.com/anchore/syft/releases",
			expected: false,
		},
		{
			name:     "empty string",
			repo:     "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fetcher.SupportsRepo(tt.repo)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGitHubFetcher_FetchMetadata_Success(t *testing.T) {
	mock := registry.NewMockHTTPFetcher()

	// Mock successful GitHub API response
	mock.AddResponse(
		"https://api.github.com/repos/anchore/syft/releases/tags/v1.33.0",
		200,
		`{
			"tag_name": "v1.33.0",
			"name": "v1.33.0",
			"published_at": "2024-11-01T10:00:00Z",
			"created_at": "2024-11-01T09:00:00Z",
			"draft": false,
			"prerelease": false,
			"assets": [
				{"name": "syft_darwin_amd64.tar.gz", "download_count": 1500},
				{"name": "syft_linux_amd64.tar.gz", "download_count": 2500},
				{"name": "syft_windows_amd64.zip", "download_count": 500}
			]
		}`,
	)

	fetcher := NewGitHubFetcherWithHTTP(mock, "", 30*time.Second)

	meta, err := fetcher.FetchMetadata("anchore/syft", "v1.33.0")
	require.NoError(t, err)
	require.NotNil(t, meta)

	assert.Equal(t, "v1.33.0", meta.Version)
	assert.Equal(t, 4500, meta.TotalDownloads) // 1500 + 2500 + 500
	assert.Equal(t, -1, meta.RecentDownloads)  // Not available from GitHub
	assert.Equal(t, "github", meta.Source)

	expectedDate := time.Date(2024, 11, 1, 10, 0, 0, 0, time.UTC)
	assert.True(t, meta.PublishDate.Equal(expectedDate))
}

func TestGitHubFetcher_FetchMetadata_WithVersionPrefix(t *testing.T) {
	mock := registry.NewMockHTTPFetcher()

	// Mock response for version with 'v' prefix
	mock.AddResponse(
		"https://api.github.com/repos/golangci/golangci-lint/releases/tags/v2.4.0",
		200,
		`{
			"tag_name": "v2.4.0",
			"name": "v2.4.0",
			"published_at": "2024-10-15T14:30:00Z",
			"created_at": "2024-10-15T14:00:00Z",
			"draft": false,
			"prerelease": false,
			"assets": [
				{"name": "golangci-lint.tar.gz", "download_count": 10000}
			]
		}`,
	)

	fetcher := NewGitHubFetcherWithHTTP(mock, "", 30*time.Second)

	// Test with version string WITHOUT 'v' prefix - fetcher should add it
	meta, err := fetcher.FetchMetadata("golangci/golangci-lint", "2.4.0")
	require.NoError(t, err)
	require.NotNil(t, meta)

	assert.Equal(t, "v2.4.0", meta.Version)
	assert.Equal(t, 10000, meta.TotalDownloads)
}

func TestGitHubFetcher_FetchMetadata_WithoutVersionPrefix(t *testing.T) {
	mock := registry.NewMockHTTPFetcher()

	// Mock 404 for version with 'v' prefix, success without
	mock.AddResponse(
		"https://api.github.com/repos/some/tool/releases/tags/v1.0.0",
		404,
		`{"message": "Not Found"}`,
	)
	mock.AddResponse(
		"https://api.github.com/repos/some/tool/releases/tags/1.0.0",
		200,
		`{
			"tag_name": "1.0.0",
			"name": "Release 1.0.0",
			"published_at": "2024-09-01T12:00:00Z",
			"created_at": "2024-09-01T11:00:00Z",
			"draft": false,
			"prerelease": false,
			"assets": []
		}`,
	)

	fetcher := NewGitHubFetcherWithHTTP(mock, "", 30*time.Second)

	meta, err := fetcher.FetchMetadata("some/tool", "1.0.0")
	require.NoError(t, err)
	require.NotNil(t, meta)

	assert.Equal(t, "1.0.0", meta.Version)
	assert.Equal(t, 0, meta.TotalDownloads) // No assets
}

func TestGitHubFetcher_FetchMetadata_NotFound(t *testing.T) {
	mock := registry.NewMockHTTPFetcher()

	// Mock 404 response for both tag formats
	mock.AddResponse(
		"https://api.github.com/repos/anchore/syft/releases/tags/v99.99.99",
		404,
		`{"message": "Not Found"}`,
	)
	mock.AddResponse(
		"https://api.github.com/repos/anchore/syft/releases/tags/99.99.99",
		404,
		`{"message": "Not Found"}`,
	)

	fetcher := NewGitHubFetcherWithHTTP(mock, "", 30*time.Second)

	meta, err := fetcher.FetchMetadata("anchore/syft", "99.99.99")
	require.Error(t, err)
	assert.Nil(t, meta)

	// Should wrap with ErrNotFound
	assert.ErrorIs(t, err, ErrNotFound)
	assert.Contains(t, err.Error(), "anchore/syft@99.99.99")
}

func TestGitHubFetcher_FetchMetadata_InvalidRepo(t *testing.T) {
	mock := registry.NewMockHTTPFetcher()
	fetcher := NewGitHubFetcherWithHTTP(mock, "", 30*time.Second)

	tests := []struct {
		name string
		repo string
	}{
		{"single part", "syft"},
		{"empty", ""},
		{"too many parts", "github.com/anchore/syft/releases"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta, err := fetcher.FetchMetadata(tt.repo, "v1.0.0")
			require.Error(t, err)
			assert.Nil(t, meta)
			assert.Contains(t, err.Error(), "unsupported repository format")
		})
	}
}

func TestGitHubFetcher_FetchMetadata_WithAuthToken(t *testing.T) {
	mock := registry.NewMockHTTPFetcher()

	mock.AddResponse(
		"https://api.github.com/repos/anchore/syft/releases/tags/v1.33.0",
		200,
		`{
			"tag_name": "v1.33.0",
			"name": "v1.33.0",
			"published_at": "2024-11-01T10:00:00Z",
			"created_at": "2024-11-01T09:00:00Z",
			"draft": false,
			"prerelease": false,
			"assets": []
		}`,
	)

	fetcher := NewGitHubFetcherWithHTTP(mock, "test-token-123", 30*time.Second)

	meta, err := fetcher.FetchMetadata("anchore/syft", "v1.33.0")
	require.NoError(t, err)
	require.NotNil(t, meta)
	assert.Equal(t, "v1.33.0", meta.Version)
}

func TestGitHubFetcher_FetchMetadata_NoAssets(t *testing.T) {
	mock := registry.NewMockHTTPFetcher()

	mock.AddResponse(
		"https://api.github.com/repos/test/repo/releases/tags/v1.0.0",
		200,
		`{
			"tag_name": "v1.0.0",
			"name": "v1.0.0",
			"published_at": "2024-11-01T10:00:00Z",
			"created_at": "2024-11-01T09:00:00Z",
			"draft": false,
			"prerelease": false,
			"assets": []
		}`,
	)

	fetcher := NewGitHubFetcherWithHTTP(mock, "", 30*time.Second)

	meta, err := fetcher.FetchMetadata("test/repo", "v1.0.0")
	require.NoError(t, err)
	require.NotNil(t, meta)

	assert.Equal(t, 0, meta.TotalDownloads)
}

func TestGitHubFetcher_FetchMetadata_RateLimit(t *testing.T) {
	mock := registry.NewMockHTTPFetcher()

	// Mock 403 rate limit response
	// Note: Mock doesn't support adding headers easily, so we test basic rate limit detection
	// The parseRateLimitError function will use default values when headers absent
	mock.AddResponse(
		"https://api.github.com/repos/test/repo/releases/tags/v1.0.0",
		403,
		`{"message": "API rate limit exceeded"}`,
	)

	fetcher := NewGitHubFetcherWithHTTP(mock, "", 30*time.Second)

	meta, err := fetcher.FetchMetadata("test/repo", "v1.0.0")
	require.Error(t, err)
	assert.Nil(t, meta)

	// Should be a RateLimitError
	var rateLimitErr *RateLimitError
	require.True(t, errors.As(err, &rateLimitErr))
	assert.Equal(t, "github", rateLimitErr.Source)
	assert.Contains(t, rateLimitErr.Message, "403")

	// IsRateLimitError should detect it
	assert.True(t, IsRateLimitError(err))
}

func TestGitHubFetcher_FetchMetadata_ServerError(t *testing.T) {
	mock := registry.NewMockHTTPFetcher()

	// Mock 500 server error
	mock.AddResponse(
		"https://api.github.com/repos/test/repo/releases/tags/v1.0.0",
		500,
		`{"message": "Internal Server Error"}`,
	)

	fetcher := NewGitHubFetcherWithHTTP(mock, "", 30*time.Second)

	meta, err := fetcher.FetchMetadata("test/repo", "v1.0.0")
	require.Error(t, err)
	assert.Nil(t, meta)

	// Should be a NetworkError (5xx = retriable)
	var netErr *NetworkError
	require.True(t, errors.As(err, &netErr))
	assert.Equal(t, "github", netErr.Source)
	assert.Contains(t, netErr.Error(), "GitHub server error")
}

func TestGitHubFetcher_FetchMetadata_NetworkError(t *testing.T) {
	mock := registry.NewMockHTTPFetcher()

	// Mock network/transport error
	mock.AddError(
		"https://api.github.com/repos/test/repo/releases/tags/v1.0.0",
		errors.New("connection refused"),
	)

	fetcher := NewGitHubFetcherWithHTTP(mock, "", 30*time.Second)

	meta, err := fetcher.FetchMetadata("test/repo", "v1.0.0")
	require.Error(t, err)
	assert.Nil(t, meta)

	// Should be wrapped in NetworkError
	var netErr *NetworkError
	require.True(t, errors.As(err, &netErr))
	assert.Equal(t, "github", netErr.Source)
	assert.Contains(t, netErr.Error(), "connection refused")
}

func TestGitHubFetcher_FetchMetadata_ParseError(t *testing.T) {
	mock := registry.NewMockHTTPFetcher()

	// Mock invalid JSON response
	mock.AddResponse(
		"https://api.github.com/repos/test/repo/releases/tags/v1.0.0",
		200,
		`{invalid json}`,
	)

	fetcher := NewGitHubFetcherWithHTTP(mock, "", 30*time.Second)

	meta, err := fetcher.FetchMetadata("test/repo", "v1.0.0")
	require.Error(t, err)
	assert.Nil(t, meta)

	// Should be a ParseError
	var parseErr *ParseError
	require.True(t, errors.As(err, &parseErr))
	assert.Equal(t, "github", parseErr.Source)
	assert.Equal(t, "release response", parseErr.Message)
}
