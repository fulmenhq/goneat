---
title: Logger Library
description: Structured logging with STDOUT hygiene for CLI applications.
---

# Logger Library

Goneat's `pkg/logger` provides a structured logging system designed specifically for CLI applications and tools. It emphasizes STDOUT hygiene, level-based filtering, and integration with goneat's ecosystem while maintaining simplicity and performance.

## Purpose

CLI applications have unique logging requirements:

- **STDOUT hygiene**: Commands often need clean output for parsing (JSON, etc.)
- **Level-based filtering**: Debug output during development, quiet production
- **Structured output**: JSON logging for monitoring and debugging
- **Performance**: Minimal overhead for high-volume logging
- **Context awareness**: Request IDs, operation tracking, user context

The `pkg/logger` addresses these needs with a simple, opinionated API that prevents common logging pitfalls.

## Key Features

- **STDOUT hygiene**: No pollution of stdout/stderr unless explicitly configured
- **Structured logging**: JSON output with consistent fields
- **Level filtering**: Debug, Info, Warn, Error, Fatal levels
- **Context propagation**: Automatic context field inclusion
- **Performance optimized**: Zero allocation for common cases
- **CLI-friendly**: Colored output for terminals, clean JSON for pipes
- **Hook integration**: Pre/post hooks for log processing

## Installation

```bash
go get github.com/fulmenhq/goneat/pkg/logger
```

## Basic Usage

### Simple Logging

```go
package main

import (
    "context"
    "fmt"
    "github.com/fulmenhq/goneat/pkg/logger"
)

func main() {
    // Create logger with default configuration
    log := logger.New(context.Background())

    // Log at different levels
    log.Debug("Debug message - only shown with --debug flag")
    log.Info("Application started", "version", "v1.2.3")
    log.Warn("Configuration warning", "setting", "deprecated")
    log.Error("Database connection failed", "error", "timeout")

    // Fatal logs and exits (like log.Fatal but with structure)
    // log.Fatal("Critical error", "reason", "unrecoverable")
}
```

### Structured Logging with Fields

```go
package main

import (
    "context"
    "fmt"
    "time"
    "github.com/fulmenhq/goneat/pkg/logger"
)

func processUserRequest(ctx context.Context, userID string, action string) {
    log := logger.FromContext(ctx)

    start := time.Now()
    log.Info("Processing user request",
        "user_id", userID,
        "action", action,
        "request_id", logger.RequestIDFromContext(ctx),
    )

    // Simulate processing
    if err := doProcessing(userID, action); err != nil {
        log.Error("User request failed",
            "user_id", userID,
            "action", action,
            "error", err,
            "duration_ms", time.Since(start).Milliseconds(),
        )
        return
    }

    log.Info("User request completed successfully",
        "user_id", userID,
        "action", action,
        "duration_ms", time.Since(start).Milliseconds(),
    )
}

func main() {
    ctx := context.Background()

    // Add request ID to context for correlation
    ctx = logger.WithRequestID(ctx, "req-12345")

    processUserRequest(ctx, "user-456", "update_profile")
}
```

## API Reference

### logger.Logger

```go
type Logger struct {
    // Structured logger instance
}

func New(ctx context.Context) *Logger
func FromContext(ctx context.Context) *Logger
func WithContext(ctx context.Context, log *Logger) context.Context

// Logging methods
func (l *Logger) Debug(msg string, fields ...Field)
func (l *Logger) Info(msg string, fields ...Field)
func (l *Logger) Warn(msg string, fields ...Field)
func (l *Logger) Error(msg string, fields ...Field)
func (l *Logger) Fatal(msg string, fields ...Field)

// Level management
func (l *Logger) SetLevel(level Level)
func (l *Logger) Level() Level
func (l *Logger) IsLevelEnabled(level Level) bool

// Output configuration
func (l *Logger) SetOutput(w io.Writer)
func (l *Logger) SetFormat(format Format)
func (l *Logger) WithFields(fields Fields) *Logger
func (l *Logger) WithField(key string, value interface{}) *Logger

// Context helpers
func RequestIDFromContext(ctx context.Context) string
func WithRequestID(ctx context.Context, id string) context.Context
func UserIDFromContext(ctx context.Context) string
func WithUserID(ctx context.Context, id string) context.Context
```

### Log Levels

```go
type Level string

const (
    LevelDebug Level = "debug"
    LevelInfo  Level = "info"
    LevelWarn  Level = "warn"
    LevelError Level = "error"
    LevelFatal Level = "fatal"
    LevelOff   Level = "off"
)
```

### Log Fields

```go
type Field struct {
    Key   string
    Value interface{}
}

func F(key string, value interface{}) Field
type Fields map[string]interface{}

func WithFields(fields Fields) *Logger
```

### Output Formats

```go
type Format string

const (
    FormatJSON   Format = "json"
    FormatText   Format = "text"
    FormatPretty Format = "pretty"
    FormatSilent Format = "silent"
)
```

## Advanced Usage

### CLI Application Logging

