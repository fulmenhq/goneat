/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package finalizer

import (
	"bytes"
	"strings"
	"unicode/utf8"
)

// NormalizeEOF normalizes the end-of-file formatting of the given content
func NormalizeEOF(input []byte, ensure bool, collapse bool, trimTrailingSpaces bool, lineEnding string, preserveMarkdownHardBreaks bool) (out []byte, changed bool, err error) {
	if len(input) == 0 {
		return input, false, nil
	}

	// Check if content is processable text
	if !IsProcessableText(input) {
		return input, false, nil
	}

	// Convert to string for easier processing
	content := string(input)
	originalContent := content
	lines := strings.Split(content, "\n")

	// Determine the line ending style used in the file
	if lineEnding == "" {
		lineEnding = detectLineEnding(content)
	}

	// Process each line
	for i, line := range lines {
		// Trim trailing whitespace if requested
		if trimTrailingSpaces {
			if preserveMarkdownHardBreaks {
				// Count trailing spaces
				n := 0
				for j := len(line) - 1; j >= 0; j-- {
					if line[j] == ' ' {
						n++
						continue
					}
					break
				}
				if n >= 2 {
					// Collapse to exactly two spaces
					trimmed := strings.TrimRight(line, " \t") + "  "
					if trimmed != line {
						lines[i] = trimmed
						changed = true
					}
					continue
				}
			}
			trimmed := strings.TrimRight(line, " \t")
			if trimmed != line {
				lines[i] = trimmed
				changed = true
			}
		}
	}

	// Rejoin lines with detected line ending
	content = strings.Join(lines, lineEnding)

	// Handle EOF newline normalization
	if ensure {
		// Ensure the file ends with exactly one newline
		content = ensureSingleTrailingNewline(content, lineEnding)
	} else if collapse {
		// Just collapse multiple trailing newlines to one (if any)
		content = collapseTrailingNewlines(content, lineEnding)
	}

	// Check if anything changed
	if content != originalContent {
		changed = true
	}

	return []byte(content), changed, nil
}

// NormalizeWhitespace removes trailing whitespace from all lines and normalizes EOF
func NormalizeWhitespace(input []byte, ensureEOF bool, lineEnding string, preserveMarkdownHardBreaks bool) (out []byte, changed bool, err error) {
	return NormalizeEOF(input, ensureEOF, true, true, lineEnding, preserveMarkdownHardBreaks)
}

// WhitespaceIssue represents a detected whitespace issue with location information
type WhitespaceIssue struct {
	Type        string // "trailing-whitespace" or "eof"
	Description string
	LineNumbers []int // Affected line numbers (1-based)
}

// DetectWhitespaceIssues detects trailing whitespace issues without modifying content
// This is the shared function used by both assessment and format commands for consistency
func DetectWhitespaceIssues(input []byte, options NormalizationOptions) (hasIssues bool, issues []WhitespaceIssue) {
	if len(input) == 0 {
		return false, nil
	}

	// Check if content is processable text
	if !IsProcessableText(input) {
		return false, nil
	}

	content := string(input)
	lines := strings.Split(content, "\n")
	var foundIssues []WhitespaceIssue

	// Check for trailing whitespace
	if options.TrimTrailingWhitespace {
		var affectedLines []int
		for i, line := range lines {
			originalLine := line
			trimmedLine := strings.TrimRight(line, " \t")

			// If preserving markdown hard breaks, allow exactly 2 trailing spaces
			if options.PreserveMarkdownHardBreaks {
				// Count trailing spaces
				n := 0
				for j := len(line) - 1; j >= 0; j-- {
					if line[j] == ' ' {
						n++
						continue
					}
					break
				}
				// If exactly 2 trailing spaces, don't consider it an issue
				if n == 2 {
					continue
				}
			}

			if trimmedLine != originalLine {
				affectedLines = append(affectedLines, i+1) // 1-based line numbers
			}
		}
		if len(affectedLines) > 0 {
			foundIssues = append(foundIssues, WhitespaceIssue{
				Type:        "trailing-whitespace",
				Description: "Trailing whitespace present on one or more lines",
				LineNumbers: affectedLines,
			})
		}
	}

	// Check for EOF issues
	if options.EnsureEOF {
		lineCount := len(lines)
		contentBytes := []byte(content)

		// Check if file ends with exactly one newline
		if len(contentBytes) > 0 && contentBytes[len(contentBytes)-1] != '\n' {
			foundIssues = append(foundIssues, WhitespaceIssue{
				Type:        "eof",
				Description: "Missing trailing newline at EOF",
				LineNumbers: []int{lineCount}, // Last line
			})
		} else if len(contentBytes) > 1 && contentBytes[len(contentBytes)-1] == '\n' && contentBytes[len(contentBytes)-2] == '\n' {
			foundIssues = append(foundIssues, WhitespaceIssue{
				Type:        "eof",
				Description: "Multiple trailing newlines at EOF",
				LineNumbers: []int{lineCount + 1}, // After last content line
			})
		}
	}

	return len(foundIssues) > 0, foundIssues
}

