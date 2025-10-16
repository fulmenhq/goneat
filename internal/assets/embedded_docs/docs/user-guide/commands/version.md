# Version Command

The `version` command provides clear information about the **goneat binary** and optionally the **host project** it's running in.

## Quick Start

### Show Goneat Version (Default)

```bash
goneat version
```

**Output**:

```
goneat 0.2.6
Go: go1.25.0
Platform: linux/amd64
```

Shows the **goneat binary version** - exactly what users expect from a CLI tool.

### Extended Binary Information

```bash
goneat version --extended
```

**Output**:

```
goneat 0.2.6
Module:
Build time: unknown
Git commit: abc12345
Git branch: main
Go version: go1.25.0
Platform: linux/amd64
```

Includes additional build details about the goneat binary.

### JSON Output

```bash
goneat version --json
```

**Output**:

```json
{
  "binaryVersion": "0.2.6",
  "moduleVersion": "",
  "goVersion": "go1.25.0",
  "platform": "linux",
  "arch": "amd64"
}
```

Clean JSON focused on the binary version for programmatic use.

## Project Version Management

### Show Host Project Version

```bash
goneat version --project
```

**Output** (when run in a project with `VERSION=1.2.3`):

```
Binary: 0.2.6
Module:
Project: myproject 1.2.3
Project Source: VERSION file
Go Version: go1.25.0
OS/Arch: linux/amd64
```

Shows the **host project's** version information (legacy behavior, now opt-in).

### Project JSON Output

```bash
goneat version --project --json
```

**Output**:

```json
{
  "binaryVersion": "0.2.6",
  "project": {
    "name": "myproject",
    "version": "1.2.3",
    "source": "VERSION file"
  },
  "moduleVersion": "",
  "goVersion": "go1.25.0",
  "platform": "linux",
  "arch": "amd64"
}
```

Includes both binary and project version information.

## Why This Distinction Matters

### Real-World Scenario: Goneat in Another Repository

Imagine you've installed goneat globally and are working on your own project:

**Your Project Structure:**

```
myproject/
├── go.mod (module github.com/user/myproject)
├── VERSION (contains "0.1.0")
└── main.go
```

### Before v0.2.6 (The Problem)

When you ran `goneat version` in your project, it would show **your project's version** instead of goneat's:

```bash
cd myproject
goneat version
```

**Incorrect Output (v0.2.5 and earlier):**

```
Binary: 0.2.5
myproject (Project) 0.1.0  # ← Shows YOUR project's version!
Project Source: VERSION file
Go Version: go1.25.0
OS/Arch: linux/amd64
```

**Problem**: You expected to see "goneat 0.2.5" but saw your project's "0.1.0". This was confusing and made it hard to verify which version of goneat you had installed.

### After v0.2.6 (The Fix)

Now the default behavior shows **goneat's binary version** clearly:

```bash
cd myproject
goneat version
```

**Correct Output (v0.2.6+):**

```
goneat 0.2.6
Go: go1.25.0
Platform: linux/amd64
```

**What users see**: Clear confirmation that they have goneat 0.2.6 installed, regardless of what project they're in.

### Accessing Project Version Information

If you need to see your project's version information (the original functionality), use the `--project` flag:

```bash
goneat version --project
```

**Output:**

```
Binary: 0.2.6
Module:
Project: myproject 0.1.0
Project Source: VERSION file
Go Version: go1.25.0
OS/Arch: linux/amd64
```

This preserves the legacy project version management functionality for users who need it.

## Subcommands

### `version init` - Initialize Project Version Management

**Note**: This subcommand always operates on the host project, regardless of the main command flags.

```bash
goneat version init [template]
```

**Available templates**:

- `basic` - VERSION file with semantic versioning (default)
- `git-tags` - Git tag-based versioning
- `calver` - Calendar versioning (YYYY.MM.DD)
- `custom` - Custom versioning scheme

**Examples**:

```bash
# Basic setup with VERSION file
goneat version init

# Calendar versioning
goneat version init calver

# Dry run to preview
goneat version init --dry-run

# Custom initial version
goneat version init --initial-version 2.0.0
```

### `version bump` - Increment Project Version

