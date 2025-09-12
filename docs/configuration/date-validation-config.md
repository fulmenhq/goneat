# Date Validation Configuration

Configure date validation patterns and file detection for your project's specific needs.

## Overview

Date validation in goneat is designed to be flexible and configurable. While it comes with sensible defaults, you can customize which files are checked and what date patterns are detected.

## Default Configuration

### File Patterns

By default, date validation checks these file patterns:

```go
filesToCheck := []string{
    "CHANGELOG.md",           // Project changelog
    "RELEASE_NOTES.md",       // Release notes
    "docs/releases/",         // Release documentation
    "docs/ops/compliance/",   // Compliance docs
    "docs/sop/",             // Standard operating procedures
}
```

### Date Patterns

The validator recognizes these date formats:

```go
datePatterns := []string{
    `(\d{4})-(\d{2})-(\d{2})`,     // YYYY-MM-DD (ISO 8601)
    `(\d{4})/(\d{2})/(\d{2})`,     // YYYY/MM/DD
    `(\d{4})\.(\d{2})\.(\d{2})`,   // YYYY.MM.DD
}
```

### File Type Detection

Text files are detected by extension:

```go
textExts := []string{
    ".md", ".txt", ".go", ".yaml", ".yml",
    ".json", ".toml", ".ini", ".cfg", ".conf"
}
```

## Customization Options

### 1. External Configuration File

The easiest way to customize date validation is through a `dates.yaml` configuration file:

**Location:** `.goneat/dates.yaml` (project root) or `$HOME/.goneat/dates.yaml` (global)

```yaml
# Example dates.yaml configuration
enabled: true
rules:
  future_dates:
    enabled: true
    severity: "error" # Options: critical, high, medium, low, info
    max_skew: "24h" # Allow dates up to 24 hours in future
    auto_fix: false
  monotonic_order:
    enabled: true
    severity: "error" # Change from default "warning" to "error"
    files: ["CHANGELOG.md", "HISTORY.md"]
    check_top_n: 10 # Check top N entries for ordering
  stale_entries:
    enabled: true
    severity: "warning"
    warn_days: 180 # Warn about entries older than 180 days
```

### 2. Severity Level Configuration

Date validation issues can be configured with different severity levels:

```yaml
# dates.yaml - Change monotonic ordering from warning to error
rules:
  monotonic_order:
    severity: "error" # Default is "warning"
```

**Available Severity Levels:**

- `critical`: Highest severity, always fails
- `high`: High severity issues
- `medium`: Medium severity issues (default for warnings)
- `low`: Low severity issues
- `info`: Informational only

**Common Configurations:**

```yaml
# Strict configuration - fail on all date issues
rules:
  future_dates:
    severity: "critical"
  monotonic_order:
    severity: "high"
  stale_entries:
    severity: "medium"

# Relaxed configuration - warnings only
rules:
  future_dates:
    severity: "medium"
  monotonic_order:
    severity: "low"
  stale_entries:
    severity: "info"
```

### 3. Adding Custom File Patterns

To add custom file patterns, you can:

**Option A: Configuration File (Recommended)**

```yaml
# dates.yaml
rules:
  monotonic_order:
    files:
      - "CHANGELOG.md"
      - "RELEASE_NOTES.md"
      - "docs/releases/"
      - "docs/announcements/"
      - "HISTORY.md"
      - "NEWS.md"
```

**Option B: Fork and Modify (Advanced)**

1. Fork the goneat repository
2. Edit `internal/assess/date_validation_runner.go`
3. Modify the `filesToCheck` slice:

```go
filesToCheck := []string{
    "CHANGELOG.md",
    "RELEASE_NOTES.md",
    "docs/releases/",
    "docs/ops/compliance/",
    "docs/sop/",
    "HISTORY.md",              // Add custom patterns
    "NEWS.md",
    "docs/announcements/",
    "*.changelog",             // Wildcard patterns
}
```

**Option B: Use .goneatignore**
Add files you want to exclude from date validation:

```gitignore
# .goneatignore
docs/announcements/
HISTORY.md
NEWS.md
```

### 2. Adding Custom Date Patterns

To add custom date patterns:

```go
datePatterns := []string{
    `(\d{4})-(\d{2})-(\d{2})`,           // YYYY-MM-DD
    `(\d{4})/(\d{2})/(\d{2})`,           // YYYY/MM/DD
    `(\d{4})\.(\d{2})\.(\d{2})`,         // YYYY.MM.DD
    `(\d{1,2})/(\d{1,2})/(\d{4})`,       // MM/DD/YYYY (US format)
    `(\d{1,2})-(\d{1,2})-(\d{4})`,       // MM-DD-YYYY
    `(\d{4})(\d{2})(\d{2})`,             // YYYYMMDD (compact)
}
```

### 3. Language-Specific Extensions

For multi-language projects, you can add language-specific patterns:

```go
// Add to date_validation_runner.go
languagePatterns := map[string][]string{
    "go":     {"go.mod", "VERSION", "*.go"},
    "js":     {"package.json", "package-lock.json"},
    "python": {"setup.py", "pyproject.toml", "setup.cfg"},
    "rust":   {"Cargo.toml", "Cargo.lock"},
    "java":   {"pom.xml", "build.gradle"},
}
```

## Configuration Examples

### Example 1: Node.js Project

```go
filesToCheck := []string{
    "CHANGELOG.md",
    "RELEASE_NOTES.md",
    "package.json",           // Check package.json for date fields
    "docs/releases/",
    "docs/announcements/",
}

datePatterns := []string{
    `(\d{4})-(\d{2})-(\d{2})`,     // ISO 8601
    `(\d{4})/(\d{2})/(\d{2})`,     // Slash format
    `"date":\s*"(\d{4})-(\d{2})-(\d{2})"`, // JSON date fields
}
```

