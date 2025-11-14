# Goneat v0.3.6 — Package Manager Installation & Tools Cooling Policy

**Release Date**: 2025-11-12
**Status**: Release

## TL;DR

- **Package Manager Installation**: Complete implementation of package manager-based tool installation (Homebrew, Scoop)
- **Tools Cooling Policy**: Supply chain security enforcement for external tool installation with configurable age/download thresholds
- **Schema v1.1.0**: Full execution of declarative package manager installs and cooling policy configuration
- **Doctor Integration**: Package manager status display, installation verification, and cooling policy enforcement
- **Cross-Platform**: Native support for macOS/Linux (brew) and Windows (scoop)
- **Example Manifests**: Ready-to-use configuration examples for brew formula, brew cask, scoop, and cooling overrides

## What's New

### Package Manager Installation Engine

Completes the package manager installation feature introduced in v0.3.4 (schema-only). Tools can now be installed declaratively via Homebrew (macOS/Linux) or Scoop (Windows) using structured configuration.

**Example Configuration** (`.goneat/tools.yaml`):

```yaml
tools:
  goneat:
    name: goneat
    kind: system
    description: "Fulmen CLI for neat code workflows"
    detect_command: "goneat --version"
    install:
      type: package_manager
      package_manager:
        manager: brew
        tap: fulmenhq/homebrew-tap
        package: fulmenhq/tap/goneat
        package_type: formula
        flags: ["--quiet"]
```

**Usage**:

```bash
# Check package manager availability
goneat doctor tools --scope foundation

# Dry-run to see what would be installed
goneat doctor tools --config .goneat/tools.yaml --dry-run

# Install tools via package manager
goneat doctor tools --config .goneat/tools.yaml --install --yes
```

### Key Features

#### 1. Platform-Aware Detection
- Automatically detects Homebrew (macOS/Linux) or Scoop (Windows)
- Version parsing for installed package managers
- Clear status reporting with installation URLs for missing managers

#### 2. Brew Installer
- **Tap Support**: Automatically adds custom taps (e.g., `fulmenhq/homebrew-tap`)
- **Formula & Cask**: Install CLI tools (formula) or GUI apps (cask)
- **Custom Flags**: Pass additional flags like `--quiet`, `--force`
- **Idempotent**: Safe to run multiple times

#### 3. Scoop Installer
- **Bucket Support**: Automatically adds custom buckets (e.g., `extras`)
- **Custom Flags**: Pass flags like `--no-cache`, `--global`
- **Windows Native**: First-class Windows package management

#### 4. Doctor Command Integration
- Package manager status shown before tool checks
- Clear remediation when managers not available
- `--install-package-managers` flag (shows manual installation instructions)

#### 5. Dry-Run Mode
```bash
goneat doctor tools --config .goneat/examples/tools-brew-formula.yaml --dry-run
```
Shows what commands would execute without making changes.

### Example Manifests

Three ready-to-use examples included in `.goneat/examples/`:

1. **tools-brew-formula.yaml**: CLI tools via Homebrew formulas (jq, ripgrep, goneat)
2. **tools-brew-cask.yaml**: GUI applications via Homebrew casks (Docker Desktop, VS Code)
3. **tools-scoop.yaml**: Windows tools via Scoop (ripgrep, jq, git)

**Validate Examples**:
```bash
goneat doctor tools --config .goneat/examples/tools-brew-formula.yaml --validate-config
```

### Implementation Phases

This release completes Phases 2-6 of the package manager feature:

- **Phase 1** (v0.3.4): Schema v1.1.0 with `install.type: package_manager` support
- **Phase 2** (v0.3.6): Detection layer with BrewManager/ScoopManager (261 LOC)
- **Phase 3** (v0.3.6): Installer implementations for brew/scoop (326 LOC)
- **Phase 4** (v0.3.6): Doctor command integration with status display
- **Phase 5** (v0.3.6): Documentation and example manifests
- **Phase 6** (v0.3.6): Comprehensive test coverage (32 new tests, 953 LOC)

### Technical Details

**Files Added**:
- `pkg/tools/package_managers.go`: Detection and status reporting
- `pkg/tools/installer_brew.go`: Homebrew installer implementation
- `pkg/tools/installer_scoop.go`: Scoop installer implementation
- `pkg/tools/package_managers_test.go`: Detection tests (503 LOC)
- `pkg/tools/installer_brew_test.go`: Brew installer tests (231 LOC)
- `pkg/tools/installer_scoop_test.go`: Scoop installer tests (219 LOC)
- `.goneat/examples/`: Example manifests + README

**Files Modified**:
- `pkg/tools/installer.go`: Added InstallWithPackageManager factory
- `pkg/tools/installer_test.go`: Added integration tests
- `cmd/doctor.go`: Package manager status display
- `internal/doctor/tools.go`: Status API forwarding

**Backward Compatibility**: v1.0.0 manifests with `install_commands` continue to work. The schema change is additive (minor version bump).

### Tools Cooling Policy (Supply Chain Security)

Blocks installation of newly-published external tools until they meet minimum age and download thresholds, protecting against supply chain attacks where malicious actors publish or compromise tool releases.

**Why It Matters**: 80% of supply chain attacks are detected within the first 7 days of package publication. Recent attacks on popular tools demonstrate the risk of adopting freshly-published releases.

**Example Configuration** (`.goneat/dependencies.yaml` - global policy):

```yaml
version: v1
cooling:
  enabled: true
  min_age_days: 7              # Require tools to be ≥7 days old
  min_downloads: 100           # Require ≥100 total downloads
  min_downloads_recent: 10     # Require ≥10 recent downloads
  alert_only: false            # Block installation (not just warn)
  grace_period_days: 3         # 3-day grace for initial publication
  exceptions:
    - pattern: "github.com/fulmenhq/*"
      reason: "Internal packages are pre-vetted"
      approved_by: "@3leapsdave"
```

**Tool-Specific Overrides** (`.goneat/tools.yaml` - risk-based policies):

```yaml
tools:
  syft:
    name: syft
    description: "SBOM generation tool"
    cooling:
      min_age_days: 14         # Stricter for critical SBOM tool
      min_downloads: 5000      # Higher threshold for supply chain tool
      min_downloads_recent: 100
```

**Usage**:

```bash
# Check tools with cooling policy enforcement
goneat doctor tools --scope foundation

# Install with cooling checks (blocks new releases)
goneat doctor tools --scope foundation --install --yes

# Bypass cooling for offline/air-gapped environments
goneat doctor tools --scope foundation --install --no-cooling
```

### Key Features

