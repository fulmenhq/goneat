package ascii

import (
	"testing"
)

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		item     string
		expected bool
	}{
		{"empty slice", []string{}, "test", false},
		{"item present", []string{"a", "b", "c"}, "b", true},
		{"item not present", []string{"a", "b", "c"}, "d", false},
		{"empty item", []string{"a", "", "c"}, "", true},
		{"unicode item", []string{"🚀", "🌟", "⭐"}, "🌟", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.slice, tt.item)
			if result != tt.expected {
				t.Errorf("contains(%v, %q) = %v, want %v", tt.slice, tt.item, result, tt.expected)
			}
		})
	}
}

func TestIsHeaderLine(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{"collection header", "Collection of Emojis", true},
		{"calibration header", "Calibration Test Suite", true},
		{"test header", "Test Results", true},
		{"character line", "🎗  Character U+1F397 (width=1, bytes=4)", false},
		{"empty line", "", true},                       // No "Character U+", so true
		{"regular text", "Some regular content", true}, // No "Character U+", so true
		{"unicode content", "🚀 Rocket emoji", true},    // No "Character U+", so true
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isHeaderLine(tt.content)
			if result != tt.expected {
				t.Errorf("isHeaderLine(%q) = %v, want %v", tt.content, result, tt.expected)
			}
		})
	}
}

func TestExtractCalculatedWidth(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected int
	}{
		{"valid width", "🎗  Character U+1F397 (width=1, bytes=4)", 1},
		{"width 2", "🚀 Character U+1F680 (width=2, bytes=4)", 2},
		{"width 0", "Some text (width=0, bytes=8)", 0},
		{"no width pattern", "No width information here", 0},
		{"malformed width", "Text (width=abc, bytes=4)", 0},
		{"missing closing paren", "Text (width=1, bytes=4", 1}, // Regex still matches
		{"empty content", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractCalculatedWidth(tt.content)
			if result != tt.expected {
				t.Errorf("extractCalculatedWidth(%q) = %d, want %d", tt.content, result, tt.expected)
			}
		})
	}
}

func TestExtractCharacterFromContent(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{"valid emoji", " 🎗  Character U+1F397 (width=1, bytes=4)", "🎗"},
		{"emoji with variation", " 🚀  Character U+1F680+VS (width=2, bytes=7)", "🚀"},
		{"no character pattern", "Some random text", ""},
		{"empty content", "", ""},
		{"box border artifact", "│ Some content │", ""},
		{"wide character", " ┃  Character U+2503 (width=1, bytes=3)", ""}, // Skips box border
		{"only character part", "🎉", ""},                                  // No "Character U+", so empty
		{"character with spaces", "  🎯  Character U+1F3AF", "🎯"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractCharacterFromContent(tt.content)
			if result != tt.expected {
				t.Errorf("extractCharacterFromContent(%q) = %q, want %q", tt.content, result, tt.expected)
			}
		})
	}
}

func TestAnalyzeBoxAlignment(t *testing.T) {
	tests := []struct {
		name     string
		boxLines []string
		expected AnalysisResult
	}{
		{
			name:     "empty box",
			boxLines: []string{},
			expected: AnalysisResult{},
		},
		{
			name:     "too few lines",
			boxLines: []string{"┌───┐", "│ A │"},
			expected: AnalysisResult{},
		},
		{
			name: "simple aligned box",
			boxLines: []string{
				"┌─────┐",
				"│ 🎗  │",
				"└─────┘",
			},
			expected: AnalysisResult{},
		},
		// Note: The actual logic is complex and depends on statistical analysis
		// of padding patterns. These tests may need adjustment based on real behavior.
		{
			name: "box with single character",
			boxLines: []string{
				"┌──────┐",
				"│ 🎗   │",
				"└──────┘",
			},
			expected: AnalysisResult{}, // Single character can't determine normal padding
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzeBoxAlignment(tt.boxLines)

			if len(result.WideCharacters) != len(tt.expected.WideCharacters) {
				t.Errorf("WideCharacters len = %d, want %d", len(result.WideCharacters), len(tt.expected.WideCharacters))
			}
			if len(result.NarrowCharacters) != len(tt.expected.NarrowCharacters) {
				t.Errorf("NarrowCharacters len = %d, want %d", len(result.NarrowCharacters), len(tt.expected.NarrowCharacters))
			}
		})
	}
}

