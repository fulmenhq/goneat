# Go Coding Standards for Goneat

**Version**: 1.0.0
**Date**: September 15, 2025
**Status**: Active
**Author**: Forge Neat (@forge-neat)
**Scope**: goneat project - All Go code and assessment runners

---

## Overview

This document establishes coding standards for the goneat project, ensuring consistency, quality, and adherence to enterprise-grade practices. As a multi-function code quality tool designed for scale, goneat requires rigorous standards to maintain its reliability and JSON-first output integrity.

**Core Principle**: Write idiomatic Go code that is simple, readable, and maintainable, with strict STDOUT hygiene for JSON output integrity.

---

## 1. Critical Rules (Zero-Tolerance)

### 1.1 STDOUT Hygiene ⚠️ **CRITICAL**

**Rule**: STDOUT must remain clean for JSON output and CLI tools that consume goneat's structured output.

**DO**: Use logger package for all output

```go
import "github.com/fulmenhq/goneat/pkg/logger"

// ✅ Correct logging
logger.Debug("repo-status: detected uncommitted files")
logger.Info("Assessment completed in 50ms: 3 issues found")
logger.Error("Failed to open git repository: %v", err)
logger.Warn("Configuration file not found, using defaults")
```

**DO NOT**: Pollute STDOUT with any direct output

```go
// ❌ CRITICAL ERROR: Breaks JSON output
fmt.Printf("DEBUG: Creating issue for %d files\n", count)
fmt.Println("Processing files...")
log.Printf("Status: %v", status)
println("Debug info")

// ❌ These break structured output consumed by CI/CD tools
os.Stdout.WriteString("Status message\n")
```

**Why Critical**: Goneat produces JSON-first output consumed by:

- CI/CD pipelines expecting clean JSON
- Automated tools parsing assessment results
- Agentic systems processing structured data
- Pre-commit/pre-push hooks expecting parseable output

**Enforcement**: Any `fmt.Print*`, `println`, or direct STDOUT writes in assessment runners or core logic will fail code review.

### 1.2 Error Handling

**DO**: Always handle errors explicitly

```go
// ✅ Proper error handling
result, err := runner.Assess(ctx, target, config)
if err != nil {
    return fmt.Errorf("assessment failed for %s: %w", category, err)
}
```

**DO NOT**: Ignore errors or use blank identifiers unnecessarily

```go
// ❌ Never ignore errors
result, _ := runner.Assess(ctx, target, config)

// ❌ Don't ignore critical errors
os.ReadFile(configFile) // Missing error check
```

### 1.3 Assessment Runner Contract

All assessment runners MUST implement the interface correctly:

```go
type AssessmentRunner interface {
    Assess(ctx context.Context, target string, config AssessmentConfig) (*AssessmentResult, error)
    CanRunInParallel() bool
    GetCategory() AssessmentCategory
    GetEstimatedTime(target string) time.Duration
    IsAvailable() bool
}
```

**Success Flag Logic**:

```go
// ✅ Correct success determination
success := len(issues) == 0

return &AssessmentResult{
    Success: success,
    Issues:  issues,
    // ...
}
```

---

## 2. Code Organization and Structure

### 2.1 Project Structure

Follow goneat's established structure:

```
goneat/
├── cmd/                    # CLI commands
├── internal/
│   ├── assess/            # Assessment engine and runners
│   ├── gitctx/           # Git context utilities
│   ├── maturity/         # Maturity validation
│   └── assets/           # Embedded assets
├── pkg/
│   ├── buildinfo/        # Build information
│   ├── logger/           # Logging utilities
│   └── schema/           # Schema validation
└── schemas/              # JSON schemas
```

### 2.2 Assessment Runners

**Location**: `internal/assess/`
**Naming**: `{category}_runner.go` (e.g., `repo_status_runner.go`)

**Template Structure**:

```go
package assess

import (
    "context"
    "time"
    "github.com/fulmenhq/goneat/pkg/logger"
)

type {Category}Runner struct{}

func New{Category}Runner() *{Category}Runner {
    return &{Category}Runner{}
}

func (r *{Category}Runner) Assess(ctx context.Context, target string, config AssessmentConfig) (*AssessmentResult, error) {
    startTime := time.Now()
    var issues []Issue

    // Assessment logic here
    // Use logger.Debug() for debugging, not fmt.Printf()

    success := len(issues) == 0

    return &AssessmentResult{
        CommandName:   "{category}",
        Category:      r.GetCategory(),
        Success:       success,
        Issues:        issues,
        ExecutionTime: HumanReadableDuration(time.Since(startTime)),
    }, nil
}

// Required interface methods...
```

