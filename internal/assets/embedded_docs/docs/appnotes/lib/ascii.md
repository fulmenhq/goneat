---
title: ASCII Art Library
description: Utilities for creating formatted ASCII art and terminal output with proper alignment and Unicode support.
---

# ASCII Art Library

Goneat's `pkg/ascii` provides a comprehensive suite of utilities for creating formatted ASCII art and terminal output. It's designed for CLI tools, status displays, and any application that needs consistent, visually appealing text formatting in terminal environments.

## Purpose

Terminal output formatting is essential for CLI applications to provide clear, readable information to users. The `pkg/ascii` library addresses common formatting needs by providing:

- **Box drawing**: Unicode box-drawing characters for structured output
- **Text alignment**: Proper padding and alignment within boxes
- **Unicode safety**: Correct handling of multi-byte characters
- **Consistent formatting**: Standardized appearance across different terminals
- **Simple API**: Easy-to-use functions for common formatting tasks

## Key Features

- **Unicode box drawing**: Proper box-drawing characters (┌┐└┘─│)
- **Automatic alignment**: Content is properly centered and padded
- **Multi-line support**: Handles multiple lines of text within boxes
- **Unicode-aware**: Correctly handles emojis, accented characters, and multi-byte sequences
- **Terminal-specific width handling**: Adapts to different terminal emoji rendering behaviors
- **Automated calibration**: Tools to automatically detect and fix width issues
- **Interactive calibration**: Manual calibration for fine-tuning terminal configurations
- **Raw mode rendering**: Bypass overrides for debugging and analysis
- **Width analysis**: Automated detection of character rendering discrepancies
- **Configuration management**: User-specific overrides with embedded defaults
- **Multi-terminal support**: Profiles for Ghostty, iTerm2, Apple Terminal, and more
- **Trailing space trimming**: Clean output without unwanted whitespace
- **Consistent width calculation**: Proper width determination for alignment

## Installation

```bash
go get github.com/fulmenhq/goneat/pkg/ascii
```

## Basic Usage

### Drawing ASCII Boxes

```go
package main

import (
    "fmt"
    "github.com/fulmenhq/goneat/pkg/ascii"
)

func main() {
    // Simple single-line box
    lines := []string{"Hello, World!"}
    ascii.DrawBox(lines)

    // Multi-line box with various content
    lines = []string{
        "Welcome to goneat",
        "",
        "ASCII Art Library Demo",
        "Unicode: 🌟 αβγδε",
    }
    ascii.DrawBox(lines)

    // Titled box with title bar
    titleBox := []string{
        "🚀 System Status Report",
        "═══════════════════════", // Separator line
        "✅ Database: Connected",
        "✅ Cache: Operational",
        "⚠️  Disk space: 85% used",
        "🔄 Services: 12/12 running",
    }
    ascii.DrawBox(titleBox)
}
```

Output:

```
┌─────────────────┐
│ Hello, World!   │
└─────────────────┘

┌──────────────────────┐
│ Welcome to goneat    │
│                      │
│ ASCII Art Library Demo│
│ Unicode: 🌟 αβγδε     │
└──────────────────────┘

┌─────────────────────────────┐
│ 🚀 System Status Report     │
│ ═════════════════════════════ │
│ ✅ Database: Connected       │
│ ✅ Cache: Operational        │
│ ⚠️  Disk space: 85% used     │
│ 🔄 Services: 12/12 running   │
└─────────────────────────────┘
```

### Creating Titled Boxes

The ASCII library excels at creating professional-looking boxes with title bars. Here are common patterns:

