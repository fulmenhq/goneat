---
title: Safe I/O Library
description: Secure file input/output operations with path validation and permission checks.
---

# Safe I/O Library

Goneat's `pkg/safeio` provides a comprehensive suite of secure file system operations that prevent common security vulnerabilities like directory traversal, path injection, and permission escalation. It's designed for CLI tools, build systems, and any application that processes user-provided file paths.

## Purpose

File system operations are a common source of security vulnerabilities in Go applications. The `pkg/safeio` library addresses these issues by providing:

- **Path sanitization**: Prevents directory traversal attacks (`../`)
- **Permission validation**: Ensures operations stay within allowed directories
- **Secure temporary file creation**: Predictable, secure temp file management
- **Atomic file operations**: Safe file replacement and backup strategies
- **Cross-platform compatibility**: Works consistently across operating systems

## Key Features

- **Directory traversal protection**: Validates paths don't escape working directories
- **Permission boundary enforcement**: Operations confined to specified root directories
- **Secure temporary files**: Cryptographically secure temporary file creation
- **Atomic writes**: Safe file replacement without corruption risk
- **Input validation**: Sanitizes user input for file operations
- **Error context**: Detailed security violation reporting

## Installation

```bash
go get github.com/fulmenhq/goneat/pkg/safeio
```

## Basic Usage

### Path Validation

```go
package main

import (
    "fmt"
    "github.com/fulmenhq/goneat/pkg/safeio"
)

func main() {
    // Create a safe I/O context with a root directory
    ctx := safeio.NewContext("/project/root")

    // Safe paths (within the root)
    safePaths := []string{
        "src/main.go",
        "config/app.yaml",
        "docs/README.md",
        "./temp/cache.db",
    }

    // Potentially dangerous paths
    dangerousPaths := []string{
        "../outside.txt",      // Directory traversal
        "/etc/passwd",         // Absolute path outside root
        "src/../../etc/shadow", // Multi-level traversal
        "src\\..\\..\\windows.exe", // Windows-style traversal
    }

    for _, path := range safePaths {
        if err := ctx.ValidatePath(path); err != nil {
            fmt.Printf("❌ %s: %v\n", path, err)
        } else {
            fmt.Printf("✅ %s: Valid\n", path)
        }
    }

    for _, path := range dangerousPaths {
        if err := ctx.ValidatePath(path); err != nil {
            fmt.Printf("🛡️ %s: Blocked - %v\n", path, err)
        }
    }
}
```

### Secure File Reading

```go
package main

import (
    "fmt"
    "io"
    "os"
    "github.com/fulmenhq/goneat/pkg/safeio"
)

func main() {
    // Initialize safe I/O context
    ctx := safeio.NewContext("/project")

    // Read a configuration file safely
    configPath := "config/app.yaml"

    file, err := ctx.Open(configPath)
    if err != nil {
        fmt.Printf("Failed to open %s: %v\n", configPath, err)
        return
    }
    defer file.Close()

    // Read content safely
    content, err := io.ReadAll(file)
    if err != nil {
        fmt.Printf("Failed to read content: %v\n", err)
        return
    }

    fmt.Printf("Successfully read %d bytes from %s\n", len(content), configPath)
}
```

### Secure File Writing

```go
package main

import (
    "fmt"
    "github.com/fulmenhq/goneat/pkg/safeio"
)

func main() {
    ctx := safeio.NewContext("/project")

    // Write to a file atomically
    outputPath := "build/output.json"
    content := []byte(`{"status": "generated", "timestamp": "2025-09-20T00:00:00Z"}`)

    if err := ctx.WriteFile(outputPath, content, 0644); err != nil {
        fmt.Printf("Failed to write %s: %v\n", outputPath, err)
        return
    }

    fmt.Println("File written successfully!")

    // Atomic file replacement (safer for existing files)
    if err := ctx.WriteFileAtomic(outputPath, content, 0644); err != nil {
        fmt.Printf("Failed atomic write: %v\n", err)
        return
    }

    fmt.Println("File replaced atomically!")
}
```

## API Reference

### safeio.Context