### 2.3 Naming Conventions

- **Types**: PascalCase (e.g., `AssessmentRunner`, `CategoryResult`)
- **Functions**: camelCase (e.g., `validateConfig`, `buildMetrics`)
- **Constants**: PascalCase for exported, camelCase for unexported
- **Files**: snake_case with descriptive names (e.g., `repo_status_runner.go`)

---

## 3. Logging and Output Standards

### 3.1 Logging Levels

```go
// Debug: Detailed information for troubleshooting
logger.Debug("repo-status: checking %d files for uncommitted changes", fileCount)

// Info: General operational messages
logger.Info("Assessment completed in %v: %d issues found", duration, issueCount)

// Warn: Warning conditions that don't stop execution
logger.Warn("Configuration file not found, using defaults")

// Error: Error conditions that may cause failures
logger.Error("Failed to open git repository: %v", err)
```

### 3.2 Structured Logging Context

Include relevant context in log messages:

```go
// ✅ Good: Contextual information
logger.Info("Assessment completed",
    zap.String("category", "repo-status"),
    zap.Duration("duration", elapsed),
    zap.Int("issues_found", len(issues)),
)

// ❌ Bad: Missing context
logger.Info("Assessment completed")
```

### 3.3 JSON Output Integrity

Never contaminate JSON output streams:

```go
// ✅ Correct: Clean JSON output
func (e *AssessmentEngine) GenerateReport() (*AssessmentReport, error) {
    // All logging goes to stderr via logger package
    logger.Debug("generating assessment report")

    report := &AssessmentReport{
        Metadata: metadata,
        Summary:  summary,
        // ...
    }

    return report, nil // Clean return for JSON marshaling
}
```

---

## 4. Concurrency and Performance

### 4.1 Goroutine Management

Use proper synchronization for concurrent operations:

```go
// ✅ Proper goroutine management
func (e *Engine) runConcurrentAssessments(categories []Category) {
    var wg sync.WaitGroup
    results := make(chan CategoryResult, len(categories))

    for _, category := range categories {
        wg.Add(1)
        go func(cat Category) {
            defer wg.Done()
            result := e.runCategory(cat)
            results <- result
        }(category)
    }

    wg.Wait()
    close(results)
}
```

### 4.2 Context Handling

Always respect context cancellation:

```go
func (r *SecurityRunner) Assess(ctx context.Context, target string, config AssessmentConfig) (*AssessmentResult, error) {
    // Check context early
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    default:
    }

    // Long-running operations should check context periodically
    for _, file := range files {
        select {
        case <-ctx.Done():
            return nil, ctx.Err()
        default:
            // Process file
        }
    }
}
```

---

## 5. Testing Standards

### 5.1 Test Organization

```
internal/assess/
├── repo_status_runner.go
├── repo_status_runner_test.go    # Unit tests
└── testdata/                     # Test fixtures
    └── git-repos/
        ├── clean/
        └── dirty/
```

### 5.2 Table-Driven Tests

Use table-driven tests for assessment runners:

```go
func TestRepoStatusRunner_Assess(t *testing.T) {
    tests := []struct {
        name           string
        repoState      string
        expectedIssues int
        expectedSuccess bool
    }{
        {"clean_repo", "clean", 0, true},
        {"dirty_repo", "dirty", 1, false},
        {"staged_files", "staged", 1, false},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            runner := NewRepoStatusRunner()
            result, err := runner.Assess(ctx, testRepo, config)

            assert.NoError(t, err)
            assert.Equal(t, tt.expectedSuccess, result.Success)
            assert.Len(t, result.Issues, tt.expectedIssues)
        })
    }
}
```

### 5.3 Test Data Management

Use `testdata/` directories for fixtures:

```go
func setupTestRepo(t *testing.T, state string) string {
    t.Helper()
    testDir := filepath.Join("testdata", "git-repos", state)
    // Setup test repository
    return testDir
}
```

---

## 6. Security and Validation