```go
package main

import (
    "context"
    "flag"
    "fmt"
    "os"
    "github.com/fulmenhq/goneat/pkg/logger"
)

func main() {
    var (
        debug   = flag.Bool("debug", false, "enable debug logging")
        verbose = flag.Bool("verbose", false, "enable verbose logging")
        quiet   = flag.Bool("quiet", false, "suppress non-error output")
        json    = flag.Bool("json", false, "JSON output format")
        version = flag.Bool("version", false, "show version")
    )

    flag.Parse()

    ctx := context.Background()

    // Configure logger based on flags
    log := logger.New(ctx)

    if *quiet {
        log.SetLevel(logger.LevelError)
        log.SetFormat(logger.FormatSilent) // Only errors to stderr
    } else if *debug {
        log.SetLevel(logger.LevelDebug)
        log.SetFormat(logger.FormatPretty) // Colored output for terminal
    } else if *verbose {
        log.SetLevel(logger.LevelInfo)
        log.SetFormat(logger.FormatText)
    } else if *json {
        log.SetLevel(logger.LevelInfo)
        log.SetFormat(logger.FormatJSON) // For piping/machine consumption
    } else {
        log.SetLevel(logger.LevelWarn) // Default: warnings and above
        log.SetFormat(logger.FormatText)
    }

    // Add global fields
    log = log.WithFields(logger.Fields{
        "app":     "goneat",
        "version": "v0.2.7",
        "pid":     os.Getpid(),
    })

    // Set as global logger for the context
    ctx = logger.WithContext(ctx, log)

    if *version {
        log.Info("Version information", "version", "v0.2.7")
        os.Exit(0)
    }

    log.Info("Starting application", "args", os.Args)

    // Your application logic here
    if err := runApplication(ctx, flag.Args()); err != nil {
        log.Error("Application failed", "error", err)
        os.Exit(1)
    }

    log.Info("Application completed successfully")
}

func runApplication(ctx context.Context, args []string) error {
    log := logger.FromContext(ctx)

    log.Debug("Processing arguments", "count", len(args), "args", args)

    if len(args) == 0 {
        log.Warn("No arguments provided, using defaults")
        return nil
    }

    // Process each argument
    for i, arg := range args {
        log.Info("Processing item",
            "index", i,
            "item", arg,
            "type", detectItemType(arg),
        )

        if err := processItem(ctx, arg); err != nil {
            log.Error("Failed to process item",
                "index", i,
                "item", arg,
                "error", err,
            )
            return err
        }
    }

    return nil
}

func processItem(ctx context.Context, item string) error {
    log := logger.FromContext(ctx)
    log.Debug("Processing individual item", "item", item)
    // Simulate processing
    return nil
}

func detectItemType(item string) string {
    // Implementation detail
    return "file"
}
```

### Context-Aware Logging with Correlation

```go
package main

import (
    "context"
    "fmt"
    "time"
    "github.com/fulmenhq/goneat/pkg/logger"
)

type Operation struct {
    ID       string
    Start    time.Time
    Logger   *logger.Logger
    Context  context.Context
}

func NewOperation(ctx context.Context, operationType string) *Operation {
    opID := generateID() // Your ID generation logic

    // Create operation-specific logger with correlation fields
    opLog := logger.FromContext(ctx).WithFields(logger.Fields{
        "operation_id": opID,
        "operation_type": operationType,
        "parent_request_id": logger.RequestIDFromContext(ctx),
    })

    // Create sub-context with operation info
    opCtx := logger.WithFieldsContext(ctx, opLog)
    opCtx = logger.WithOperationID(opCtx, opID)

    return &Operation{
        ID:      opID,
        Start:   time.Now(),
        Logger:  opLog,
        Context: opCtx,
    }
}

func (op *Operation) LogInfo(msg string, fields logger.Fields) {
    op.Logger.Info(msg,
        "operation_id", op.ID,
        "duration_ms", time.Since(op.Start).Milliseconds(),
        fields...,
    )
}

func (op *Operation) Complete(success bool, result interface{}) {
    duration := time.Since(op.Start)

    fields := logger.Fields{
        "duration_ms": duration.Milliseconds(),
        "success":     success,
    }

    if result != nil {
        fields["result"] = result
    }

    if success {
        op.Logger.Info("Operation completed", fields)
    } else {
        op.Logger.Error("Operation failed", fields)
    }
}

func processBatch(ctx context.Context, items []string) error {
    log := logger.FromContext(ctx)

    // Create batch operation
    batchOp := NewOperation(ctx, "batch_process")

    batchOp.LogInfo("Starting batch processing", logger.Fields{
        "item_count": len(items),
        "batch_id":   batchOp.ID,
    })

    var results []string
    var errors int

    for i, item := range items {
        // Create sub-operation for each item
        itemOp := NewOperation(batchOp.Context, "process_item")

        itemOp.LogInfo("Processing item", logger.Fields{
            "index": i,
            "item":  item,
            "parent_operation": batchOp.ID,
        })

        if err := processSingleItem(itemOp.Context, item); err != nil {
            itemOp.Complete(false, err)
            errors++
        } else {
            itemOp.Complete(true, "success")
            results = append(results, item)
        }
    }

    batchOp.LogInfo("Batch processing completed", logger.Fields{
        "processed": len(results),
        "errors":    errors,
        "total":     len(items),
    })

    batchOp.Complete(errors == 0, results)

    if errors > 0 {
        return fmt.Errorf("batch processing had %d errors", errors)
    }

    return nil
}

func main() {
    ctx := context.Background()
    ctx = logger.WithRequestID(ctx, "main-request-001")

    items := []string{"file1.go", "file2.js", "file3.py"}

    if err := processBatch(ctx, items); err != nil {
        log := logger.FromContext(ctx)
        log.Error("Batch failed", "error", err)
        os.Exit(1)
    }

    fmt.Println("All items processed successfully")
}
```