**Note**: Always affects the host project.

```bash
goneat version bump [patch|minor|major]
```

**Examples**:

```bash
goneat version bump patch    # 0.1.0 → 0.1.1 (updates your project's VERSION file)
goneat version bump minor    # 0.1.0 → 0.2.0
goneat version bump major    # 0.1.0 → 1.0.0
```

### `version set` - Set Specific Project Version

```bash
goneat version set <version>
```

**Example**:

```bash
goneat version set 1.0.0     # Sets your project's version to 1.0.0
```

### `version validate` - Validate Version Format

```bash
goneat version validate <version>
```

**Example**:

```bash
goneat version validate 1.2.3      # Valid ✓
goneat version validate v1.2.3     # Valid ✓
goneat version validate invalid    # Invalid ✗
```

### `version check-consistency` - Verify Source Consistency

```bash
goneat version check-consistency
```

Checks that version information is consistent across all configured sources in the host project.

### `version propagate` - Synchronize Version to Package Managers

```bash
goneat version propagate [OPTIONS]
```

Propagates the VERSION file content to package manager files (package.json, pyproject.toml, go.mod) according to policy configuration. This ensures the VERSION file remains the single source of truth while automatically synchronizing version information across your project.

**Key Features:**
- **Multi-format support**: Updates package.json, pyproject.toml, and go.mod files
- **Workspace aware**: Handles monorepos with selective propagation
- **Policy driven**: Configurable via `.goneat/version-policy.yaml`
- **Safe operations**: Backup creation, dry-run mode, atomic updates

**Options:**
- `--dry-run`: Preview changes without making them
- `--force`: Overwrite files without confirmation
- `--target strings`: Specific files or package managers to target
- `--exclude strings`: Files to exclude from propagation
- `--backup`: Create backup files before changes
- `--validate-only`: Only validate current version consistency

**Examples:**

```bash
# Preview propagation changes
goneat version propagate --dry-run

# Propagate to all detected files
goneat version propagate

# Target specific package managers
goneat version propagate --target package.json --target pyproject.toml

# Validate without changes
goneat version propagate --validate-only

# Propagate with backups
goneat version propagate --backup
```

**Policy Configuration:**

Version propagation behavior is controlled by `.goneat/version-policy.yaml`. This file defines which package manager files to update, workspace handling, and safety guards.

#### Quick Start Configuration

```yaml
$schema: https://schemas.fulmenhq.dev/config/goneat/version-policy-v1.0.0.schema.json
version:
  scheme: semver          # semver | calver
  allow_extended: true    # enables prerelease/build metadata

propagation:
  defaults:
    include: ["package.json", "pyproject.toml"]
    exclude: ["**/node_modules/**", "docs/**"]
    backup:
      enabled: true
      retention: 5

  workspace:
    strategy: single-version  # single-version | opt-in | opt-out

guards:
  required_branches: ["main", "release/*"]
  disallow_dirty_worktree: true
```

#### Advanced Monorepo Configuration

```yaml
$schema: https://schemas.fulmenhq.dev/config/goneat/version-policy-v1.0.0.schema.json
version:
  scheme: semver
  allow_extended: true

propagation:
  defaults:
    include: ["package.json", "pyproject.toml"]
    exclude: ["**/node_modules/**", "docs/**"]
    backup:
      enabled: true
      retention: 5

  workspace:
    strategy: opt-out  # Allow independent versioning by default

  targets:
    # JavaScript/TypeScript packages
    package.json:
      include: ["./package.json", "apps/*/package.json", "packages/*/package.json"]
      exclude: ["packages/legacy-*"]  # Legacy packages don't get updates

    # Python services
    pyproject.toml:
      include: ["services/*/pyproject.toml"]
      mode: project  # Use [project] section (default)

    # Go modules (validation only)
    go.mod:
      validate_only: true

guards:
  required_branches: ["main", "develop", "release/*"]
  disallow_dirty_worktree: true
```

#### Generating Policy Files

Generate a complete policy file with all options and comments:

```bash
goneat version propagate --generate-policy
```

This creates `.goneat/version-policy.yaml` with comprehensive examples and documentation.

#### Configuration Reference

