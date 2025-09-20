# Date Validation Configuration

The dates feature automatically scans your codebase for future dates in documentation and configuration files. This prevents common mistakes like accidentally using tomorrow's date or incorrect year when creating releases, changelogs, or documentation.

## Configuration File Structure

Date validation is controlled by `.goneat/dates.yaml` (or `.goneat/dates.json`) in your project root. The configuration uses a declarative YAML format with the following structure:

### Core Sections

#### `enabled` (boolean)

Enable or disable date validation entirely.

**Default:** `true`

```yaml
enabled: true # Set to false to disable date validation
```

#### `date_patterns` (array)

Define regex patterns to extract dates from files. Each pattern must capture exactly 3 groups: year, month, day.

**Required:** At least one pattern
**Default:** ISO 8601 format only

```yaml
date_patterns:
  # ISO 8601 (YYYY-MM-DD) - most common
  - regex: "(\\d{4})-(\\d{2})-(\\d{2})"
    order: "YMD" # Year-Month-Day order
    description: "ISO 8601 date format"
    files: ["**/*.md", "**/*.yaml"] # Optional: apply to specific files

  # US format (MM/DD/YYYY)
  - regex: "(\\d{1,2})/(\\d{1,2})/(\\d{4})"
    order: "MDY" # Month-Day-Year order
    description: "US date format"
    files: ["legacy/**"] # Only for legacy documentation

  # European format (DD.MM.YYYY)
  - regex: "(\\d{1,2})\\.(\\d{1,2})\\.(\\d{4})"
    order: "DMY" # Day-Month-Year order
    description: "European date format"
```

**Pattern Requirements:**

- Regex must have exactly 3 capture groups: `(year)(month)(day)`
- `order` must be one of: `"YMD"`, `"MDY"`, `"DMY"`
- `description` is for documentation only
- `files` is optional (if omitted, applies to all files)

#### `rules` (object)

Define validation rules to apply to extracted dates.

**Required:** At least one rule
**Default:** Basic future date checking

```yaml
rules:
  # Prevent future dates
  future_dates:
    enabled: true
    max_skew: "24h" # Allow dates up to 24 hours in future
    severity: "error" # error, warning, info, debug
    description: "Prevents accidentally committing future dates"
    grace_period: "7d" # Optional: allow planned future dates within grace period

  # Ensure chronological order (changelogs)
  monotonic_order:
    enabled: true
    files: # Apply only to specific files
      - "CHANGELOG.md"
      - "**/CHANGELOG*.md"
      - "RELEASE_NOTES.md"
      - "docs/releases/**"
    severity: "warning"
    description: "Ensures changelog entries appear in chronological order"
    allow_duplicates: false # Allow multiple entries with same date

  # AI safety features (catches common AI-generated errors)
  ai_safety:
    enabled: true
    detect_placeholders: true # Catches "2025-09-01" patterns in examples
    detect_impossible: true # Dates before repo creation (with small grace window)
    severity: "medium"
    placeholder_patterns: # Custom placeholder detection
      - "20XX-\\d{2}-\\d{2}" # Year placeholders
      - "\\d{4}-XX-\\d{2}" # Month placeholders
```

**Rule Types:**

- `future_dates`: Blocks commits with dates in the future
- `monotonic_order`: Ensures dates appear in descending chronological order
- `ai_safety`: Catches common AI-generated date mistakes (placeholders, impossible chronology)

#### `exclusions` (array)

Exclude files or patterns that contain synthetic/example dates.

**Default:** Common false-positive patterns
**Format:** Array of exclusion objects