### Performance-Optimized Logging

```go
package main

import (
    "context"
    "fmt"
    "testing"
    "github.com/fulmenhq/goneat/pkg/logger"
)

// High-performance logging for hot paths
type PerformanceSensitiveProcessor struct {
    log    *logger.Logger
    quiet  bool
    buffer []string // For batch logging
}

func NewPerformanceSensitiveProcessor(ctx context.Context) *PerformanceSensitiveProcessor {
    log := logger.FromContext(ctx)

    // Configure for performance
    log.SetLevel(logger.LevelInfo) // Avoid debug overhead
    log.SetFormat(logger.FormatJSON) // Consistent, fast serialization

    return &PerformanceSensitiveProcessor{
        log:   log,
        quiet: false,
        buffer: make([]string, 0, 1000),
    }
}

func (psp *PerformanceSensitiveProcessor) ProcessWithMinimalLogging(items []string) {
    if psp.quiet {
        // Fast path: no logging overhead
        psp.processItemsFast(items)
        return
    }

    // Buffered logging for performance
    start := time.Now()

    for i, item := range items {
        // Only log every 100th item to reduce overhead
        if i%100 == 0 {
            psp.log.Info("Processing batch",
                "batch_size", len(items),
                "current_index", i,
                "progress_percent", float64(i)/float64(len(items))*100,
            )
        }

        // Fast processing
        if err := psp.processItem(item); err != nil {
            // Buffer error for batch reporting
            psp.buffer = append(psp.buffer, fmt.Sprintf("error at %d: %v", i, err))
        }
    }

    // Batch report
    duration := time.Since(start)
    psp.log.Info("Batch processing completed",
        "total_items", len(items),
        "duration_ms", duration.Milliseconds(),
        "errors_count", len(psp.buffer),
    )

    // Report errors in batch
    if len(psp.buffer) > 0 {
        psp.log.Warn("Batch processing errors",
            "error_count", len(psp.buffer),
            "errors_sample", psp.buffer[:min(5, len(psp.buffer))],
        )
    }
}

func (psp *PerformanceSensitiveProcessor) processItemsFast(items []string) {
    // Implementation without logging overhead
    for _, item := range items {
        _ = psp.processItem(item) // Process without logging
    }
}

func (psp *PerformanceSensitiveProcessor) processItem(item string) error {
    // Simulate processing
    time.Sleep(10 * time.Microsecond)
    if item == "fail" {
        return fmt.Errorf("simulated failure")
    }
    return nil
}

// Benchmark to demonstrate performance characteristics
func BenchmarkLoggerPerformance(b *testing.B) {
    ctx := context.Background()
    log := logger.New(ctx)

    // Test different logging approaches
    b.Run("no_logging", func(b *testing.B) {
        for i := 0; i < b.N; i++ {
            // Just a function call, no logging
            _ = processWithoutLogging(i)
        }
    })

    b.Run("debug_logging", func(b *testing.B) {
        log.SetLevel(logger.LevelDebug)
        for i := 0; i < b.N; i++ {
            log.Debug("Processing item", "index", i)
        }
    })

    b.Run("info_logging", func(b *testing.B) {
        log.SetLevel(logger.LevelInfo)
        for i := 0; i < b.N; i++ {
            log.Info("Processing item", "index", i)
        }
    })

    b.Run("structured_info", func(b *testing.B) {
        log.SetLevel(logger.LevelInfo)
        fields := logger.Fields{"index": 0, "type": "test"}
        for i := 0; i < b.N; i++ {
            fields["index"] = i
            log.Info("Processing item", fields...)
        }
    })
}

func processWithoutLogging(index int) error {
    // Simulate work without logging
    return nil
}
```

## Configuration Options

### Comprehensive Logger Setup

```go
package main

import (
    "context"
    "fmt"
    "os"
    "github.com/fulmenhq/goneat/pkg/logger"
)

func createProductionLogger(ctx context.Context) *logger.Logger {
    log := logger.New(ctx)

    // Production configuration
    log.SetLevel(logger.LevelInfo)

    // JSON output for log aggregation systems
    log.SetFormat(logger.FormatJSON)

    // Log to both stdout and a file
    if file, err := os.OpenFile("goneat.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err != nil {
        log.Warn("Failed to open log file, using only stdout", "error", err)
    } else {
        log.SetOutput(io.MultiWriter(os.Stdout, file))
    }

    // Add global metadata
    log = log.WithFields(logger.Fields{
        "environment": "production",
        "service":     "goneat-core",
        "version":     "v0.2.7",
        "hostname":    getHostname(),
    })

    // Configure hooks for additional processing
    log.AddHook(&logger.JSONSanitizationHook{
        SensitiveFields: []string{"password", "token", "secret"},
    })

    log.AddHook(&logger.RequestCorrelationHook{})

    return log
}

func createDevelopmentLogger(ctx context.Context) *logger.Logger {
    log := logger.New(ctx)

    // Development: more verbose, human-readable
    log.SetLevel(logger.LevelDebug)
    log.SetFormat(logger.FormatPretty) // Colored output

    // Global development fields
    log = log.WithFields(logger.Fields{
        "environment": "development",
        "debug_mode":  true,
    })

    // Development-specific hooks
    log.AddHook(&logger.DevelopmentStackTraceHook{
        MaxFrames: 10,
    })

    return log
}

func getHostname() string {
    hostname, _ := os.Hostname()
    return hostname
}

// Usage in main
func main() {
    ctx := context.Background()

    var log *logger.Logger
    if os.Getenv("GONEAT_ENV") == "production" {
        log = createProductionLogger(ctx)
    } else {
        log = createDevelopmentLogger(ctx)
    }

    // Set as context logger
    ctx = logger.WithContext(ctx, log)

    log.Info("Logger configured", "mode", os.Getenv("GONEAT_ENV"))

    // Your application code here
    runApplication(ctx)
}
```

