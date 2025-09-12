# Dates

Validate dates in documentation and configuration files to prevent embarrassing date mistakes.

## Overview

The dates feature automatically scans your codebase for future dates in documentation and configuration files. This prevents common mistakes like accidentally using tomorrow's date or incorrect year when creating releases, changelogs, or documentation.

## Important Limitations

**This tool can only detect obvious future dates - it cannot determine if historical dates are "wrong".**

### What the Tool CAN Do

- ✅ Catch obvious future dates (e.g., `2026-01-01` when today is `2025-01-01`)
- ✅ Catch typos in dates (e.g., `2026-01-01` when you meant `2025-01-01`)
- ✅ Prevent new mistakes when adding changelog entries
- ✅ Validate the latest entry in your changelog
- ✅ **Warn about stale changelog entries** (e.g., "latest entry is 6 months old")
- ✅ **Support all major programming languages** and project types
- ✅ **Detect dates in package manager files** (package.json, Cargo.toml, etc.)

### What the Tool CANNOT Do

- ❌ Determine if historical dates are accurate (e.g., was v1.2.3 really released on 2025-01-15?)
- ❌ Know your project's actual release timeline
- ❌ Understand context about planned vs actual release dates
- ❌ Fix existing incorrect dates in your changelog
- ❌ Understand project-specific release schedules

**For existing incorrect dates in your CHANGELOG.md, you'll need to fix those manually.** The tool's value is in preventing NEW mistakes and alerting you to potentially stale documentation.

## Usage

### Basic Assessment

```bash
# Run date validation as part of comprehensive assessment
goneat assess

# Run only date validation
goneat assess --categories dates

# Run with JSON output for automation
goneat assess --categories dates --format json
```

### Hook Integration

Date validation is automatically included in git hooks:

```bash
# Pre-commit hook (format+lint+dates)
goneat assess --hook=pre-commit

# Pre-push hook (format+lint+security+dates)
goneat assess --hook=pre-push
```

## What Gets Checked

### File Patterns

The date validation scans these file patterns by default:

#### Standard Changelog Files

- `CHANGELOG.md`, `CHANGELOG`, `CHANGELOG.txt`, `CHANGELOG.rst`
- `CHANGES.md`, `CHANGES`, `CHANGES.txt`, `CHANGES.rst`
- `HISTORY.md`, `HISTORY`, `HISTORY.txt`, `HISTORY.rst`
- `NEWS.md`, `NEWS`, `NEWS.txt`, `NEWS.rst`
- `RELEASE_NOTES.md`, `RELEASE_NOTES`, `RELEASE_NOTES.txt`
- `RELEASES.md`, `RELEASES`, `RELEASES.txt`
- `VERSION.md`, `VERSION`, `VERSION.txt`

#### Documentation Directories

- `docs/releases/`, `docs/changelog/`, `docs/history/`, `docs/news/`
- `doc/releases/`, `doc/changelog/`, `doc/history/`, `doc/news/`
- `documentation/releases/`, `documentation/changelog/`

#### Language-Specific Patterns

- **JavaScript/TypeScript**: `**/package.json`, `**/package-lock.json`, `**/yarn.lock`
- **Rust**: `**/Cargo.toml`, `packages/*/Cargo.toml`
- **Python**: `**/pyproject.toml`, `**/setup.py`, `**/requirements.txt`
- **Java**: `**/pom.xml`, `**/build.gradle`
- **PHP**: `**/composer.json`
- **Go**: `**/go.mod`, `**/go.sum`
- **Ruby**: `**/Gemfile`, `**/Gemfile.lock`
- **Dart**: `**/pubspec.yaml`
- **Elixir**: `**/mix.exs`
- **And many more...**

#### Monorepo Support

- `packages/*/CHANGELOG.md`, `apps/*/CHANGELOG.md`, `libs/*/CHANGELOG.md`
- `modules/*/CHANGELOG.md`, `services/*/CHANGELOG.md`

**Note**: These patterns are configurable - you can customize them for your project's structure. The tool is designed to be language-agnostic and will work with any project type.

### Date Formats Detected

The validator recognizes these date formats:

#### International Standards

- `YYYY-MM-DD` - ISO 8601 format (e.g., `2025-09-09`)
- `YYYY/MM/DD` - Slash-separated format (e.g., `2025/09/09`)
- `YYYY.MM.DD` - Dot-separated format (e.g., `2025.09.09`)
- `YYYYMMDD` - Compact format (e.g., `20250909`)

#### Regional Formats

- `MM/DD/YYYY` - US format (e.g., `09/09/2025`)
- `DD.MM.YYYY` - European format (e.g., `09.09.2025`)
- `DD-MM-YYYY` - European format (e.g., `09-09-2025`)
- `DD/MM/YYYY` - European format (e.g., `09/09/2025`)

#### Asian Formats

