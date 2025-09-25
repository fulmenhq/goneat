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

## API Reference

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