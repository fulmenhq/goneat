---
title: "Hooks Command Reference"
description: "Complete reference for the goneat hooks command - manage git hooks with intelligent validation"
author: "goneat contributors"
date: "2025-08-28"
last_updated: "2026-01-15"
status: "approved"
tags: ["cli", "hooks", "git", "validation", "commands"]
category: "user-guide"
---

# Hooks Command Reference

The `goneat hooks` command provides comprehensive git hook management with native goneat integration, enabling intelligent code quality validation that goes beyond traditional shell scripts.

## Overview

Goneat hooks transform git's basic hook system into an intelligent validation platform that:

### Related Configuration

- Hook policy lives in `.goneat/hooks.yaml`.
- Lint rule tuning for what `assess` reports (and fixes) lives in `.goneat/assess.yaml`.

To scaffold a starter `.goneat/assess.yaml`, run:

```bash
goneat doctor assess init
```

- **Orchestrates multiple tools** through goneat's assess engine
- **Provides unified reporting** with actionable feedback
- **Enables parallel execution** for faster validation
- **Supports enterprise features** like audit trails and compliance reporting
- **Maintains simplicity** with easy setup and configuration

## Command Structure

```bash
goneat hooks [command] [flags]

Available commands:
  init       Initialize hooks system
  generate   Generate hook files from manifest
  install    Install hooks to .git/hooks
  validate   Validate hook configuration
  remove     Remove installed hooks
  upgrade    Upgrade hook configuration to latest version
  inspect    Inspect current hook configuration and status
  configure  Configure pre-commit/pre-push behavior without editing YAML
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

- Reads `.goneat/hooks.yaml`
- Generates platform-specific hook scripts from embedded templates
- Includes fallback logic when goneat isn't available
- Optionally injects guardian approval checks when enabled
- Writes generated files to `.goneat/hooks/`

**Guardian integration:**

- Use `--with-guardian` to force guardian enforcement into the generated hooks.
- Use `--reset-guardian` to generate a guardian-protected pre-reset hook for protecting `git reset` operations on protected branches.
- When the guardian config sets `guardian.integrations.hooks.auto_install: true`, the flag defaults on automatically.
- Guardian metadata (scope, method, risk, expiry) is embedded so terminal prompts show approval context when an operation is blocked.
- When guardian blocks an operation, re-run the command using `goneat guardian approve <scope> <operation> -- <command>` so the action executes atomically after approval (for example, `goneat guardian approve git push -- git push origin main`).

**Example with guardian:**

```bash
goneat hooks generate --with-guardian
```

**Example with reset protection:**

```bash
goneat hooks generate --reset-guardian
```

When auto-install is enabled in the config, the same guardian block is emitted without passing the explicit flag, so hooks stay in sync with security policy updates.

**Example output:**

```bash
🔨 Generating hook files from manifest...
🛡️  Guardian integration enabled in generated hooks
✅ Hook files generated successfully!
📁 Created: .goneat/hooks/pre-commit
📁 Created: .goneat/hooks/pre-push
📁 Created: .goneat/hooks/pre-reset
📌 Next: Run 'goneat hooks install' to install hooks to .git/hooks
```

### `goneat hooks install`

Install generated hook files to the active git hooks directory.

```bash
goneat hooks install [flags]
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--unset-hookspath` | Clear `core.hooksPath` git config before installing (fixes husky/lefthook migration) |
| `--respect-hookspath` | Install hooks to the path specified in `core.hooksPath` instead of `.git/hooks` |
| `--force` | Alias for `--unset-hookspath` |

**What it does:**

- Detects `core.hooksPath` git config (common remnant from husky, lefthook, etc.) and warns if set
- Copies generated hooks from `.goneat/hooks/` to `.git/hooks/` (or custom path if `--respect-hookspath`)
- Sets executable permissions on hook files
- Provides backup of existing hooks if they exist
- Detects guardian-enabled scripts and ensures guardian configuration is bootstrapped
- Confirms git can execute the hooks

**Example output:**

```bash
📦 Installing hooks to .git/hooks...
✅ Installed pre-commit hook
✅ Installed pre-push hook
✅ Installed pre-reset hook
🎯 Successfully installed 3 hook(s)!
🛡️  Guardian integration detected. Config available at /Users/alex/.goneat/guardian/config.yaml
🔐 Protected operations will require guardian approval before proceeding
```

**Example with core.hooksPath detected:**

```bash
$ goneat hooks install

