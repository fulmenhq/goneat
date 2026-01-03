# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [v0.4.1] - 2026-01-02

### Added

- **Explicit incremental lint checking**: New `--new-issues-only` and `--new-issues-base` flags for `goneat assess`
  - `--new-issues-only`: Only report issues introduced since a baseline git reference (opt-in)
  - `--new-issues-base`: Git reference for baseline comparison (default: `HEAD~`)
  - Supports golangci-lint (`--new-from-rev`) and biome (`--changed --since`)
  - Documentation: `docs/appnotes/assess/incremental-lint-checking.md`

### Changed

- **Hook mode behavior**: Removed implicit incremental lint checking from hook mode
  - **Before v0.4.1**: Hook mode implicitly applied `--lint-new-from-rev HEAD~`
  - **After v0.4.1**: Hook mode reports ALL lint issues by default (consistent with direct assess)
  - To restore previous behavior, add `--new-issues-only` to hooks.yaml args
- **Patch dependency updates**: go-git v5.16.4, cobra v1.10.2, go-runewidth v0.0.19
- **Indirect dependency update**: added `github.com/clipperhouse/uax29/v2` via go-runewidth

## [v0.4.0] - 2025-12-31

### Added

- **Python lint/format**: Language-aware assessment via [ruff](https://docs.astral.sh/ruff/) for Python files
- **JavaScript/TypeScript lint/format**: Language-aware assessment via [biome](https://biomejs.dev/) for JS/TS files
- **Tool-present gating**: Gracefully skip tools that aren't installed (no errors, informational logs)
- **Language support table**: README.md now documents supported languages with install commands

### Changed

- **Agentic attribution v2**: Migrated from named agents (Forge Neat, Code Scout) to role-based attribution (devlead, secrev, releng) per [3leaps crucible](https://crucible.3leaps.dev/) standards
- **AGENTS.md**: Simplified to role-based operating model (568 → 199 lines)
- **Session protocol**: Updated for role-based workflow

### Fixed

- **Dates tests**: Fixed temporal stability by using far-future test dates (2099-12-31)

## [v0.3.25] - 2025-12-27

### Fixed

- **Checkmake discovery**: `assess --categories lint` now reliably finds and lints root-level `Makefile` targets (no more silent skip)
- **Release upload homedir**: `make release-upload` now honors `GONEAT_GPG_HOMEDIR` (legacy `GPG_HOMEDIR` still supported)
- **Cmd test isolation**: reset validate globals to prevent cross-test state bleed

## [v0.3.24] - 2025-12-23

### Added

- **Offline canonical ID lookup**: `validate data` and `validate suite` can resolve canonical URL `schema_id` values from local `--ref-dir` trees (no-network CI)
- **Schema resolution mode**: `--schema-resolution prefer-id|id-strict|path-only`
- **Offline $id index**: internal registry for collision-safe `$id` → schema bytes/path indexing

### Changed

- **Validate docs**: documented dual-run CI strategy (offline strict pre-deploy + post-deploy spec-host probe)
- **Crucible SSOT**: synced to Crucible v0.2.27

### Fixed

- **Format dogfooding**: `goneat format <explicit-file>` force-includes targets even if ignored by `.goneatignore`

## [v0.3.23] - 2025-12-21

### Added

- **Validate suite (bulk)**: `goneat validate suite` validates many YAML/JSON files in one run with parallel workers and JSON output
- **Schema mapping shorthand**: `schema_path` in `.goneat/schema-mappings.yaml` to map directly to local schema files
- **Embedded release docs**: `goneat docs show release-notes` and `goneat docs show releases/latest`

### Fixed

- **Offline ref-dir duplicates**: `validate data --ref-dir` no longer fails when the root schema is also present in the ref-dir tree
- **Validate suite local schema path resolution**: local `schema_id` mappings now honor `overrides.path` and include resolved `schema.path` in JSON output

### Changed

- **Validate docs**: expanded `validate` docs with bulk validation and local schema examples

## [v0.3.22] - 2025-12-20

### Added

- **Assess config scaffolding**: `goneat doctor assess init` can generate a starter `.goneat/assess.yaml` based on repo type
- **Hooks UX**: `goneat hooks validate` and `goneat hooks inspect` now show the effective hook wrapper invocation and classify internal vs external commands
- **Hooks JSON output**: `goneat hooks validate --format json` and `goneat hooks inspect --format json`
- **Offline $ref resolution**: `goneat validate data --ref-dir` can preload local schemas to resolve remote `$ref` URLs without a live schema registry

### Fixed

- **Bash hook glob expansion**: generated bash hooks now include `set -f` to prevent unquoted glob patterns (e.g., `.cache/**`) from exploding into many args

### Changed

- **Embedded hooks template layout**: standardized on canonical embedded templates under `embedded_templates/templates/hooks/...` (legacy embedded path removed)

## [v0.3.21] - 2025-12-15

### Added

- **Cooling Policy Metadata**: `doctor tools` can now resolve upstream metadata for more tool install types
  - Infer GitHub repo for `kind: go` tools from `install_package` (e.g. actionlint)
  - Support PyPI metadata (`pypi:<package>`) for Python-installed tools (e.g. yamllint)
- **shfmt Style Args Override**: Shell lint can now pass repo-specific `shfmt` style flags via `.goneat/assess.yaml` (`lint.shell.shfmt.args`)

### Fixed

- **Go 1.25 Dependencies Regression**: `goneat dependencies` no longer fails on stdlib "module info" errors from go-licenses
  - Cooling/policy evaluation uses `go list -deps -json` module discovery (stdlib skipped)
  - go-licenses runs only when `--licenses` is requested; falls back to module-dir license file presence when degraded
  - Ensures JSON output matches schema (arrays never serialize as null)

### Changed

- **Lint Debt Cleanup**: Resolved Makefile checkmake backlog without raising thresholds
  - Set repo `lint.make.checkmake.config.max_body_length: 15`
  - Refactored large Makefile targets into helper targets

## [v0.3.20] - 2025-12-14

### Added

- **NOTICE for Distribution**: Added a top-level `NOTICE` file and include `LICENSE`/`NOTICE` in packaged release archives
- **ADR**: Documented the Linux `CGO_ENABLED=0` (musl/glibc compatible) release artifact policy
- **Docs**: Documented checkmake rule override keys for `.goneat/assess.yaml`

## [v0.3.19] - 2025-12-14

### Fixed

- **Linux Release Compatibility (musl/glibc)**: Build Linux release artifacts with `CGO_ENABLED=0` to avoid glibc-only linkage
  - Prevents `invalid cross-device link`-style libc relocation failures in Alpine/musl containers
  - Adds a release workflow smoke test running the linux binary in both Alpine (musl) and Debian (glibc)

## [v0.3.18] - 2025-12-13

### Added

- **Checkmake Config Overrides (MVP)**: `.goneat/assess.yaml` can now generate a checkmake config for rules that support it
  - `lint.make.checkmake.config.max_body_length`
  - `lint.make.checkmake.config.min_phony_targets`

## [v0.3.17] - 2025-12-13

### Added

- **Unified Ignore Scope for Lint Sidecars**: Shell/Makefile/GitHub Actions/YAML lint runners now respect `.gitignore` + `.goneatignore` by default
  - Applies to shfmt, shellcheck, actionlint, checkmake, and yamllint target resolution
  - `--force-include` can re-include ignored paths for targeted runs
  - Reduces DRY duplication of ignores between `.gitignore` and `.goneat/assess.yaml`

### Changed

- **Ignore Matcher APIs**: Added repo-root-relative matching helpers to make ignore behavior deterministic across runners
  - Supports `!pattern` negation in `.goneatignore` consistently for sidecar tooling

## [v0.3.16] - 2025-12-12

### Fixed

- **CRITICAL: Release Build Version Embedding**: Fixed ldflags in `scripts/build-all.sh` targeting non-existent variable `main.Version` instead of `github.com/fulmenhq/goneat/pkg/buildinfo.BinaryVersion`
  - Root cause: Release binaries reported `goneat dev` instead of actual version (e.g., `goneat v0.3.16`)
  - All three buildinfo variables now correctly embedded: `BinaryVersion`, `BuildTime`, `GitCommit`
  - Aligns release builds with Makefile patterns

- **Release Signature Invalidation**: Added safeguards preventing checksum regeneration after signing
  - Guard in `release-checksums` target blocks regeneration if `.asc` or `.minisig` files exist
  - New `release-verify-checksums` target for non-destructive checksum validation
  - Updated RELEASE_CHECKLIST.md with one-way sequence warning and recovery procedures

### Added

- **VERSION/Tag Validation**: Release workflow now fails fast if VERSION file doesn't match git tag
  - Prevents publishing releases where VERSION and tag diverge
  - Clear error message guides maintainers to fix before tagging

- **Bootstrap Documentation**: New and updated bootstrap guides
  - `docs/user-guide/bootstrap/sfetch.md` - Secure fetch bootstrap guide
  - Updated `docs/user-guide/install.md` with streamlined instructions
  - Enhanced `docs/appnotes/bootstrap-patterns.md` with current patterns

- **Release Scripts**: New scripts for release management
  - `scripts/upload-release-assets.sh` - Automated release asset upload

### Changed

- **Makefile Cleanup**: Simplified release-related targets and removed redundant code
- **Embedded Docs Sync**: Bootstrap and install documentation synced to embedded assets

## [v0.3.15] - 2025-12-11

### Added

- **Expanded Lint Coverage**: Added comprehensive linting for shell scripts (shfmt/shellcheck), Makefiles (checkmake), and GitHub Actions workflows (actionlint)
- **Hook Manifest Execution**: `goneat assess --hook` now executes all commands defined in hooks.yaml, not just assess commands
- **Yamllint Integration**: Added yamllint support with configurable paths and strict mode for YAML files
- **Tool Defaults**: Added shfmt, actionlint, and checkmake to foundation tool defaults for local installation

### Changed

- **Hook Behavior**: Hook manifests now execute external commands (make, etc.) in priority order with timeout enforcement
- **Lint Assessment**: Enhanced lint runner to handle multiple tool types with graceful skipping when tools are unavailable

### Fixed

- **assess command**: Prevent creation of poorly named output files when format names are used as filenames (e.g., `--output json` now shows helpful error suggesting `--format json`)
- **Shell Scripts**: Resolved all syntax errors in shell scripts (malformed content, missing loop closures, redirection issues)
- **Shell Lint**: Fixed shfmt diff name handling and improved fixture exclusion logic
- **CI Cache**: Removed redundant Go cache step to prevent CI collisions

## [0.3.14] - 2025-12-08

### Added

- **Two-Path CI Validation**: Restructured CI to validate both container and package manager approaches
  - `container-probe`: Proves goneat works inside goneat-tools container (LOW friction, validates first)
  - `bootstrap-probe`: Proves `goneat doctor tools --install` works on fresh runners (HIGHER friction)
  - Dependency ordering: bootstrap-probe only runs if container-probe passes
  - Gives users confidence: "If goneat CI passes, my container-based CI will work"

- **Container-Based CI (Recommended)**: `container-probe` job uses `ghcr.io/fulmenhq/goneat-tools:latest`
  - Downloads goneat binary from build job, runs `goneat doctor tools --scope foundation` inside container
  - Proves goneat integration works, not just that tools exist (`--version`)
  - Tools available in container: prettier, yamlfmt, jq, yq, rg, git, bash
  - HIGH confidence approach: Container IS the contract

- **ToolExecutor Package (Phase 1)**: New `pkg/tools/executor*.go` infrastructure for future Docker-based tool execution
  - `executor.go` - Interface and factory pattern
  - `executor_local.go` - Local tool execution (existing behavior)
  - `executor_docker.go` - Docker-based execution via goneat-tools container
  - `executor_auto.go` - Smart selection (CI → docker, local → local tools)
  - `executor_test.go` - Comprehensive test coverage
  - Note: Phase 1 only - executor created but NOT yet integrated into cmd/format.go

- **Local CI Runner Support**: New Makefile targets for running CI locally with containers
  - `make local-ci-format` - Run format-check job locally using container
  - `make local-ci-all` - Run all CI jobs locally
  - Documentation: `docs/cicd/local-runner.md`

- **CI/CD Configuration**: New config directory structure for CI/CD patterns
  - `config/cicd/` - CI/CD configuration templates
  - `docs/cicd/` - CI/CD documentation

### Changed

- **CI Workflow Restructured**: Three jobs with explicit dependency ordering
  - `build-test-lint`: Go build/test/lint, uploads goneat binary as artifact
  - `container-probe`: Validates container path (depends on build)
  - `bootstrap-probe`: Validates package manager path (depends on container-probe)
  - Strategic: Low-friction path validates first, don't waste cycles if container fails

- **golangci-lint Updated to v2**: Updated install path and minimum version
  - Old: `github.com/golangci/golangci-lint/cmd/golangci-lint@latest` (v1.x)
  - New: `github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest` (v2.x)
  - Minimum version updated from 1.54.0 to 2.0.0

### Fixed

- **Prettier/Brew CI Failures**: Root cause was OpenSSL@3 test suite failing in containers
  - Strategic decision: Use goneat-tools container instead of installing tools in CI
  - Documentation: `.plans/runjournals/cicd-runners/tools/prettier/`

## [0.3.13] - 2025-12-04

### Added

- **Dynamic yamlfmt Indent Detection** (`cmd/doctor_tools_init.go`):
  - `goneat doctor tools init` now reads `.yamlfmt` config to determine indent value
  - Walks up directory tree to find `.yamlfmt` (supports monorepo structures)
  - Security: Rejects malicious/corrupt indent values outside 1-8 range
  - Falls back to 2-space indent if no `.yamlfmt` found or value invalid
  - Enterprise-friendly: Respects organization yamlfmt configurations
  - New tests: `TestDetectYamlfmtIndent*`, `TestDoctorToolsInitUsesYamlfmtIndent`

### Changed

- **Crucible SSOT Updated to v0.2.22**:
  - Updated `.goneat/ssot-consumer.yaml` to sync from Crucible v0.2.22
  - Schema `goneat-tools-config.schema.yaml` now includes `node` and `python` installer kinds
  - Various config, schema, and documentation updates from upstream

### Fixed

- **CI Tool Detection** (committed in v0.3.13 prep):
  - Added `findToolPath()` helper that checks PATH then shim directories
  - Fixed `DetectPackageManager()` to use `tools.DetectBrew()` for brew detection
  - Format command now uses findToolPath for yamlfmt, prettier, goimports
  - CI bootstrap workflows now properly detect foundation tools

## [0.3.12] - 2025-12-04

### Fixed

- **CRITICAL: yamlfmt Compatibility in Doctor Tools Init** (`cmd/doctor_tools_init.go`):
  - Fixed bug where `goneat doctor tools init` generated `.goneat/tools.yaml` with incorrect indentation
  - Root cause: Used `yaml.Marshal` with default 4-space indent instead of 2-space (yamlfmt standard)
  - Impact: Fresh clones failed `make fmt` workflow with "did not find expected key" yamlfmt error
  - Fix: Replace `yaml.Marshal` with `yaml.NewEncoder` + `SetIndent(2)` to match `.yamlfmt` config
  - Result: Generated tools.yaml now passes yamlfmt validation without manual edits
  - Added regression test: `TestDoctorToolsInitGeneratesYamlfmtCompatibleFile`
  - Properly handle file.Close() errors per linter requirements

## [0.3.11] - 2025-12-03

### Added

- **SBOM Tools Support**: Added syft tool and sbom scope for Software Bill of Materials generation
  - New `sbom` scope with syft tool definition
  - syft installable via brew (darwin/linux) and scoop (windows: `scoop install main/syft`)
  - Integrated with `goneat dependencies --sbom` command
  - Added to standard scopes generated by `goneat doctor tools init`
  - Use `goneat doctor tools --scope sbom --install --yes` to install syft

- **Windows Platform Support (Limited)**: Initial support for Windows development and testing
  - Binary now builds as `goneat.exe` on Windows via Makefile platform detection
  - Integration test helpers (`testenv.go`) detect and use `.exe` binaries on Windows
  - Foundation tools (ripgrep, jq) work via Scoop package manager
  - Note: Full Windows support pending performance improvements for build/test cycles

- **Test Parallelization Infrastructure**: Support for parallel test execution to improve test performance
  - New `GONEAT_TEST_PARALLEL` Makefile variable (default: 1, override with env var or command line)
  - Supports both `export GONEAT_TEST_PARALLEL=3` and `make test GONEAT_TEST_PARALLEL=3`
  - Proof-of-concept: Added `t.Parallel()` to 124 tests across 3 packages:
    - `internal/doctor`: 69 tests parallelized
    - `pkg/versioning`: 23 tests parallelized
    - `pkg/schema`: 32 tests parallelized (3 tests excluded due to t.Setenv usage)
  - Measured 1.79x speedup (44% faster) with `-parallel 3` on Windows
  - Test timeout increased from 10m to 15m for Windows compatibility

- **Cross-Platform Line Ending Standard** (`.gitattributes`): Ensures consistent line endings across all platforms
  - Auto-normalizes text files to LF on commit (Windows, Mac, Linux)
  - Shell scripts forced to LF (required for Unix/Mac execution)
  - Windows-specific files (.bat, .cmd, .ps1) use CRLF
  - Binary files marked appropriately
  - Prevents CRLF/LF inconsistencies in repository

### Changed

- **Test Infrastructure Improvements**:
  - Added 30-second timeouts to all test helper commands (prevents hanging on Windows)
  - `RunVersionCommand()` in `testenv.go` now uses `context.WithTimeout`
  - `runCommand()` and `runGitCommand()` in test fixtures now have timeouts
  - Package manager detection (`brew --version`, `scoop --version`) now has 5-second timeout

- **Cross-Platform Test Compatibility**:
  - `doctor_workflow_test.go`: Added `goneatBinaryPath()` helper for platform-specific binary names
  - `testenv.go`: `findGoneatBinary()` now searches for correct binary extension per platform
  - All integration tests now properly detect Windows vs Unix binary paths

### Fixed

- **CRITICAL: Line Ending Corruption on Windows** (`pkg/format/finalizer/finalizer.go`):
  - Fixed bug where finalizer corrupted CRLF files by creating double CR characters (`\r\r\n`)
  - Root cause: `strings.Split(content, "\n")` on CRLF content left CRs attached to lines
  - When rejoined with CRLF, produced `line\r` + `\r\n` = `line\r\r\n` (corrupted)
  - Fix: Normalize to LF before processing, detect line ending first, rejoin with detected ending
  - Impact: Files with CRLF on Windows were repeatedly detected as needing formatting
  - Result: Windows formatting now stable, preserves file's original line ending style

- **Format Command Error Handling** (`cmd/format.go:950`):
  - Fixed "finalized" error being treated as failure instead of success
  - "finalized" indicates finalizer made changes (EOF, trailing spaces, line endings)
  - Now treated same as "needs formatting" - successful file modification
  - Eliminates false ERROR logs for finalizer-only changes on files without dedicated formatters

- **Windows Test Execution**: Fixed multiple issues preventing tests from running on Windows
  - Package manager version detection no longer hangs indefinitely
  - Integration tests now find `goneat.exe` instead of looking for Unix-style `goneat`
  - Test commands properly handle Windows executable extensions
  - Makefile binary name detection works correctly on Windows (via `OS` environment variable)

### Known Issues

- **yamlfmt on Windows**: Occasional error with `.git/**` path syntax in yamlfmt dry run
  - Error: `CreateFile .git/**: The filename, directory name, or volume label syntax is incorrect.`
  - Appears to be yamlfmt issue with git-related file handling on Windows
  - Does not block formatting of other files
  - Under investigation

## [0.3.10] - 2025-12-01

### Added

- **Install Probe CI Validation**: New `make install-probe` target validates package manager + tool combinations
  - Runs in CI to catch invalid installer configurations (e.g., scoop+prettier)
  - Uses build tag `installprobe` for opt-in execution
  - Non-destructive: probes package managers with info commands, never installs
  - Static `TestDefaultConfigInstallability` validates schema correctness in normal tests

- **All Standard Scopes in Tools Init**: `goneat doctor tools init` now generates all 4 standard scopes
  (foundation, security, format, all) regardless of `--scope` flag value
  - Ensures .goneat/tools.yaml is fully functional immediately after init
  - Users no longer need to run init multiple times for different scopes
  - New `ConvertToToolsConfigWithAllScopes()` function handles multi-scope generation

- **--no-cooling Flag for CI**: New `--no-cooling` flag for `goneat doctor tools` command
  - Disables package cooling policy checks
  - Essential for offline/air-gapped environments and CI runners
  - Prevents failures when unable to verify package release dates

- **Node/Python Tool Kinds**: Schema now allows "node" and "python" as valid tool kinds
  - Supports tools like prettier (node) and ruff (python)
  - Updated schemas/tools/v1.0.0/tools-config.yaml and v1.1.0/tools-config.yaml

### Changed

- **Package Manager Strategy Cleanup**:
  - `brew`: Primary for system binaries on darwin/linux (supports user-mode install)
  - `scoop/winget`: Primary for Windows
  - `go-install`: Primary for Go tools (no package manager needed)
  - `bun/npm`: Node.js packages only (e.g., eslint for TypeScript repos)
  - `uv/pip`: Python packages only
  - **Removed**: bun from system tool installers (ripgrep, jq, yq) - bun can't install system binaries
  - **Removed**: mise from default installers (it's a version manager, not general package manager)
  - **Removed**: scoop from prettier Windows installers (scoop lacks prettier package; uses bun only)
  - **Changed**: prettier now uses brew instead of bun on darwin/linux

- **Bootstrap Documentation**: Updated bootstrap-patterns appnote to v0.3.10 patterns

### Fixed

- **Makefile Bootstrap Exit Code Propagation**: Changed `;` to `&&` in bootstrap target
  - Bootstrap now fails fast on first error instead of continuing silently
  - CI now correctly detects bootstrap failures

- **Go Version Parsing**: Fixed semver comparison for Go versions like "go1.25.4"
  - Now strips "go" prefix before semver parsing
  - Prevents "invalid semver format" errors

- **Doctor Tool Installation Reliability**: Multiple fixes for package manager detection and installation
  - Route node-kind tools to `installSystemTool` for proper brew/bun installation
  - Derive candidate binary names from `detect_command` for accurate post-install lookup
  - Add brew to `GetShimPath` for proper PATH resolution
  - Add detected package manager paths to PATH before tool installation
  - Enhanced installer diagnostics with output capture and exit codes
  - Added INFO-level logging for package manager detection

- **Logger Nil Error Handling**: Fixed panic when `Err()` field constructor receives nil error

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
  - Uploads binaries (_.tar.gz, _.zip), SHA256SUMS, all signatures (.asc), and public key
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
