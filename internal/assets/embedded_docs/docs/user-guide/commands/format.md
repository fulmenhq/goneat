---
title: "Format Command Reference"
description: "Complete reference for the goneat format command - comprehensive code formatting with extended file operations and unified detection logic"
author: "@forge-neat"
date: "2025-08-31"
last_updated: "2025-09-15"
status: "approved"
tags:
  [
    "cli",
    "formatting",
    "file-operations",
    "normalization",
    "commands",
    "consistency",
    "dx",
  ]
category: "user-guide"
---

# Format Command Reference

The `goneat format` command provides comprehensive code formatting with advanced file-level operations that go beyond traditional syntax formatting. It supports multiple programming languages and includes sophisticated file normalization features.

## Overview

Goneat format is a multi-purpose formatting tool that:

- **Formats code** using language-specific formatters (Go, YAML, JSON, Markdown)
- **Normalizes files** with comprehensive file-level operations (EOF, BOM, line endings, whitespace)
- **Supports work planning** with dry-run, plan-only, and parallel execution modes
- **Integrates with CI/CD** through check mode and structured output
- **Handles mixed codebases** with intelligent file type detection
- **Provides idempotent operations** that are safe to run repeatedly

## Recent Improvements (v0.2.5)

### 🎯 Consistent Format & Assess Commands

**Problem**: The `goneat format --check` and `goneat assess --categories format` commands were giving conflicting results for the same files.

**Solution**: Unified detection logic ensures both commands use identical formatting rules and options.

#### Key Fixes

**1. Markdown Hard Break Preservation**

```bash
# Both commands now consistently handle markdown hard breaks
goneat format --check --finalize-trim-trailing-spaces file.md
goneat assess --categories format file.md

# Result: Both agree on exactly 2 trailing spaces (markdown hard breaks)
# ✅ format: "All files properly formatted"
# ✅ assess: "0 issues found"
```

**2. Finalizer Options Support**

- All format functions (`formatGoFile`, `formatYAMLFile`, `formatJSONFile`, `formatMarkdownFile`) now accept and apply finalizer options
- `--finalize-trim-trailing-spaces`, `--finalize-eof`, `--finalize-line-endings`, `--finalize-remove-bom` work consistently across all file types
- Check mode (`--check`) uses the same detection logic as actual formatting

**3. Enhanced Markdown Handling**

```bash
# Before: Inconsistent behavior
goneat format --check --finalize-trim-trailing-spaces file.md  # Said "formatted"
goneat assess --categories format file.md                       # Said "issues found"

# After: Consistent behavior
goneat format --check --finalize-trim-trailing-spaces file.md  # ✅ "formatted"
goneat assess --categories format file.md                       # ✅ "0 issues found"
```

#### Technical Details

**Detection Logic Unification**

- Both `format --check` and `assess` now use `ComprehensiveFileNormalization()` for consistent issue detection
- `DetectWhitespaceIssues()` respects `PreserveMarkdownHardBreaks` option
- Finalizer options are applied after language-specific formatting (prettier, gofmt, etc.)

**File Type Coverage**

- **Go**: gofmt + finalizer options
- **YAML**: yamlfmt + finalizer options
- **JSON**: jq + finalizer options
- **Markdown**: prettier + finalizer options
- **Text files**: Generic normalization + finalizer options

**Impact**

- ✅ Eliminates false positive formatting issues
- ✅ Consistent CI/CD validation across different commands
- ✅ Reliable pre-commit hook behavior
- ✅ Better developer experience with unified tooling

## Command Structure

```bash
goneat format [target] [flags]
```

### Breaking Change in v0.1.6

The `-f` flag has been replaced with clearer file selection flags:

**Old (v0.1.5 and earlier):**

```bash
goneat format -f "*.go"     # ❌ Deprecated - treated files as patterns
```

**New (v0.1.6+):**

```bash
goneat format --files main.go,utils.go      # ✅ Explicit file list
goneat format --patterns "*.go","test_*"    # ✅ Glob patterns for discovery
```

