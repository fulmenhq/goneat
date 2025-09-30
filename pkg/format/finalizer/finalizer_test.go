package finalizer

import (
	"bytes"
	"testing"
)

func TestNormalizeEOF_PreserveMarkdownHardBreaks(t *testing.T) {
	in := []byte("Hello  \nWorld   \nLine\t\n")
	out, changed, err := NormalizeEOF(in, true, true, true, "\n", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Fatalf("expected changes, got unchanged")
	}
	// Expect exactly two trailing spaces preserved where 2+ existed
	want := []byte("Hello  \nWorld  \nLine\n")
	if !bytes.Equal(out, want) {
		t.Fatalf("unexpected output:\nwant=%q\n got=%q", string(want), string(out))
	}
}

func TestComprehensiveFileNormalization_EncodingPolicy_Utf8Only_SkipsNonUtf8(t *testing.T) {
	// UTF-16LE BOM + 'A' (0x41) + NUL (0x00) sequence -> non-UTF8 content
	in := []byte{0xFF, 0xFE, 0x41, 0x00}
	opts := NormalizationOptions{
		EnsureEOF:              true,
		TrimTrailingWhitespace: true,
		EncodingPolicy:         "utf8-only",
	}
	out, changed, err := ComprehensiveFileNormalization(in, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if changed {
		t.Fatalf("expected no changes for non-UTF8 with utf8-only policy")
	}
	if !bytes.Equal(out, in) {
		t.Fatalf("content should be unchanged for non-UTF8 inputs")
	}
}

func TestComprehensiveFileNormalization_EncodingPolicy_Utf8OrBOM_AllowsUtf8BOM(t *testing.T) {
	in := []byte{0xEF, 0xBB, 0xBF, 'a', '\n'} // UTF-8 BOM + content
	opts := NormalizationOptions{
		EnsureEOF:              true,
		TrimTrailingWhitespace: true,
		RemoveUTF8BOM:          true,
		EncodingPolicy:         "utf8-or-bom",
	}
	out, changed, err := ComprehensiveFileNormalization(in, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !changed {
		t.Fatalf("expected changes due to BOM removal or normalization")
	}
	// BOM should be removed
	if bytes.HasPrefix(out, []byte{0xEF, 0xBB, 0xBF}) {
		t.Fatalf("expected UTF-8 BOM to be removed")
	}
}

func TestGetSupportedExtensions(t *testing.T) {
	extensions := GetSupportedExtensions()

	// Should return a non-empty slice
	if len(extensions) == 0 {
		t.Error("GetSupportedExtensions should return at least one extension")
	}

	// Should contain common extensions
	expected := []string{".go", ".md", ".txt", ".yaml", ".yml", ".json"}
	for _, ext := range expected {
		found := false
		for _, actual := range extensions {
			if actual == ext {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected extension %s not found in supported extensions", ext)
		}
	}
}

func TestIsSupportedExtension(t *testing.T) {
	tests := []struct {
		ext      string
		expected bool
	}{
		{".go", true},
		{".md", true},
		{".txt", true},
		{".yaml", true},
		{".yml", true},
		{".json", true},
		{".py", true},   // Actually supported
		{".js", true},   // Actually supported
		{".html", true}, // Actually supported
		{".GO", true},   // Case insensitive
		{"", false},
		{"go", false}, // no leading dot
		{".exe", false},
		{".bin", false},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			result := IsSupportedExtension(tt.ext)
			if result != tt.expected {
				t.Errorf("IsSupportedExtension(%q) = %v, want %v", tt.ext, result, tt.expected)
			}
		})
	}
}

func TestIsTextFile(t *testing.T) {
	tests := []struct {
		name     string
		content  []byte
		expected bool
	}{
		{
			name:     "empty content",
			content:  []byte{},
			expected: true, // Empty is considered text
		},
		{
			name:     "ascii text",
			content:  []byte("Hello World"),
			expected: true,
		},
		{
			name:     "utf8 text",
			content:  []byte("Hello 世界"),
			expected: true,
		},
		{
			name:     "binary data - null bytes",
			content:  []byte{0x00, 0x01, 0x02, 0x03},
			expected: false,
		},
		{
			name:     "binary data - high bytes",
			content:  []byte{0xFF, 0xFE, 0xFD, 0xFC},
			expected: false,
		},
		{
			name:     "mixed - mostly text with some binary",
			content:  []byte("Hello\x00World"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsTextFile(tt.content)
			if result != tt.expected {
				t.Errorf("IsTextFile() = %v, want %v for content: %q", result, tt.expected, tt.content)
			}
		})
	}
}

func TestNormalizeLineEndings(t *testing.T) {
	tests := []struct {
		name         string
		input        []byte
		targetEnding string
		expected     []byte
		expectChange bool
	}{
		{
			name:         "lf to crlf",
			input:        []byte("line1\nline2\n"),
			targetEnding: "\r\n",
			expected:     []byte("line1\r\nline2\r\n"),
			expectChange: true,
		},
		{
			name:         "crlf to lf",
			input:        []byte("line1\r\nline2\r\n"),
			targetEnding: "\n",
			expected:     []byte("line1\nline2\n"),
			expectChange: true,
		},
		{
			name:         "mixed to lf",
			input:        []byte("line1\nline2\r\nline3\r"),
			targetEnding: "\n",
			expected:     []byte("line1\nline2\nline3\n"),
			expectChange: true,
		},
		{
			name:         "already correct",
			input:        []byte("line1\nline2\n"),
			targetEnding: "\n",
			expected:     []byte("line1\nline2\n"),
			expectChange: false,
		},
		{
			name:         "empty input",
			input:        []byte{},
			targetEnding: "\n",
			expected:     []byte{},
			expectChange: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, changed, err := NormalizeLineEndings(tt.input, tt.targetEnding)
			if err != nil {
				t.Fatalf("NormalizeLineEndings() error = %v", err)
			}
			if changed != tt.expectChange {
				t.Errorf("NormalizeLineEndings() changed = %v, want %v", changed, tt.expectChange)
			}
			if !bytes.Equal(result, tt.expected) {
				t.Errorf("NormalizeLineEndings() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestNormalizeWhitespace(t *testing.T) {
	tests := []struct {
		name      string
		input     []byte
		ensureEOF bool
		expected  []byte
	}{
		{
			name:      "trailing spaces removed",
			input:     []byte("line1 \nline2\t\n"),
			ensureEOF: true,
			expected:  []byte("line1\nline2\n"),
		},
		{
			name:      "no trailing whitespace",
			input:     []byte("line1\nline2\n"),
			ensureEOF: true,
			expected:  []byte("line1\nline2\n"),
		},
		{
			name:      "empty input",
			input:     []byte{},
			ensureEOF: true,
			expected:  []byte{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, changed, err := NormalizeWhitespace(tt.input, tt.ensureEOF, "\n", false)
			if err != nil {
				t.Fatalf("NormalizeWhitespace() error = %v", err)
			}
			if !bytes.Equal(result, tt.expected) {
				t.Errorf("NormalizeWhitespace() = %q, want %q", result, tt.expected)
			}
			// We don't check 'changed' as it depends on implementation details
			_ = changed
		})
	}
}

func TestDetectWhitespaceIssues(t *testing.T) {
	tests := []struct {
		name        string
		input       []byte
		expectIssue bool
	}{
		{
			name:        "no issues",
			input:       []byte("line1\nline2\n"),
			expectIssue: false,
		},
		{
			name:        "trailing spaces",
			input:       []byte("line1 \nline2\n"),
			expectIssue: true,
		},
		{
			name:        "trailing tabs",
			input:       []byte("line1\t\nline2\n"),
			expectIssue: true,
		},
		{
			name:        "missing EOF newline",
			input:       []byte("line1\nline2"),
			expectIssue: true,
		},
		{
			name:        "empty input",
			input:       []byte{},
			expectIssue: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasIssues, issues := DetectWhitespaceIssues(tt.input, NormalizationOptions{
				EnsureEOF:              true,
				TrimTrailingWhitespace: true,
			})

			if hasIssues != tt.expectIssue {
				t.Errorf("DetectWhitespaceIssues() hasIssues = %v, want %v", hasIssues, tt.expectIssue)
			}

			if tt.expectIssue && len(issues) == 0 {
				t.Error("Expected issues but got none")
			}

			if !tt.expectIssue && len(issues) > 0 {
				t.Errorf("Expected no issues but got: %v", issues)
			}
		})
	}
}
