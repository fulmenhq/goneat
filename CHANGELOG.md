# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

### Changed

### Fixed

## [0.3.9] - 2025-12-01

### Added

- **prettier in Foundation Scope**: Added prettier to foundation tools scope in `.goneat/tools.yaml`
  - Resolves CI failures in downstream repos (groningen) where prettier was required but not installed
  - Foundation tools now include: ripgrep, jq, yq, go, go-licenses, golangci-lint, yamlfmt, prettier
  - CI bootstrap (`make bootstrap`) automatically installs all foundation tools including prettier

- **bun Package Manager Installer Support**: Implemented missing bun installer in `defaultInstallerCommand()`
  - Command: `bun add -g <package>` for global installation
  - Installs to `~/.bun/bin/` (user-level, no sudo required)
  - Completes v0.3.7 package manager support (bun was defined but not implemented)
  - Enables bun as primary installer for Node.js tools on all platforms

- **bun Auto-Install in CI**: Package manager bootstrap now auto-installs bun when needed
  - New `InstallBun()` function in `pkg/tools/installer_bun.go` handles cross-platform bun installation
  - `autoInstallPackageManagers()` tries bun first (simpler, fewer dependencies, priority 1) before brew
  - After installation, bun's bin directory (`~/.bun/bin`) is immediately added to PATH for current session
  - Enables CI runners without any pre-installed package managers to bootstrap foundation tools
  - Works alongside existing brew auto-install (v0.3.7) as a fallback

- **Enhanced Package Manager Auto-Install Logging**: Added debug logging to trace package manager bootstrap
  - Logs which package managers are needed and whether they're safe to auto-install
  - Logs detection results for existing installations
  - Logs PATH additions after successful installations
  - Helps diagnose CI bootstrap failures

- **Automated Release Upload**: New `make release-upload` target for complete GitHub release uploads
  - Uploads binaries (*.tar.gz, *.zip), SHA256SUMS, all signatures (.asc), and public key
  - Updates release notes automatically
  - Safety checks verify signatures exist before upload
  - Includes verification helper showing command to confirm upload succeeded

### Changed

- **Installer Priority Updates (User-Level, Well-Behaved)**:
  - **prettier**: Now uses brew (darwin/linux), scoop/winget/bun (windows) instead of bun/npm globally
  - **yamlfmt**: Now uses brew (darwin/linux), scoop (windows), go-install fallback instead of go-install only
  - All installers respect system package managers over language-specific tools
  - Follows v0.3.7 user-level pattern (no sudo required)
  - Users can pin versions via system package managers (brew pin, scoop hold)

- **Config Loading Enhancement**: Updated `cmd/doctor.go` to use `LoadToolsConfig()` for upward directory search
  - Enables goneat to find `.goneat/tools.yaml` when run from subdirectories
  - Searches up directory tree to repository root
  - Provides helpful error message if config not found (suggests `goneat doctor tools init`)

### Fixed

- **Release Checklist Documentation Gap**: RELEASE_CHECKLIST.md now explicitly documents uploading binaries AND signatures
  - Previous version only mentioned `.asc` signature files, missing actual binaries and SHA256SUMS
  - Caused v0.3.8 release issue where CI-built binaries remained instead of locally-signed builds
  - Added explicit warning: "Upload BOTH binaries and signatures, not just signatures!"
  - Provides two complete options: automated (`make release-upload`) or manual with full command list

### Security

- **Guardian URL Auto-Approval Bypass Fixed**: Closed security gap where URL-based remotes bypassed approval
  - **Issue**: When `--remote` was a URL (e.g., `https://github.com/user/repo.git`) instead of a name (`origin`), guardian auto-approved the operation because the URL didn't match configured patterns like `["origin", "upstream"]`
  - **Impact**: Push operations to protected branches could bypass guardian approval when git resolved remotes to URLs
  - **Fix**: New `looksLikeURL()` function detects URL-based remotes and applies fail-closed security
  - URL remotes now require approval regardless of pattern matching (cannot verify trust via URL)
  - Detects: `https://`, `http://`, `ssh://`, `git://`, `git@host:path`, and common hosting domains
  - Added comprehensive test coverage for URL detection and approval behavior
  - Thanks to Python Forge Engineer (@python-forge-engineer) for the detailed bug report

