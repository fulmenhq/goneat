package ascii

import (
	"os"
	"strings"
)

func init() {
	// Initialize go-runewidth based on terminal environment
	initRuneWidth()
}

// initRuneWidth configures go-runewidth for optimal terminal compatibility
func initRuneWidth() {
	// Check if we're in Ghostty terminal
	term := os.Getenv("TERM")
	termProgram := os.Getenv("TERM_PROGRAM")

	if strings.Contains(term, "ghostty") || termProgram == "ghostty" {
		// Ghostty renders some emojis with variation selectors as width 2
		// even though Unicode/go-runewidth calculates them as width 1.
		// We need a custom wrapper to handle these cases.
		ghosttyMode = true
		return
	}

	// For other terminals, the library auto-detects based on environment
	// No explicit initialization needed - go-runewidth handles it automatically
}
