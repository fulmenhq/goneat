package ascii

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestGetTerminalWidth(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "simple ascii",
			input:    "hello",
			expected: 5,
		},
		{
			name:     "empty string",
			input:    "",
			expected: 0,
		},
		{
			name:     "unicode characters",
			input:    "🚀🌟",
			expected: 4, // 2 wide characters
		},
		{
			name:     "mixed content",
			input:    "test 🚀",
			expected: 7, // 5 + 2
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetTerminalWidth(tt.input)
			if result != tt.expected {
				t.Errorf("GetTerminalWidth(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetTerminalConfig(t *testing.T) {
	config := GetTerminalConfig()

	// Config may be nil if no terminal detected, but should not panic
	// We can't predict exact config without knowing the test environment
	if config != nil {
		// If config exists, it should have a name
		if config.Name == "" {
			t.Error("TerminalConfig should have a non-empty name when present")
		}
	}
}

func TestGetAllTerminalConfigs(t *testing.T) {
	configs := GetAllTerminalConfigs()

	// Should return a map, may be nil if catalog not loaded
	if configs != nil {
		// If loaded, should contain some terminals
		if len(configs) == 0 {
			t.Error("Expected some terminal configurations when catalog is loaded")
		}

		// Check that each config has required fields
		for termID, config := range configs {
			if termID == "" {
				t.Error("Terminal ID should not be empty")
			}
			if config.Name == "" {
				t.Errorf("Terminal %s should have a non-empty name", termID)
			}
		}
	}
}

func TestExportTerminalData(t *testing.T) {
	data, err := ExportTerminalData()

	if err != nil {
		t.Fatalf("ExportTerminalData() error = %v", err)
	}

	// Should return valid JSON
	if data == "" {
		t.Error("Expected non-empty JSON data")
	}

	// Should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(data), &result); err != nil {
		t.Errorf("ExportTerminalData() returned invalid JSON: %v", err)
	}

	// Should contain version field
	if _, exists := result["version"]; !exists {
		t.Error("JSON should contain version field")
	}
}

func TestTerminalTestReport(t *testing.T) {
	// Test with a known terminal program name
	report := TerminalTestReport("xterm")

	if report == "" {
		t.Error("TerminalTestReport should return non-empty string")
	}

	// Should contain the terminal name
	if !strings.Contains(report, "xterm") {
		t.Errorf("Report should contain terminal name 'xterm', got: %s", report)
	}

	// Should have basic structure
	lines := strings.Split(strings.TrimSpace(report), "\n")
	if len(lines) < 3 {
		t.Error("Report should have at least 3 lines")
	}

	// First line should contain terminal name
	if !strings.Contains(lines[0], "xterm") {
		t.Errorf("First line should contain terminal name, got: %s", lines[0])
	}
}

func TestTerminalTestReport_UnknownTerminal(t *testing.T) {
	// Test with unknown terminal
	report := TerminalTestReport("unknown-terminal-12345")

	if report == "" {
		t.Error("TerminalTestReport should return non-empty string even for unknown terminals")
	}

	// Should still contain the terminal name
	if !strings.Contains(report, "unknown-terminal-12345") {
		t.Errorf("Report should contain terminal name even for unknown, got: %s", report)
	}
}