```yaml
exclusions:
  # Documentation (common source of false positives)
  - pattern: "docs/**"
    reason: "Documentation examples use future/hypothetical dates for illustration"
    files: ["**/*.md", "**/*.rst"] # Specific file types within pattern

  # Test data and fixtures
  - pattern: "tests/**"
    reason: "Test data with synthetic dates for validation purposes"
  - pattern: "test-fixtures/**"
    reason: "Test data with synthetic dates"
  - pattern: "**/fixtures/**"
    reason: "Test fixtures with artificial dates"

  # Code examples and documentation
  - pattern: "**/examples/**"
    reason: "Code examples with illustrative dates"
  - pattern: "**/tutorials/**"
    reason: "Tutorial content with hypothetical timelines"

  # Internal and generated content
  - pattern: "internal/assets/**"
    reason: "Embedded/generated content with arbitrary dates"
  - pattern: "**/generated/**"
    reason: "Auto-generated files with build timestamps"

  # Dependencies
  - pattern: "**/vendor/**"
    reason: "Third-party dependencies with package metadata dates"
  - pattern: "**/node_modules/**"
    reason: "JavaScript dependencies with package dates"
  - pattern: "**/bower_components/**"
    reason: "Legacy JS dependencies"

  # Build artifacts
  - pattern: "build/**"
    reason: "Build outputs with generation dates"
  - pattern: "dist/**"
    reason: "Distribution artifacts"
  - pattern: "**/*.min.*"
    reason: "Minified files with build timestamps"

  # Logs and temp files
  - pattern: "**/*.log"
    reason: "Log files with timestamps"
  - pattern: "*.tmp"
    reason: "Temporary files"

  # IDE and OS files
  - pattern: "**/.DS_Store"
    reason: "macOS metadata"
  - pattern: "**/Thumbs.db"
    reason: "Windows thumbnails"
```

**Exclusion Format:**

- `pattern`: Glob pattern or regex (supports `**` for recursion)
- `reason`: Human-readable explanation (for documentation)
- `files`: Optional array of file extensions within the pattern
- `extract_from`: Optional: `"content"`, `"comments"`, `"metadata"` (for code files)

#### `file_types` (object)

Apply different rules based on file type.

**Default:** Basic type detection
**Purpose:** Different severity levels for documentation vs. release files

```yaml
file_types:
  # Documentation files (lower severity - often illustrative)
  markdown:
    severity_modifier: "low" # Reduce severity for all markdown issues
    patterns: # Type-specific patterns
      - regex: "20\\d{2}-\\d{2}-\\d{2}"
        description: "Four-digit years (more likely real dates)"
        severity: "medium" # Override default low severity
      - regex: "\\d{1,2}/\\d{1,2}/\\d{2,4}"
        description: "Slash-separated dates (often examples)"
        severity: "low"
    files: ["**/*.md", "**/*.rst", "**/*.txt"]

  # Configuration files (medium priority)
  yaml:
    severity_modifier: "medium"
    patterns:
      - regex: "(?:created|updated|timestamp):\\s*(\\d{4})-(\\d{2})-(\\d{2})"
        description: "YAML metadata timestamps"
        severity: "high" # Configuration timestamps are critical
      - regex: "date:\\s*(\\d{4})-(\\d{2})-(\\d{2})"
        description: "Explicit YAML date fields"
        severity: "medium"
    files: ["**/*.yaml", "**/*.yml"]

  # JSON configuration
  json:
    severity_modifier: "medium"
    patterns:
      - regex: "\"(?:date|created|updated|timestamp)\"\\s*:\\s*\"(\\d{4})-(\\d{2})-(\\d{2})\""
        description: "JSON date fields"
        severity: "high"
    files: ["**/*.json", "**/package.json"]

  # Changelogs (highest priority)
  changelog:
    severity_modifier: "high"
    patterns:
      - regex: "\\d{4}-\\d{2}-\\d{2}"
        description: "Standard changelog dates"
        severity: "critical" # Release history is mission-critical
        files: ["CHANGELOG.*", "**/CHANGELOG.*", "releases/*"]

  # Package manifests (dependency dates)
  package_manifest:
    severity_modifier: "medium"
    patterns:
      - regex: "\"version\"\\s*:\\s*\"(\\d{1,3}\\.\\d{1,3}\\.\\d{1,3})\""
        description: "Package version dates (semantic versioning)"
        severity: "low"
      - regex: "\"published\"\\s*:\\s*\"(\\d{4})-(\\d{2})-(\\d{2})\""
        description: "Package publish dates"
        severity: "high"
    files:
      [
        "**/package.json",
        "**/Cargo.toml",
        "**/pyproject.toml",
        "**/composer.json",
      ]

  # Code files (extract from comments only)
  go:
    severity_modifier: "low"
    extract_from: "comments" # Only extract from comments, not string literals
    patterns:
      - regex: "//\\s*(\\d{4})-(\\d{2})-(\\d{2})"
        description: "Go comments with dates"
        severity: "info"
    files: ["**/*.go"]
```

