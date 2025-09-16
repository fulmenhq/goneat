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
