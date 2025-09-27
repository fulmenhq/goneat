package ascii

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// AnalysisResult represents width analysis findings
type AnalysisResult struct {
	WideCharacters    []string // Characters that appear wider than calculated
	NarrowCharacters  []string // Characters that appear narrower than calculated
	WideSequences     []string // Multi-character sequences that appear wider
	NarrowSequences   []string // Multi-character sequences that appear narrower
	Terminal          string   // Target terminal for adjustments
	SequenceDetection bool     // Whether sequence detection was used
}

// StringInfoLine represents a parsed stringinfo output line
type StringInfoLine struct {
	Character           string
	DisplayWidth        int
	Content             string
	IsVariationSelector bool
}

// AnalyzeBoxOutput analyzes box rendering output to detect alignment issues
func AnalyzeBoxOutput(input io.Reader, terminal string, generateMarks, apply bool) error {
	scanner := bufio.NewScanner(input)
	var boxLines []string

	// Read all lines
	for scanner.Scan() {
		boxLines = append(boxLines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading input: %w", err)
	}

	result := analyzeBoxAlignment(boxLines)
	result.Terminal = terminal

	if apply {
		return applyAdjustments(result)
	}

	if generateMarks {
		return generateMarkCommands(result)
	}

	return reportAnalysis(result)
}

// AnalyzeStringInfoOutput analyzes stringinfo output for width discrepancies
func AnalyzeStringInfoOutput(input io.Reader, terminal string, generateMarks, apply bool) error {
	scanner := bufio.NewScanner(input)
	result := &AnalysisResult{Terminal: terminal, SequenceDetection: true}

	// Pattern to match stringinfo lines
	// Line 0123: byte_len=42     display_width=40   content="🎗  Character U+1F397 (width=1, bytes=4)"
	linePattern := regexp.MustCompile(`^Line \d+: byte_len=(\d+)\s+display_width=(\d+)\s+content="([^"]*)"`)

	// Pattern to extract terminal from environment comments
	termPattern := regexp.MustCompile(`^# TERM_PROGRAM=(.+)$`)

	var allLines []StringInfoLine

	for scanner.Scan() {
		line := scanner.Text()

		// Check for terminal environment info
		if result.Terminal == "" {
			if matches := termPattern.FindStringSubmatch(line); len(matches) > 1 {
				result.Terminal = matches[1]
				continue
			}
		}

		// Check for stringinfo data lines
		matches := linePattern.FindStringSubmatch(line)

		if len(matches) == 4 {
			displayWidth, _ := strconv.Atoi(matches[2])
			content := matches[3]

			// Store line info for sequence analysis (skip header lines)
			if len(content) > 0 && !isHeaderLine(content) {
				char := string([]rune(content)[0])
				allLines = append(allLines, StringInfoLine{
					Character:           char,
					DisplayWidth:        displayWidth,
					Content:             content,
					IsVariationSelector: char == "️" || unicode.Is(unicode.Variation_Selector, []rune(char)[0]),
				})
			}
		}
	}

	// Analyze for emoji+variation selector sequences
	result = analyzeEmojiSequences(allLines, result)

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading input: %w", err)
	}

	if apply {
		return applyAdjustments(result)
	}

	if generateMarks {
		return generateMarkCommands(result)
	}

	return reportAnalysis(result)
}