**Version Section:**
- `scheme`: `semver` (default) or `calver` - versioning scheme used
- `allow_extended`: `true` (default) - allow prerelease and build metadata
- `channel`: Optional release channel name

**Propagation Section:**
- `defaults.include`: Default package managers to include
- `defaults.exclude`: Glob patterns to exclude
- `defaults.backup.enabled`: Create backup files before changes
- `defaults.backup.retention`: Number of backup files to keep
- `workspace.strategy`: How to handle monorepos
- `targets`: Package-manager specific overrides

**Guards Section:**
- `required_branches`: Branch patterns where propagation is allowed
- `disallow_dirty_worktree`: Prevent propagation with uncommitted changes

#### Workspace Strategies

| Strategy | Description | Use Case |
|----------|-------------|----------|
| `single-version` | All packages use root VERSION | Simple monorepos, unified releases |
| `opt-in` | Only explicitly configured packages | Selective independent versioning |
| `opt-out` | All except excluded packages | Most packages version independently |

#### Package Manager Support

| Language | File | Update Mode | Notes |
|----------|------|-------------|-------|
| JavaScript/TypeScript | `package.json` | Full update | Supports npm/yarn workspaces |
| Python | `pyproject.toml` | Full update | `[project]` or `[tool.poetry]` sections |
| Go | `go.mod` | Validate only | Checks module name consistency |

#### Schema and Examples

- **Complete Schema**: [Version Policy Schema](../../../schemas/crucible-go/config/goneat/v1.0.0/version-policy.schema.yaml)
- **Schema Documentation**: Full field descriptions, validation rules, and examples
- **Generated Template**: Run `goneat version propagate --generate-policy` for a complete example

#### Safety Features

- **Branch Guards**: Prevent accidental propagation on feature branches
- **Worktree Validation**: Ensure clean git state before changes
- **Backup Creation**: Automatic `.bak` files with configurable retention
- **Dry-run Mode**: Preview all changes before execution
- **Atomic Operations**: All-or-nothing updates with rollback on failure

## Flags Reference

| Flag         | Shorthand | Description                                         | Default |
| ------------ | --------- | --------------------------------------------------- | ------- |
| `--project`  | `-p`      | Show host project version information (legacy mode) | `false` |
| `--extended` |           | Show detailed build and git information             | `false` |
| `--json`     |           | JSON output format                                  | `false` |
| `--no-op`    |           | Assessment mode (logging only)                      | `false` |

## Complete Example: Working in Your Project

Let's walk through a complete example of using goneat in your own project:

### 1. Verify Goneat Installation

```bash
# Check what version of goneat you have installed
goneat version
```

**Output:**

```
goneat 0.2.6
Go: go1.25.0
Platform: linux/amd64
```

✅ You now know you have goneat 0.2.6 installed.

### 2. Check Your Project's Version

```bash
# See your project's current version status
goneat version --project
```

**Output:**

```
Binary: 0.2.6
Module:
Project: myproject 0.1.0
Project Source: VERSION file
Go Version: go1.25.0
OS/Arch: linux/amd64
```

✅ Your project is at version 0.1.0.

### 3. Initialize Version Management (First Time)

```bash
# Preview what will be created
goneat version init --dry-run

# Apply the setup
goneat version init
```

This creates a `VERSION` file in your project with initial version 0.1.0.

### 4. Bump Your Project Version

```bash
# Preview the bump
goneat version bump patch --no-op

# Apply the patch bump
goneat version bump patch
```

**Before:** `VERSION` contains "0.1.0"  
**After:** `VERSION` contains "0.1.1"

### 5. Verify Everything

```bash
# Check goneat version (binary)
goneat version

# Check project version
goneat version --project

# Validate the new version
goneat version validate 0.1.1
```

### 6. JSON for Automation

```bash
# Get both versions in JSON for CI/CD
goneat version --project --json
```

**Output:**

```json
{
  "binaryVersion": "0.2.6",
  "project": {
    "name": "myproject",
    "version": "0.1.1",
    "source": "VERSION file"
  },
  "moduleVersion": "",
  "goVersion": "go1.25.0",
  "platform": "linux",
  "arch": "amd64"
}
```

