package metadata

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fulmenhq/goneat/pkg/registry"
)

func TestPyPIFetcher_FetchLatestAndVersion(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/pypi/yamllint/json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
  "info": {"version": "1.2.3"},
  "releases": {
    "1.2.3": [{"upload_time_iso_8601": "2025-01-02T03:04:05Z"}]
  }
}`))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	client := &http.Client{Timeout: 5 * time.Second}
	fetcher := NewPyPIFetcherWithHTTP(registry.NewRealHTTPFetcher(client), server.URL, 5*time.Second)

	if !fetcher.SupportsRepo("pypi/yamllint") {
		t.Fatalf("expected SupportsRepo true")
	}

	latest, err := fetcher.FetchLatestMetadata("pypi/yamllint")
	if err != nil {
		t.Fatalf("FetchLatestMetadata error: %v", err)
	}
	if latest.Version != "1.2.3" {
		t.Fatalf("expected latest version 1.2.3, got %q", latest.Version)
	}
	if latest.Source != "pypi" {
		t.Fatalf("expected source pypi, got %q", latest.Source)
	}

	meta, err := fetcher.FetchMetadata("pypi/yamllint", "v1.2.3")
	if err != nil {
		t.Fatalf("FetchMetadata error: %v", err)
	}
	if meta.Version != "1.2.3" {
		t.Fatalf("expected version 1.2.3, got %q", meta.Version)
	}
	if meta.PublishDate.Format(time.RFC3339) != "2025-01-02T03:04:05Z" {
		t.Fatalf("unexpected publish date: %s", meta.PublishDate.Format(time.RFC3339))
	}
}