### Custom Log Hooks

```go
// Example: Custom hook for request correlation
type RequestCorrelationHook struct{}

func (h *RequestCorrelationHook) Levels() []logger.Level {
    return []logger.Level{logger.LevelInfo, logger.LevelWarn, logger.LevelError}
}

func (h *RequestCorrelationHook) Fire(entry *logger.Entry) error {
    // Add correlation ID if not present
    if _, exists := entry.Fields["request_id"]; !exists {
        if id := logger.RequestIDFromContext(entry.Context); id != "" {
            entry.Fields["request_id"] = id
        }
    }

    // Add operation ID if available
    if opID := logger.OperationIDFromContext(entry.Context); opID != "" {
        entry.Fields["operation_id"] = opID
    }

    return nil
}

// Example: Sensitive data sanitization hook
type JSONSanitizationHook struct {
    SensitiveFields []string
}

func (h *JSONSanitizationHook) Levels() []logger.Level {
    return []logger.Level{logger.LevelDebug, logger.LevelInfo, logger.LevelWarn, logger.LevelError}
}

func (h *JSONSanitizationHook) Fire(entry *logger.Entry) error {
    for _, field := range h.SensitiveFields {
        if value, exists := entry.Fields[field]; exists {
            // Replace sensitive values with [REDACTED]
            entry.Fields[field] = "[REDACTED]"
        }
    }
    return nil
}

// Example: Stack trace hook for development
type DevelopmentStackTraceHook struct {
    MaxFrames int
}

func (h *DevelopmentStackTraceHook) Levels() []logger.Level {
    return []logger.Level{logger.LevelError, logger.LevelFatal}
}

func (h *DevelopmentStackTraceHook) Fire(entry *logger.Entry) error {
    if entry.Level != logger.LevelError && entry.Level != logger.LevelFatal {
        return nil
    }

    // Capture stack trace (using runtime/debug or similar)
    stack := captureStackTrace(h.MaxFrames)
    entry.Fields["stack_trace"] = stack

    return nil
}

func captureStackTrace(maxFrames int) string {
    // Implementation using debug.Stack() or similar
    // This is a simplified example
    return "stack trace placeholder"
}
```

## STDOUT Hygiene Best Practices

### Clean Command Output

```go
// Example: Command that produces parseable JSON output
type JSONCommand struct {
    log *logger.Logger
}

func (cmd *JSONCommand) Run(args []string) error {
    // Configure logger for clean JSON output
    cmd.log.SetFormat(logger.FormatJSON)
    cmd.log.SetLevel(logger.LevelError) // Only errors during execution

    // Create structured output
    result := struct {
        Status  string                 `json:"status"`
        Data    map[string]interface{}  `json:"data,omitempty"`
        Errors  []string               `json:"errors,omitempty"`
        Version string                 `json:"version"`
        Timestamp string               `json:"timestamp"`
    }{
        Status:  "success",
        Version: "v0.2.7",
        Timestamp: time.Now().UTC().Format(time.RFC3339),
    }

    // Process without polluting stdout
    if err := cmd.processData(args, &result); err != nil {
        cmd.log.Error("Command execution failed", "error", err)
        result.Status = "error"
        result.Errors = []string{err.Error()}
    } else {
        // Log success to stderr (doesn't pollute stdout)
        cmd.log.Info("Command completed successfully")
    }

    // Output clean JSON to stdout
    output, err := json.MarshalIndent(result, "", "  ")
    if err != nil {
        cmd.log.Error("Failed to marshal output", "error", err)
        return err
    }

    // Write directly to stdout without logger (STDOUT hygiene)
    _, err = os.Stdout.Write(output)
    _, err = os.Stdout.Write([]byte("\n"))
    return err
}

func (cmd *JSONCommand) processData(args []string, result *structResult) error {
    // All logging goes to stderr
    cmd.log.Debug("Processing data", "arg_count", len(args))

    // Your processing logic here
    result.Data = map[string]interface{}{
        "processed_count": len(args),
        "arguments":       args,
    }

    return nil
}
```

### Mixed Output Modes

