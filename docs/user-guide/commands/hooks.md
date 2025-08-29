---
title: "Hooks Command Reference"
description: "Complete reference for the goneat hooks command - manage git hooks with intelligent validation"
author: "@forge-neat"
date: "2025-08-28"
last_updated: "2025-08-28"
status: "approved"
tags: ["cli", "hooks", "git", "validation", "commands"]
category: "user-guide"
---

# Hooks Command Reference

The `goneat hooks` command provides comprehensive git hook management with native goneat integration, enabling intelligent code quality validation that goes beyond traditional shell scripts.

## Overview

Goneat hooks transform git's basic hook system into an intelligent validation platform that:

- **Orchestrates multiple tools** through goneat's assess engine
- **Provides unified reporting** with actionable feedback
- **Enables parallel execution** for faster validation
- **Supports enterprise features** like audit trails and compliance reporting
- **Maintains simplicity** with easy setup and configuration

## Command Structure

```bash
goneat hooks [command] [flags]

Available commands:
  init      Initialize hooks system
  generate  Generate hook files from manifest
  install   Install hooks to .git/hooks
  validate  Validate hook configuration
  remove    Remove installed hooks
  upgrade   Upgrade hook configuration to latest version
  inspect   Inspect current hook configuration and status
```

## Available Commands

### `goneat hooks init`

Initialize the hooks system by creating the basic configuration structure.

```bash
goneat hooks init
```

**What it does:**
- Creates `.goneat/hooks.yaml` with default configuration
- Sets up the `.goneat/` directory structure
- Provides sensible defaults for common use cases

**Example output:**
```bash
🐾 Initializing goneat hooks system...

✅ Created .goneat/hooks.yaml with default configuration
✅ Created .goneat/ directory structure
📝 Ready for hook generation
```

### `goneat hooks generate`

Generate executable hook files based on the configuration manifest.

```bash
goneat hooks generate
```

**What it does:**
- Reads `.goneat/hooks.yaml` configuration
- Generates simple bash scripts for each hook type
- Creates fallback logic for when goneat isn't available
- Places generated files in `.goneat/hooks/` directory

**Example output:**
```bash
🔨 Generating hook files from manifest...

✅ Generated .goneat/hooks/pre-commit
✅ Generated .goneat/hooks/pre-push
📦 Ready for installation
```

### `goneat hooks install`

Install generated hook files to the active git hooks directory.

```bash
goneat hooks install
```

**What it does:**
- Copies generated hooks from `.goneat/hooks/` to `.git/hooks/`
- Sets executable permissions on hook files
- Provides backup of existing hooks if they exist
- Ensures git can execute the hooks

**Example output:**
```bash
📦 Installing hooks to .git/hooks...

✅ Installed pre-commit hook
✅ Installed pre-push hook
✅ Set executable permissions
🎯 Hooks are now active!
```

### `goneat hooks validate`

Validate the hooks configuration and installation.

```bash
goneat hooks validate
```

**What it does:**
- Checks `.goneat/hooks.yaml` for syntax errors
- Validates generated hook files exist and are executable
- Tests hook execution with dry-run mode
- Provides remediation steps for any issues

**Example output:**
```bash
🔍 Validating hook configuration...

✅ Hook manifest syntax is valid
✅ Generated hook files are present
✅ Hook permissions are correct
✅ Test execution successful
🎉 Hooks configuration is valid!
```

### `goneat hooks remove`

Remove installed hooks and restore the previous state.

```bash
goneat hooks remove
```

**What it does:**
- Removes goneat hooks from `.git/hooks/` directory
- Restores any previously backed up original hooks
- Optionally cleans up generated hook files
- Provides confirmation of successful removal

**Example output:**
```bash
🗑️  Removing goneat hooks...

✅ Goneat hooks removed
✅ Original hooks restored (if any existed)
💡 Your git hooks have been restored to their previous state
```

### `goneat hooks upgrade`

Upgrade hook configuration to the latest schema version.

```bash
goneat hooks upgrade
```

**What it does:**
- Detects current schema version in `.goneat/hooks.yaml`
- Downloads the latest schema version
- Migrates configuration to new format automatically
- Updates manifest with new schema version
- Provides migration summary and any manual steps needed

