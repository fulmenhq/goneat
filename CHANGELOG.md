# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

## [Unreleased]

### Added

- Schema: Complete directory-based versioning system implementation for maintainable schema evolution
- Schema: Comprehensive schema validation integration for dates and main goneat configurations
- Testing: Significantly improved test coverage for pkg/config and pkg/schema packages

### Fixed

- Assessment: Updated JSON test fixtures to use human-readable time durations instead of nanoseconds
- Assessment: Removed manual validation in favor of schema validation in dates runner tests
- Assessment: Fixed test expectations for CHANGELOG monotonic ordering (now working correctly)

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

## [0.1.6] - In Development

### Added

- Comprehensive test coverage improvements for `pkg/work/format_processor`, `pkg/work/planner`, and `pkg/format/finalizer` packages
- New intuitive CLI flags for format command: `--files` and `--patterns` for clearer file selection

### Changed

- **BREAKING**: Replaced confusing `-f/--files` flag behavior in format command
  - **Old**: `-f "*.go"` treated as glob pattern for file discovery
  - **New**: `--files file1 file2` for explicit file lists, `--patterns "*.go"` for glob filtering
  - **Migration**: Use `--patterns` for old `-f` pattern behavior, `--files` for specific files
  - **Validation**: Clear error messages prevent conflicting flag combinations

### Fixed

- Fixed os.RemoveAll error handling in test cleanup code (addressed high-severity lint issues)

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

[Unreleased]: https://github.com/fulmenhq/goneat/compare/v0.1.5...HEAD
[0.1.5]: https://github.com/fulmenhq/goneat/compare/v0.1.4...v0.1.5
[0.1.4]: https://github.com/fulmenhq/goneat/compare/v0.1.2...v0.1.4
[0.1.2]: https://github.com/fulmenhq/goneat/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/fulmenhq/goneat/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/fulmenhq/goneat/releases/tag/v0.1.0

## [0.1.4] - 2025-09-04

### Added

- 🛠️ Enhanced Configuration Schema Support
  - Comprehensive YAML schema validation with proper formatter options structure
  - JSON and Markdown formatting configuration support
  - Improved schema organization with consistent indentation and structure

### Changed

- 🔧 Work Planner File Discovery Improvements
  - Fixed eliminateRedundancies logic to preserve sibling files instead of incorrectly filtering by directory
  - Enhanced file validation to prevent corrupted path processing
  - Improved hook configuration consistency with proper YAML formatting

### Fixed

- 🐛 Critical Auto-fix Reliability Issues
  - Resolved work planner bug that was dropping valid files from processing queue
  - Fixed YAML schema structural corruption that prevented format operations
  - Corrected test environment binary discovery to use dist/ directory structure
  - Fixed Makefile GOTEST variable reference for proper test execution

## [0.1.2] - 2025-08-30

### Added

- 🛠️ Hooks Dogfooding & Template Engine
  - Schema-driven hook templates under `templates/hooks/bash/` rendered via `goneat hooks generate`
  - Templates consume `.goneat/hooks.yaml` for args, fallback, and optimization (`only_changed_files`)
  - Dev-mode fallback and robust binary discovery (PATH + repo `dist/` + common locations)
  - Docs updated with setup, output modes, and JSON piping
    - `docs/user-guide/workflows/git-hooks-operation.md`
    - `docs/user-guide/commands/hooks.md`

- 🔎 Concise Hook Output + Pretty Renderer (prototype)
  - New `concise` output format for short, colorized summaries in hooks (top-N files listed)
  - `goneat pretty` (stub) renders JSON to console (concise) or HTML using existing formatter
  - Env override: `GONEAT_HOOK_OUTPUT=concise|markdown|json|html|both`

- ✍️ Format Command Improvements
  - `--staged-only` to operate on staged files (ACMR)
  - `--ignore-missing-tools` to skip YAML/JSON/MD formatting if external tools absent
  - Plan/dry-run works with staged-only (synthesized plan)

- 📚 Environment Variables (SSOT)
  - Added `docs/environment-variables.md` covering `GONEAT_HOOK_OUTPUT`, `NO_COLOR`, `GONEAT_TEMPLATE_PATH`, and future vars

### Changed

- Hook mode output selection:
  - Honors explicit `--format`; otherwise `GONEAT_HOOK_OUTPUT`, else `--verbose` → markdown, else concise
- Reduced runner “failed without error” log noise to debug in hook mode context
- Concise output: fallback to first issue message when no file path is available

### Fixed

- Robust JSON parsing in `goneat pretty` (tolerates log preambles)
- Hook templates prefer repo-local `dist/goneat`; improved fail-fast guidance when missing

