// Package ascii provides utilities for creating ASCII art and formatted text output
package ascii

import (
	"fmt"
	"strings"
)

// DrawBox draws a properly aligned ASCII box around the given lines
func DrawBox(lines []string) {
	if len(lines) == 0 {
		return
	}

	// Trim trailing spaces and find the maximum line length
	maxLen := 0
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
		if len(lines[i]) > maxLen {
			maxLen = len(lines[i])
		}
	}

	// Add padding (2 spaces on each side, plus 2 for margins)
	contentWidth := maxLen + 4
	border := strings.Repeat("─", contentWidth)

	// Top border
	fmt.Printf("┌%s┐\n", border)

	// Content lines
	for _, line := range lines {
		padding := contentWidth - len(line)
		fmt.Printf("│ %s%s │\n", line, strings.Repeat(" ", padding))
	}

	// Bottom border
	fmt.Printf("└%s┘\n", border)
}

// TruncateForBox truncates a string to fit within a box of the given width,
// adding "..." if truncated
func TruncateForBox(value string, width int) string {
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	if width <= 3 {
		return string(runes[:width])
	}
	return string(runes[:width-3]) + "..."
}
