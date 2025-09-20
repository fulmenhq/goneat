# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

## [0.2.7] - 2025-09-20

### Added

- **Version Policy Enforcement**: Comprehensive tool version management and compliance checking
  - Added version policy configuration to `.goneat/tools.yaml` with `minimum_version`, `recommended_version`, and `version_scheme` fields
  - Implemented version detection in `doctor` command using `DetectCommand` parsing for accurate version extraction
  - Enhanced `assess` command to enforce version policies with proper severity levels (minimum violations = high, recommended = medium)
  - Added support for multiple version schemes: `semver` (semantic versioning) and `lexical` (string comparison)
  - Demonstrated real-world capability with golangci-lint v1.64.8 → v2.4.0 upgrade enforcement

- **Tool Assessment Improvements**: Enhanced tool checking capabilities across assessment and doctor commands
  - Fixed version detection logic to properly parse tool version commands
  - Added version policy violation reporting with actionable guidance
  - Improved cross-platform version detection for Go, Node.js, and system tools
  - Enhanced error messaging for version policy violations

### Fixed

- **YAML Schema Validation**: Corrected indentation issues in `schemas/config/v1.0.0/dates.yaml`
  - Fixed `cross_file_consistency` and `monotonic_order` property alignments
  - Resolved YAML syntax errors that were causing schema validation failures

### Changed

- **Documentation Updates**: Updated command documentation to reflect new version checking capabilities
  - Enhanced `docs/user-guide/commands/doctor.md` with version policy checking details
  - Updated `docs/appnotes/intelligent-tool-installation.md` with version policy configuration examples

### Added

- **Versioning Library**: New `pkg/versioning` library for version comparison and policy enforcement
  - Support for multiple version schemes: semver-full, semver-compact, semver-legacy, calver, lexical
  - Version policy evaluation with minimum/recommended version checking
  - Comprehensive test coverage and integration tests
  - Used internally by tool assessment for version policy enforcement

- **Developer Libraries Documentation**: Comprehensive documentation for reusable pkg/ libraries
  - Added detailed guides for all public libraries in `docs/appnotes/lib/` (config, pathfinder, schema, ignore, safeio, logger, exitcode, buildinfo, versioning)
  - Created `docs/user-guide/libraries.md` overview with import guidelines, stability matrix, and integration patterns
  - Updated README.md with "Developer Libraries" section clarifying single `go install` covers CLI + libraries (no duplicate installs needed)
  - Added code examples, API references, and best practices for each library

- **Dates Configuration and Documentation**: Enhanced date validation configuration and user guidance
  - Updated `.goneat/dates.yaml` with production-ready configuration including docs exclusion to eliminate false positives
  - Enhanced `docs/user-guide/commands/dates.md` with complete quick start, go install instructions, and production examples
  - Updated `docs/configuration/date-validation-config.md` with full reference, troubleshooting, and advanced patterns
  - Added comprehensive exclusions for docs/tests/vendor, file type severity modifiers, and performance tuning

### Changed

- **Dates Assessment**: Improved DX by excluding documentation from date validation (reduces noise in library docs and examples)
  - Default configuration now excludes `docs/**` while preserving CHANGELOG/release notes validation
  - Added detailed comments and rationale in `.goneat/dates.yaml` for all sections and exclusions

### Fixed

- **Dates False Positives**: Resolved documentation date issues by implementing `docs/**` exclusion pattern
  - Reduced date assessment issues from 7 to 0 in the repository
  - Preserved AI safety features and critical file validation (CHANGELOG.md, RELEASE_NOTES.md)



## [v0.2.6] - 2025-09-15

### Added

- **Repository Validation System (Phase 1)**: New comprehensive repository health and release readiness validation
  - Git state validation (clean working directory, correct branch, proper tags)
  - Version consistency checking (VERSION file vs RELEASE_PHASE policies)
  - Documentation sync validation (CHANGELOG.md, RELEASE_NOTES.md version alignment)
  - Release readiness assessment for different phases (dev/rc/release)
  - Schema validation for configuration files
  - New `repository` command for phase management (`phase set/show`)
  - New `maturity` command for validation (`validate`, `release-check`)
  - Assessment integration with `maturity` category
  - Pre-commit/pre-push hook integration
  - JSON output support for CI/CD pipelines
- **Release Readiness Workflow Documentation**: New comprehensive workflow guide at `docs/user-guide/workflows/release-readiness.md`
- **Command Documentation**: New maturity.md and repository.md command documentation files

### Fixed

- **Version Command Binary-First Display**: `goneat version` now correctly shows goneat binary version (0.2.6) instead of host project version
  - Default behavior prioritizes embedded binary version
  - Project version management preserved via `--project` flag
  - Clear separation in JSON output structure
