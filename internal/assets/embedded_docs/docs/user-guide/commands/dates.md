# Dates Command

The `dates` command validates date consistency across your codebase using an **opt-in approach**, ensuring chronological accuracy in changelogs, release notes, and critical files. It prevents common mistakes like future dates, out-of-order changelog entries, and impossible chronology while focusing on files that matter for release engineering.

## Quick Start

Install goneat and run basic date validation:

```bash
# Install goneat (single command covers CLI + all libraries)
go install github.com/fulmenhq/goneat@latest

# Basic assessment (opt-in: only checks changelog/release files by default)
goneat assess --categories=dates

# Full scan (includes everything, overrides opt-in defaults)
goneat assess --categories=dates --no-exclusions

# JSON output for CI/CD
goneat assess --categories=dates --json

# Fix auto-fixable issues (reorders dates in changelogs)
goneat dates fix --dry-run  # Preview changes first
goneat dates fix           # Apply fixes
```

## How It Works

The dates command:

1. **Scans files** using configurable patterns (markdown, YAML, JSON, etc.)
2. **Extracts dates** matching defined regex patterns (ISO 8601, US format, etc.)
3. **Validates** against rules:
   - No future dates (with configurable clock skew tolerance)
   - Chronological order in changelogs (monotonic ordering)
   - No "impossible chronology" (dates before repo creation)
4. **Excludes** common false-positive locations (docs, tests, vendor) by default
5. **Reports** issues with context, severity, and estimated fix time

## Configuration File

Date validation is controlled by `.goneat/dates.yaml` in your project root. This file is **optional**—goneat uses sensible defaults, but customization is recommended for production use.

### Creating `.goneat/dates.yaml`

If you installed goneat via `go install` but haven't cloned the repository, create `.goneat/dates.yaml` manually:

```bash
# Create the configuration directory
mkdir -p .goneat

# Option 1: Generate template (requires full goneat installation with docs embedded)
goneat dates config template > .goneat/dates.yaml

# Option 2: Use the production-ready example below
# Copy the example below into .goneat/dates.yaml
```

### Production-Ready Example

Here's a comprehensive `.goneat/dates.yaml` for most projects:

```yaml
# .goneat/dates.yaml
#
# Date validation configuration for goneat
#
# This file controls date consistency checking across your repository.
# Dates are extracted from files matching the patterns below and validated
# for chronological consistency, future dates, and monotonic ordering.
#
# Key concepts:
# - Future dates: Warns about dates in the future (with configurable skew)
# - Monotonic order: Ensures dates appear in chronological order in files like CHANGELOG
# - Repository context: Uses git history to establish baseline dates
#
# Exclusions: Use patterns to skip files that contain illustrative/synthetic dates
#
# For full documentation:
#   - Terminal: goneat docs show configuration/date-validation-config
#   - Online: https://github.com/fulmenhq/goneat/blob/main/docs/configuration/date-validation-config.md
#

# Enable/disable date validation entirely
enabled: true

# Date extraction patterns (regex + expected format)
date_patterns:
  # ISO 8601 (most common in modern projects)
  - regex: "(\\d{4})-(\\d{2})-(\\d{2})"
    order: "YMD"
    description: "ISO 8601 date format (YYYY-MM-DD) - common in docs, configs, changelogs"

  # US format (common in comments and legacy docs)
  - regex: "(\\d{1,2})/(\\d{1,2})/(\\d{4})"
    order: "MDY"
    description: "US date format (MM/DD/YYYY) - common in comments and examples"

  # European format (dot separator)
  - regex: "(\\d{1,2})\\.(\\d{1,2})\\.(\\d{4})"
    order: "DMY"
    description: "European date format (DD.MM.YYYY)"

# Validation rules
rules:
  # Prevent future dates (clock skew tolerance)
  future_dates:
    enabled: true
    max_skew: "24h" # Allow 24 hours for clock differences
    severity: "error"
    description: "Prevents accidentally committing future dates (allows small clock skew)"

  # Ensure chronological order in changelogs
  monotonic_order:
    enabled: true
    files:
      # Standard changelog locations
      - "**/CHANGELOG*.md"
      - "**/CHANGELOG.yaml"
      - "**/HISTORY*.md"
      - "**/NEWS*.md"
      - "**/*changelog*.md"
      - "RELEASE_NOTES*.md"
      # Release directories
      - "docs/releases/**"
      - "releases/**"
      - "**/releases/**"
    severity: "warning" # Don't block commits, but warn about issues
    description: "Ensures changelog entries appear in chronological order"

  # AI safety features (catches common AI-generated date errors)
  ai_safety:
    enabled: true
    detect_placeholders: true # Catches "2025-09-01" patterns in examples
    detect_impossible: true # Catches dates before repo creation
    severity: "medium"

# File inclusions (opt-in approach - only check these files by default)
includes:
  # Default includes (focused on changelog and release files)
  - "CHANGELOG.md"
  - "**/CHANGELOG*.md"
  - "**/HISTORY.md"
  - "RELEASE_NOTES.md"
  - "**/RELEASE*.md"
  - "**/VERSION"

# File exclusions (merged with defaults - additive safety net)
exclusions:
  # Your custom exclusions (added to standard defaults)
  - "custom-dir/**"  # Example: exclude your custom directory
  # Standard defaults are automatically included:
  # - "**/node_modules/**", "**/.git/**", "**/dist/**", "**/build/**", "**/.scratchpad/**"

# File type specific rules (severity modifiers)
file_types:
  # Documentation (lower severity - illustrative content)
  markdown:
    severity_modifier: "low"
    patterns:
      - regex: "20\\d{2}-\\d{2}-\\d{2}"
        description: "Four-digit years (more likely to be real dates)"
        severity: "medium"
      - regex: "\\d{1,2}/\\d{1,2}/\\d{2,4}"
        description: "Slash-separated dates (often examples)"
        severity: "low"

  # Configuration files (medium priority)
  yaml:
    severity_modifier: "medium"
    patterns:
      - regex: "timestamp: \\d{4}-\\d{2}-\\d{2}"
        description: "Explicit YAML timestamps"
        severity: "high"

  # Changelogs (highest priority)
  changelog:
    severity_modifier: "high"
    patterns:
      - regex: "\\d{4}-\\d{2}-\\d{2}"
        description: "Changelog release dates"
        severity: "high"
        files: ["CHANGELOG.*", "**/CHANGELOG.*", "releases/*"]

# Performance tuning
max_file_size: "1MB" # Skip files larger than 1MB
max_date_count: 1000 # Limit date extraction per file
parallel_workers: 4 # Number of concurrent file processors

# Output preferences
output_format: "markdown" # markdown, json, html, concise
show_context: true # Include surrounding lines for context
group_by_file: true # Group issues by file rather than type
```