- `YYYY年MM月DD日` - Japanese format (e.g., `2025年09月09日`)
- `YYYY.MM.DD` - Korean/Chinese format (e.g., `2025.09.09`)

#### Alternative Separators

- `YYYY_MM_DD` - Underscore format (e.g., `2025_09_09`)
- `YYYY MM DD` - Space format (e.g., `2025 09 09`)
- `M/D/YYYY` - Flexible format (e.g., `9/9/2025`)
- `M/D/YY` - 2-digit year (e.g., `9/9/25`)

**Note**: All formats are configurable - you can add custom patterns for your specific needs.

### Common Patterns

The validator looks for dates in these contexts:

```markdown
## [v1.2.3] - 2025-01-25 # Changelog entries

Release date: 2025-01-25 # Release notes
Created: 2025-01-25 # Documentation headers
Updated: 2025-01-25 # Update timestamps
```

## Configuration

### File Type Detection

The validator automatically detects text files by extension:

- Markdown: `.md`
- Text: `.txt`
- Go: `.go`
- YAML: `.yaml`, `.yml`
- JSON: `.json`
- Config: `.toml`, `.ini`, `.cfg`, `.conf`

### Configuration File

Date validation is fully configurable through YAML or JSON configuration files. Create a configuration file in your project's `.goneat/` directory to customize behavior.

#### Configuration File Location

The validator looks for configuration files in this order:

1. `.goneat/dates.yaml`
2. `.goneat/dates.json`

If no configuration file is found, default settings are used.

#### Canonical Configuration (v0.2.3+)

Create a `.goneat/dates.yaml` file in your project root:

```yaml
# .goneat/dates.yaml
enabled: true

files:
  include:
    - "CHANGELOG.md"
    - "docs/releases/**"
  exclude:
    - "**/node_modules/**"
    - "**/.git/**"

date_patterns:
  - regex: "(\\d{4})-(\\d{2})-(\\d{2})" # YYYY-MM-DD (ISO 8601)
    order: "YMD"

rules:
  future_dates:
    enabled: true
    max_skew: "24h" # also supports shorthand like "5d"
    severity: "error"

  stale_entries:
    enabled: true
    warn_days: 180 # warn if latest entry is older than 180 days
    severity: "warning"

  monotonic_order:
    enabled: false # opt-in by default; set to true to enforce
    files:
      - "CHANGELOG.md"
      - "docs/releases/**"
    severity: "warning"
```

Notes:

- `date_patterns` are order-aware (YMD, MDY, DMY) and require 3 capture groups.
- `max_skew` accepts Go durations (e.g., `24h`, `72h`) and `Nd` shorthand (e.g., `5d`).
- `stale_entries` checks only the latest date by design.
- Exclude paths keep the repo scan fast and focused.

#### Advanced Checks

- Monotonic Order (optional): Ensures dates appear in descending order in configured files (e.g., `CHANGELOG.md`). Useful to catch accidentally out-of-sequence entries.
- Repo-Time Plausibility (error): If any date predates the repository’s first commit (with a small grace window), an error is raised. This catches “impossible chronology” mistakes (e.g., dates months before the repository existed).

#### Legacy Configuration

Older examples in this page (`file_patterns`, `warn_stale_days`) are retained for illustration; prefer the canonical schema above for v0.2.3+.

#### Schema Validation

Configuration files are validated using JSON Schema 2020 with fast-fail validation. Invalid configurations will fall back to defaults with a warning message.

## Language-Specific Examples

### JavaScript/TypeScript Projects

```yaml
# .goneat/dates.yaml
file_patterns:
  - "CHANGELOG.md"
  - "**/package.json"
  - "packages/*/CHANGELOG.md"
  - "apps/*/package.json"
warn_stale_days: 90 # Warn if no updates in 3 months
```

### Rust Projects

```yaml
# .goneat/dates.yaml
file_patterns:
  - "CHANGELOG.md"
  - "**/Cargo.toml"
  - "crates/*/CHANGELOG.md"
  - "crates/*/Cargo.toml"
warn_stale_days: 120 # Warn if no updates in 4 months
```

### Python Projects

```yaml
# .goneat/dates.yaml
file_patterns:
  - "CHANGELOG.md"
  - "**/pyproject.toml"
  - "**/setup.py"
  - "packages/*/CHANGELOG.md"
warn_stale_days: 60 # Warn if no updates in 2 months
```

### Monorepo Projects

```yaml
# .goneat/dates.yaml
file_patterns:
  - "CHANGELOG.md"
  - "packages/*/CHANGELOG.md"
  - "apps/*/CHANGELOG.md"
  - "libs/*/CHANGELOG.md"
  - "**/package.json"
  - "**/Cargo.toml"
  - "**/pyproject.toml"
warn_stale_days: 180 # Warn if no updates in 6 months
```

## Examples

### Valid Dates (No Issues)

