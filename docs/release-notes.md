# Goneat v0.4.5 — Rust License Scanning Improvements

**Release Date**: 2026-01-13
**Status**: Stable

## TL;DR

- **Rich cargo-deny output**: Error messages now include specific license names, crate versions, and deny.toml file:line references
- **License enumeration for Rust**: `goneat dependencies --licenses` now lists all Rust dependencies with their licenses (like Go)
- **Unified implementation**: Single source of truth for cargo-deny integration (fixes stderr/NDJSON parsing bugs)

## What Changed

### Phase 1: Rich cargo-deny Output

Previously, cargo-deny output was generic:

```
cargo-deny: license: rejected, failing due to license requirements
```

Now it includes full context:

```
cargo-deny: license: rejected, failing due to license requirements [0BSD; unmatched license allowance; at deny.toml:53:6]
```

**What's included:**

| Context Type | Example |
|--------------|---------|
| Specific license name | `0BSD`, `GPL-3.0`, `Unlicense` |
| License action | `unmatched license allowance`, `rejected by policy` |
| deny.toml reference | `at deny.toml:53:6` |
| Crate version | `windows-sys v0.52.0` (for duplicate warnings) |

### Phase 2: License Enumeration for Rust

`goneat dependencies --licenses` now works identically for Go and Rust projects:

```bash
# Rust project
$ goneat dependencies --licenses --format json
{"Dependencies":[{"Name":"serde","Version":"1.0.215","Language":"rust","License":{"Name":"MIT OR Apache-2.0","Type":"MIT OR Apache-2.0"}},...]}
```

**Features:**
- Parses `cargo deny list` output (table format)
- Handles SPDX-like license expressions (`MIT OR Apache-2.0`)
- Same `Dependency` schema as Go analyzer
- Works in Cargo workspaces

### Bug Fixes

- **cargo-deny STDERR output**: Fixed reading from stderr (cargo-deny design)
- **Command order**: Fixed `--format json` positioning (must precede `check`)
- **Unified implementation**: Removed duplicate parsing code
- **Severity mapping**: "note"/"help" now correctly map to low severity

---

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

### Toolchain Scopes

Tools are now organized into language-specific scopes:

| Scope | Purpose | Key Tools |
|-------|---------|-----------|
| `foundation` | Language-agnostic | ripgrep, jq, yq, yamlfmt, prettier, yamllint, shfmt, shellcheck, actionlint, checkmake, minisign |
| `go` | Go development | go, go-licenses, golangci-lint, goimports, gofmt, gosec, govulncheck |
| `rust` | Rust Cargo plugins | cargo-deny, cargo-audit |
| `python` | Python tools | ruff (replaces black/flake8/isort) |
| `typescript` | TS/JS tools | biome (replaces eslint/prettier for TS/JS/JSON) |
| `sbom` | SBOM & vuln scanning | syft (SBOM generation), grype (vulnerability scanning) |

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

```bash
# Default parallel (auto-detects CPU count)
goneat format

# Explicit worker count
goneat format --workers 4
```

### Schema Validation

- **Parallel validate-schema**: Meta-schema validation supports `--workers` flag
- **Deterministic output**: Results maintain input file order regardless of parallelism
- **New validate-data subcommand**: Validate data files against JSON schemas

### Performance

| Operation | Sequential | Parallel (4 workers) | Speedup |
|-----------|------------|---------------------|---------|
| Schema validation (776 files) | 6.77s | 3.97s | ~1.7x |
| Format (large codebase) | varies | varies | ~1.5-2x |

---

**Previous Releases**: See `docs/releases/` for older release notes.
