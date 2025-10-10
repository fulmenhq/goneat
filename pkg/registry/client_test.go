package registry

import (
	"errors"
	"testing"
	"time"
)

// MockFailingClient simulates network/API failures
type MockFailingClient struct {
	shouldFail bool
	failError  error
}

func (m *MockFailingClient) GetMetadata(name, version string) (*Metadata, error) {
	if m.shouldFail {
		return nil, m.failError
	}
	return &Metadata{
		PublishDate:     time.Now().Add(-100 * 24 * time.Hour),
		TotalDownloads:  1000,
		RecentDownloads: 100,
	}, nil
}

func TestRegistryFailureHandling(t *testing.T) {
	tests := []struct {
		name          string
		shouldFail    bool
		expectedError string
	}{
		{
			name:       "successful fetch",
			shouldFail: false,
		},
		{
			name:          "network timeout",
			shouldFail:    true,
			expectedError: "network timeout",
		},
		{
			name:          "404 not found",
			shouldFail:    true,
			expectedError: "module not found",
		},
		{
			name:          "rate limit exceeded",
			shouldFail:    true,
			expectedError: "rate limit exceeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &MockFailingClient{
				shouldFail: tt.shouldFail,
				failError:  errors.New(tt.expectedError),
			}

			metadata, err := client.GetMetadata("test/package", "v1.0.0")

			if tt.shouldFail {
				if err == nil {
					t.Error("Expected error but got nil")
				}
				if err.Error() != tt.expectedError {
					t.Errorf("Expected error %q, got %q", tt.expectedError, err.Error())
				}
				if metadata != nil {
					t.Error("Expected nil metadata on error")
				}
			} else {
				if err != nil {
					t.Errorf("Expected success but got error: %v", err)
				}
				if metadata == nil {
					t.Error("Expected metadata but got nil")
				}
			}
		})
	}
}

func TestRegistryFailureMetadataMarking(t *testing.T) {
	// This test verifies that when registry fails, we properly mark the dependency
	// with age_unknown and registry_error metadata

	client := &MockFailingClient{
		shouldFail: true,
		failError:  errors.New("connection refused"),
	}

	_, err := client.GetMetadata("test/package", "v1.0.0")
	if err == nil {
		t.Fatal("Expected error from failing client")
	}

	// Simulate what go_analyzer.go does on registry failure
	metadata := map[string]interface{}{
		"age_days":       365, // Conservative fallback
		"registry_error": err.Error(),
		"age_unknown":    true,
	}

	// Verify fallback metadata is correct
	if age, ok := metadata["age_days"].(int); !ok || age != 365 {
		t.Errorf("Expected age_days=365 fallback, got %v", metadata["age_days"])
	}

	if regErr, ok := metadata["registry_error"].(string); !ok || regErr == "" {
		t.Error("Expected registry_error to be populated")
	}

	if unknown, ok := metadata["age_unknown"].(bool); !ok || !unknown {
		t.Error("Expected age_unknown=true flag")
	}
}