**Type Detection:**

- Files are classified by extension and content patterns
- Multiple types can match a single file (e.g., YAML changelog)
- Type-specific rules override global rules
- `severity_modifier` adjusts all rules for that type: `"low"`, `"medium"`, `"high"`, `"critical"`

### Performance Configuration

#### `max_file_size` (string)

Skip files larger than specified size to prevent performance issues.

**Default:** `"10MB"`
**Format:** Go byte units (`"1MB"`, `"10MB"`, `"1GB"`, etc.)

```yaml
max_file_size: "5MB" # Skip files > 5MB (logs, minified JS, etc.)
```

#### `max_date_count` (integer)

Limit number of dates extracted per file to prevent regex DoS attacks.

**Default:** `1000`
**Range:** `1-10000`

```yaml
max_date_count: 500 # Stop after finding 500 dates in a single file
```

#### `parallel_workers` (integer)

Number of concurrent file processors.

**Default:** `runtime.NumCPU()`
**Recommended:** `2-8` for typical machines

```yaml
parallel_workers: 4 # Use 4 cores for processing
```

### Output Configuration

#### `output_format` (string)

Control the format of assessment output.

**Default:** `"markdown"`
**Options:** `"markdown"`, `"json"`, `"html"`, `"concise"`, `"silent"`

```yaml
output_format: "markdown" # Human-readable with context
# output_format: "json"    # Machine-readable for CI/CD
# output_format: "concise" # One-line summary only
```

#### `show_context` (boolean)

Include surrounding lines for date issues.

**Default:** `true`

```yaml
show_context: true # Show 2 lines before/after each date issue
```

#### `group_by_file` (boolean)

Group issues by file instead of by type.

**Default:** `true`

```yaml
group_by_file: true # "file: issues" vs "type: files with issues"
```

## Complete Example Configuration

Here's a production-ready configuration for a typical Go/JavaScript monorepo:

````yaml
# .goneat/dates.yaml - Production Configuration
#
# Comprehensive date validation for a Go/JavaScript monorepo
# Covers changelogs, release notes, package metadata, and documentation
#

enabled: true

# Date extraction patterns
date_patterns:
  # Primary: ISO 8601 (used in most modern projects)
  - regex: "(\\d{4})-(\\d{2})-(\\d{2})"
    order: "YMD"
    description: "ISO 8601 (YYYY-MM-DD) - changelogs, configs, modern docs"

  # Secondary: US format (legacy docs, comments)
  - regex: "(\\d{1,2})/(\\d{1,2})/(\\d{4})"
    order: "MDY"
    description: "US format (MM/DD/YYYY) - legacy content, comments"

  # European format (international teams)
  - regex: "(\\d{1,2})\\.(\\d{2})\\.(\\d{4})"
    order: "DMY"
    description: "European format (DD.MM.YYYY)"