// analyzeBoxAlignment analyzes box output for alignment issues
func analyzeBoxAlignment(boxLines []string) *AnalysisResult {
	result := &AnalysisResult{}

	if len(boxLines) < 3 {
		return result // Not enough lines for a box
	}

	// Find the expected content width by analyzing the box structure
	var contentLines []BoxContentLine

	// Extract content lines and analyze their padding patterns
	for _, line := range boxLines {
		if strings.Contains(line, "│") && strings.Contains(line, "Character U+") {
			// Find content between the first and last │ using rune-aware operations
			// We need to handle UTF-8 properly since │ is a multi-byte character
			runes := []rune(line)
			firstPipe := -1
			lastPipe := -1

			for i, r := range runes {
				if r == '│' {
					if firstPipe == -1 {
						firstPipe = i
					}
					lastPipe = i
				}
			}

			if firstPipe >= 0 && lastPipe > firstPipe {
				// Extract content between borders (skip the space after first pipe too)
				content := string(runes[firstPipe+1 : lastPipe])

				if strings.TrimSpace(content) == "" {
					continue
				}

				// Measure actual content and padding
				trimmedContent := strings.TrimRight(content, " ")
				paddingLength := len(content) - len(trimmedContent)

				// Extract character if this looks like our calibration format
				char := extractCharacterFromContent(trimmedContent)
				if char != "" {
					contentLines = append(contentLines, BoxContentLine{
						Character:     char,
						Content:       trimmedContent,
						PaddingLength: paddingLength,
						TotalWidth:    len(content),
					})
				}
			}
		}
	}

	if len(contentLines) == 0 {
		return result
	}

	// Find the most common padding length (normal alignment)
	paddingStats := make(map[int]int)
	for _, line := range contentLines {
		paddingStats[line.PaddingLength]++
	}

	// Find the most frequent padding length (this is "normal")
	var normalPadding int
	var maxCount int
	for padding, count := range paddingStats {
		if count > maxCount {
			maxCount = count
			normalPadding = padding
		}
	}

	// Analyze deviations from normal padding
	var narrowLines, wideLines []string

	for _, line := range contentLines {
		if line.PaddingLength > normalPadding {
			// Too much padding = character renders narrower than calculated
			// Use the simpler extraction that just gets the emoji at the start
			if char := line.Character; char != "" {
				narrowLines = append(narrowLines, char)
			}
		} else if line.PaddingLength < normalPadding {
			// Too little padding = character renders wider than calculated
			if char := line.Character; char != "" {
				wideLines = append(wideLines, char)
			}
		}
	}

	// Add to results
	if len(narrowLines) > 0 {
		result.NarrowCharacters = append(result.NarrowCharacters, narrowLines...)
	}
	if len(wideLines) > 0 {
		result.WideCharacters = append(result.WideCharacters, wideLines...)
	}

	return result
}

// BoxContentLine represents a parsed box content line
type BoxContentLine struct {
	Character     string
	Content       string
	PaddingLength int
	TotalWidth    int
}

// extractCharacterFromContent extracts the emoji/character from calibration format content
func extractCharacterFromContent(content string) string {
	// For our format: " 🎟️  Character U+1F39F+VS (width=1, bytes=7)"
	// Skip header lines that don't have "Character U+" pattern
	if !strings.Contains(content, "Character U+") {
		return ""
	}

	// The emoji is at the start of the trimmed content, before "Character"
	trimmed := strings.TrimSpace(content)

	// Find where "Character" starts
	charIndex := strings.Index(trimmed, "Character")
	if charIndex <= 0 {
		return "" // No emoji before "Character" or "Character" not found
	}

	// Extract everything before "Character" and trim spaces
	emojiPart := strings.TrimSpace(trimmed[:charIndex])

	// Verify it's not empty and contains Unicode characters
	if emojiPart == "" {
		return ""
	}

	// Verify it's a Unicode character (not a box border character)
	if strings.Contains(emojiPart, "│") || strings.Contains(emojiPart, "┃") {
		return "" // Skip box border artifacts
	}

	// Return the extracted emoji/character sequence
	// This handles both single emoji and emoji+VS sequences correctly
	return emojiPart
}

// extractEmojiFromLine extracts emoji character from a content line using Unicode identifier
func extractEmojiFromLine(content string) string {
	// For our format: "🎟️  Character U+1F39F+VS (width=1, bytes=7)"
	// Extract the Unicode identifier and rebuild the character

	// Pattern to match: "Character U+XXXX+VS" or "Character U+XXXX"
	unicodePattern := regexp.MustCompile(`Character U\+([0-9A-F]+)(\+VS)?`)
	matches := unicodePattern.FindStringSubmatch(content)

	if len(matches) >= 2 {
		// Parse the hex Unicode value
		if codepoint, err := strconv.ParseInt(matches[1], 16, 32); err == nil {
			char := string(rune(codepoint))

			// If it has +VS suffix, add variation selector
			if len(matches) > 2 && matches[2] == "+VS" {
				char += "\uFE0F" // Add variation selector
			}

			return char
		}
	}

	return ""
}

