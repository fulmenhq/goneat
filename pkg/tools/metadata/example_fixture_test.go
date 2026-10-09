package metadata_test

import (
	"errors"
	"testing"
	"time"

	"github.com/fulmenhq/goneat/pkg/tools/metadata"
)

// exampleFixtureFetcher supplies explicit fixture data, not GitHub provenance.
type exampleFixtureFetcher struct {
	err   error
	calls int
}

func (f *exampleFixtureFetcher) SupportsRepo(string) bool { return true }
func (f *exampleFixtureFetcher) FetchMetadata(_ string, version string) (*metadata.Metadata, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return &metadata.Metadata{
		Version: version, Source: "fixture", PublishDate: time.Date(2024, 11, 1, 0, 0, 0, 0, time.UTC),
		TotalDownloads: -1, RecentDownloads: -1,
	}, nil
}
func (f *exampleFixtureFetcher) FetchLatestMetadata(repo string) (*metadata.Metadata, error) {
	return f.FetchMetadata(repo, "v1.0.0")
}

func TestRegistry_GetMetadata_FailedFixtureNotCached(t *testing.T) {
	cause := errors.New("fixture fetch failed")
	fetcher := &exampleFixtureFetcher{err: cause}
	reg := metadata.NewRegistry(time.Hour)
	reg.RegisterFetcher("fixture", fetcher)
	for i := 0; i < 2; i++ {
		meta, err := reg.GetMetadata("test/tool", "v1.0.0")
		if meta != nil || !errors.Is(err, cause) {
			t.Fatalf("fetch failure must remain nil plus wrapped cause: meta=%v error=%v", meta, err)
		}
	}
	if fetcher.calls != 2 || reg.CacheStats() != (metadata.CacheStats{Misses: 2}) {
		t.Fatalf("failed fetch must not be cached: calls=%d stats=%+v", fetcher.calls, reg.CacheStats())
	}
	fetcher.err = nil
	first, err := reg.GetMetadata("test/tool", "v1.0.0")
	if err != nil || first == nil || first.Source != "fixture" {
		t.Fatalf("successful retry must fetch fixture data: meta=%v error=%v", first, err)
	}
	second, err := reg.GetMetadata("test/tool", "v1.0.0")
	if err != nil || second == nil || second.Source != "cache" || fetcher.calls != 3 {
		t.Fatalf("only successful fetch is cached: meta=%v error=%v calls=%d", second, err, fetcher.calls)
	}
}