# Validation rules
rules:
  # Block future dates (critical for release accuracy)
  future_dates:
    enabled: true
    max_skew: "24h"        # Allow clock drift
    grace_period: "30d"    # Allow planned releases within 30 days
    severity: "error"

  # Ensure changelog ordering (important but fixable)
  monotonic_order:
    enabled: true
    files:
      # Core changelog files
      - "CHANGELOG.md"
      - "**/CHANGELOG*.md"
      - "**/CHANGELOG.yaml"
      # Release artifacts
      - "RELEASE_NOTES.md"
      - "RELEASE_NOTES.yaml"
      - "docs/releases/**"
      # Monorepo packages
      - "packages/*/CHANGELOG.md"
      - "apps/*/CHANGELOG.md"
      - "libs/*/CHANGELOG.md"
    severity: "warning"    # Don't block CI, but flag for review
    allow_duplicates: true # Allow multiple entries on same date

  # AI safety (catches common documentation mistakes)
  ai_safety:
    enabled: true
    detect_placeholders: true    # "2025-09-01", "XXXX-XX-XX"
    detect_impossible: true      # Dates before repo creation
    severity: "medium"

# Smart exclusions (reduce false positives)
exclusions:
  # Documentation (contains illustrative dates)
  - pattern: "docs/**"
    reason: "Documentation examples use future/hypothetical dates"
    files: ["**/*.md", "**/*.rst", "**/*.txt"]

  # Code examples and tutorials
  - pattern: "**/examples/**"
    reason: "Code examples with synthetic dates"
  - pattern: "**/tutorials/**"
    reason: "Tutorial content with hypothetical timelines"
  - pattern: "**/samples/**"
    reason: "Sample code with example dates"

  # Test data
  - pattern: "tests/**"
    reason: "Test data with synthetic dates"
  - pattern: "test-fixtures/**"
    reason: "Test fixtures with artificial dates"
  - pattern: "**/fixtures/**"
    reason: "Generic test fixtures"

  # Internal/generated content
  - pattern: "internal/**"
    reason: "Internal tools and generated content"
  - pattern: "**/generated/**"
    reason: "Auto-generated files with timestamps"
  - pattern: "internal/assets/**"
    reason: "Embedded assets with metadata dates"

  # Dependencies (lots of date metadata)
  - pattern: "**/vendor/**"
    reason: "Go vendor directory"
  - pattern: "**/node_modules/**"
    reason: "JavaScript dependencies"
  - pattern: "**/bower_components/**"
    reason: "Legacy JS dependencies"

  # Build artifacts
  - pattern: "build/**"
    reason: "Build outputs with generation dates"
  - pattern: "dist/**"
    reason: "Distribution artifacts"
  - pattern: "**/*.min.*"
    reason: "Minified files with build timestamps"

  # Logs and temp files
  - pattern: "**/*.log"
    reason: "Log files with timestamps"
  - pattern: "*.tmp"
    reason: "Temporary files"

  # IDE and OS files
  - pattern: "**/.DS_Store"
    reason: "macOS metadata"
  - pattern: "**/Thumbs.db"
    reason: "Windows thumbnails"

