package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestNewHierarchicalConfig(t *testing.T) {
	hc := NewHierarchicalConfig()
	if hc == nil {
		t.Fatal("NewHierarchicalConfig() returned nil")
	}
	if hc.sources == nil {
		t.Error("sources slice should be initialized")
	}
	if hc.cache == nil {
		t.Error("cache map should be initialized")
	}
	if hc.merger == nil {
		t.Error("merger should be initialized")
	}
}

func TestHierarchicalConfigAddSource(t *testing.T) {
	hc := NewHierarchicalConfig()

	// Create a mock config source
	source := &mockConfigSource{name: "test", priority: 100}

	hc.AddSource(source)

	if len(hc.sources) != 1 {
		t.Errorf("Expected 1 source, got %d", len(hc.sources))
	}
	if hc.sources[0] != source {
		t.Error("Source was not added correctly")
	}
}

func TestNewFileConfigSource(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test.yaml")

	// Create test config file
	configContent := `format:
  go:
    simplify: true`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	source := NewFileConfigSource(configPath, 100)
	if source == nil {
		t.Fatal("NewFileConfigSource returned nil")
	}
	if source.Priority() != 100 {
		t.Errorf("Expected priority 100, got %d", source.Priority())
	}
	if source.Name() == "" {
		t.Error("Name should not be empty")
	}
}

func TestFileConfigSourceLoad(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test.yaml")

	// Create test config file
	configContent := `format:
  go:
    simplify: true
  yaml:
    indent: 4`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create test config: %v", err)
	}

	source := NewFileConfigSource(configPath, 100)
	ctx := context.Background()

	v, err := source.Load(ctx)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if v == nil {
		t.Fatal("Load() returned nil viper instance")
	}

	// Check that config was loaded correctly
	if !v.GetBool("format.go.simplify") {
		t.Error("Expected format.go.simplify to be true")
	}
	if v.GetInt("format.yaml.indent") != 4 {
		t.Errorf("Expected format.yaml.indent to be 4, got %d", v.GetInt("format.yaml.indent"))
	}
}

func TestFileConfigSourceLoadNonExistentFile(t *testing.T) {
	source := NewFileConfigSource("/non/existent/file.yaml", 100)
	ctx := context.Background()

	_, err := source.Load(ctx)
	if err == nil {
		t.Error("Expected Load() to fail for non-existent file")
	}
}

func TestNewEnvConfigSource(t *testing.T) {
	source := NewEnvConfigSource("GONEAT", 200)
	if source == nil {
		t.Fatal("NewEnvConfigSource returned nil")
	}
	if source.Priority() != 200 {
		t.Errorf("Expected priority 200, got %d", source.Priority())
	}
	if source.Name() == "" {
		t.Error("Name should not be empty")
	}
}

func TestEnvConfigSourceLoad(t *testing.T) {
	// Set test environment variables
	originalEnv := os.Getenv("GONEAT_FORMAT_GO_SIMPLIFY")
	defer func() {
		if originalEnv == "" {
			_ = os.Unsetenv("GONEAT_FORMAT_GO_SIMPLIFY")
		} else {
			_ = os.Setenv("GONEAT_FORMAT_GO_SIMPLIFY", originalEnv)
		}
	}()

	_ = os.Setenv("GONEAT_FORMAT_GO_SIMPLIFY", "false")

	source := NewEnvConfigSource("GONEAT", 200)
	ctx := context.Background()

	v, err := source.Load(ctx)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if v == nil {
		t.Fatal("Load() returned nil viper instance")
	}

	// Check that environment variable was loaded
	if v.GetBool("format.go.simplify") {
		t.Error("Expected format.go.simplify to be false from env var")
	}
}

func TestDetectConfigTypeBasic(t *testing.T) {
	tests := []struct {
		filename string
		expected string
	}{
		{"config.yaml", "yaml"},
		{"config.yml", "yaml"},
		{"config.json", "json"},
		{"config.toml", "toml"},
		{"config", "yaml"}, // default
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := detectConfigType(tt.filename)
			if result != tt.expected {
				t.Errorf("detectConfigType(%q) = %q, expected %q", tt.filename, result, tt.expected)
			}
		})
	}
}

func TestLoadEnterpriseConfigBasic(t *testing.T) {
	// Test with empty sources (should return empty config)
	ctx := context.Background()
	config, err := LoadEnterpriseConfig(ctx)
	if err != nil {
		t.Errorf("LoadEnterpriseConfig() failed: %v", err)
	}
	if config == nil {
		t.Error("LoadEnterpriseConfig() returned nil config")
	}
}

// Mock config source for testing
type mockConfigSource struct {
	name     string
	priority int
	config   map[string]interface{}
	err      error
}

func (m *mockConfigSource) Load(ctx context.Context) (*viper.Viper, error) {
	if m.err != nil {
		return nil, m.err
	}

	v := viper.New()
	if m.config != nil {
		for key, value := range m.config {
			v.Set(key, value)
		}
	}
	return v, nil
}

func (m *mockConfigSource) Priority() int {
	return m.priority
}

func (m *mockConfigSource) Name() string {
	return m.name
}

// Note: DefaultConfigMerger is already defined in hierarchy.go

func TestHierarchicalConfigLoadWithSources(t *testing.T) {
	hc := NewHierarchicalConfig()

	// Add sources with different priorities
	lowPrioritySource := &mockConfigSource{
		name:     "low",
		priority: 100,
		config:   map[string]interface{}{"test.value": "low", "low.only": "present"},
	}

	highPrioritySource := &mockConfigSource{
		name:     "high",
		priority: 200,
		config:   map[string]interface{}{"test.value": "high", "high.only": "present"},
	}

	hc.AddSource(lowPrioritySource)
	hc.AddSource(highPrioritySource)

	ctx := context.Background()
	config, err := hc.Load(ctx)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if config == nil {
		t.Fatal("Load() returned nil config")
	}

	// Since Load returns *Config, we can't directly test the merged values here
	// This test verifies the basic functionality works
	// To test the actual merge behavior, we'd need to examine the underlying implementation
}