#### 1. 3-Level Configuration Hierarchy

Policy resolution follows this precedence:

1. **CLI Flag** (`--no-cooling`) - Disables for this run
2. **Tool-Specific** (`.goneat/tools.yaml`) - Per-tool overrides
3. **Global Default** (`.goneat/dependencies.yaml`) - Organization-wide policy

Example hierarchy in action:
- `syft`: 14 days / 5000 downloads (tool-specific override)
- `ripgrep`: 7 days / 100 downloads (global default)
- `jq`: 7 days / 100 downloads (global default)
- With `--no-cooling`: All checks disabled for this run

#### 2. GitHub Releases API Integration

- Fetches release metadata (publish date, download counts) from GitHub Releases API
- 24-hour caching layer minimizes rate-limit risk
- Automatic fallback to latest release when tool version not specified
- Repository auto-detection from artifact URLs and common tool name patterns
- Rate limit parsing with clear "retry in X minutes" messaging

#### 3. Alert-Only Mode

For gradual adoption or monitoring:

```yaml
cooling:
  enabled: true
  alert_only: true  # Warn but don't block installation
```

Useful for:
- Pilot deployments to measure policy impact
- Teams transitioning from no policy to strict enforcement
- Development environments where cooling is informational

#### 4. Offline Support

Use `--no-cooling` flag for:
- Air-gapped environments without GitHub API access
- CI/CD pipelines with network restrictions
- Emergency deployments requiring immediate tool adoption

### Technical Details

**Files Added**:
- `pkg/tools/metadata/registry.go`: Metadata fetching registry with caching
- `pkg/tools/metadata/cache.go`: 24-hour TTL cache implementation
- `pkg/tools/metadata/github.go`: GitHub Releases API client
- `pkg/tools/metadata/errors.go`: Structured error types
- `internal/doctor/cooling.go`: Cooling policy enforcement
- `pkg/tools/config.go`: CoolingConfig structs and inheritance logic
- Comprehensive test coverage (515 LOC across metadata and config tests)

**Files Modified**:
- `cmd/doctor.go`: Added `--no-cooling` flag and enforcement integration
- `internal/doctor/tools.go`: Extended Tool struct with cooling config
- `schemas/tools/v1.1.0/tools-config.yaml`: Added `cooling` field to schema

**Documentation**:
- Enhanced `docs/guides/package-cooling-policy.md` with tool-specific overrides
- Updated `docs/appnotes/dogfooding-dependency-protection.md` with hierarchy examples
- Living example in `.goneat/tools.yaml` (syft with stricter policy)

### Use Cases

**Critical SBOM Tools** (stricter policy):
```yaml
syft:
  cooling:
    min_age_days: 14
    min_downloads: 5000
```
SBOM tools document your entire supply chain - compromised SBOM tooling could hide vulnerabilities.

**Standard CLI Tools** (use global defaults):
Tools like `ripgrep`, `jq`, `golangci-lint` use organization-wide policy (typically 7 days).

**Low-Risk Tools** (disable cooling):
```yaml
local-dev-tool:
  cooling:
    enabled: false
```
For non-production tools or when risk is acceptable.

## What's Fixed

### Platform Filtering for Doctor Tools

Fixed a critical bug where `goneat doctor tools` was incorrectly checking platform-specific tools on incompatible platforms, causing false failures in multi-platform CI/CD pipelines.

**Problem**: When using shared tool configurations across platforms (common in template repositories and CI/CD), Windows-only tools like `scoop` were reported as "missing" on macOS/Linux, and Unix-only tools like `mise` were reported as "missing" on Windows. This caused `goneat doctor tools` to exit with code 1 even when all platform-applicable tools were present.

**Root Cause**: The `GetToolsForScope()` function returned all tools in a scope without filtering by platform. Tools with `platforms: ["windows"]` were checked on all platforms, leading to false "missing tool" errors.

**Solution**: Added platform filtering before checking tools in all doctor modes:

- **New Helper Function**: `SupportsCurrentPlatform()` checks if a tool is applicable to the current platform
- **Platform Matching**:
  - Empty `platforms` list = supports all platforms (no restriction)
  - `*` or `all` in list = supports all platforms (explicit wildcard)
  - Otherwise, current platform must be explicitly listed
- **Applied to All Modes**: check, install, dry-run, check-updates

**Impact**:

- ✅ **Multi-Platform CI/CD**: Shared tool configs now work across different runner platforms
- ✅ **Template Repositories**: Bootstrap scripts with platform-specific tools no longer fail
- ✅ **Make Targets**: `make bootstrap` commands succeed on all supported platforms

**Example**:

```yaml
# .goneat/tools.yaml (shared across platforms)
scopes:
  bootstrap:
    tools: ["curl", "scoop", "mise"]

tools:
  curl:
    platforms: ["darwin", "linux", "windows"]  # All platforms
  scoop:
    platforms: ["windows"]  # Windows-only
  mise:
    platforms: ["darwin", "linux"]  # Unix-only
```

**Before Fix** (on macOS):
```bash
goneat doctor tools --scope bootstrap
# ERROR: scoop missing (incorrectly checked on macOS)
# EXIT: 1 (failure)
```

**After Fix** (on macOS):
```bash
goneat doctor tools --scope bootstrap
# INFO: curl present
# INFO: mise present
# DEBUG: Skipping scoop (not applicable to darwin platform)
# EXIT: 0 (success)
```

**Test Coverage**: 12 unit tests covering platform matching scenarios, plus integration test for the bug scenario.

**Files Modified**:
- `internal/doctor/tools.go`: Added `SupportsCurrentPlatform()` helper (35 LOC)
- `cmd/doctor.go`: Applied filtering to all tool check modes (4 locations)
- `internal/doctor/tools_test.go`: Platform filtering unit tests (198 LOC)
- `cmd/doctor_test.go`: Integration test for bug scenario

### Manual Installer Execution

Fixed a critical bug where manual installers were never executed, blocking package manager bootstrap workflows.

**Problem**: The manual installer kind was designed for bootstrapping package managers (mise, scoop) via official install scripts, but `isInstallerAvailable()` returned `false` for `installerManual`, causing these commands to be skipped entirely.

**Root Cause**: Manual installer was treated like other installers that require a package manager to be present. However, manual installers are just scripts/commands to execute and don't have prerequisites.

**Solution**: Changed `isInstallerAvailable()` to return `true` for `installerManual`, treating it as always available.