// NormalizeLineEndings converts all line endings to the specified style
func NormalizeLineEndings(input []byte, targetEnding string) (out []byte, changed bool, err error) {
	if len(input) == 0 {
		return input, false, nil
	}

	// Check for binary content
	if bytes.Contains(input, []byte{0}) {
		return input, false, nil
	}

	content := string(input)
	originalContent := content

	// Normalize line endings
	content = strings.ReplaceAll(content, "\r\n", "\n") // CRLF -> LF
	content = strings.ReplaceAll(content, "\r", "\n")   // CR -> LF

	// Convert to target ending if specified
	if targetEnding == "\r\n" {
		content = strings.ReplaceAll(content, "\n", "\r\n")
	}

	if content != originalContent {
		changed = true
	}

	return []byte(content), changed, nil
}

// RemoveUTF8BOM removes UTF-8 Byte Order Mark if present
func RemoveUTF8BOM(input []byte) (out []byte, changed bool, err error) {
	if len(input) >= 3 && bytes.HasPrefix(input, []byte{0xEF, 0xBB, 0xBF}) {
		return input[3:], true, nil
	}
	return input, false, nil
}

// RemoveBOM removes Byte Order Mark of any supported encoding if present
func RemoveBOM(input []byte) (out []byte, changed bool, err error) {
	if len(input) == 0 {
		return input, false, nil
	}

	// Check for UTF-32BE BOM (4 bytes)
	if len(input) >= 4 && bytes.HasPrefix(input, []byte{0x00, 0x00, 0xFE, 0xFF}) {
		return input[4:], true, nil
	}

	// Check for UTF-32LE BOM (4 bytes)
	if len(input) >= 4 && bytes.HasPrefix(input, []byte{0xFF, 0xFE, 0x00, 0x00}) {
		return input[4:], true, nil
	}

	// Check for UTF-8 BOM (3 bytes)
	if len(input) >= 3 && bytes.HasPrefix(input, []byte{0xEF, 0xBB, 0xBF}) {
		return input[3:], true, nil
	}

	// Check for UTF-16BE BOM (2 bytes)
	if len(input) >= 2 && bytes.HasPrefix(input, []byte{0xFE, 0xFF}) {
		return input[2:], true, nil
	}

	// Check for UTF-16LE BOM (2 bytes)
	if len(input) >= 2 && bytes.HasPrefix(input, []byte{0xFF, 0xFE}) {
		return input[2:], true, nil
	}

	return input, false, nil
}

// GetBOMInfo returns information about detected BOM
func GetBOMInfo(input []byte) (encoding string, bomSize int, found bool) {
	if len(input) == 0 {
		return "", 0, false
	}

	// Check for UTF-32BE BOM
	if len(input) >= 4 && bytes.HasPrefix(input, []byte{0x00, 0x00, 0xFE, 0xFF}) {
		return "UTF-32BE", 4, true
	}

	// Check for UTF-32LE BOM
	if len(input) >= 4 && bytes.HasPrefix(input, []byte{0xFF, 0xFE, 0x00, 0x00}) {
		return "UTF-32LE", 4, true
	}

	// Check for UTF-8 BOM
	if len(input) >= 3 && bytes.HasPrefix(input, []byte{0xEF, 0xBB, 0xBF}) {
		return "UTF-8", 3, true
	}

	// Check for UTF-16BE BOM
	if len(input) >= 2 && bytes.HasPrefix(input, []byte{0xFE, 0xFF}) {
		return "UTF-16BE", 2, true
	}

	// Check for UTF-16LE BOM
	if len(input) >= 2 && bytes.HasPrefix(input, []byte{0xFF, 0xFE}) {
		return "UTF-16LE", 2, true
	}

	return "", 0, false
}

// ComprehensiveFileNormalization applies all normalization operations
func ComprehensiveFileNormalization(input []byte, options NormalizationOptions) (out []byte, changed bool, err error) {
	// Enforce encoding policy
	switch strings.ToLower(strings.TrimSpace(options.EncodingPolicy)) {
	case "", "utf8-only":
		// For utf8-only, reject any content with non-UTF8 BOMs
		if HasBOM(input) {
			enc, _, found := GetBOMInfo(input)
			if found && enc != "UTF-8" {
				return input, false, nil
			}
		}
		// Also reject if content without BOM is not valid UTF-8
		contentNoBOM := RemoveBOMSafe(input)
		if !utf8.Valid(contentNoBOM) {
			return input, false, nil
		}
	case "utf8-or-bom":
		if HasBOM(input) {
			enc, _, found := GetBOMInfo(input)
			if !found || enc != "UTF-8" {
				return input, false, nil
			}
		} else if !utf8.Valid(input) {
			return input, false, nil
		}
	case "any-text":
		if !IsTextFile(input) {
			return input, false, nil
		}
	default:
		contentNoBOM := RemoveBOMSafe(input)
		if !utf8.Valid(contentNoBOM) {
			return input, false, nil
		}
	}

	content := input
	totalChanged := false

	// Remove BOM if requested (supports UTF-8, UTF-16, UTF-32)
	if options.RemoveUTF8BOM {
		if result, hasChanged, err := RemoveBOM(content); err != nil {
			return nil, false, err
		} else if hasChanged {
			content = result
			totalChanged = true
		}
	}

	// Normalize line endings if requested
	if options.NormalizeLineEndings != "" {
		if result, hasChanged, err := NormalizeLineEndings(content, options.NormalizeLineEndings); err != nil {
			return nil, false, err
		} else if hasChanged {
			content = result
			totalChanged = true
		}
	}

	// Apply EOF and whitespace normalization
	if result, hasChanged, err := NormalizeEOF(content, options.EnsureEOF, true, options.TrimTrailingWhitespace, "", options.PreserveMarkdownHardBreaks); err != nil {
		return nil, false, err
	} else if hasChanged {
		content = result
		totalChanged = true
	}

	return content, totalChanged, nil
}

