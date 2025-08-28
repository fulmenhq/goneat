package logger

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"strings"
	"time"
)

// Level represents the severity level of log messages
type Level int

const (
	TraceLevel Level = iota
	DebugLevel
	InfoLevel
	WarnLevel
	ErrorLevel
)

// String returns the string representation of the level
func (l Level) String() string {
	switch l {
	case TraceLevel:
		return "TRACE"
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Config holds the logger configuration
type Config struct {
	Level     Level
	UseColor  bool
	JSON      bool
	Component string
	NoOp      bool
}

// Logger represents the logger instance
type Logger struct {
	config Config
	logger *log.Logger
}

// Default logger instance
var defaultLogger *Logger

// Initialize sets up the default logger
func Initialize(config Config) error {
	defaultLogger = &Logger{
		config: config,
		logger: log.New(os.Stderr, "", 0),
	}
	return nil
}

// Log writes a log message
func (l *Logger) Log(level Level, message string, fields ...Field) {
	if level < l.config.Level {
		return
	}

	entry := LogEntry{
		Time:      time.Now(),
		Level:     level.String(),
		Message:   message,
		Component: l.config.Component,
		Fields:    make(map[string]interface{}),
	}

	// Add caller info for debug and trace
	if level <= DebugLevel {
		_, file, line, ok := runtime.Caller(2)
		if ok {
			entry.File = file
			entry.Line = line
		}
	}

	// Add fields
	for _, field := range fields {
		entry.Fields[field.Key] = field.Value
	}

	var output string
	if l.config.JSON {
		jsonBytes, _ := json.Marshal(entry)
		output = string(jsonBytes)
	} else {
		output = l.formatPretty(entry)
	}

	l.logger.Print(output)
}

// formatPretty formats the log entry in a human-readable way
func (l *Logger) formatPretty(entry LogEntry) string {
	var builder strings.Builder

	// Time
	builder.WriteString(entry.Time.Format("2006-01-02 15:04:05"))

	// Level with color
	level := entry.Level
	if l.config.UseColor {
		switch entry.Level {
		case "TRACE":
			level = "\033[37mTRACE\033[0m" // White
		case "DEBUG":
			level = "\033[36mDEBUG\033[0m" // Cyan
		case "INFO":
			level = "\033[32mINFO\033[0m" // Green
		case "WARN":
			level = "\033[33mWARN\033[0m" // Yellow
		case "ERROR":
			level = "\033[31mERROR\033[0m" // Red
		}
	}

	builder.WriteString(fmt.Sprintf(" [%s]", level))

	// Component
	if entry.Component != "" {
		builder.WriteString(fmt.Sprintf(" %s:", entry.Component))
	}

	// No-op indicator
	if l.config.NoOp {
		if l.config.UseColor {
			builder.WriteString(" \033[35m[NO-OP]\033[0m") // Magenta
		} else {
			builder.WriteString(" [NO-OP]")
		}
	}

	// Message
	builder.WriteString(fmt.Sprintf(" %s", entry.Message))

	// Fields
	if len(entry.Fields) > 0 {
		builder.WriteString(" {")
		first := true
		for k, v := range entry.Fields {
			if !first {
				builder.WriteString(", ")
			}
			builder.WriteString(fmt.Sprintf("%s=%v", k, v))
			first = false
		}
		builder.WriteString("}")
	}

	// File and line for debug/trace
	if entry.File != "" {
		builder.WriteString(fmt.Sprintf(" (%s:%d)", entry.File, entry.Line))
	}

	return builder.String()
}

// Field represents a structured field in a log entry
type Field struct {
	Key   string
	Value interface{}
}

// String creates a string field
func String(key, value string) Field {
	return Field{Key: key, Value: value}
}

// Int creates an int field
func Int(key string, value int) Field {
	return Field{Key: key, Value: value}
}

// Bool creates a bool field
func Bool(key string, value bool) Field {
	return Field{Key: key, Value: value}
}

// Err creates an error field
func Err(err error) Field {
	return Field{Key: "error", Value: err.Error()}
}

// LogEntry represents a log entry
type LogEntry struct {
	Time      time.Time              `json:"time"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Component string                 `json:"component,omitempty"`
	File      string                 `json:"file,omitempty"`
	Line      int                    `json:"line,omitempty"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

// Convenience functions for default logger
func Trace(message string, fields ...Field) {
	if defaultLogger != nil {
		defaultLogger.Log(TraceLevel, message, fields...)
	}
}

func Debug(message string, fields ...Field) {
	if defaultLogger != nil {
		defaultLogger.Log(DebugLevel, message, fields...)
	}
}

func Info(message string, fields ...Field) {
	if defaultLogger != nil {
		defaultLogger.Log(InfoLevel, message, fields...)
	} else {
		// Fallback to stderr if logger not initialized
		os.Stderr.WriteString(fmt.Sprintf("[INFO] goneat: %s\n", message))
	}
}

func Warn(message string, fields ...Field) {
	if defaultLogger != nil {
		defaultLogger.Log(WarnLevel, message, fields...)
	}
}

func Error(message string, fields ...Field) {
	if defaultLogger != nil {
		defaultLogger.Log(ErrorLevel, message, fields...)
	}
}

// SetOutput sets the output writer for the logger
func SetOutput(w io.Writer) {
	if defaultLogger != nil {
		defaultLogger.logger.SetOutput(w)
	}
}