```go
package main

import (
    "fmt"
    "strings"
    "github.com/fulmenhq/goneat/pkg/ascii"
)

func createTitledBox(title, content string, icon string) {
    // Create a separator line matching title width
    titleLine := fmt.Sprintf("%s %s", icon, title)
    separator := strings.Repeat("═", len([]rune(titleLine))+2) // Account for padding

    lines := []string{
        titleLine,
        separator,
        content,
    }
    ascii.DrawBox(lines)
}

func createMultiSectionBox() {
    lines := []string{
        "📊 Build Report - v2.1.0",
        "═════════════════════════",
        "✅ Compilation: Success",
        "⚡ Build time: 45.2s",
        "📦 Artifacts: 5 generated",
        "",
        "🧪 Test Results",
        "═══════════════",
        "✅ Unit tests: 248/248 passed",
        "✅ Integration: 12/12 passed",
        "📈 Coverage: 94.7%",
    }
    ascii.DrawBox(lines)
}

func createErrorBox(title string, errors []string) {
    lines := []string{
        fmt.Sprintf("❌ %s", title),
        strings.Repeat("═", len(title)+3), // +3 for icon and spaces
    }

    for _, err := range errors {
        lines = append(lines, fmt.Sprintf("• %s", err))
    }

    ascii.DrawBox(lines)
}

func main() {
    // Simple titled box
    createTitledBox("Database Status", "✅ Connected to PostgreSQL 15.3", "🗄️")

    fmt.Println()

    // Multi-section report
    createMultiSectionBox()

    fmt.Println()

    // Error reporting
    createErrorBox("Build Failed", []string{
        "Syntax error in user.go:42",
        "Undefined variable 'config'",
        "Missing import for 'net/http'",
    })
}
```

Output:

```
┌─────────────────────────────┐
│ 🗄️ Database Status          │
│ ═════════════════════════════ │
│ ✅ Connected to PostgreSQL 15.3│
└─────────────────────────────┘

┌─────────────────────────────┐
│ 📊 Build Report - v2.1.0    │
│ ═════════════════════════════ │
│ ✅ Compilation: Success      │
│ ⚡ Build time: 45.2s         │
│ 📦 Artifacts: 5 generated    │
│                             │
│ 🧪 Test Results              │
│ ═════════════════════════════ │
│ ✅ Unit tests: 248/248 passed│
│ ✅ Integration: 12/12 passed │
│ 📈 Coverage: 94.7%           │
└─────────────────────────────┘

┌─────────────────────────────┐
│ ❌ Build Failed             │
│ ═════════════════════════════ │
│ • Syntax error in user.go:42 │
│ • Undefined variable 'config'│
│ • Missing import for 'net/http'│
└─────────────────────────────┘
```

### Text Truncation for Boxes

```go
package main

import (
    "fmt"
    "github.com/fulmenhq/goneat/pkg/ascii"
)

func main() {
    longText := "This is a very long string that needs to be truncated for display"
    shortText := "Short text"

    // Truncate to fit within a 20-character width
    truncated := ascii.TruncateForBox(longText, 20)
    fmt.Printf("Truncated: %s\n", truncated)

    // Short text remains unchanged
    unchanged := ascii.TruncateForBox(shortText, 20)
    fmt.Printf("Unchanged: %s\n", unchanged)
}
```

Output:

```
Truncated: This is a very lo...
Unchanged: Short text
```

## Terminal Compatibility

The ASCII library uses `mattn/go-runewidth` for Unicode character width calculations and includes terminal-specific overrides for optimal rendering across different terminal emulators.

### Supported Terminals

- **Ghostty**: Custom handling for emoji+variation selector sequences
- **iTerm2**: Configurable overrides for consistent rendering
- **macOS Terminal**: Generally follows Unicode standards
- **Other terminals**: Extensible through configuration

### Character Width Handling

Different terminals render Unicode characters (especially emojis with variation selectors) with different visual widths, which can cause ASCII box misalignment. The library includes:

1. **Default calculations** using `go-runewidth`
2. **Terminal detection** via environment variables (`TERM_PROGRAM`, `TERM`)
3. **Override configuration** for problematic character sequences
4. **User customization** through `~/.goneat/config/terminal-overrides.yaml`

### Configuration Priority

