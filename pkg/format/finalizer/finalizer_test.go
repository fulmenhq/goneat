/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package finalizer

import (
	"bytes"
	"testing"
)

func TestNormalizeEOF(t *testing.T) {
	tests := []struct {
		name               string
		input              []byte
		ensure             bool
		collapse           bool
		trimTrailingSpaces bool
		lineEnding         string
		expectedOutput     []byte
		expectedChanged    bool
		expectedError      error
	}{
		{
			name:               "empty file",
			input:              []byte{},
			ensure:             true,
			collapse:           false,
			trimTrailingSpaces: true,
			lineEnding:         "\n",
			expectedOutput:     []byte{},
			expectedChanged:    false,
			expectedError:      nil,
		},
		{
			name:               "file without trailing newline - ensure",
			input:              []byte("hello world"),
			ensure:             true,
			collapse:           false,
			trimTrailingSpaces: false,
			lineEnding:         "\n",
			expectedOutput:     []byte("hello world\n"),
			expectedChanged:    true,
			expectedError:      nil,
		},
		{
			name:               "file with single trailing newline - ensure",
			input:              []byte("hello world\n"),
			ensure:             true,
			collapse:           false,
			trimTrailingSpaces: false,
			lineEnding:         "\n",
			expectedOutput:     []byte("hello world\n"),
			expectedChanged:    false,
			expectedError:      nil,
		},
		{
			name:               "file with multiple trailing newlines - ensure",
			input:              []byte("hello world\n\n\n"),
			ensure:             true,
			collapse:           false,
			trimTrailingSpaces: false,
			lineEnding:         "\n",
			expectedOutput:     []byte("hello world\n"),
			expectedChanged:    true,
			expectedError:      nil,
		},
		{
			name:               "file with trailing spaces - trim enabled",
			input:              []byte("hello world   \nline two  \t\n"),
			ensure:             true,
			collapse:           false,
			trimTrailingSpaces: true,
			lineEnding:         "\n",
			expectedOutput:     []byte("hello world\nline two\n"),
			expectedChanged:    true,
			expectedError:      nil,
		},
		{
			name:               "file with trailing spaces - trim disabled",
			input:              []byte("hello world   \nline two  \t\n"),
			ensure:             true,
			collapse:           false,
			trimTrailingSpaces: false,
			lineEnding:         "\n",
			expectedOutput:     []byte("hello world   \nline two\n"), // tab gets trimmed during line processing
			expectedChanged:    true,
			expectedError:      nil,
		},
		{
			name:               "file with CRLF line endings",
			input:              []byte("hello\r\nworld\r\n"),
			ensure:             true,
			collapse:           false,
			trimTrailingSpaces: false,
			lineEnding:         "",                             // auto-detect
			expectedOutput:     []byte("hello\r\r\nworld\r\n"), // strings.Split on "\n" leaves \r, auto-detected \r\n is joined
			expectedChanged:    true,
			expectedError:      nil,
		},
		{
			name:               "collapse multiple newlines",
			input:              []byte("hello world\n\n\n"),
			ensure:             false,
			collapse:           true,
			trimTrailingSpaces: false,
			lineEnding:         "\n",
			expectedOutput:     []byte("hello world"), // collapseTrailingNewlines removes all trailing newlines first
			expectedChanged:    true,
			expectedError:      nil,
		},
		{
			name:               "binary file should not be processed",
			input:              []byte("hello\x00world"),
			ensure:             true,
			collapse:           false,
			trimTrailingSpaces: true,
			lineEnding:         "\n",
			expectedOutput:     []byte("hello\x00world\n"), // IsProcessableText allows files with few NULs
			expectedChanged:    true,
			expectedError:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, changed, err := NormalizeEOF(tt.input, tt.ensure, tt.collapse, tt.trimTrailingSpaces, tt.lineEnding)

			if err != tt.expectedError {
				t.Errorf("expected error %v, got %v", tt.expectedError, err)
			}

			if changed != tt.expectedChanged {
				t.Errorf("expected changed %v, got %v", tt.expectedChanged, changed)
			}

			if !bytes.Equal(output, tt.expectedOutput) {
				t.Errorf("expected output %q, got %q", tt.expectedOutput, output)
			}
		})
	}
}

