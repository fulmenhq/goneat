package metadata

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/fulmenhq/goneat/pkg/registry"
)

type PyPIFetcher struct {
	httpFetcher registry.HTTPFetcher
	baseURL     string
	timeout     time.Duration
}

func NewPyPIFetcher(timeout time.Duration) *PyPIFetcher {
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		},
	}
	return NewPyPIFetcherWithHTTP(registry.NewRealHTTPFetcher(client), "https://pypi.org", timeout)
}

func NewPyPIFetcherWithHTTP(httpFetcher registry.HTTPFetcher, baseURL string, timeout time.Duration) *PyPIFetcher {
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		baseURL = "https://pypi.org"
	}
	return &PyPIFetcher{httpFetcher: httpFetcher, baseURL: baseURL, timeout: timeout}
}

type pypiReleaseFile struct {
	UploadTimeISO8601 string `json:"upload_time_iso_8601"`
}

type pypiResponse struct {
	Info struct {
		Version string `json:"version"`
	} `json:"info"`
	Releases map[string][]pypiReleaseFile `json:"releases"`
}

func (f *PyPIFetcher) SupportsRepo(repo string) bool {
	repo = strings.TrimSpace(repo)
	return strings.HasPrefix(repo, "pypi/") || strings.HasPrefix(repo, "pypi:")
}

func (f *PyPIFetcher) FetchMetadata(repo, version string) (*Metadata, error) {
	pkg := normalizePyPIPackage(repo)
	if pkg == "" {
		return nil, fmt.Errorf("unsupported repository format: %s (expected pypi/<package>)", repo)
	}

	resp, err := f.fetchPackage(pkg)
	if err != nil {
		return nil, err
	}

	files, ok := resp.Releases[version]
	if !ok {
		// PyPI versions do not include a leading "v", but some tools report one.
		trimmed := strings.TrimPrefix(version, "v")
		files, ok = resp.Releases[trimmed]
		if !ok {
			return nil, fmt.Errorf("%w: %s@%s", ErrNotFound, pkg, version)
		}
		version = trimmed
	}

	publish, err := earliestUploadTime(files)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PyPI upload time for %s@%s: %w", pkg, version, err)
	}

	return &Metadata{
		Version:         version,
		PublishDate:     publish,
		TotalDownloads:  -1,
		RecentDownloads: -1,
		Source:          "pypi",
	}, nil
}

func (f *PyPIFetcher) FetchLatestMetadata(repo string) (*Metadata, error) {
	pkg := normalizePyPIPackage(repo)
	if pkg == "" {
		return nil, fmt.Errorf("unsupported repository format: %s (expected pypi/<package>)", repo)
	}

	resp, err := f.fetchPackage(pkg)
	if err != nil {
		return nil, err
	}

	latest := strings.TrimSpace(resp.Info.Version)
	if latest == "" {
		return nil, fmt.Errorf("%w: no version found for %s", ErrNotFound, pkg)
	}

	files, ok := resp.Releases[latest]
	if !ok {
		return nil, fmt.Errorf("%w: no release found for %s@%s", ErrNotFound, pkg, latest)
	}

	publish, err := earliestUploadTime(files)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PyPI upload time for %s@%s: %w", pkg, latest, err)
	}

	return &Metadata{
		Version:         latest,
		PublishDate:     publish,
		TotalDownloads:  -1,
		RecentDownloads: -1,
		Source:          "pypi",
	}, nil
}

func (f *PyPIFetcher) fetchPackage(pkg string) (*pypiResponse, error) {
	apiURL := fmt.Sprintf("%s/pypi/%s/json", f.baseURL, pkg)
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "goneat-tools-metadata")
	req.Header.Set("Accept", "application/json")

	resp, err := f.httpFetcher.Do(req)
	if err != nil {
		return nil, &NetworkError{Source: "pypi", URL: apiURL, Wrapped: err}
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, pkg)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("PyPI API error: HTTP %d", resp.StatusCode)
	}

	var decoded pypiResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return nil, &ParseError{Source: "pypi", Message: "package response", Wrapped: err}
	}

	return &decoded, nil
}

func normalizePyPIPackage(repo string) string {
	repo = strings.TrimSpace(repo)
	repo = strings.TrimPrefix(repo, "pypi:")
	repo = strings.TrimPrefix(repo, "pypi/")
	repo = strings.Trim(repo, "/")
	return repo
}

func earliestUploadTime(files []pypiReleaseFile) (time.Time, error) {
	var earliest time.Time
	for _, f := range files {
		if strings.TrimSpace(f.UploadTimeISO8601) == "" {
			continue
		}
		t, err := time.Parse(time.RFC3339, f.UploadTimeISO8601)
		if err != nil {
			return time.Time{}, err
		}
		if earliest.IsZero() || t.Before(earliest) {
			earliest = t
		}
	}
	if earliest.IsZero() {
		return time.Time{}, fmt.Errorf("no upload time found")
	}
	return earliest, nil
}