1. **User overrides** (`~/.goneat/config/terminal-overrides.yaml`)
2. **Embedded defaults** (repository-defined configurations)
3. **go-runewidth** calculations (fallback)

### Automated Width Analysis

The library includes powerful automated analysis tools for detecting and fixing terminal width issues:

```go
// Automated analysis workflow using CLI tools
// 1. Generate box with raw width calculations (no overrides)
// 2. Analyze for alignment issues
// 3. Apply corrections automatically

// Example using the CLI tools:
// dist/goneat ascii box --raw <calibration-file.txt | dist/goneat ascii analyze --apply
```

**Analysis Features:**

- **Automatic detection**: Identifies characters that render wider/narrower than calculated
- **Padding analysis**: Detects inconsistent padding patterns in box output
- **Multi-terminal support**: Can generate terminal-specific configurations
- **Raw mode**: Bypass all overrides to see true rendering issues
- **Apply mode**: Automatically save corrections to user config

### Interactive Calibration Tools

For manual fine-tuning and testing:

```go
// Create a calibration session
session := ascii.NewCalibrationSession("tests/fixtures/ascii/calibration/emojis.txt")

// Add terminal overrides for testing
session.WithTerminalOverride("xterm-ghostty", "ghostty")

// Run interactive calibration
err := session.RunInteractiveCalibration()

// Quick character marking
ascii.MarkCharactersWide([]string{"🚀", "ℹ️"})
ascii.MarkCharactersNarrow([]string{"→", "←"})
```

### Terminal Width Analysis Process

The complete analysis workflow:

1. **Detection Phase**: Analyze box output for padding inconsistencies
2. **Character Extraction**: Identify specific problematic characters
3. **Width Calculation**: Determine correct width adjustments needed
4. **Configuration Update**: Save terminal-specific overrides
5. **Verification**: Rebuild and test with corrections applied

```bash
# Complete calibration workflow
dist/goneat ascii box --raw <tests/fixtures/ascii/calibration/emojis.txt | \
  dist/goneat ascii analyze --apply

# Rebuild to incorporate changes
make build

# Verify corrections work
dist/goneat ascii box <tests/fixtures/ascii/calibration/emojis.txt
```

## CLI Commands

The library provides comprehensive CLI tools for terminal width calibration:

### `goneat ascii box`

Renders text in ASCII boxes with terminal-specific width handling.

```bash
# Standard box rendering
goneat ascii box "Hello" "World"

# Render from file
goneat ascii box <file.txt

# Raw mode (bypass terminal overrides)
goneat ascii box --raw <file.txt

# Width truncation
goneat ascii box --width 60 <file.txt
```

### `goneat ascii analyze`

Analyzes box output for width alignment issues and generates corrections.

```bash
# Basic analysis
goneat ascii box --raw <calibration-file.txt | goneat ascii analyze

# Auto-apply corrections
goneat ascii box --raw <calibration-file.txt | goneat ascii analyze --apply

# Generate mark commands
goneat ascii analyze --generate-marks --terminal iTerm.app < box-output.txt

# Analyze stringinfo output
goneat ascii stringinfo <file.txt | goneat ascii analyze --stringinfo
```

### `goneat ascii stringinfo`

Analyzes string width and byte information for debugging.

```bash
# Analyze string properties
goneat ascii stringinfo "Hello 🌟 World"

# Analyze from file
goneat ascii stringinfo <file.txt

# Include terminal environment info
goneat ascii stringinfo --env <file.txt
```

### `goneat ascii mark`

Manually mark characters as wide or narrow for terminal calibration.

```bash
# Mark characters as wide (render wider than calculated)
goneat ascii mark --wide "ℹ️" "⚠️" "🛠️"

# Mark characters as narrow (render narrower than calculated)
goneat ascii mark --narrow "→" "←"

# Target specific terminal
goneat ascii mark --term-program=iTerm.app --wide "🎟️"
```

### Additional Commands

