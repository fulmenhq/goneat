---
title: Terminal Unicode Width Rendering
description: Understanding how different terminals render Unicode character widths and how goneat adapts to ensure consistent ASCII box alignment.
---

# Terminal Unicode Width Rendering

Different terminal emulators render Unicode characters with varying visual widths, particularly emoji and characters with variation selectors. This creates alignment challenges for ASCII box rendering that can only be resolved through terminal-specific configuration and visual inspection.

## The Core Challenge

When you copy and paste the same ASCII box output from one terminal program to another, it may appear completely different. A perfectly aligned box in Apple Terminal might have jagged borders in Ghostty or iTerm2. This happens because:

1. **Terminals use different font rendering engines** with varying glyph metrics
2. **Emoji engines differ** in how they handle variation selectors (VS16: U+FE0F)
3. **Width calculations vary** from the Unicode standard recommendations
4. **Font substitution** can change character widths unexpectedly

## Visual Inspection is Essential

There is **no programmatic way** to determine how a terminal will actually render a character's width. The only reliable method is visual inspection - generating test output and manually checking alignment in each terminal.

This is why goneat provides:

- Test fixtures with known problematic characters
- Raw rendering mode to see true terminal behavior
- Automated analysis tools to detect alignment issues
- Visual calibration workflows for each terminal

## How goneat Handles Width Differences

### 1. Base Calculations with go-runewidth

