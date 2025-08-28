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

## Usage

### Display Current Version

```bash
# Show current version from highest priority source
goneat version

# Show detailed version information
goneat version --extended

# Output in JSON format
goneat version --json
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
# Bump patch version
goneat version bump patch

# Bump minor version
goneat version bump minor

# Bump major version
goneat version bump major

# Preview bump without making changes
goneat version bump patch --no-op
```

### Version Setting

```bash
# Set specific version
goneat version set 1.2.3

# Set version with pre-release
goneat version set 1.2.3-alpha

# Preview version change
goneat version set 1.2.3 --no-op
```

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
      - 'v*'

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

## Best Practices

### 1. Source Priority

Always configure your primary source with the highest priority:

```yaml
sources:
  - type: version_file
    path: VERSION
    priority: 1  # Primary source
  - type: git_tags
    pattern: "v*"
    priority: 2  # Secondary source
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