```go
// Support both human-readable and machine-readable output
type DualModeCommand struct {
    log      *logger.Logger
    jsonMode bool
}

func NewDualModeCommand(ctx context.Context, jsonMode bool) *DualModeCommand {
    log := logger.FromContext(ctx)

    if jsonMode {
        log.SetFormat(logger.FormatJSON)
        log.SetOutput(os.Stderr) // Logs to stderr, output to stdout
    } else {
        log.SetFormat(logger.FormatPretty)
        log.SetOutput(os.Stdout) // Human-readable to stdout
    }

    return &DualModeCommand{
        log:      log,
        jsonMode: jsonMode,
    }
}

func (cmd *DualModeCommand) Execute(operation string, data interface{}) error {
    start := time.Now()

    if cmd.jsonMode {
        // JSON mode: clean structured output to stdout, logs to stderr
        cmd.log.Info("Starting operation",
            "operation", operation,
            "mode", "json",
            "timestamp", time.Now().UTC().Format(time.RFC3339),
        )

        result, err := cmd.executeWithJSON(operation, data)

        // Log to stderr
        if err != nil {
            cmd.log.Error("Operation failed",
                "operation", operation,
                "duration_ms", time.Since(start).Milliseconds(),
                "error", err,
            )
        } else {
            cmd.log.Info("Operation completed",
                "operation", operation,
                "duration_ms", time.Since(start).Milliseconds(),
            )
        }

        // Clean JSON output to stdout
        if err != nil {
            return cmd.outputJSONError(operation, err)
        }
        return cmd.outputJSONResult(operation, result)

    } else {
        // Human-readable mode: everything to stdout
        cmd.log.Info(fmt.Sprintf("Starting %s operation...", operation))

        result, err := cmd.executeWithPretty(operation, data)

        if err != nil {
            cmd.log.Error(fmt.Sprintf("Operation %s failed: %v", operation, err))
            return err
        }

        cmd.log.Info(fmt.Sprintf("Operation %s completed successfully", operation))
        return cmd.outputPrettyResult(operation, result)
    }
}

func (cmd *DualModeCommand) outputJSONResult(operation string, result interface{}) error {
    output := map[string]interface{}{
        "status":    "success",
        "operation": operation,
        "result":    result,
        "timestamp": time.Now().UTC().Format(time.RFC3339),
    }

    data, err := json.Marshal(output)
    if err != nil {
        return err
    }

    // Direct stdout write (bypasses logger for STDOUT hygiene)
    _, err = os.Stdout.Write(data)
    _, err = os.Stdout.Write([]byte("\n"))
    return err
}

func (cmd *DualModeCommand) outputJSONError(operation string, err error) error {
    output := map[string]interface{}{
        "status":    "error",
        "operation": operation,
        "error":     err.Error(),
        "timestamp": time.Now().UTC().Format(time.RFC3339),
    }

    data, _ := json.Marshal(output)
    _, err = os.Stdout.Write(data)
    _, err = os.Stdout.Write([]byte("\n"))
    return err
}

func (cmd *DualModeCommand) outputPrettyResult(operation string, result interface{}) error {
    // Human-readable output with logger
    cmd.log.Info(fmt.Sprintf("%s operation completed", operation))
    // Additional pretty printing logic here
    return nil
}
```

## Integration with Goneat Ecosystem

### With pkg/config

```go
import (
    "context"
    "github.com/fulmenhq/goneat/pkg/config"
    "github.com/fulmenhq/goneat/pkg/logger"
)

func createConfiguredLogger(ctx context.Context) (*logger.Logger, error) {
    cfg, err := config.New(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to load config: %w", err)
    }

    // Configure logger from configuration
    log := logger.New(ctx)

    logLevel := cfg.GetString("logging.level")
    switch logLevel {
    case "debug":
        log.SetLevel(logger.LevelDebug)
    case "info":
        log.SetLevel(logger.LevelInfo)
    case "warn":
        log.SetLevel(logger.LevelWarn)
    case "error":
        log.SetLevel(logger.LevelError)
    default:
        log.Info("Unknown log level, using info", "level", logLevel)
        log.SetLevel(logger.LevelInfo)
    }

    logFormat := cfg.GetString("logging.format")
    switch logFormat {
    case "json":
        log.SetFormat(logger.FormatJSON)
    case "text":
        log.SetFormat(logger.FormatText)
    case "pretty":
        log.SetFormat(logger.FormatPretty)
    default:
        log.SetFormat(logger.FormatText)
    }

    // Configure output destinations
    outputs := cfg.GetStringSlice("logging.outputs")
    for _, output := range outputs {
        switch output {
        case "stdout":
            log.AddOutput(os.Stdout)
        case "stderr":
            log.AddOutput(os.Stderr)
        case "file":
            if filePath := cfg.GetString("logging.file.path"); filePath != "" {
                if file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
                    log.AddOutput(file)
                } else {
                    log.Warn("Failed to open log file", "path", filePath, "error", err)
                }
            }
        }
    }

    // Add global fields from config
    globalFields := logger.Fields{}
    if service := cfg.GetString("service.name"); service != "" {
        globalFields["service"] = service
    }
    if env := cfg.GetString("environment"); env != "" {
        globalFields["environment"] = env
    }
    if version := cfg.GetString("version"); version != "" {
        globalFields["version"] = version
    }

    if len(globalFields) > 0 {
        log = log.WithFields(globalFields)
    }

    log.Info("Logger configured from configuration",
        "level", logLevel,
        "format", logFormat,
        "outputs", len(outputs),
    )

    return log, nil
}
```

### With pkg/exitcode

