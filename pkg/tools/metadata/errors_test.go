package metadata

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRateLimitError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *RateLimitError
		contains string
	}{
		{
			name: "with retry after",
			err: &RateLimitError{
				Source:     "github",
				RetryAfter: time.Now().Add(42 * time.Minute),
				Limit:      60,
				Remaining:  0,
				Message:    "rate limit exceeded",
			},
			contains: "retry after 42m",
		},
		{
			name: "without retry after",
			err: &RateLimitError{
				Source:    "github",
				Limit:     5000,
				Remaining: 0,
				Message:   "rate limit exceeded",
			},
			contains: "github rate limit exceeded (0/5000)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errMsg := tt.err.Error()
			assert.Contains(t, errMsg, tt.contains)
		})
	}
}

func TestIsRateLimitError(t *testing.T) {
	rateLimitErr := &RateLimitError{Source: "github"}
	regularErr := errors.New("regular error")
	wrappedRateLimitErr := fmt.Errorf("wrapped: %w", rateLimitErr)

	assert.True(t, IsRateLimitError(rateLimitErr))
	assert.True(t, IsRateLimitError(wrappedRateLimitErr))
	assert.False(t, IsRateLimitError(regularErr))
	assert.False(t, IsRateLimitError(nil))
}

func TestGetRetryAfter(t *testing.T) {
	retryTime := time.Now().Add(1 * time.Hour)

	tests := []struct {
		name     string
		err      error
		expected time.Time
	}{
		{
			name: "rate limit error with retry",
			err: &RateLimitError{
				Source:     "github",
				RetryAfter: retryTime,
			},
			expected: retryTime,
		},
		{
			name:     "rate limit error without retry",
			err:      &RateLimitError{Source: "github"},
			expected: time.Time{},
		},
		{
			name:     "regular error",
			err:      errors.New("regular"),
			expected: time.Time{},
		},
		{
			name:     "nil error",
			err:      nil,
			expected: time.Time{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetRetryAfter(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNetworkError(t *testing.T) {
	originalErr := errors.New("connection refused")
	netErr := &NetworkError{
		Source:  "github",
		URL:     "https://api.github.com/repos/test/repo",
		Wrapped: originalErr,
	}

	// Error message should include context
	assert.Contains(t, netErr.Error(), "github")
	assert.Contains(t, netErr.Error(), "https://api.github.com")
	assert.Contains(t, netErr.Error(), "connection refused")

	// Should unwrap to original error
	assert.ErrorIs(t, netErr, originalErr)
}

func TestParseError(t *testing.T) {
	originalErr := errors.New("unexpected EOF")
	parseErr := &ParseError{
		Source:  "github",
		Message: "release response",
		Wrapped: originalErr,
	}

	// Error message should include context
	assert.Contains(t, parseErr.Error(), "github")
	assert.Contains(t, parseErr.Error(), "release response")
	assert.Contains(t, parseErr.Error(), "unexpected EOF")

	// Should unwrap to original error
	assert.ErrorIs(t, parseErr, originalErr)
}