**Impact**:
- ✅ **CI/CD Bootstrap**: Automated workflows can now bootstrap package managers from scratch
- ✅ **Template Repositories**: Projects can include package manager bootstrap in their tool configs
- ✅ **Fresh Workstations**: Onboarding developers can run `goneat doctor tools --scope bootstrap --install` to auto-setup

**Example Configuration**:

```yaml
# .goneat/tools.yaml (bootstrap scope)
tools:
  mise:
    name: "mise"
    description: "Polyglot runtime manager"
    kind: "system"
    detect_command: "mise --version"
    platforms: ["linux", "darwin"]
    installer_priority:
      linux: ["manual"]
      darwin: ["manual"]
    install_commands:
      manual: |
        curl https://mise.jdx.dev/install.sh | sh && \
        echo 'Add $HOME/.local/bin to PATH'
```

**Usage**:

```bash
# Automatically bootstrap mise if missing
goneat doctor tools --scope bootstrap --install --yes

# Verify installation
mise --version

# Add to PATH for persistence
export PATH="$HOME/.local/bin:$PATH"
```

**Test Coverage**: 3 new unit tests for manual installer availability + integration test for bootstrap scenario.

**Files Modified**:
- `internal/doctor/tools.go`: Fixed `isInstallerAvailable()` for manual installer (3 LOC)
- `internal/doctor/tools_test.go`: Manual installer unit tests (155 LOC)
- `cmd/doctor_test.go`: Bootstrap integration test (48 LOC)

### Install Commands Documentation Clarification

Fixed misleading documentation that caused widespread confusion about `install_commands` syntax.

**Problem**: Documentation and examples showed `install_commands` using platform keys (`linux:`, `darwin:`, `windows:`), but the code only recognizes **installer-kind keys** (`mise`, `brew`, `apt-get`, `manual`, etc.). Platform keys were silently ignored, causing tools to never install.

**Root Cause**: Historical documentation carried over platform key examples when the feature was originally designed differently. The code evolved to use installer-kind keys for flexibility, but docs weren't updated.

**Solution**:

1. **Code Validation**: Added `ValidateInstallerCommands()` function that warns when:
   - Platform keys (linux, darwin, windows) are used instead of installer-kind keys
   - Unknown/unrecognized keys are used

2. **Documentation Updates**: Fixed all examples in:
   - `docs/appnotes/lib/tools.md` - Added comprehensive "Understanding Installer Kinds vs Platforms" section
   - `docs/appnotes/tools-runner-usage.md` - Fixed all examples to use installer-kind keys with priority fallbacks
   - `docs/user-guide/bootstrap/package-managers.md` - Added automatic bootstrap section with v0.3.6 examples

**Example Warnings**:

```bash
# User config with platform keys (WRONG):
tools:
  ripgrep:
    install_commands:
      linux: "apt-get install ripgrep"  # This is ignored!

# Warning output:
WARN Tool ripgrep: install_commands key 'linux' looks like a platform.
     Use installer-kind keys instead (mise, brew, apt-get, manual, etc.).
     Platform keys are ignored by the installer.
```

**Correct Usage**:

```yaml
# ✅ CORRECT - Installer-kind keys with priority
tools:
  ripgrep:
    platforms: ["linux", "darwin", "windows"]
    installer_priority:
      linux: ["mise", "apt-get"]
      darwin: ["mise", "brew"]
      windows: ["mise", "scoop"]
    install_commands:
      mise: "mise use -g ripgrep@latest"
      apt-get: "sudo apt-get install -y ripgrep"
      brew: "brew install ripgrep"
      scoop: "scoop install ripgrep"
```

**Impact**:
- ✅ Users will see warnings when using wrong keys
- ✅ All documentation now consistently shows correct patterns
- ✅ Explains why installer-kind approach is better (flexible, platform-independent, priority control)

**Files Modified**:
- `internal/doctor/tools.go`: Added `ValidateInstallerCommands()` with platform detection (42 LOC)
- `docs/appnotes/lib/tools.md`: Added 80 lines of installer-kinds explanation + corrected examples
- `docs/appnotes/tools-runner-usage.md`: Fixed 3 examples to use installer-kind keys
- `docs/user-guide/bootstrap/package-managers.md`: Added 50 lines on automatic bootstrap with mise example

## Breaking Changes

None. This is a purely additive feature.

## Known Limitations

- **Auto-Install**: Package manager auto-installation (`--install-package-managers`) shows manual instructions but does not execute automatic installation. This is intentional for security and will be considered for v0.3.7+ based on user feedback.
- **Manager Support**: Currently supports Homebrew and Scoop. Other package managers (apt, dnf, winget) may be added in future releases.

## Upgrade Guide

No changes required. Existing configurations continue to work. To adopt package manager installations:

1. Update your `.goneat/tools.yaml` to use v1.1.0 schema
2. Replace `install_commands` with `install.package_manager` configuration
3. Run `goneat doctor tools --validate-config` to verify
4. Test with `--dry-run` before production use

See `.goneat/examples/` for reference configurations.

## Installation

```bash
# Go install (recommended)
go install github.com/fulmenhq/goneat@v0.3.6

# Homebrew (macOS/Linux)
brew upgrade goneat

# Verify installation
goneat version
```

## What's Next (v0.3.7+)

- Dependency vulnerability scanning integration
- Package manager auto-install enhancement (optional)
- Additional language support (TypeScript, Python) in v0.4.x

---

# Goneat v0.3.5 — Dependencies Bug Fix

**Release Date**: 2025-11-11
**Status**: Release

## TL;DR

- **Dependencies Bug Fix**: Fixed `goneat dependencies` language detection for Go projects without root .go files
- **Impact**: Projects with code in subdirectories (cmd/, pkg/, internal/) now work correctly
- **Fast Follow**: Critical bug fix for v0.3.0 dependencies feature affecting sumpter and similar project structures

## What's Fixed

### Dependencies Language Detection

Fixed a critical bug where `goneat dependencies` would fail on Go projects that don't have `.go` files in the repository root directory.

**Problem**: The `go-licenses` library integration was passing `"."` as the scan pattern, which requires Go source files to exist in the current directory. Projects with all source code in subdirectories (cmd/, pkg/, internal/) would fail with:

```
Error: errors for ["."]:
.: -: no Go files in /path/to/project
```

**Solution**: Changed the scan pattern from `"."` to `"./..."` to scan all packages in the module, regardless of directory structure.

**Affected Projects**:

- ✅ **goneat** (has `main.go` at root) - worked before, still works
- ✅ **sumpter** (all code in subdirectories) - failed before, now works
- ✅ **Any Go project** with go.mod at root but no .go files in root directory

**Technical Details**:

Location: `pkg/dependencies/go_analyzer.go:64`

```go
// Before (v0.3.0-v0.3.4)
libraries, err := licenses.Libraries(ctx, classifier, false, nil, ".")

// After (v0.3.5+)
libraries, err := licenses.Libraries(ctx, classifier, false, nil, "./...")
```

The `./...` pattern is standard Go convention for "all packages in module" and works correctly whether or not there are .go files at the root.

## What's Changed

### Dependency Updates

- **go-git**: Updated from v5.16.2 to v5.16.3 for latest bug fixes and improvements

## Installation

```bash
# Go install (recommended)
go install github.com/fulmenhq/goneat@v0.3.5

# Verify installation
goneat version
# Output: goneat v0.3.5
```

## Upgrade Notes

**No configuration changes required**. Simply upgrade to v0.3.5:

```bash
go install github.com/fulmenhq/goneat@v0.3.5
```

**For sumpter users**: The dependencies infrastructure you set up in v0.3.4 will now work correctly without the language detection error. No changes needed to your `.goneat/dependencies.yaml` configuration.

## Testing

Tested against two project structures:

1. **goneat** (root .go files): `main.go` at root + cmd/pkg structure
2. **sumpter** (no root .go files): Only go.mod at root, all code in cmd/pkg

Both projects now successfully run `goneat dependencies --licenses` and generate correct dependency reports.

## Links

- **Repository**: https://github.com/fulmenhq/goneat
- **CHANGELOG**: See [CHANGELOG.md](../../CHANGELOG.md) for detailed changes
- **Previous Release**: [v0.3.4](v0.3.4.md) - Package Managers & SSOT DX
- **Dependencies Feature**: Introduced in [v0.3.0](v0.3.0.md)

---

# Goneat v0.3.4 — Package Managers & SSOT DX

**Release Date**: 2025-11-08
**Status**: Release

## TL;DR

- **Package Manager Installation**: Tools config v1.1.0 with declarative Homebrew/Scoop installations
- **SSOT Remote Cloning**: Automatic GitHub repository cloning with go-git (no local checkout needed)
- **SSOT Force-Remote**: Explicit remote sync with improved auto-detection DX
- **Schema Versioning**: Provenance schemas v1.1.0 with force-remote tracking
- **Dates Assessment Fix**: Eliminated noise from dates reports (96 info messages → 0 when correct)
- **Developer Experience**: .local.yaml now signals local dev intent for cleaner workflows

## What's New

### Package Manager Installation Support

Tools configuration schema v1.1.0 introduces structured package manager installation support, enabling declarative configuration for Homebrew and Scoop installations.

**New `install` Field**:

```yaml
version: v1.1.0

tools:
  - name: ripgrep
    description: Fast recursive grep
    kind: infrastructure
    detect_command: rg --version
    install: # New declarative format
      package_manager: brew
      package_name: ripgrep
      tap: homebrew/core # Optional
      binary_name: rg # Optional - if different from package name
      destination: /opt/homebrew/bin # Optional
      flags: # Optional - for complex installations
        - --force
        - --overwrite
```

**Features**:

- **Declarative Configuration**: Structured YAML instead of shell commands
- **Package Manager Support**: Homebrew (macOS/Linux), Scoop (Windows)
- **Custom Taps/Buckets**: Support for third-party package sources
- **Binary Name Mapping**: Handle cases where package name ≠ binary name
- **Installation Destinations**: Specify custom install locations
- **Multiple Flags**: Support complex installation scenarios
- **Schema Validation**: Enforced mutual exclusivity with legacy `install_commands`
- **Better Error Messages**: Clear validation feedback for configuration issues

**Migration Path**:

```yaml
# Old format (v1.0.0)
tools:
  - name: ripgrep
    install_commands:
      - "brew install ripgrep"

# New format (v1.1.0)
tools:
  - name: ripgrep
    install:
      package_manager: brew
      package_name: ripgrep
```

**Breaking Change Note**: `install` and `install_commands` are mutually exclusive (enforced via schema). Choose one approach per tool.

### SSOT Force-Remote Sync

Enable explicit remote syncing even when local directories exist, with improved developer experience through smarter auto-detection.

**New Command Options**:

```bash
# Force remote sync (ignore local auto-detection)
goneat ssot sync --force-remote

# Force remote via environment variable
GONEAT_FORCE_REMOTE_SYNC=1 goneat ssot sync

# Per-source config option
# .goneat/ssot-consumer.yaml:
sources:
  - name: crucible
    repo: fulmenhq/crucible
    ref: v0.2.8
    force_remote: true  # Always use remote
```

**DX Improvement - Auto-Detection Signal**:

The major DX improvement in v0.3.4 is making `.local.yaml` presence signal local development intent:

**Before v0.3.4**:

- Auto-detection always ran if `../crucible` directory existed
- Even without `.local.yaml`, goneat would use local directory
- Needed `--force-remote` flag to test remote sync behavior
- Confusing for production/CI usage

**After v0.3.4**:

- Auto-detection only runs when `.local.yaml` exists
- Absence of `.local.yaml` signals "use production config"
- No need for `--force-remote` in common case
- Clear signal: `.local.yaml` = local dev, no `.local.yaml` = production

**Configuration Precedence**:

1. **Command-line flags** (`--local-path` or `--force-remote`)
2. **Environment variables** (`GONEAT_FORCE_REMOTE_SYNC=1`)
3. **Local override** (`.goneat/ssot-consumer.local.yaml`)
4. **Primary manifest** (`.goneat/ssot-consumer.yaml`)
5. **Auto-detection** (`../<source>`) - **only if `.local.yaml` exists**

**Example: TSFulmen Testing**:

```bash
# Before v0.3.4 - needed flag
cd tsfulmen
rm .goneat/ssot-consumer.local.yaml  # Remove local config
goneat ssot sync --force-remote      # Still needed flag!

# After v0.3.4 - clean workflow
cd tsfulmen
# No .local.yaml? Auto-detection disabled automatically
goneat ssot sync  # Uses production config (remote)
```

**Use Cases**:

- **Local Development**: Create `.local.yaml` pointing to `../crucible` for local testing
- **Production/CI**: Don't create `.local.yaml`, uses remote repos from production config
- **Edge Cases**: Use `--force-remote` when you have `.local.yaml` but want to temporarily test remote behavior

### SSOT Provenance Schemas v1.1.0

Proper schema versioning for force-remote metadata tracking:

**New Schemas**:

- `schemas/crucible-go/content/ssot-provenance/v1.1.0/ssot-provenance.schema.json`
- `schemas/ssot/source-metadata.v1.1.0.json`

