---
title: "Rust Toolchain"
description: "How goneat handles Rust: rustfmt, cargo-clippy, cargo-deny, cargo-audit — edition notes, license expression handling, and common findings"
author: "goneat contributors"
date: "2026-02-26"
last_updated: "2026-02-26"
status: "published"
tags: ["rust", "rustfmt", "clippy", "cargo-deny", "cargo-audit", "toolchain"]
category: "user-guide"
---

# Rust Toolchain

goneat integrates with the Rust toolchain via rustfmt (format), clippy (lint),
cargo-deny (license and dependency policy), and cargo-audit (vulnerability scanning).
All Rust tools are installed via rustup or cargo.

## Tools

| Tool | Category | Install |
|------|----------|---------|
| `rustfmt` | format | `rustup component add rustfmt` |
| `cargo-clippy` | lint | `rustup component add clippy` |
| `cargo-deny` | dependencies, license | `cargo install cargo-deny` |
| `cargo-audit` | security | `cargo install cargo-audit` |

```bash
goneat doctor tools --scope rust --install --yes
```

## Format

goneat uses `rustfmt` (via `cargo fmt --check`) to verify Rust formatting.

```bash
goneat format                             # fix formatting
goneat assess --categories format         # check only
```

`goneat` invokes formatting either per-workspace or per-crate depending on the structure found. It is aware of Cargo editions (2018, 2021, 2024), delegating directly to `cargo fmt` which utilizes `rustfmt.toml` configurations implicitly.

### Common Findings

| Finding | Meaning | Fix |
|---------|---------|-----|
| "File needs formatting" | rustfmt would rewrite the file | Run `cargo fmt` or `goneat format` |

## Lint

goneat runs `cargo clippy` to surface lint findings. Clippy lints range from
style to correctness to performance.

```bash
goneat assess --categories lint
```

`goneat` parses the JSON message format emitted by `cargo clippy --message-format=json`. It intelligently maps Rust's warning/error levels into its own standard output.

### Configuration

Clippy can be configured per-crate in `Cargo.toml`:

```toml
[lints.clippy]
complexity = "warn"
pedantic = "allow"
```

Or with a `clippy.toml` / `.clippy.toml` in the project root for rule-level tuning. Suppressions in code use standard attributes:

```rust
#[allow(clippy::too_many_arguments)]
fn my_func(...) {}
```

### Common Findings

| Rule | Meaning |
|------|---------|
| `clippy::clone_on_copy` | Using `.clone()` on a type that implements `Copy`. |
| `clippy::unwrap_used` | Discourages `.unwrap()` in production code in favor of proper error handling. |

## Dependencies

### License Compliance

`cargo-deny` checks license compliance, banned crates, and version advisories.
goneat integrates with `cargo-deny` for the `--licenses` flag.

```bash
goneat dependencies --licenses
```

It parses your `deny.toml` file to check allowable licenses. `cargo-deny` robustly understands SPDX expressions (e.g., `MIT OR Apache-2.0`), which `goneat` normalizes and presents cleanly.

### Vulnerability Scanning

`cargo-audit` checks `Cargo.lock` against the RustSec advisory database.

While `goneat dependencies --vuln` uses `grype` for general SBOM scanning, `cargo-audit` offers a more deeply integrated approach specific to the Rust ecosystem. `cargo-audit` findings can be suppressed inside `cargo-audit.toml` or using inline cargo attributes, providing strict Rust security assertions.

## Known Behaviors and Edge Cases

**Workspace projects**: goneat discovers Rust projects by `Cargo.toml` presence.
In workspaces, goneat operates on the workspace root. Per-member linting is not
currently supported separately.

**cargo-deny output format**: cargo-deny writes diagnostics to stderr by design.
goneat reads from stderr for this tool. Rich output (crate names, license
names, deny.toml file:line references) was added in goneat v0.4.5.

**License expressions**: Rust crates commonly use SPDX expression syntax
(`MIT OR Apache-2.0`). goneat normalizes these for reporting alongside
Go and Python license data.

## See Also

- [`assess` command reference](../commands/assess.md)
- [`format` command reference](../commands/format.md)
- [`dependencies` command reference](../commands/dependencies.md)