**Example output:**
```bash
⬆️  Upgrading hook configuration...

📋 Current version: 1.0.0
⬆️  Latest version: 1.1.0
🔄 Migrating configuration...
✅ Schema upgrade completed
📝 Review the migration summary above for any manual steps
```

### `goneat hooks inspect`

Inspect current hook configuration and system status.

```bash
goneat hooks inspect [--format json]
```

**What it does:**
- Displays detailed information about hook configuration
- Shows installation status and system state
- Lists all configured hooks and their settings
- Provides health check of the hook system
- Supports both human-readable and JSON output formats

**Example output (default format):**
```bash
🔍 Inspecting hook configuration and status...

📊 Current Hook Status:
├── Configuration: .goneat/hooks.yaml ✅ (v1.0.0)
├── Generated Hooks: .goneat/hooks/ ✅
├── Installed Hooks: .git/hooks/ ✅
├── System Health: All systems operational 🎯
└── Active Hooks:
    ├── pre-commit: format,lint (priority: 1,2)
    └── pre-push: security (priority: 1)
```

**Example output (JSON format):**
```bash
goneat hooks inspect --format json
```

```json
{
  "configuration": {
    "path": ".goneat/hooks.yaml",
    "version": "1.0.0",
    "last_modified": "2025-08-28T12:34:56Z"
  },
  "generated_hooks": {
    "path": ".goneat/hooks/",
    "exists": true,
    "hooks": ["pre-commit", "pre-push"]
  },
  "installed_hooks": {
    "path": ".git/hooks/",
    "exists": true,
    "hooks": ["pre-commit", "pre-push"],
    "permissions": "executable"
  },
  "active_hooks": {
    "pre-commit": {
      "categories": ["format", "lint"],
      "priorities": [1, 2],
      "timeout": "2m"
    },
    "pre-push": {
      "categories": ["security"],
      "priorities": [1],
      "timeout": "3m"
    }
  },
  "system_health": "operational"
}
```

## Configuration

### Hook Manifest (`.goneat/hooks.yaml`)

The hook manifest defines what validation runs for each hook type:

```yaml
version: "1.0.0"
hooks:
  pre-commit:
    - command: "assess"
      args: ["--categories", "format,lint", "--fail-on", "error"]
      stage_fixed: true
      priority: 10
      timeout: "2m"
    - command: "format"
      args: ["--check", "--quiet"]
      fallback: "go fmt ./..."
      when:
        - files_match: "*.go"
      timeout: "30s"
  pre-push:
    - command: "assess"
      args: ["--full", "--format", "json", "--output", ".goneat/reports/"]
      priority: 10
      timeout: "3m"

optimization:
  only_changed_files: true
  cache_results: true
  parallel: "auto"
```

### Configuration Options

#### Hook Commands

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `command` | string | Goneat subcommand to execute | `"assess"`, `"format"` |
| `args` | array | Arguments to pass to command | `["--check", "--quiet"]` |
| `fallback` | string | Shell command if goneat unavailable | `"go fmt ./..."` |
| `when` | array | Conditions for execution | `[{"files_match": "*.go"}]` |
| `priority` | integer | Execution priority (higher = first) | `10` |
| `timeout` | string | Maximum execution time | `"2m"` |
| `stage_fixed` | boolean | Stage files fixed by command | `true` |
| `skip` | array | Skip in these git scenarios | `["merge", "rebase"]` |

#### Optimization Settings

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `only_changed_files` | boolean | Only validate changed files | `true` |
| `cache_results` | boolean | Cache validation results | `true` |
| `parallel` | string | Parallel execution mode | `"auto"` |

## Usage Examples

### Basic Setup

```bash
# Initialize hooks system
goneat hooks init

# Generate hook files
goneat hooks generate

# Install to git
goneat hooks install

# Validate setup
goneat hooks validate
```

### Custom Configuration

```bash
# Edit configuration
vim .goneat/hooks.yaml

# Regenerate with new config
goneat hooks generate

# Reinstall updated hooks
goneat hooks install
```

### Testing Hooks

```bash
# Test what pre-commit hook would do
goneat assess --hook pre-commit

# Test with verbose output
goneat assess --hook pre-commit --verbose

# Test specific categories
goneat assess --categories format,lint
```