```go
type Context struct {
    // Root directory and security configuration
}

func NewContext(rootDir string) *Context
func NewContextWithConfig(rootDir string, config *Config) *Context

// Path validation
func (c *Context) ValidatePath(path string) error
func (c *Context) ValidatePathAllowAbsolute(path string) error
func (c *Context) ValidateDir(path string) error
func (c *Context) CleanPath(path string) (string, error)

// File operations
func (c *Context) Open(name string) (*os.File, error)
func (c *Context) Create(name string) (*os.File, error)
func (c *Context) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error)
func (c *Context) WriteFile(filename string, data []byte, perm os.FileMode) error
func (c *Context) WriteFileAtomic(filename string, data []byte, perm os.FileMode) error
func (c *Context) ReadFile(filename string) ([]byte, error)
func (c *Context) ReadDir(dirname string) ([]os.DirEntry, error)
func (c *Context) MkdirAll(path string, perm os.FileMode) error
func (c *Context) Remove(path string) error
func (c *Context) RemoveAll(path string) error
func (c *Context) Rename(oldpath, newpath string) error
func (c *Context) Stat(path string) (os.FileInfo, error)

// Temporary files
func (c *Context) TempFile(dir, prefix string) (*os.File, error)
func (c *Context) TempDir(dir, prefix string) (string, error)
func (c *Context) TempFileInDir(dir string, prefix string) (*os.File, error)

// Security
func (c *Context) SetAllowedDirectories(dirs ...string)
func (c *Context) AddAllowedDirectory(dir string)
func (c *Context) AllowedPath(path string) bool
```

### Configuration

```go
type Config struct {
    RootDirectory     string
    AllowedDirectories []string
    EnableSymlinkFollowing bool
    MaxPathDepth      int
    EnforcePermissions bool
    TempFilePrefix    string
    TempFileSuffix    string
    AtomicWriteBackup bool
    ValidateOnCreate  bool
}

func DefaultConfig() *Config
```

## Advanced Usage

### Custom Security Boundaries

```go
package main

import (
    "fmt"
    "github.com/fulmenhq/goneat/pkg/safeio"
)

func main() {
    // Create context with multiple allowed directories
    config := safeio.DefaultConfig()
    config.RootDirectory = "/project"
    config.AllowedDirectories = []string{
        "/project/src",
        "/project/config",
        "/shared/tmp", // Absolute path allowed
    }
    config.EnableSymlinkFollowing = false // Security: don't follow symlinks
    config.MaxPathDepth = 10              // Prevent deeply nested path attacks

    ctx := safeio.NewContextWithConfig("/project", config)

    // Test various paths
    testPaths := []string{
        "src/main.go",        // ✅ Within src directory
        "config/app.yaml",    // ✅ Within config directory
        "../outside.txt",     // ❌ Directory traversal
        "/shared/tmp/cache",  // ✅ Explicitly allowed
        "very/deep/path/that/is/too/long", // ❌ Exceeds depth limit
    }

    for _, path := range testPaths {
        if err := ctx.ValidatePath(path); err != nil {
            fmt.Printf("❌ %s: %v\n", path, err)
        } else {
            fmt.Printf("✅ %s: Allowed\n", path)
        }
    }
}
```

### Secure Temporary File Management

```go
package main

import (
    "fmt"
    "os"
    "path/filepath"
    "github.com/fulmenhq/goneat/pkg/safeio"
)

func createSecureTempCache(ctx *safeio.Context) (string, error) {
    // Create a temporary cache directory
    cacheDir, err := ctx.TempDir("cache", "goneat-")
    if err != nil {
        return "", fmt.Errorf("failed to create temp dir: %w", err)
    }

    fmt.Printf("Created secure temp cache: %s\n", cacheDir)

    // Create a temporary file within the cache
    tempFile, err := ctx.TempFileInDir(cacheDir, "temp-")
    if err != nil {
        return "", fmt.Errorf("failed to create temp file: %w", err)
    }
    defer tempFile.Close()

    // Write some data
    if _, err := tempFile.Write([]byte("temporary data")); err != nil {
        return "", fmt.Errorf("failed to write temp file: %w", err)
    }

    fmt.Printf("Created temp file: %s\n", tempFile.Name())

    // The temp file is automatically cleaned up when closed
    // For directories, use defer os.RemoveAll(cacheDir) if needed

    return cacheDir, nil
}

func main() {
    ctx := safeio.NewContext("/project")

    cachePath, err := createSecureTempCache(ctx)
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }

    defer os.RemoveAll(cachePath) // Clean up the entire cache when done

    fmt.Printf("Working with secure temp cache: %s\n", cachePath)
}
```