**Migration Guide:**

- Replace `-f pattern` with `--patterns pattern`
- Replace `-f file.go` with `--files file.go`
- Cannot combine `--files` and `--patterns` (validation enforced)

## Core Use Cases

### Basic Code Formatting

Format code using appropriate language-specific tools:

```bash
# Format current directory
goneat format

# Format specific directory
goneat format ./src

# Format specific files (explicit list)
goneat format --files main.go,utils.go

# Format using glob patterns
goneat format --patterns "*.go","pkg/**/*.yaml"

# Format with language-specific options
goneat format --types go,yaml
```

### File Normalization

Apply comprehensive file-level operations:

```bash
# Ensure all files end with single newline
goneat format --finalize-eof

# Remove trailing whitespace and normalize line endings
goneat format --finalize-trim-trailing-spaces --finalize-line-endings=lf

# Remove BOM and normalize everything
goneat format --finalize-remove-bom --finalize-eof --finalize-trim-trailing-spaces

# Comprehensive normalization
goneat format --finalize-eof --finalize-trim-trailing-spaces --finalize-line-endings=lf --finalize-remove-bom
```

### CI/CD Integration

Use check mode for automated validation:

```bash
# Check formatting without making changes
goneat format --check

# Check specific operations
goneat format --check --finalize-eof --finalize-trim-trailing-spaces

# Fail on formatting issues (for CI)
goneat format --check --finalize-eof || exit 1
```

### Work Planning and Analysis

Plan and analyze formatting operations:

```bash
# Preview what would be formatted
goneat format --dry-run

# Generate detailed execution plan
goneat format --plan-only --plan-file plan.json

# Analyze by file type
goneat format --plan-only --group-by-type

# Analyze by file size for optimization
goneat format --plan-only --group-by-size
```

## Command Flags

### File Selection Flags

| Flag          | Type    | Description                              | Example                      |
| ------------- | ------- | ---------------------------------------- | ---------------------------- |
| `--files`     | strings | Specific files to format (explicit list) | `--files main.go,utils.go`   |
| `--patterns`  | strings | Glob patterns to filter discovered files | `--patterns "*.go","test_*"` |
| `--folders`   | strings | Directories to process                   | `--folders src/,pkg/`        |
| `--types`     | strings | File types to include                    | `--types go,yaml,json`       |
| `--max-depth` | int     | Maximum directory depth                  | `--max-depth 3`              |

**File Selection Precedence:**

- `--files`: Processes exact files (no patterns, no discovery)
- `--patterns`: Filters files during discovery (cannot combine with --files)
- `--folders`, `--types`, `--include`, `--exclude`: Additional filtering options

### Operation Mode Flags

| Flag          | Type    | Description               | Example                 |
| ------------- | ------- | ------------------------- | ----------------------- |
| `--check`     | boolean | Check mode (no changes)   | `--check`               |
| `--dry-run`   | boolean | Preview mode (no changes) | `--dry-run`             |
| `--plan-only` | boolean | Generate plan only        | `--plan-only`           |
| `--plan-file` | string  | Save plan to file         | `--plan-file plan.json` |
| `--no-op`     | boolean | No-operation mode         | `--no-op`               |

### File Operation Flags

| Flag                              | Type    | Description                                                                         | Example                            |
| --------------------------------- | ------- | ----------------------------------------------------------------------------------- | ---------------------------------- |
| `--finalize-eof`                  | boolean | Ensure single trailing newline                                                      | `--finalize-eof`                   |
| `--finalize-trim-trailing-spaces` | boolean | Remove trailing whitespace                                                          | `--finalize-trim-trailing-spaces`  |
| `--finalize-line-endings`         | string  | Normalize line endings                                                              | `--finalize-line-endings=lf`       |
| `--finalize-remove-bom`           | boolean | Remove UTF-8/16/32 BOM                                                              | `--finalize-remove-bom`            |
| `--text-normalize`                | boolean | Apply generic text normalization to any text file (unknown extensions included)     | `--text-normalize`                 |
| `--text-encoding-policy`          | string  | Encoding policy for normalization: `utf8-only` (default), `utf8-or-bom`, `any-text` | `--text-encoding-policy=utf8-only` |
| `--preserve-md-linebreaks`        | boolean | Preserve Markdown hard line breaks (two trailing spaces)                            | `--preserve-md-linebreaks`         |