**New Fields**:

```json
{
  "sources": [
    {
      "name": "crucible",
      "forced_remote": true, // New: Was force-remote used?
      "forced_by": "flag", // New: How? "flag"|"env"|"config"
      "method": "git_clone",
      "commit": "abc123..."
    }
  ]
}
```

**Schema Versioning**:

- ✅ v1.0.0 schemas preserved unchanged
- ✅ v1.1.0 schemas include force-remote fields
- ✅ All code updated to reference v1.1.0
- ✅ Synced to embedded assets
- ✅ Tests updated and passing

**Audit Trail**: The `forced_remote` and `forced_by` fields enable CI enforcement and audit trails to track whether syncs used remote repos or local paths.

### SSOT Remote Repository Cloning

Automatic GitHub repository cloning using go-git enables production and CI workflows without requiring local checkouts.

**How It Works**:

```yaml
# .goneat/ssot-consumer.yaml
sources:
  - name: crucible
    repo: fulmenhq/crucible # Short-form GitHub reference
    ref: v0.2.8 # Branch, tag, or commit SHA
    sync_path_base: lang/go
```

**Cloning Process**:

1. Constructs GitHub URL: `https://github.com/fulmenhq/crucible.git`
2. Clones to cache: `~/.goneat/cache/ssot/<hash>` (deterministic SHA-256 hash of repo+ref)
3. Checks out specified ref (branch, tag, or commit SHA)
4. Syncs assets from `<clone>/sync_path_base`
5. Reuses cached clone on subsequent runs (fetches updates if needed)

**Supported Protocols**:

- ✅ **HTTPS** (public repositories): `https://github.com/org/repo.git`
- ✅ **File URLs** (local testing): `file:///path/to/repo.git`
- ⏳ **SSH** (future): Private repository authentication

**Cache Performance**:

- First run: Full clone (~5-30s depending on repo size)
- Subsequent runs: Reuse cache + fetch (~1-5s)
- Cache location: `~/.goneat/cache/ssot/`
- Safe to delete manually to force re-clone

**Example Workflow**:

```bash
# Fresh clone of goneat (no crucible checkout)
git clone https://github.com/fulmenhq/goneat.git
cd goneat

# Sync will clone crucible@v0.2.8 automatically
make sync-crucible
# → Clones to ~/.goneat/cache/ssot/abc123...
# → Syncs from cloned path

# Second run reuses cache
make sync-crucible
# → Reuses ~/.goneat/cache/ssot/abc123...
# → Much faster (~1-5s)
```

**Integration with Force-Remote**:

Works seamlessly with force-remote and auto-detection features. When `force_remote: true` is set or `--force-remote` flag is used, remote cloning is always used even if `.local.yaml` exists.

## Quality Improvements

### Dates Assessment Noise Reduction

Eliminated informational noise from dates assessment reports to focus on actual problems.

**Problem Solved**:

- Pre-commit hooks were reporting 96 "issues" that were just informational scan receipts
- Real problems (monotonic order violations, missing dates) were buried in noise
- Assessment reports showed "96 issues" when everything was actually correct

**Solution**:

- Removed info-level "Changelog scan: found N entries" per-file messages
- Replaced with debug logging (still visible with `--verbose` or `GONEAT_DEBUG=1`)
- Reports now show only actual problems requiring fixes

**Impact**:

```bash
# Before (on crucible sync with 145 staged files)
Assessment: 96 total issues
 - Dates: 96 issue(s) (est 1.6 hours)
   # All informational scan receipts, no actual problems

# After (same scenario)
Assessment: 0 total issues
 - Dates: ok (est 0 seconds)
   # Clean report when everything is correct

# With actual problem
Assessment: 2 total issues
 - Dates: 2 issue(s)
   CHANGELOG.md: Monotonic order violation: 2025-01-15 appears before 2025-03-28
   CHANGELOG.md: Missing date in entry [0.3.1]
```

**Still Detects**:

- ✅ Monotonic order violations (dates out of sequence)
- ✅ Missing dates in changelog entries
- ✅ Future dates beyond reasonable skew
- ✅ AI/human placeholders (YYYY-MM-DD, [DATE])
- ✅ Multiple "Unreleased" sections
- ✅ Stale entries (configurable threshold)

**DX Improvement**:

- "0 issues" actually means everything is good
- Pre-commit hooks show meaningful issue counts
- Assessment reports are actionable (only show things needing fixing)

## Installation

```bash
# Go install (recommended)
go install github.com/fulmenhq/goneat@v0.3.4

# Verify installation
goneat version
```

## Upgrade Notes

### For SSOT Users

**Review Auto-Detection Behavior**:

```bash
# Check if you have local overrides
ls -la .goneat/ssot-consumer.local.yaml

# If you DON'T have .local.yaml:
# ✅ No change - production config works as before

# If you DO have .local.yaml:
# ✅ Auto-detection continues working (signals local dev)

# If you have .local.yaml but want to test remote:
goneat ssot sync --force-remote
```

**Migration**: No configuration changes required. The DX improvement is backward compatible:

- **With `.local.yaml`**: Auto-detection works as before (local dev signal)
- **Without `.local.yaml`**: Auto-detection now properly disabled (production signal)

### For Tools Config Users

**Upgrade to v1.1.0 Schema**:

```yaml
# Update schema version
version: v1.1.0 # Was: v1.0.0

# Optionally migrate to declarative install format
tools:
  - name: your-tool
    # Old: install_commands: ["brew install your-tool"]
    # New:
    install:
      package_manager: brew
      package_name: your-tool
```

## Breaking Changes

**None**. All changes are backward compatible:

- **Tools Config**: v1.1.0 is optional, v1.0.0 continues to work
- **SSOT**: Auto-detection improvement is more correct, not breaking
- **Schemas**: v1.0.0 schemas preserved, v1.1.0 is additive

## Documentation

- **SSOT Guide**: Updated `docs/appnotes/lib/ssot.md` with:
  - Force-remote flag documentation
  - Auto-detection behavior section (v0.3.4+)
  - Configuration precedence with `.local.yaml` signal
  - TSFulmen use case example
  - Provenance field reference

## Testing

All tests passing:

```bash
# Schema validation tests
go test ./pkg/tools/... -v
# Provenance tests
go test ./pkg/ssot/... -v -run TestProvenance
# Auto-detection behavior tests
go test ./pkg/ssot/... -v -run TestAutoDetection
```

**Coverage**:

- ✅ Tools config v1.1.0 schema validation
- ✅ Package manager installation fixtures
- ✅ Force-remote flag behavior
- ✅ Auto-detection with/without `.local.yaml`
- ✅ Provenance schema v1.1.0 validation

## Known Issues

None at release time.

## What's Next (v0.3.5+)

Planned enhancements:

- **Tool Installation Execution**: Implement actual package manager installation (currently schema-only)
- **Multi-Language SSOT**: Support for TypeScript/Python SSOT patterns
- **Provenance Validation**: CI gates for enforcing clean provenance

## Links

- **Repository**: https://github.com/fulmenhq/goneat
- **CHANGELOG**: See [CHANGELOG.md](../../CHANGELOG.md) for detailed changes
- **Previous Release**: [v0.3.3](v0.3.3.md) - Cryptographic Release Signing

---

# Goneat v0.3.3 — Cryptographic Release Signing

**Release Date**: 2025-10-28
**Status**: Release

## TL;DR

- **Release Signing Infrastructure**: Mandatory PGP/GPG signing for all release artifacts
- **Supply Chain Security**: Cryptographic verification establishes artifact authenticity
- **Homebrew/Scoop Ready**: Signing infrastructure prerequisite for package manager distribution

## What's New

### Cryptographic Release Signing Infrastructure

Goneat v0.3.3 establishes the foundation for cryptographically signed releases, ensuring users can verify the authenticity and integrity of all release artifacts. This is a critical prerequisite for distribution through package managers like Homebrew and Scoop.

**Key Components**:

1. **FulmenHQ Release Signing Key**: Official PGP keypair with hardware-backed signing subkey
2. **Manual Signing Workflow**: Documented process for maintainers using YubiKey
3. **User Verification**: Complete instructions for verifying artifact signatures
4. **CI Prerequisites**: GitHub Actions workflow updated with GPG tooling

**Documentation**:

- **Security Guide**: `docs/security/release-signing.md` - Complete signing and verification guide
- **Release Checklist**: Updated with cryptographic signing steps
- **Key Management**: Documented custodianship, rotation, and revocation procedures

### For Users: Verifying Releases

Starting with v0.3.3, all release artifacts will be signed. Verify authenticity before installing:

```bash
# Download artifact and signature
curl -LO https://github.com/fulmenhq/goneat/releases/download/v0.3.3/goneat-darwin-arm64.tar.gz
curl -LO https://github.com/fulmenhq/goneat/releases/download/v0.3.3/goneat-darwin-arm64.tar.gz.asc

# Import FulmenHQ public key
curl -L https://github.com/fulmenhq/goneat/releases/download/v0.3.3/fulmenhq-release-signing-key.asc | gpg --import

# Verify signature
gpg --verify goneat-darwin-arm64.tar.gz.asc goneat-darwin-arm64.tar.gz
```

**Expected Output**:

```
gpg: Good signature from "FulmenHQ Release Signing <security@fulmenhq.dev>"
```

### For Maintainers: Signing Workflow

Release managers will sign artifacts using the manual workflow:

1. **Build and Package**: Standard release build
2. **Generate Checksums**: `sha256sum *.tar.gz *.zip > SHA256SUMS`
3. **Sign Artifacts**: Using YubiKey-backed subkey
4. **Verify Locally**: Test all signatures before upload
5. **Publish**: Upload binaries, signatures, and public key

See `docs/security/release-signing.md` for complete workflow.

### Infrastructure Updates

**GitHub Actions** (`.github/workflows/release.yml`):

- ✅ GPG tooling prerequisites installed (gnupg2)
- ✅ Workflow prepared for future signing automation
- ⏳ Actual signing deferred to post-v0.3.3 (manual workflow first)

**Security Documentation**:

- ✅ `docs/security/release-signing.md` - Comprehensive signing guide
- ✅ Key management and rotation procedures
- ✅ Verification instructions for users
- ✅ Troubleshooting and emergency procedures

## Roadmap: Signing Automation

**Phase 1 (v0.3.3)**: ✅ Manual signing + infrastructure

- Manual signing workflow with YubiKey
- CI tooling prerequisites installed
- Documentation complete

**Phase 2 (v0.3.4+)**: Automated CI signing

- Deploy CI signing subkey
- Secrets management (OIDC/Vault)
- Automated signature generation

**Phase 3 (v0.4.0+)**: Verification gates

- Automated signature verification in CI
- Pre-merge verification gates

**Phase 4 (v0.5.0+)**: Advanced provenance

- Sigstore integration
- SLSA provenance attestation

## Installation

```bash
# Go install (recommended)
go install github.com/fulmenhq/goneat@v0.3.3

# Verify installation
goneat version
```

## Upgrade Notes

No breaking changes. Simply upgrade to v0.3.3:

```bash
go install github.com/fulmenhq/goneat@v0.3.3
```

**Recommendation**: Verify signatures for all future downloads to ensure authenticity and protect against supply chain attacks.

## Security

For security concerns or to report key compromise:

- Email: security@fulmenhq.dev
- GitHub Security Advisories: https://github.com/fulmenhq/goneat/security

## Links

- **Repository**: https://github.com/fulmenhq/goneat
- **CHANGELOG**: See [CHANGELOG.md](CHANGELOG.md) for full details
- **Signing Documentation**: [docs/security/release-signing.md](docs/security/release-signing.md)
- **v0.3.2 Release**: See [docs/releases/v0.3.2.md](docs/releases/v0.3.2.md)

---

# Goneat v0.3.2 — Version Conflict Management

**Release Date**: 2025-10-28
**Status**: Release

## TL;DR

- **Version Conflict Detection**: New `goneat doctor versions` command to detect and manage multiple goneat installations
- **Automatic Conflict Resolution**: Purge stale global installations or update to latest with single command
- **Developer Experience**: Solves common version conflict issues when using multiple repositories

## What's New

### Version Conflict Detection and Management

Users working with multiple repositories may encounter version conflicts when goneat is installed both globally (`go install`) and locally (project bootstrap). This release introduces comprehensive version management capabilities to detect and resolve these conflicts.

**New Command**: `goneat doctor versions`

```bash
# Detect all goneat installations and identify conflicts
goneat doctor versions

# Remove stale global installation
goneat doctor versions --purge --yes

# Update global installation to latest
goneat doctor versions --update --yes

# JSON output for automation
goneat doctor versions --json
```

**Detection Coverage**:

- **Global installations**: GOPATH/bin (from `go install`)
- **Project-local**: ./bin/goneat (bootstrap pattern)
- **Development builds**: ./dist/goneat
- **PATH scanning**: All directories in system PATH