- **Content Command Glob Pattern Matching**: Fixed `**/*.json` and `docs/**/*.md` patterns
  - Pure Go implementation of comprehensive glob pattern matching
  - Recursive `**` wildcard support for directory trees
  - Proper path segment processing with boundary validation
- **Import Cycle Resolution**: Fixed circular dependency between `pkg/pathfinder` and `pkg/pathfinder/loaders`
  - Removed duplicate type/function declarations
  - Cleaned up conflicting implementations
- **Security Issues**: Resolved 3 security vulnerabilities
  - G304 file inclusion via variable (2 instances)
  - G104 unhandled error in file operations
- **Lint Issues**: Fixed 8 golangci-lint violations
  - Unhandled errors in test functions
  - Empty branch conditions
  - Missing error handling in defer statements

### Changed

- **Code Formatting**: Applied consistent formatting across all Go and documentation files
- **Documentation Formatting**: Standardized Markdown and YAML formatting throughout docs tree

## [v0.2.5] - 2025-09-13

### Added

- **Infrastructure Tools Management**: New `infrastructure` scope for `goneat doctor tools` command
  - Cross-platform detection and installation of ripgrep, jq, and go-licenses
  - Schema-driven configuration system with JSON Schema validation
  - Support for tool name vs binary name distinction (e.g., `ripgrep` vs `rg`)
  - User-configurable tool policies via `.goneat/tools.yaml`
- **Enhanced Doctor Tools CLI**:
  - `--dry-run` flag to preview installations without executing
  - `--config` flag to specify custom tools configuration file
  - `--list-scopes` flag to display available tool scopes
  - `--validate-config` flag to validate configuration files
  - JSON output support for AI agent consumption
- **Assessment Integration**: Tools checking integrated into `goneat assess` command
  - New `tools` category with priority 1 (critical for CI/CD)
  - Automatic tools validation in pre-commit and pre-push hooks
  - Parallel execution with other assessment categories
- **Schema System**: New tools configuration schema (`schemas/tools/v1.0.0/tools-config.yaml`)
  - JSON Schema Draft 2020-12 compliance
  - Embedded default configuration with user override support
  - Comprehensive validation with detailed error reporting

### Changed

- **Version Command**: Changed `basic` template default version from `1.0.0` to `0.1.0` for better semver practices
- **Version Command**: Enhanced `--initial-version` flag documentation with examples for custom version specification
- **Hooks System**: Updated default pre-commit and pre-push hooks to include tools checking
- **Assessment Engine**: Added tools runner for infrastructure tool validation
- **Configuration Management**: Enhanced schema validation system with tools configuration support

### Fixed

- **Tool Detection**: Improved cross-platform tool detection with proper command parsing
- **Installation Methods**: Enhanced platform-specific installation command handling
- **Error Handling**: Better fallback instructions for manual tool installation
- **Assessment Engine**: Fixed shouldFail function to properly handle category errors (e.g., lint config failures)

## [v0.2.4] - 2025-09-12

### Added

- **Schema Validation DX**: Three new ergonomic helper functions eliminate 80%+ boilerplate code
  - `ValidateFileWithSchemaPath()` - Simple file-to-file validation with automatic format detection
  - `ValidateFromFileWithBytes()` - Validate raw data bytes against schema file
  - `ValidateWithOptions()` - Enhanced validation with custom context and options
- **Comprehensive Test Coverage**: 13 new test functions covering all helper functions and edge cases
- **Enhanced Documentation**: Updated library appnotes with 9 examples including migration guides
- **Security Hardening**: All new functions include path sanitization and security controls

### Changed

- **Schema Library API**: Extended with backward-compatible ergonomic helpers
- **Documentation**: Updated appnotes with v0.2.4 features and migration patterns
- **Release Notes**: Comprehensive documentation of DX improvements and usage examples

### Fixed

- **DX Friction**: Eliminated 15+ lines of boilerplate code for common validation patterns
- **Error Context**: Enhanced error reporting with file paths and validation context
- **API Consistency**: All new functions follow existing patterns and conventions
- **Project Name Detection**: `goneat version` now correctly detects project names from go.mod, directory, or git remote instead of hardcoding "goneat"
- **Version Output**: Added `projectName` field to JSON output for programmatic consumers

## [v0.2.3] - 2025-09-09

### Added

- Assessment: Date validation category to prevent embarrassing date mistakes in documentation
- Assessment: Configurable file patterns for date validation (CHANGELOG.md, RELEASE_NOTES.md, docs/releases/, etc.)
- Assessment: Support for multiple date formats (YYYY-MM-DD, YYYY/MM/DD, YYYY.MM.DD)
- Hooks: Cross-platform hook template support for Windows (PowerShell/CMD) and Unix-like systems (bash)
- Envinfo: Enhanced system information display including user home directory and temp directory paths
- Documentation: Comprehensive user guide for date validation feature
- Documentation: Configuration guide for customizing date validation patterns
- Assessment: Extended output format (`--extended`) provides detailed workplan information for debugging and automation
- Assessment: JSON output now displays human-readable time durations (e.g., "15m", "2h30m") instead of nanoseconds for better AI agent and API consumer usability
- Assessment: Complete schema validation integration for all configuration files with graceful fallback to defaults
- Changelog: Corrected monotonic date ordering in release version headers