```markdown
## [v1.2.3] - 2025-09-09 # Today's date

Release date: 2025-09-09 # Today's date
Created: 2025-09-08 # Yesterday's date
Updated: 2025-09-07 # Past date
```

### Invalid Dates (Will Trigger Issues)

```markdown
## [v1.2.3] - 2025-01-25 # Future date (wrong month)

Release date: 2025-12-31 # Future date
Created: 2026-01-01 # Future year
Updated: 2025-09-10 # Tomorrow's date
```

### Stale Entry Warnings

```markdown
## [v1.2.3] - 2025-03-15 # Latest entry is 6 months old

Release date: 2025-03-15 # This will trigger a stale warning
```

**Note**: Stale warnings help you identify when your changelog hasn't been updated recently, which might indicate:

- The project is no longer actively maintained
- You forgot to update the changelog after a release
- The project is in a maintenance-only phase

## Output

### Success Case

```bash
$ goneat assess --categories dates
2025-09-09 21:06:47 [INFO] goneat: Starting assessment of . with 1 categories (workers=6)
2025-09-09 21:06:47 [INFO] goneat: Running date validation assessment...
2025-09-09 21:06:47 [INFO] goneat: Date validation completed: 0 issues found
Assessment health=100% | total issues: 0 | time: 154.523ms
```

### Error Case

```bash
$ goneat assess --categories dates
2025-09-09 21:06:47 [INFO] goneat: Starting assessment of . with 1 categories (workers=6)
2025-09-09 21:06:47 [INFO] goneat: Running date validation assessment...
2025-09-09 21:06:47 [INFO] goneat: Date validation completed: 2 issues found

# Codebase Assessment Report

**Generated:** 2025-09-09T21:06:47-04:00
**Tool:** goneat
**Version:** 1.0.0
**Target:** .
**Execution Time:** 154.523ms

## Executive Summary

- **Overall Health:** 🔴 Needs Attention (50%)
- **Critical Issues:** 0
- **Total Issues:** 2
- **Estimated Fix Time:** 0 seconds

## Assessment Results

### Date Validation

- **Status:** ❌ Failed
- **Issues Found:** 2
- **Files Checked:** 5

#### Issues

1. **CHANGELOG.md** - Future date found: 2025-01-25 (current date: 2025-09-09)
   - **Context:** `## [v1.2.3] - 2025-01-25`
   - **Severity:** Error
   - **Rule:** dates

2. **docs/releases/1.2.3.md** - Future date found: 2025-01-25 (current date: 2025-09-09)
   - **Context:** `Release date: 2025-01-25`
   - **Severity:** Error
   - **Rule:** dates
```

## Integration with CI/CD

### GitHub Actions

```yaml
name: Date Validation
on: [push, pull_request]

jobs:
  dates:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: Install goneat
        run: |
          curl -L https://github.com/fulmenhq/goneat/releases/latest/download/goneat_linux_amd64.tar.gz | tar -xz
          sudo mv goneat /usr/local/bin/
      - name: Run date validation
        run: goneat assess --categories dates --format json
```

### Pre-commit Hook

```yaml
# .pre-commit-config.yaml
repos:
  - repo: local
    hooks:
      - id: goneat-dates
        name: goneat date validation
        entry: goneat assess --categories dates --fail-on high
        language: system
        files: \.(md|txt|yaml|yml|json)$
```

## Troubleshooting

### Common Issues

**1. False Positives**

- The validator may flag dates that are intentionally in the future (e.g., planned release dates)
- Consider adding these files to `.goneatignore` or modifying the validation patterns

**2. Missing Files**

- The validator only checks specific file patterns
- If you have documentation in other locations, you may need to customize the patterns

**3. Date Format Issues**

- The validator only recognizes specific date formats
- Ensure your dates follow the supported patterns

### Debugging

Enable verbose output to see which files are being checked:

```bash
goneat assess --categories dates --verbose
```

## Best Practices

1. **Run Early and Often**: Include date validation in pre-commit hooks
2. **Fix Immediately**: Address date issues as soon as they're detected
3. **Use Consistent Formats**: Stick to ISO 8601 (YYYY-MM-DD) format
4. **Document Intentions**: If you need future dates, document why in comments
5. **Automate**: Include date validation in your CI/CD pipeline
6. **Manual Review**: For historical dates, manually verify accuracy
7. **Context Matters**: Remember that the tool can't understand project context

## Manual Fix Requirements

**Important**: This tool cannot fix existing incorrect dates in your changelog. You must manually correct:

- Historical release dates that are wrong
- Dates that were planned but changed
- Context-dependent date corrections
- Any dates that require project knowledge to validate

The tool's value is in preventing NEW mistakes, not fixing existing ones.

## Related Commands

- [`goneat assess`](assess.md) - Comprehensive codebase assessment
- [`goneat format`](format.md) - Code formatting and style consistency
- [`goneat hooks`](hooks.md) - Git hook management and automation