### Technical Details

- Taxonomy docs: `docs/architecture/command-taxonomy-validation-adr.md`
- Hook docs: `docs/user-guide/workflows/git-hooks-operation.md`, `docs/user-guide/commands/hooks.md`
- Structured fixtures: `tests/fixtures/` for ongoing lint/format testing

## [0.1.1] - 2025-08-28

### Added

- **Assessment System Enhancement**: Concurrency support for parallel processing
  - Configurable worker count and CPU percentage utilization
  - Improved performance for large codebase assessments
  - JSON-first reporting with HTML fallback

### Changed

- **Report Format**: Enhanced HTML template with better styling and information architecture
- **Assessment Engine**: Format run summaries and improved error handling
- **Git Integration**: Better semver/calver tag detection and validation

### Fixed

- Lint issues across assessment engine and formatter modules
- Static analysis warnings in runner and engine components

## [0.1.0] - 2025-08-28

### Added

- **Version Command**: Complete version management system
  - Multi-source version detection (VERSION files, git tags, Go constants)
  - Version bumping (patch, minor, major)
  - Version setting with validation
  - First-run detection and intelligent setup guidance
  - Git integration with tag creation
  - JSON and extended output formats
  - Assessment mode (`--no-op`) for safe testing

- **Format Command**: Code formatting with Go support
  - Go file formatting using `gofmt`
  - Dry-run and plan-only modes
  - Sequential and parallel execution strategies
  - File discovery and filtering
  - Comprehensive error handling

- **Test Infrastructure**: Enterprise-grade testing framework
  - Integration test suite (28+ tests)
  - Test environment framework (`TestEnv`)
  - Fixture helpers for various scenarios
  - Cross-platform testing support

- **Standards & Documentation**: Comprehensive project standards
  - Document frontmatter standard
  - Copyright template for code files
  - Authoring guidelines and templates
  - Repository safety protocols
  - User guides and API documentation

- **Internal Architecture**: Robust internal systems
  - Operations registry for command management
  - Assessment engine foundation
  - Configuration management system
  - Logger infrastructure

### Changed

- Repository structure optimized for Fulmen ecosystem
- Build system enhanced with cross-platform support
- Error handling improved throughout codebase

### Fixed

- Errcheck issues resolved in test files
- Code formatting consistency improved
- Static analysis warnings addressed

### Technical Details

- **Go Version**: 1.21+
- **Dependencies**: Cobra CLI, Viper config, Testify testing
- **Platforms**: Linux, macOS, Windows (AMD64/ARM64)
- **Test Coverage**: 75%+ of testable code
- **Build System**: Makefile with cross-platform targets

---

## Release Notes Template

When creating a new release, copy this template and fill in the details:

```markdown
## [x.y.z] - YYYY-MM-DD

### Added

- New features and functionality

### Changed

- Modifications to existing functionality

### Deprecated

- Features scheduled for removal

### Removed

- Removed features

### Fixed

- Bug fixes and patches

### Security

- Security-related changes
```

### Version Numbering

- **MAJOR**: Breaking changes (1.0.0 → 2.0.0)
- **MINOR**: New features, backward compatible (1.0.0 → 1.1.0)
- **PATCH**: Bug fixes, backward compatible (1.0.0 → 1.0.1)

### Pre-release Versions

- **Alpha**: `1.1.0-alpha.1` - Early testing
- **Beta**: `1.1.0-beta.1` - Feature complete, testing
- **RC**: `1.1.0-rc.1` - Release candidate

---

## Guidelines

### Contributing to the Changelog

1. **Keep entries brief but descriptive**
2. **Group changes by type** (Added, Changed, Fixed, etc.)
3. **Use present tense** for changes ("Add feature" not "Added feature")
4. **Reference issues/PRs** when applicable
5. **Update on release** - Move unreleased changes to version section

### Release Process

1. Update VERSION file with new version
2. Move unreleased changes to new version section
3. Add release date
4. Commit changes
5. Create git tag
6. Push to all remotes
7. Create GitHub release

---

**Legend:**

- 🎉 Major features and milestones
- 🔧 Technical improvements
- 🐛 Bug fixes
- 📚 Documentation updates
- 🏗️ Infrastructure changes

- Curated docs are selected via `docs/embed-manifest.yaml` and mirrored to `internal/assets/embedded_docs/docs/` to ensure `go install` includes assets.
- Frontmatter‑based selection planned for v0.2.2.

### Security

- All content operations are rooted under `docs/`; writes use 0644; no network access.