⚠️  Warning: core.hooksPath is set to '.husky/_'
   Git will ignore hooks in .git/hooks/

   This is typically left over from husky, lefthook, or similar tools.

   Options:
   1. Run: goneat hooks install --unset-hookspath
      (Removes core.hooksPath so git uses .git/hooks/)

   2. Run: goneat hooks install --respect-hookspath
      (Installs hooks to .husky/_/ instead)

   3. Manually fix:
      git config --local --unset core.hooksPath

❌ Hooks installation aborted due to core.hooksPath override
```

**Example with --unset-hookspath:**

```bash
$ goneat hooks install --unset-hookspath
📦 Installing hooks to .git/hooks...
ℹ️  Unsetting core.hooksPath (was: .husky/_)
✅ core.hooksPath cleared - git will use .git/hooks/
✅ Installed pre-commit hook
✅ Installed pre-push hook
🎯 Successfully installed 2 hook(s)!
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
✅ Pre-commit hook generated
✅ Pre-push hook generated
✅ Pre-reset hook generated
✅ Pre-commit hook installed and executable
✅ Pre-push hook installed and executable
✅ Pre-reset hook installed and executable
✅ Hook configuration validation complete
🎉 Ready to commit with intelligent validation!
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

Note: In v0.1.3 this is a placeholder. It validates your current configuration and prints a "coming soon" message. No migration is performed yet.

```bash
goneat hooks upgrade
```

What it does today:

- Reads `.goneat/hooks.yaml`
- Validates the configuration is present/readable
- Prints "Schema upgrade functionality coming soon!"
- Exits successfully without modifying your files

**Example output:**

```bash
⬆️  Upgrading hook configuration...
🔄 Schema upgrade functionality coming soon!
📋 This command will automatically migrate your hooks configuration
   to the latest schema version when implemented
✅ Current configuration validated
```

### `goneat hooks configure`

Configure common hook behaviors (scope, content source, apply mode) via CLI—no manual YAML edits required. This command updates `.goneat/hooks.yaml`, regenerates hook scripts, and can optionally install them.

```bash
# Show current pre-commit effective settings
goneat hooks configure --show

# Reset to defaults (recommended for most teams)
goneat hooks configure --reset

# Recommended staged-only, check-only pre-commit
goneat hooks configure \
  --pre-commit-only-changed-files=true \
  --pre-commit-content-source=index \
  --pre-commit-apply-mode=check \
  --install

# Opt-in: allow auto-fixes during pre-commit (re-stages fixed files)
goneat hooks configure --pre-commit-apply-mode=fix --install
```

Flags:

- `--show` Print the current effective settings (only_changed_files, content_source, apply_mode)
- `--reset` Restore recommended defaults (only_changed_files=true, content_source=index)
- `--pre-commit-only-changed-files` true|false to scope to changed files
- `--pre-commit-content-source` index|working
  - index = staged content only (preferred)
  - working = current working tree (includes unstaged edits)
- `--pre-commit-apply-mode` check|fix
  - check = read-only validation (recommended)
  - fix = apply changes and re-stage (StageFixed on relevant entries)
- `--optimization-parallel` auto|max|sequential (sets optimization.parallel)
- `--install` Install after regeneration

Notes:

- The generated pre-commit/pre-push scripts will pass `--staged-only` automatically when:
  - optimization.only_changed_files=true OR
  - optimization.content_source=index