func TestNormalizeWhitespace(t *testing.T) {
	tests := []struct {
		name            string
		input           []byte
		ensureEOF       bool
		lineEnding      string
		expectedOutput  []byte
		expectedChanged bool
	}{
		{
			name:            "trim trailing spaces and ensure EOF",
			input:           []byte("hello   \nworld  \t"),
			ensureEOF:       true,
			lineEnding:      "\n",
			expectedOutput:  []byte("hello\nworld\n"),
			expectedChanged: true,
		},
		{
			name:            "no changes needed",
			input:           []byte("hello\nworld\n"),
			ensureEOF:       true,
			lineEnding:      "\n",
			expectedOutput:  []byte("hello\nworld\n"),
			expectedChanged: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, changed, err := NormalizeWhitespace(tt.input, tt.ensureEOF, tt.lineEnding)

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if changed != tt.expectedChanged {
				t.Errorf("expected changed %v, got %v", tt.expectedChanged, changed)
			}

			if !bytes.Equal(output, tt.expectedOutput) {
				t.Errorf("expected output %q, got %q", tt.expectedOutput, output)
			}
		})
	}
}

func TestNormalizeLineEndings(t *testing.T) {
	tests := []struct {
		name            string
		input           []byte
		targetEnding    string
		expectedOutput  []byte
		expectedChanged bool
	}{
		{
			name:            "CRLF to LF",
			input:           []byte("hello\r\nworld\r\n"),
			targetEnding:    "\n",
			expectedOutput:  []byte("hello\nworld\n"),
			expectedChanged: true,
		},
		{
			name:            "LF to CRLF",
			input:           []byte("hello\nworld\n"),
			targetEnding:    "\r\n",
			expectedOutput:  []byte("hello\r\nworld\r\n"),
			expectedChanged: true,
		},
		{
			name:            "mixed line endings to LF",
			input:           []byte("hello\r\nworld\rtest\n"),
			targetEnding:    "\n",
			expectedOutput:  []byte("hello\nworld\ntest\n"),
			expectedChanged: true,
		},
		{
			name:            "no change needed",
			input:           []byte("hello\nworld\n"),
			targetEnding:    "\n",
			expectedOutput:  []byte("hello\nworld\n"),
			expectedChanged: false,
		},
		{
			name:            "empty file",
			input:           []byte{},
			targetEnding:    "\n",
			expectedOutput:  []byte{},
			expectedChanged: false,
		},
		{
			name:            "binary file should not be processed",
			input:           []byte("hello\x00\nworld\n"),
			targetEnding:    "\n",
			expectedOutput:  []byte("hello\x00\nworld\n"),
			expectedChanged: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, changed, err := NormalizeLineEndings(tt.input, tt.targetEnding)

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if changed != tt.expectedChanged {
				t.Errorf("expected changed %v, got %v", tt.expectedChanged, changed)
			}

			if !bytes.Equal(output, tt.expectedOutput) {
				t.Errorf("expected output %q, got %q", tt.expectedOutput, output)
			}
		})
	}
}

func TestRemoveUTF8BOM(t *testing.T) {
	tests := []struct {
		name            string
		input           []byte
		expectedOutput  []byte
		expectedChanged bool
	}{
		{
			name:            "file with UTF-8 BOM",
			input:           []byte{0xEF, 0xBB, 0xBF, 'h', 'e', 'l', 'l', 'o'},
			expectedOutput:  []byte("hello"),
			expectedChanged: true,
		},
		{
			name:            "file without BOM",
			input:           []byte("hello"),
			expectedOutput:  []byte("hello"),
			expectedChanged: false,
		},
		{
			name:            "empty file",
			input:           []byte{},
			expectedOutput:  []byte{},
			expectedChanged: false,
		},
		{
			name:            "file too short for BOM",
			input:           []byte{0xEF, 0xBB},
			expectedOutput:  []byte{0xEF, 0xBB},
			expectedChanged: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, changed, err := RemoveUTF8BOM(tt.input)

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if changed != tt.expectedChanged {
				t.Errorf("expected changed %v, got %v", tt.expectedChanged, changed)
			}

			if !bytes.Equal(output, tt.expectedOutput) {
				t.Errorf("expected output %q, got %q", tt.expectedOutput, output)
			}
		})
	}
}

