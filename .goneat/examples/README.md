# Tools Configuration Examples

This directory contains example tool configurations demonstrating various installation methods.

## Package Manager Examples (v1.1.0+)

### Homebrew Formula (macOS/Linux)
**File**: `tools-brew-formula.yaml`

Demonstrates installing CLI tools via Homebrew formulas:
- Standard formula (jq, ripgrep)
- Tap-based formula (goneat from fulmenhq/tap)
- Custom flags

**Usage**:
```bash
goneat doctor tools --config .goneat/examples/tools-brew-formula.yaml --scope example-brew
```

### Homebrew Cask (macOS)
**File**: `tools-brew-cask.yaml`

Demonstrates installing GUI applications via Homebrew casks:
- Docker Desktop
- Visual Studio Code

**Usage**:
```bash
goneat doctor tools --config .goneat/examples/tools-brew-cask.yaml --scope example-cask
```

### Scoop (Windows)
**File**: `tools-scoop.yaml`

Demonstrates installing CLI tools via Scoop:
- Tools from main bucket (ripgrep, jq, git)
- Custom flags

**Usage**:
```bash
goneat doctor tools --config .goneat/examples/tools-scoop.yaml --scope example-scoop
```

## Testing Examples

Validate manifests:
```bash
goneat doctor tools --config .goneat/examples/tools-brew-formula.yaml --validate-config
```

Dry run to see what would be installed:
```bash
goneat doctor tools --config .goneat/examples/tools-brew-formula.yaml --scope example-brew --dry-run
```

## Schema Version

These examples use tools schema **v1.1.0** which adds native package manager support.

For more information, see:
- Schema documentation: `schemas/tools/v1.1.0/tools-config.yaml`
- User guide: `goneat docs show user-guide/commands/doctor`