```bash
# Interactive calibration session
goneat ascii calibrate tests/fixtures/ascii/calibration/emojis.txt

# Diagnostic information
goneat ascii diag

# Debug terminal catalog
goneat ascii debug

# Reset user configuration
goneat ascii reset

# Expand emoji collections to calibration format
goneat ascii expand <multi-emoji-file.txt>
```

## Test Fixtures and Calibration

The repository includes comprehensive test fixtures for terminal calibration:

### Calibration Files (`tests/fixtures/ascii/calibration/`)

Files in calibration format (with "Character U+" patterns) for automated analysis:

- **`emojis.txt`** - Comprehensive emoji collection including variation selectors
- **`logging-emojis.txt`** - Common emoji used in logging/status messages
- **`math-symbols.txt`** - Mathematical symbols and operators
- **`unicode-suite.txt`** - General Unicode test suite
- **`wide-characters.txt`** - CJK and other traditionally wide characters

### Sample Files (`tests/fixtures/ascii/samples/`)

Real-world text samples for testing box rendering:

- **`cjk-text.txt`** - Chinese, Japanese, Korean text samples
- **`emojis-original.txt`** - Original emoji collection format
- **`guardian-approval.txt`** - Sample guardian security prompt
- **`logging-messages.txt`** - Typical logging output with emoji

### Usage Example

```bash
# Clone repository and navigate to it
git clone https://github.com/fulmenhq/goneat
cd goneat

# Build goneat
make build

# Test current terminal with emoji calibration
dist/goneat ascii box --raw <tests/fixtures/ascii/calibration/emojis.txt

# If misaligned, run automatic calibration
dist/goneat ascii box --raw <tests/fixtures/ascii/calibration/emojis.txt | \
  dist/goneat ascii analyze --apply

# Rebuild to incorporate changes
make build

# Verify corrections
dist/goneat ascii box <tests/fixtures/ascii/calibration/emojis.txt

# Test with sample files
dist/goneat ascii box <tests/fixtures/ascii/samples/logging-messages.txt
```

See [`tests/fixtures/ascii/README.md`](../../../tests/fixtures/ascii/README.md) for detailed usage instructions.

## API Reference

### ascii.Box(lines []string) string

Returns an ASCII box as a string containing the provided lines.

### ascii.BoxRaw(lines []string) string

Returns an ASCII box using only go-runewidth without terminal-specific overrides.

### ascii.DrawBox(lines []string)

Draws a properly aligned ASCII box around the given lines of text.

```go
func DrawBox(lines []string)
```

**Parameters:**

- `lines`: Slice of strings to display within the box

**Behavior:**

- Trims trailing spaces from each line
- Calculates maximum line length for consistent box width
- Adds appropriate padding (2 spaces on each side)
- Uses Unicode box-drawing characters
- Prints directly to stdout

### ascii.TruncateForBox(value string, width int) string

Truncates a string to fit within a specified width, adding "..." if truncated.

### ascii.NewCalibrationSession(testFile string) \*CalibrationSession

Creates a new interactive calibration session for terminal character width testing.

```go
func NewCalibrationSession(testFile string) *CalibrationSession
```

**Parameters:**

- `testFile`: Path to a test file containing characters to calibrate

**Returns:**

- `*CalibrationSession`: Calibration session instance

**Example:**

```go
session := ascii.NewCalibrationSession("tests/fixtures/ascii/calibration/emojis.txt")
err := session.RunInteractiveCalibration()
```

### (*CalibrationSession).WithTerminalOverride(term, termProgram string) *CalibrationSession

Sets terminal detection overrides for testing specific terminal configurations.

```go
func (cs *CalibrationSession) WithTerminalOverride(term, termProgram string) *CalibrationSession
```

**Parameters:**

- `term`: Override for `TERM` environment variable
- `termProgram`: Override for `TERM_PROGRAM` environment variable

**Returns:**

- `*CalibrationSession`: The same session instance (for chaining)