### Example 2: Python Project

```go
filesToCheck := []string{
    "CHANGELOG.rst",          // reStructuredText
    "HISTORY.rst",
    "setup.py",               // Python setup files
    "pyproject.toml",
    "docs/releases/",
}

datePatterns := []string{
    `(\d{4})-(\d{2})-(\d{2})`,           // ISO 8601
    `(\d{4})/(\d{2})/(\d{2})`,           // Slash format
    `(\d{4})-(\d{1,2})-(\d{1,2})`,       // Flexible separators
}
```

### Example 3: Multi-Language Monorepo

```go
filesToCheck := []string{
    "CHANGELOG.md",
    "RELEASE_NOTES.md",
    "docs/releases/",
    "docs/announcements/",
    "packages/*/CHANGELOG.md",    // Package-specific changelogs
    "packages/*/package.json",    // Package.json files
    "packages/*/Cargo.toml",      // Rust packages
}

datePatterns := []string{
    `(\d{4})-(\d{2})-(\d{2})`,           // ISO 8601
    `(\d{4})/(\d{2})/(\d{2})`,           // Slash format
    `(\d{4})\.(\d{2})\.(\d{2})`,         // Dot format
    `"date":\s*"(\d{4})-(\d{2})-(\d{2})"`, // JSON dates
    `date\s*=\s*"(\d{4})-(\d{2})-(\d{2})"`, // TOML dates
}
```

## Advanced Configuration

### Custom File Detection

To add custom file type detection:

```go
func (r *DateValidationAssessmentRunner) isTextFile(path string) bool {
    ext := strings.ToLower(filepath.Ext(path))

    // Default text extensions
    textExts := []string{".md", ".txt", ".go", ".yaml", ".yml", ".json", ".toml", ".ini", ".cfg", ".conf"}
    for _, textExt := range textExts {
        if ext == textExt {
            return true
        }
    }

    // Custom extensions
    customExts := []string{".rst", ".adoc", ".tex", ".org"}
    for _, customExt := range customExts {
        if ext == customExt {
            return true
        }
    }

    // Check for files without extensions
    if ext == "" {
        return true
    }

    return false
}
```

### Custom Date Validation Logic

To add custom date validation logic:

```go
func (r *DateValidationAssessmentRunner) isFutureDate(year, month, day, currentYear, currentMonth, currentDay int) bool {
    // Default future date check
    if year > currentYear {
        return true
    }
    if year == currentYear && month > currentMonth {
        return true
    }
    if year == currentYear && month == currentMonth && day > currentDay {
        return true
    }

    // Custom logic: allow dates up to 7 days in the future for planning
    if year == currentYear && month == currentMonth {
        if day > currentDay && day <= currentDay+7 {
            return false // Allow up to 7 days in future
        }
    }

    return false
}
```

## Integration with Project Structure

### Monorepo Structure

```
project/
├── packages/
│   ├── frontend/
│   │   ├── CHANGELOG.md
│   │   └── package.json
│   ├── backend/
│   │   ├── CHANGELOG.md
│   │   └── Cargo.toml
│   └── shared/
│       ├── CHANGELOG.md
│       └── package.json
├── docs/
│   ├── releases/
│   └── announcements/
└── CHANGELOG.md
```

**Configuration:**

```go
filesToCheck := []string{
    "CHANGELOG.md",
    "packages/*/CHANGELOG.md",
    "packages/*/package.json",
    "packages/*/Cargo.toml",
    "docs/releases/",
    "docs/announcements/",
}
```

### Microservices Structure

```
services/
├── api-gateway/
│   ├── CHANGELOG.md
│   └── package.json
├── user-service/
│   ├── CHANGELOG.md
│   └── Cargo.toml
├── payment-service/
│   ├── CHANGELOG.md
│   └── go.mod
└── docs/
    ├── releases/
    └── compliance/
```

**Configuration:**

```go
filesToCheck := []string{
    "services/*/CHANGELOG.md",
    "services/*/package.json",
    "services/*/Cargo.toml",
    "services/*/go.mod",
    "docs/releases/",
    "docs/compliance/",
}
```

## Testing Your Configuration

### Test with Sample Files

Create test files with future dates to verify your configuration:

```bash
# Create test files
echo "## [v1.0.0] - 2025-12-31" > test-changelog.md
echo "Release date: 2026-01-01" > test-release-notes.md

# Run date validation
goneat assess --categories date-validation

# Clean up
rm test-changelog.md test-release-notes.md
```

### Validate Configuration

```bash
# Run with verbose output to see which files are checked
goneat assess --categories date-validation --verbose

# Run with JSON output for programmatic validation
goneat assess --categories date-validation --format json | jq '.categories["date-validation"]'
```

## Best Practices

1. **Start with Defaults**: Use the default configuration first, then customize as needed
2. **Test Thoroughly**: Test your configuration with sample files before deploying
3. **Document Changes**: Document any customizations in your project's README
4. **Version Control**: Keep configuration changes in version control
5. **Team Alignment**: Ensure your team understands the configuration choices

## Troubleshooting

### Common Issues

**1. Files Not Being Checked**

- Verify file patterns match your project structure
- Check that files have supported extensions
- Ensure files are not in `.goneatignore`

**2. False Positives**

- Review date patterns for your specific use case
- Consider adding custom validation logic
- Use `.goneatignore` for files that should be excluded

**3. Performance Issues**

- Limit file patterns to necessary files only
- Use specific patterns rather than broad wildcards
- Consider excluding large directories

### Debug Mode

Enable debug logging to troubleshoot issues:

```bash
# Set debug environment variable
export GONEAT_DEBUG=1

# Run date validation
goneat assess --categories date-validation --verbose
```
