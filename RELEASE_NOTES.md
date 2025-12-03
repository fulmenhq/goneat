# Goneat v0.3.11 — Windows Compatibility & Test Parallelization

**Release Date**: 2025-12-03
**Status**: Release

## TL;DR

- **Windows Platform Support**: goneat is now operational on Windows with proper binary naming, test compatibility, and line ending handling
- **Test Parallelization**: Added `t.Parallel()` to 124 tests across 3 packages with 1.79x speedup measured on Windows
- **SBOM Support**: New syft tool integration for Software Bill of Materials generation
- **Line Ending Standards**: Established `.gitattributes` for consistent cross-platform line endings
- **Critical Fix**: Resolved CRLF file corruption bug in formatter that was breaking Windows formatting

## Breaking Changes

None. All changes are backwards compatible.

## Highlights

### Install Probe CI Validation

New CI job validates that package manager + tool combinations are actually installable:

```bash
# Run locally (opt-in, requires network)
make install-probe
```

- Caught invalid `scoop` + `prettier` combination (scoop lacks prettier package)
- Uses build tag `installprobe` to avoid slowing down normal test runs
- Non-destructive: probes with `brew info`, `scoop info`, etc. - never installs
- Static validation runs in normal tests; runtime probe is opt-in

### Doctor Tool Installation Reliability

Multiple fixes improve package manager detection and tool installation:

- Route node-kind tools to `installSystemTool` for proper brew/bun installation
- Derive candidate binary names from `detect_command` for accurate post-install lookup
- Add brew to `GetShimPath` for proper PATH resolution
- Add detected package manager paths to PATH before tool installation
- Enhanced installer diagnostics with output capture and exit codes

### Multi-Scope Tools Init

Previously, `goneat doctor tools init` only generated a single scope in `.goneat/tools.yaml`. This caused issues when tests or other code expected all standard scopes to exist.

**Before (v0.3.9)**:

```bash
goneat doctor tools init --scope foundation
# Only creates foundation scope - security, format, all are missing
```

**After (v0.3.10)**:

```bash
goneat doctor tools init
# Creates all 4 scopes: foundation, security, format, all
# Scopes: 4, Tools: 13 (for Go repos)
```

### Package Manager Strategy Cleanup

The v0.3.10 release establishes a clear package manager strategy:

| Package Manager | Use Case                                                    |
| --------------- | ----------------------------------------------------------- |
| `brew`          | System binaries on darwin/linux (ripgrep, jq, yq, prettier) |
| `scoop/winget`  | System binaries on Windows                                  |
| `go-install`    | Go tools (golangci-lint, gosec, yamlfmt, etc.)              |
| `bun/npm`       | Node.js packages ONLY (eslint for TypeScript repos)         |
| `uv/pip`        | Python packages ONLY (ruff, etc.)                           |

**Removed from system tools**:

- `bun` - Cannot install system binaries, only npm packages
- `mise` - Version manager, not a general package manager

### CI Bootstrap Improvements

1. **Exit Code Propagation**: The Makefile bootstrap target now uses `&&` instead of `;` to chain commands, ensuring failures stop the build immediately.

2. **--no-cooling Flag**: For CI environments without network access to verify package release dates:

   ```bash
   goneat doctor tools --scope foundation --install --yes --no-cooling
   ```

3. **Go Version Parsing**: Fixed parsing of Go versions like "go1.25.4" by stripping the "go" prefix before semver comparison.

## Upgrade Notes

### For CI Pipelines

If your CI is failing due to cooling policy checks:

```bash
# Add --no-cooling to your bootstrap command
goneat doctor tools --scope foundation --install --yes --no-cooling
```

### For .goneat/tools.yaml

If your tools.yaml is missing scopes, regenerate it:

```bash
goneat doctor tools init --force
```

This will create a complete config with all 4 standard scopes.

## Files Changed

- `.github/workflows/ci.yml`: New install-probe CI job
- `Makefile`: Bootstrap target fix, --no-cooling support, install-probe target
- `.goneat/tools.yaml`: Removed scoop from prettier Windows installers
- `config/tools/foundation-tools-defaults.yaml`: Package manager cleanup
- `internal/doctor/tools.go`: Go version parsing fix
- `internal/doctor/tools_install_probe_test.go`: Runtime install probe tests
- `internal/doctor/tools_installability_test.go`: Static installability validation
- `internal/doctor/tools_defaults_loader.go`: ConvertToToolsConfigWithAllScopes
- `internal/doctor/package_managers.go`: PATH and shim detection fixes
- `cmd/doctor_tools_init.go`: Multi-scope generation
- `pkg/logger/fields.go`: Nil error handling fix
- `schemas/**/tools-config.yaml`: node/python kind enum

## Full Changelog

See [CHANGELOG.md](CHANGELOG.md) for complete details.

---

**Previous Release**: [v0.3.9](docs/releases/v0.3.9.md)