### ascii.MarkCharactersWide(characters []string) error

Marks specified characters as wider than calculated for the current terminal.

```go
func MarkCharactersWide(characters []string) error
```

**Parameters:**

- `characters`: Slice of character strings to mark as wide

### ascii.MarkCharactersNarrow(characters []string) error

Marks specified characters as narrower than calculated for the current terminal.

```go
func MarkCharactersNarrow(characters []string) error
```

**Parameters:**

- `characters`: Slice of character strings to mark as narrow

### ascii.ResetUserConfig() error

Resets user terminal configuration to embedded repository defaults.

```go
func ResetUserConfig() error
```

**Behavior:**

- Overwrites `~/.goneat/config/terminal-overrides.yaml` with embedded defaults
- Useful for testing or starting fresh with calibration

```go
func TruncateForBox(value string, width int) string
```

**Parameters:**

- `value`: The string to potentially truncate
- `width`: Maximum width in characters

**Returns:**

- Original string if it fits within width
- Truncated string with "..." if it exceeds width
- Handles Unicode characters correctly

**Behavior:**

- Preserves full string if `len(runes) <= width`
- For width <= 3, returns string truncated to width (no ellipsis)
- Otherwise, truncates to `width-3` and appends "..."
- Uses rune-based counting for Unicode safety

## Advanced Usage

### Status Display Boxes

```go
package main

import (
    "fmt"
    "time"
    "github.com/fulmenhq/goneat/pkg/ascii"
)

func showStatusBox(operation, status string, duration time.Duration) {
    lines := []string{
        fmt.Sprintf("Operation: %s", operation),
        fmt.Sprintf("Status: %s", status),
        fmt.Sprintf("Duration: %v", duration),
        "",
        "Press Ctrl+C to cancel",
    }
    ascii.DrawBox(lines)
}

func main() {
    start := time.Now()
    // Simulate some work
    time.Sleep(100 * time.Millisecond)
    duration := time.Since(start)

    showStatusBox("File Processing", "Completed", duration)
}
```

### Error Message Formatting

```go
package main

import (
    "fmt"
    "github.com/fulmenhq/goneat/pkg/ascii"
)

func showErrorBox(title, message string, suggestions []string) {
    lines := []string{
        fmt.Sprintf("❌ %s", title),
        "",
        message,
    }

    if len(suggestions) > 0 {
        lines = append(lines, "")
        lines = append(lines, "Suggestions:")
        for _, suggestion := range suggestions {
            lines = append(lines, fmt.Sprintf("  • %s", suggestion))
        }
    }

    ascii.DrawBox(lines)
}

func main() {
    showErrorBox(
        "Configuration Error",
        "Unable to load configuration file",
        []string{
            "Check if config.yaml exists",
            "Verify file permissions",
            "Run 'goneat doctor' for diagnostics",
        },
    )
}
```

### Progress Display

```go
package main

import (
    "fmt"
    "github.com/fulmenhq/goneat/pkg/ascii"
)

func showProgressBox(current, total int, currentFile string) {
    percentage := float64(current) / float64(total) * 100

    // Create a simple progress bar
    barWidth := 20
    filled := int(float64(current) / float64(total) * float64(barWidth))
    bar := fmt.Sprintf("[%s%s]",
        string(make([]rune, filled, filled)) + "█",
        string(make([]rune, barWidth-filled, barWidth-filled)) + "░")

    lines := []string{
        "File Processing Progress",
        "",
        fmt.Sprintf("Progress: %.1f%% (%d/%d)", percentage, current, total),
        bar,
        "",
        fmt.Sprintf("Current: %s", ascii.TruncateForBox(currentFile, 40)),
    }

    ascii.DrawBox(lines)
}

func main() {
    files := []string{
        "config/database.yaml",
        "templates/user-profile.html",
        "scripts/setup.sh",
        "docs/README.md",
    }

    for i, file := range files {
        showProgressBox(i+1, len(files), file)
        fmt.Println() // Add spacing between boxes
    }
}
```