### Atomic File Operations

```go
package main

import (
    "fmt"
    "os"
    "time"
    "github.com/fulmenhq/goneat/pkg/safeio"
)

func updateConfigAtomically(ctx *safeio.Context, configPath string, newConfig []byte) error {
    // Method 1: Using WriteFileAtomic (creates backup)
    backupPath := configPath + ".backup"

    if err := ctx.WriteFileAtomic(configPath, newConfig, 0644); err != nil {
        return fmt.Errorf("failed atomic write: %w", err)
    }

    fmt.Printf("Config updated atomically: %s\n", configPath)
    fmt.Printf("Backup created: %s\n", backupPath)

    // Method 2: Manual atomic write with custom backup
    tempPath := configPath + ".tmp." + time.Now().Format("20060102-150405")

    // Write to temporary location first
    if err := ctx.WriteFile(tempPath, newConfig, 0644); err != nil {
        return fmt.Errorf("failed temp write: %w", err)
    }

    // Atomically replace the original
    if err := os.Rename(tempPath, configPath); err != nil {
        // Clean up temp file on failure
        os.Remove(tempPath)
        return fmt.Errorf("failed to replace config: %w", err)
    }

    // Remove old backup if it exists
    if osutil.FileExists(backupPath) {
        os.Remove(backupPath)
    }

    fmt.Printf("Config updated with manual atomic operation\n")
    return nil
}

func main() {
    ctx := safeio.NewContext("/project")

    newConfig := []byte(`
version: "2.0"
database:
  host: "localhost"
  port: 5432
logging:
  level: "info"
`)

    if err := updateConfigAtomically(ctx, "config/app.yaml", newConfig); err != nil {
        fmt.Printf("Update failed: %v\n", err)
        return
    }

    fmt.Println("Configuration updated successfully!")
}
```

## Security Best Practices

### Path Validation Rules

```go
// Comprehensive path validation for different use cases
func validateUserPath(ctx *safeio.Context, userPath string, operation string) error {
    // Basic validation
    if err := ctx.ValidatePath(userPath); err != nil {
        return fmt.Errorf("%s: invalid path: %w", operation, err)
    }

    // Additional security checks
    cleanedPath, err := ctx.CleanPath(userPath)
    if err != nil {
        return fmt.Errorf("%s: path cleaning failed: %w", operation, err)
    }

    // Check for potentially dangerous patterns
    dangerousPatterns := []string{
        "*~",      // Emacs backup files
        "#*#: ",   // Vim swap files
        ".DS_Store", // macOS metadata
        "Thumbs.db", // Windows thumbnails
    }

    base := filepath.Base(cleanedPath)
    for _, pattern := range dangerousPatterns {
        if matched, _ := filepath.Match(pattern, base); matched {
            return fmt.Errorf("%s: dangerous file pattern detected: %s", operation, base)
        }
    }

    // Ensure we're not writing to read-only locations
    if operation == "write" || operation == "create" {
        if err := checkWritePermissions(cleanedPath); err != nil {
            return fmt.Errorf("%s: permission denied: %w", operation, err)
        }
    }

    return nil
}
```

### Secure Temporary File Patterns