## Generated Hook Files

The `generate` command creates simple bash scripts that delegate to goneat:

### `.git/hooks/pre-commit` (generated)
```bash
#!/bin/bash
# Generated by goneat hooks generate on 2025-08-28

set -e

echo "🔍 Running goneat pre-commit validation..."

# Check if goneat is available
if ! command -v goneat &> /dev/null; then
    echo "⚠️  goneat not found, falling back to basic validation"
    go fmt ./... || { echo "❌ go fmt failed"; exit 1; }
    go vet ./... || { echo "❌ go vet failed"; exit 1; }
    echo "✅ Basic validation passed"
    exit 0
fi

# Use goneat's orchestrated assessment
goneat assess --hook pre-commit --manifest .goneat/hooks.yaml

echo "✅ Pre-commit validation passed!"
```

## Integration with Git

### Automatic Execution

Once installed, hooks run automatically with git operations:

```bash
# Pre-commit hook runs automatically
git commit -m "Add feature"
# → Executes .git/hooks/pre-commit
# → Calls goneat assess --hook pre-commit
# → Blocks commit if validation fails

# Pre-push hook runs automatically
git push origin main
# → Executes .git/hooks/pre-push
# → Calls goneat assess --hook pre-push
# → Blocks push if validation fails
```

### Manual Testing

Test hooks without triggering git operations:

```bash
# Test pre-commit validation
goneat assess --hook pre-commit

# Test with different configurations
goneat assess --hook pre-commit --fail-on critical

# Debug hook execution
goneat assess --hook pre-commit --verbose
```

## Troubleshooting

### Common Issues

**Hooks not running:**
```bash
# Check if hooks are executable
ls -la .git/hooks/pre-commit

# Verify goneat is in PATH
which goneat

# Test manual execution
goneat assess --hook pre-commit
```

**Configuration errors:**
```bash
# Validate configuration
goneat hooks validate

# Check manifest syntax
cat .goneat/hooks.yaml
```

**Performance issues:**
```bash
# Test with timing
time goneat assess --hook pre-commit

# Check optimization settings
vim .goneat/hooks.yaml
```

### Getting Help

```bash
# Show hooks command help
goneat hooks --help

# Show specific subcommand help
goneat hooks init --help

# Check goneat version
goneat version
```

## Advanced Usage

### Custom Hook Types

Add custom hook configurations for specific workflows:

```yaml
hooks:
  pre-commit:
    # Standard validation
  post-commit:
    - command: "assess"
      args: ["--categories", "docs", "--format", "json"]
  pre-push:
    # Security and integration tests
```

### Environment-Specific Configuration

Use different configurations for different environments:

```yaml
# Development - fast feedback
hooks:
  pre-commit:
    - command: "assess"
      args: ["--categories", "format,lint"]
      timeout: "30s"

# CI/CD - comprehensive validation
# Use different manifest or command-line overrides
```

### Integration with CI/CD

Hooks work alongside CI/CD pipelines:

```yaml
# Local development - fast feedback
pre-commit:
  - command: "assess"
    args: ["--categories", "format,lint"]

# CI pipeline - comprehensive validation
# GitHub Actions, etc. can run:
# goneat assess --full --format json
```

## Future Enhancements

The hooks system is designed for extensibility:

- **Additional hook types** (post-merge, pre-rebase, etc.)
- **Plugin system** for custom validation tools
- **Remote validation** against repository state
- **Team collaboration** features for shared configurations
- **Performance monitoring** and optimization insights

## Related Commands

- [`goneat assess`](assess.md) - Core assessment engine
- [`goneat version`](version.md) - Version information
- [`goneat format`](format.md) - Code formatting (planned)
- [`goneat lint`](lint.md) - Code linting (planned)

## See Also

- [Git Hooks Operation Workflow](../workflows/git-hooks-operation.md) - Complete setup guide with diagrams
- [Assessment Architecture](../../architecture/assess-workflow.md) - Technical details
- [Hooks Architecture](../../architecture/hooks-command-architecture.md) - Design decisions</content>
</xai:function_call/>
</xai:function_call name="write">
<parameter name="filePath">/Users/davethompson/dev/fulmenhq/goneat/goneat/docs/user-guide/commands/assess.md