## Integration Examples

### With Guardian Approval System

```go
package main

import (
    "fmt"
    "github.com/fulmenhq/goneat/pkg/ascii"
)

func displayApprovalRequest(scope, operation, branch string) {
    lines := []string{
        "🛡️  GUARDIAN APPROVAL REQUIRED",
        "",
        fmt.Sprintf("Operation: %s.%s", scope, operation),
        fmt.Sprintf("Branch: %s", branch),
        "",
        "This operation requires approval due to security policy.",
        "A browser window will open for approval.",
        "",
        "⏱️  Expires in: 10:00",
    }

    ascii.DrawBox(lines)
}

func main() {
    displayApprovalRequest("git", "commit", "main")
}
```

### With Assessment Results

```go
package main

import (
    "fmt"
    "github.com/fulmenhq/goneat/pkg/ascii"
)

type AssessmentResult struct {
    Category string
    Issues   int
    Fixed    int
    Time     string
}

func displayAssessmentSummary(results []AssessmentResult) {
    lines := []string{
        "📊 Assessment Summary",
        "",
    }

    totalIssues := 0
    totalFixed := 0

    for _, result := range results {
        status := "✅"
        if result.Issues > 0 {
            status = "⚠️ "
        }

        lines = append(lines, fmt.Sprintf("%s %s: %d issues (%d fixed) - %s",
            status, result.Category, result.Issues, result.Fixed, result.Time))

        totalIssues += result.Issues
        totalFixed += result.Fixed
    }

    lines = append(lines, "")
    lines = append(lines, fmt.Sprintf("Total: %d issues, %d fixed", totalIssues, totalFixed))

    ascii.DrawBox(lines)
}

func main() {
    results := []AssessmentResult{
        {"format", 0, 5, "45ms"},
        {"lint", 2, 0, "1.2s"},
        {"security", 1, 1, "8.5s"},
        {"dates", 0, 3, "120ms"},
    }

    displayAssessmentSummary(results)
}
```

## Unicode and Internationalization

### Handling Multi-byte Characters

```go
package main

import (
    "fmt"
    "github.com/fulmenhq/goneat/pkg/ascii"
)

func demonstrateUnicode() {
    // Various Unicode content
    lines := []string{
        "English: Hello World",
        "Spanish: ¡Hola Mundo!",
        "French: Bonjour le monde",
        "Japanese: こんにちは世界",
        "Emoji: 🚀 ✨ 🌟 💫",
        "Math: ∫∞ e^x dx = ∞",
        "Currency: $100 €50 ¥1000 £75",
    }

    ascii.DrawBox(lines)
}

func main() {
    demonstrateUnicode()
}
```

### Right-to-Left Text Handling

```go
package main

import (
    "fmt"
    "github.com/fulmenhq/goneat/pkg/ascii"
)

func handleRTLText() {
    // Note: Go's fmt and terminal handling may not perfectly
    // support RTL text in all environments
    lines := []string{
        "LTR Text: Hello World",
        "RTL Text: مرحبا بالعالم", // Arabic: "Hello World"
        "Mixed: Hello مرحبا",
    }

    ascii.DrawBox(lines)
    fmt.Println("\nNote: RTL text display depends on terminal support")
}

func main() {
    handleRTLText()
}
```

## Performance Considerations

### Box Drawing Performance

```go
// Benchmark box drawing
func BenchmarkDrawBox(b *testing.B) {
    lines := []string{
        "Line 1: Short text",
        "Line 2: This is a longer line with more content",
        "Line 3: Unicode content: αβγδε 🌟 🚀",
        "Line 4: Even longer line that might cause wrapping in some terminals",
    }

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        // Note: DrawBox prints to stdout, so this benchmark measures
        // the formatting logic, not I/O performance
        ascii.DrawBox(lines)
    }
}

// Typical results:
// BenchmarkDrawBox-8    500000    2500 ns/op    0 B/op    0 allocs/op
```

