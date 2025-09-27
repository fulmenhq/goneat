// Package ascii provides utilities for creating ASCII art and formatted text output
package ascii

import (
	"fmt"
	"strings"

	"github.com/mattn/go-runewidth"
)

// ghosttyMode indicates we need special handling for Ghostty terminal
var ghosttyMode bool

// Box builds a box containing the provided lines and returns it as a string.
// Lines are left-aligned with single-space padding on each side. Multi-width
// runes (emoji, CJK, etc.) are accounted for so the borders stay aligned.
func Box(lines []string) string {
	if len(lines) == 0 {
		return ""
	}

	trimmed := make([]string, len(lines))
	maxWidth := 0
	for i, line := range lines {
		trimmed[i] = strings.TrimRight(line, " ")
		if w := StringWidth(trimmed[i]); w > maxWidth {
			maxWidth = w
		}
	}

	leftPadding, rightPadding := 1, 1
	innerWidth := maxWidth + leftPadding + rightPadding
	border := strings.Repeat("─", innerWidth)

	var sb strings.Builder
	sb.WriteString("┌" + border + "┐\n")
	for _, line := range trimmed {
		lineWidth := StringWidth(line)
		fill := innerWidth - leftPadding - rightPadding - lineWidth
		if fill < 0 {
			fill = 0
		}
		sb.WriteString("│ " + line + strings.Repeat(" ", fill) + " │\n")
	}
	sb.WriteString("└" + border + "┘\n")
	return sb.String()
}

// BoxRaw builds a box using only go-runewidth without terminal-specific overrides.
// This is useful for debugging and analyzing terminal width issues.
func BoxRaw(lines []string) string {
	if len(lines) == 0 {
		return ""
	}

	trimmed := make([]string, len(lines))
	maxWidth := 0
	for i, line := range lines {
		trimmed[i] = strings.TrimRight(line, " ")
		// Use runewidth directly without terminal overrides
		if w := runewidth.StringWidth(trimmed[i]); w > maxWidth {
			maxWidth = w
		}
	}

	leftPadding, rightPadding := 1, 1
	innerWidth := maxWidth + leftPadding + rightPadding
	border := strings.Repeat("─", innerWidth)

	var sb strings.Builder
	sb.WriteString("┌" + border + "┐\n")
	for _, line := range trimmed {
		// Use runewidth directly without terminal overrides
		lineWidth := runewidth.StringWidth(line)
		fill := innerWidth - leftPadding - rightPadding - lineWidth
		if fill < 0 {
			fill = 0
		}
		sb.WriteString("│ " + line + strings.Repeat(" ", fill) + " │\n")
	}
	sb.WriteString("└" + border + "┘\n")
	return sb.String()
}

// DrawBox prints a box containing the provided lines.
func DrawBox(lines []string) {
	if len(lines) == 0 {
		return
	}
	fmt.Print(Box(lines))
}

// TruncateForBox truncates a string so that its display width fits within the
// provided width. An ellipsis ("...") is appended when truncation occurs and
// there is space for it.
func TruncateForBox(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if StringWidth(value) <= width {
		return value
	}
	if width <= 3 {
		return substringWithWidth(value, width)
	}
	truncated := substringWithWidth(value, width-3)
	return truncated + "..."
}

func substringWithWidth(s string, target int) string {
	if target <= 0 {
		return ""
	}
	width := 0
	var sb strings.Builder
	for _, r := range s {
		w := RuneWidth(r)
		if width+w > target {
			break
		}
		width += w
		sb.WriteRune(r)
	}
	return sb.String()
}

// RuneWidth returns the display width of a single rune, accounting for multi-width
// Unicode characters (emoji, CJK, etc.).
func RuneWidth(r rune) int {
	// For single runes, we use go-runewidth directly
	// The Ghostty adjustments are handled in StringWidth for complete sequences
	return runewidth.RuneWidth(r)
}

// StringWidth returns the display width of a string, accounting for multi-width
// Unicode characters (emoji, CJK, etc.) and terminal-specific overrides.
func StringWidth(s string) int {
	// Use the terminal catalog system for width calculation
	return GetTerminalWidth(s)
}

// StringWidthInfo returns both the byte length and display width of a string.
// This is useful for debugging Unicode width calculations.
func StringWidthInfo(s string) (byteLen int, displayWidth int) {
	return len(s), StringWidth(s)
}

// CharInfo represents information about a single character in a string.
type CharInfo struct {
	Rune         rune   // The Unicode rune
	ByteStart    int    // Starting byte position in the string
	ByteEnd      int    // Ending byte position in the string
	DisplayWidth int    // Display width of this character
	UTF8Bytes    string // The UTF-8 byte representation
}

// AnalyzeString provides a detailed breakdown of each character in a string,
// including their byte positions, display widths, and UTF-8 representations.
// This is useful for debugging complex Unicode width calculation issues.
func AnalyzeString(s string) []CharInfo {
	var result []CharInfo
	bytePos := 0

	for _, r := range s {
		width := runewidth.RuneWidth(r)
		charBytes := string(r)
		byteLen := len(charBytes)

		result = append(result, CharInfo{
			Rune:         r,
			ByteStart:    bytePos,
			ByteEnd:      bytePos + byteLen,
			DisplayWidth: width,
			UTF8Bytes:    charBytes,
		})

		bytePos += byteLen
	}

	return result
}