### Fixed

- Assessment: Enabled monotonic date ordering validation by default in CHANGELOG.md and documentation files
- Assessment: Fixed single file assessment to properly detect date validation issues
- Format: Enhanced finalizer logic to ensure trailing whitespace trimming takes precedence over "already formatted" responses
- Assessment: Improved file pattern matching for date validation runner
- Lint: Fixed golangci-lint "mixed-directory" error when processing files from different Go packages; added automatic package detection and --package-mode flag for safe multi-package linting
- Version: Cleaned up erroneous 1.x git tags (v1.0.0, v1.1.0) from early development; aligns with 0.x semver
- Assess: Dynamic version reporting in assessment output (from hardcoded "1.0.0" to buildinfo.BinaryVersion)

## [v0.2.2] - 2025-09-09

### Fixed

- Hooks: Fixed hardcoded invalid severity level "error" in hook generation (changed to "high")
- Hooks: Fixed default `--staged-only` behavior - now defaults to `false` for better developer experience
- Hooks: Added helpful comments in default hooks.yaml explaining `only_changed_files` option
- Error handling: Resolved 15 high‑severity errcheck issues across cmd/ (fmt writes, WalkDir, file Close)
- Security: Hardened `content` embed/verify (path validation under repo root, restrictive perms ≤0750/0640)

### Changed

- Hooks: `--staged-only` mode is now opt-in rather than opt-out, improving flexibility for teams
- Hooks: Default configuration now includes explanatory comments for better understanding
- Hooks: Updated help text to only show valid severity levels: critical|high|medium|low

## [v0.2.2-rc.4] - 2025-09-09

### Fixed

- Hooks: Fixed hardcoded invalid severity level "error" in hook generation (changed to "high")
- Error handling: Resolved 15 high‑severity errcheck issues across cmd/ (fmt writes, WalkDir, file Close)
- Security: Hardened `content` embed/verify (path validation under repo root, restrictive perms ≤0750/0640)
- Hooks: Updated help text to only show valid severity levels: critical|high|medium|low
- Hooks: Fixed default pre-commit hook configuration to use valid severity levels

## [0.2.2-rc.3] - 2025-09-09

### Fixed

- CI: Updated golangci-lint-action from v6 to v7 to resolve compatibility issue with golangci-lint v2.4.0
- CI: Aligned Go version from 1.22.x to 1.25.x in release workflow to match project requirements
- CI: Fixed .golangci.yml configuration to be compatible with golangci-lint v2.4.0 (removed unsupported settings)
- CI: Added golangci-lint config verification as preflight check in lint assessments

### Added

- CI: Preflight config verification for golangci-lint to catch configuration issues early
- Test: Added test fixtures for golangci-lint configuration validation (valid/invalid configs)
- Test: Unit tests for config verification functionality

## [0.2.2-rc.1] - 2025-09-07

### Added

- Format text normalizer (see Unreleased) and unit tests in finalizer package
- Security policy filter suppressing gosec G302/G306 for required git hook exec perms (0700)
- Docs: Security memo documenting hooks permissions policy exception

### Changed

- Hooks policy reverted to strict gates (pre‑push=high) ahead of v0.2.2 fast‑follow
- CI tab fix for coverage target; ensures GitHub build-all job passes
- Assess tests stabilized (fresh Cobra instances; no os.Exit on fail gates)

### Fixed

- Error handling: fmt.Fprintf/Fprintln returns and file Close handled across commands
- Content path security: manifest and copy/verify operations constrained to repo root; variable path reads annotated

[0.2.2-rc.1]: https://github.com/fulmenhq/goneat/compare/v0.2.1...v0.2.2-rc.1

## [0.2.1] - 2025-09-07

### Added

- New `docs` command for read-only offline docs (list/show; JSON/markdown/html, `--open` to browser)
- New `content` command for curated docs management (`find`, `embed`, `verify`), with JSON report schema
- JSON Schema for docs embed manifest (`schemas/content/docs-embed-manifest-v1.0.0.json`)
- Tests for content/docs commands (JSON outputs and verify path)
- Security quick alias: `goneat security secrets` (gitleaks-only convenience)

### Changed

- Embedding SOP extended to include curated docs via content command
- `embed-assets` prefers CLI for docs mirroring; `verify-embeds` uses `goneat content verify`
- README and root help emphasize `docs` (viewing) vs `content` (curation)

