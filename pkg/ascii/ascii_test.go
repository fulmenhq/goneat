package ascii

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBox(t *testing.T) {
	forceTestTerminal(t, "ghostty")

	tests := []struct {
		name  string
		lines []string
		want  string
	}{
		{
			name:  "single line",
			lines: []string{"Hello"},
			want:  "┌───────┐\n│ Hello │\n└───────┘\n",
		},
		{
			name:  "multiple lines",
			lines: []string{"Line 1", "Longer line here", "Short"},
			want: "┌──────────────────┐\n" +
				"│ Line 1           │\n" +
				"│ Longer line here │\n" +
				"│ Short            │\n" +
				"└──────────────────┘\n",
		},
		{
			name:  "emoji width",
			lines: []string{"Status: ✅", "Guard 🛡️"},
			want: "┌────────────┐\n" +
				"│ Status: ✅ │\n" +
				"│ Guard 🛡️    │\n" +
				"└────────────┘\n",
		},
		{
			name: "guardian approval box",
			lines: []string{
				"GUARDIAN APPROVAL REQUIRED for project goneat on operation 'git.commit'",
				"═══════════════════════════════════════════════════════════════════════",
				"",
				"Open this URL in your browser to approve/deny the operation:",
				"",
				"🔗  http://127.0.0.1:63411/approve/test-token-placeholder",
				"",
				"⏱️  Expires in:  4:59",
				"",
				"📋  Copy the URL: Select the link above or use Ctrl+C / right-click copy",
				"",
				"📂  Project folder: goneat",
				"💻  Machine: bluefin.local",
				"",
				"ℹ️  Auto-open was attempted (if enabled). If it opened in the wrong",
				"     browser/profile, or this is CI/CD/headless, paste the URL manually.",
				"     No browser? Use curl or another tool to visit the URL.",
			},
			want: "┌──────────────────────────────────────────────────────────────────────────┐\n" +
				"│ GUARDIAN APPROVAL REQUIRED for project goneat on operation 'git.commit'  │\n" +
				"│ ═══════════════════════════════════════════════════════════════════════  │\n" +
				"│                                                                          │\n" +
				"│ Open this URL in your browser to approve/deny the operation:             │\n" +
				"│                                                                          │\n" +
				"│ 🔗  http://127.0.0.1:63411/approve/test-token-placeholder                │\n" +
				"│                                                                          │\n" +
				"│ ⏱️  Expires in:  4:59                                                    │\n" +
				"│                                                                          │\n" +
				"│ 📋  Copy the URL: Select the link above or use Ctrl+C / right-click copy │\n" +
				"│                                                                          │\n" +
				"│ 📂  Project folder: goneat                                               │\n" +
				"│ 💻  Machine: bluefin.local                                               │\n" +
				"│                                                                          │\n" +
				"│ ℹ️  Auto-open was attempted (if enabled). If it opened in the wrong      │\n" +
				"│      browser/profile, or this is CI/CD/headless, paste the URL manually. │\n" +
				"│      No browser? Use curl or another tool to visit the URL.              │\n" +
				"└──────────────────────────────────────────────────────────────────────────┘\n",
		},
		{
			name: "mixed unicode characters",
			lines: []string{
				"Unicode Test Suite 🚀",
				"─────────────────────",
				"Emojis: 😀 🎉 ⭐ 🌟",
				"Math: α β γ ∑ ∫ √",
				"Arrows: ← → ↑ ↓ ↔",
				"Symbols: © ® ™ € £ ¥",
				"CJK: 你好 こんにちは 안녕하세요",
				"Combining: n̈ ô å",
			},
			want: "┌─────────────────────────────────┐\n" +
				"│ Unicode Test Suite 🚀           │\n" +
				"│ ─────────────────────           │\n" +
				"│ Emojis: 😀 🎉 ⭐ 🌟             │\n" +
				"│ Math: α β γ ∑ ∫ √               │\n" +
				"│ Arrows: ← → ↑ ↓ ↔               │\n" +
				"│ Symbols: © ® ™ € £ ¥            │\n" +
				"│ CJK: 你好 こんにちは 안녕하세요 │\n" +
				"│ Combining: n̈ ô å                │\n" +
				"└─────────────────────────────────┘\n",
		},
		{
			name: "wide characters only",
			lines: []string{
				"ＷＩＤＥ　ＣＨＡＲＡＣＴＥＲＳ",
				"━━━━━━━━━━━━━━━━━━━━━",
				"全角文字テスト",
				"ひらがなカタカナ",
				"１２３４５６７８９０",
			},
			want: "┌────────────────────────────────┐\n" +
				"│ ＷＩＤＥ　ＣＨＡＲＡＣＴＥＲＳ │\n" +
				"│ ━━━━━━━━━━━━━━━━━━━━━          │\n" +
				"│ 全角文字テスト                 │\n" +
				"│ ひらがなカタカナ               │\n" +
				"│ １２３４５６７８９０           │\n" +
				"└────────────────────────────────┘\n",
		},
		{
			name: "zero-width characters",
			lines: []string{
				"Text with combining marks: çâűt̆įǫn̨",
				"Emoji with modifiers: 👨‍💻 👩🏽‍🔬 🧑🏻‍🎨",
				"Flags: 🇺🇸 🇯🇵 🇰🇷 🇩🇪 🇫🇷",
				"ZWSP test: word1\u200bword2",
				"ZWJ test: 👨‍👩‍👧‍👦",
			},
			want: "┌────────────────────────────────────┐\n" +
				"│ Text with combining marks: çâűt̆įǫn̨ │\n" +
				"│ Emoji with modifiers: 👨‍💻 👩🏽‍🔬 🧑🏻‍🎨     │\n" +
				"│ Flags: 🇺🇸 🇯🇵 🇰🇷 🇩🇪 🇫🇷                   │\n" +
				"│ ZWSP test: word1\u200bword2              │\n" +
				"│ ZWJ test: 👨‍👩‍👧‍👦                       │\n" +
				"└────────────────────────────────────┘\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Box(tt.lines); got != tt.want {
				t.Errorf("Box() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDrawBoxEmpty(t *testing.T) {
	DrawBox(nil)
	DrawBox([]string{})
}

func TestStringWidth(t *testing.T) {
	forceTestTerminal(t, "ghostty")

	tests := []struct {
		input string
		want  int
	}{
		{"Hello", 5},
		{"Hello 🌟", 8},                       // 6 chars + 2 for emoji
		{"⏱️   Expires in:  4:59", 22},       // Timer emoji + spaces + text (width 2 in Ghostty)
		{"ℹ️   Auto-open was attempted", 28}, // Info emoji + spaces + text (width 2 in Ghostty)
		{"🛡️  Project: goneat", 18},          // Shield emoji + text
		{"🔗  http://127.0.0.1:63411/approve/test-token-placeholder", 57},                // Link emoji + URL
		{"⏱️  Expires in:  4:59", 21},                                                   // Timer emoji + text (width 2 in Ghostty)
		{"📋  Copy the URL: Select the link above or use Ctrl+C / right-click copy", 72}, // Clipboard emoji + text
		{"📂  Project folder: goneat", 26},                                               // Folder emoji + text
		{"💻  Machine: bluefin.local", 26},                                               // Computer emoji + text
		{"ℹ️  Auto-open was attempted (if enabled). If it opened in the wrong", 67},     // Info emoji + text (width 2 in Ghostty)
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := StringWidth(tt.input); got != tt.want {
				t.Errorf("StringWidth(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestRuneWidth(t *testing.T) {
	tests := []struct {
		r    rune
		want int
		desc string
	}{
		{'A', 1, "ASCII letter"},
		{'\U0001f680', 2, "rocket emoji"},
		{'\u4f60', 2, "CJK character"},
		{'a', 1, "lowercase ASCII"},
		{'\u0300', 0, "combining mark"},
		{'\U0001f3fb', 2, "emoji modifier"},
		{'\U0001f1e6', 1, "regional indicator"},
		{'\u0000', 0, "null character"},
		{'\ufe0f', 1, "variation selector"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			if got := RuneWidth(tt.r); got != tt.want {
				t.Errorf("RuneWidth(%q U+%04X) = %d, want %d", tt.r, tt.r, got, tt.want)
			}
		})
	}
}

func TestStringWidthInfo(t *testing.T) {
	tests := []struct {
		input            string
		wantByteLen      int
		wantDisplayWidth int
		desc             string
	}{
		{"Hello", 5, 5, "ASCII string"},
		{"Hello 🌟", 10, 8, "ASCII with emoji"},
		{"你好世界", 12, 8, "CJK string"},
		{"", 0, 0, "empty string"},
		{"café", 5, 4, "string with combining mark"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			byteLen, displayWidth := StringWidthInfo(tt.input)
			if byteLen != tt.wantByteLen {
				t.Errorf("StringWidthInfo(%q) byteLen = %d, want %d", tt.input, byteLen, tt.wantByteLen)
			}
			if displayWidth != tt.wantDisplayWidth {
				t.Errorf("StringWidthInfo(%q) displayWidth = %d, want %d", tt.input, displayWidth, tt.wantDisplayWidth)
			}
		})
	}
}

func TestAnalyzeString(t *testing.T) {
	tests := []struct {
		input string
		want  []CharInfo
		desc  string
	}{
		{
			"Hi!",
			[]CharInfo{
				{'H', 0, 1, 1, "H"},
				{'i', 1, 2, 1, "i"},
				{'!', 2, 3, 1, "!"},
			},
			"ASCII string",
		},
		{
			"🚀",
			[]CharInfo{
				{'\U0001f680', 0, 4, 2, "🚀"},
			},
			"single emoji",
		},
		{
			"café",
			[]CharInfo{
				{'c', 0, 1, 1, "c"},
				{'a', 1, 2, 1, "a"},
				{'f', 2, 3, 1, "f"},
				{'é', 3, 5, 1, "é"}, // é is 2 bytes but 1 display width
			},
			"string with accented character",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got := AnalyzeString(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("AnalyzeString(%q) len = %d, want %d", tt.input, len(got), len(tt.want))
			}
			for i, want := range tt.want {
				if got[i] != want {
					t.Errorf("AnalyzeString(%q)[%d] = %+v, want %+v", tt.input, i, got[i], want)
				}
			}
		})
	}
}

func TestTruncateForBox(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		width    int
		expected string
	}{
		{"no truncation", "Hello", 10, "Hello"},
		{"truncation", "This is a very long string", 10, "This is..."},
		{"exact width", "Hello", 5, "Hello"},
		{"width too small", "Hello", 2, "He"},
		{"unicode", "Hello 世界 🌍", 10, "Hello ..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TruncateForBox(tt.value, tt.width); got != tt.expected {
				t.Errorf("TruncateForBox(%q, %d) = %q, want %q", tt.value, tt.width, got, tt.expected)
			}
		})
	}
}