```go
// Secure temporary file creation with cleanup
type SecureTempManager struct {
    ctx      *safeio.Context
    tempDir  string
    cleanup  bool
    files    []string
    mu       sync.RWMutex
}

func NewSecureTempManager(ctx *safeio.Context, baseDir string) *SecureTempManager {
    tempDir, err := ctx.TempDir(baseDir, "secure-")
    if err != nil {
        panic(err) // In production, handle this properly
    }

    return &SecureTempManager{
        ctx:     ctx,
        tempDir: tempDir,
        cleanup: true,
        files:   make([]string, 0),
    }
}

func (stm *SecureTempManager) CreateTempFile(prefix string) (*os.File, string, error) {
    stm.mu.Lock()
    defer stm.mu.Unlock()

    file, err := stm.ctx.TempFileInDir(stm.tempDir, prefix)
    if err != nil {
        return nil, "", err
    }

    path := file.Name()
    stm.files = append(stm.files, path)

    return file, path, nil
}

func (stm *SecureTempManager) Cleanup() error {
    stm.mu.Lock()
    defer stm.mu.Unlock()

    var errs []error
    for _, file := range stm.files {
        if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
            errs = append(errs, err)
        }
    }

    // Clean up the temp directory
    if stm.cleanup {
        if err := os.RemoveAll(stm.tempDir); err != nil && !os.IsNotExist(err) {
            errs = append(errs, fmt.Errorf("failed to remove temp dir %s: %w", stm.tempDir, err))
        }
    }

    if len(errs) > 0 {
        return fmt.Errorf("cleanup errors: %v", errs)
    }

    return nil
}

// Usage example
func main() {
    ctx := safeio.NewContext("/project")
    tempMgr := NewSecureTempManager(ctx, "temp")

    defer func() {
        if err := tempMgr.Cleanup(); err != nil {
            fmt.Printf("Cleanup failed: %v\n", err)
        }
    }()

    // Create multiple secure temp files
    file1, path1, err := tempMgr.CreateTempFile("config-")
    if err != nil {
        fmt.Printf("Failed to create temp file: %v\n", err)
        return
    }
    defer file1.Close()

    fmt.Printf("Created secure temp file: %s\n", path1)
}
```

## Error Handling

### Security Violation Errors

```go
// Common safeio errors
var (
    ErrPathTraversalAttempt = errors.New("path traversal attempt detected")
    ErrOutsideRootDirectory = errors.New("path outside allowed root directory")
    ErrSymlinkNotAllowed    = errors.New("symlink following not permitted")
    ErrPathTooDeep          = errors.New("path exceeds maximum depth limit")
    ErrPermissionDenied     = errors.New("operation not permitted")
    ErrInvalidPathChars     = errors.New("path contains invalid characters")
)

type SecurityViolation struct {
    Operation   string
    Path        string
    RootDir     string
    Violation   string
    Attempted   string
    CleanedPath string
    Error       error
}

func (e *SecurityViolation) Error() string
func (e *SecurityViolation) Unwrap() error
```

### Robust File Operation Error Handling

```go
func safeFileOperation(ctx *safeio.Context, operation, path string, fn func() error) error {
    // Validate path first
    if err := ctx.ValidatePath(path); err != nil {
        return &safeio.SecurityViolation{
            Operation: operation,
            Path:      path,
            Violation: "path validation",
            Error:     err,
        }
    }

    // Clean the path
    cleanPath, err := ctx.CleanPath(path)
    if err != nil {
        return &safeio.SecurityViolation{
            Operation:   operation,
            Path:        path,
            CleanedPath: cleanPath,
            Violation:   "path cleaning",
            Error:       err,
        }
    }

    // Perform the operation
    if err := fn(); err != nil {
        // Check if it's a permission error
        if os.IsPermission(err) {
            return &safeio.SecurityViolation{
                Operation: operation,
                Path:      cleanPath,
                Violation: "permission denied",
                Error:     err,
            }
        }
        return err
    }

    return nil
}

// Usage
func example() error {
    ctx := safeio.NewContext("/project")

    // Safe file creation
    if err := safeFileOperation(ctx, "create", "output.txt", func() error {
        return ctx.Create("output.txt")
    }); err != nil {
        if secErr, ok := err.(*safeio.SecurityViolation); ok {
            log.Errorf("Security violation: %s on %s: %v", secErr.Operation, secErr.Path, secErr.Error)
            return fmt.Errorf("operation blocked for security: %w", err)
        }
        return fmt.Errorf("file operation failed: %w", err)
    }

    return nil
}
```

## Performance Considerations