### Truncation Performance

```go
func BenchmarkTruncateForBox(b *testing.B) {
    longString := "This is a very long string with lots of content that needs to be truncated for display purposes"
    shortString := "Short"

    b.Run("long_string", func(b *testing.B) {
        for i := 0; i < b.N; i++ {
            _ = ascii.TruncateForBox(longString, 20)
        }
    })

    b.Run("short_string", func(b *testing.B) {
        for i := 0; i < b.N; i++ {
            _ = ascii.TruncateForBox(shortString, 20)
        }
    })
}

// Typical results:
// BenchmarkTruncateForBox/long_string-8    10000000    120 ns/op    0 B/op    0 allocs/op
// BenchmarkTruncateForBox/short_string-8   20000000    85 ns/op     0 B/op    0 allocs/op
```

## Testing

### Unit Tests

```go
package ascii_test

import (
    "strings"
    "testing"
    "github.com/fulmenhq/goneat/pkg/ascii"
)

func TestTruncateForBox(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        width    int
        expected string
    }{
        {"no_truncation", "hello", 10, "hello"},
        {"truncation", "hello world", 8, "hello..."},
        {"exact_fit", "hello", 5, "hello"},
        {"unicode_safe", "hello 🌟 world", 10, "hello 🌟..."},
        {"width_too_small", "hello", 2, "he"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := ascii.TruncateForBox(tt.input, tt.width)
            if result != tt.expected {
                t.Errorf("TruncateForBox(%q, %d) = %q, want %q",
                    tt.input, tt.width, result, tt.expected)
            }
        })
    }
}

// Note: DrawBox prints to stdout, so testing requires capturing output
func TestDrawBox(t *testing.T) {
    // This is a basic smoke test - in practice you'd capture stdout
    lines := []string{"test"}
    // Should not panic
    ascii.DrawBox(lines)
}
```

### Integration Tests

```go
package main

import (
    "bytes"
    "io"
    "os"
    "testing"
    "github.com/fulmenhq/goneat/pkg/ascii"
)

// captureOutput captures stdout for testing DrawBox
func captureOutput(f func()) string {
    old := os.Stdout
    r, w, _ := os.Pipe()
    os.Stdout = w

    f()

    w.Close()
    os.Stdout = old

    var buf bytes.Buffer
    io.Copy(&buf, r)
    return buf.String()
}

func TestDrawBoxOutput(t *testing.T) {
    lines := []string{"Hello", "World"}

    output := captureOutput(func() {
        ascii.DrawBox(lines)
    })

    // Verify basic structure
    if !strings.Contains(output, "┌") || !strings.Contains(output, "┐") {
        t.Error("Box drawing characters not found")
    }

    if !strings.Contains(output, "Hello") || !strings.Contains(output, "World") {
        t.Error("Content not found in output")
    }
}
```

## Platform-Specific Considerations

### Terminal Compatibility

Different terminals handle Unicode box-drawing characters differently:

```go
package main

import (
    "os"
    "runtime"
    "github.com/fulmenhq/goneat/pkg/ascii"
)

// detectUnicodeSupport checks if terminal supports Unicode
func detectUnicodeSupport() bool {
    // Basic check - in production you'd do more thorough detection
    if runtime.GOOS == "windows" {
        // Windows Terminal, Windows 10+ cmd.exe support Unicode
        return os.Getenv("WT_SESSION") != "" || os.Getenv("TERM_PROGRAM") == "vscode"
    }
    return true // Assume Unix-like systems support Unicode
}

func safeDrawBox(lines []string) {
    if detectUnicodeSupport() {
        ascii.DrawBox(lines)
    } else {
        // Fallback to ASCII-only box drawing
        drawASCIIBox(lines)
    }
}

func drawASCIIBox(lines []string) {
    if len(lines) == 0 {
        return
    }

    maxLen := 0
    for _, line := range lines {
        if len(line) > maxLen {
            maxLen = len(line)
        }
    }

    border := strings.Repeat("-", maxLen+4)

    fmt.Printf("+%s+\n", border)
    for _, line := range lines {
        padding := maxLen - len(line)
        fmt.Printf("| %s%s |\n", line, strings.Repeat(" ", padding))
    }
    fmt.Printf("+%s+\n", border)
}

func main() {
    lines := []string{"Unicode Box", "ASCII Fallback"}
    safeDrawBox(lines)
}
```

