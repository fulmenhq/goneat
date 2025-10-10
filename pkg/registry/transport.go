package registry

import (
	"io"
	"net/http"
	"net/url"
	"strings"
)

// HTTPFetcher abstracts HTTP calls for testability
type HTTPFetcher interface {
	Get(url string) (*http.Response, error)
	Do(req *http.Request) (*http.Response, error)
}

// RealHTTPFetcher wraps http.Client for production use
type RealHTTPFetcher struct {
	client *http.Client
}

// NewRealHTTPFetcher creates a production HTTP fetcher
func NewRealHTTPFetcher(client *http.Client) HTTPFetcher {
	return &RealHTTPFetcher{client: client}
}

func (f *RealHTTPFetcher) Get(url string) (*http.Response, error) {
	return f.client.Get(url)
}

func (f *RealHTTPFetcher) Do(req *http.Request) (*http.Response, error) {
	return f.client.Do(req)
}

// MockHTTPFetcher simulates HTTP responses for testing
type MockHTTPFetcher struct {
	responses map[string]*http.Response
	errors    map[string]error
}

// NewMockHTTPFetcher creates a mock HTTP fetcher
func NewMockHTTPFetcher() *MockHTTPFetcher {
	return &MockHTTPFetcher{
		responses: make(map[string]*http.Response),
		errors:    make(map[string]error),
	}
}

// AddResponse registers a mock response for a URL
func (m *MockHTTPFetcher) AddResponse(urlStr string, statusCode int, body string) {
	parsedURL, _ := url.Parse(urlStr)
	m.responses[urlStr] = &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
		Request: &http.Request{
			URL: parsedURL,
		},
	}
}

// AddError registers a mock error for a URL
func (m *MockHTTPFetcher) AddError(urlStr string, err error) {
	m.errors[urlStr] = err
}

func (m *MockHTTPFetcher) Get(urlStr string) (*http.Response, error) {
	if err, ok := m.errors[urlStr]; ok {
		return nil, err
	}
	if resp, ok := m.responses[urlStr]; ok {
		return resp, nil
	}
	// Return 404 for unknown URLs
	return &http.Response{
		StatusCode: 404,
		Body:       io.NopCloser(strings.NewReader("Not Found")),
		Header:     make(http.Header),
	}, nil
}

func (m *MockHTTPFetcher) Do(req *http.Request) (*http.Response, error) {
	return m.Get(req.URL.String())
}