// NormalizationOptions configures file normalization behavior
type NormalizationOptions struct {
	EnsureEOF                  bool   // Ensure file ends with exactly one newline
	TrimTrailingWhitespace     bool   // Remove trailing spaces/tabs from all lines
	NormalizeLineEndings       string // Target line ending style ("", "\n", or "\r\n")
	RemoveUTF8BOM              bool   // Remove Byte Order Mark (UTF-8 only recommended)
	PreserveMarkdownHardBreaks bool   // Preserve exactly two trailing spaces in Markdown
	EncodingPolicy             string // "utf8-only" (default), "utf8-or-bom", "any-text"
}

// detectLineEnding detects the primary line ending style used in the content
func detectLineEnding(content string) string {
	// Count LF and CRLF occurrences
	lfCount := strings.Count(content, "\n") - strings.Count(content, "\r\n")
	crlfCount := strings.Count(content, "\r\n")

	// Use the more common line ending, default to LF
	if crlfCount > lfCount {
		return "\r\n"
	}
	return "\n"
}

// ensureSingleTrailingNewline ensures the content ends with exactly one newline
func ensureSingleTrailingNewline(content, lineEnding string) string {
	// First, collapse any existing trailing newlines
	content = collapseTrailingNewlines(content, lineEnding)

	// Then ensure it ends with exactly one
	if !strings.HasSuffix(content, lineEnding) {
		content += lineEnding
	}

	return content
}

// collapseTrailingNewlines collapses multiple trailing newlines to a single one
func collapseTrailingNewlines(content, lineEnding string) string {
	// Remove all trailing whitespace including newlines
	content = strings.TrimRight(content, " \t\r\n")

	// Add back a single newline if the original had any trailing newlines
	// We determine this by checking if the original content had trailing newlines
	originalLen := len(content)
	trimmed := strings.TrimRight(content, "\r\n")
	if originalLen > len(trimmed) {
		content = trimmed + lineEnding
	}

	return content
}

// IsTextFile performs a heuristic check to determine if content is likely text
func IsTextFile(content []byte) bool {
	if len(content) == 0 {
		return true
	}

	// Check for NUL bytes (binary file indicator)
	if bytes.Contains(content, []byte{0}) {
		return false
	}

	// Check if content is valid UTF-8
	return utf8.Valid(content)
}

// IsProcessableText performs a more sophisticated check for text that can be processed
// This allows UTF-16/UTF-32 files with BOMs to be processed
func IsProcessableText(content []byte) bool {
	if len(content) == 0 {
		return true
	}

	// Allow files with BOMs (UTF-8, UTF-16, UTF-32)
	if HasBOM(content) {
		// Remove BOM and check if the rest is processable
		_, _, found := GetBOMInfo(content)
		if found {
			// For UTF-16/UTF-32, we can still process if it's text-like
			// For now, allow UTF-16/UTF-32 with BOMs
			return true
		}
	}

	// Check for excessive NUL bytes (likely binary)
	nulCount := bytes.Count(content, []byte{0})
	if nulCount > len(content)/10 { // More than 10% NUL bytes
		return false
	}

	// Check if content is valid UTF-8 (after potential BOM removal)
	contentWithoutBOM := RemoveBOMSafe(content)
	return utf8.Valid(contentWithoutBOM)
}

// HasBOM checks if the content starts with a known BOM
func HasBOM(content []byte) bool {
	_, _, found := GetBOMInfo(content)
	return found
}

// RemoveBOMSafe removes BOM if present, returns original content if not
func RemoveBOMSafe(content []byte) []byte {
	result, _, _ := RemoveBOM(content)
	return result
}

// GetSupportedExtensions returns the list of file extensions supported by the finalizer
func GetSupportedExtensions() []string {
	return []string{
		".go",
		".yaml", ".yml",
		".json",
		".md", ".markdown",
		".txt",
		".sh",
		".py",
		".js", ".jsx", ".ts", ".tsx",
		".html", ".htm",
		".css",
		".xml",
		".toml",
		".ini",
		".cfg",
		".conf",
	}
}

// IsSupportedExtension checks if the given file extension is supported by the finalizer
func IsSupportedExtension(ext string) bool {
	ext = strings.ToLower(ext)
	supported := GetSupportedExtensions()
	for _, supportedExt := range supported {
		if ext == supportedExt {
			return true
		}
	}
	return false
}
