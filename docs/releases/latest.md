# Goneat v0.4.5 — Rust License Scanning & Biome 2.x Compatibility

**Release Date**: 2026-01-13
**Status**: Stable

## TL;DR

- **Biome 2.x compatibility**: Format assessment updated for biome 2.x breaking changes (removed `--check` flag)
- **Rich cargo-deny output**: Error messages now include specific license names, crate versions, and deny.toml file:line references
- **License enumeration for Rust**: `goneat dependencies --licenses` now lists all Rust dependencies with their licenses (like Go)
- **Format assess fix mode**: Normalizes files when running `assess --categories format --fix`

## What Changed

### Biome 2.x Compatibility

Biome 2.x introduced breaking changes that affected goneat's format assessment:

- **Removed `--check` flag**: Biome 2.x uses exit codes instead of the `--check` flag
- **JSON diagnostics**: Now parses biome JSON output for reliable format issue detection
- **Respects ignore rules**: Properly honors `.biome.json` ignore configuration
- **Version requirement**: goneat now requires biome 2.x or higher

This fix eliminates false positive format issues in repos using biome 2.x.

### Format Assess Fix Mode

`assess --categories format --fix` now applies finalizer normalization:

- Adds EOF newlines where missing
- Removes trailing whitespace
- Consistent behavior with `goneat format` command

### Rich cargo-deny Output

Previously, cargo-deny output was generic:

```
cargo-deny: license: rejected, failing due to license requirements
```

Now it includes full context:

```
cargo-deny: license: rejected, failing due to license requirements [0BSD; unmatched license allowance; at deny.toml:53:6]
```

**What's included:**

| Context Type          | Example                                             |
| --------------------- | --------------------------------------------------- |
| Specific license name | `0BSD`, `GPL-3.0`, `Unlicense`                      |
| License action        | `unmatched license allowance`, `rejected by policy` |
| deny.toml reference   | `at deny.toml:53:6`                                 |
| Crate version         | `windows-sys v0.52.0` (for duplicate warnings)      |

This makes diagnosing license issues actionable without digging through cargo-deny's raw output.

### License Enumeration for Rust

`goneat dependencies --licenses` now works identically for Go and Rust projects:

**Go project:**

```bash
$ goneat dependencies --licenses --format json
{"Dependencies":[{"Name":"github.com/spf13/cobra","Version":"v1.8.1","Language":"go","License":{"Name":"Apache-2.0","Type":"Apache-2.0"}},...]}
```

**Rust project:**

```bash
$ goneat dependencies --licenses --format json
{"Dependencies":[{"Name":"serde","Version":"1.0.215","Language":"rust","License":{"Name":"MIT OR Apache-2.0","Type":"MIT OR Apache-2.0"}},...]}
```

**Features:**

- Parses `cargo deny list` output (license-grouped format: `MIT (89): crate@version, ...`)
- Handles SPDX-like license expressions (`MIT OR Apache-2.0`, `Unlicense OR MIT`)
- Same `Dependency` schema as Go analyzer
- Works in Cargo workspaces

### Bug Fixes

- **Biome 2.x false positives**: Fixed format assessment exit code interpretation
- **cargo-deny STDERR output**: Fixed reading from stderr (cargo-deny design)
- **Command order**: Fixed `--format json` positioning (must precede `check`)
- **Unified implementation**: Removed duplicate parsing code
- **Severity mapping**: "note"/"help" now correctly map to low severity

---

## Migration

No breaking changes. Existing integrations will automatically benefit from:

- Richer error messages (no code changes required)
- Dependency listing via `--licenses` flag (opt-in)
- Correct format assessment with biome 2.x

---

**Previous Releases**: See `docs/releases/` for older release notes.
