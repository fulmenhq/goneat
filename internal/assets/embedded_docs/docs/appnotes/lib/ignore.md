---
title: Ignore Pattern Library
description: Gitignore-style pattern matching for file exclusion in Go applications.
---

# Ignore Patterns Library

Goneat's `pkg/ignore` provides a robust, high-performance pattern matching system for excluding files and directories, similar to `.gitignore` and `.dockerignore`. It's designed for build tools, linters, and file processing applications that need to respect ignore patterns.

## Purpose

File exclusion is a fundamental requirement for many Go applications that process source code or file systems:

- Build tools that skip generated files
- Linters that ignore vendor directories
- Documentation generators that skip tests
- Backup tools that exclude temporary files

The `pkg/ignore` library provides a standardized, efficient way to handle these patterns without reinventing the wheel.

## Key Features

- **Gitignore-compatible syntax**: Supports all standard `.gitignore` patterns
- **High performance**: Optimized for large file sets and complex patterns
- **Multiple sources**: Load patterns from files, strings, or embedded resources
- **Path normalization**: Handles platform-specific path separators
- **Negation support**: Include/exclude patterns with `!` prefix
- **Hierarchical matching**: Respects directory-specific ignore files

## Installation

```bash
go get github.com/fulmenhq/goneat/pkg/ignore
```

## Basic Usage

### Simple Pattern Matching

```go
package main

import (
    "fmt"
    "github.com/fulmenhq/goneat/pkg/ignore"
)

func main() {
    // Create matcher with common patterns
    patterns := []string{
        "*.log",
        "node_modules/",
        "build/",
        "*.tmp",
        "!important.log", // Don't ignore this one
    }

    matcher, err := ignore.NewMatcher(patterns)
    if err != nil {
        panic(err)
    }

    // Test individual files
    testFiles := []string{
        "app.log",
        "src/main.go",
        "node_modules/react.js",
        "build/output.js",
        "important.log",
        "temp.tmp",
    }

    for _, file := range testFiles {
        if matcher.Match(file) {
            fmt.Printf("%s: IGNORED\n", file)
        } else {
            fmt.Printf("%s: INCLUDED\n", file)
        }
    }
}
```

### Loading from Files

```go
package main

import (
    "fmt"
    "os"
    "github.com/fulmenhq/goneat/pkg/ignore"
)

func main() {
    // Load from .gitignore file
    gitignoreData, err := os.ReadFile(".gitignore")
    if err != nil {
        panic(err)
    }

    // Also load from .goneatignore if it exists
    var goneatIgnore []byte
    if data, err := os.ReadFile(".goneatignore"); err == nil {
        goneatIgnore = data
    }

    matcher, err := ignore.NewMatcherFromBytes([][]byte{gitignoreData, goneatIgnore})
    if err != nil {
        panic(err)
    }

    // Use the matcher
    if matcher.Match("vendor/") {
        fmt.Println("Vendor directory will be ignored")
    }
}
```

## API Reference

### ignore.Matcher

```go
type Matcher struct {
    // Contains compiled ignore patterns
}

func NewMatcher(patterns []string) (*Matcher, error)
func NewMatcherFromFiles(filenames ...string) (*Matcher, error)
func NewMatcherFromBytes(contents [][]byte) (*Matcher, error)
func NewMatcherFromReader(r io.Reader, filename string) (*Matcher, error)

func (m *Matcher) Match(path string) bool
func (m *Matcher) MatchDir(dir string) bool
func (m *Matcher) ShouldInclude(path string) bool // Opposite of Match
func (m *Matcher) Patterns() []string
func (m *Matcher) Reset()
```

### Pattern Types

The library supports all standard gitignore pattern types:

```go
// These patterns all work as expected
patterns := []string{
    // Exact file match
    "README.md",

    // Wildcard matching
    "*.go",
    "*.{js,ts}",

    // Directory matching
    "vendor/",
    "node_modules/",

    // Path prefix matching
    "src/internal/",

    // Negation (overrides previous patterns)
    "!public/README.md",

    // Double asterisk for recursive matching
    "**/*.log",
    "logs/**/*",

    // Parent directory matching
    "../temp/",
}
```

## Advanced Usage

### Hierarchical Ignore Files

Load ignore patterns from multiple locations based on directory structure:

```go
package main

import (
    "context"
    "fmt"
    "path/filepath"
    "github.com/fulmenhq/goneat/pkg/ignore"
)

type HierarchicalMatcher struct {
    matchers map[string]*ignore.Matcher
    baseDir  string
}

func NewHierarchicalMatcher(baseDir string) *HierarchicalMatcher {
    return &HierarchicalMatcher{
        matchers: make(map[string]*ignore.Matcher),
        baseDir:  baseDir,
    }
}

func (hm *HierarchicalMatcher) LoadForPath(ctx context.Context, relPath string) error {
    dirPath := filepath.Join(hm.baseDir, relPath)

    // Load .gitignore
    if gitignorePath := filepath.Join(dirPath, ".gitignore"); fileExists(gitignorePath) {
        if content, err := os.ReadFile(gitignorePath); err == nil {
            if matcher, err := ignore.NewMatcherFromBytes([][]byte{content}); err == nil {
                hm.matchers[relPath] = matcher
            }
        }
    }

    // Load .goneatignore
    if goneatIgnorePath := filepath.Join(dirPath, ".goneatignore"); fileExists(goneatIgnorePath) {
        if content, err := os.ReadFile(goneatIgnorePath); err == nil {
            var combined []byte
            if existing, exists := hm.matchers[relPath]; exists {
                // Combine with existing patterns
                combined = append(existing.Patterns(), "\n"+string(content)...)
            } else {
                combined = content
            }
            if matcher, err := ignore.NewMatcherFromBytes([][]byte{combined}); err == nil {
                hm.matchers[relPath] = matcher
            }
        }
    }

    return nil
}

func (hm *HierarchicalMatcher) Match(path string) bool {
    // Walk up the directory tree to find applicable patterns
    relPath := strings.TrimPrefix(path, hm.baseDir+string(filepath.Separator))
    current := filepath.Dir(relPath)

    for current != "." && current != "/" {
        if matcher, exists := hm.matchers[current]; exists {
            if matcher.Match(path) {
                return true // Ignored
            }
        }
        current = filepath.Dir(current)
    }

    // Check root patterns
    if matcher, exists := hm.matchers["."]; exists {
        if matcher.Match(path) {
            return true
        }
    }

    return false
}
```

### Performance-Optimized Batch Matching

For applications processing many files:

```go
package main

import (
    "fmt"
    "github.com/fulmenhq/goneat/pkg/ignore"
    "path/filepath"
    "sync"
)

func processFilesConcurrently(files []string, matcher *ignore.Matcher) []string {
    var wg sync.WaitGroup
    includedFiles := make([]string, 0, len(files))
    mu := sync.Mutex{}

    for _, file := range files {
        wg.Add(1)
        go func(f string) {
            defer wg.Done()
            if !matcher.Match(f) {
                mu.Lock()
                includedFiles = append(includedFiles, f)
                mu.Unlock()
            }
        }(file)
    }

    wg.Wait()
    return includedFiles
}

// Usage
func main() {
    matcher, _ := ignore.NewMatcher([]string{"*.log", "temp/*", "build/**"})

    // Process 10,000 files efficiently
    allFiles := generateFileList() // Your file discovery logic
    filesToProcess := processFilesConcurrently(allFiles, matcher)

    fmt.Printf("Processing %d files (ignored %d)\n",
        len(filesToProcess), len(allFiles)-len(filesToProcess))
}
```

## Common Patterns and Use Cases

### Build Tool Integration

```go
// Example for a Go build tool
func buildPackage(rootDir string) error {
    matcher, err := ignore.NewMatcherFromFiles(".gitignore", ".buildignore")
    if err != nil {
        return err
    }

    // Walk the directory, skipping ignored files
    return filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
        if err != nil {
            return err
        }

        // Skip if ignored
        if matcher.Match(path) {
            if d.IsDir() {
                return filepath.SkipDir
            }
            return nil
        }

        // Process the file
        if !d.IsDir() {
            if err := compileFile(path); err != nil {
                return err
            }
        }

        return nil
    })
}
```

### Linter Exclusion

```go
// Example for a code linter
type Linter struct {
    ignoreMatcher *ignore.Matcher
    includeTests  bool
}

func (l *Linter) New(rootDir string) (*Linter, error) {
    patterns := []string{
        "vendor/",
        "node_modules/",
        ".git/",
        "build/",
        "*.min.js",
    }

    if !l.includeTests {
        patterns = append(patterns, "*/test_*", "*_test.go")
    }

    matcher, err := ignore.NewMatcher(patterns)
    if err != nil {
        return nil, err
    }

    return &Linter{ignoreMatcher: matcher}, nil
}

func (l *Linter) LintFile(filePath string) error {
    if l.ignoreMatcher.Match(filePath) {
        log.Debugf("Skipping ignored file: %s", filePath)
        return nil
    }

    // Run linting logic
    return l.runLinting(filePath)
}
```

## Pattern Reference

### Basic Patterns

| Pattern          | Matches                      | Example                                             |
| ---------------- | ---------------------------- | --------------------------------------------------- |
| `file.txt`       | Exact file                   | `file.txt`                                          |
| `*.go`           | All `.go` files              | `main.go`, `utils.go`                               |
| `dir/`           | Directory and contents       | `dir/file.go`, `dir/sub/`                           |
| `**/*.log`       | All `.log` files recursively | `logs/error.log`, `tmp/access.log`                  |
| `!important.txt` | Negates previous ignore      | Includes `important.txt` even if `*.txt` is ignored |

### Advanced Patterns

```go
// Complex pattern examples
complexPatterns := []string{
    // Ignore all but explicitly include some
    "*.log",
    "!access.log",
    "!error.log",

    // Directory-specific ignores
    "src/generated/",
    "!src/generated/public/",

    // Path prefix matching
    "internal/private/",

    // Recursive exclusion with exceptions
    "**/node_modules/",
    "!**/node_modules/.bin/", // But keep executables

    // Comment lines are ignored
    "# This is a comment",
}
```