- See “File Filtering with .goneatignore” for project-level filtering

### `goneat hooks policy`

Manage hook policy without manual YAML edits.

```bash
# Show effective policy for a hook
goneat hooks policy show --hook pre-commit --format json

# Set fail-on and categories for pre-push, and enable max parallelism
goneat hooks policy set --hook pre-push \
  --fail-on high \
  --categories format,lint,security \
  --parallel max \
  --dry-run   # preview changes

# Apply the change
goneat hooks policy set --hook pre-push --fail-on high --categories format,lint,security --parallel max --yes

# Reset to defaults for pre-commit (dry-run first)
goneat hooks policy reset --hook pre-commit --dry-run

# Validate hooks.yaml against schema
goneat hooks policy validate
```

Flags (set):

- `--hook` pre-commit|pre-push
- `--fail-on` critical|high|medium|low|info|error
- `--categories` Comma list, e.g., `format,lint[,security]`
- `--timeout` e.g., `90s|2m|3m`
- `--only-changed-files` true|false
- `--parallel` auto|max|sequential
- `--dry-run` preview YAML without writing
- `--yes` apply without prompt
- `--install` install hooks after regeneration

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
├── Configuration: ✅ Found
├── Generated Hooks: ✅ Found
│   ├── Pre-commit: ✅ Present
│   ├── Pre-push: ✅ Present
│   └── Pre-reset: ✅ Present
├── Installed Hooks: ✅ Found
│   ├── Pre-commit: ✅ Installed & executable
│   ├── Pre-push: ✅ Installed & executable
│   └── Pre-reset: ✅ Installed & executable
└── System Health: ✅ Good (10/10)
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
    "hooks": ["pre-commit", "pre-push", "pre-reset"]
  },
  "installed_hooks": {
    "path": ".git/hooks/",
    "exists": true,
    "hooks": ["pre-commit", "pre-push", "pre-reset"],
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
    },
    "pre-reset": {
      "guardian_protected": true,
      "scope": "git",
      "operation": "reset"
    }
  },
  "system_health": "operational"
}
```

## Configuration

### Hook Output Modes

Hooks are optimized for readable, actionable summaries while preserving structured data for automation:

- Concise (default in hook mode): Colorized, single-screen summary per category with totals and pass/fail footer
- JSON: Machine-readable report suitable for piping to pretty/HTML renderers or storing as artifacts
- Markdown/HTML: Use in CI or open locally for rich views

You can force JSON and pipe to a renderer:

```bash
goneat assess --hook pre-commit --hook-manifest .goneat/hooks.yaml --format json | goneat pretty --from json --to console
```

Disable color in terminals that don’t support it:

```bash
NO_COLOR=1 goneat assess --hook pre-commit
```

Environment override:

```bash
# Force concise or markdown output in hook mode without changing flags
GONEAT_HOOK_OUTPUT=concise goneat assess --hook pre-commit
GONEAT_HOOK_OUTPUT=markdown goneat assess --hook pre-commit --verbose
```

### Hook Manifest (`.goneat/hooks.yaml`)

The hook manifest defines what validation runs for each hook type:

```yaml
version: "1.0.0"
hooks:
  pre-commit:
    - command: "assess"
      args: ["--categories", "format,lint", "--fail-on", "medium"]
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
      args: ["--categories", "format,lint,security", "--fail-on", "high"]
      priority: 10
      timeout: "3m"

optimization:
  only_changed_files: true
  cache_results: true
  parallel: "auto"
```

### Incremental Lint Checking (v0.4.1+)

By default, hook mode reports **all lint issues**. To report only issues introduced since a baseline reference, add `--new-issues-only` to your hooks.yaml args:

```yaml
hooks:
  pre-commit:
    - command: "assess"
      # Only report NEW lint issues since previous commit
      args:
        [
          "--categories",
          "format,lint",
          "--fail-on",
          "high",
          "--new-issues-only",
        ]
  pre-push:
    - command: "assess"
      # Only report NEW lint issues since main branch
      args:
        [
          "--categories",
          "lint,security",
          "--fail-on",
          "high",
          "--new-issues-only",
          "--new-issues-base",
          "main",
        ]
