# Goneat v0.3.16 — Release Build Integrity

**Release Date**: 2025-12-12
**Status**: Release

## TL;DR

- **CRITICAL FIX**: Release binaries now correctly report version (was showing `goneat dev`)
- **Release Safety**: Safeguards prevent checksum regeneration after signing
- **CI Validation**: VERSION/tag mismatch now fails release builds early
- **Documentation**: Updated bootstrap and install guides

## What's Fixed

### Release Build Version Embedding (CRITICAL)

Release binaries were reporting `goneat dev` instead of the actual version because `scripts/build-all.sh` was setting ldflags on a non-existent variable path.

**Before**:

```bash
$ goneat --version
goneat dev
```

**After**:

```bash
$ goneat --version
goneat v0.3.16
```

### Release Signature Invalidation

Added safeguards preventing checksum regeneration after signing:

- `release-checksums` target blocks regeneration if signatures exist
- New `release-verify-checksums` target for non-destructive validation
- RELEASE_CHECKLIST.md updated with one-way sequence warnings

## What's New

### VERSION/Tag Validation

Release workflow now validates VERSION file matches git tag before building. Prevents publishing releases where VERSION and tag diverge.

### Bootstrap Documentation

- New `docs/user-guide/bootstrap/sfetch.md`
- Updated `docs/user-guide/install.md`
- Enhanced `docs/appnotes/bootstrap-patterns.md`

### Release Scripts

- New `scripts/upload-release-assets.sh` for automated asset upload

## Breaking Changes

None. This is a fix release.

## Upgrade Notes

No action required. Download new binaries for correct version reporting.

---

# Goneat v0.3.15 — Lint Expansion & Hook Execution

**Release Date**: 2025-12-11
**Status**: Release

## TL;DR

- **Expanded Lint Coverage**: Added shell script (shfmt/shellcheck), Makefile (checkmake), and GitHub Actions (actionlint) linting
- **Hook Manifest Execution**: `goneat assess --hook` now executes ALL commands in hooks.yaml (make, assess, etc.) in priority order
- **Yamllint Integration**: Configurable YAML linting with strict mode for workflows and configs
- **DX Improvements**: Better error messages and graceful tool handling

## What's New

### Expanded Lint Capabilities

The `goneat assess --categories lint` command now includes comprehensive linting for:

- **Shell Scripts**: `shfmt` (BSD-3, format+fix) and `shellcheck` (GPL-3, verify-only)
- **Makefiles**: `checkmake` (MIT, comprehensive Makefile validation)
- **GitHub Actions**: `actionlint` (MIT, workflow validation)
- **YAML Files**: `yamllint` with configurable paths and strict mode

### Hook Manifest Execution

**BREAKING CHANGE FOR HOOK USERS**: Hook manifests now execute ALL commands, not just assess commands.

**Migration**: Update hooks to use check-only commands:

- `make format-all` → `make format-check`
- `make test` → `make test-fast`

### Developer Experience Improvements

- **Helpful Error Messages**: Using `--output json` now shows clear error message guiding users to `--format json` instead
- **Graceful Tool Skipping**: Missing tools skip with informative messages rather than failing
- **Container-Ready**: All new tools pre-installed in goneat-tools container

## Breaking Changes

None. All changes are additive and backwards compatible.

---

# Goneat v0.3.14 — Container-Based CI & ToolExecutor Infrastructure

**Release Date**: 2025-12-08
**Status**: Release

## TL;DR

- **Two-Path CI Validation**: Validates both container path (LOW friction) and package manager path (HIGHER friction)
- **Container-Probe**: Proves goneat works inside `ghcr.io/fulmenhq/goneat-tools` container (HIGH confidence)
- **Bootstrap-Probe**: Validates package manager installation (only runs if container passes)
- **Infrastructure**: New ToolExecutor package for future Docker-based tool execution
- **golangci-lint v2**: Updated to v2 install path and minimum version

## What's New

### Two-Path CI Validation

goneat now validates **two approaches** for tool availability with explicit dependency ordering:

```
build-test-lint
       ↓ (uploads goneat binary as artifact)
container-probe  ← LOW friction, validates first
       ↓ (only runs if container passes)
bootstrap-probe  ← HIGHER friction, validates second
```

### Container-Probe (Recommended for CI)

The `container-probe` job downloads the goneat binary and validates it works inside the container:

- **Proves goneat integration** - not just that tools exist (`--version`)
- **Same image everywhere** = same behavior (container IS the contract)
- **Pre-built tools**: prettier, yamlfmt, jq, yq, rg, git, bash

### ToolExecutor Package (Phase 1)

New infrastructure in `pkg/tools/executor*.go` for future Docker-based tool execution:

- `executor.go` - Interface and factory pattern
- `executor_local.go` - Local tool execution (existing behavior)
- `executor_docker.go` - Docker-based execution via goneat-tools
- `executor_auto.go` - Smart mode selection (CI → docker, local → local)

### golangci-lint v2

Updated install path for golangci-lint v2. Minimum version updated from 1.54.0 to 2.0.0.

## Breaking Changes

None. This is a feature/infrastructure release, fully backwards compatible.

---

**Previous Releases**: See `docs/releases/` for older release notes.
