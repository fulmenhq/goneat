# Goneat v0.4.4 — Rust Dependency Analysis via cargo-deny

**Release Date**: 2026-01-09
**Status**: Stable

## TL;DR

- **Rust license checking**: `goneat dependencies --licenses` now works for Rust projects
- **cargo-deny integration**: License compliance and banned crate detection via cargo-deny
- **Cargo tool installer**: `kind: cargo` support in tools.yaml for installing Rust tools
- **Toolchain scopes**: Language-specific tool scopes (`go`, `rust`, `python`, `typescript`)
- **Smart guidance**: Helpful messages when cargo-deny is not installed
- **SSOT fix**: Provenance files now include trailing newlines (prevents format diffs in downstream repos)

## What Changed

### Rust Dependency Analysis

`goneat dependencies --licenses` now supports Rust projects via cargo-deny:

```bash
# Analyze Rust project licenses
cd my-rust-project
goneat dependencies --licenses

# JSON output for CI integration
goneat dependencies --licenses --format json
```

**Detection & Guidance**: When a Rust project is detected but cargo-deny is not available:

```
Rust project detected but cargo-deny not installed.

To set up Rust dependency checking:
  1. Install cargo-deny: cargo install cargo-deny
  2. Initialize config:  cargo deny init
  3. Learn more:         goneat docs show user-guide/rust/dependencies
```

### Severity Mapping

| Finding Type | Severity | Example |
|--------------|----------|---------|
| License violation | high | Unlicensed crate, GPL in MIT project |
| Banned crate | medium | Duplicate versions, denied crate |
| Informational | low | "license-not-encountered" warnings |

### Cargo Tool Installer

New `kind: cargo` in tools.yaml schema enables installing Rust tools:

```yaml
tools:
  cargo-deny:
    name: cargo-deny
    description: License and dependency checking for Rust
    kind: cargo
    detect_command: cargo deny --version
    install_package: cargo-deny
    minimum_version: 0.14.0
```

Install via: `goneat doctor tools --scope rust --install --yes`

### Toolchain Scopes

Tools are now organized into language-specific scopes instead of mixing everything in `foundation`:

| Scope | Purpose | Key Tools |
|-------|---------|-----------|
| `foundation` | Language-agnostic | ripgrep, jq, yq, yamlfmt, prettier, yamllint, shfmt, shellcheck, actionlint, checkmake, minisign |
| `go` | Go development | go, go-licenses, golangci-lint, goimports, gofmt, gosec, govulncheck |
| `rust` | Rust Cargo plugins | cargo-deny, cargo-audit |
| `python` | Python tools | ruff (replaces black/flake8/isort) |
| `typescript` | TS/JS tools | biome (replaces eslint/prettier for TS/JS/JSON) |

**Usage examples:**

```bash
# Go project
goneat doctor tools --scope foundation,go --install --yes

# Rust project
goneat doctor tools --scope foundation,rust --install --yes

# Polyglot project (like goneat itself)
goneat doctor tools --scope foundation,go,rust,python,typescript --install --yes
```

**Why prettier stays in foundation**: It's our only Markdown formatter - every repo has README.md/docs. For TypeScript projects, biome handles TS/JS/JSON formatting (faster than prettier+eslint).

### Implementation Notes

Several cargo-deny integration quirks were discovered and documented:

1. **Argument order**: `--format json` must come BEFORE `check` subcommand
2. **STDERR output**: cargo-deny outputs JSON to stderr (intentional design)
3. **NDJSON format**: Output is newline-delimited JSON with nested `fields` object
4. **Informational codes**: "license-not-encountered" is informational, not a violation

See `pkg/dependencies/cargo_deny.go` for detailed comments.

### Bug Fixes

- **SSOT provenance trailing newline**: `goneat ssot sync` now writes provenance.json and metadata mirrors with trailing newlines, preventing format diffs when downstream repos run formatters after sync.