### Language-specific Organizer Flags (Go)

| Flag              | Type    | Description                                    | Example           |
| ----------------- | ------- | ---------------------------------------------- | ----------------- |
| `--use-goimports` | boolean | Organize Go imports with goimports after gofmt | `--use-goimports` |

Notes:

- When `--strategy parallel` is used, goimports is currently skipped with a warning. Use sequential strategy for import alignment until the parallel processor is extended.
- If `goimports` is not installed:
  - With `--ignore-missing-tools`: import alignment is skipped (warn once).
  - Without `--ignore-missing-tools`: the command fails fast with a helpful error that includes install guidance (`go install golang.org/x/tools/cmd/goimports@latest`) and mentions the planned `goneat doctor` workflow.

### Execution Control Flags

| Flag              | Type    | Description           | Example               |
| ----------------- | ------- | --------------------- | --------------------- |
| `--strategy`      | string  | Execution strategy    | `--strategy parallel` |
| `--group-by-size` | boolean | Group by file size    | `--group-by-size`     |
| `--group-by-type` | boolean | Group by content type | `--group-by-type`     |
| `--concurrency`   | int     | Worker count override | `--concurrency 4`     |

### Filtering and Scope Flags

| Flag                     | Type    | Description                 | Example                  |
| ------------------------ | ------- | --------------------------- | ------------------------ |
| `--staged-only`          | boolean | Only staged files (git)     | `--staged-only`          |
| `--ignore-missing-tools` | boolean | Skip missing external tools | `--ignore-missing-tools` |
| `--include`              | strings | Include patterns            | `--include "*.go"`       |
| `--exclude`              | strings | Exclude patterns            | `--exclude "vendor/**"`  |

## Supported File Types

### Primary Formatters

| Language     | Extension          | Tool            | Description                    |
| ------------ | ------------------ | --------------- | ------------------------------ |
| **Go**       | `.go`              | gofmt/goimports | Standard Go formatting         |
| **YAML**     | `.yaml`, `.yml`    | yamlfmt         | YAML structure formatting      |
| **JSON**     | `.json`            | jq              | JSON formatting and validation |
| **Markdown** | `.md`, `.markdown` | prettier        | Markdown formatting            |

### Extended File Operations

File normalization operations work on **all supported file types**:

- **Text files**: `.txt`, `.md`, `.sh`, `.py`, `.js`, `.ts`, `.html`, `.css`, `.xml`, `.toml`, `.ini`, `.cfg`, `.conf`
- **Config files**: All common configuration formats
- **Script files**: Shell scripts, Python, JavaScript, etc.
- **Documentation**: README files, documentation in various formats

## File Normalization Operations

### EOF Newline Enforcement

Ensures all files end with exactly one newline character:

```bash
# Check for missing EOF newlines
goneat format --check --finalize-eof

# Add missing EOF newlines
goneat format --finalize-eof

# Examples:
# Before: "last line"     → After: "last line\n"
# Before: "last line\n\n" → After: "last line\n"
```

### Trailing Whitespace Removal

Removes trailing spaces and tabs from all lines:

```bash
# Check for trailing whitespace
goneat format --check --finalize-trim-trailing-spaces

# Remove trailing whitespace
goneat format --finalize-trim-trailing-spaces

# Examples:
# Before: "line with spaces   " → After: "line with spaces"
# Before: "line with tabs\t\t"  → After: "line with tabs"
```

### Line Ending Normalization

Convert all line endings to consistent format:

```bash
# Normalize to LF (Unix)
goneat format --finalize-line-endings=lf

# Normalize to CRLF (Windows)
goneat format --finalize-line-endings=crlf

# Auto-detect and preserve existing style
goneat format --finalize-line-endings=auto

# Examples:
# CRLF → LF: "line\r\n" → "line\n"
# Mixed → LF: "line1\nline2\r\n" → "line1\nline2\n"
```

### BOM Detection and Removal

Remove Unicode Byte Order Marks from files:

```bash
# Check for BOMs
goneat format --check --finalize-remove-bom

# Remove BOMs
goneat format --finalize-remove-bom

# Supported BOM types:
# UTF-8:    \xef\xbb\xbf (3 bytes)
# UTF-16BE: \xfe\xff (2 bytes)
# UTF-16LE: \xff\xfe (2 bytes)
# UTF-32BE: \x00\x00\xfe\xff (4 bytes)
# UTF-32LE: \xff\xfe\x00\x00 (4 bytes)
```

## Operation Modes

### Check Mode (`--check`)

Validates formatting without making changes:

```bash
# Check all formatting
goneat format --check

# Check specific operations
goneat format --check --finalize-eof --finalize-trim-trailing-spaces

# Use in CI/CD pipelines
goneat format --check || echo "Formatting issues found"
```

**Exit Codes:**

- `0`: All files properly formatted
- `1`: Formatting issues found
- `2`: Error occurred

### Dry Run Mode (`--dry-run`)

Shows what would be formatted without making changes:

```bash
# Preview formatting operations
goneat format --dry-run

# Preview with detailed output
goneat format --dry-run --verbose
```

### Plan Only Mode (`--plan-only`)

Generates detailed execution plans:

```bash
# Generate formatting plan
goneat format --plan-only

# Save plan to file
goneat format --plan-only --plan-file format-plan.json

# Analyze by content type
goneat format --plan-only --group-by-type
```

### Normal Mode (Default)

Applies formatting changes to files:

```bash
# Format current directory
goneat format

# Format with specific operations
goneat format --finalize-eof --finalize-trim-trailing-spaces
```

## Work Planning Features

### Execution Strategies

Control how formatting operations are executed:

```bash
# Sequential execution (default)
goneat format --strategy sequential

# Parallel execution
goneat format --strategy parallel

# Parallel with custom worker count
goneat format --strategy parallel --concurrency 8
```

### Grouping Options

Optimize execution through intelligent grouping:

```bash
# Group by content type (recommended for mixed projects)
goneat format --group-by-type

# Group by file size (recommended for large projects)
goneat format --group-by-size

# Default grouping
goneat format
```

### Plan Analysis

Understand the scope and impact of formatting operations:

```json
{
  "plan": {
    "command": "format",
    "total_files": 150,
    "filtered_files": 45,
    "execution_strategy": "parallel"
  },
  "work_items": [
    {
      "path": "src/main.go",
      "content_type": "go",
      "size": 2048,
      "estimated_time": 0.5
    }
  ],
  "groups": [
    {
      "name": "Go Files",
      "recommended_parallelization": 4
    }
  ]
}
```

## Usage Examples

### Development Workflow

```bash
# Quick format check
goneat format --check

# Auto-fix formatting issues
goneat format --finalize-eof --finalize-trim-trailing-spaces

# Format before commit
goneat format && git add .
```

### Pre-commit Hook Integration

```bash
# Check formatting before commit
goneat format --check --finalize-eof --finalize-trim-trailing-spaces

# Auto-fix and check
goneat format --finalize-eof --finalize-trim-trailing-spaces
goneat format --check --finalize-eof --finalize-trim-trailing-spaces
```

### CI/CD Pipeline

```yaml
# .github/workflows/format.yml
name: Format Check
on: [pull_request]

jobs:
  format:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: Check formatting
        run: |
          goneat format --check --finalize-eof --finalize-trim-trailing-spaces
          if [ $? -ne 0 ]; then
            echo "Formatting issues found. Run: goneat format --finalize-eof --finalize-trim-trailing-spaces"
            exit 1
          fi
```

