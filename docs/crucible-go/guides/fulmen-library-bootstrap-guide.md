---
title: "Fulmen Library Bootstrap Guide"
description: "Bootstrap a new Fulmen ecosystem library with minimal configuration"
author: "Schema Cartographer"
author_of_record: "Dave Thompson (https://github.com/3leapsdave)"
supervised_by: "@3leapsdave"
date: "2025-10-08"
last_updated: "2025-10-08"
status: "draft"
tags: ["bootstrap", "fuldx", "setup", "new-repository", "library"]
---

# Fulmen Library Bootstrap Guide

This guide explains how to bootstrap a new Fulmen ecosystem library (Python, Go, TypeScript) from an empty repository with minimal manual configuration.

**Goal**: Get from empty GitHub repository to fully-integrated FulDX + Crucible setup in under 5 minutes.

**Audience**: Developers creating new libraries like pyfulmen, gofulmen-next, tsfulmen-next, etc.

**Not covered**: Full production DX setup with Goneat (release automation, changelog generation, etc.). This is minimal bootstrap - Goneat can be added later.

## Philosophy

Fulmen libraries follow a consistent bootstrap pattern:

1. **FulDX for DX workflows** - Version management, SSOT sync, tooling
2. **Crucible for standards** - Schemas, coding standards, integration patterns
3. **Language-specific conventions** - Linting, testing, building per language
4. **(Later) Goneat for releases** - When ready for production releases

This guide focuses on steps 1-3, getting you to "working library with tests and Crucible integration" quickly.

## Prerequisites

- Git repository created on GitHub
- Local clone of the repository
- Command line access (bash/zsh)
- (Optional) Local clone of Crucible repository at `../crucible` for faster syncing

## Quick Start

**Automated (future):**

```bash
# In your empty repository:
curl -L https://github.com/fulmenhq/fuldx/releases/latest/download/fuldx-$(uname -s)-$(uname -m) -o fuldx
chmod +x fuldx
./fuldx init --language python
make bootstrap
```

**Manual (current - until `fuldx init` is implemented):**

