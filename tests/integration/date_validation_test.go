package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// TestDateValidation ensures all dates in documentation and changelog files are current or past dates
func TestDateValidation(t *testing.T) {
	// Get current date
	now := time.Now()
	currentYear := now.Year()
	currentMonth := int(now.Month())
	currentDay := now.Day()

	// Define patterns to check for dates
	datePatterns := []string{
		`(\d{4})-(\d{2})-(\d{2})`,   // YYYY-MM-DD format
		`(\d{4})/(\d{2})/(\d{2})`,   // YYYY/MM/DD format
		`(\d{4})\.(\d{2})\.(\d{2})`, // YYYY.MM.DD format
	}

	// Files to check
	filesToCheck := []string{
		"CHANGELOG.md",
		"RELEASE_NOTES.md",
		"docs/releases/",
		"docs/ops/compliance/",
		"docs/sop/",
	}

	var errors []string

	for _, filePattern := range filesToCheck {
		err := checkFilesForDateErrors(filePattern, datePatterns, currentYear, currentMonth, currentDay)
		if err != nil {
			errors = append(errors, err.Error())
		}
	}

	if len(errors) > 0 {
		t.Errorf("Date validation failed:\n%s", strings.Join(errors, "\n"))
	}
}

func checkFilesForDateErrors(filePattern string, datePatterns []string, currentYear, currentMonth, currentDay int) error {
	var errors []string

	// Walk through files
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-matching files
		if info.IsDir() {
			return nil
		}

		// Check if file matches our pattern
		matched, err := filepath.Match(filePattern, path)
		if err != nil {
			return err
		}
		if !matched && !strings.Contains(path, filePattern) {
			return nil
		}

		// Skip binary files and non-text files
		if !isTextFile(path) {
			return nil
		}

		// Read file content
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		// Check for date patterns
		for _, pattern := range datePatterns {
			re := regexp.MustCompile(pattern)
			matches := re.FindAllStringSubmatch(string(content), -1)

			for _, match := range matches {
				if len(match) >= 4 {
					year := parseInt(match[1])
					month := parseInt(match[2])
					day := parseInt(match[3])

					// Check if date is in the future
					if isFutureDate(year, month, day, currentYear, currentMonth, currentDay) {
						errors = append(errors,
							filepath.Join(path)+": Found future date "+match[0]+" (line may contain: "+getLineContaining(string(content), match[0])+")")
					}
				}
			}
		}

		return nil
	})

	if err != nil {
		return err
	}

	if len(errors) > 0 {
		return fmt.Errorf("Date validation errors:\n%s", strings.Join(errors, "\n"))
	}

	return nil
}

func isTextFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	textExts := []string{".md", ".txt", ".go", ".yaml", ".yml", ".json", ".toml", ".ini", ".cfg", ".conf"}

	for _, textExt := range textExts {
		if ext == textExt {
			return true
		}
	}

	// Check if it's a file without extension that might be text
	if ext == "" {
		return true
	}

	return false
}

func parseInt(s string) int {
	// Simple integer parsing for our use case
	result := 0
	for _, char := range s {
		if char >= '0' && char <= '9' {
			result = result*10 + int(char-'0')
		}
	}
	return result
}

func isFutureDate(year, month, day, currentYear, currentMonth, currentDay int) bool {
	// Check if date is in the future
	if year > currentYear {
		return true
	}
	if year == currentYear && month > currentMonth {
		return true
	}
	if year == currentYear && month == currentMonth && day > currentDay {
		return true
	}
	return false
}

func getLineContaining(content, search string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.Contains(line, search) {
			// Truncate long lines
			if len(line) > 100 {
				return line[:100] + "..."
			}
			return line
		}
	}
	return ""
}