```

**Note:** Prior to v0.4.1, hook mode implicitly applied incremental checking. This has been changed to explicit opt-in for consistency and transparency.

See [Incremental Lint Checking](../../appnotes/assess/incremental-lint-checking.md) for detailed guidance on when and how to use this feature.

### Configuration Options

#### Hook Commands

| Field         | Type    | Description                         | Example                     |
| ------------- | ------- | ----------------------------------- | --------------------------- |
| `command`     | string  | Goneat subcommand to execute        | `"assess"`, `"format"`      |
| `args`        | array   | Arguments to pass to command        | `["--check", "--quiet"]`    |
| `fallback`    | string  | Shell command if goneat unavailable | `"go fmt ./..."`            |
| `when`        | array   | Conditions for execution            | `[{"files_match": "*.go"}]` |
| `priority`    | integer | Execution priority (higher = first) | `10`                        |
| `timeout`     | string  | Maximum execution time              | `"2m"`                      |
| `stage_fixed` | boolean | Stage files fixed by command        | `true`                      |
| `skip`        | array   | Skip in these git scenarios         | `["merge", "rebase"]`       |

#### Optimization Settings

| Field                | Type    | Description                 | Default  |
| -------------------- | ------- | --------------------------- | -------- |
| `only_changed_files` | boolean | Only validate changed files | `true`   |
| `cache_results`      | boolean | Cache validation results    | `true`   |
| `parallel`           | string  | Parallel execution mode     | `"auto"` |

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

### Migration from Other Hook Managers

When migrating from husky, lefthook, or similar tools, the `core.hooksPath` git config often remains set after uninstallation. This causes git to ignore hooks in `.git/hooks/`, making goneat hooks appear to not work.

**Husky migration:**

```bash
# 1. Remove husky from package.json
npm uninstall husky

# 2. Remove the .husky directory (optional)
rm -rf .husky

# 3. Install goneat hooks (auto-detects and fixes core.hooksPath)
goneat hooks init
goneat hooks generate
goneat hooks install --unset-hookspath
```

**Lefthook migration:**

```bash
# 1. Remove lefthook
# (varies by installation method)

# 2. Install goneat hooks
goneat hooks init
goneat hooks generate
goneat hooks install --unset-hookspath
```

**Manual fix (if needed):**

```bash
# Check if core.hooksPath is set
git config --local core.hooksPath

# If set, clear it
git config --local --unset core.hooksPath

# Then install goneat hooks
goneat hooks install
```

**Troubleshooting: hooks not running after migration**

```bash
# Use inspect to diagnose
goneat hooks inspect

# If core.hooksPath warning appears:
goneat hooks install --unset-hookspath
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

### Command Execution Safety (v0.3.15+)

- Hooks now execute manifest commands in order (including `assess`, `dependencies`, and external commands).
- Manifest changes take effect on the next hook run—no regenerate/install needed unless templates/guardian/optimization settings change; edit `.goneat/hooks.yaml` and rerun git operation.
- Avoid invoking `make` targets that mutate the working tree (e.g., `format-all`, `verify-embeds`, custom builds) to prevent self-triggered loops in git hooks.
- Prefer check-only invocations such as `assess --categories format,lint,security --fail-on critical --package-mode` for pre-commit and `--fail-on high` for pre-push. These run read-only assessments and keep the tree stable.
- If you must run formatters, use staged-only/check flags (`format --staged-only --check --quiet`) instead of repo-wide mutate operations.

### Testing Hooks

