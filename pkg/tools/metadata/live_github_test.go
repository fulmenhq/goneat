package metadata_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/fulmenhq/goneat/pkg/tools/metadata"
)

// TestRegistry_GetMetadata_LiveGitHub retains the real unauthenticated lookup
// formerly exercised by the caching example. It is separate live coverage:
// GONEAT_METADATA_LIVE_GITHUB=1 make test GOTEST='go test -count=1 -run TestRegistry_GetMetadata_LiveGitHub'
// Default fixture gate success does not establish this live result.
func TestRegistry_GetMetadata_LiveGitHub(t *testing.T) {
	if os.Getenv("GONEAT_METADATA_LIVE_GITHUB") != "1" {
		t.Skip("live GitHub coverage not requested; set GONEAT_METADATA_LIVE_GITHUB=1 explicitly")
	}
	reg := metadata.NewRegistry(24 * time.Hour)
	reg.RegisterFetcher("github", metadata.NewGitHubFetcher("", 30*time.Second))
	first, err := reg.GetMetadata("golangci/golangci-lint", "v2.4.0")
	if err != nil {
		// Do not print response bodies, headers, credentials or ambient environment.
		t.Fatalf("explicit live GitHub lookup failed: %s; no skip or fixture fallback", liveMetadataFailure(err))
	}
	if first == nil || first.Source != "github" {
		t.Fatalf("live lookup did not return GitHub metadata: %+v", first)
	}
	second, err := reg.GetMetadata("golangci/golangci-lint", "v2.4.0")
	if err != nil || second == nil || second.Source != "cache" {
		t.Fatalf("live cache lookup failed: nil=%t source-cache=%t cause=%s", second == nil, second != nil && second.Source == "cache", liveMetadataFailure(err))
	}
}

// Report known structured classifications, never raw response bodies, URLs,
// headers or arbitrary error strings. Unknown errors remain failures with their
// type and an explicit diagnostic gap rather than an invented cause.
func liveMetadataFailure(err error) string {
	if err == nil {
		return "none"
	}
	var rate *metadata.RateLimitError
	var network *metadata.NetworkError
	var parse *metadata.ParseError
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "context deadline exceeded"
	case errors.Is(err, metadata.ErrNotFound):
		return "release not found"
	case errors.As(err, &rate):
		return fmt.Sprintf("rate-limit classification: limit=%d remaining=%d retry-after=%s", rate.Limit, rate.Remaining, rate.RetryAfter.UTC().Format(time.RFC3339))
	case errors.As(err, &parse):
		return "response parse failure"
	case errors.As(err, &network):
		if code := liveHTTPStatus(network.Wrapped); code != "" {
			return "server failure: " + code
		}
		return fmt.Sprintf("network classification; transport cause type=%T (detail withheld)", network.Wrapped)
	}
	if code := liveHTTPStatus(err); code != "" {
		return "API failure: " + code
	}
	return fmt.Sprintf("cause type=%T; unstructured detail withheld", err)
}

var liveHTTPStatusPattern = regexp.MustCompile(`^GitHub (?:API|server) error: (HTTP [1-5][0-9]{2})$`)

func liveHTTPStatus(err error) string {
	for err != nil {
		if match := liveHTTPStatusPattern.FindStringSubmatch(err.Error()); len(match) == 2 {
			return match[1]
		}
		err = errors.Unwrap(err)
	}
	return ""
}

func TestLiveMetadataFailureSanitized(t *testing.T) {
	for _, tt := range []struct {
		err  error
		want string
	}{
		{fmt.Errorf("fetch: %w", context.DeadlineExceeded), "context deadline exceeded"},
		{fmt.Errorf("fetch: %w", metadata.ErrNotFound), "release not found"},
		{fmt.Errorf("fetch: %w", fmt.Errorf("GitHub API error: HTTP 401")), "HTTP 401"},
		{&metadata.NetworkError{Wrapped: fmt.Errorf("GitHub server error: HTTP 503")}, "HTTP 503"},
		{&metadata.RateLimitError{Message: "secret-response"}, "rate-limit classification"},
		{&metadata.ParseError{Message: "secret-response", Wrapped: errors.New("secret-response")}, "response parse failure"},
		{&metadata.NetworkError{URL: "secret-response", Wrapped: errors.New("secret-response")}, "network classification"},
		{errors.New("secret-response"), "detail withheld"},
	} {
		result := liveMetadataFailure(tt.err)
		if !strings.Contains(result, tt.want) || strings.Contains(result, "secret-response") {
			t.Errorf("sanitized cause lost classification or leaked raw detail: %q", result)
		}
	}
}
