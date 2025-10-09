---
title: goneat ssot
description: Manage Single Source of Truth (SSOT) asset synchronization
---

# `goneat ssot`

Manage Single Source of Truth (SSOT) asset synchronization from upstream repositories like Crucible.

## Synopsis

```bash
goneat ssot [command]
```

## Available Commands

- `sync` - Sync assets from SSOT repositories

## `goneat ssot sync`

Sync documentation, schemas, and other assets from configured SSOT repositories.

### Usage

```bash
goneat ssot sync [flags]
```

### Flags

| Flag | Description |
|------|-------------|
| `--local-path string` | Local path to source repository (overrides config) |
| `--dry-run` | Show what would be synced without performing sync |
| `--verbose` | Show verbose output including file-level operations |

### Examples

```bash
# Sync from configured SSOT repositories
goneat ssot sync

# Sync with local override path (for development)
goneat ssot sync --local-path ../crucible

# Dry run to preview changes
goneat ssot sync --dry-run

# Verbose output showing all file operations
goneat ssot sync --verbose
```

### Configuration

The `ssot sync` command reads configuration from:

- **`.goneat/ssot-consumer.yaml`** - Production configuration (committed to git)
- **`.goneat/ssot-consumer.local.yaml`** - Local overrides (gitignored, for development)

Configuration priority (highest to lowest):
1. Command-line flags (`--local-path`)
2. Environment variables (`GONEAT_SSOT_CONSUMER_<SOURCE>_LOCAL_PATH`)
3. `.goneat/ssot-consumer.local.yaml` (local overrides)
4. `.goneat/ssot-consumer.yaml` (production config)

### Configuration Example

**Production config** (`.goneat/ssot-consumer.yaml`):

```yaml
version: v1.1.0

sources:
  - name: crucible
    repo: fulmenhq/crucible
    ref: main
    sync_path_base: lang/go
    assets:
      - type: doc
        paths: ['docs/**/*']
        subdir: docs/crucible-go
      - type: schema
        paths: ['schemas/**/*']
        subdir: schemas/crucible-go
```

**Local override** (`.goneat/ssot-consumer.local.yaml`):

```yaml
version: v1.1.0

sources:
  - name: crucible
    localPath: ../crucible
```

### Workflow

The sync process follows these steps:

1. Load production configuration from `.goneat/ssot-consumer.yaml`
2. Merge local overrides from `.goneat/ssot-consumer.local.yaml` (if present)
3. Apply command-line flag overrides
4. Validate source repositories exist
5. Copy assets from source to destination directories
6. Report sync results

### Exit Codes

- `0` - Success
- `1` - Configuration error
- `2` - Source not found
- `3` - Sync operation failed

### Integration with Makefile

Common Makefile targets for SSOT operations:

```makefile
.PHONY: sync-ssot verify-ssot

sync-ssot: ## Sync SSOT assets from upstream repositories
	@echo "Syncing SSOT assets..."
	@dist/goneat ssot sync

verify-ssot: ## Verify SSOT assets are up-to-date
	@echo "Verifying SSOT sync..."
	@dist/goneat ssot sync --dry-run >/dev/null 2>&1
	@if git diff --exit-code docs/crucible-go schemas/crucible-go; then \
		echo "✓ SSOT content is up-to-date"; \
	else \
		echo "❌ SSOT content is stale - run 'make sync-ssot'"; \
		exit 1; \
	fi

bootstrap: sync-ssot ## Bootstrap development environment
```

### CI/CD Integration

Example GitHub Actions workflow to verify SSOT sync:

```yaml
name: SSOT Sync Check

on:
  pull_request:
    paths:
      - 'docs/crucible-go/**'
      - 'schemas/crucible-go/**'

jobs:
  check-sync:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Clone Crucible
        run: git clone https://github.com/fulmenhq/crucible.git ../crucible

      - name: Build goneat
        run: make build

      - name: Check if synced content is up-to-date
        run: |
          dist/goneat ssot sync --dry-run
          if git diff --exit-code docs/crucible-go schemas/crucible-go; then
            echo "✓ SSOT content is up-to-date"
          else
            echo "❌ SSOT content is stale - run 'make sync-ssot'"
            exit 1
          fi
```

### Development Workflow

1. **Initial setup**: Copy `.goneat/ssot-consumer.local.yaml.example` to `.goneat/ssot-consumer.local.yaml` and configure local paths
2. **Sync assets**: Run `make sync-ssot` to pull latest assets from upstream
3. **Verify sync**: Use `make verify-ssot` to ensure content is up-to-date before committing
4. **CI checks**: PRs affecting synced content will automatically verify sync status

### Troubleshooting

**Configuration not found**:
- Ensure `.goneat/ssot-consumer.yaml` exists and is valid
- Check that the file follows the expected schema

**Source not found**:
- Verify source repository exists at configured path
- For local development, ensure `localPath` points to correct directory
- Check environment variable overrides if using CI/CD

**Sync fails**:
- Run with `--verbose` flag for detailed file operation logs
- Use `--dry-run` to preview changes without modifying files
- Check file permissions and repository access

### See Also

- [`goneat content`](../content.md) - Manage embedded documentation and schemas
- [SSOT Library Documentation](../../appnotes/lib/ssot.md) - Programmatic SSOT operations
- [Crucible Bootstrap Guide](../../crucible-go/guides/bootstrap-goneat.md) - Setting up SSOT in new repositories