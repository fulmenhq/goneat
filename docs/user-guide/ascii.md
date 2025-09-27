---
title: ASCII Command
description: Generate formatted ASCII boxes and text using goneat's terminal helpers.
---

# `goneat ascii`

The `goneat ascii` command exposes the `pkg/ascii` utilities from the CLI so you can create well-aligned ASCII boxes (with proper Unicode handling) directly from pipelines, docs, or the terminal.

## Subcommands

### `goneat ascii box`

Render the supplied lines inside a Unicode box. Input can come from arguments or stdin.

```bash
# Provide lines as arguments (each argument is one line)
goneat ascii box "Guardian Approval" "Expires in 10m"

# Read lines from a file via stdin
goneat ascii box < message.txt

# Truncate long lines to 60 columns before boxing
goneat ascii box --width 60 < report.txt
```

Example output:

```
┌────────────────────┐
│ Guardian Approval  │
│ Expires in 10m     │
└────────────────────┘
```

## Flags

### `--width`, `-w`

Truncates each line to the specified display width (Unicode-aware) before rendering the box. Useful when you need consistent box widths or want to prevent wrapping in downstream consumers.

```bash
goneat ascii box --width 50 < long-text.txt
```

### `goneat ascii stringinfo`

Analyze string display width and byte length for Unicode debugging.

```bash
# Analyze character properties
goneat ascii stringinfo "🚀"
goneat ascii stringinfo "ℹ️ Information"

# Output example:
# byte_len=4   display_width=2  content="🚀"
# byte_len=13  display_width=3  content="ℹ️ Information"
```

### `goneat ascii diag`

Diagnose terminal Unicode width handling with comprehensive environment and character analysis.

```bash
# Run terminal diagnostics
goneat ascii diag

# Test specific characters
goneat ascii diag "🚀" "ℹ️" "📋"
```

Features:

- Terminal environment detection (`TERM`, `TERM_PROGRAM`, etc.)
- Character width analysis with recommendations
- Visual alignment test with sample box
- Terminal-specific behavior profiles

### `goneat ascii calibrate`

Interactively calibrate character widths for your terminal using visual feedback.

```bash
# Calibrate with emoji collection
goneat ascii calibrate tests/fixtures/ascii/emojis-collection.txt

# Override terminal detection for testing
goneat ascii calibrate --term-program=ghostty tests/fixtures/ascii/logging-emojis.txt
goneat ascii calibrate --term=xterm-ghostty file.txt
```

The calibration process:

1. Displays a test box with problematic characters
2. Guides you through visual inspection
3. Allows marking characters as too wide/narrow
4. Tests adjustments in real-time
5. Saves validated configuration to `~/.goneat/config/terminal-overrides.yaml`

### `goneat ascii mark`

Quickly mark specific characters as wider or narrower than calculated.

```bash
# Mark emojis as too wide
goneat ascii mark --wide "🚀" "ℹ️" "📋"

# Mark symbols as too narrow
goneat ascii mark --narrow "→" "←" "↑"

# Override terminal for testing
goneat ascii mark --term-program=iTerm.app --wide "✨" "🔥"
```

### `goneat ascii reset`

Reset user terminal configuration to repository defaults.

```bash
# Reset to defaults
goneat ascii reset

# Reset and immediately calibrate
goneat ascii reset && goneat ascii calibrate tests/fixtures/ascii/emojis-collection.txt
```

## Flags

### Global Flags

#### `--term`

Override `TERM` environment variable for testing different terminal configurations.

#### `--term-program`

Override `TERM_PROGRAM` environment variable for testing specific terminal emulators.

### Box-specific Flags

#### `--width`, `-w`

Truncates each line to the specified display width (Unicode-aware) before rendering the box. Useful when you need consistent box widths or want to prevent wrapping in downstream consumers.

```bash
goneat ascii box --width 50 < long-text.txt
```

## Terminal Compatibility

The ASCII commands automatically detect your terminal and apply appropriate character width adjustments for optimal rendering. Different terminals handle Unicode characters (especially emojis with variation selectors) differently:

- **Ghostty**: Handles emoji+variation selector sequences as double-width
- **iTerm2**: Configurable through user calibration
- **macOS Terminal**: Generally follows Unicode standards
- **Other terminals**: Extensible through configuration

### Configuration

Character width overrides are stored in:

- **User config**: `~/.goneat/config/terminal-overrides.yaml` (takes precedence)
- **Embedded defaults**: Built into goneat binary
- **Runtime fallback**: `mattn/go-runewidth` calculations

## Tips

- Empty input results in no output (the command is a no-op).
- Lines are trimmed of trailing spaces; leading spaces are preserved.
- The command understands multi-width runes (emoji, CJK characters) so borders remain aligned regardless of content.
- Use `goneat ascii diag` to troubleshoot alignment issues in your terminal.
- Run `goneat ascii calibrate` to create custom configurations for your specific terminal setup.
- For scripting, you can combine with other UNIX tools:
  ```bash
  goneat assess --summary | goneat ascii box --width 80
  ```

## Related Documentation

- [`pkg/ascii` library](../appnotes/lib/ascii.md) – Go API reference and examples
- [Guardian SOP updates](../sop/repository-operations-sop.md) – Example of ASCII boxes in terminal instructions