### Fixed

- Removed footer attributions from embedded docs to keep output clean

[0.2.1]: https://github.com/fulmenhq/goneat/compare/v0.2.0...v0.2.1

## [0.2.1-rc.1] - 2025-09-07

### Added

- Introduced curated docs pipeline and commands ahead of GA
- Initial manifest and CI verification wiring for embedded docs

### Notes

- Superseded by 0.2.1 GA with minor polish

[0.2.1-rc.1]: https://github.com/fulmenhq/goneat/compare/v0.2.0...v0.2.1-rc.1

## [0.2.0] - 2025-09-06

### Added

- Schema validation (JSON/YAML) with offline-first checks (Draft-07, 2020-12)
- Discovery controls: `--scope`, `--force-include`, `--no-ignore`
- Opt-in meta-schema validation: `--enable-meta` / `--schema-enable-meta`

### Changed

- Scoped traversal and DX improvements with quoted glob guidance

### Performance

- Bad fixtures: ~260–280ms scoped; single file ~200ms; repo schemas ~2–3s

[0.2.0]: https://github.com/fulmenhq/goneat/releases/tag/v0.2.0

## [0.2.0-rc.1] - 2025-09-05

### Added

- Schema validation (JSON/YAML) with offline-first structural checks (Draft-07, 2020-12)
- New flags for discovery control:
  - `--scope` to limit traversal to include paths and force-include anchors
  - `--force-include` to bring back ignored files/dirs (repeatable; glob-friendly)
  - `--no-ignore` to bypass ignore files for a run
  - `--enable-meta` / `--schema-enable-meta` to perform meta-schema validation (opt-in)
- Non-schema JSON/YAML fixtures in both good and bad sets to ensure no false positives

### Changed

- Improved file discovery to avoid skipping ancestors of forced paths
- Scoped-dir discovery uses targeted traversal for predictable DX
- Documentation updated with quoting globs guidance and scoped examples

### Fixed

- Path detection for schema files (relative `schemas` segment)
- Eliminated previous slowdowns from remote meta-schema fetch during default validation

[0.2.0-rc.1]: https://github.com/fulmenhq/goneat/compare/v0.1.5...v0.2.0-rc.1

## [0.1.5] - 2025-09-05

### Added

- 🎉 Diff‑Aware Assessment (Change‑Set Intelligence)
  - `metadata.change_context` with modified files, total changes, scope (small/medium/large), branch and SHA
  - Issue annotations: `change_related` and best‑effort `lines_modified`
  - Go‑git–first collection with CLI fallback for unified diffs
- 🔎 Suppression Insights (Security)
  - `categories.security.suppression_report.summary` now includes `by_rule_files`, `by_file`, `top_rules`, `top_files`
  - New CLI flag `--track-suppressions` on `assess` to expose intentional suppressions
- 📚 Documentation
  - Assess docs updated with change‑aware assessment and suppression examples
  - README highlights diff‑aware assessment and suppression insights
- 🧪 Smart Semantic Validation (Preview)
  - Schema‑aware validation category scaffolding (pending finalize for 0.1.5)
  - Config‑first patterns and opt‑in auto‑detect (planned)

### Changed

- 🔧 Assessment status normalization
  - Category status values standardized to `success`, `error`, or `skipped`
- 🧪 CLI test robustness
  - Fresh `assess` command instance per subtest to avoid flag reuse

### Fixed

- 🐛 Invalid mode validation for `assess --mode` now errors properly for unknown values

---

[Unreleased]: https://github.com/fulmenhq/goneat/compare/v0.2.7...HEAD
[v0.2.7]: https://github.com/fulmenhq/goneat/compare/v0.2.6...v0.2.7
[v0.2.6]: https://github.com/fulmenhq/goneat/compare/v0.2.5...v0.2.6
[v0.2.5]: https://github.com/fulmenhq/goneat/compare/v0.2.4...v0.2.5
[v0.2.4]: https://github.com/fulmenhq/goneat/compare/v0.2.3...v0.2.4
[v0.2.3]: https://github.com/fulmenhq/goneat/compare/v0.2.2...v0.2.3
[v0.2.2]: https://github.com/fulmenhq/goneat/compare/v0.2.1...v0.2.2
[v0.2.1]: https://github.com/fulmenhq/goneat/compare/v0.2.0...v0.2.1
[v0.2.0]: https://github.com/fulmenhq/goneat/releases/tag/v0.2.0
[0.1.5]: https://github.com/fulmenhq/goneat/compare/v0.1.4...v0.1.5
[0.1.4]: https://github.com/fulmenhq/goneat/compare/v0.1.2...v0.1.4
[0.1.2]: https://github.com/fulmenhq/goneat/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/fulmenhq/goneat/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/fulmenhq/goneat/releases/tag/v0.1.0