```go
import (
    "context"
    "fmt"
    "os"
    "github.com/fulmenhq/goneat/pkg/exitcode"
    "github.com/fulmenhq/goneat/pkg/logger"
)

type SafeApplication struct {
    log *logger.Logger
}

func (app *SafeApplication) Run(ctx context.Context) int {
    log := logger.FromContext(ctx)

    log.Info("Application startup", "pid", os.Getpid())

    // Phase 1: Initialization
    if err := app.initialize(ctx); err != nil {
        log.Error("Initialization failed", "error", err)
        return exitcode.ErrInitialization.Failure()
    }
    log.Debug("Initialization completed")

    // Phase 2: Main processing
    if err := app.mainProcess(ctx); err != nil {
        log.Error("Main processing failed", "error", err)
        return exitcode.ErrProcessing.Failure()
    }
    log.Info("Main processing completed")

    // Phase 3: Cleanup
    if err := app.cleanup(ctx); err != nil {
        log.Error("Cleanup failed", "error", err)
        // Return specific cleanup error code
        return exitcode.ErrCleanup.Failure()
    }
    log.Debug("Cleanup completed")

    log.Info("Application completed successfully")
    return exitcode.ExitSuccess
}

func (app *SafeApplication) initialize(ctx context.Context) error {
    log := logger.FromContext(ctx)
    log.Debug("Starting initialization phase")

    // Simulate initialization steps
    if err := app.loadConfig(ctx); err != nil {
        return fmt.Errorf("config load: %w", err)
    }

    if err := app.connectDatabase(ctx); err != nil {
        return fmt.Errorf("database connection: %w", err)
    }

    log.Debug("Initialization steps completed")
    return nil
}

func (app *SafeApplication) mainProcess(ctx context.Context) error {
    log := logger.FromContext(ctx)
    log.Info("Starting main processing")

    // Your main application logic
    return nil
}

func (app *SafeApplication) cleanup(ctx context.Context) error {
    log := logger.FromContext(ctx)
    log.Debug("Starting cleanup")

    // Cleanup logic
    return nil
}

func (app *SafeApplication) loadConfig(ctx context.Context) error {
    log := logger.FromContext(ctx)
    log.Debug("Loading configuration")
    // Config loading logic
    return nil
}

func (app *SafeApplication) connectDatabase(ctx context.Context) error {
    log := logger.FromContext(ctx)
    log.Info("Connecting to database")
    // Database connection logic
    return nil
}

// Usage
func main() {
    ctx := context.Background()
    log := logger.New(ctx)
    ctx = logger.WithContext(ctx, log)

    app := &SafeApplication{log: log}

    exitCode := app.Run(ctx)
    log.Info("Application exiting", "exit_code", exitCode)

    os.Exit(exitCode)
}
```

## Testing Logging Behavior

### Unit Tests for Logging

```go
package logger_test

import (
    "bytes"
    "context"
    "testing"
    "github.com/fulmenhq/goneat/pkg/logger"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestLoggerLevels(t *testing.T) {
    // Capture output
    var buf bytes.Buffer
    log := logger.New(context.Background())
    log.SetOutput(&buf)
    log.SetFormat(logger.FormatText)

    // Test different log levels
    t.Run("debug_not_logged_at_info", func(t *testing.T) {
        log.SetLevel(logger.LevelInfo)
        log.Debug("This should not appear")
        assert.NotContains(t, buf.String(), "This should not appear")
    })

    t.Run("info_logged_at_info", func(t *testing.T) {
        buf.Reset()
        log.Info("This should appear")
        assert.Contains(t, buf.String(), "This should appear")
    })

    t.Run("warn_logged_at_info", func(t *testing.T) {
        buf.Reset()
        log.Warn("Warning message")
        assert.Contains(t, buf.String(), "Warning message")
    })

    t.Run("error_logged_at_info", func(t *testing.T) {
        buf.Reset()
        log.Error("Error message")
        assert.Contains(t, buf.String(), "Error message")
    })
}

func TestStructuredLogging(t *testing.T) {
    var buf bytes.Buffer
    log := logger.New(context.Background())
    log.SetOutput(&buf)
    log.SetFormat(logger.FormatJSON)
    log.SetLevel(logger.LevelInfo)

    log.Info("Test structured log",
        "user_id", "123",
        "action", "create",
        "timestamp", "2025-09-20T12:00:00Z",
    )

    // Verify JSON structure
    expectedFields := map[string]interface{}{
        "level":   "info",
        "message": "Test structured log",
        "user_id": "123",
        "action":  "create",
        "timestamp": "2025-09-20T12:00:00Z",
        "time":    expectAnyTimestamp(),
    }

    var actual map[string]interface{}
    require.NoError(t, json.Unmarshal(buf.Bytes(), &actual))

    for key, expectedValue := range expectedFields {
        if key == "time" {
            // Timestamp should be present but we don't care about exact value
            assert.Contains(t, actual, "time")
        } else {
            assert.Equal(t, expectedValue, actual[key])
        }
    }
}

func TestContextPropagation(t *testing.T) {
    ctx := context.Background()

    // Add fields to context
    ctx = logger.WithRequestID(ctx, "test-request-123")
    ctx = logger.WithUserID(ctx, "test-user-456")

    log := logger.FromContext(ctx)

    log.Info("Context test")

    var buf bytes.Buffer
    log.SetOutput(&buf)
    log.SetFormat(logger.FormatJSON)

    // Should include context fields
    assert.Contains(t, buf.String(), `"request_id":"test-request-123"`)
    assert.Contains(t, buf.String(), `"user_id":"test-user-456"`)
}

func expectAnyTimestamp() interface{} {
    return "any" // Helper for test expectations
}
```

### Integration Tests