goneat uses the [go-runewidth](https://github.com/mattn/go-runewidth) package for Unicode-compliant width calculations:

```go
import "github.com/mattn/go-runewidth"

// Base width calculation
width := runewidth.StringWidth("🎟️")  // Returns 1 (emoji + VS = narrow)
```

### 2. Terminal-Specific Overrides

For terminals that render differently than the standard, goneat maintains override configurations:

```yaml
# config/ascii/terminal-overrides.yaml
terminals:
  ghostty:
    name: "Ghostty"
    overrides:
      "ℹ️": 2 # Renders as 2 columns despite Unicode width 1
      "⚠️": 2 # Variation selector sequences
      "🛠️": 2 # Need explicit width overrides
```

### 3. Automatic Detection and Calibration

goneat provides tools to automatically detect and fix alignment issues:

```bash
# Generate test output with raw (no overrides) calculations
dist/goneat ascii box --raw <tests/fixtures/ascii/calibration/emojis.txt

# Analyze for alignment issues and apply fixes
dist/goneat ascii box --raw <calibration-file.txt | dist/goneat ascii analyze --apply

# Rebuild to incorporate new terminal-specific overrides
make build
```

## Supported Terminals

### Tested and Calibrated

- **Apple Terminal** (`TERM_PROGRAM=Apple_Terminal`)
  - Generally follows Unicode width standards correctly
  - Emoji+VS sequences render as width 1 (correct)
  - Requires minimal overrides

- **Ghostty** (`TERM_PROGRAM=ghostty`)
  - Renders emoji+VS sequences as width 2
  - Requires overrides for: ℹ️, ⚠️, ⏱️, 🛠️, and others
  - Well-tested with comprehensive override set

- **iTerm2** (`TERM_PROGRAM=iTerm.app`)
  - Similar behavior to Ghostty for emoji+VS sequences
  - Requires same overrides as Ghostty for most characters
  - Active calibration profile maintained

### Other Terminals

goneat's terminal detection system can be extended for any terminal that sets `TERM_PROGRAM` or `TERM` environment variables. The calibration workflow works the same:

1. Run calibration files through the terminal
2. Use visual inspection to identify misaligned characters
3. Run automated analysis to generate overrides
4. Test and refine the configuration

## Calibration Workflow

### Test Fixtures for Calibration

goneat includes comprehensive test fixtures in `tests/fixtures/ascii/calibration/`:

- **`emojis.txt`** - Comprehensive emoji collection with problematic sequences
- **`logging-emojis.txt`** - Common status/logging emoji
- **`math-symbols.txt`** - Mathematical operators and symbols
- **`unicode-suite.txt`** - General Unicode test cases
- **wide-characters.txt** - CJK and traditionally wide characters

Each file uses the calibration format:

```
🎟️  Character U+1F39F+VS           (width=1, bytes=7)
🛠️  Character U+1F6E0+VS           (width=1, bytes=7)
ℹ️  Character U+2139+VS            (width=1, bytes=6)
```

### Manual Calibration Process

1. **Visual Test**: Run calibration files in your terminal

   ```bash
   dist/goneat ascii box --raw <tests/fixtures/ascii/calibration/emojis.txt
   ```

2. **Identify Issues**: Look for characters that push box borders out of alignment

3. **Automated Analysis**: Let goneat detect and fix the issues

   ```bash
   dist/goneat ascii box --raw <calibration-file.txt | dist/goneat ascii analyze --apply
   ```

4. **Verify**: Rebuild and test the corrections
   ```bash
   make build
   dist/goneat ascii box <tests/fixtures/ascii/calibration/emojis.txt
   ```

### Configuration Files

Terminal overrides are stored in a hierarchy:

1. **User Config** (highest priority): `$GONEAT_HOME/config/terminal-overrides.yaml`
   - Created when you run `analyze --apply`
   - Personal overrides for your terminal setup

2. **Embedded Defaults** (repository): `config/ascii/terminal-overrides.yaml`
   - Ships with goneat
   - Community-contributed terminal profiles
   - Read-only, embedded in the binary

Example user config structure:

```yaml
# ~/.goneat/config/terminal-overrides.yaml
terminals:
  iTerm.app:
    name: iTerm2
    overrides:
      "ℹ️": 2
      "⚠️": 2
      "🛠️": 2
    notes: Auto-calibrated for iTerm2 v3.4.19
```

## Common Character Issues

### Emoji with Variation Selectors

The most common alignment problems occur with emoji followed by Variation Selector-16 (U+FE0F):

- `ℹ️` (U+2139 + U+FE0F) - Information symbol
- `⚠️` (U+26A0 + U+FE0F) - Warning sign
- `🛠️` (U+1F6E0 + U+FE0F) - Hammer and wrench
- `⏱️` (U+23F1 + U+FE0F) - Stopwatch

**Unicode Standard**: These should render as width 1
**Reality**: Many terminals render them as width 2

### CJK Characters

Chinese, Japanese, and Korean characters are traditionally "wide" (width 2), but:

- Font choice affects actual rendering width
- Some terminals have configurable CJK width modes
- Mixed content can cause alignment issues

### Mathematical Symbols

Mathematical operators like ∫∞∑∏ can have inconsistent widths across terminals due to font substitution.

## Extending Terminal Support

### Adding a New Terminal

1. **Detection**: Ensure the terminal sets identifiable environment variables:

   ```bash
   echo "TERM=$TERM TERM_PROGRAM=$TERM_PROGRAM"
   ```

2. **Calibration**: Run the test suite and identify problematic characters:

   ```bash
   for file in tests/fixtures/ascii/calibration/*.txt; do
     echo "Testing: $file"
     dist/goneat ascii box --raw <"$file" | dist/goneat ascii analyze --apply
   done
   ```

3. **Testing**: Verify fixes work across different content:

   ```bash
   make build
   for file in tests/fixtures/ascii/samples/*.txt; do
     dist/goneat ascii box <"$file"
   done
   ```

4. **Contributing**: Submit your terminal profile to the repository's embedded defaults.

### Manual Override Creation

If automated analysis doesn't work perfectly, you can manually create overrides:

```bash
# Mark specific characters as wide (render as 2 columns)
goneat ascii mark --term-program=YourTerminal --wide "ℹ️" "⚠️"

# Mark characters as narrow (render as 1 column)
goneat ascii mark --term-program=YourTerminal --narrow "→" "←"
```

## Troubleshooting

### Boxes Still Misaligned

1. **Check terminal detection**:

   ```bash
   goneat ascii diag
   ```

2. **Verify configuration is loaded**:

   ```bash
   goneat ascii debug
   ```

3. **Rebuild after changes**:

   ```bash
   make build
   ```

4. **Test with raw mode** to see actual vs expected behavior:
   ```bash
   dist/goneat ascii box --raw <test-file.txt
   dist/goneat ascii box <test-file.txt
   ```

### Characters Missing from Analysis

The automated analysis only works with calibration-format files. For general text:

1. Create a calibration version with problematic characters
2. Use the `goneat ascii expand` command to convert multi-character lines
3. Run manual `goneat ascii mark` commands for specific characters

### Terminal Updates Break Alignment

Terminal updates can change rendering behavior. When this happens:

1. Re-run calibration on affected terminals
2. Update overrides with new findings
3. Submit updates to the community repository

## Best Practices

### For Users

- **Test in your actual terminal** - don't rely on screenshots or examples
- **Use calibration files** before deploying ASCII output in production
- **Keep overrides up to date** when updating terminal software
- **Report new terminal findings** to help the community

### For Developers

- **Use goneat's ASCII library** rather than rolling your own
- **Test across multiple terminals** during development
- **Provide fallback formatting** for terminals with poor Unicode support
- **Document terminal requirements** if perfect alignment is critical

### For Contributors

- **Test comprehensively** across all supported terminals
- **Document terminal versions** when contributing overrides
- **Use consistent naming** for terminal identifiers
- **Provide reproduction steps** for reported issues

## Technical Implementation

### Width Calculation Pipeline

```go
func StringWidth(s string) int {
    // 1. Check for terminal-specific overrides
    if override := getTerminalOverride(s); override != nil {
        return *override
    }

    // 2. Fall back to go-runewidth calculation
    return runewidth.StringWidth(s)
}
```

### Terminal Detection

```go
func detectTerminal() string {
    if program := os.Getenv("TERM_PROGRAM"); program != "" {
        return program  // "iTerm.app", "ghostty", etc.
    }
    return os.Getenv("TERM")  // "xterm-256color", etc.
}
```

### Override Application

The system checks for character overrides in this order:

1. Exact string match in terminal overrides
2. Substring matching for longer strings with known characters
3. Base go-runewidth calculation

This ensures that both individual characters and strings containing those characters get the correct width calculations.

---

The fundamental lesson: **terminal Unicode rendering is inherently variable**. goneat's approach acknowledges this reality and provides practical tools to handle the variation through community knowledge, automated detection, and user-specific configuration.