## Performance Characteristics

- **Pattern compilation**: O(n) where n is number of patterns (typically < 1ms for 100 patterns)
- **Path matching**: O(m) where m is pattern complexity (usually < 1μs per path)
- **Memory usage**: ~2KB per 100 patterns, plus minimal per-path overhead
- **Scalability**: Handles 100K+ files efficiently in batch mode

## Security Considerations

- **Pattern injection**: Validate pattern sources to prevent malicious glob patterns
- **Path traversal**: The library normalizes paths and prevents `../` traversal attacks
- **Resource exhaustion**: Limit pattern complexity to prevent regex DoS attacks
- **File system access**: Always combine with `pkg/safeio` for secure file operations

## Error Handling

### Common Errors

```go
var (
    ErrInvalidPattern = errors.New("invalid ignore pattern")
    ErrPatternTooComplex = errors.New("pattern too complex for safe matching")
    ErrEmptyPatternSet = errors.New("no valid patterns provided")
)
```

### Robust Error Handling

```go
func createMatcher(sources []string) (*ignore.Matcher, error) {
    var allPatterns []string
    var errs []error

    for _, source := range sources {
        patterns, err := loadPatternsFromSource(source)
        if err != nil {
            errs = append(errs, fmt.Errorf("failed to load %s: %w", source, err))
            continue
        }
        allPatterns = append(allPatterns, patterns...)
    }

    if len(allPatterns) == 0 && len(errs) > 0 {
        return nil, fmt.Errorf("all pattern sources failed: %v", errs)
    }

    matcher, err := ignore.NewMatcher(allPatterns)
    if err != nil {
        return nil, fmt.Errorf("failed to compile patterns: %w", err)
    }

    return matcher, nil
}
```

## Testing Your Ignore Patterns

Create comprehensive tests for your ignore patterns:

```go
func TestIgnoreMatcher(t *testing.T) {
    patterns := []string{
        "*.log",
        "temp/",
        "!keep.log",
        "build/**",
    }

    matcher, err := ignore.NewMatcher(patterns)
    require.NoError(t, err)

    testCases := []struct {
        path     string
        expected bool // true = should be ignored
    }{
        {"app.log", true},
        {"data.json", false},
        {"temp/file.txt", true},
        {"keep.log", false},     // Negation works
        {"build/output.js", true},
        {"src/main.go", false},
    }

    for _, tc := range testCases {
        t.Run(tc.path, func(t *testing.T) {
            if matcher.Match(tc.path) != tc.expected {
                t.Errorf("Expected %s to be %s, but got opposite",
                    tc.path, boolToString(!tc.expected))
            }
        })
    }
}

func boolToString(b bool) string {
    if b {
        return "ignored"
    }
    return "included"
}
```

## Integration with Other Goneat Libraries

### With pkg/pathfinder

```go
import (
    "github.com/fulmenhq/goneat/pkg/ignore"
    "github.com/fulmenhq/goneat/pkg/pathfinder"
)

func createSafeWalker(rootDir string) *pathfinder.Walker {
    // Load ignore patterns
    matcher, err := ignore.NewMatcherFromFiles(".gitignore", ".goneatignore")
    if err != nil {
        panic(err)
    }

    // Create walker that respects ignore patterns
    walker := pathfinder.NewWalker(rootDir, pathfinder.WithIgnoreMatcher(matcher))
    return walker
}
```

### With pkg/safeio

```go
import (
    "github.com/fulmenhq/goneat/pkg/ignore"
    "github.com/fulmenhq/goneat/pkg/safeio"
)

func processSourceFiles(rootDir string) error {
    ignoreMatcher, _ := ignore.NewMatcher([]string{"*.log", "temp/**"})

    files, err := safeio.ListFiles(rootDir, safeio.WithIgnoreMatcher(ignoreMatcher))
    if err != nil {
        return err
    }

    for _, file := range files {
        // Process only non-ignored files
        if !ignoreMatcher.Match(file) {
            if err := processFile(file); err != nil {
                return err
            }
        }
    }

    return nil
}
```

## Limitations

- **Pattern complexity**: Extremely complex regex patterns may impact performance
- **Unicode support**: Limited support for Unicode characters in patterns
- **Windows paths**: Path separator handling may have edge cases on Windows
- **Memory patterns**: Very large pattern sets (>10K patterns) may require optimization

## Future Enhancements

- Full Unicode pattern support
- Advanced regex pattern validation and optimization
- Caching mechanisms for frequently matched paths
- Integration with file system change notifications
- Visual pattern testing tools

## Related Libraries

- [`pkg/pathfinder`](pathfinder.md) - Safe file system traversal
- [`pkg/safeio`](safeio.md) - Secure file I/O operations
- [`pkg/config`](config.md) - Hierarchical configuration management
- [gitignore](https://git-scm.com/docs/gitignore) - Original pattern specification

For more information, see the [GoDoc documentation](https://pkg.go.dev/github.com/fulmenhq/goneat/pkg/ignore).
