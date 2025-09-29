package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestSave(t *testing.T) {
	// Create temp directory for testing
	tempDir, err := os.MkdirTemp("", "goneat-server-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Mock the config.EnsureGoneatHome to return our temp dir
	originalHome := os.Getenv("GONEAT_HOME")
	_ = os.Setenv("GONEAT_HOME", tempDir)
	defer func() {
		if originalHome == "" {
			_ = os.Unsetenv("GONEAT_HOME")
		} else {
			_ = os.Setenv("GONEAT_HOME", originalHome)
		}
	}()

	tests := []struct {
		name     string
		info     Info
		hasError bool
	}{
		{
			name: "valid info",
			info: Info{
				Name:      "test-server",
				Port:      8080,
				PID:       1234,
				Version:   "1.0.0",
				StartedAt: time.Now(),
			},
			hasError: false,
		},
		{
			name: "missing name",
			info: Info{
				Port:      8080,
				PID:       1234,
				Version:   "1.0.0",
				StartedAt: time.Now(),
			},
			hasError: true,
		},
		{
			name: "invalid port",
			info: Info{
				Name:      "test-server",
				Port:      0,
				PID:       1234,
				Version:   "1.0.0",
				StartedAt: time.Now(),
			},
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Save(tt.info)
			if (err != nil) != tt.hasError {
				t.Errorf("Save() error = %v, hasError %v", err, tt.hasError)
			}

			if !tt.hasError {
				// Verify file was created
				path, _ := metadataPath(tt.info.Name)
				if _, err := os.Stat(path); os.IsNotExist(err) {
					t.Errorf("Save() did not create metadata file")
				}
			}
		})
	}
}

func TestLoad(t *testing.T) {
	// Create temp directory for testing
	tempDir, err := os.MkdirTemp("", "goneat-server-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Mock the config.EnsureGoneatHome to return our temp dir
	originalHome := os.Getenv("GONEAT_HOME")
	_ = os.Setenv("GONEAT_HOME", tempDir)
	defer func() {
		if originalHome == "" {
			_ = os.Unsetenv("GONEAT_HOME")
		} else {
			_ = os.Setenv("GONEAT_HOME", originalHome)
		}
	}()

	testInfo := Info{
		Name:      "test-server",
		Port:      8080,
		PID:       1234,
		Version:   "1.0.0",
		StartedAt: time.Now(),
	}

	// Save first
	err = Save(testInfo)
	if err != nil {
		t.Fatalf("Failed to save test info: %v", err)
	}

	tests := []struct {
		name     string
		loadName string
		expected *Info
		hasError bool
	}{
		{
			name:     "existing server",
			loadName: "test-server",
			expected: &testInfo,
			hasError: false,
		},
		{
			name:     "non-existing server",
			loadName: "non-existing",
			expected: nil,
			hasError: false,
		},
		{
			name:     "empty name",
			loadName: "",
			expected: nil,
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Load(tt.loadName)
			if (err != nil) != tt.hasError {
				t.Errorf("Load() error = %v, hasError %v", err, tt.hasError)
			}

			if tt.expected == nil {
				if result != nil {
					t.Errorf("Load() = %v, want nil", result)
				}
			} else {
				if result == nil {
					t.Errorf("Load() = nil, want %v", tt.expected)
				} else if result.Name != tt.expected.Name || result.Port != tt.expected.Port {
					t.Errorf("Load() = %v, want %v", result, tt.expected)
				}
			}
		})
	}
}

func TestRemove(t *testing.T) {
	// Create temp directory for testing
	tempDir, err := os.MkdirTemp("", "goneat-server-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Mock the config.EnsureGoneatHome to return our temp dir
	originalHome := os.Getenv("GONEAT_HOME")
	_ = os.Setenv("GONEAT_HOME", tempDir)
	defer func() {
		if originalHome == "" {
			_ = os.Unsetenv("GONEAT_HOME")
		} else {
			_ = os.Setenv("GONEAT_HOME", originalHome)
		}
	}()

	testInfo := Info{
		Name:      "test-server",
		Port:      8080,
		PID:       1234,
		Version:   "1.0.0",
		StartedAt: time.Now(),
	}

	// Save first
	err = Save(testInfo)
	if err != nil {
		t.Fatalf("Failed to save test info: %v", err)
	}

	tests := []struct {
		name       string
		removeName string
		hasError   bool
	}{
		{
			name:       "existing server",
			removeName: "test-server",
			hasError:   false,
		},
		{
			name:       "non-existing server",
			removeName: "non-existing",
			hasError:   false, // Remove is idempotent
		},
		{
			name:       "empty name",
			removeName: "",
			hasError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Remove(tt.removeName)
			if (err != nil) != tt.hasError {
				t.Errorf("Remove() error = %v, hasError %v", err, tt.hasError)
			}

			if !tt.hasError && tt.removeName != "" {
				// Verify file was removed
				path, _ := metadataPath(tt.removeName)
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Errorf("Remove() did not remove metadata file")
				}
			}
		})
	}
}

