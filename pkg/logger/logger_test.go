/*
Copyright © 2025 3 Leaps <info@3leaps.net>
*/
package logger

import (
	"bytes"
	"encoding/json"
	"log"
	"strings"
	"testing"
	"time"
)

func TestLevelString(t *testing.T) {
	tests := []struct {
		level    Level
		expected string
	}{
		{TraceLevel, "TRACE"},
		{DebugLevel, "DEBUG"},
		{InfoLevel, "INFO"},
		{WarnLevel, "WARN"},
		{ErrorLevel, "ERROR"},
		{Level(999), "UNKNOWN"}, // Invalid level
	}

	for _, test := range tests {
		if result := test.level.String(); result != test.expected {
			t.Errorf("Level.String() = %v, expected %v", result, test.expected)
		}
	}
}

func TestLoggerInitialization(t *testing.T) {
	// Test that Initialize creates a default logger
	config := Config{
		Level:     InfoLevel,
		UseColor:  false,
		JSON:      false,
		Component: "test",
		NoOp:      false,
	}

	err := Initialize(config)
	if err != nil {
		t.Fatalf("Initialize() failed: %v", err)
	}

	if defaultLogger == nil {
		t.Fatal("Initialize() did not set defaultLogger")
	}

	if defaultLogger.config.Component != "test" {
		t.Errorf("Initialize() did not set config correctly, got component: %s", defaultLogger.config.Component)
	}
}

func TestLoggerPrettyFormatting(t *testing.T) {
	var buf bytes.Buffer
	logger := &Logger{
		config: Config{
			Level:     InfoLevel,
			UseColor:  false,
			JSON:      false,
			Component: "test",
			NoOp:      false,
		},
		logger: log.New(&buf, "", 0),
	}

	// Create a test entry
	entry := LogEntry{
		Time:      time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
		Level:     "INFO",
		Message:   "test message",
		Component: "test",
		Fields:    map[string]interface{}{"key": "value"},
	}

	result := logger.formatPretty(entry)

	// Check that the result contains expected elements
	expectedParts := []string{
		"2025-01-01 12:00:00",
		"[INFO]",
		"test:",
		"test message",
		"{key=value}",
	}

	for _, part := range expectedParts {
		if !strings.Contains(result, part) {
			t.Errorf("formatPretty() result missing expected part: %s\nResult: %s", part, result)
		}
	}
}

func TestLoggerJSONFormatting(t *testing.T) {
	var buf bytes.Buffer
	logger := &Logger{
		config: Config{
			Level:     InfoLevel,
			UseColor:  false,
			JSON:      true,
			Component: "test",
			NoOp:      false,
		},
		logger: log.New(&buf, "", 0),
	}

	logger.Log(InfoLevel, "test message", String("key", "value"))

	// Check that output is valid JSON
	output := buf.String()
	if !strings.HasPrefix(strings.TrimSpace(output), "{") {
		t.Errorf("Log() with JSON config did not produce JSON output: %s", output)
	}

	// Parse the JSON to verify structure
	var parsed LogEntry
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &parsed); err != nil {
		t.Errorf("Log() produced invalid JSON: %v\nOutput: %s", err, output)
	}

	if parsed.Message != "test message" {
		t.Errorf("Parsed JSON message = %v, expected 'test message'", parsed.Message)
	}

	if parsed.Level != "INFO" {
		t.Errorf("Parsed JSON level = %v, expected 'INFO'", parsed.Level)
	}
}

func TestLoggerLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	logger := &Logger{
		config: Config{
			Level:     WarnLevel, // Only WARN and above
			UseColor:  false,
			JSON:      false,
			Component: "test",
			NoOp:      false,
		},
		logger: log.New(&buf, "", 0),
	}

	// These should not appear in output
	logger.Log(InfoLevel, "info message")
	logger.Log(DebugLevel, "debug message")

	// This should appear
	logger.Log(WarnLevel, "warn message")
	logger.Log(ErrorLevel, "error message")

	output := buf.String()

	// Check that lower level messages are filtered out
	if strings.Contains(output, "info message") {
		t.Error("INFO level message should be filtered out")
	}

	if strings.Contains(output, "debug message") {
		t.Error("DEBUG level message should be filtered out")
	}

	// Check that higher level messages appear
	if !strings.Contains(output, "warn message") {
		t.Error("WARN level message should appear")
	}

	if !strings.Contains(output, "error message") {
		t.Error("ERROR level message should appear")
	}
}

func TestFieldConstructors(t *testing.T) {
	// Test String field
	stringField := String("key", "value")
	if stringField.Key != "key" || stringField.Value != "value" {
		t.Errorf("String() = %+v, expected {Key: 'key', Value: 'value'}", stringField)
	}

	// Test Int field
	intField := Int("count", 42)
	if intField.Key != "count" || intField.Value != 42 {
		t.Errorf("Int() = %+v, expected {Key: 'count', Value: 42}", intField)
	}

	// Test Bool field
	boolField := Bool("enabled", true)
	if boolField.Key != "enabled" || boolField.Value != true {
		t.Errorf("Bool() = %+v, expected {Key: 'enabled', Value: true}", boolField)
	}
}

func TestErrField(t *testing.T) {
	testErr := &testError{message: "test error"}
	errField := Err(testErr)

	if errField.Key != "error" {
		t.Errorf("Err() key = %v, expected 'error'", errField.Key)
	}

	if errField.Value != "test error" {
		t.Errorf("Err() value = %v, expected 'test error'", errField.Value)
	}
}

func TestConvenienceFunctions(t *testing.T) {
	// Initialize logger for testing convenience functions
	config := Config{
		Level:     InfoLevel,
		UseColor:  false,
		JSON:      false,
		Component: "test",
		NoOp:      false,
	}
	Initialize(config)

	var buf bytes.Buffer
	SetOutput(&buf)

	// Test Info function (should work since logger is initialized)
	Info("test info message")

	output := buf.String()
	if !strings.Contains(output, "test info message") {
		t.Errorf("Info() did not produce expected output: %s", output)
	}

	// Test that other levels work (though they might be filtered)
	Debug("test debug message")
	Trace("test trace message")
	Warn("test warn message")
	Error("test error message")

	// These should not appear due to level filtering, but the calls should not panic
}

func TestFallbackLogging(t *testing.T) {
	// Reset default logger to nil to test fallback
	originalLogger := defaultLogger
	defaultLogger = nil

	// This should use fallback logging to stderr
	// We can't easily capture stderr in this test, so we just verify it doesn't panic
	Info("fallback test message")

	// Restore original logger
	defaultLogger = originalLogger

	// If we get here without panicking, the test passes
}

func TestSetOutput(t *testing.T) {
	var buf bytes.Buffer

	// Initialize logger
	config := Config{
		Level:     InfoLevel,
		UseColor:  false,
		JSON:      false,
		Component: "test",
		NoOp:      false,
	}
	Initialize(config)

	// Set output to our buffer
	SetOutput(&buf)

	// Log a message
	Info("output test message")

	// Check that it went to our buffer
	output := buf.String()
	if !strings.Contains(output, "output test message") {
		t.Errorf("SetOutput() did not redirect output correctly: %s", output)
	}
}

// testError implements error interface for testing
type testError struct {
	message string
}

func (e *testError) Error() string {
	return e.message
}