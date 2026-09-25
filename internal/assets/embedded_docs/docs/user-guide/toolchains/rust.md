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

| Tool           | Category              | Install                        |
| -------------- | --------------------- | ------------------------------ |
| `rustfmt`      | format                | `rustup component add rustfmt` |
| `cargo-clippy` | lint                  | `rustup component add clippy`  |
| `cargo-deny`   | dependencies, license | `cargo install cargo-deny`     |
| `cargo-audit`  | security              | `cargo install cargo-audit`    |

```bash
goneat doctor tools --scope rust --install --yes
```

## Format

goneat runs `cargo fmt --all` for the Cargo workspace that contains the target.
rustfmt reads `rustfmt.toml` / `.rustfmt.toml` and the crate edition as usual.

```bash
goneat format                             # rewrite: cargo fmt --all
goneat format --check                     # check: cargo fmt --all -- --check -l
goneat assess --categories format         # report one issue per unformatted file
```

- **Scope**: discovery runs cover the Cargo project containing each path. With
  `--files` or `--staged-only`, Rust is in scope only when a selected file ends
  in `.rs`. Selected `.rs` files are never given to the per-file formatter:
  inside a Cargo project they are formatted by `cargo fmt`, and otherwise
  (Rust disabled, excluded by `--types`, or no Cargo project) they are skipped
  with a log line. `--types` excludes Rust unless it lists `rust`.
- **Whole workspace**: `cargo fmt` always formats the whole workspace. When a file
  subset is selected in fix mode, goneat warns that other files may change.
  Check mode reports only the selected files.
- **Failures**: a parse error, manifest error or `rustfmt.toml` error fails the
  run. rustfmt `Warning:` lines (for example nightly-only options on stable) do not.
- **Missing rustfmt**: when Rust is in scope and cargo or rustfmt is missing,
  both `goneat format` and `goneat assess` fail rather than pass unchecked Rust.
  Opt out with `format.rust.enabled: false`, or with `--ignore-missing-tools` on
  `goneat format`. A toolchain named in config that lacks rustfmt always fails;
  `--ignore-missing-tools` does not cover it.

Configure in the project `.goneat.yaml`, which is used by both `goneat format`
and `goneat assess --categories format`. `goneat format` reads it from the working
directory; `goneat assess <target>` reads it from the target directory.

```yaml
format:
  rust:
    enabled: true # default true; false skips cargo fmt
    toolchain: stable # optional: cargo +stable fmt (must already be installed)
```

### Common Findings

| Finding                               | Meaning                        | Fix                                |
| ------------------------------------- | ------------------------------ | ---------------------------------- |
| "Rust file not formatted (cargo fmt)" | rustfmt would rewrite the file | Run `goneat format` or `cargo fmt` |

## Lint

goneat runs `cargo clippy --message-format=json` (plus `--workspace` for
workspaces) and maps clippy warnings to medium and errors to high severity.
Use `--fail-on medium` to gate on warnings; `-D warnings` is not needed.

```bash
goneat assess --categories lint --fail-on medium
```

**Fail-closed**: if Cargo exits non-zero, the lint category fails even when
some diagnostics were parsed. Compilation errors appear as high-severity issues;
failures before a lint run completes (manifest or dependency resolution errors,
build-script failures, missing toolchain or target) are reported with the tail of
Cargo's stderr. Diagnostics parsed before the failure are kept.

### Clippy invocation (`.goneat/assess.yaml`)

All fields are optional; unset fields keep the default invocation.

```yaml
version: 1
lint:
  rust:
    clippy:
      enabled: true # default true
      toolchain: stable # cargo +stable clippy (must already be installed)
      all_targets: true # --all-targets (tests, benches, examples)
      all_features: false # --all-features
      features: [async] # --features async
      no_default_features: false # --no-default-features
      locked: true # --locked
      packages: [] # -p <pkg> each; empty = whole workspace
      targets: [] # one run per triple, duplicate findings merged
```

- Values are passed to Cargo as separate arguments, never through a shell.
  A value that does not look like the Cargo token it names (for example one that
  starts with `-`) is rejected and the lint category fails.
- goneat never installs toolchains or targets. rustup auto-install is disabled
  for these runs; a missing toolchain or target standard library fails with a
  `rustup` hint. Install them in your bootstrap step.
- There is no free-form command string.

Example: a stable CI lint over all targets and features, and a cross-target lint
of selected crates for Linux and Windows (one configuration per CI job):

```yaml
# Host job
version: 1
lint:
  rust:
    clippy:
      toolchain: stable
      all_targets: true
      all_features: true
```

```yaml
# Cross-target job (install targets first with rustup target add)
version: 1
lint:
  rust:
    clippy:
      toolchain: stable
      locked: true
      packages: [mycrate-transport, mycrate-frame, mycrate-peer]
      all_targets: true
      features: [async]
      targets: [x86_64-unknown-linux-gnu, aarch64-unknown-linux-gnu, x86_64-pc-windows-msvc]
```

A top-level `rust:` block in `.goneat/assess.yaml` is not supported. goneat
warns and ignores it; other sections in the file still apply.

### Clippy rule configuration

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

| Rule                    | Meaning                                                                       |
| ----------------------- | ----------------------------------------------------------------------------- |
| `clippy::clone_on_copy` | Using `.clone()` on a type that implements `Copy`.                            |
| `clippy::unwrap_used`   | Discourages `.unwrap()` in production code in favor of proper error handling. |

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

**Workspace projects**: goneat discovers Rust projects by `Cargo.toml` presence,
including a `Cargo.toml` in a parent directory. In workspaces, goneat operates on
the workspace root; use `lint.rust.clippy.packages` to lint selected members.

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