```bash
# Test what pre-commit hook would do (explicit manifest)
goneat assess --hook pre-commit --hook-manifest .goneat/hooks.yaml

# Test staged-only behavior from CLI
goneat assess --hook pre-commit --staged-only

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

# Prefer local build if available
GONEAT_BIN="goneat"
if ! command -v "$GONEAT_BIN" &> /dev/null; then
    if [ -x "./dist/goneat" ]; then GONEAT_BIN="./dist/goneat"; fi
fi

if ! command -v "$GONEAT_BIN" &> /dev/null && [ ! -x "$GONEAT_BIN" ]; then
    echo "⚠️  goneat not found, falling back to basic validation"
    go fmt ./... || { echo "❌ go fmt failed"; exit 1; }
    go vet ./... || { echo "❌ go vet failed"; exit 1; }
    echo "✅ Basic validation passed"
    exit 0
fi

# Use goneat's orchestrated assessment
"$GONEAT_BIN" assess --hook pre-commit --hook-manifest .goneat/hooks.yaml

echo "✅ Pre-commit validation passed!"
```

### `.git/hooks/pre-reset` (generated with `--reset-guardian`)

```bash
#!/bin/bash
# Generated by goneat hooks generate on 2025-08-28

set -e

echo "🔍 Running goneat pre-reset guardian check..."

# Prefer local build if available
GONEAT_BIN="goneat"
if ! command -v "$GONEAT_BIN" &> /dev/null; then
    if [ -x "./dist/goneat" ]; then GONEAT_BIN="./dist/goneat"; fi
fi

if ! command -v "$GONEAT_BIN" &> /dev/null && [ ! -x "$GONEAT_BIN" ]; then
    echo "⚠️  goneat not found, allowing reset without guardian check"
    exit 0
fi

# Guardian enforcement for protected git reset operations
CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "")
GUARDIAN_SCOPE="git"
GUARDIAN_OPERATION="reset"

GUARDIAN_ARGS=("$GONEAT_BIN" guardian check "$GUARDIAN_SCOPE" "$GUARDIAN_OPERATION")
if [ -n "$CURRENT_BRANCH" ]; then
  GUARDIAN_ARGS+=("--branch" "$CURRENT_BRANCH")
fi

if ! "${GUARDIAN_ARGS[@]}"; then
  echo ""
  echo "❌ Git reset blocked by guardian"
  echo "🔐 Approval required for: ${GUARDIAN_SCOPE} ${GUARDIAN_OPERATION}"
  if [ -n "$CURRENT_BRANCH" ]; then
    echo "   • Branch: $CURRENT_BRANCH"
  fi
  echo "   • Method: browser"
  echo "   • Approval expires in: 5m"
  echo ""
  echo "Wrap your git reset with guardian approval to continue:"
  echo "  $GONEAT_BIN guardian approve $GUARDIAN_SCOPE $GUARDIAN_OPERATION -- git reset"
  echo "  # add your usual reset arguments after git reset"
  exit 1
fi

echo "✅ Guardian approval satisfied for git reset"

echo "✅ Pre-reset check passed!"
```

## File Filtering with .goneatignore

Goneat hooks respect file filtering patterns to control which files are assessed:

### .goneatignore File

Create a `.goneatignore` file in your repository root:

```bash
# Goneat ignore patterns (follows gitignore syntax)
*.tmp
*.temp
/dist/
*.pb.go
*_mock.go

# Override gitignore exclusions
!important-ignored-file.go
```

### Ignore Behavior

1. **Independent System**: Uses its own ignore patterns (separate from git)
2. **Pattern Support**: Glob patterns (`*.tmp`), directory patterns (`dist/`), exact matches
3. **Override**: `!pattern` syntax allows including files that would otherwise be ignored
4. **Hierarchy**: Repository → User ignore files (processed in order)

### File Locations (Priority Order)

1. `.goneatignore` (repository root - highest priority)
2. `~/.goneatignore` (user global - lower priority)

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