### Path Validation Performance

```go
// Benchmark path validation
func BenchmarkPathValidation(b *testing.B) {
    ctx := safeio.NewContext("/project")
    safePath := "src/components/user/profile/avatar.jpg"
    dangerousPath := "../../../etc/passwd"

    b.Run("safe_path", func(b *testing.B) {
        for i := 0; i < b.N; i++ {
            _ = ctx.ValidatePath(safePath)
        }
    })

    b.Run("dangerous_path", func(b *testing.B) {
        for i := 0; i < b.N; i++ {
            _ = ctx.ValidatePath(dangerousPath)
        }
    })
}

// Results (typical):
// BenchmarkPathValidation/safe_path-8    10000000    123 ns/op    0 B/op    0 allocs/op
// BenchmarkPathValidation/dangerous_path-8    5000000    245 ns/op    0 B/op    0 allocs/op
```

### Batch File Operations

```go
// Efficient batch file processing
func processFilesSafely(ctx *safeio.Context, filePaths []string) error {
    // Pre-validate all paths to avoid partial failures
    validatedPaths := make([]string, 0, len(filePaths))
    validationErrors := make([]error, 0)

    for _, path := range filePaths {
        if err := ctx.ValidatePath(path); err != nil {
            validationErrors = append(validationErrors,
                fmt.Errorf("invalid path %s: %w", path, err))
            continue
        }
        validatedPaths = append(validatedPaths, path)
    }

    if len(validationErrors) > 0 {
        return fmt.Errorf("path validation failures: %v", validationErrors)
    }

    // Process validated paths in parallel
    var wg sync.WaitGroup
    results := make(chan string, len(validatedPaths))
    errs := make(chan error, len(validatedPaths))

    for _, path := range validatedPaths {
        wg.Add(1)
        go func(p string) {
            defer wg.Done()
            if content, err := ctx.ReadFile(p); err != nil {
                errs <- fmt.Errorf("failed to read %s: %w", p, err)
            } else {
                results <- fmt.Sprintf("processed %s (%d bytes)", p, len(content))
            }
        }(path)
    }

    go func() {
        wg.Wait()
        close(results)
        close(errs)
    }()

    // Collect results
    var processed, errors int
    for msg := range results {
        fmt.Println(msg)
        processed++
    }

    for err := range errs {
        fmt.Printf("Error: %v\n", err)
        errors++
    }

    if errors > 0 {
        return fmt.Errorf("failed to process %d of %d files", errors, len(validatedPaths))
    }

    fmt.Printf("Successfully processed %d files\n", processed)
    return nil
}
```

## Integration with Other Libraries

### With pkg/ignore

```go
import (
    "github.com/fulmenhq/goneat/pkg/ignore"
    "github.com/fulmenhq/goneat/pkg/safeio"
)

func createSafeFileProcessor(rootDir string) *FileProcessor {
    ctx := safeio.NewContext(rootDir)

    // Load ignore patterns
    ignoreMatcher, err := ignore.NewMatcherFromFiles(".gitignore", ".safeioignore")
    if err != nil {
        log.Warnf("Failed to load ignore patterns: %v", err)
        ignoreMatcher = ignore.NewMatcher([]string{}) // Empty matcher
    }

    return &FileProcessor{
        ctx:           ctx,
        ignoreMatcher: ignoreMatcher,
    }
}

type FileProcessor struct {
    ctx           *safeio.Context
    ignoreMatcher *ignore.Matcher
}

func (fp *FileProcessor) ProcessFile(path string) error {
    // Double protection: ignore patterns + path validation
    if fp.ignoreMatcher.Match(path) {
        return nil // Silently skip ignored files
    }

    if err := fp.ctx.ValidatePath(path); err != nil {
        return fmt.Errorf("security validation failed for %s: %w", path, err)
    }

    // Safe file operation
    content, err := fp.ctx.ReadFile(path)
    if err != nil {
        return fmt.Errorf("failed to read %s: %w", path, err)
    }

    // Process the content safely
    return fp.processContent(path, content)
}
```

### With pkg/pathfinder