---

# Goneat v0.4.3 — Parallel Execution for Format & Schema Validation

**Release Date**: 2026-01-08
**Status**: Stable

## TL;DR

- **Parallel format**: `goneat format` now defaults to parallel execution for faster formatting
- **Parallel schema validation**: `goneat schema validate-schema --workers N` for parallel meta-validation
- **New validate-data command**: `goneat schema validate-data` for data file validation against schemas
- **Cached validators**: Meta-schema validators (draft-07, 2020-12) compiled once, reused across files

## What Changed

### Format Command

- **Default parallel execution**: The format command now uses `--strategy parallel` by default
- **Configurable workers**: Use `--workers N` to control parallelism (0=auto, 1=sequential, max 8)
- **Python/JS/TS support**: Parallel formatting now includes ruff (Python) and biome (JS/TS)

```bash
# Default parallel (auto-detects CPU count)
goneat format

# Explicit worker count
goneat format --workers 4

# Sequential (legacy behavior)
goneat format --strategy sequential
```

See [format command docs](docs/user-guide/commands/format.md) for configuration details.

### Schema Validation

- **Parallel validate-schema**: Meta-schema validation supports `--workers` flag
- **Cached validators**: Draft-07 and 2020-12 validators compiled once and reused
- **Deterministic output**: Results maintain input file order regardless of parallelism
- **New validate-data subcommand**: Validate data files against JSON schemas

```bash
# Parallel schema validation (auto workers)
goneat schema validate-schema --workers 0 schemas/

# Validate data against schema
goneat schema validate-data --schema schemas/config.json config.yaml
```

### Performance

Benchmarks confirm meaningful speedup for large file sets:

| Operation | Sequential | Parallel (4 workers) | Speedup |
|-----------|------------|---------------------|---------|
| Schema validation (776 files) | 6.77s | 3.97s | ~1.7x |
| Format (large codebase) | varies | varies | ~1.5-2x |

For small file sets (<200 files), parallelization overhead roughly equals gains—sequential is fine.

---

# Goneat v0.4.2 — Build & Dependencies Improvements

**Release Date**: 2026-01-03
**Status**: Stable

## TL;DR

- **Idempotent release doc embedding**: `docs/releases/latest.md` no longer regenerates on every build
- **Dependencies fix**: License detection no longer reports false "degraded" warnings
- **Format check fix**: `goneat format --check` correctly detects yamlfmt formatting issues
- **Rust lint**: `cargo-clippy` now runs under `assess --categories lint` when present
- **Doctor rust scope**: manual cargo install hints for `cargo-deny` and `cargo-audit`
- **Docs clarification**: Policy file requirement for license enforcement now prominently documented

## What Changed

### Build System

- **Idempotent release doc embedding**: `docs/releases/latest.md` no longer regenerates on every build; only updates when version-specific release notes change (see `scripts/embed-assets.sh`)

### Dependencies

- **Suppress stdlib noise**: License detection no longer reports "degraded" due to harmless go-licenses warnings about Go standard library packages lacking module info
- **Documentation**: Added prominent note to dependency-protection-overview.md clarifying that `.goneat/dependencies.yaml` policy file is **required** for license violation detection

### Format

- **Check mode fix**: `goneat format --check` now correctly reports files needing formatting when the primary formatter (e.g., yamlfmt for YAML) detects issues, even when the finalizer (EOF/whitespace normalization) says the file is OK. Previously, the finalizer result could incorrectly override the primary formatter's "needs formatting" status.

### Rust Tooling

- **Lint integration**: `cargo-clippy` runs as part of `goneat assess --categories lint` when available, mapping clippy warnings to medium severity.
- **Doctor scope**: `goneat doctor tools --scope rust` now surfaces manual install commands for `cargo-deny` and `cargo-audit` (cargo install with `--locked`).

---

**Previous Releases**: See `docs/releases/` for older release notes.