```go
func TestCLICommandLogging(t *testing.T) {
    // Test CLI command with different log levels
    tests := []struct {
        name      string
        args      []string
        expected  string
        notExpect string
    }{
        {
            name:     "debug_mode",
            args:     []string{"--debug", "command"},
            expected: "DEBUG: Processing",
        },
        {
            name:      "normal_mode",
            args:      []string{"command"},
            expected:  "INFO: Starting",
            notExpect: "DEBUG: Processing",
        },
        {
            name:     "json_mode",
            args:     []string{"--json", "command"},
            expected: `"level":"info"`,
        },
        {
            name:     "quiet_mode",
            args:     []string{"--quiet", "command"},
            expected: `"level":"error"`, // Only errors
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Capture stdout and stderr
            oldStdout := os.Stdout
            oldStderr := os.Stderr
            rOut, wOut, _ := os.Pipe()
            rErr, wErr, _ := os.Pipe()

            os.Stdout = wOut
            os.Stderr = wErr

            defer func() {
                os.Stdout = oldStdout
                os.Stderr = oldStderr
            }()

            // Run command with arguments
            cmd := exec.Command(cliBinaryPath, tt.args...)
            err := cmd.Run()
            require.NoError(t, err)

            // Restore output
            wOut.Close()
            wErr.Close()

            outBytes, err := io.ReadAll(rOut)
            require.NoError(t, err)
            errBytes, err := io.ReadAll(rErr)
            require.NoError(t, err)

            output := string(outBytes)
            stderr := string(errBytes)

            if tt.expected != "" {
                assert.Contains(t, output+stderr, tt.expected)
            }

            if tt.notExpect != "" {
                assert.NotContains(t, output+stderr, tt.notExpect)
            }
        })
    }
}
```

## Common Patterns and Anti-Patterns

### ✅ Good Practices

```go
// 1. Context-aware logging
func processRequest(ctx context.Context, req *Request) error {
    log := logger.FromContext(ctx) // Gets request-specific logger

    log.Info("Processing request",
        "request_id", logger.RequestIDFromContext(ctx),
        "user_id", req.UserID,
        "method", req.Method,
    )

    // Use context logger throughout the request
    return processRequestInternal(ctx, req)
}

// 2. Structured fields for all logs
func validateConfig(config *Config) error {
    log := logger.FromContext(context.Background())

    issues := validateAllFields(config)

    if len(issues) > 0 {
        log.Warn("Configuration validation issues found",
            "issue_count", len(issues),
            "config_version", config.Version,
            "issues", issues, // Structured error details
        )
        return fmt.Errorf("validation failed with %d issues", len(issues))
    }

    log.Debug("Configuration validation passed",
        "config_size", fmt.Sprintf("%d fields", countFields(config)),
    )

    return nil
}

// 3. Proper error enrichment
func databaseQuery(ctx context.Context, query string, params map[string]interface{}) (*Result, error) {
    log := logger.FromContext(ctx)

    start := time.Now()
    result, err := db.Execute(query, params)

    fields := logger.Fields{
        "query_length": len(query),
        "param_count":  len(params),
        "duration_ms":  time.Since(start).Milliseconds(),
    }

    if err != nil {
        fields["error"] = err
        fields["query_hash"] = hashQuery(query) // For debugging without exposing full query
        log.Error("Database query failed", fields)
        return nil, fmt.Errorf("database query failed: %w", err)
    }

    fields["rows_affected"] = result.RowsAffected
    log.Debug("Database query succeeded", fields)

    return result, nil
}
```

### ❌ Anti-Patterns to Avoid

```go
// 1. Stdout pollution (STDOUT hygiene violation)
func badCLICommand() error {
    fmt.Println("Processing...") // ❌ Pollutes stdout, breaks JSON parsing
    fmt.Printf("DEBUG: %d items\n", count) // ❌ Debug in stdout

    // Even worse:
    fmt.Fprintf(os.Stderr, "Error: %v\n", err) // ❌ Inconsistent error format

    return nil
}

// 2. Unstructured logging
func badErrorHandling(err error) {
    log.Println(err) // ❌ No structure, no context, no level

    // Even worse:
    fmt.Printf("ERROR: something went wrong: %v\n", err) // ❌ fmt instead of logger
}

// 3. Missing context correlation
func badRequestProcessing(req *Request) {
    log.Info("Processing") // ❌ No request ID, can't correlate in logs

    if err := doSomething(); err != nil {
        log.Error("Failed") // ❌ No context about what failed
    }
}

// 4. Performance issues
func badHighVolumeLogging(items []string) {
    for _, item := range items {
        log.Info("Processing item", "item", item) // ❌ Logs every single item
    }

    // Better: batch or sample logging
    log.Info("Processing batch", "count", len(items), "sample", items[:3])
}

// 5. Security issues
func badSensitiveLogging(password string) {
    log.Info("User login", "password", password) // ❌ Never log secrets!

    // Even with sanitization:
    log.Debug("Config loaded", "full_config", config) // ❌ May contain secrets
}
```

## Performance Characteristics

### Logging Overhead Benchmarks

```go
// Typical performance numbers (approximate):
// - Debug log (disabled level): ~50ns/op, 0 allocs/op
// - Info log (enabled, no fields): ~200ns/op, 1 alloc/op
// - Info log (with 5 fields): ~500ns/op, 3 allocs/op
// - JSON formatting overhead: ~1μs/op for complex objects
// - File I/O overhead: ~10μs/op (depends on disk)

// In hot paths, use level checks:
func hotPathProcessing(items []string) {
    log := logger.FromContext(ctx)

    if log.IsLevelEnabled(logger.LevelDebug) {
        // Only log if debug is enabled (rare in production)
        log.Debug("Hot path processing", "count", len(items))
    }

    // Fast path for 99% of cases
    for _, item := range items {
        processItemFast(item)
    }

    // Batch log results
    log.Info("Batch completed",
        "processed", len(items),
        "start_time", startTime,
        "duration_ms", time.Since(startTime).Milliseconds(),
    )
}
```