### Large Project Optimization

```bash
# Analyze project structure
goneat format --plan-only --group-by-type

# Format by content type
goneat format --types go --finalize-eof
goneat format --types yaml,json --finalize-trim-trailing-spaces

# Parallel execution for large codebases
goneat format --strategy parallel --concurrency 8
```

### Custom Workflows

```bash
# Format only Go files
goneat format --types go

# Format specific directories
goneat format --folders src/,pkg/ --types go,yaml

# Format with custom depth limit
goneat format --max-depth 3 --types go

# Format staged files only
goneat format --staged-only
```

## Integration Examples

### Make Integration

```makefile
.PHONY: format format-check format-plan

format:
	goneat format --finalize-eof --finalize-trim-trailing-spaces

format-check:
	goneat format --check --finalize-eof --finalize-trim-trailing-spaces

format-plan:
	goneat format --plan-only --plan-file format-plan.json
```

### Git Integration

```bash
# Format only changed files
goneat format --files $(git diff --name-only | tr '\n' ',')

# Format staged files
goneat format --staged-only

# Check formatting in CI
goneat format --check --finalize-eof || exit 1
```

### IDE Integration

```json
// VS Code settings.json
{
  "go.formatTool": "goneat",
  "go.formatOnSave": true,
  "go.formatOnSaveMode": "file"
}
```

## Performance Considerations

### Optimization Strategies

- **Parallel Execution**: Use `--strategy parallel` for multi-core systems
- **File Type Filtering**: Use `--types` to limit processing scope
- **Depth Limiting**: Use `--max-depth` to avoid deep directory traversal
- **Incremental Processing**: Use `--staged-only` for pre-commit hooks

### Performance Metrics

- **Small files**: < 1ms per file
- **Typical files**: 1-5ms per file
- **Large files**: 10-50ms per file
- **Parallel scaling**: Near-linear scaling with CPU cores

### Memory Usage

- **Base memory**: ~10MB
- **Per file**: ~100KB additional
- **Large files**: Scales with file size
- **Parallel workers**: Multiplicative scaling

## Troubleshooting

### Common Issues

**"No files found to format"**

```bash
# Check current directory contents
ls -la

# Try explicit path
goneat format --folders .

# Check supported file types
goneat format --plan-only --types go,yaml,json,markdown
```

**"Tool not found" errors**

```bash
# Check tool availability
which gofmt yamlfmt jq prettier

# Skip missing tools
goneat format --ignore-missing-tools

# Install missing tools
go install golang.org/x/tools/cmd/goimports@latest
npm install -g prettier yamlfmt
```

**"Permission denied" errors**

```bash
# Check file permissions
ls -la problematic-file

# Fix permissions
chmod 644 problematic-file

# Skip permission issues
goneat format --ignore-errors
```

### Debug Mode

Enable verbose output for troubleshooting:

```bash
# Verbose formatting
goneat format --verbose

# Debug specific operations
goneat format --verbose --finalize-eof

# Check tool execution
goneat format --verbose --types yaml
```

### Recovery Options

**Undo formatting changes:**

```bash
# If using git
git checkout -- file-to-revert
git reset --hard HEAD~1  # If committed

# Manual recovery
cp backup-file original-file
```

**Partial recovery:**

```bash
# Format only specific files
goneat format --files specific-file.go

# Exclude problematic files
goneat format --exclude "problematic-file.*"
```

## Advanced Usage

### Custom Configuration

Create project-specific formatting rules:

```yaml
# .goneat/config.yaml
format:
  finalizer:
    ensure_final_newline: true
    trim_trailing_whitespace: true
    normalize_line_endings: "lf"
    remove_utf8_bom: true

  overrides:
    "*.md":
      ensure_final_newline: false
    "scripts/*":
      fix_executable_permissions: true
```

### Batch Processing

Handle large codebases efficiently:

```bash
# Process in batches by type
for type in go yaml json markdown; do
  goneat format --types $type --finalize-eof
done

# Parallel processing by directory
find . -type d -name "src" -exec goneat format --folders {} \;
```