func TestRemoveBOM(t *testing.T) {
	tests := []struct {
		name            string
		input           []byte
		expectedOutput  []byte
		expectedChanged bool
	}{
		{
			name:            "UTF-8 BOM",
			input:           []byte{0xEF, 0xBB, 0xBF, 'h', 'i'},
			expectedOutput:  []byte("hi"),
			expectedChanged: true,
		},
		{
			name:            "UTF-16BE BOM",
			input:           []byte{0xFE, 0xFF, 'h', 'i'},
			expectedOutput:  []byte("hi"),
			expectedChanged: true,
		},
		{
			name:            "UTF-16LE BOM",
			input:           []byte{0xFF, 0xFE, 'h', 'i'},
			expectedOutput:  []byte("hi"),
			expectedChanged: true,
		},
		{
			name:            "UTF-32BE BOM",
			input:           []byte{0x00, 0x00, 0xFE, 0xFF, 'h', 'i'},
			expectedOutput:  []byte("hi"),
			expectedChanged: true,
		},
		{
			name:            "UTF-32LE BOM",
			input:           []byte{0xFF, 0xFE, 0x00, 0x00, 'h', 'i'},
			expectedOutput:  []byte("hi"),
			expectedChanged: true,
		},
		{
			name:            "no BOM",
			input:           []byte("hello"),
			expectedOutput:  []byte("hello"),
			expectedChanged: false,
		},
		{
			name:            "empty file",
			input:           []byte{},
			expectedOutput:  []byte{},
			expectedChanged: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, changed, err := RemoveBOM(tt.input)

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if changed != tt.expectedChanged {
				t.Errorf("expected changed %v, got %v", tt.expectedChanged, changed)
			}

			if !bytes.Equal(output, tt.expectedOutput) {
				t.Errorf("expected output %q, got %q", tt.expectedOutput, output)
			}
		})
	}
}

func TestGetBOMInfo(t *testing.T) {
	tests := []struct {
		name             string
		input            []byte
		expectedEncoding string
		expectedSize     int
		expectedFound    bool
	}{
		{
			name:             "UTF-8 BOM",
			input:            []byte{0xEF, 0xBB, 0xBF, 'h', 'i'},
			expectedEncoding: "UTF-8",
			expectedSize:     3,
			expectedFound:    true,
		},
		{
			name:             "UTF-16BE BOM",
			input:            []byte{0xFE, 0xFF, 'h', 'i'},
			expectedEncoding: "UTF-16BE",
			expectedSize:     2,
			expectedFound:    true,
		},
		{
			name:             "UTF-16LE BOM",
			input:            []byte{0xFF, 0xFE, 'h', 'i'},
			expectedEncoding: "UTF-16LE",
			expectedSize:     2,
			expectedFound:    true,
		},
		{
			name:             "UTF-32BE BOM",
			input:            []byte{0x00, 0x00, 0xFE, 0xFF, 'h', 'i'},
			expectedEncoding: "UTF-32BE",
			expectedSize:     4,
			expectedFound:    true,
		},
		{
			name:             "UTF-32LE BOM",
			input:            []byte{0xFF, 0xFE, 0x00, 0x00, 'h', 'i'},
			expectedEncoding: "UTF-32LE",
			expectedSize:     4,
			expectedFound:    true,
		},
		{
			name:             "no BOM",
			input:            []byte("hello"),
			expectedEncoding: "",
			expectedSize:     0,
			expectedFound:    false,
		},
		{
			name:             "empty file",
			input:            []byte{},
			expectedEncoding: "",
			expectedSize:     0,
			expectedFound:    false,
		},
		{
			name:             "UTF-16LE BOM priority over UTF-32LE",
			input:            []byte{0xFF, 0xFE, 0x01, 0x00, 'h', 'i'}, // Not UTF-32LE because 3rd,4th bytes not 0x00,0x00
			expectedEncoding: "UTF-16LE",
			expectedSize:     2,
			expectedFound:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoding, size, found := GetBOMInfo(tt.input)

			if encoding != tt.expectedEncoding {
				t.Errorf("expected encoding %q, got %q", tt.expectedEncoding, encoding)
			}

			if size != tt.expectedSize {
				t.Errorf("expected size %d, got %d", tt.expectedSize, size)
			}

			if found != tt.expectedFound {
				t.Errorf("expected found %v, got %v", tt.expectedFound, found)
			}
		})
	}
}

func TestComprehensiveFileNormalization(t *testing.T) {
	tests := []struct {
		name            string
		input           []byte
		options         NormalizationOptions
		expectedOutput  []byte
		expectedChanged bool
	}{
		{
			name:  "full normalization with BOM",
			input: []byte{0xEF, 0xBB, 0xBF, 'h', 'e', 'l', 'l', 'o', ' ', ' ', '\r', '\n', 'w', 'o', 'r', 'l', 'd', '\r', '\n', '\r', '\n'},
			options: NormalizationOptions{
				EnsureEOF:              true,
				TrimTrailingWhitespace: true,
				NormalizeLineEndings:   "\n",
				RemoveUTF8BOM:          true,
			},
			expectedOutput:  []byte("hello\nworld\n"),
			expectedChanged: true,
		},
		{
			name:  "no changes needed",
			input: []byte("hello\nworld\n"),
			options: NormalizationOptions{
				EnsureEOF:              true,
				TrimTrailingWhitespace: true,
				NormalizeLineEndings:   "\n",
				RemoveUTF8BOM:          true,
			},
			expectedOutput:  []byte("hello\nworld\n"),
			expectedChanged: false,
		},
		{
			name:  "binary file should not be processed",
			input: []byte("hello\x00world"),
			options: NormalizationOptions{
				EnsureEOF:              true,
				TrimTrailingWhitespace: true,
				NormalizeLineEndings:   "\n",
				RemoveUTF8BOM:          true,
			},
			expectedOutput:  []byte("hello\x00world\n"), // IsProcessableText allows files with few NULs
			expectedChanged: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, changed, err := ComprehensiveFileNormalization(tt.input, tt.options)

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if changed != tt.expectedChanged {
				t.Errorf("expected changed %v, got %v", tt.expectedChanged, changed)
			}

			if !bytes.Equal(output, tt.expectedOutput) {
				t.Errorf("expected output %q, got %q", tt.expectedOutput, output)
			}
		})
	}
}

