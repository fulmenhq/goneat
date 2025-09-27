# ASCII Test Fixtures

This directory contains test fixtures for the goneat ASCII box rendering and terminal width calibration system.

## Directory Structure

### calibration/
Contains files in calibration format (with "Character U+" patterns) used for automated terminal width detection and adjustment. These files work with the `goneat ascii analyze` command to detect rendering discrepancies across different terminals.

**Files:**
- `emojis.txt` - Comprehensive emoji collection including variation selectors
- `logging-emojis.txt` - Common emoji used in logging/status messages
- `math-symbols.txt` - Mathematical symbols and operators
- `unicode-suite.txt` - General Unicode test suite
- `wide-characters.txt` - CJK and other traditionally wide characters

**Usage:**
```bash
# Detect terminal width issues (run in target terminal)
dist/goneat ascii box --raw <calibration/emojis.txt | dist/goneat ascii analyze

# Auto-apply corrections to user config
dist/goneat ascii box --raw <calibration/emojis.txt | dist/goneat ascii analyze --apply

# Generate mark commands for manual adjustment
dist/goneat ascii box --raw <calibration/emojis.txt | dist/goneat ascii analyze --generate-marks
```

### samples/
Contains sample text files for testing box rendering with real-world content. These files demonstrate how boxes appear with various types of text but are not in the calibration format needed for automated analysis.

**Files:**
- `cjk-text.txt` - Chinese, Japanese, Korean text samples
- `emojis-original.txt` - Original emoji collection format
- `guardian-approval.txt` - Sample guardian security prompt
- `logging-messages.txt` - Typical logging output with emoji

**Usage:**
```bash
# Test box rendering
goneat ascii box <samples/logging-messages.txt

# Test with width truncation
goneat ascii box --width 60 <samples/guardian-approval.txt
```

## Terminal Width Calibration Workflow

1. **Identify Issues**: Run a calibration file through box with `--raw` flag to bypass overrides
2. **Analyze**: Pipe output to `analyze` to detect width discrepancies
3. **Apply**: Use `--apply` flag to save corrections to your config
4. **Rebuild**: Run `make build` to incorporate new overrides
5. **Verify**: Test boxes now render correctly

## Known Terminals

The system has been tested and calibrated for:

- **Ghostty** (`TERM_PROGRAM=ghostty`)
  - Renders emoji+VS sequences as width 2 (needs overrides)

- **iTerm2** (`TERM_PROGRAM=iTerm.app`)
  - Renders emoji+VS sequences as width 2 (needs overrides)

- **Apple Terminal** (`TERM_PROGRAM=Apple_Terminal`)
  - Correctly follows Unicode standards (no overrides needed)

Additional terminals can be calibrated using the same workflow.

## Configuration Files

Terminal-specific width overrides are stored in:

- **User config**: `~/.goneat/config/terminal-overrides.yaml`
  - Created automatically when you run `analyze --apply`
  - Takes precedence over embedded defaults

- **Embedded defaults**: `config/ascii/terminal-overrides.yaml`
  - Ships with goneat
  - Contains community-contributed terminal profiles

## Documentation

For detailed information, see:
- [ASCII User Guide](../../../docs/user-guide/ascii.md)
- [Terminal Rendering Challenges](../../../docs/appnotes/terminal-rendering-challenges.md)
- [ASCII Library Documentation](../../../docs/appnotes/lib/ascii.md)

## Contributing Terminal Profiles

To contribute a new terminal profile:

1. Set your terminal's `TERM_PROGRAM` environment variable
2. Run calibration: `goneat ascii box --raw <calibration/emojis.txt | goneat ascii analyze --apply`
3. Test thoroughly with all calibration files
4. Submit PR with your additions to `config/ascii/terminal-overrides.yaml`

## Troubleshooting

If boxes are misaligned:
1. Check your terminal with: `echo $TERM_PROGRAM`
2. Run calibration on problematic characters
3. Ensure you've rebuilt after applying corrections: `make build`
4. Verify overrides are loaded: `goneat ascii diag`