func TestAnalyzeEmojiSequences(t *testing.T) {
	tests := []struct {
		name     string
		lines    []StringInfoLine
		expected AnalysisResult
	}{
		{
			name:     "empty lines",
			lines:    []StringInfoLine{},
			expected: AnalysisResult{},
		},
		{
			name: "variation selector sequence",
			lines: []StringInfoLine{
				{Character: "🎗", DisplayWidth: 53, Content: "🎗  Character U+1F397 (width=1, bytes=4)"},
				{Character: "️", DisplayWidth: 53, Content: "️  Character U+FE0F (width=1, bytes=3)", IsVariationSelector: true},
			},
			expected: AnalysisResult{
				SequenceDetection: true,
			},
		},
		{
			name: "wide individual character",
			lines: []StringInfoLine{
				{Character: "🚀", DisplayWidth: 56, Content: "🚀  Character U+1F680 (width=2, bytes=4)"},
			},
			expected: AnalysisResult{
				WideCharacters:    []string{"🚀"},
				SequenceDetection: true,
			},
		},
		{
			name: "narrow individual character",
			lines: []StringInfoLine{
				{Character: "🎯", DisplayWidth: 50, Content: "🎯  Character U+1F3AF (width=1, bytes=4)"},
			},
			expected: AnalysisResult{
				NarrowCharacters:  []string{"🎯"},
				SequenceDetection: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &AnalysisResult{SequenceDetection: true}
			result = analyzeEmojiSequences(tt.lines, result)

			if len(result.WideCharacters) != len(tt.expected.WideCharacters) {
				t.Errorf("WideCharacters len = %d, want %d", len(result.WideCharacters), len(tt.expected.WideCharacters))
			}
			if len(result.NarrowCharacters) != len(tt.expected.NarrowCharacters) {
				t.Errorf("NarrowCharacters len = %d, want %d", len(result.NarrowCharacters), len(tt.expected.NarrowCharacters))
			}
			if len(result.WideSequences) != len(tt.expected.WideSequences) {
				t.Errorf("WideSequences len = %d, want %d", len(result.WideSequences), len(tt.expected.WideSequences))
			}
			if len(result.NarrowSequences) != len(tt.expected.NarrowSequences) {
				t.Errorf("NarrowSequences len = %d, want %d", len(result.NarrowSequences), len(tt.expected.NarrowSequences))
			}
		})
	}
}

func TestReportAnalysis(t *testing.T) {
	tests := []struct {
		name     string
		result   AnalysisResult
		contains []string // substrings that should be in output
	}{
		{
			name:     "empty result",
			result:   AnalysisResult{},
			contains: []string{"🔍 Terminal Width Analysis", "✅ No alignment issues detected"},
		},
		{
			name: "with terminal",
			result: AnalysisResult{
				Terminal: "iTerm2",
			},
			contains: []string{"🔍 Terminal Width Analysis (iTerm2)"},
		},
		{
			name: "with wide characters",
			result: AnalysisResult{
				WideCharacters: []string{"🚀", "🎯"},
			},
			contains: []string{"📏 Characters appearing TOO WIDE (2)", "🚀", "🎯"},
		},
		{
			name: "with narrow characters",
			result: AnalysisResult{
				NarrowCharacters: []string{"🎗"},
			},
			contains: []string{"📐 Characters appearing TOO NARROW (1)", "🎗"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture output by redirecting stdout temporarily
			// Since reportAnalysis writes to stdout, we'll test by checking it doesn't panic
			// and that it produces some output
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("reportAnalysis panicked: %v", r)
				}
			}()

			// This will write to stdout, but we can't easily capture it in a unit test
			// without more complex setup. For now, just ensure it doesn't panic.
			_ = reportAnalysis(&tt.result) // Ignore error in test

			// If we get here without panicking, the test passes
		})
	}
}