### 6.1 Input Validation

Validate all inputs, especially file paths:

```go
func validateTarget(target string) error {
    // Ensure target is within expected bounds
    absTarget, err := filepath.Abs(target)
    if err != nil {
        return fmt.Errorf("invalid target path: %w", err)
    }

    // Check for path traversal
    if strings.Contains(absTarget, "..") {
        return errors.New("path traversal detected")
    }

    return nil
}
```

### 6.2 File Operations

Use secure file operations with proper permissions:

```go
// ✅ Secure file operations
func writeConfigFile(filename string, data []byte) error {
    // Create with restrictive permissions
    return os.WriteFile(filename, data, 0640)
}
```

---

## 7. Assessment Runner Best Practices

### 7.1 Issue Creation

Create consistent, actionable issues:

```go
func createIssue(file string, severity Severity, message string, category AssessmentCategory) Issue {
    return Issue{
        File:        file,
        Line:        0, // Set if specific line
        Severity:    severity,
        Message:     message,
        Category:    category,
        AutoFixable: false, // Set true if fixable
    }
}
```

### 7.2 Metrics Collection

Include useful metrics for reporting:

```go
metrics := map[string]interface{}{
    "files_checked":     fileCount,
    "issues_found":      len(issues),
    "execution_time_ms": time.Since(startTime).Milliseconds(),
}

// Add category-specific metrics
if categorySpecificData != nil {
    metrics["specific_metric"] = categorySpecificData
}
```

### 7.3 Error Recovery

Handle errors gracefully without stopping other assessments:

```go
func (r *Runner) Assess(ctx context.Context, target string, config AssessmentConfig) (*AssessmentResult, error) {
    defer func() {
        if r := recover(); r != nil {
            logger.Error("Assessment runner panic recovered: %v", r)
        }
    }()

    // Assessment logic with proper error handling
}
```

---

## 8. Common Anti-Patterns to Avoid

### 8.1 Output Contamination

```go
// ❌ NEVER: Contaminates JSON output
fmt.Printf("DEBUG: Processing %s\n", filename)

// ✅ ALWAYS: Use logger
logger.Debug("processing file", zap.String("filename", filename))
```

### 8.2 Hardcoded Values

```go
// ❌ Bad: Hardcoded paths
configPath := "/home/user/.goneat/config.yaml"

// ✅ Good: Dynamic paths
configPath := filepath.Join(homeDir, ".goneat", "config.yaml")
```

### 8.3 Ignored Errors

```go
// ❌ Bad: Ignored error
file, _ := os.Open(filename)

// ✅ Good: Proper error handling
file, err := os.Open(filename)
if err != nil {
    return fmt.Errorf("failed to open file %s: %w", filename, err)
}
defer file.Close()
```

---

## 9. Code Review Checklist

Before submitting code, verify:

- [ ] No `fmt.Print*` or direct STDOUT writes in assessment logic
- [ ] All errors are properly handled and wrapped
- [ ] Logger is used for all debug/info/error output
- [ ] Tests cover happy path and error conditions
- [ ] Assessment runners implement the interface correctly
- [ ] Success flag logic is correct (`success := len(issues) == 0`)
- [ ] Context cancellation is respected in long-running operations
- [ ] File operations use proper permissions
- [ ] No hardcoded paths or values

---

## 10. Tools and Enforcement

### 10.1 Required Tools

- `golangci-lint` with goneat configuration
- `go fmt` for consistent formatting
- `go vet` for static analysis

### 10.2 Pre-commit Hooks

Use goneat's own hooks to enforce standards:

```bash
./dist/goneat hooks init
./dist/goneat hooks generate
./dist/goneat hooks install
```

### 10.3 CI Integration

Ensure CI pipelines check for STDOUT contamination:

```bash
# Check for forbidden patterns
if grep -r "fmt\.Print" internal/assess/; then
    echo "ERROR: fmt.Print* found in assessment code"
    exit 1
fi
```

---

## Conclusion

These standards ensure goneat maintains its reliability as a production-grade code quality tool. The emphasis on STDOUT hygiene is critical for goneat's JSON-first architecture and integration with automated systems.

**Remember**: Clean STDOUT = Clean JSON = Happy CI/CD pipelines.

_Adherence to these standards ensures enterprise-grade reliability and seamless integration across development workflows._
