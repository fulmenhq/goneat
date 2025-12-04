# Goneat v0.3.13 — Dynamic yamlfmt Detection & Crucible v0.2.22

**Release Date**: 2025-12-04
**Status**: Release

## TL;DR

- **Enterprise Feature**: `goneat doctor tools init` now auto-detects `.yamlfmt` indent configuration
- **CI Fix**: Tools installed via brew/bun now properly detected in CI environments
- **Crucible Update**: Synced to Crucible v0.2.22 with `node` and `python` installer kinds

## Breaking Changes

None. This is a feature/bugfix release, fully backwards compatible.

## What's New

### Dynamic yamlfmt Indent Detection

Previously, `goneat doctor tools init` hardcoded 2-space indentation. Now it reads your `.yamlfmt` configuration:

```bash
# Enterprise repo with 4-space indent
$ cat .yamlfmt
formatter:
  indent: 4

$ goneat doctor tools init --force
# Generated .goneat/tools.yaml now uses 4-space indent!
```

**Features**:

- Walks up directory tree to find `.yamlfmt` (monorepo-friendly)
- Security hardening: Rejects indent values outside 1-8 range
- Falls back to 2-space indent if no config found

**New Tests**:

- `TestDetectYamlfmtIndent` - Table-driven tests for all edge cases
- `TestDetectYamlfmtIndentWalksUpTree` - Parent directory detection
- `TestDoctorToolsInitUsesYamlfmtIndent` - Integration test

### CI Tool Detection Fix

Fixed issue where tools installed via brew or bun weren't detected in CI environments:

- Added `findToolPath()` helper that checks PATH then shim directories
- Fixed `DetectPackageManager()` to use `tools.DetectBrew()` for brew detection
- Format command now properly finds yamlfmt, prettier, goimports

### Crucible v0.2.22

Updated SSOT sync to Crucible v0.2.22:

- Schema `goneat-tools-config.schema.yaml` now includes `node` and `python` installer kinds
- Various config, schema, and documentation updates from upstream
- Resolves schema validation issues for node/python tools

## Upgrade Notes

### For CI Pipelines

If you were experiencing issues with tool detection after bootstrap:

```bash
# Update goneat version
GONEAT_VERSION="v0.3.13"

# Tools should now be properly detected after:
goneat doctor tools --scope foundation --install --yes
```

### For Enterprise Users

If your organization uses a custom yamlfmt indent (e.g., 4 spaces):

```bash
# Regenerate tools.yaml with correct indent
goneat doctor tools init --force
# Now matches your .yamlfmt configuration
```

## Files Changed

- `cmd/doctor_tools_init.go`: Add yamlfmt indent detection
- `cmd/doctor_tools_init_test.go`: Add detection tests
- `cmd/format.go`: Add findToolPath helper
- `internal/doctor/package_managers.go`: Fix brew detection
- `.goneat/ssot-consumer.yaml`: Update to Crucible v0.2.22
- Various Crucible-synced files in config/, docs/, schemas/

## Full Changelog

See [CHANGELOG.md](CHANGELOG.md) for complete details.

---

**Previous Releases**:

- [v0.3.12](docs/releases/v0.3.12.md) - yamlfmt Compatibility Fix
- [v0.3.11](docs/releases/v0.3.11.md) - Windows Compatibility & Test Parallelization
- [v0.3.10](docs/releases/v0.3.10.md) - Install Probe & Multi-Scope Tools Init