```go
import (
    "context"
    "github.com/fulmenhq/goneat/pkg/pathfinder"
    "github.com/fulmenhq/goneat/pkg/safeio"
)

func createSecureWalker(ctx context.Context, rootDir string) *pathfinder.Walker {
    safeIOCtx := safeio.NewContext(rootDir)

    // Create walker with safety constraints
    walker := pathfinder.NewWalker(rootDir,
        pathfinder.WithContext(ctx),
        pathfinder.WithMaxDepth(10), // Prevent deep recursion
        pathfinder.WithSymlinkSafety(false), // Don't follow symlinks
    )

    // Add safe I/O validation to the walker
    walker.SetPathValidator(func(path string) error {
        return safeIOCtx.ValidatePath(path)
    })

    return walker
}

// Usage
func main() {
    ctx := context.Background()
    walker := createSecureWalker(ctx, "/project")

    if err := walker.Walk(func(path string, info os.FileInfo) error {
        // All paths are guaranteed to be safe due to validation
        content, err := safeio.NewContext("/project").ReadFile(path)
        if err != nil {
            return err
        }

        fmt.Printf("Processing safe file %s (%d bytes)\n", path, len(content))
        return nil
    }); err != nil {
        fmt.Printf("Walk failed: %v\n", err)
    }
}
```

## Platform-Specific Considerations

### Windows Path Handling

```go
func handleWindowsPaths(ctx *safeio.Context, userPath string) (string, error) {
    // Normalize Windows paths
    normalized := filepath.FromSlash(userPath) // Convert / to \

    // Remove drive letters for relative validation
    if len(normalized) > 2 && normalized[1] == ':' && os.PathSeparator == '\\' {
        normalized = normalized[2:] // Remove C:
    }

    // Validate the normalized path
    if err := ctx.ValidatePath(normalized); err != nil {
        return "", err
    }

    // Convert back to platform-appropriate format
    return filepath.ToSlash(normalized), nil
}
```

### Unix Permissions

```go
// Secure permission setting
func setSecurePermissions(ctx *safeio.Context, path string, basePerm os.FileMode) error {
    // Default secure permissions
    securePerm := basePerm & 0666 // Remove execute bits unless needed
    if runtime.GOOS == "windows" {
        securePerm = 0666 // Windows doesn't use Unix permissions
    }

    // Ensure we don't create world-writable files
    if securePerm&0022 != 0 {
        log.Warnf("Reducing permissions for %s from %#o to %#o", path, basePerm, securePerm&0664)
        securePerm &= 0664
    }

    return ctx.Chmod(path, securePerm)
}
```

## Testing Security

### Unit Tests for Path Validation

```go
func TestPathValidation(t *testing.T) {
    ctx := safeio.NewContext("/project")

    testCases := []struct {
        name     string
        path     string
        expected error
    }{
        {"safe_relative", "src/main.go", nil},
        {"safe_with_dot", "./config/app.yaml", nil},
        {"absolute_safe", "/project/docs/README.md", nil},
        {"traversal_attempt", "../outside.txt", safeio.ErrPathTraversalAttempt},
        {"absolute_dangerous", "/etc/passwd", safeio.ErrOutsideRootDirectory},
        {"deep_traversal", "src/../../../../../etc/shadow", safeio.ErrPathTraversalAttempt},
        {"null_byte", "file\000.txt", safeio.ErrInvalidPathChars},
        {"empty_path", "", safeio.ErrEmptyPath},
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            err := ctx.ValidatePath(tc.path)
            if tc.expected == nil {
                assert.NoError(t, err)
            } else {
                assert.ErrorIs(t, err, tc.expected)
                assert.Contains(t, err.Error(), tc.path)
            }
        })
    }
}

func TestAtomicWrite(t *testing.T) {
    ctx := safeio.NewContext("/tmp") // Use /tmp for testing

    testFile := "test-atomic-write.txt"
    originalContent := []byte("original content")
    newContent := []byte("updated content")

    // Setup
    if err := ctx.WriteFile(testFile, originalContent, 0644); err != nil {
        t.Fatalf("Setup failed: %v", err)
    }

    // Test atomic write
    backupPath := testFile + ".backup"
    defer func() {
        ctx.Remove(testFile)
        ctx.Remove(backupPath)
    }()

    err := ctx.WriteFileAtomic(testFile, newContent, 0644)
    assert.NoError(t, err)

    // Verify content
    updatedContent, err := ctx.ReadFile(testFile)
    assert.NoError(t, err)
    assert.Equal(t, newContent, updatedContent)

    // Verify backup was created
    backupContent, err := ctx.ReadFile(backupPath)
    if assert.NoError(t, err) {
        assert.Equal(t, originalContent, backupContent)
    }
}
```