**What It Does**:

1. **Scans** your system for all goneat binaries
2. **Compares** versions and identifies the currently running binary
3. **Reports** version conflicts with clear visual indicators
4. **Recommends** solutions based on conflict type
5. **Resolves** conflicts automatically with `--purge` or `--update` flags

**Example Output**:

```
Goneat Version Analysis
=======================

Current running version: v0.3.2
Current binary path: /Users/you/project/dist/goneat

Detected installations:
   v0.2.11      | global | /Users/you/go/bin/goneat
▶️ v0.3.2       | development | /Users/you/project/dist/goneat

⚠️  Warning: 1 version conflict(s) detected

Recommendations:
1. Remove stale global installation:
   goneat doctor versions --purge --yes

2. Or update global installation to latest:
   goneat doctor versions --update --yes

3. Or use project-local installations (recommended):
   - Bootstrap to ./bin/goneat per project
   - See: goneat docs show user-guide/bootstrap
```

### Problem Solved

**Scenario**: Developer has multiple repositories on their machine:

- Repository A uses goneat v0.3.0 (bootstrapped to ./bin/goneat)
- Repository B just added goneat v0.3.2
- Developer previously ran `go install goneat@v0.2.11` (now in ~/go/bin)

**Before v0.3.2**:

- Commands might use the wrong version depending on PATH order
- Hooks might call stale global version
- Difficult to diagnose which version is running where
- Manual removal required understanding of GOPATH/bin location

**After v0.3.2**:

- Single command shows all installations and conflicts
- One-line fix to purge stale versions
- Clear recommendations for resolution strategy
- JSON output for CI/CD integration

### Use Cases

1. **Multi-Repository Development**: Detect when different repos use different versions
2. **Onboarding**: New team members can quickly identify and fix version mismatches
3. **CI/CD**: Validate no stale installations in build environments
4. **Troubleshooting**: Diagnose unexpected behavior due to version conflicts

### SSOT Dirty Detection Fix

Fixed a false positive bug in SSOT provenance dirty state detection that caused crucible repositories to incorrectly show as "dirty" when files matched only global gitignore patterns.

**The Bug**:

- go-git's `Status().IsClean()` includes ALL untracked files, even those matched by global gitignore (`~/.config/git/ignore`)
- This differs from git CLI behavior, which only checks repository `.gitignore`
- **Example**: `.claude/settings.local.json` in global gitignore but not repo `.gitignore` triggered false positive

**The Fix**:

- Now filters untracked files through repository `.gitignore` patterns only
- Repository `.gitignore` is the source of truth (matches CI/CD behavior)
- Includes `.git/info/exclude` for repository-local excludes
- Verified with 3-pass testing demonstrating correct behavior

**Impact**:

- ✅ **Team Consistency**: All developers see the same dirty state
- ✅ **CI/CD Alignment**: Local detection matches CI/CD behavior
- ✅ **Prepush Validation**: Correctly blocks only real uncommitted changes
- ✅ **Developer Experience**: Add common patterns (`.claude/`, `.vscode/`) to repo `.gitignore` for proper ignore behavior

**Design Decision**: See [ADR-0002](docs/architecture/decisions/adr-0002-ssot-dirty-detection.md) for detailed rationale on why repository `.gitignore` is the correct source of truth over global gitignore.

**Before Fix**:

```bash
$ cd crucible && git status
working tree clean  # Git CLI says clean

$ cd ../goneat && make sync-ssot
crucible: dirty (false positive from .claude/settings.local.json)
```

**After Fix**:

```bash
$ cd crucible && git status
working tree clean

$ cd ../goneat && make sync-ssot
crucible: clean ✅ (correctly ignores files in repo .gitignore)
```

## Installation

```bash
# Go install (recommended)
go install github.com/fulmenhq/goneat@v0.3.2

# From source
git clone https://github.com/fulmenhq/goneat.git
cd goneat
git checkout v0.3.2
make build
```

## Upgrade Notes

No configuration changes required. Upgrade to v0.3.2 and run `goneat doctor versions` to audit your installations:

```bash
# Upgrade
go install github.com/fulmenhq/goneat@v0.3.2

# Audit your installations
goneat doctor versions

# Clean up if conflicts detected
goneat doctor versions --purge --yes
```

**Recommended Practice**: After upgrading, run `goneat doctor versions` to ensure clean state across all your development environments.

---

# Goneat v0.3.1 — Build System Fix

**Release Date**: 2025-10-28
**Status**: Release

## TL;DR

- **Build System Fix**: Resolved chicken-and-egg dependency preventing fresh checkouts from building
- **Fast Follow**: Critical bug fix for v0.3.0 embed-assets workflow

## What's Fixed

### Build System Chicken-and-Egg Dependency

**Problem**: v0.3.0 introduced a circular dependency that prevented fresh repository checkouts from building:

1. `make build` requires `embed-assets` target to run first
2. `embed-assets.sh` script was trying to use `dist/goneat` binary
3. But `dist/goneat` doesn't exist until after build completes
4. Result: Fresh checkouts couldn't complete `make build` without manual intervention

**Solution**: Changed embed and verify scripts to use `go run .` instead of requiring the prebuilt binary:

- `scripts/embed-assets.sh`: Now uses `go run . content embed` instead of `dist/goneat content embed`
- `scripts/verify-embeds.sh`: Now uses `go run . content verify` instead of `dist/goneat content verify`
- Added explanatory notes in Makefile documenting the approach

**Impact**: Fresh checkouts can now run `make build` successfully without any manual steps.

## Installation

```bash
# Go install
go install github.com/fulmenhq/goneat@v0.3.1

# From source
git clone https://github.com/fulmenhq/goneat.git
cd goneat
git checkout v0.3.1
make build  # Now works on fresh checkouts!
```

## Upgrade Notes

No configuration changes required. Simply upgrade to v0.3.1 to get the build system fix:

```bash
go install github.com/fulmenhq/goneat@v0.3.1
```

---

# Goneat v0.3.0 — Dependency Protection (2025-10-28)

## TL;DR

- **Dependency Protection System**: License compliance, package cooling policy, and SBOM generation
- **Supply Chain Security**: Configurable package age thresholds prevent supply chain attacks
- **Assessment Integration**: Dependencies as first-class category in `goneat assess`
- **SSOT Provenance**: Automatic audit trail generation for SSOT sync operations

## Key Features

### Dependency Protection (`goneat dependencies`)

- Multi-language analyzer framework (Go production-ready)
- OPA policy engine for license compliance
- Network-aware execution with registry API integration
- Git hook integration