### Key Configuration Concepts

1. **Opt-in Includes**: Specify exactly which files to check (focused approach)
   - Default includes: `CHANGELOG.md`, `**/CHANGELOG*.md`, `RELEASE_NOTES.md`, etc.
   - Add custom patterns for your project's critical date files
   - No false positives from documentation/examples by default

2. **Date Patterns**: Define regex patterns to extract dates from files
   - Each pattern must capture exactly 3 groups (year, month, day)
   - `order` specifies the expected sequence: YMD, MDY, DMY
   - Common formats: ISO 8601 (YYYY-MM-DD), US (MM/DD/YYYY), European (DD.MM.YYYY)

3. **Validation Rules**:
   - `future_dates`: Blocks commits with future dates (allows clock skew)
   - `monotonic_order`: Ensures changelog entries are in chronological order
   - `ai_safety`: Catches common AI-generated date errors (placeholders, impossible chronology)

4. **Exclusions**: Merged with defaults for comprehensive safety net
   - Your custom exclusions are added to standard defaults
   - Standard exclusions: node_modules, .git, dist, build, .scratchpad
   - Useful for project-specific directories that should be skipped

4. **File Type Rules**: Different severity for different contexts
   - **Markdown docs**: Low severity (illustrative content)
   - **YAML configs**: Medium severity (timestamps matter)
   - **Changelogs**: High severity (release history is critical)

5. **Performance Tuning**: Limits to prevent scanning huge files or extracting thousands of dates

### Common Use Cases

#### Changelog Validation (Most Common)

Ensure CHANGELOG.md entries are chronologically ordered:

```yaml
rules:
  monotonic_order:
    enabled: true
    files:
      - "CHANGELOG.md"
      - "**/CHANGELOG*.md"
    severity: "error" # Fail builds on out-of-order entries
```

#### Release Notes Consistency

Validate dates across multiple release artifacts:

```yaml
date_patterns:
  - regex: "(\\d{4})-(\\d{2})-(\\d{2})"
    order: "YMD"
    files: ["**/*.md", "**/*.yaml", "**/*.json"]
  - regex: "(\\d{1,2})/(\\d{1,2})/(\\d{4})"
    order: "MDY"
    files: ["legacy-docs/**"] # Only for legacy content
```

#### Documentation-Focused Projects

For documentation-heavy projects, customize the includes to focus on critical files:

```yaml
includes:
  # Core release files
  - "CHANGELOG.md"
  - "RELEASE_NOTES.md"
  - "**/CHANGELOG*.md"

  # Include specific documentation that matters
  - "docs/releases/**" # Release-specific docs
  - "docs/CHANGELOG*.md" # Changelog docs

  # Exclude everything else (no exclusions needed - opt-in approach)
```

#### Code-Only Projects

For projects without documentation, use minimal includes:

```yaml
includes:
  # Only check changelog and version files
  - "CHANGELOG.md"
  - "VERSION"
```

### Command Reference

#### Assessment Commands

```bash
# Basic date validation (uses .goneat/dates.yaml if present)
goneat assess --categories=dates

# Verbose output with line context
goneat dates assess --verbose --show-context

# JSON output for automation/CI
goneat dates assess --json | jq '.issues | length'

# Scan specific files only
goneat dates assess --files CHANGELOG.md RELEASE_NOTES.md

# Override configuration (for testing)
goneat dates assess --config dates-test.yaml

# Include everything (ignore exclusions)
goneat dates assess --no-exclusions

# Dry run for fixes
goneat dates fix --dry-run --verbose
```

#### Fix Commands

```bash
# Auto-fix sortable date issues (changelogs, lists)
goneat dates fix

# Fix specific files
goneat dates fix --files CHANGELOG.md

# Preview changes first
goneat dates fix --dry-run

# Only fix high-severity issues
goneat dates fix --min-severity high
```

#### Configuration Commands

```bash
# Generate template configuration
goneat dates config template

# Validate current configuration
goneat dates config validate

# Show effective configuration (merged defaults + .goneat/dates.yaml)
goneat dates config show

# Convert legacy config to new format (if needed)
goneat dates config migrate
```

### Output Examples

#### Terminal Output (Success)

```
$ goneat assess --categories=dates
2025-09-19 11:40:00 [INFO] goneat: Starting assessment of . with 1 categories
2025-09-19 11:40:00 [INFO] goneat: Running dates assessment...
2025-09-19 11:40:00 [INFO] goneat: Scanned 127 files, found 15 dates
2025-09-19 11:40:00 [INFO] goneat: Dates assessment completed: 0 issues found

✅ Assessment health=100% | total issues: 0 | time: 245ms

Configuration applied:
- Files excluded: docs/**, tests/**, vendor/**
- Patterns: ISO 8601, US format
- Rules: future_dates (error), monotonic_order (warning)
- Customize: .goneat/dates.yaml
```

#### Terminal Output (With Issues)

```
$ goneat assess --categories=dates
2025-09-19 11:40:00 [INFO] goneat: Starting assessment of . with 1 categories
2025-09-19 11:40:00 [INFO] goneat: Running dates assessment...
2025-09-19 11:40:01 [WARN] goneat: Found 2 date issues

# Codebase Assessment Report

**Generated:** 2025-09-19T11:40:01-04:00
**Tool:** goneat v0.2.7
**Target:** .
**Execution Time:** 1.2s

## Issues Found (2 total)

### CHANGELOG.md [HIGH - future_date]
**Line:** 15
**Date:** 2025-09-19T00:00:00Z
**Context:**
```

## [v2.0.0] - 2025-09-19

- Major feature release

```
**Message:** Future date detected (85 days from now)
**Fix:** Update to realistic release date or remove date
**Auto-fixable:** No

### RELEASE_NOTES.md [MEDIUM - monotonic_order]
**Line:** 23-25
**Dates Found:** 2025-06-15, 2025-05-30, 2025-06-01
**Expected Order:** 2025-05-30, 2025-06-01, 2025-06-15
**Context:**
```

## v1.5.1 - 2025-06-15

## v1.5.0 - 2025-05-30

## v1.4.9 - 2025-06-01 # Out of order

```
**Message:** Changelog entries not in chronological order
**Fix:** Reorder entries or use `goneat dates fix`
**Auto-fixable:** Yes

## Configuration Summary
- **Enabled:** Yes
- **Files Scanned:** 127 (excluded: docs/**, tests/**, vendor/**)
- **Dates Found:** 15 total
- **Rules Applied:** future_dates (error), monotonic_order (warning)
- **Customization:** Edit `.goneat/dates.yaml` to modify behavior
- **Docs:** `goneat docs show configuration/date-validation-config`

**Next Steps:**
1. Run `goneat dates fix` to auto-sort changelog entries
2. Manually update future dates in CHANGELOG.md
3. Customize exclusions in `.goneat/dates.yaml` if needed
```

#### JSON Output (Machine-Readable)