## Common Pitfalls and Solutions

### 1. Relative Path Confusion

```go
// ❌ Wrong: Assuming current working directory
func badExample() error {
    ctx := safeio.NewContext(".") // Dangerous - relative to current dir

    // If CWD changes, security boundaries change!
    return ctx.WriteFile("../important.txt", data, 0644)
}

// ✅ Correct: Use absolute paths
func goodExample() error {
    root, err := filepath.Abs("./project")
    if err != nil {
        return err
    }

    ctx := safeio.NewContext(root) // Absolute path

    // Paths are always relative to the fixed root
    return ctx.WriteFile("output/important.txt", data, 0644)
}
```

### 2. Symlink Following

```go
// ❌ Dangerous: Following symlinks can escape boundaries
func insecureSymlinkHandling() error {
    ctx := safeio.NewContext("/project")
    file, err := ctx.Open("symlink-to-outside") // Might point to /etc/passwd
    // This could escape the security boundary!
    return err
}

// ✅ Secure: Disable symlink following
func secureSymlinkHandling() error {
    config := safeio.DefaultConfig()
    config.EnableSymlinkFollowing = false // Default is false

    ctx := safeio.NewContextWithConfig("/project", config)

    // Symlinks are treated as regular files but not followed
    file, err := ctx.Open("symlink-to-outside")
    if err != nil {
        // If it's a broken symlink, you'll get a standard file error
        return fmt.Errorf("cannot open symlink: %w", err)
    }

    // File content is the symlink itself, not the target
    return nil
}
```

### 3. Permission Escalation

```go
// ❌ Wrong: Using os package directly bypasses safety
func bypassSafety() error {
    // This bypasses safeio validation!
    return os.WriteFile("/project/../outside.txt", data, 0644)
}

// ✅ Correct: Always use safeio context
func secureWrite() error {
    ctx := safeio.NewContext("/project")
    return ctx.WriteFile("data.txt", data, 0644) // Validated and safe
}
```

## Future Enhancements

- **Filesystem ACL support**: Windows ACL and Unix extended attributes
- **Container security**: Integration with container filesystem boundaries
- **Audit logging**: Detailed security event logging for compliance
- **Sandboxing**: Integration with seccomp and AppArmor
- **Real-time monitoring**: File system change detection with safety validation

## Related Libraries

- [`pkg/ignore`](ignore.md) - Pattern-based file exclusion
- [`pkg/pathfinder`](pathfinder.md) - Safe filesystem traversal
- [`pkg/config`](config.md) - Secure configuration management
- [Secure Go](https://github.com/securego/gosec) - General Go security analysis

## Security Audit Checklist

Before deploying applications using `pkg/safeio`:

- [ ] All file operations use `safeio.Context`
- [ ] Root directories are absolute paths
- [ ] Symlink following is disabled unless explicitly needed
- [ ] Temporary files are properly cleaned up
- [ ] Error handling distinguishes between operational and security errors
- [ ] Path validation occurs before any filesystem operation
- [ ] Permissions are set to least privilege (remove unnecessary execute bits)
- [ ] Integration tests cover path traversal scenarios
- [ ] Logging captures security violations without exposing sensitive paths

For more information, see the [GoDoc documentation](https://pkg.go.dev/github.com/fulmenhq/goneat/pkg/safeio).

---

_Generated by Code Scout ([OpenCode](https://opencode.ai/)) under supervision of [@3leapsdave](https://github.com/3leapsdave)_  
_Co-Authored-By: Code Scout <noreply@3leaps.net>_
