# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

> **Note**: This changelog keeps the latest 10 releases for readability. For older releases, see `docs/releases/` archive.

## [Unreleased]

## [v0.5.4] - 2026-02-25

### Fixed

- **Biome nested config handling (`format`)**: `goneat format` and `goneat format --check` now resolve Biome context per file and run from the nearest `biome.json`/`biome.jsonc`, fixing monorepo nested-root failures.
- **YAML parse error DX in check mode**: `yamlfmt` check paths now distinguish syntax errors from formatting differences, surfacing invalid YAML as a parse error instead of the misleading "needs formatting" message.
- **Biome nested config handling (`assess`)**: `goneat assess --categories format` and Biome lint assess paths now group files by resolved Biome context and run per-group, preventing `failed to parse biome json: no json output from biome` in nested-config repos and improving error detail when Biome emits non-JSON output.
- **Incremental lint fix-mode regression**: restored `--fix --new-issues-only` behavior so Biome fix pass executes before incremental reporting.

### Toolchain Updates

#### Go

- **gosec 2.23.0**: Three inter-procedural taint analysis rules are now active — G702 (command injection), G703 (path traversal), G704 (SSRF). These trace data flow across call boundaries and are distinct from the older G204/G304 rules; existing `#nosec G304` comments do **not** suppress G703. Projects running gosec ≥ 2.23.0 will see new findings in code that intentionally passes env-var paths to `os.Open` or config-derived URLs to `http.Client`. Use targeted `#nosec G702`/`G703`/`G704` with justification for acknowledged patterns.
- **golangci-lint 2.10**: QF1012 (`WriteString(fmt.Sprintf(…))` → `fmt.Fprintf`) is now reported across more code patterns. With the default `max-same-issues: 3`, findings rotate across files per run; set `max-same-issues: 0` in `.golangci.yml` to surface all instances at once.

## [v0.5.3] - 2026-02-09

### Fixed

- **Data validation draft gate**: `ValidateFromBytes` and `validate data` now accept all five supported JSON Schema drafts (Draft-04, Draft-06, Draft-07, 2019-09, 2020-12). Previously only Draft-07 and 2020-12 were accepted for data validation, despite v0.5.2 adding meta-schema validation for all drafts. This blocked downstream consumers (e.g., crucible) from validating configs against 2019-09 schemas.

## [v0.5.2] - 2026-01-21

### Added

- **Full JSON Schema draft coverage**: Meta-validation now supports Draft-04, Draft-06, Draft-07, 2019-09, and 2020-12
  - Embedded meta-schemas for air-gapped CI environments
  - Auto-detection from `$schema` field or explicit `--schema-id` flag
- **Schema CLI improvements**: Glob patterns (`"schemas/**/*.json"`) and `--recursive` directory validation
- **`min_version` alias**: Tools config now accepts `min_version` as deprecated alias for `minimum_version` for backwards compatibility

### Fixed

- **Tools config schema validation**: `goneat doctor tools` now correctly validates `.goneat/tools.yaml` against embedded schema (was silently skipping validation)
- **Vulnerability summary counts**: `--vuln` output now distinguishes between total findings and unique CVEs after deduplication
- **PATH ordering**: `doctor tools --install` now appends shim directories to PATH instead of prepending, avoiding unexpected shadowing of system-installed tools (e.g., Homebrew)

### Changed

- **Dependency modernization**: 84 packages updated across 3 staged releases
  - Stage 1: Patch versions, `golang.org/x/*`
  - Stage 2: Security updates, minor bumps
  - Stage 3: OPA, OpenTelemetry, container ecosystem
- **Makefile install target**: `make install` now only copies binary (assumes `make build` was run); use `make build install` for both
- **OS-aware install path**: `INSTALL_DIR` defaults to `%LOCALAPPDATA%/Programs/goneat` on Windows, `~/.local/bin` on Unix

## [v0.5.1] - 2026-01-17

### Security

- **Dependency vulnerability remediation**: Upgraded `go-licenses` v1.6.0 → v2.0.1 to remove 4 critical/high vulnerabilities (GHSA-449p-3h89-pw88, GHSA-v725-9546-7q7m) from transitive `gopkg.in/src-d/go-git.v4` dependency
  - go-licenses v2 dropped go-git.v4 entirely
  - API migration: `NewClassifier(0.9)` → `NewClassifier()`, `LicensePath` → `LicenseFile`

### Added

- **Security Decision Records (SDR) framework**: New `docs/security/` structure for documenting security-related decisions
  - `docs/security/decisions/` — SDRs for vulnerability assessments, false positive analysis, accepted risks
  - `docs/security/bulletins/` — User-facing security announcements
  - `docs/security/decisions/TEMPLATE.md` — SDR template for consistent documentation