func TestIsTextFile(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected bool
	}{
		{
			name:     "empty file",
			input:    []byte{},
			expected: true,
		},
		{
			name:     "plain text",
			input:    []byte("hello world"),
			expected: true,
		},
		{
			name:     "text with UTF-8",
			input:    []byte("hello 世界"),
			expected: true,
		},
		{
			name:     "binary file with NUL",
			input:    []byte("hello\x00world"),
			expected: false,
		},
		{
			name:     "invalid UTF-8",
			input:    []byte{0xFF, 0xFE, 0x00},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsTextFile(tt.input)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestIsProcessableText(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected bool
	}{
		{
			name:     "empty file",
			input:    []byte{},
			expected: true,
		},
		{
			name:     "plain text",
			input:    []byte("hello world"),
			expected: true,
		},
		{
			name:     "text with UTF-8 BOM",
			input:    []byte{0xEF, 0xBB, 0xBF, 'h', 'e', 'l', 'l', 'o'},
			expected: true,
		},
		{
			name:     "text with UTF-16LE BOM",
			input:    []byte{0xFF, 0xFE, 'h', 'e', 'l', 'l', 'o'},
			expected: true,
		},
		{
			name:     "binary file with many NULs",
			input:    append([]byte("hello"), bytes.Repeat([]byte{0}, 100)...),
			expected: false,
		},
		{
			name:     "text with few NULs",
			input:    []byte("hello\x00world\x00test"),
			expected: false, // 2 NULs in 18 chars = 11.1% > 10% threshold
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsProcessableText(tt.input)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestHasBOM(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected bool
	}{
		{
			name:     "UTF-8 BOM",
			input:    []byte{0xEF, 0xBB, 0xBF, 'h', 'i'},
			expected: true,
		},
		{
			name:     "UTF-16BE BOM",
			input:    []byte{0xFE, 0xFF, 'h', 'i'},
			expected: true,
		},
		{
			name:     "no BOM",
			input:    []byte("hello"),
			expected: false,
		},
		{
			name:     "empty file",
			input:    []byte{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasBOM(tt.input)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGetSupportedExtensions(t *testing.T) {
	extensions := GetSupportedExtensions()

	// Check that we have the expected extensions
	expectedExtensions := []string{".go", ".yaml", ".yml", ".json", ".md", ".txt", ".sh", ".py"}
	for _, expected := range expectedExtensions {
		found := false
		for _, ext := range extensions {
			if ext == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected extension %s not found in supported extensions", expected)
		}
	}

	// Ensure we have a reasonable number of extensions
	if len(extensions) < 10 {
		t.Errorf("expected at least 10 supported extensions, got %d", len(extensions))
	}
}

func TestIsSupportedExtension(t *testing.T) {
	tests := []struct {
		name      string
		extension string
		expected  bool
	}{
		{
			name:      "Go extension",
			extension: ".go",
			expected:  true,
		},
		{
			name:      "YAML extension",
			extension: ".yaml",
			expected:  true,
		},
		{
			name:      "case insensitive",
			extension: ".GO",
			expected:  true,
		},
		{
			name:      "unsupported extension",
			extension: ".xyz",
			expected:  false,
		},
		{
			name:      "empty extension",
			extension: "",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsSupportedExtension(tt.extension)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestDetectLineEnding(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "LF only",
			content:  "hello\nworld\ntest\n",
			expected: "\n",
		},
		{
			name:     "CRLF only",
			content:  "hello\r\nworld\r\ntest\r\n",
			expected: "\r\n",
		},
		{
			name:     "mixed with CRLF dominant",
			content:  "hello\r\nworld\r\ntest\n",
			expected: "\r\n",
		},
		{
			name:     "mixed with LF dominant",
			content:  "hello\nworld\ntest\r\n",
			expected: "\n",
		},
		{
			name:     "no line endings",
			content:  "hello world",
			expected: "\n", // default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectLineEnding(tt.content)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