### Integration with Other Tools

Combine with existing workflows:

```bash
# Format with gofmt first
gofmt -w .
goneat format --finalize-eof --finalize-trim-trailing-spaces

# Format with prettier first
npx prettier --write "**/*.{md,yml,yaml,json}"
goneat format --finalize-eof --finalize-remove-bom
```

## File Type Support Matrix

| Operation               | Go  | YAML | JSON | Markdown | Text | Scripts | Config |
| ----------------------- | --- | ---- | ---- | -------- | ---- | ------- | ------ |
| **Syntax Formatting**   | ✅  | ✅   | ✅   | ✅       | ❌   | ❌      | ❌     |
| **EOF Newline**         | ✅  | ✅   | ✅   | ✅       | ✅   | ✅      | ✅     |
| **Trailing Whitespace** | ✅  | ✅   | ✅   | ✅       | ✅   | ✅      | ✅     |
| **Line Endings**        | ✅  | ✅   | ✅   | ✅       | ✅   | ✅      | ✅     |
| **BOM Removal**         | ✅  | ✅   | ✅   | ✅       | ✅   | ✅      | ✅     |
| **Binary Detection**    | ✅  | ✅   | ✅   | ✅       | ✅   | ✅      | ✅     |

**Note (v0.2.5)**: All finalizer operations now work consistently across all supported file types. The `goneat format --check` and `goneat assess --categories format` commands use unified detection logic for reliable CI/CD validation.

## Future Enhancements

Planned improvements for the format command:

- **Additional Language Support**: Python, JavaScript, TypeScript, Rust
- **Custom Formatters**: Plugin system for proprietary tools
- **Advanced Normalization**: Header/footer management, import organization
- **Performance Optimization**: Incremental formatting, caching
- **IDE Integration**: Real-time formatting feedback
- **Configuration Management**: Project-specific rule sets

## Related Commands

- [`goneat assess`](assess.md) - Comprehensive codebase assessment
- [`goneat hooks`](hooks.md) - Git hook management
- [`goneat version`](version.md) - Version information

## See Also

- [Work Planning Guide](../work-planning.md) - Advanced work planning features
- [Environment Variables](../../environment-variables.md) - Configuration options
- [Format Architecture](../../architecture/format-workflow.md) - Technical implementation
- [Extended File Operations](../../plans/active/v0.1.3/extended-file-operations.md) - Feature roadmap

---

**Command Status**: ✅ Implemented and tested with unified detection logic
**Last Updated**: 2025-09-15
**Author**: @forge-neat
**Supported File Types**: 15+ extensions with consistent finalizer support
**Performance**: Sub-millisecond per file
**DX Rating**: ⭐⭐⭐⭐⭐ (Unified format/assess commands)

### Generic Text Normalization (New)

A safe, format-agnostic normalizer runs on any text file (including unknown extensions) to:

- Ensure exactly one trailing newline (EOF)
- Trim trailing whitespace at end of lines
- Optionally normalize CRLF to LF
- Optionally remove UTF‑8 BOM
- Preserve Markdown hard line breaks (two trailing spaces) when enabled

Controls:

```bash
# Enable/disable generic text normalization (default: enabled)
goneat format --text-normalize                # on unknown text files too

# Encoding policy (default: utf8-only)
goneat format --text-encoding-policy=utf8-only   # safest
goneat format --text-encoding-policy=utf8-or-bom # allow UTF-8 + UTF-8 BOM
goneat format --text-encoding-policy=any-text    # any UTF-8 text heuristics

# Preserve Markdown hard breaks (default: true)
goneat format --preserve-md-linebreaks
```

Notes:

- Unknown or non‑UTF‑8 encodings are skipped by default (no changes) to prevent corruption.
- Use `.goneatignore` and `.gitignore` to exclude paths from normalization.
- Markdown: when preserving hard breaks, trailing spaces are collapsed to exactly two.