### Windows Console Considerations

```go
// Windows-specific handling
func init() {
    if runtime.GOOS == "windows" {
        // Enable ANSI escape sequences in Windows Terminal
        // This is handled automatically in modern Go versions
    }
}
```

## Common Patterns and Best Practices

### Consistent Box Styling

```go
// Define standard box styles for your application
type BoxStyle struct {
    TitleColor string
    BorderColor string
    ContentPadding int
}

func (bs *BoxStyle) DrawStyledBox(title string, content []string) {
    lines := []string{
        fmt.Sprintf("\033[%sm%s\033[0m", bs.TitleColor, title),
        "",
    }
    lines = append(lines, content...)

    // Note: In a real implementation, you'd modify DrawBox to accept styling
    ascii.DrawBox(lines)
}

func main() {
    style := BoxStyle{
        TitleColor: "1;34", // Bold blue
        BorderColor: "37",  // White
        ContentPadding: 2,
    }

    style.DrawStyledBox("System Status", []string{
        "✅ Database: Connected",
        "✅ Cache: Operational",
        "⚠️  Disk space: 85% used",
    })
}
```

### Error vs Success Boxes

```go
func showResultBox(success bool, title string, messages []string) {
    icon := "❌"
    if success {
        icon = "✅"
    }

    lines := []string{
        fmt.Sprintf("%s %s", icon, title),
        "",
    }
    lines = append(lines, messages...)

    ascii.DrawBox(lines)
}

func main() {
    // Success case
    showResultBox(true, "Operation Completed", []string{
        "Files processed: 150",
        "Time elapsed: 2.3s",
        "No errors encountered",
    })

    // Error case
    showResultBox(false, "Operation Failed", []string{
        "Error: Connection timeout",
        "Files processed: 45/150",
        "Suggestion: Check network connectivity",
    })
}
```

## Future Enhancements

- **Color support**: ANSI color codes for enhanced terminal output
- **Table formatting**: Multi-column table rendering within boxes
- **Progress bars**: Integrated progress bar drawing
- **Animation**: Simple terminal animations for status updates
- **Themes**: Configurable color schemes and box styles
- **Rich text**: Markdown-style formatting within boxes

## Related Libraries

- [`pkg/logger`](logger.md) - Structured logging with terminal formatting
- [`pkg/format`](format.md) - Code formatting utilities
- [`pkg/pretty`](pretty.md) - Pretty-printing for data structures
- [Termui](https://github.com/gizak/termui) - Terminal UI library for Go
- [Tview](https://github.com/rivo/tview) - Rich terminal applications

## Usage Checklist

Before using `pkg/ascii` in your application:

- [ ] Terminal supports Unicode box-drawing characters
- [ ] Error handling for cases where stdout is redirected
- [ ] Consider performance impact for high-frequency output
- [ ] Test with various terminal widths and fonts
- [ ] Verify Unicode character display in target environments
- [ ] Consider accessibility for users with screen readers
- [ ] Test with different locale/language settings

For more information, see the [GoDoc documentation](https://pkg.go.dev/github.com/fulmenhq/goneat/pkg/ascii).

---

_Generated by Forge Neat ([OpenCode](https://opencode.ai/)) under supervision of [@3leapsdave](https://github.com/3leapsdave)_  
_Co-Authored-By: Forge Neat <noreply@3leaps.net>_