func TestList(t *testing.T) {
	// Create temp directory for testing
	tempDir, err := os.MkdirTemp("", "goneat-server-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Mock the config.EnsureGoneatHome to return our temp dir
	originalHome := os.Getenv("GONEAT_HOME")
	_ = os.Setenv("GONEAT_HOME", tempDir)
	defer func() {
		if originalHome == "" {
			_ = os.Unsetenv("GONEAT_HOME")
		} else {
			_ = os.Setenv("GONEAT_HOME", originalHome)
		}
	}()

	// Save multiple servers
	servers := []Info{
		{Name: "server-a", Port: 8080, PID: 1234, Version: "1.0.0", StartedAt: time.Now()},
		{Name: "server-b", Port: 8081, PID: 1235, Version: "1.0.1", StartedAt: time.Now()},
		{Name: "server-c", Port: 8082, PID: 1236, Version: "1.0.2", StartedAt: time.Now()},
	}

	for _, server := range servers {
		err := Save(server)
		if err != nil {
			t.Fatalf("Failed to save server %s: %v", server.Name, err)
		}
	}

	// Test List
	result, err := List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(result) != len(servers) {
		t.Errorf("List() len = %d, want %d", len(result), len(servers))
	}

	// Verify servers are sorted by name
	expectedNames := []string{"server-a", "server-b", "server-c"}
	for i, server := range result {
		if server.Name != expectedNames[i] {
			t.Errorf("List()[%d].Name = %s, want %s", i, server.Name, expectedNames[i])
		}
	}
}

func TestIsPortAvailable(t *testing.T) {
	tests := []struct {
		name     string
		port     int
		expected bool
	}{
		{"invalid port", 0, false},
		{"negative port", -1, false},
		{"valid port", 8080, true}, // Assuming 8080 is available in test environment
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsPortAvailable(tt.port)
			if result != tt.expected {
				t.Errorf("IsPortAvailable(%d) = %v, want %v", tt.port, result, tt.expected)
			}
		})
	}
}

func TestProbeHello(t *testing.T) {
	// Create a test server
	testResponse := HelloResponse{
		Name:      "test-server",
		Version:   "1.0.0",
		StartedAt: time.Now(),
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/hello" && r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"name":"test-server","version":"1.0.0","started_at":"` + testResponse.StartedAt.Format(time.RFC3339) + `"}`))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Parse the actual port from the test server URL
	serverURL := server.URL
	portStr := serverURL[len("http://127.0.0.1:"):]
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("Failed to parse port from test server URL: %v", err)
	}

	testInfo := Info{
		Name: "test-server",
		Port: port,
	}

	result, err := ProbeHello(testInfo, &http.Client{Timeout: 2 * time.Second})
	if err != nil {
		t.Errorf("ProbeHello() error = %v", err)
	}

	if result == nil {
		t.Errorf("ProbeHello() returned nil")
	} else if result.Name != testResponse.Name {
		t.Errorf("ProbeHello().Name = %s, want %s", result.Name, testResponse.Name)
	}
}

func TestMetadataPath(t *testing.T) {
	// Create temp directory for testing
	tempDir, err := os.MkdirTemp("", "goneat-server-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Mock the config.EnsureGoneatHome to return our temp dir
	originalHome := os.Getenv("GONEAT_HOME")
	_ = os.Setenv("GONEAT_HOME", tempDir)
	defer func() {
		if originalHome == "" {
			_ = os.Unsetenv("GONEAT_HOME")
		} else {
			_ = os.Setenv("GONEAT_HOME", originalHome)
		}
	}()

	path, err := metadataPath("test-server")
	if err != nil {
		t.Errorf("metadataPath() error = %v", err)
	}

	expectedPath := filepath.Join(tempDir, "servers", "test-server.json")
	if path != expectedPath {
		t.Errorf("metadataPath() = %s, want %s", path, expectedPath)
	}
}