```json
{
  "metadata": {
    "tool": "goneat",
    "version": "v0.2.7",
    "command": "assess",
    "categories": ["dates"],
    "timestamp": "2025-09-19T11:40:01-04:00",
    "target": ".",
    "execution_time_ms": 1245
  },
  "summary": {
    "overall_health": 0.85,
    "total_issues": 2,
    "critical_issues": 0,
    "estimated_fix_time_minutes": 5,
    "files_scanned": 127,
    "files_excluded": 89,
    "dates_found": 15,
    "auto_fixable_issues": 1
  },
  "configuration": {
    "enabled": true,
    "date_patterns": 2,
    "exclusions": 7,
    "rules": {
      "future_dates": { "enabled": true, "severity": "error" },
      "monotonic_order": { "enabled": true, "severity": "warning" }
    },
    "config_file": ".goneat/dates.yaml"
  },
  "issues": [
    {
      "category": "dates",
      "type": "future_date",
      "file": "CHANGELOG.md",
      "line": 15,
      "severity": "high",
      "message": "Future date detected (85 days from now)",
      "date": "2025-09-19T00:00:00Z",
      "context": "## [v2.0.0] - 2025-09-19\n- Major feature release",
      "auto_fixable": false,
      "suggested_fix": "Update to realistic release date or remove date",
      "rule": "future_dates",
      "pattern": "(\\d{4})-(\\d{2})-(\\d{2})"
    },
    {
      "category": "dates",
      "type": "monotonic_order",
      "file": "RELEASE_NOTES.md",
      "lines": [23, 24, 25],
      "severity": "medium",
      "message": "Changelog entries not in chronological order",
      "dates_found": ["2025-06-15", "2025-05-30", "2025-06-01"],
      "expected_order": ["2025-05-30", "2025-06-01", "2025-06-15"],
      "context": "## v1.5.1 - 2025-06-15\n## v1.5.0 - 2025-05-30\n## v1.4.9 - 2025-06-01",
      "auto_fixable": true,
      "suggested_fix": "Run 'goneat dates fix --files RELEASE_NOTES.md'",
      "rule": "monotonic_order",
      "affected_files": ["RELEASE_NOTES.md"]
    }
  ],
  "resolution_hints": {
    "dates": {
      "configurable": true,
      "config_file": ".goneat/dates.yaml",
      "exclude_patterns": "Add patterns to exclusions array",
      "auto_fix": "Use 'goneat dates fix' for sortable issues",
      "docs": "goneat docs show configuration/date-validation-config"
    }
  }
}
```

### Integration Examples

#### Git Hooks

Add date validation to your git workflow:

**.goneat/hooks.yaml:**

```yaml
hooks:
  pre-commit:
    commands:
      - name: dates-light
        run: goneat assess --categories=dates --fail-on high --scope=staged
        description: "Validate dates in staged changes"
        timeout: 30s
        parallel: true

  pre-push:
    commands:
      - name: dates-release
        run: |
          goneat dates assess --files CHANGELOG.md RELEASE_NOTES.md --fail-on high
          goneat dates assess --no-exclusions --severity medium --json | jq '.issues | length == 0'
        description: "Full date validation for release artifacts"
        timeout: 60s

  post-merge:
    commands:
      - name: dates-update
        run: goneat dates assess --changed-only --verbose
        description: "Check dates after merge (warnings only)"
        fail_on: low
```

#### CI/CD Pipelines

**GitHub Actions:**

```yaml
name: Code Quality
on: [push, pull_request]

jobs:
  dates:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Install goneat
        run: |
          curl -fsSL https://github.com/fulmenhq/goneat/releases/latest/download/goneat_linux_amd64.tar.gz | tar -xz
          sudo mv goneat /usr/local/bin/
          goneat version

      - name: Validate Dates
        run: |
          # Check critical files first
          if ! goneat dates assess --files CHANGELOG.md --fail-on high; then
            echo "::error::Changelog date validation failed"
            exit 1
          fi

          # Full assessment (warnings OK)
          goneat assess --categories=dates --json > dates-report.json

          # Fail only on high severity
          high_issues=$(jq '.issues | map(.severity == "high") | any' dates-report.json)
          if [ "$high_issues" = "true" ]; then
            echo "::error::High severity date issues found"
            cat dates-report.json
            exit 1
          fi

          echo "::notice::Date validation passed ($(jq '.issues | length' dates-report.json) warnings)"

      - name: Upload Report
        uses: actions/upload-artifact@v4
        with:
          name: dates-report
          path: dates-report.json
```

**Shell Script (Generic CI):**