# File type specific rules
file_types:
  # Documentation (illustrative, lower priority)
  markdown:
    severity_modifier: "low"
    patterns:
      - regex: "20\\d{2}-\\d{2}-\\d{2}"
        description: "Full four-digit years (more likely real)"
        severity: "medium"
      - regex: "\\d{1,2}/\\d{1,2}/\\d{2,4}"
        description: "Slash dates (often examples)"
        severity: "low"
    files: ["**/*.md", "**/*.rst", "**/*.txt", "**/*.markdown"]

  # Configuration files (timestamps matter)
  yaml:
    severity_modifier: "medium"
    patterns:
      - regex: "(?:created|updated|timestamp):\\s*(\\d{4})-(\\d{2})-(\\d{2})"
        description: "YAML metadata timestamps"
        severity: "high"  # Configuration timestamps are critical
      - regex: "date:\\s*(\\d{4})-(\\d{2})-(\\d{2})"
        description: "Explicit YAML date fields"
        severity: "medium"
    files: ["**/*.yaml", "**/*.yml"]

  # JSON configuration
  json:
    severity_modifier: "medium"
    patterns:
      - regex: "\"(?:date|created|updated|timestamp)\"\\s*:\\s*\"(\\d{4})-(\\d{2})-(\\d{2})\""
        description: "JSON date fields"
        severity: "high"
    files: ["**/*.json", "**/package.json"]

  # Changelogs (release history - highest priority)
  changelog:
    severity_modifier: "high"
    patterns:
      - regex: "\\d{4}-\\d{2}-\\d{2}"
        description: "Standard changelog dates"
        severity: "critical"  # Release history is mission-critical
        files: ["CHANGELOG.*", "**/CHANGELOG.*", "RELEASE_NOTES.*", "releases/*"]

  # Package manifests (dependency dates)
  package_manifest:
    severity_modifier: "medium"
    patterns:
      - regex: "\"version\"\\s*:\\s*\"(\\d{1,3}\\.\\d{1,3}\\.\\d{1,3})\""
        description: "Package version dates (semantic versioning)"
        severity: "low"
      - regex: "\"published\"\\s*:\\s*\"(\\d{4})-(\\d{2})-(\\d{2})\""
        description: "Package publish dates"
        severity: "high"
    files: ["**/package.json", "**/Cargo.toml", "**/pyproject.toml", "**/composer.json"]

  # Code files (mostly comments)
  go:
    severity_modifier: "low"
    extract_from: "comments"  # Only extract from comments, not string literals
    patterns:
      - regex: "//\\s*(\\d{4})-(\\d{2})-(\\d{2})"
        description: "Go comments with dates"
        severity: "info"
    files: ["**/*.go"]

# Performance optimization
max_file_size: "2MB"       # Skip files larger than 2MB
max_date_count: 1000       # Maximum dates to extract per file
parallel_workers: 6        # Use 6 CPU cores for parallel processing
cache_results: true        # Cache file scanning results

# Output preferences
output_format: "markdown"  # Human-readable with rich formatting
show_context: true         # Include 2 lines of context around each issue
group_by_file: true        # Group issues by file (easier to fix)
max_issues_per_file: 10    # Limit issues shown per file (prevents spam)


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
````

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

### 4. Layered Ignore System

Date validation uses a sophisticated layered ignore system that respects multiple ignore files with different priorities:

#### Ignore Layers (Highest to Lowest Priority)

1. **Default Patterns** (Always Ignored)
   - `.git/**` - Git repository files
   - `node_modules/**` - Node.js dependencies
   - `.scratchpad/**` - Scratch/temporary files
   - These cannot be overridden

2. **.gitignore** (Foundation)
   - Standard git ignore patterns
   - Applied to all files in the repository

3. **.goneatignore** (Repository Overrides)
   - Project-specific exclusions
   - Can override .gitignore patterns
   - Located in repository root

4. **~/.goneat/.goneatignore** (User Overrides)
   - Personal exclusions
   - Can override all repository-level patterns
   - Located in user's goneat configuration directory

#### Ignore File Format

All ignore files use standard `.gitignore` syntax:

```gitignore
# .goneatignore - Repository-level overrides

# Exclude entire directories
docs/drafts/
temp/
build/

# Exclude specific files
*.tmp
*.bak
DRAFT.md

# Exclude by pattern
**/temp/**
**/cache/**

# Include previously excluded files (using !)
!docs/drafts/important.md
```

#### Examples

**Override .gitignore exclusions:**

```gitignore
# .gitignore
docs/drafts/

# .goneatignore (overrides .gitignore)
!docs/drafts/important.md  # Include this specific file
```

**User-level exclusions:**

```gitignore
# ~/.goneat/.goneatignore
**/node_modules/**        # Exclude all node_modules (already default)
**/.DS_Store             # Exclude macOS system files
**/Thumbs.db             # Exclude Windows system files
```

#### Debugging Ignore Patterns

Use debug logging to see which files are being ignored:

```bash
goneat dates check --log-level debug
```

This will show:

- Which files are being processed
- Which files are being ignored and why
- The ignore patterns that matched

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
echo "## [v1.0.0] - 2025-09-19" > test-changelog.md
echo "Release date: 2025-09-20" > test-release-notes.md

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