Follow [Step-by-Step Setup](#step-by-step-setup) below.

## Understanding FulDX + Crucible Integration

### Two-Tier Model

When you bootstrap with FulDX, you get two sources of Crucible content:

**1. Embedded in FulDX binary** (Read-only reference)

- FulDX downloads with Crucible docs/schemas already embedded
- Access offline: `fuldx docs --topic standards`
- Use case: Reading coding standards, browsing guides
- Updated: When FulDX releases (infrequent - quarterly or as-needed)

**2. Synced to `.crucible/` directory** (Your repo's copy)

- FulDX pulls fresh schemas/docs from Crucible GitHub repo
- Stored in: `.crucible/`, `schemas/crucible-ts/`, `docs/crucible-ts/`
- Use case: Runtime schema validation, CI documentation checks
- Updated: Anytime you run `make sync-crucible` (weekly or as-needed)

### Why Both?

```bash
# Developer workflow:
./bin/fuldx docs --topic logging
# Reads: Embedded docs (instant, offline)
# Shows: Crucible logging standards

# Your code workflow:
validate(data, schema_path="schemas/crucible-ts/pathfinder/v1.0.0/find-query.schema.json")
# Reads: Synced schemas (from .crucible/)
# Uses: Fresh schema for validation
```

**Analogy**: FulDX is like a "talking guidebook" - it has the guide built-in AND helps you fetch the latest maps (schemas) for your journey (code).

## Step-by-Step Setup

### Step 1: Repository Structure

```bash
# In your repository root
mkdir -p bin .fuldx .crucible/metadata

# Create VERSION file (CalVer or SemVer)
echo "0.1.0" > VERSION
```

### Step 2: Download FulDX Binary

```bash
# Detect platform and download
curl -L https://github.com/fulmenhq/fuldx/releases/latest/download/fuldx-$(uname -s)-$(uname -m) -o bin/fuldx
chmod +x bin/fuldx

# Verify
./bin/fuldx --version
```

**Platform detection:**

- macOS: `fuldx-Darwin-arm64` (Apple Silicon) or `fuldx-Darwin-x86_64` (Intel)
- Linux: `fuldx-Linux-x86_64` or `fuldx-Linux-arm64`
- Windows: `fuldx-Windows-x86_64.exe`

### Step 3: Configure Crucible Sync

Create `.fuldx/sync-consumer.yaml`:

```yaml
version: 1.0
sources:
  - name: crucible
    repo: fulmenhq/crucible
    ref: main
    output: .
    sync_path_base: "lang/typescript" # Adjust for lang/python or lang/go when available

    # Use sync keys for discoverability
    keys:
      - crucible.docs.standards # Coding standards, logging, etc.
      - crucible.docs.guides # Integration guides, bootstrap docs
      # Add more as needed - see: fuldx ssot keys crucible

    # Explicit asset paths (for fine control)
    assets:
      - type: schema
        paths: ["schemas/**/*"]
        subdir: schemas/crucible-ts
      - type: doc
        paths: ["docs/**/*"]
        subdir: docs/crucible-ts
      - type: config
        paths: ["config/sync/**/*"]
        subdir: .crucible/metadata
```

**Available sync keys:**

```bash
# List all keys
./bin/fuldx ssot keys crucible

# Common by use case:
# - All repos: crucible.docs.standards, crucible.docs.guides
# - Schema validation: crucible.schemas.pathfinder, crucible.schemas.logging
# - Terminal apps: crucible.schemas.terminal
```

### Step 4: Local Development Override (Optional)

If you have Crucible cloned locally, create `.fuldx/sync-consumer.local.yaml`:

```yaml
version: 1.0
sources:
  - name: crucible
    localPath: ../crucible
```

**Key points:**

- **Gitignored**: This file is already in `.gitignore` (never commit)
- **Precedence**: Local config overrides `sync-consumer.yaml`
- **Partial**: Only specify fields you want to override (just `localPath`)
- **Team flexibility**: Each developer can point to their own Crucible clone
- **Stable path**: `../crucible` is canonical for sibling repos

**Effect:**

```bash
# With .fuldx/sync-consumer.local.yaml:
make sync-crucible
# Syncs from: ../crucible (filesystem - instant)

# Without local override:
make sync-crucible
# Syncs from: https://github.com/fulmenhq/crucible (API - slower)
```

### Step 5: Create Makefile

Copy this minimal, language-agnostic template:

```makefile
# Fulmen Library Makefile
# Repository: $(notdir $(CURDIR))
# Bootstrapped with: FulDX

.PHONY: help
help:
	@echo "Available targets:"
	@echo "  make bootstrap         - Install dependencies and setup environment"
	@echo "  make sync-crucible     - Sync Crucible schemas and docs"
	@echo "  make lint              - Run linting"
	@echo "  make test              - Run tests"
	@echo "  make version-bump-*    - Bump version (patch/minor/major/calver)"
	@echo "  make clean             - Remove build artifacts"

# Download FulDX binary if not present
bin/fuldx:
	@echo "Installing FulDX..."
	@mkdir -p bin
	@curl -L https://github.com/fulmenhq/fuldx/releases/latest/download/fuldx-$$(uname -s)-$$(uname -m) -o bin/fuldx
	@chmod +x bin/fuldx
	@echo "✓ FulDX installed: $$(bin/fuldx --version)"

.PHONY: bootstrap
bootstrap: bin/fuldx
	@echo "Bootstrapping development environment..."
	# Add language-specific commands here:
	# Python: @pip install -e .[dev]
	# Go: @go mod tidy
	# TypeScript: @bun install
	@echo "✓ Bootstrap complete"

.PHONY: sync-crucible
sync-crucible: bin/fuldx
	@echo "Syncing Crucible assets..."
	@bin/fuldx ssot pull crucible
	@echo "✓ Crucible synced to .crucible/"

.PHONY: sync-ssot
sync-ssot: sync-crucible

.PHONY: lint
lint:
	@echo "Running linter..."
	# Add language-specific linter:
	# Python: @ruff check src/ tests/
	# Go: @golangci-lint run
	# TypeScript: @bunx biome check .

.PHONY: test
test:
	@echo "Running tests..."
	# Add language-specific test runner:
	# Python: @pytest tests/ -v
	# Go: @go test ./...
	# TypeScript: @bunx vitest run

.PHONY: version-bump-patch
version-bump-patch: bin/fuldx
	@bin/fuldx version bump patch

.PHONY: version-bump-minor
version-bump-minor: bin/fuldx
	@bin/fuldx version bump minor

.PHONY: version-bump-major
version-bump-major: bin/fuldx
	@bin/fuldx version bump major

.PHONY: version-bump-calver
version-bump-calver: bin/fuldx
	@bin/fuldx version bump calver

.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf dist/ build/ *.egg-info __pycache__/ .pytest_cache/
	@find . -type d -name "__pycache__" -exec rm -rf {} + 2>/dev/null || true
	@echo "✓ Clean complete"
```

**Customize for your language** by uncommenting the appropriate commands in:

- `bootstrap` target
- `lint` target
- `test` target

### Step 6: Configure Gitignore

Add to `.gitignore`:

```gitignore
# FulDX binary (downloaded during bootstrap)
/bin/fuldx

# Crucible synced assets (regenerated from source)
/.crucible/
!/.crucible/metadata/
schemas/crucible-*/
docs/crucible-*/

# FulDX local overrides (development only)
.fuldx/*.local.yaml
.fuldx/*.local.json
.crucible/*.local.yaml

# Language-specific artifacts
__pycache__/
*.pyc
.pytest_cache/
.coverage
node_modules/
dist/
build/
*.egg-info
```

### Step 7: Language-Specific Setup

#### Python (pyproject.toml)

```toml
[project]
name = "pyfulmen"
version = "0.1.0"
description = "Python Fulmen libraries"
authors = [
    {name = "3 Leaps Team", email = "dev@3leaps.net"}
]
requires-python = ">=3.11"
license = {text = "MIT"}
readme = "README.md"

dependencies = []

[project.optional-dependencies]
dev = [
    "pytest>=8.0.0",
    "pytest-cov>=4.1.0",
    "ruff>=0.1.0",
]

[build-system]
requires = ["hatchling"]
build-backend = "hatchling.build"

[tool.ruff]
line-length = 100
target-version = "py311"

[tool.pytest.ini_options]
testpaths = ["tests"]
python_files = ["test_*.py"]
```

**Makefile additions:**

```makefile
.PHONY: bootstrap
bootstrap: bin/fuldx
	@echo "Bootstrapping Python environment..."
	@pip install -e .[dev]
	@echo "✓ Python bootstrap complete"

.PHONY: lint
lint:
	@ruff check src/ tests/

.PHONY: test
test:
	@pytest tests/ -v
```

#### Go (go.mod)

```go
module github.com/fulmenhq/gofulmen-next

go 1.21

require (
    // Add dependencies
)
```

**Makefile additions:**

```makefile
.PHONY: bootstrap
bootstrap: bin/fuldx
	@echo "Bootstrapping Go environment..."
	@go mod tidy
	@echo "✓ Go bootstrap complete"

.PHONY: lint
lint:
	@golangci-lint run

.PHONY: test
test:
	@go test ./...
```

#### TypeScript (package.json)

```json
{
  "name": "@fulmenhq/tsfulmen-next",
  "version": "0.1.0",
  "description": "TypeScript Fulmen libraries",
  "type": "module",
  "scripts": {
    "build": "tsup",
    "test": "vitest run",
    "lint": "biome check .",
    "typecheck": "tsc --noEmit"
  },
  "devDependencies": {
    "@biomejs/biome": "^2.2.5",
    "tsup": "^8.0.0",
    "typescript": "^5.7.0",
    "vitest": "^2.1.0"
  }
}
```

**Makefile additions:**

```makefile
.PHONY: bootstrap
bootstrap: bin/fuldx
	@echo "Bootstrapping TypeScript environment..."
	@bun install
	@echo "✓ TypeScript bootstrap complete"

.PHONY: lint
lint:
	@bunx biome check .

.PHONY: test
test:
	@bunx vitest run
```

### Step 8: Initial Bootstrap

```bash
# Run bootstrap
make bootstrap

# Sync Crucible assets
make sync-crucible

# Verify setup
./bin/fuldx --version
./bin/fuldx info
ls -la .crucible/
```

**Expected output:**

```
✓ FulDX installed: 0.1.4
Bootstrapping development environment...
✓ Bootstrap complete
Syncing Crucible assets...
✓ Crucible synced to .crucible/
```

### Step 9: Verify Integration

```bash
# Check synced schemas
ls schemas/crucible-ts/

# Check synced docs
ls docs/crucible-ts/

# Browse embedded docs (offline)
./bin/fuldx docs --list
./bin/fuldx docs --topic standards

# Check version management
cat VERSION
./bin/fuldx version sync
```

### Step 10: First Commit

```bash
git add .
git commit -m "chore: bootstrap repository with FulDX"
git push origin main
```

## Bootstrap Checklist

Use this when setting up a new library:

- [ ] Repository created on GitHub
- [ ] Local clone exists
- [ ] `VERSION` file created (0.1.0)
- [ ] `bin/fuldx` downloaded and executable
- [ ] `.fuldx/sync-consumer.yaml` configured
- [ ] (Optional) `.fuldx/sync-consumer.local.yaml` for local Crucible
- [ ] `Makefile` created with standard targets
- [ ] `.gitignore` updated for FulDX/Crucible
- [ ] Language-specific files created (pyproject.toml/go.mod/package.json)
- [ ] `make bootstrap` runs successfully
- [ ] `make sync-crucible` runs successfully
- [ ] `./bin/fuldx --version` shows version
- [ ] `.crucible/` directory populated
- [ ] `make lint` and `make test` work (even if no code yet)
- [ ] Initial commit pushed to main

## Common Patterns

### Version Management

```bash
# Bump patch version (0.1.0 → 0.1.1)
make version-bump-patch

# Bump minor version (0.1.1 → 0.2.0)
make version-bump-minor

# Bump major version (0.2.0 → 1.0.0)
make version-bump-major

# Use CalVer (YYYY.MM.PATCH)
make version-bump-calver
```

### Sync Patterns

```bash
# Sync from GitHub (uses .fuldx/sync-consumer.yaml)
make sync-crucible

# Sync from local Crucible (uses .fuldx/sync-consumer.local.yaml if present)
make sync-crucible

# One-time sync from specific path
./bin/fuldx ssot pull crucible --local-path /path/to/crucible

# Check what sync keys are available
./bin/fuldx ssot keys crucible
```

### Documentation Access

```bash
# List embedded topics
./bin/fuldx docs --list

# View specific topic
./bin/fuldx docs --topic standards
./bin/fuldx docs --topic logging

# View specific file
./bin/fuldx docs --file docs/crucible-ts/standards/typescript.md

# Search documentation
./bin/fuldx docs --search "logging"
```

## When to Add Goneat

After initial bootstrap and some development, consider adding Goneat when:

- Ready for automated releases
- Need changelog generation
- Want release checklist enforcement
- Need cross-repo version coordination
- Ready for production release automation

Goneat-based release automation will be documented once the CLI owns SSOT sync and publishing tasks.

## Troubleshooting

### FulDX Download Fails

```bash
# Check platform detection
uname -s  # Should be: Darwin, Linux
uname -m  # Should be: x86_64, arm64

# Manual download (example for macOS ARM)
curl -L https://github.com/fulmenhq/fuldx/releases/latest/download/fuldx-Darwin-arm64 -o bin/fuldx
chmod +x bin/fuldx
```

### Crucible Sync Fails

```bash
# Check network connectivity
curl -I https://api.github.com/repos/fulmenhq/crucible

# Use local clone instead
echo "version: 1.0\nsources:\n  - name: crucible\n    localPath: ../crucible" > .fuldx/sync-consumer.local.yaml
make sync-crucible

# Check available keys
./bin/fuldx ssot keys crucible
```

### Makefile Won't Run

```bash
# Check make is installed
make --version

# Check file has Unix line endings
dos2unix Makefile  # If on Windows

# Run specific target
make bootstrap
```

### Version Sync Issues

```bash
# Check VERSION file exists
cat VERSION

# Manual version sync
./bin/fuldx version sync

# Verify sync (language-specific)
grep version pyproject.toml  # Python
cat go.mod | grep module     # Go
grep version package.json    # TypeScript
```

## Advanced Topics

### Multi-Source SSOT Sync

Sync from multiple repositories:

```yaml
# .fuldx/sync-consumer.yaml
version: 1.0
sources:
  - name: crucible
    repo: fulmenhq/crucible
    ref: main
    output: .
    keys:
      - crucible.docs.standards

  - name: cosmography
    repo: fulmenhq/cosmography
    ref: main
    output: docs/cosmography
    assets:
      - type: doc
        paths: ["docs/**/*"]
```

### CI/CD Integration

```yaml
# .github/workflows/test.yml
name: Test
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Bootstrap FulDX
        run: make bin/fuldx

      - name: Sync Crucible
        run: make sync-crucible

      - name: Run tests
        run: make test
```

## Future: `fuldx init` Command

**Vision:** One command to bootstrap everything.

```bash
# In empty repository
curl -L https://github.com/fulmenhq/fuldx/releases/latest/download/fuldx-$(uname -s)-$(uname -m) -o fuldx
chmod +x fuldx

# Initialize Python library
./fuldx init --language python --template library

# Creates:
# - .fuldx/sync-consumer.yaml
# - Makefile (with language-specific targets)
# - VERSION (0.1.0)
# - .gitignore
# - pyproject.toml (minimal)
# - src/{{package_name}}/__init__.py
# - tests/conftest.py
# - README.md (template)
```

**Status:** Planned for FulDX v0.2.0

## See Also

- [FulDX Documentation](https://github.com/fulmenhq/fuldx/tree/main/docs) - FulDX CLI commands and usage
- [SSOT Sync Guide](sync-consumers-guide.md) - Advanced sync patterns
- [Repository Versioning](../standards/repository-versioning.md) - CalVer vs SemVer
- [Makefile Standard](../standards/makefile-standard.md) - Standard targets
- Goneat integration guide (TBD) - Release automation patterns

---