# Pre-reset hook runs automatically (when --reset-guardian used)
git reset --hard HEAD~1
# → Executes .git/hooks/pre-reset
# → Calls goneat guardian check git reset
# → Blocks reset if guardian approval required
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

# Test guardian reset protection
goneat guardian check git reset --branch main
```

## Troubleshooting

### Common Issues

**Hooks not running:**

```bash
# First, check for core.hooksPath override (common after husky/lefthook migration)
git config --local core.hooksPath
# If this returns a path, git is ignoring .git/hooks/
# Fix: goneat hooks install --unset-hookspath

# Use inspect to diagnose hook system health
goneat hooks inspect

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
  pre-reset:
    # Note: pre-reset is generated separately with --reset-guardian
    # It performs guardian approval checks, not assessment
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

- **Additional hook types** (post-merge, pre-rebase, pre-reset, etc.)
- **Plugin system** for custom validation tools
- **Remote validation** against repository state
- **Team collaboration** features for shared configurations
- **Performance monitoring** and optimization insights

## Related Commands

- [`goneat assess`](assess.md) - Core assessment engine
- [`goneat version`](version.md) - Version information
- [`goneat format`](format.md) - Code formatting (planned)
- [`goneat lint`](lint.md) - Code linting (planned)

## Command Execution

When using `goneat assess --hook <type> --hook-manifest <path>`, all commands defined in the manifest are executed, not just `assess` commands.

### Execution Order

Commands are executed in priority order (lower numbers run first):

```yaml
hooks:
  pre-commit:
    - command: "format"
      args: ["--check"]
      priority: 5 # Runs first
    - command: "make"
      args: ["precommit"]
      priority: 10 # Runs second
    - command: "assess"
      args: ["--categories", "lint"]
      priority: 15 # Runs third
```

For commands with equal priority, the original manifest order is preserved (stable sort).

### Command Types

| Command Type                   | Behavior                                |
| ------------------------------ | --------------------------------------- |
| `assess`                       | Runs internal goneat assessment         |
| `format`                       | Runs internal goneat format             |
| `dependencies`                 | Runs internal goneat dependencies check |
| `lint`, `security`, `validate` | Runs internal goneat commands           |
| Other (e.g., `make`, `npm`)    | Executed as external shell command      |

### Timeout Enforcement

Each command respects its configured timeout:

```yaml
- command: "make"
  args: ["test"]
  timeout: "90s" # Command killed after 90 seconds
```

Default timeout is 2 minutes if not specified.

### Fail-Fast Behavior

Execution stops on the first command failure. If `make precommit` fails, subsequent commands do not run.

## Security and Trust Model

The `.goneat/hooks.yaml` file executes commands with the same privileges as the user running the hook. This is the same trust model as:

- `Makefile` - can run arbitrary commands
- `.git/hooks/*` - can run arbitrary commands
- CI workflow files - can run arbitrary commands

**Important**: Anyone who can modify `hooks.yaml` can already execute arbitrary code through these other vectors. The hooks manifest has the same trust level as any other checked-in configuration file that defines executable commands.

### What Goneat Does

1. **Logs commands before execution** - aids debugging and provides an audit trail
2. **Enforces timeouts** - prevents runaway commands from blocking hooks
3. **Propagates exit codes** - command failures properly fail the hook

### What Goneat Does NOT Do

| Protection            | Why Not                                                   |
| --------------------- | --------------------------------------------------------- |
| Command allowlist     | Breaks flexibility. Makefile can call anything.           |
| Argument sanitization | Same trust boundary as Makefile.                          |
| Sandboxing            | Hooks need file write (formatting), network (tests), etc. |
| User confirmation     | Defeats automation purpose.                               |

## See Also

- [Git Hooks Operation Workflow](../workflows/git-hooks-operation.md) - Complete setup guide with diagrams
- [Assessment Architecture](../../architecture/assess-workflow.md) - Technical details
- [Hooks Architecture](../../architecture/hooks-command-architecture.md) - Design decisions
