/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package exitcode

import (
	"testing"
)

func TestExitCodeConstants(t *testing.T) {
	// Test that all constants have expected values
	if Success != 0 {
		t.Errorf("Success = %v, expected 0", Success)
	}
	if GeneralError != 1 {
		t.Errorf("GeneralError = %v, expected 1", GeneralError)
	}
	if ConfigError != 2 {
		t.Errorf("ConfigError = %v, expected 2", ConfigError)
	}
	if ValidationError != 3 {
		t.Errorf("ValidationError = %v, expected 3", ValidationError)
	}
	if FileSystemError != 4 {
		t.Errorf("FileSystemError = %v, expected 4", FileSystemError)
	}
	if NetworkError != 5 {
		t.Errorf("NetworkError = %v, expected 5", NetworkError)
	}
	if PermissionError != 6 {
		t.Errorf("PermissionError = %v, expected 6", PermissionError)
	}
	if TimeoutError != 7 {
		t.Errorf("TimeoutError = %v, expected 7", TimeoutError)
	}
	if UnsupportedFormat != 8 {
		t.Errorf("UnsupportedFormat = %v, expected 8", UnsupportedFormat)
	}
	if ToolNotFound != 9 {
		t.Errorf("ToolNotFound = %v, expected 9", ToolNotFound)
	}
}

func TestString(t *testing.T) {
	tests := []struct {
		code     int
		expected string
	}{
		{Success, "Success"},
		{GeneralError, "General error"},
		{ConfigError, "Configuration error"},
		{ValidationError, "Validation error"},
		{FileSystemError, "File system error"},
		{NetworkError, "Network error"},
		{PermissionError, "Permission error"},
		{TimeoutError, "Timeout error"},
		{UnsupportedFormat, "Unsupported format"},
		{ToolNotFound, "Tool not found"},
		{999, "Unknown error"}, // Test unknown code
	}

	for _, test := range tests {
		result := String(test.code)
		if result != test.expected {
			t.Errorf("String(%d) = %v, expected %v", test.code, result, test.expected)
		}
	}
}

func TestStringAllConstants(t *testing.T) {
	// Test that all defined constants return non-empty strings
	constants := []int{
		Success,
		GeneralError,
		ConfigError,
		ValidationError,
		FileSystemError,
		NetworkError,
		PermissionError,
		TimeoutError,
		UnsupportedFormat,
		ToolNotFound,
	}

	for _, code := range constants {
		result := String(code)
		if result == "" {
			t.Errorf("String(%d) returned empty string", code)
		}
		if result == "Unknown error" {
			t.Errorf("String(%d) returned 'Unknown error' for defined constant", code)
		}
	}
}

func TestStringUnknownCodes(t *testing.T) {
	// Test various unknown codes
	unknownCodes := []int{-1, 10, 100, 9999}

	for _, code := range unknownCodes {
		result := String(code)
		if result != "Unknown error" {
			t.Errorf("String(%d) = %v, expected 'Unknown error'", code, result)
		}
	}
}

func TestExitCodeUniqueness(t *testing.T) {
	// Test that all exit codes are unique
	codes := []int{
		Success,
		GeneralError,
		ConfigError,
		ValidationError,
		FileSystemError,
		NetworkError,
		PermissionError,
		TimeoutError,
		UnsupportedFormat,
		ToolNotFound,
	}

	seen := make(map[int]bool)
	for _, code := range codes {
		if seen[code] {
			t.Errorf("Exit code %d is not unique", code)
		}
		seen[code] = true
	}
}