- **SDR-001**: Documents GHSA-v778-237x-gjrc false positive in x/crypto (grype flags minimum version requirement, not resolved version)
- **Vulnerability allowlist with SDR references**: `.goneat/dependencies.yaml` now supports `sdr:` field linking suppressions to detailed analysis
- **Makefile install target**: `make install` builds and installs goneat to `~/.local/bin` for local testing
- **Makefile integration docs**: New `docs/user-guide/workflows/makefile-integration.md` covering development workflows

### Changed

- **Vulnerability policy mode**: Default `fail_on: none` (visibility mode) to allow incremental adoption

### Fixed

- **Zero-config vulnerability scanning**: `goneat dependencies --vuln` now works without explicit `.goneat/dependencies.yaml` config (uses sensible defaults)
- **Dependencies output clarity**: Shows "Packages scanned: N" instead of misleading "Dependencies: 0" during vulnerability scans

## [v0.5.0] - 2026-01-15

### Added

- **core.hooksPath detection**: `goneat hooks install` now detects `core.hooksPath` git config (common remnant from husky, lefthook, etc.) and provides clear fix options
  - Warns and aborts by default when detected
  - `--unset-hookspath` / `--force`: Clear the override and install to `.git/hooks/`
  - `--respect-hookspath`: Install hooks to the custom path instead
  - `hooks inspect` and `hooks validate` now report this diagnostic (text + JSON)
  - Relative path resolution: Works correctly when running from subdirectories
- **TypeScript typecheck assessment**: New `typecheck` category runs `tsc --noEmit`
  - Supports `file_mode: true` with `--include` for single-file checks
  - Configurable via `.goneat/assess.yaml`
- **Biome config diagnostics**: Lint now surfaces Biome schema mismatch warnings for `biome.json`
- **Toolchain support**: TypeScript scope now includes `tsc` for type checking
- **Assess config validation**: `.goneat/assess.yaml` is now schema-validated on read and during `doctor assess init`

### Fixed

- **Mutually exclusive flags**: `--respect-hookspath` and `--unset-hookspath` now error if both set

## [v0.4.5] - 2026-01-13

### Added

- **Rich cargo-deny output context**: Error messages now include specific license names, crate versions, and deny.toml file:line references
  - Labels field added to CargoDenyFinding struct (Message, Span, Line, Column)
  - FormatMessage() enriched to include label context: `[0BSD; unmatched license allowance; at deny.toml:53:6]`
  - Actionable context for license issues, ban violations, and duplicate crate warnings

- **Rust license enumeration**: `goneat dependencies --licenses` now lists all Rust dependencies with their licenses
  - Parses `cargo deny list` table output
  - Handles SPDX-like expressions (`MIT OR Apache-2.0`, `Unlicense OR MIT`)
  - Same `Dependency` schema as Go analyzer for unified output
  - Works in Cargo workspaces

### Fixed

- **Biome 2.x compatibility**: Updated format assessment for biome 2.x breaking changes
  - Biome 2.x removed `--check` flag; now uses exit codes for format detection
  - Version check requires biome 2.x or higher
  - Parse biome JSON diagnostics for reliable format issue detection
  - Respects `.biome.json` ignore rules correctly
- **Format assess fix mode**: Normalizes files (EOF newlines, trailing whitespace) when running `assess --categories format --fix`
- **cargo-deny STDERR output**: Fixed reading from stderr (cargo-deny outputs JSON to stderr by design)
- **cargo-deny command order**: Fixed `--format json` positioning (must precede `check` subcommand)
- **Unified cargo-deny implementation**: Removed duplicate parsing code in internal/assess/rust_cargo_deny.go
  - Now uses canonical `pkg/dependencies.RunCargoDeny()` implementation
- **Severity mapping**: "note" and "help" severities now correctly map to low (was defaulting to medium)

## [v0.4.4] - 2026-01-09

### Added

- **Rust dependency analysis**: `goneat dependencies --licenses` now works for Rust projects via cargo-deny integration
  - Detects Rust projects (Cargo.toml) and runs cargo-deny license/bans checks
  - Smart guidance when cargo-deny not installed (install instructions, docs link)
  - Shared runner in `pkg/dependencies/cargo_deny.go` (reusable by assess path)
  - Proper severity mapping: license violations → high, bans → medium, informational → low

- **Cargo tool installer**: `kind: cargo` support in tools.yaml schema
  - Install Rust tools via `cargo install <package>` (e.g., cargo-deny)
  - New `cargo-install` installer type in doctor tools infrastructure
  - Pattern updated to allow packages without @version suffix

