# Goneat v0.3.17 — Unified Ignore Scope for Lint Sidecars

# Goneat v0.3.18 — Checkmake Config Overrides (MVP)

**Release Date**: 2025-12-13
**Status**: Draft

## TL;DR

- **Less Noise**: Configure checkmake rules via `.goneat/assess.yaml`
- **MVP Scope**: Supports `max_body_length` and `min_phony_targets`

## What Changed

### checkmake configuration

`.goneat/assess.yaml` now supports generating a temporary checkmake config file and passing it via `checkmake --config`.

Supported settings:

- `lint.make.checkmake.config.max_body_length`
- `lint.make.checkmake.config.min_phony_targets`

---

# Goneat v0.3.17 — Unified Ignore Scope for Lint Sidecars

**Release Date**: 2025-12-13
**Status**: Release

## TL;DR

- **Less DRY + More Predictable**: Lint sidecars now respect `.gitignore` + `.goneatignore` by default
- **Force Include Works Everywhere**: `--force-include` applies consistently across lint integrations
- **Better Foundations for Future Sidecars**: Establishes scope contract for upcoming biome/ruff integrations

## What Changed

### Unified Ignore Scope

The glob-based lint integrations (shell, Makefile, GitHub Actions, YAML) previously relied on `.goneat/assess.yaml` exclude globs and could require duplicating ignore patterns that already exist in `.gitignore`/`.goneatignore`.

v0.3.17 makes path scope consistent:

- Files ignored by `.gitignore`/`.goneatignore` are not scanned by sidecars
- `goneat assess --force-include <glob>` can re-include ignored paths when needed
- `.goneat/assess.yaml` `ignore:` remains available as a tool-specific, additive exclusion layer

Affected integrations:

- shfmt / shellcheck
- actionlint
- checkmake
- yamllint target resolution

---

# Goneat v0.3.16 — Release Signing Dogfooding Fixes

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

**Previous Releases**: See `docs/releases/` for older release notes.
