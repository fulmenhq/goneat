---
title: "Format Library Reference"
description: "Detailed reference for the goneat format library - reusable formatting utilities for custom commands"
author: "@forge-neat"
date: "2025-09-29"
last_updated: "2025-09-29"
status: "approved"
tags: ["library", "formatting", "json", "utilities", "custom-commands"]
category: "appnotes"
---

# Format Library Reference

The `pkg/format` library provides reusable formatting utilities that can be used in custom commands or other parts of the goneat codebase.

## JSON Prettification

The `PrettifyJSON` function provides reliable, built-in JSON formatting using Go's standard library.

### Function Signature

```go
func PrettifyJSON(input []byte, indent string, sizeWarningMB int) ([]byte, bool, error)
```

### Parameters

- `input`: The JSON content as a byte slice
- `indent`: The indentation string (e.g., " " for 2 spaces, "\t" for tabs, "" for compact). Default: " " (2 spaces)
- `sizeWarningMB`: Threshold in MB for warning on large files (0 to disable). Default: 500

### Returns

- `[]byte`: The prettified JSON content
- `bool`: Whether the content was changed
- `error`: Any error encountered (e.g., invalid JSON)

### Usage Examples

```go
package main

import (
    "fmt"
    "os"
    formatpkg "github.com/fulmenhq/goneat/pkg/format"
)

func main() {
    // Read JSON file
    content, err := os.ReadFile("data.json")
    if err != nil {
        panic(err)
    }

    // Prettify with 2 spaces
    output, changed, err := formatpkg.PrettifyJSON(content, "  ", 500)
    if err != nil {
        panic(err)
    }

    if changed {
        fmt.Println("JSON was prettified")
        os.WriteFile("data.json", output, 0644)
    } else {
        fmt.Println("JSON was already formatted")
    }
}
```

### Compact Mode

For compact JSON output (no indentation):

```go
// Compact mode
output, changed, err := formatpkg.PrettifyJSON(content, "", 500)
```

### Size Warnings

The function automatically warns for large files:

```go
// Warns if file >500MB
output, changed, err := formatpkg.PrettifyJSON(content, "  ", 500)
```

### Error Handling

```go
output, changed, err := formatpkg.PrettifyJSON(content, "  ", 500)
if err != nil {
    if err.Error() == "invalid JSON" {
        fmt.Println("File contains invalid JSON")
    } else {
        fmt.Printf("Formatting failed: %v\n", err)
    }
}
```

## Integration with Commands

### Using in Custom Commands

```go
// In a custom command
func formatCustomJSON(file string) error {
    content, err := os.ReadFile(file)
    if err != nil {
        return err
    }

    output, _, err := formatpkg.PrettifyJSON(content, "  ", 500)
    if err != nil {
        return err
    }

    return os.WriteFile(file, output, 0644)
}
```

### CLI Flag Integration

```bash
# Use with custom indent count
goneat custom-command --json-indent-count 4

# Skip prettification
goneat custom-command --json-indent-count 0

# Use custom indent string
goneat custom-command --json-indent "\t"
```

## Best Practices

### Performance

- For very large files (>500MB), consider processing in chunks or warning users
- Use compact mode (`""` indent) for storage efficiency
- Validate JSON before calling to avoid unnecessary processing

### Error Handling

- Always check for "invalid JSON" errors
- Handle size warnings appropriately in your application
- Consider fallback strategies for critical formatting operations

### Consistency

- Use consistent indentation across your application
- Align with project standards (e.g., 2 spaces for most projects)
- Document your formatting choices in project guidelines

## Related Libraries

- [`finalizer`](../finalizer.md) - File normalization utilities
- [`config`](../config.md) - Configuration management
- [`logger`](../logger.md) - Logging utilities

## See Also

- [Format Command Reference](../../user-guide/commands/format.md) - CLI usage
- [Work Planning Guide](../../user-guide/work-planning.md) - Advanced features