// testFixtureBox tests that a fixture file produces a properly aligned box
func testFixtureBox(t *testing.T, fixtureName string) {
	fixturePath := filepath.Join("..", "..", "tests", "fixtures", "ascii", fixtureName)
	content, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("Failed to read fixture file %s: %v", fixtureName, err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")

	// Calculate expected box dimensions
	maxWidth := 0
	for _, line := range lines {
		if w := StringWidth(line); w > maxWidth {
			maxWidth = w
		}
	}
	expectedInnerWidth := maxWidth + 1 + 1         // left padding + right padding
	expectedDisplayWidth := expectedInnerWidth + 2 // add borders

	t.Logf("Fixture: %s", fixtureName)
	t.Logf("Max input width: %d", maxWidth)
	t.Logf("Expected inner width: %d", expectedInnerWidth)
	t.Logf("Expected display width: %d", expectedDisplayWidth)

	// Debug: print the input lines and their widths
	for i, line := range lines {
		byteLen, displayWidth := StringWidthInfo(line)
		t.Logf("Input line %d: %q (len=%d, width=%d)", i, line, byteLen, displayWidth)
	}

	result := Box(lines)

	// The result should be a properly formatted box
	// Check that it starts and ends with box drawing characters
	if !strings.HasPrefix(result, "┌") || !strings.HasSuffix(strings.TrimSpace(result), "┘") {
		t.Errorf("Box result doesn't have proper box drawing characters")
	}

	// Check that all lines have the same display width
	boxLines := strings.Split(strings.TrimSpace(result), "\n")
	if len(boxLines) < 3 {
		t.Errorf("Box should have at least 3 lines (top, content, bottom)")
	}

	// Debug: print the output lines and their properties
	for i, line := range boxLines {
		displayWidth := StringWidth(line)
		byteLength := len(line)
		t.Logf("Output line %d: display_width=%d, byte_len=%d", i, displayWidth, byteLength)
	}

	// All lines should have the same display width
	actualDisplayWidth := StringWidth(boxLines[0])
	if actualDisplayWidth != expectedDisplayWidth {
		t.Errorf("Box display width %d doesn't match expected %d", actualDisplayWidth, expectedDisplayWidth)
	}

	for i, line := range boxLines {
		if width := StringWidth(line); width != actualDisplayWidth {
			t.Errorf("Line %d has display width %d, expected %d", i, width, actualDisplayWidth)
		}
	}

	// Verify the box structure
	topLine := boxLines[0]
	bottomLine := boxLines[len(boxLines)-1]

	if !strings.HasPrefix(topLine, "┌") || !strings.HasSuffix(topLine, "┐") {
		t.Errorf("Top line doesn't have proper borders: %q", topLine)
	}
	if !strings.HasPrefix(bottomLine, "└") || !strings.HasSuffix(bottomLine, "┘") {
		t.Errorf("Bottom line doesn't have proper borders: %q", bottomLine)
	}

	// Content lines should start and end with │
	for i := 1; i < len(boxLines)-1; i++ {
		line := boxLines[i]
		if !strings.HasPrefix(line, "│") || !strings.HasSuffix(line, "│") {
			t.Errorf("Content line %d doesn't have proper borders: %q", i, line)
		}
	}
}

func TestBoxWithFixture(t *testing.T) {
	testFixtureBox(t, "samples/guardian-approval.txt")
}

func TestUnicodeTestSuiteFixture(t *testing.T) {
	testFixtureBox(t, "calibration/unicode-suite.txt")
}

func TestEmojisCollectionFixture(t *testing.T) {
	testFixtureBox(t, "samples/emojis-collection.txt")
}

func TestMathSymbolsFixture(t *testing.T) {
	testFixtureBox(t, "calibration/math-symbols.txt")
}

func TestCJKCharactersFixture(t *testing.T) {
	testFixtureBox(t, "samples/cjk-text.txt")
}

func TestWideCharactersFixture(t *testing.T) {
	testFixtureBox(t, "calibration/wide-characters.txt")
}

func TestLoggingEmojisFixture(t *testing.T) {
	testFixtureBox(t, "calibration/logging-emojis.txt")
}

func TestValidateLoggingEmojisBox(t *testing.T) {
	// Read the logging-emojis.txt fixture
	fixturePath := filepath.Join("..", "..", "tests", "fixtures", "ascii", "calibration", "logging-emojis.txt")
	content, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("Failed to read fixture file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")

	// Generate the box
	box := Box(lines)

	// Split box output into lines
	boxLines := strings.Split(strings.TrimSpace(box), "\n")

	// Check each line's display width
	t.Logf("Box has %d lines", len(boxLines))
	for i, line := range boxLines {
		byteLen, displayWidth := StringWidthInfo(line)
		t.Logf("Line %d: len=%d, width=%d, content=%q", i, byteLen, displayWidth, line)
	}

	// Check if all lines have the same display width
	if len(boxLines) > 0 {
		expectedWidth := StringWidth(boxLines[0])
		for i, line := range boxLines {
			if width := StringWidth(line); width != expectedWidth {
				t.Errorf("Line %d has display width %d, expected %d", i, width, expectedWidth)
			}
		}
		t.Logf("All lines should have display width: %d", expectedWidth)
	}
}

func TestLoggingEmojiWidths(t *testing.T) {
	emojis := []string{
		"ℹ️  Information message",
		"⚠️  Warning message",
		"❌  Error message",
		"✅  Success message",
		"🔄  Processing message",
		"⏳  Waiting message",
		"⏱️  Timer message",
		"📋  Copy message",
		"📂  Folder message",
		"💻  Machine message",
		"🔗  Link message",
		"🚀  Launch message",
		"🎯  Target message",
		"🛠️  Tool message",
		"🔧  Config message",
		"📊  Stats message",
		"🔍  Search message",
		"📝  Note message",
		"💡  Idea message",
		"🎉  Celebration message",
	}

	for _, line := range emojis {
		byteLen, displayWidth := StringWidthInfo(line)
		t.Logf("Line: %q (len=%d, width=%d)", line, byteLen, displayWidth)

		// Analyze all runes in the line
		chars := AnalyzeString(line)
		t.Logf("  Runes:")
		for _, char := range chars {
			t.Logf("    %U (%s): width=%d, bytes=%d-%d", char.Rune, char.UTF8Bytes, char.DisplayWidth, char.ByteStart, char.ByteEnd)
		}

		// Check the emoji part specifically
		if len(line) > 0 {
			runes := []rune(line)
			if len(runes) > 0 {
				emoji := string(runes[0])
				width := RuneWidth(runes[0])
				t.Logf("  First emoji: %q (U+%04X) width=%d", emoji, runes[0], width)
				if emoji == "🚀" && width != 2 {
					t.Errorf("🚀 should have width 2, got %d", width)
				}
				if emoji == "🛠" && width != 1 {
					t.Errorf("🛠 should have width 1, got %d", width)
				}
			}
		}
		t.Logf("")
	}
}

func TestBoxRaw(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		want  string
	}{
		{
			name:  "empty lines",
			lines: []string{},
			want:  "",
		},
		{
			name:  "single line",
			lines: []string{"Hello"},
			want:  "┌───────┐\n│ Hello │\n└───────┘\n",
		},
		{
			name:  "multiple lines",
			lines: []string{"Line 1", "Longer line here", "Short"},
			want: "┌──────────────────┐\n" +
				"│ Line 1           │\n" +
				"│ Longer line here │\n" +
				"│ Short            │\n" +
				"└──────────────────┘\n",
		},
		{
			name:  "emoji width",
			lines: []string{"🚀 Rocket"},
			want:  "┌───────────┐\n│ 🚀 Rocket │\n└───────────┘\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BoxRaw(tt.lines)
			if result != tt.want {
				t.Errorf("BoxRaw() = %q, want %q", result, tt.want)
			}
		})
	}
}

func TestSubstringWithWidth(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		target int
		want   string
	}{
		{
			name:   "empty string",
			input:  "",
			target: 5,
			want:   "",
		},
		{
			name:   "zero target",
			input:  "hello",
			target: 0,
			want:   "",
		},
		{
			name:   "negative target",
			input:  "hello",
			target: -1,
			want:   "",
		},
		{
			name:   "exact fit",
			input:  "hello",
			target: 5,
			want:   "hello",
		},
		{
			name:   "partial fit",
			input:  "hello world",
			target: 5,
			want:   "hello",
		},
		{
			name:   "emoji partial",
			input:  "🚀hello",
			target: 3,
			want:   "🚀h",
		},
		{
			name:   "target larger than string",
			input:  "hi",
			target: 10,
			want:   "hi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := substringWithWidth(tt.input, tt.target)
			if result != tt.want {
				t.Errorf("substringWithWidth(%q, %d) = %q, want %q", tt.input, tt.target, result, tt.want)
			}
		})
	}
}