## [0.3.8] - 2025-12-01

### Added

- **Hooks Schema Validation Helper**: New `validateHooksManifestSchema()` function validates hooks.yaml against embedded JSON schema before Go struct parsing
  - Provides clear, actionable error messages instead of raw Go type errors
  - Shows specific validation failures with field paths (e.g., `hooks.pre-commit: Invalid type. Expected: array, given: object`)
  - Includes expected format example and remediation guidance (`Run 'goneat hooks init'`)
  - Gracefully skips validation if embedded schema not found (shouldn't happen in production)

### Changed

- **Hooks Generate Validation Flow**: `goneat hooks generate` now validates hooks.yaml against schema before attempting struct unmarshal
  - Catches configuration errors at schema level where error messages are descriptive
  - Users see clear validation errors instead of cryptic Go unmarshal type errors
  - Example improvement: `hooks.pre-commit: Invalid type. Expected: array` instead of `cannot unmarshal !!map into []struct { Command string...`

### Fixed

- **Embedded Schema Path**: Corrected hooks manifest schema lookup path from `schemas/work/hooks-manifest-v1.0.0.yaml` to `work/hooks-manifest-v1.0.0.yaml`
  - Path is relative to embedded schemas filesystem root (after `fs.Sub` extracts `embedded_schemas`)
  - Affects both new `validateHooksManifestSchema()` helper and existing `runHooksPolicyValidate` command
  - Schema validation now works correctly in both `hooks generate` and `hooks policy-validate`

### Security

- **Hardened File and Directory Permissions**: Tightened permissions to follow principle of least privilege
  - Directory creation: Changed from `0755` to `0750` (pkg/tools/installer_brew_local.go:26, cmd/doctor_tools_init.go:113)
  - File creation: Changed from `0644` to `0600` for tool configuration files (cmd/doctor_tools_init.go:171)
  - GitHub Actions `$GITHUB_PATH` file: Retained `0644` per GitHub Actions standard with documented justification (cmd/doctor.go:435)

- **Documented Security Suppressions**: Added comprehensive `#nosec` comments with detailed justifications for safe operations
  - **G204 (Command Injection)**: Documented safe subprocess execution where commands come from validated/trusted sources
    - `pkg/tools/installer_brew_local.go:66`: brewPath constructed from sanitized user home directory
    - `internal/doctor/package_managers.go:177`: Detection commands from embedded trusted configuration
  - **G304 (File Inclusion)**: Documented safe file operations where paths are validated or system-controlled
    - `internal/doctor/tools_config.go:41`: Config path from safe upward directory traversal
    - `cmd/doctor_tools_init.go:182`: Validation of just-created config file
    - `cmd/doctor.go:438`: GitHub Actions managed `$GITHUB_PATH` environment variable
  - **G104 (Unhandled Errors)**: Documented intentional best-effort cleanup in error paths (pkg/tools/installer_brew_local.go:52)

- **Security Assessment**: All 10 gosec findings resolved, achieving 100% security health rating

## [0.3.7] - 2025-11-20

### Added

- **Public Key Verification Script**: Automated cryptographic safety checks for release signing workflow
  - Three-layer verification: negative check (no PRIVATE KEY blocks), positive check (PUBLIC KEY blocks present), GPG verification
  - Defense-in-depth to prevent accidental private key disclosure during releases
  - Clear error messages with visual warnings if private key detected
  - Script: `scripts/verify-public-key.sh` with exit code 0 (safe) or 1 (danger)
  - RELEASE_CHECKLIST.md updated with automated verification workflow
  - Documentation encourages inspection of script before first use

- **Externalized Common Tools Repository Mappings**: Tool→GitHub repo mappings now configuration-driven
  - Configuration: `config/tools/common-tools-repos.yaml` with 14 tool mappings
  - Schema validation: `schemas/tools/common-tools-repos.v1.0.0.json`
  - Alphabetically sorted within categories (Security/SBOM, Go Tools, General CLI)
  - Package-level caching to avoid reparsing YAML on every cooling check
  - Backward-compatible hardcoded fallback if config loading fails
  - Tools included: cosign, gitleaks, golangci-lint, gosec, grype, jq, prettier, ripgrep, shellcheck, shfmt, syft, trivy, yamlfmt, yq
  - No code changes required to add new tool mappings (edit YAML only)

- **Foundation Tools Externalization** (Phases 1-5A): Tool configuration moved from hardcoded to explicit SSOT
  - **Configuration Files** (Phase 1): Created config/tools/foundation-tools-defaults.yaml and foundation-package-managers.yaml
    - Package manager safety matrix: bun, mise, scoop, winget with sudo requirements
    - Tool definitions with package manager preferences
    - Schema validation for both configurations
  - **Repo Type Detection** (Phase 2): Language-aware tool selection
    - Detects Go, Python, TypeScript, Rust, C# repositories
    - Uses Crucible language taxonomy for consistent detection
    - Filters tools based on detected repository type
  - **Package Manager Commands** (Phase 3): Package manager discovery and status
    - Detection for 8 package managers across all platforms
    - Version parsing and sudo requirement detection
    - `goneat doctor package-managers` command with JSON output
    - Installation instructions (manual in v0.3.7)
  - **Tools Init Command** (Phase 4): Explicit tool configuration seeding
    - `goneat doctor tools init` generates .goneat/tools.yaml
    - Auto-detects repository type and filters appropriate tools
    - `--minimal` flag for CI-safe, language-native tools only
    - `--scope` flag for foundation, security, format, all
    - `--force` flag to overwrite existing configuration
    - Validates generated configuration via schema
  - **PATH Management** (Phase 5): Automatic shim directory detection
    - PATH extension in PersistentPreRun (all goneat commands benefit)
    - GitHub Actions integration: automatic `$GITHUB_PATH` updates with `--install`
    - Shell activation helper: `goneat doctor tools env --activate`
    - Shim detection for mise, bun, scoop, go-install
    - Platform support: Tested on macOS; Linux/Windows implementations present
    - Documentation: `docs/guides/goneat-tools-cicd-runner-support.md`
  - **Hardcoded Tools Removal** (Phase 5A): Complete elimination of hidden defaults
    - Removed `KnownInfrastructureTools()`, `KnownSecurityTools()`, `KnownFormatTools()`, `KnownAllTools()` functions
    - Removed `ensureCoreScopes()` function that forced foundation tools back in
    - Removed all runtime merging of hardcoded tool definitions
    - Deprecated `internal/doctor/tools-defaults.yaml` (replaced by foundation-tools-defaults.yaml)
    - All tests passing with no hardcoded tool dependencies

- **Package Manager Research Documentation**: Arch Eagle research on CI runner brew support
  - Document: `docs/reference/tools-packaging-ci-runner-reference.md`
  - Analysis of brew sudo requirements on CI runners (GitHub Actions, GitLab CI, etc.)
  - Recommendations for sudo-free package managers in CI environments
  - Alternative approaches: mise, bun, scoop, language-native package managers

- **User-Local Homebrew Auto-Install**: Safe bootstrap path for hosted runners
  - `goneat doctor tools --install` now prefers `~/homebrew` when Homebrew must be installed, avoiding `/usr/local` sudo flows
  - Automatic detection and dogfooding ensure the repo itself validates the user-local install path
  - Documented alongside the runner guidance in `docs/guides/goneat-tools-cicd-runner-support.md`

### Changed

- **Tools Configuration Now Required**: Breaking change - `.goneat/tools.yaml` is now mandatory
  - **Before v0.3.7**: Hidden defaults merged at runtime, unconditional foundation tools
  - **After v0.3.7**: Explicit configuration required, no hidden defaults
  - **Migration**: Run `goneat doctor tools init` to generate configuration
  - **CI/CD Impact**: Pipelines must initialize tools config before using `goneat doctor tools`
  - Error message guides users to init command if file missing

- **STDOUT Hygiene Enforcement**: Fixed output pollution breaking structured data consumers
  - Changed `logger.Info` to `logger.Debug` in path_manager.go (line 142)
  - **Issue**: Logger output was polluting STDOUT, breaking JSON consumers and tests
  - **Impact**: TestSecurity_Gitleaks_Smoke test was failing due to stdout pollution
  - **Fix**: Debug messages now go to STDERR via logger.Debug
  - Aligns with Go coding standards: STDOUT for data, STDERR for logs

### Fixed

- **STDOUT Pollution in PATH Manager**: Fixed logger.Info calls breaking structured output
  - Test failure: `TestSecurity_Gitleaks_Smoke` was failing due to unexpected STDOUT
  - Root cause: path_manager.go line 142 using logger.Info for debug messages
  - Solution: Changed to logger.Debug to preserve STDOUT cleanliness
  - Impact: All security tests now passing, JSON output clean

- **Go Version Detection Accuracy**: `doctor tools` now parses `go version` output lexically to handle prefixes like `go1.23.4`
  - Prevents false negatives when enforcing version policies against the Go toolchain
  - Ensures assessment output and doctor reports show the correct Go version on all supported platforms

### Breaking Changes & Migration Guide

#### What Changed

**Old Behavior** (v0.3.6 and earlier):
- Foundation tools (go, ripgrep, jq, yq) were **hardcoded** in three locations
- `goneat doctor tools` **automatically included** these tools without user control
- **Hidden runtime merging** of defaults even if user tried to remove them
- **No way to opt-out** for CI/CD environments
- Tools installed via brew (**requires sudo** on both macOS and Linux now)

**New Behavior** (v0.3.7+):
- **Explicit configuration required**: Must run `goneat doctor tools init` first
- **Single source of truth**: Only `.goneat/tools.yaml` is used (no hidden defaults)
- **Full user control**: Edit, customize, or minimize tool list as needed
- **CI-safe options**: `--minimal` flag generates sudo-free, language-native tools only
- **Sudo-free package managers**: Prefer bun, mise, scoop/winget over brew

#### Migration Steps

**For CI/CD Pipelines** (Critical - prevents sudo errors):

```bash
# One-time setup (commit .goneat/tools.yaml to repo)
goneat doctor tools init --minimal  # CI-safe, no sudo required
git add .goneat/tools.yaml
git commit -m "chore: add goneat tools config (v0.3.7)"

# CI workflow (uses committed config)
goneat doctor tools --install --yes
# ✅ Now works without sudo - only uses go-install
```

**For Local Development**:

```bash
# One-time setup per repository
goneat doctor tools init                # Auto-detects repo type, generates full config
git add .goneat/tools.yaml
git commit -m "chore: configure goneat tools"

# Day-to-day usage (unchanged)
goneat doctor tools --install           # Install missing tools
```

**For Existing Repos with Custom Tools**:

```bash
# Initialize with auto-detection
goneat doctor tools init --force        # Overwrites existing config

# Or manually add to existing .goneat/tools.yaml
# See docs/appnotes/tools-runner-usage.md for examples
```

#### Affected Commands

- `goneat doctor tools` - Now **requires** `.goneat/tools.yaml` (error if missing)
- `goneat doctor tools --scope foundation` - Uses config, not hardcoded defaults
- `make bootstrap` - Must initialize tools config first (update Makefiles)

#### Error Messages

If `.goneat/tools.yaml` doesn't exist:

```
❌ Error: .goneat/tools.yaml not found

This file defines which tools goneat should manage for this repository.

Initialize with:
  goneat doctor tools init           # Recommended defaults for your repo type
  goneat doctor tools init --minimal # CI-safe (only language-native tools)

For more info: goneat doctor tools init --help
```

#### Recommended Actions

1. **CI/CD Pipelines**: Update to run `goneat doctor tools init --minimal` before bootstrap
2. **Local Development**: Run `goneat doctor tools init` and commit the generated config
3. **Makefiles**: Add `bootstrap-init` target for one-time setup (see planning doc for examples)
4. **Documentation**: Update README/setup guides with init command requirement

### See Also

- **Migration Guide**: `docs/appnotes/tools-runner-usage.md` (updated for v0.3.7)
- **Package Managers**: `docs/guides/package-managers.md` (new in v0.3.7)
- **Planning Document**: `.plans/active/v0.3.7/foundation-tools-package-managers.md`
- **CI Runner Research**: `docs/reference/tools-packaging-ci-runner-reference.md`


---

**Note**: Older releases (0.3.6 and earlier) have been archived. See git history for details.
