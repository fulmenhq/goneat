# Goneat v0.3.12 — yamlfmt Compatibility Fix

**Release Date**: 2025-12-04
**Status**: Release

## TL;DR

- **Critical Fix**: `goneat doctor tools init` now generates yamlfmt-compatible `.goneat/tools.yaml` files
- **Impact**: Fresh clones no longer fail `make fmt` with "did not find expected key" errors
- **Regression Test**: Added test coverage to prevent future formatting incompatibilities

## Breaking Changes

None. This is a bugfix release, fully backwards compatible.

## The Problem

Previously, `goneat doctor tools init` generated `.goneat/tools.yaml` files that failed yamlfmt validation:

```bash
# After fresh clone
goneat doctor tools init
make fmt

# Result: ❌ FAILED
yamlfmt: .goneat/tools.yaml: yaml: line 15: did not find expected key
```

**Root Cause**:
- Used `yaml.Marshal()` with default 4-space indentation
- Repository `.yamlfmt` config requires 2-space indentation
- Generated files required manual reformatting before passing CI

**Impact**:
- Fresh clone workflows broke after initialization
- CI bootstrap (`init` → `format` → `check`) failed consistently
- Undermined "single source of truth" guarantee for tools.yaml
- Developers had to manually format generated config files

## The Fix

**Technical Changes**:
- Replaced `yaml.Marshal()` with `yaml.NewEncoder()` + `SetIndent(2)`
- Match indentation to `.yamlfmt` configuration (2 spaces)
- Remove extra blank line between header comments and content
- Properly handle `file.Close()` errors per linter requirements

**Verification**:
```bash
goneat doctor tools init --force
yamlfmt -dry .goneat/tools.yaml
# Output: "No files will be changed" ✅
```

**Regression Prevention**:
- New test: `TestDoctorToolsInitGeneratesYamlfmtCompatibleFile`
- Verifies generated files use proper 2-space indentation
- Ensures future changes maintain yamlfmt compatibility

## Upgrade Notes

### Immediate Action Required

If you're experiencing CI failures with forge-workhorse-groningen or similar repos:

1. **Update goneat version** in `scripts/install-goneat.sh`:
   ```bash
   GONEAT_VERSION="v0.3.12"
   ```

2. **Update SHA256 checksums** (from GitHub release):
   ```bash
   # Get checksums from release artifacts
   # Update platform-specific EXPECTED_SHA values
   ```

3. **Optionally regenerate** `.goneat/tools.yaml`:
   ```bash
   goneat doctor tools init --force
   # Now generates properly formatted config
   ```

### For Existing Installations

No action required if your `.goneat/tools.yaml` is already formatted correctly. The fix only affects newly generated files.

## Files Changed

- `cmd/doctor_tools_init.go`: Replace yaml.Marshal with yaml.NewEncoder
- `cmd/doctor_tools_init_test.go`: Add yamlfmt compatibility regression test
- `CHANGELOG.md`: Document v0.3.12 changes
- `VERSION`: Bump to v0.3.12

## Full Changelog

See [CHANGELOG.md](CHANGELOG.md) for complete details.

---

**Previous Releases**:
- [v0.3.11](docs/releases/v0.3.11.md) - Windows Compatibility & Test Parallelization
- [v0.3.10](docs/releases/v0.3.10.md) - Install Probe & Multi-Scope Tools Init