- **Toolchain scopes**: Reorganized tools into language-specific scopes
  - `foundation`: Language-agnostic tools (ripgrep, jq, yq, yamlfmt, prettier, yamllint, shfmt, shellcheck, actionlint, checkmake, minisign)
  - `go`: Go development tools (go, go-licenses, golangci-lint, goimports, gofmt, gosec, govulncheck)
  - `rust`: Rust Cargo plugins (cargo-deny, cargo-audit)
  - `python`: Python tools (ruff - replaces black/flake8/isort)
  - `typescript`: TypeScript/JavaScript tools (biome - replaces eslint/prettier for TS)
  - `sbom`: SBOM generation and vulnerability scanning (syft, grype)
  - Usage: `goneat doctor tools --scope foundation,go --install --yes`

- **grype vulnerability scanner**: Added to sbom scope as companion to syft
  - syft generates SBOMs, grype scans them for vulnerabilities
  - Complete SBOM-based security workflow for CI/CD pipelines

### Changed

- **Crucible upstream**: Updated from v0.3.1 to v0.4.5
  - New agentic roles (prodmktg, uxdev)
  - Similarity library schemas and fixtures
  - Foundry signal resolution fixtures
  - Platform modules taxonomy v1.1.0

### Fixed

- **cargo-deny JSON output**: Fixed multiple integration issues discovered during testing
  - Argument order: `--format json` must precede `check` subcommand
  - STDERR output: cargo-deny outputs JSON to stderr, not stdout (documented behavior)
  - NDJSON parsing: Handle `type: "diagnostic"` entries with nested `fields` object
  - Severity mapping: "license-not-encountered" is informational (low), not a violation (high)

- **SSOT provenance trailing newline**: `goneat ssot sync` now writes provenance.json and metadata mirrors with trailing newlines
  - Prevents format diffs when downstream repos run formatters after ssot sync
  - Affects both aggregate provenance (.goneat/ssot/provenance.json) and per-source mirrors

## [v0.4.3] - 2026-01-08

### Added

- **Parallel format execution**: `goneat format` now defaults to `--strategy parallel`, providing significant speedup for large codebases
- **Parallel schema validation**: `goneat schema validate-schema` supports `--workers N` flag (0=auto, 1=sequential) for parallel meta-validation
- **Schema data validation subcommand**: New `goneat schema validate-data` subcommand for validating data files against JSON schemas
- **Cached meta-schema validators**: Draft-07 and 2020-12 meta-schema validators are compiled once and reused across all files

### Changed

- **Format default strategy**: Changed from sequential to parallel execution for better performance on multi-core systems
- **Deterministic validation output**: Schema validation results maintain input file order regardless of parallelism

### Performance

- **Format**: Parallel execution with configurable `--workers` (default auto-detects CPU count, max 8)
- **Schema validation**: ~1.7x speedup on large schema sets (776 files: 6.77s → 3.97s with 4 workers)

## [v0.4.2] - 2026-01-03

### Added

- **Rust lint**: `cargo-clippy` now runs under `assess --categories lint` when present
- **Doctor rust scope**: manual cargo install hints for `cargo-deny` and `cargo-audit`

### Changed

- **Dependencies docs**: Added prominent note that `.goneat/dependencies.yaml` policy file is required for license violation detection

### Fixed

- **Release notes embedding**: Release notes are embedded from `docs/releases/v<version>.md` (no shadow copies under `docs/`).
- **Dependencies: suppress stdlib noise**: License detection no longer reports "degraded" due to harmless go-licenses warnings about Go standard library packages
- **Format check mode**: `goneat format --check` now correctly reports files needing formatting when primary formatter (e.g., yamlfmt) detects issues but finalizer says OK

## [v0.4.1] - 2026-01-02

### Added

- **Explicit incremental lint checking**: New `--new-issues-only` and `--new-issues-base` flags for `goneat assess`
  - `--new-issues-only`: Only report issues introduced since a baseline git reference (opt-in)
  - `--new-issues-base`: Git reference for baseline comparison (default: `HEAD~`)
  - Supports golangci-lint (`--new-from-rev`) and biome (`--changed --since`)
  - Documentation: `docs/appnotes/assess/incremental-lint-checking.md`

### Changed

- **Crucible SSOT**: synced to Crucible v0.3.0
- **Lifecycle phases**: Added experimental, rc, lts; removed maintenance (per crucible schema)
- **Release phases**: Added ga; deferred hotfix to crucible v1.1.0
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

---

**Note**: Older releases (0.3.25 and earlier) have been archived. See git history or `docs/releases/` for details.
