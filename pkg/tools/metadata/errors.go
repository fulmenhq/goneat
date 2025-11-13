package metadata

import (
	"errors"
	"fmt"
	"time"
)

// Common metadata errors with structured information for handling

var (
	// ErrNotFound indicates the requested version/release was not found
	ErrNotFound = errors.New("release not found")

	// ErrUnsupportedRepo indicates the repository format is not supported by any fetcher
	ErrUnsupportedRepo = errors.New("unsupported repository format")
)

// RateLimitError indicates a rate limit was hit when fetching metadata
// TODO(Phase 2): Add structured error tagging for rate limit handling
// This enables:
//   - Exponential backoff retry logic
//   - User messaging ("GitHub rate limit exceeded, try again in 42 minutes")
//   - CLI output with RetryAfter hint
//   - Differentiation between 403 rate-limit vs real 404/authentication errors
type RateLimitError struct {
	// Source is the API that rate limited (e.g., "github", "npm")
	Source string

	// RetryAfter is when the rate limit resets (if provided by API)
	// GitHub provides X-RateLimit-Reset header with Unix timestamp
	RetryAfter time.Time

	// Limit is the rate limit that was exceeded (requests per hour)
	Limit int

	// Remaining is how many requests are left (should be 0 when this error occurs)
	Remaining int

	// Message is a human-readable explanation
	Message string
}

func (e *RateLimitError) Error() string {
	if e.RetryAfter.IsZero() {
		return fmt.Sprintf("%s rate limit exceeded (%d/%d): %s", e.Source, e.Remaining, e.Limit, e.Message)
	}

	wait := time.Until(e.RetryAfter)
	if wait < 0 {
		wait = 0
	}

	return fmt.Sprintf("%s rate limit exceeded (%d/%d), retry after %v: %s",
		e.Source, e.Remaining, e.Limit, wait.Round(time.Minute), e.Message)
}

// IsRateLimitError checks if an error is a rate limit error
func IsRateLimitError(err error) bool {
	var rateLimitErr *RateLimitError
	return errors.As(err, &rateLimitErr)
}

// GetRetryAfter extracts the retry-after time from a rate limit error
// Returns zero time if not a rate limit error or no retry time available
func GetRetryAfter(err error) time.Time {
	var rateLimitErr *RateLimitError
	if errors.As(err, &rateLimitErr) {
		return rateLimitErr.RetryAfter
	}
	return time.Time{}
}

// NetworkError indicates a network/transport error when fetching metadata
type NetworkError struct {
	Source  string // API source (e.g., "github")
	URL     string // URL that failed
	Wrapped error  // Underlying error
}

func (e *NetworkError) Error() string {
	return fmt.Sprintf("network error fetching from %s (%s): %v", e.Source, e.URL, e.Wrapped)
}

func (e *NetworkError) Unwrap() error {
	return e.Wrapped
}

// ParseError indicates a response parsing/decoding error
type ParseError struct {
	Source  string // API source
	Message string // What failed to parse
	Wrapped error  // Underlying error
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("failed to parse %s response from %s: %v", e.Message, e.Source, e.Wrapped)
}

func (e *ParseError) Unwrap() error {
	return e.Wrapped
}
