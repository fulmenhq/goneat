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
