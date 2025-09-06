# Version Management

Goneat provides comprehensive version management capabilities that support multiple versioning schemes and sources, ensuring zero version drift across your DevOps toolchain.

## Overview

The version management system is designed to be:

- **Multi-scheme**: Supports semantic versioning, calendar versioning, and custom patterns
- **Multi-source**: Reads from and writes to multiple version sources with priority ordering
- **Assessment-first**: Includes `--no-op` mode for safe testing and validation
- **CI/CD ready**: Perfect for automated release workflows

## Version Schemes

### Semantic Versioning (semver)

Standard semantic versioning following [semver.org](https://semver.org) specifications.

**Supported Formats:**

- `1.2.3` - Standard release
- `v1.2.3` - With 'v' prefix
- `1.2.3-alpha` - Pre-release
- `1.2.3+build.1` - With build metadata
- `v1.2.3-rc.1+build.1` - Full semver

**Bump Rules:**

- **Patch**: `1.2.3` → `1.2.4`
- **Minor**: `1.2.3` → `1.3.0`
- **Major**: `1.2.3` → `2.0.0`

### Calendar Versioning (calver)

Date-based versioning following calendar patterns.

**Supported Formats:**

- `2024.01.15` - YYYY.MM.DD
- `24.01` - YY.MM
- `2024.01` - YYYY.MM

**Planned Bump Rules:**

- **Patch**: Day increment
- **Minor**: Month increment
- **Major**: Year increment

### Custom Versioning

Regex-based custom versioning for proprietary schemes.

**Configuration:**

```yaml
version:
  method: custom
  custom:
    pattern: "^(\\d+)\\.(\\d+)\\.(\\d+)$"
    bump_logic:
      patch: "increment_last"
      minor: "increment_middle"
      major: "increment_first"
```

## Version Sources

Goneat can read from and write to multiple version sources simultaneously.

### Supported Sources

#### 1. Version Files

Simple text files containing version numbers.

```yaml
sources:
  - type: version_file
    path: VERSION
    priority: 1
```

**File Content:**

```
1.2.3
```

#### 2. Go Module Files

Reads version from go.mod module declarations.

```yaml
sources:
  - type: go_mod
    path: go.mod
    priority: 2
```

**Note:** go.mod files typically don't contain explicit versions for unreleased modules.

#### 3. Git Tags

Reads latest version from git tags.

```yaml
sources:
  - type: git_tags
    pattern: "v*"
    priority: 3
```

**Supported Patterns:**

- `v*` - All tags starting with 'v'
- `release/*` - Tags in release branch
- `*` - All tags

#### 4. Go Constants

Reads version from Go source code constants.

```yaml
sources:
  - type: version_const
    path: version.go
    pattern: 'const\s+Version\s*=\s*"([^"]+)"'
    priority: 4
```

**Go Code Example:**

```go
package main

const Version = "1.2.3"
```

#### 5. Package.json

Reads version from Node.js package files.

```yaml
sources:
  - type: package_json
    path: package.json
    priority: 5
```

## Configuration

### Basic Configuration

```yaml
version:
  method: semver
  sources:
    - type: version_file
      path: VERSION
      priority: 1
    - type: git_tags
      pattern: "v*"
      priority: 2
```

### Advanced Configuration

```yaml
version:
  method: semver
  sources:
    - type: version_file
      path: VERSION
      priority: 1
    - type: git_tags
      pattern: "v*"
      priority: 2
    - type: go_mod
      path: go.mod
      priority: 3
  bump_rules:
    semver:
      patch: micro
      minor: minor
      major: major
```

## First-Time Setup

When you run `goneat version` for the first time in a repository, goneat will intelligently detect your current version management setup:

### Automatic Detection

Goneat automatically detects version sources in this priority order:

1. **VERSION file** - Plain text file containing version
2. **Git tags** - Latest semver-formatted git tag
3. **Go module** - Version from go.mod (limited support)
4. **Go constants** - Version constants in Go source files

### Setup Guidance

If no version management is detected, goneat provides comprehensive setup guidance:

```bash
$ goneat version
🚀 Welcome to goneat version management!

📝 Quick Setup (Recommended):
  goneat version init --template basic

🔧 Manual Setup:
  1. Create a VERSION file: echo '1.0.0' > VERSION
  2. Or create a git tag: git tag v1.0.0

📋 Available Templates:
  • basic     - VERSION file with semantic versioning
  • git-tags  - Git tag-based versioning
  • calver    - Calendar versioning (YYYY.MM.DD)
  • custom    - Custom versioning scheme

💡 Pro Tips:
  • Use 'goneat version init --dry-run' to preview setup
  • Run 'goneat version --help' for all options
  • Version management is non-destructive by default
```

### Quick Setup Templates

#### Basic Template (Recommended)

```bash
# Preview setup
goneat version init basic --dry-run

# Apply setup
goneat version init basic
```

Creates a `VERSION` file with semantic versioning support.

#### Git Tags Template

```bash
# For git tag-based versioning
goneat version init git-tags
```

Sets up git tag-based versioning with automatic tag creation.

#### Calendar Versioning

```bash
# For date-based versioning
goneat version init calver
```

Creates calendar versioning with format `YYYY.MM.DD` (e.g., `2025.08.28`).

#### Custom Template

```bash
# For custom versioning schemes
goneat version init custom
```

Provides guidance for implementing custom versioning patterns.

## Usage

### Display Current Version

```bash
# Show current version from highest priority source
goneat version

# Show detailed version information with git integration
goneat version --extended

# Output in JSON format for automation
goneat version --json
```

**Extended Output Example:**

```bash
$ goneat version --extended
goneat 1.0.1
Build time: unknown
Git commit: 4896bdfc
Source: VERSION file
Git branch: main
Git status: clean
Go version: go1.25.0
Platform: darwin/arm64
```

### Version Assessment (No-Op Mode)

```bash
# Assess current version state without making changes
goneat version --no-op

# Check version consistency across all sources
goneat version check-consistency --no-op

# Validate version format
goneat version validate 1.2.3 --no-op
```

### Version Bumping

```bash
# Bump patch version (creates git tag automatically)
goneat version bump patch

# Bump minor version
goneat version bump minor

# Bump major version
goneat version bump major

# Preview bump without making changes
goneat version bump patch --no-op

# Force bump even if git operations fail
goneat version bump patch --force
```

**Git Integration:** Version bumps automatically create corresponding git tags and update VERSION files. If git tagging fails, the command continues with a warning (use `--force` to override).

### Version Setting

```bash
# Set specific version (creates git tag automatically)
goneat version set 1.2.3

# Set version with pre-release
goneat version set 1.2.3-alpha

# Preview version change
goneat version set 1.2.3 --no-op

# Force set even if git operations fail
goneat version set 1.2.3 --force
```

**Git Integration:** Version setting automatically creates corresponding git tags and updates VERSION files. If git tagging fails, the command continues with a warning (use `--force` to override).

### Version Initialization

```bash
# Initialize version management with basic template
goneat version init basic

# Initialize with git tag-based versioning
goneat version init git-tags

# Initialize with calendar versioning
goneat version init calver

# Preview initialization without making changes
goneat version init basic --dry-run

# Override existing version management
goneat version init basic --force

# Set custom initial version
goneat version init basic --initial-version 2.0.0
```

**Available Templates:**

- **`basic`**: VERSION file with semantic versioning (recommended)
- **`git-tags`**: Git tag-based versioning
- **`calver`**: Calendar versioning (YYYY.MM.DD format)
- **`custom`**: Custom versioning scheme guidance

### Validation & Consistency

```bash
# Validate version format
goneat version validate 1.2.3

# Check consistency across all sources
goneat version check-consistency

# Show version information from all sources
goneat version --all-sources
```

## Examples

### Basic Semver Workflow

```bash
# Check current version
goneat version

# Assess what patch bump would do
goneat version bump patch --no-op

# Apply the patch bump
goneat version bump patch

# Verify the change
goneat version
```

### Multi-Source Consistency

```yaml
# goneat.yaml
version:
  method: semver
  sources:
    - type: version_file
      path: VERSION
      priority: 1
    - type: git_tags
      pattern: "v*"
      priority: 2
    - type: go_mod
      path: go.mod
      priority: 3
```

```bash
# Check consistency
goneat version check-consistency

# Bump version across all sources
goneat version bump minor
```

### CI/CD Integration

```yaml
# .github/workflows/release.yml
name: Release
on:
  push:
    tags:
      - "v*"

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Setup goneat
        run: |
          # Install goneat

      - name: Validate version consistency
        run: goneat version check-consistency

      - name: Create release
        run: |
          VERSION=$(goneat version)
          gh release create $VERSION --generate-notes
```

## Version Discovery & Learning

Goneat automatically discovers version information from multiple sources with intelligent fallback logic:

### Discovery Priority Order

1. **VERSION file** (highest priority - explicit user configuration)
2. **Git tags** (automatic detection from repository history)
3. **Go module** (limited support for released modules)
4. **Go constants** (version constants in source code)

### Learning from Existing Patterns

When goneat encounters version information in non-primary sources, it can "learn" and suggest optimal configurations:

```bash
# Example: Repository with git tags but no VERSION file
$ goneat version
goneat v1.2.3
Source: git tag

# Suggestion for optimization
💡 Consider creating a VERSION file for faster lookups:
   echo '1.2.3' > VERSION
   goneat version init basic --force
```

### Configuration Learning

Goneat can analyze your repository patterns and suggest optimal version management strategies:

- **Monorepo**: Suggests VERSION file as primary source
- **Microservices**: Suggests per-service VERSION files
- **GitOps**: Suggests git tag-based versioning
- **Calendar releases**: Suggests calver template

### Handling Missing VERSION Files

When version information exists in secondary sources (like git tags) but not in a VERSION file, goneat provides intelligent guidance:

#### Scenario: Git Tags Exist, No VERSION File

```bash
$ goneat version
goneat v1.2.3
Source: git tag

# goneat suggests optimization
💡 Consider creating a VERSION file for faster lookups:
   echo '1.2.3' > VERSION
   goneat version init basic --force
```

#### Learning from Existing Patterns

Goneat can "learn" from your repository's version patterns:

1. **Pattern Recognition**: Analyzes existing git tags, commit messages, and release patterns
2. **Scheme Detection**: Identifies semver, calver, or custom versioning schemes
3. **Migration Suggestions**: Provides commands to migrate to optimal configurations
4. **Consistency Validation**: Ensures all version sources remain synchronized

#### Automatic Learning Examples

```bash
# Repository with git tags v1.0.0, v1.1.0, v2.0.0
$ goneat version
goneat v2.0.0
Source: git tag
Learned: semver pattern detected

# Repository with tags 2024.01.15, 2024.02.01, 2024.03.15
$ goneat version
goneat 2024.03.15
Source: git tag
Learned: calendar versioning pattern detected
Suggestion: Use 'goneat version init calver' for optimization
```

#### Specifying Custom Version Sources

For advanced scenarios, you can specify custom version sources:

```bash
# Version in custom file
echo '1.2.3' > custom-version.txt

# Version in Go constant
const AppVersion = "1.2.3"

# Version in environment
export APP_VERSION=1.2.3
```

Goneat's learning system can detect and incorporate these patterns into its version discovery logic.

## Best Practices

### 1. First-Time Setup

Always use dry-run mode for initial setup:

```bash
# Preview what will be created
goneat version init basic --dry-run

# Apply the setup
goneat version init basic
```

### 2. Source Priority

Configure your primary source with the highest priority:

```yaml
sources:
  - type: version_file
    path: VERSION
    priority: 1 # Primary source
  - type: git_tags
    pattern: "v*"
    priority: 2 # Secondary source
```

### 2. Use Assessment Mode

Always test version operations with `--no-op` first:

```bash
# Test bump operation
goneat version bump patch --no-op

# Test version setting
goneat version set 1.2.3 --no-op

# Check consistency
goneat version check-consistency --no-op

# Preview initialization
goneat version init basic --dry-run
```

### 3. Dry-Run for Safe Setup

The `init` command supports `--dry-run` for safe experimentation:

```bash
# Preview any template setup
goneat version init basic --dry-run
goneat version init calver --dry-run
goneat version init git-tags --dry-run

# Test with custom initial version
goneat version init basic --dry-run --initial-version 2.0.0
```

### 3. Regular Consistency Checks

Add consistency checks to your CI pipeline:

```bash
# In CI script
goneat version check-consistency

if [ $? -ne 0 ]; then
  echo "Version inconsistency detected!"
  exit 1
fi
```

### 4. Version Validation

Validate version formats in your release process:

```bash
# Validate before tagging
goneat version validate "$NEW_VERSION"

if [ $? -ne 0 ]; then
  echo "Invalid version format: $NEW_VERSION"
  exit 1
fi
```

## Integration with Other Commands

### Format Command

```bash
# Format code and bump version
goneat format --folders src/
goneat version bump patch
```

### Assessment Workflows

```bash
# Full assessment workflow
goneat format --no-op --folders .
goneat version --no-op
goneat version check-consistency --no-op
```

## Troubleshooting

### Common Issues

#### Version Not Found

```
Error: no version found in any configured source
```

**Solutions:**

- Check that your version sources exist and are readable
- Verify source paths in configuration
- Ensure at least one source contains a valid version

#### Inconsistent Versions

```
Warning: Version inconsistency detected
Source VERSION: 1.2.3
Source git_tags: v1.2.4
```

**Solutions:**

- Use `goneat version check-consistency` to identify issues
- Use `goneat version set <version>` to synchronize all sources
- Review your source priority configuration

#### Invalid Version Format

```
Error: invalid semver format: 1.2.3.4
```

**Solutions:**

- Check your version format against the expected pattern
- Use `goneat version validate <version>` to test formats
- Review your version scheme configuration

### Debug Mode

Enable debug logging for detailed version operations:

```bash
goneat version --log-level debug bump patch
```

## Future Extensions

### Planned Features

- **Changelog Generation**: Automatic changelog from git history
- **Release Automation**: Integration with GitHub releases
- **Version Aliases**: Support for version aliases and channels
- **Dependency Versioning**: Version management for dependencies
- **Multi-repo Coordination**: Version coordination across multiple repositories

### Custom Bump Logic

Advanced custom versioning with programmable bump rules:

```yaml
version:
  method: custom
  custom:
    pattern: "^(\\d+)\\.(\\d+)\\.(\\d+)$"
    bump_logic:
      patch: "increment_last"
      minor: "increment_middle"
      major: "increment_first"
```

This version management system provides the foundation for zero version drift across your entire DevOps toolchain, ensuring consistency and reliability in your release process.</content>
</xai:function_call name="todowrite">
<parameter name="todos">[{"content":"Add version validation functionality","status":"completed","priority":"medium","id":"implement_validation"},{"content":"Add version consistency checking across sources","status":"completed","priority":"medium","id":"implement_consistency_check"},{"content":"Extend version command with bump, set, validate subcommands","status":"in_progress","priority":"high","id":"extend_version_command"}]
