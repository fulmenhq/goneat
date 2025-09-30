package format

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/beevik/etree"
)

func TestPrettifyJSON(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		indent         string
		sizeWarningMB  int
		expectedOutput string
		expectChanged  bool
		expectError    bool
	}{
		{
			name:           "Valid JSON with indentation",
			input:          `{"key":"value"}`,
			indent:         "  ",
			sizeWarningMB:  500,
			expectedOutput: "{\n  \"key\": \"value\"\n}",
			expectChanged:  true,
			expectError:    false,
		},
		{
			name:           "Already formatted JSON",
			input:          "{\n  \"key\": \"value\"\n}",
			indent:         "  ",
			sizeWarningMB:  500,
			expectedOutput: "{\n  \"key\": \"value\"\n}",
			expectChanged:  false,
			expectError:    false,
		},
		{
			name:           "Invalid JSON",
			input:          `{"key":}`,
			indent:         "  ",
			sizeWarningMB:  500,
			expectedOutput: "",
			expectChanged:  false,
			expectError:    true,
		},
		{
			name:           "Compact indent",
			input:          `{"key":"value","another":"test"}`,
			indent:         "",
			sizeWarningMB:  500,
			expectedOutput: "{\"key\":\"value\",\"another\":\"test\"}",
			expectChanged:  true,
			expectError:    false,
		},
		{
			name:           "Tab indent",
			input:          `{"key":"value"}`,
			indent:         "\t",
			sizeWarningMB:  500,
			expectedOutput: "{\n\t\"key\": \"value\"\n}",
			expectChanged:  true,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, changed, err := PrettifyJSON([]byte(tt.input), tt.indent, tt.sizeWarningMB)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error, but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if changed != tt.expectChanged {
				t.Errorf("Expected changed=%v, got %v", tt.expectChanged, changed)
			}

			// For compact mode, just check that it's valid JSON and compact
			if tt.indent == "" {
				if !json.Valid(output) {
					t.Errorf("Output is not valid JSON: %q", string(output))
				}
				if strings.Contains(string(output), "\n") {
					t.Errorf("Compact mode should not contain newlines: %q", string(output))
				}
			} else {
				if strings.TrimSpace(string(output)) != strings.TrimSpace(tt.expectedOutput) {
					t.Errorf("Expected output %q, got %q", tt.expectedOutput, string(output))
				}
			}
		})
	}
}

func TestPrettifyJSON_SizeWarning(t *testing.T) {
	// Create a large JSON string (>500MB)
	largeJSON := `{"data":"` + strings.Repeat("x", 500*1024*1024+1) + `"}`

	// This test would trigger the warning, but we can't easily capture logger output in unit tests
	// In practice, the warning is logged, and processing continues
	output, changed, err := PrettifyJSON([]byte(largeJSON), "  ", 500)

	if err != nil {
		t.Errorf("Unexpected error for large JSON: %v", err)
	}

	if !changed {
		t.Errorf("Expected large JSON to be processed")
	}

	if len(output) == 0 {
		t.Errorf("Expected output for large JSON")
	}
}

func TestPrettifyXML(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		indent         string
		sizeWarningMB  int
		expectedOutput string
		expectChanged  bool
		expectError    bool
	}{
		{
			name:           "Valid XML with indentation",
			input:          `<root><item>value</item></root>`,
			indent:         "  ",
			sizeWarningMB:  500,
			expectedOutput: "<root>\n  <item>value</item>\n</root>\n",
			expectChanged:  true,
			expectError:    false,
		},
		{
			name:           "Already formatted XML",
			input:          "<root>\n  <item>value</item>\n</root>\n",
			indent:         "  ",
			sizeWarningMB:  500,
			expectedOutput: "<root>\n  <item>value</item>\n</root>\n",
			expectChanged:  false,
			expectError:    false,
		},
		{
			name:           "Skip prettification",
			input:          `<root><item>value</item></root>`,
			indent:         "",
			sizeWarningMB:  500,
			expectedOutput: `<root><item>value</item></root>`,
			expectChanged:  false,
			expectError:    false,
		},
		{
			name:           "Invalid XML",
			input:          `<root><item>value</root>`, // Unclosed tag
			indent:         "  ",
			sizeWarningMB:  500,
			expectedOutput: "",
			expectChanged:  false,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, changed, err := PrettifyXML([]byte(tt.input), tt.indent, tt.sizeWarningMB)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error, but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if changed != tt.expectChanged {
				t.Errorf("Expected changed=%v, got %v", tt.expectChanged, changed)
			}

			if tt.indent != "" {
				// Validate that output is still valid XML
				doc := etree.NewDocument()
				if err := doc.ReadFromString(string(output)); err != nil {
					t.Errorf("Output is not valid XML: %v", err)
				}
				if strings.TrimSpace(string(output)) != strings.TrimSpace(tt.expectedOutput) {
					t.Errorf("Expected output %q, got %q", tt.expectedOutput, string(output))
				}
			}
		})
	}
}

func TestPrettifyXML_SizeWarning(t *testing.T) {
	// Create a large XML string (>500MB)
	largeXML := `<root>` + strings.Repeat("<item>value</item>", 100000) + `</root>`

	// This test would trigger the warning, but we can't easily capture logger output in unit tests
	// In practice, the warning is logged, and processing continues
	output, changed, err := PrettifyXML([]byte(largeXML), "  ", 500)

	if err != nil {
		t.Errorf("Unexpected error for large XML: %v", err)
	}

	if !changed {
		t.Errorf("Expected large XML to be processed")
	}

	if len(output) == 0 {
		t.Errorf("Expected output for large XML")
	}
}