// analyzeEmojiSequences analyzes for emoji+variation selector sequences
func analyzeEmojiSequences(lines []StringInfoLine, result *AnalysisResult) *AnalysisResult {
	for i, line := range lines {
		// Look for variation selector patterns
		if line.IsVariationSelector && i > 0 {
			// This is a variation selector - check the previous line for base emoji
			prevLine := lines[i-1]

			// Combine base emoji + variation selector
			sequence := prevLine.Character + line.Character

			// Calculate expected vs actual width for the sequence
			// Get the calculated widths from the content descriptions
			baseWidth := extractCalculatedWidth(prevLine.Content)
			vsWidth := extractCalculatedWidth(line.Content)
			expectedWidth := baseWidth + vsWidth
			actualSequenceWidth := StringWidth(sequence)

			// If the sequence renders differently than the sum of parts
			if actualSequenceWidth != expectedWidth {
				if actualSequenceWidth > expectedWidth {
					result.WideSequences = append(result.WideSequences, sequence)
				} else {
					result.NarrowSequences = append(result.NarrowSequences, sequence)
				}
			}
		}

		// Check for individual characters that deviate from normal patterns
		// Most emoji lines should have similar display widths (around 53 with normalized descriptions)
		// Characters with significantly different widths might be problematic

		normalWidth := 53 // Baseline for our normalized calibration format
		tolerance := 2    // Allow some variation

		if line.DisplayWidth < normalWidth-tolerance {
			// Line is too short - this specific character might be narrow
			if !contains(result.NarrowCharacters, line.Character) {
				result.NarrowCharacters = append(result.NarrowCharacters, line.Character)
			}
		} else if line.DisplayWidth > normalWidth+tolerance {
			// Line is too long - this specific character might be wide
			if !contains(result.WideCharacters, line.Character) {
				result.WideCharacters = append(result.WideCharacters, line.Character)
			}
		}
	}

	return result
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// isHeaderLine checks if a content line is a header/title line that should be skipped
func isHeaderLine(content string) bool {
	// Skip lines that are obviously headers/titles
	return strings.Contains(content, "Collection") ||
		strings.Contains(content, "Calibration") ||
		strings.Contains(content, "Test") ||
		!strings.Contains(content, "Character U+")
}

// extractCalculatedWidth extracts the calculated width from content description
func extractCalculatedWidth(content string) int {
	// For our calibration format: "🎗  Character U+1F397 (width=1, bytes=4)"
	widthPattern := regexp.MustCompile(`\(width=(\d+),`)
	if matches := widthPattern.FindStringSubmatch(content); len(matches) > 1 {
		if width, err := strconv.Atoi(matches[1]); err == nil {
			return width
		}
	}
	return 0
}

// applyAdjustments directly applies width adjustments to user config
func applyAdjustments(result *AnalysisResult) error {
	totalAdjustments := len(result.WideCharacters) + len(result.NarrowCharacters) +
		len(result.WideSequences) + len(result.NarrowSequences)

	if totalAdjustments == 0 {
		fmt.Println("✅ No width adjustments needed")
		return nil
	}

	fmt.Printf("🔧 Applying adjustments for %s...\n", result.Terminal)

	// Apply sequence adjustments (higher priority)
	if len(result.WideSequences) > 0 {
		if err := MarkCharactersWideWithOverride(result.WideSequences, "", result.Terminal); err != nil {
			return fmt.Errorf("failed to apply wide sequence adjustments: %w", err)
		}
		fmt.Printf("✅ Marked %d sequences as wide\n", len(result.WideSequences))
	}

	if len(result.NarrowSequences) > 0 {
		if err := MarkCharactersNarrowWithOverride(result.NarrowSequences, "", result.Terminal); err != nil {
			return fmt.Errorf("failed to apply narrow sequence adjustments: %w", err)
		}
		fmt.Printf("✅ Marked %d sequences as narrow\n", len(result.NarrowSequences))
	}

	// Apply individual character adjustments
	if len(result.WideCharacters) > 0 {
		if err := MarkCharactersWideWithOverride(result.WideCharacters, "", result.Terminal); err != nil {
			return fmt.Errorf("failed to apply wide character adjustments: %w", err)
		}
		fmt.Printf("✅ Marked %d characters as wide\n", len(result.WideCharacters))
	}

	if len(result.NarrowCharacters) > 0 {
		// NarrowCharacters are those that render wider than calculated, so mark them as wide
		if err := MarkCharactersWideWithOverride(result.NarrowCharacters, "", result.Terminal); err != nil {
			return fmt.Errorf("failed to apply character width adjustments: %w", err)
		}
		fmt.Printf("✅ Marked %d characters as wider (they render wider than calculated)\n", len(result.NarrowCharacters))
	}

	fmt.Println("🔄 Restart goneat or reload configuration to apply changes.")

	return nil
}

// generateMarkCommands generates goneat ascii mark commands
func generateMarkCommands(result *AnalysisResult) error {
	terminalFlag := ""
	if result.Terminal != "" {
		terminalFlag = fmt.Sprintf(" --term-program=%s", result.Terminal)
	}

	// Generate sequence commands first
	if len(result.WideSequences) > 0 {
		chars := strings.Join(result.WideSequences, `" "`)
		fmt.Printf("# Sequences that appear too wide:\n")
		fmt.Printf("goneat ascii mark%s --wide \"%s\"\n\n", terminalFlag, chars)
	}

	if len(result.NarrowSequences) > 0 {
		chars := strings.Join(result.NarrowSequences, `" "`)
		fmt.Printf("# Sequences that appear too narrow:\n")
		fmt.Printf("goneat ascii mark%s --narrow \"%s\"\n\n", terminalFlag, chars)
	}

	// Generate individual character commands
	if len(result.WideCharacters) > 0 {
		chars := strings.Join(result.WideCharacters, `" "`)
		fmt.Printf("# Characters that appear too wide:\n")
		fmt.Printf("goneat ascii mark%s --wide \"%s\"\n\n", terminalFlag, chars)
	}

	if len(result.NarrowCharacters) > 0 {
		chars := strings.Join(result.NarrowCharacters, `" "`)
		fmt.Printf("# Characters that render wider than calculated:\n")
		fmt.Printf("goneat ascii mark%s --wide \"%s\"\n\n", terminalFlag, chars)
	}

	totalItems := len(result.WideCharacters) + len(result.NarrowCharacters) +
		len(result.WideSequences) + len(result.NarrowSequences)
	if totalItems == 0 {
		fmt.Println("# No width adjustments detected")
	}

	return nil
}

// reportAnalysis reports analysis findings in human-readable format
func reportAnalysis(result *AnalysisResult) error {
	fmt.Printf("🔍 Terminal Width Analysis")
	if result.Terminal != "" {
		fmt.Printf(" (%s)", result.Terminal)
	}
	fmt.Println()
	fmt.Println(strings.Repeat("=", 50))

	if len(result.WideCharacters) > 0 {
		fmt.Printf("\n📏 Characters appearing TOO WIDE (%d):\n", len(result.WideCharacters))
		for _, char := range result.WideCharacters {
			fmt.Printf("  %s\n", char)
		}
	}

	if len(result.NarrowCharacters) > 0 {
		fmt.Printf("\n📐 Characters appearing TOO NARROW (%d):\n", len(result.NarrowCharacters))
		for _, char := range result.NarrowCharacters {
			fmt.Printf("  %s\n", char)
		}
	}

	if len(result.WideCharacters) == 0 && len(result.NarrowCharacters) == 0 {
		fmt.Println("\n✅ No alignment issues detected")
	}

	fmt.Println("\n💡 Recommendations:")
	fmt.Println("1. Run with --generate-marks to get exact commands")
	fmt.Println("2. Use goneat ascii mark --wide/--narrow to apply adjustments")
	fmt.Println("3. Test with goneat ascii box after adjustments")

	return nil
}