## Exit Codes

- `0`: Success
- `1`: Invalid version format or missing version sources
- `2`: Git operations failed (non-fatal for some operations)

## Integration Examples

### CI/CD Pipeline

```yaml
# .github/workflows/release.yml
name: Release
on:
  push:
    tags: ["v*"]

jobs:
  validate-versions:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Verify goneat version
        run: |
          GONEAT_VERSION=$(goneat version --json | jq -r '.binaryVersion')
          echo "Goneat version: $GONEAT_VERSION"
          if [[ "$GONEAT_VERSION" != "0.2.6" ]]; then
            echo "Error: Expected goneat 0.2.6, got $GONEAT_VERSION"
            exit 1
          fi

      - name: Validate project version
        run: |
          PROJECT_VERSION=$(goneat version --project --json | jq -r '.project.version')
          echo "Project version: $PROJECT_VERSION"
          goneat version validate "$PROJECT_VERSION"
```

### Pre-commit Hook for Version Consistency

```yaml
# .goneat/hooks.yaml
pre-commit:
  commands:
    - command: version
      args: ["check-consistency"]
      fail_threshold: medium
      only_changed_files: true
```

### Shell Script Integration

```bash
#!/bin/bash

set -e

# Get goneat version for logging
GONEAT_VERSION=$(goneat version --json | jq -r '.binaryVersion')
echo "Using goneat $GONEAT_VERSION"

# Get current project version
PROJECT_VERSION=$(goneat version --project --json | jq -r '.project.version')
echo "Current project version: $PROJECT_VERSION"

# Validate versions
goneat version validate "$GONEAT_VERSION"
goneat version validate "$PROJECT_VERSION"

# For release: bump and validate
if [[ "$1" == "release" ]]; then
  echo "Bumping project version..."
  goneat version bump patch

  NEW_VERSION=$(goneat version --project --json | jq -r '.project.version')
  echo "New project version: $NEW_VERSION"

  goneat version validate "$NEW_VERSION"
fi
```

## Backward Compatibility

### v0.2.6 Changes Summary

| Behavior           | Before v0.2.6          | After v0.2.6                               | Command                    |
| ------------------ | ---------------------- | ------------------------------------------ | -------------------------- |
| **Default**        | Showed project version | Shows **binary version** (0.2.6)           | `goneat version`           |
| **Project Info**   | Default behavior       | Opt-in via flag                            | `goneat version --project` |
| **JSON Structure** | Mixed fields           | Clear `binaryVersion` + optional `project` | `goneat version --json`    |
| **Subcommands**    | Project-focused        | Unchanged (always project)                 | `goneat version bump` etc. |

### Migration Guide

**No breaking changes** - existing scripts using project version management continue to work:

```bash
# These commands were always project-focused and remain unchanged:
goneat version bump patch          # Still bumps project version
goneat version init                # Still initializes project
goneat version set 1.2.3           # Still sets project version

# New default behavior for simple version checking:
goneat version                     # Now shows goneat 0.2.6 (was project version)
goneat version --project           # Shows project version (old default behavior)
```

### For Script Authors

Update scripts that rely on the default output format:

```bash
# OLD (v0.2.5): Assumed project version in default output
OLD_VERSION=$(goneat version | grep -oP '\(\K[^)]+')

# NEW (v0.2.6+): Use --project flag for project version
PROJECT_VERSION=$(goneat version --project --json | jq -r '.project.version')

# NEW: Binary version is now default and cleaner
GONEAT_VERSION=$(goneat version --json | jq -r '.binaryVersion')
```

## Notes

- **Binary Version**: Embedded at build time via `ldflags`; shows the actual installed goneat version
- **Module Version**: Go module version (visible for `go install` builds from source)
- **Project Version**: Host project's version from `VERSION` file, git tags, etc.
- **Security**: All file operations include path validation to prevent traversal attacks
- **Performance**: Binary version lookup is O(1); project version requires file/git operations
- **Cross-platform**: Works on Linux, macOS, Windows with consistent output

For more details, see the [CHANGELOG](https://github.com/fulmenhq/goneat/blob/main/CHANGELOG.md).