### Package Cooling Policy

- Mitigate supply chain attacks by enforcing minimum package age
- Registry integration: npm, PyPI, crates.io, NuGet, Go modules
- 24-hour caching layer for performance
- Exception management for trusted sources

### SBOM Generation

- CycloneDX 1.5 format via managed Syft integration
- Automatic tool installation with SHA256 verification
- Doctor integration: `goneat doctor tools --scope sbom --install`

### SSOT Provenance Metadata

- Git introspection: commit SHA, dirty state detection
- Version detection from VERSION file
- CI enforcement support for clean sources
- Configurable mirrors and output paths

## Installation

```bash
go install github.com/fulmenhq/goneat@v0.3.0
```

See [docs/releases/v0.3.0.md](v0.3.0.md) for comprehensive details.

---

# Previous Releases

# Goneat v0.2.11 — Guardian UX Enhancement & CI/CD Hardening (2025-09-30)

## TL;DR

- **Guardian Approval UX**: Fixed guardian approval browser page to display full command details with arguments
- **CI/CD Quality Gates**: Added embed verification to pre-push validation to prevent asset drift
- **Hook Enhancements**: All guardian-protected hooks now capture and pass command arguments for better visibility

## What's New

### Guardian Approval Command Visibility

Enhanced the guardian approval workflow to provide command transparency:

- **Full Command Display for Direct Usage**: When using `guardian approve` directly (e.g., `goneat guardian approve system ls -- ls -la /tmp`), the approval page shows the complete command with all arguments
- **Pre-push Hook**: Displays remote name and branch being pushed (e.g., `git push origin main`)
- **Git Hook Limitations**: Pre-commit and pre-reset hooks show generic placeholders (e.g., `git commit -m <pending commit message>`) because Git does not pass original command-line arguments to hook scripts
- **Command Details Section**: Collapsible section on approval page displays available command information with proper formatting
- **Best User Experience**: For full command visibility in git operations, wrap commands with `guardian approve` instead of relying on automatic hook triggers

### CI/CD Process Hardening

- **Embed Verification**: Added `make verify-embeds` to pre-push quality gates
- **Asset Drift Prevention**: Ensures embedded templates, schemas, and config stay synchronized with source
- **Release Validation**: Strengthens release process with automated embed consistency checks

## Installation

```bash
# Go install
go install github.com/fulmenhq/goneat@latest

# From source
git clone https://github.com/fulmenhq/goneat.git
cd goneat
make build
```

## Upgrade Notes

After upgrading, regenerate your hooks to get the enhanced guardian command visibility:

```bash
goneat hooks generate --with-guardian
goneat hooks install
```

---

# Goneat v0.2.8 — Guardian Repository Protection & Format Intelligence (2025-09-27)

## TL;DR

- **goneat pathfinder**: Expanded schema discovery system with simplified FinderFacade API for enterprise-grade path discovery
- **goneat format**: Added built-in XML and JSON prettification with configurable indentation and size warnings
- **goneat content**: Enhanced embedding system supporting schemas, templates, and configuration files
- **goneat hooks**: Added pre-reset hook support with guardian protection for reset operations
- **ASCII Terminal Support**: New ASCII art calibration system for accurate boxes across multiple terminal types
- **50% Test Coverage**: Comprehensive test coverage expansion with automated testing infrastructure

## What's New

### Pathfinder Schema Discovery (`goneat pathfinder`)

- **Expanded Schema Detection Engine**: Intelligent pattern matching for 10+ schema formats with enhanced discovery capabilities
- **FinderFacade API**: High-level entry point for enterprise-grade path discovery workflows with simplified interface
- **Schema Validation**: Comprehensive validation with meta-schema compliance checking and structured error reporting
- **Local Loader**: Production-ready filesystem loader with streaming text output and transform support

### Format Command Enhancements (`goneat format`)

- **JSON Prettification**: Built-in JSON formatting using Go's `json.Indent` with configurable options
  - New flags: `--json-indent` (custom string), `--json-indent-count` (1-10 spaces, 0 to skip), `--json-size-warning` (MB threshold)
  - Replaces external `jq` dependency for reliable, cross-platform JSON formatting
  - Supports compact mode and size-based warnings for large files

- **XML Prettification**: Built-in XML formatting using `etree` library with configurable options
  - New flags: `--xml-indent` (custom string), `--xml-indent-count` (1-10 spaces, 0 to skip), `--xml-size-warning` (MB threshold)
  - Validates XML well-formedness before formatting
  - Supports size-based warnings for large files

### Content Management (`goneat content`)

- **Enhanced Embedding System**: Support for embedding schemas, templates, and configuration files beyond just documentation
- **Asset Synchronization**: Better SSOT (Single Source of Truth) management for all embedded assets
- **Build Optimization**: Streamlined asset embedding process with verification and sync steps

### Git Hooks (`goneat hooks`)

- **Pre-reset Hook Support**: New `pre-reset` hook with guardian protection for reset operations
- **Guardian Integration**: Enhanced hook templates with automated guardian policy installation
- **Template Corrections**: Fixed trailing newline issues in embedded hook templates

### ASCII Terminal Calibration (`goneat ascii` command & `pkg/ascii` library)

- **Terminal Catalog System**: Comprehensive ASCII art calibration for accurate boxes across multiple terminal types
- **Display Functions**: Enhanced terminal display capabilities with proper box drawing characters
- **Cross-Platform Support**: Terminal-aware rendering for consistent visual output
- **ASCII Command**: New `goneat ascii` command for terminal calibration and display testing

### Testing Infrastructure

- **50% Test Coverage Achievement**: Comprehensive test expansion across packages
  - `pkg/format/finalizer`: 72.5% coverage with normalization utilities
  - `pkg/ascii`: 31.9% coverage with terminal display tests
  - `cmd` package: 40.7% coverage with guardian compatibility

- **Automated Testing**:
  - `GONEAT_GUARDIAN_AUTO_DENY` environment variable for CI/CD
  - Enhanced test fixtures and helper utilities
  - Guardian approval testing with automated denial mechanisms

## Bug Fixes

- **Guardian Approve Bug**: Fixed `runGuardianApprove` to always execute wrapped commands after policy checks
- **Guardian Error Messages**: Enhanced denial error handling with clear messages and proper exit codes
- **Code Quality**: Resolved golangci-lint ST1015 switch statement issues
- **Security Suppressions**: Added proper `#nosec` comments for controlled file access patterns
- **Template Formatting**: Corrected trailing newline EOF enforcement in hook templates

## Installation

See v0.2.11 installation instructions above.