```bash
#!/bin/bash
set -e

echo "Installing goneat..."
curl -fsSL https://github.com/fulmenhq/goneat/releases/latest/download/goneat_linux_amd64.tar.gz | tar -xz
sudo mv goneat /usr/local/bin/

echo "Validating dates..."
# Create minimal config if none exists
if [ ! -f .goneat/dates.yaml ]; then
  mkdir -p .goneat
  cat > .goneat/dates.yaml << 'EOF'
enabled: true
rules:
  future_dates:
    enabled: true
    severity: "error"
  monotonic_order:
    enabled: true
    severity: "warning"
    files:
      - "CHANGELOG*"
exclusions:
  - "docs/**"
  - "tests/**"
EOF
  echo "Created default .goneat/dates.yaml"
fi

# Run validation
goneat assess --categories=dates --json > dates-report.json

# Check for critical issues
critical=$(jq '.issues? // [] | map(.severity == "high") | any' dates-report.json)
if [ "$critical" = "true" ]; then
  echo "ERROR: High severity date issues found:"
  jq '.issues[]? | select(.severity == "high")' dates-report.json
  exit 1
fi

total_issues=$(jq '.issues? // [] | length' dates-report.json)
echo "Date validation passed ($total_issues warnings)"
```

### Troubleshooting Common Issues

#### "Need to Check Additional Files"

**Problem:** Want to validate dates in files beyond the defaults.

**Solution:** Add custom includes to `.goneat/dates.yaml`:

```yaml
includes:
  # Default includes (automatically included)
  # - "CHANGELOG.md"
  # - "**/CHANGELOG*.md"
  # - etc.

  # Add your custom files
  - "docs/releases/**" # Release documentation
  - "api/CHANGELOG.md" # API changelog
  - "SECURITY.md" # Security advisories
```

#### "Missing Date Format"

**Problem:** Your project uses a custom date format not recognized by default patterns.

**Solution:** Add custom patterns:

```yaml
date_patterns:
  # Your custom format (e.g., YYYY.MM.DD)
  - regex: "(\\d{4})\\.(\\d{2})\\.(\\d{2})"
    order: "YMD"
    description: "Custom dot-separated format (YYYY.MM.DD)"

  # Flexible month (1-12)
  - regex: "(\\d{4})-(\\d{1,2})-(\\d{1,2})"
    order: "YMD"
    description: "ISO 8601 with flexible month/day (1-12)"
```

#### "Performance Issues"

**Problem:** Large repository takes too long to scan.

**Solution:** Tune performance settings:

```yaml
# Performance tuning
max_file_size: "512KB" # Skip files > 512KB
max_date_count: 500 # Max 500 dates per file
parallel_workers: 8 # Use 8 cores
exclusions:
  - "**/*.log" # Skip log files
  - "build/**" # Skip build artifacts
  - "**/coverage/**" # Skip coverage reports
```

#### "Changelog Out-of-Order Errors"

**Problem:** `monotonic_order` rule flags correctly ordered entries.

**Solution:** Verify the rule is configured correctly and run auto-fix:

```bash
# Check current configuration
goneat dates config show | grep -A 10 monotonic_order

# Auto-fix sortable issues
goneat dates fix --files CHANGELOG.md --dry-run

# Apply fixes
goneat dates fix --files CHANGELOG.md
```

### Best Practices

1. **Start Simple**: Use the default opt-in configuration (works for most projects)
2. **Add Selectively**: Only add custom includes for files that truly need date validation
3. **Focus on Critical Files**: CHANGELOG.md and RELEASE_NOTES.md are already included by default
4. **Automate**: Include in pre-commit hooks and CI/CD pipelines
5. **Document**: Add comments to `.goneat/dates.yaml` explaining your custom includes
6. **Review Regularly**: Periodically review includes as your project adds new date-critical files
7. **Use Auto-Fix**: Leverage `goneat dates fix` for sortable issues

### Exit Codes

The dates command uses standard exit codes:

- `0`: Success (no issues or only warnings)
- `1`: General error
- `2`: Configuration error
- `64`: Invalid command line usage
- `78`: Configuration schema validation failed
- `128+N`: Fatal error signal N

**For CI/CD:**

- Use `--fail-on high` to block on critical issues only
- Use `--json` for programmatic parsing
- Check exit code `1` for any issues (including warnings)

### Related Features

- [`assess`](assess.md): Comprehensive codebase analysis including dates
- [`format`](format.md): Code formatting (dates in code comments)
- [`hooks`](hooks.md): Git hook integration for automated validation
- [`maturity`](maturity.md): Release readiness including date consistency
- [Configuration](configuration.md): Full configuration system reference

The dates command provides essential quality control for release engineering and documentation workflows, catching common mistakes before they reach production.
