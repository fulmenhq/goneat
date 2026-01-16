# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

> **Note**: This changelog keeps the latest 10 releases for readability. For older releases, see `docs/releases/` archive.

## [Unreleased]

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

- **Idempotent release doc embedding**: `docs/releases/latest.md` no longer regenerates on every build; only updates when version-specific release notes change (see `scripts/embed-assets.sh`)
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

---

**Note**: Older releases (0.3.21 and earlier) have been archived. See git history or `docs/releases/` for details.
