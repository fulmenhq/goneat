package metadata

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fulmenhq/goneat/pkg/registry"
)

// GitHubFetcher fetches metadata from GitHub Releases API
type GitHubFetcher struct {
	httpFetcher registry.HTTPFetcher
	token       string
	timeout     time.Duration
}

// NewGitHubFetcher creates a GitHub metadata fetcher
func NewGitHubFetcher(token string, timeout time.Duration) *GitHubFetcher {
	// Secure HTTP client with timeout and TLS verification
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
		},
	}

	return &GitHubFetcher{
		httpFetcher: registry.NewRealHTTPFetcher(client),
		token:       token,
		timeout:     timeout,
	}
}

// NewGitHubFetcherWithHTTP creates a fetcher with injectable HTTP for testing
func NewGitHubFetcherWithHTTP(httpFetcher registry.HTTPFetcher, token string, timeout time.Duration) *GitHubFetcher {
	return &GitHubFetcher{
		httpFetcher: httpFetcher,
		token:       token,
		timeout:     timeout,
	}
}

// githubReleaseResponse matches GitHub's release API response structure
type githubReleaseResponse struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	PublishedAt time.Time `json:"published_at"`
	CreatedAt   time.Time `json:"created_at"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
	Assets      []struct {
		Name          string `json:"name"`
		DownloadCount int    `json:"download_count"`
	} `json:"assets"`
}

// SupportsRepo returns true for GitHub repository formats
func (f *GitHubFetcher) SupportsRepo(repo string) bool {
	// Support formats: "github.com/owner/repo", "owner/repo"
	repo = strings.TrimPrefix(repo, "https://")
	repo = strings.TrimPrefix(repo, "http://")
	repo = strings.TrimPrefix(repo, "github.com/")

	// Should have format "owner/repo"
	parts := strings.Split(repo, "/")
	return len(parts) == 2 && parts[0] != "" && parts[1] != ""
}

// FetchMetadata fetches release metadata from GitHub API
func (f *GitHubFetcher) FetchMetadata(repo, version string) (*Metadata, error) {
	// Normalize repo format to "owner/repo"
	repo = strings.TrimPrefix(repo, "https://")
	repo = strings.TrimPrefix(repo, "http://")
	repo = strings.TrimPrefix(repo, "github.com/")

	if !f.SupportsRepo(repo) {
		return nil, fmt.Errorf("unsupported repository format: %s (expected owner/repo)", repo)
	}

	// Normalize version (GitHub uses tags like "v1.33.0")
	tag := version
	if !strings.HasPrefix(tag, "v") && strings.Contains(version, ".") {
		// If version looks like semver without 'v', try with 'v'
		tag = "v" + version
	}

	// GitHub Releases API: https://docs.github.com/en/rest/releases/releases
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/tags/%s", repo, tag)

	// Create request
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication if token provided (increases rate limit from 60 to 5000/hour)
	if f.token != "" {
		req.Header.Set("Authorization", "token "+f.token)
	}

	// GitHub API requires User-Agent header
	req.Header.Set("User-Agent", "goneat-tools-metadata")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	// Execute request
	resp, err := f.httpFetcher.Do(req)
	if err != nil {
		// Network/transport errors
		return nil, &NetworkError{
			Source:  "github",
			URL:     apiURL,
			Wrapped: err,
		}
	}
	defer func() { _ = resp.Body.Close() }()

	// Handle HTTP status codes
	switch {
	case resp.StatusCode == 404:
		// Try without 'v' prefix if we added it
		if tag != version {
			return f.fetchWithTag(repo, version, req)
		}
		// Wrap with ErrNotFound for missing releases
		return nil, fmt.Errorf("%w: %s@%s", ErrNotFound, repo, version)

	case resp.StatusCode == 403, resp.StatusCode == 429:
		// Rate limit exceeded - parse headers for retry information
		return nil, f.parseRateLimitError(resp, apiURL)

	case resp.StatusCode >= 500:
		// Server errors (5xx) - retriable
		return nil, &NetworkError{
			Source:  "github",
			URL:     apiURL,
			Wrapped: fmt.Errorf("GitHub server error: HTTP %d", resp.StatusCode),
		}

	case resp.StatusCode != 200:
		// Other errors (401, etc.) - likely fatal
		return nil, fmt.Errorf("GitHub API error: HTTP %d", resp.StatusCode)
	}

	// Parse response
	var release githubReleaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, &ParseError{
			Source:  "github",
			Message: "release response",
			Wrapped: err,
		}
	}

	// Calculate total downloads from assets
	totalDownloads := 0
	for _, asset := range release.Assets {
		totalDownloads += asset.DownloadCount
	}

	meta := &Metadata{
		Version:         release.TagName,
		PublishDate:     release.PublishedAt,
		TotalDownloads:  totalDownloads,
		RecentDownloads: -1, // GitHub doesn't provide recent download stats
		Source:          "github",
	}

	return meta, nil
}

// fetchWithTag is a helper to retry with a different tag format
func (f *GitHubFetcher) fetchWithTag(repo, tag string, baseReq *http.Request) (*Metadata, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/tags/%s", repo, tag)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create retry request: %w", err)
	}

	// Copy headers from base request
	req.Header = baseReq.Header.Clone()

	resp, err := f.httpFetcher.Do(req)
	if err != nil {
		return nil, &NetworkError{
			Source:  "github",
			URL:     apiURL,
			Wrapped: err,
		}
	}
	defer func() { _ = resp.Body.Close() }()

	// Handle errors same as main fetch
	switch {
	case resp.StatusCode == 404:
		// Both tag formats failed - wrap with ErrNotFound
		return nil, fmt.Errorf("%w: %s@%s (tried both v-prefix and without)", ErrNotFound, repo, tag)

	case resp.StatusCode == 403, resp.StatusCode == 429:
		return nil, f.parseRateLimitError(resp, apiURL)

	case resp.StatusCode >= 500:
		return nil, &NetworkError{
			Source:  "github",
			URL:     apiURL,
			Wrapped: fmt.Errorf("GitHub server error: HTTP %d", resp.StatusCode),
		}

	case resp.StatusCode != 200:
		return nil, fmt.Errorf("GitHub API error: HTTP %d", resp.StatusCode)
	}

	var release githubReleaseResponse
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, &ParseError{
			Source:  "github",
			Message: "release response",
			Wrapped: err,
		}
	}

	totalDownloads := 0
	for _, asset := range release.Assets {
		totalDownloads += asset.DownloadCount
	}

	return &Metadata{
		Version:         release.TagName,
		PublishDate:     release.PublishedAt,
		TotalDownloads:  totalDownloads,
		RecentDownloads: -1,
		Source:          "github",
	}, nil
}

// parseRateLimitError extracts rate limit information from GitHub response headers
func (f *GitHubFetcher) parseRateLimitError(resp *http.Response, url string) error {
	// GitHub rate limit headers:
	// X-RateLimit-Limit: total requests per hour
	// X-RateLimit-Remaining: requests remaining
	// X-RateLimit-Reset: Unix timestamp when limit resets

	limit := 60 // Default unauthenticated limit
	if limitStr := resp.Header.Get("X-RateLimit-Limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil {
			limit = parsed
		}
	}

	remaining := 0
	if remainingStr := resp.Header.Get("X-RateLimit-Remaining"); remainingStr != "" {
		if parsed, err := strconv.Atoi(remainingStr); err == nil {
			remaining = parsed
		}
	}

	var retryAfter time.Time
	if resetStr := resp.Header.Get("X-RateLimit-Reset"); resetStr != "" {
		if resetUnix, err := strconv.ParseInt(resetStr, 10, 64); err == nil {
			retryAfter = time.Unix(resetUnix, 0)
		}
	}

	message := fmt.Sprintf("GitHub API rate limit exceeded for %s", url)
	if resp.StatusCode == 403 {
		message = fmt.Sprintf("GitHub API returned 403 (likely rate limit) for %s", url)
	}

	return &RateLimitError{
		Source:     "github",
		RetryAfter: retryAfter,
		Limit:      limit,
		Remaining:  remaining,
		Message:    message,
	}
}