## Security Considerations

### Sensitive Data Handling

```go
// 1. Never log secrets directly
func badPasswordLogging(user, password string) {
    log.Info("User authenticated", "user", user, "password", password) // ❌
}

// 2. Use sanitization hooks or manual redaction
func securePasswordLogging(user, password string) {
    log.Info("User authentication attempt",
        "user", user,
        "password", "[REDACTED]", // ✅
        "password_hash", hashPassword(password), // ✅ Log hash instead
    )
}

// 3. Sanitize complex structures
type Config struct {
    DatabaseURL string `json:"database_url"`
    APIKey      string `json:"api_key"`
    Debug       bool   `json:"debug"`
}

func logConfigSafely(config *Config) {
    safeConfig := *config
    safeConfig.DatabaseURL = "[REDACTED]"
    safeConfig.APIKey = "[REDACTED]"

    log.Debug("Configuration loaded", "config", safeConfig)

    // Or use the sanitization hook
    log.Debug("Full configuration loaded", "config", config) // Hook will redact
}

// 4. Query logging without exposing parameters
func logQuerySafely(query string, params map[string]interface{}) {
    // Log query structure without parameters
    queryHash := hashQuery(query)
    paramKeys := make([]string, 0, len(params))
    for k := range params {
        paramKeys = append(paramKeys, k)
    }

    log.Debug("Executing query",
        "query_hash", queryHash,
        "query_preview", previewQuery(query),
        "param_count", len(params),
        "param_keys", paramKeys,
    )
}

func hashQuery(query string) string {
    h := sha256.New()
    h.Write([]byte(query))
    return hex.EncodeToString(h.Sum(nil))[:16]
}

func previewQuery(query string) string {
    // Show first 100 chars and indicate truncation
    if len(query) <= 100 {
        return query
    }
    return query[:100] + "..."
}
```

### Log Injection Prevention

```go
// Prevent log injection attacks
func safeLogMessage(userInput string) string {
    // Remove or escape control characters that could break log parsing
    clean := strings.Map(func(r rune) rune {
        // Remove control characters except newline and tab
        if r < 32 && r != '\n' && r != '\t' || r > 126 {
            return -1 // Remove
        }
        return r
    }, userInput)

    // Limit length to prevent log flooding
    if len(clean) > 1000 {
        clean = clean[:1000] + " [TRUNCATED]"
    }

    return clean
}

func userAction(ctx context.Context, action string, userInput string) {
    log := logger.FromContext(ctx)

    safeAction := safeLogMessage(action)
    safeInput := safeLogMessage(userInput)

    log.Info("User action",
        "action", safeAction,
        "user_input", safeInput,
        "user_id", logger.UserIDFromContext(ctx),
    )
}
```

## Troubleshooting

### Common Issues

1. **Logs not appearing**: Check log level configuration

   ```go
   log.SetLevel(logger.LevelDebug) // Enable debug logging
   log.IsLevelEnabled(logger.LevelDebug) // Check if enabled
   ```

2. **STDOUT pollution**: Ensure proper output separation

   ```go
   // For CLI tools with JSON output:
   log.SetOutput(os.Stderr) // Logs to stderr
   // Direct stdout writes for structured output
   fmt.Fprint(os.Stdout, jsonOutput)
   ```

3. **Performance degradation**: Use level checks in hot paths

   ```go
   if log.IsLevelEnabled(logger.LevelDebug) {
       log.Debug("Expensive debug info")
   }
   ```

4. **Context losing fields**: Always use logger.FromContext

   ```go
   // ❌ Wrong
   globalLog.Debug("message")

   // ✅ Correct
   log := logger.FromContext(ctx)
   log.Debug("message")
   ```

5. **JSON parsing errors**: Ensure proper field types

   ```go
   // These will cause JSON marshaling issues:
   log.Info("bad", "cycle", &circular{}) // ❌ Circular references
   log.Info("bad", "chan", make(chan int)) // ❌ Channels not JSON serializable

   // Safe alternatives:
   log.Info("safe", "struct", safeStruct)
   log.Info("safe", "error", fmt.Sprintf("%v", err)) // Stringify errors
   ```

## Future Enhancements

- **Structured logging v2**: Support for OpenTelemetry integration
- **Log aggregation**: Built-in support for log shipping to ELK, Splunk
- **Performance**: Zero-allocation logging for common cases
- **Metrics integration**: Automatic metric emission alongside logs
- **Log replay**: Support for replaying structured logs for debugging

## Related Libraries

- [`pkg/config`](config.md) - Configuration management with logging integration
- [`pkg/exitcode`](exitcode.md) - Exit code management with error logging
- [`pkg/safeio`](safeio.md) - Safe file operations with audit logging
- [Zap](https://github.com/uber-go/zap) - High-performance structured logging (underlying implementation)

For more information, see the [GoDoc documentation](https://pkg.go.dev/github.com/fulmenhq/goneat/pkg/logger).

---

_Generated by Code Scout ([OpenCode](https://opencode.ai/)) under supervision of [@3leapsdave](https://github.com/3leapsdave)_  
_Co-Authored-By: Code Scout <noreply@3leaps.net>_
