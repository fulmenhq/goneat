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

# Goneat v0.4.1 — Explicit Incremental Lint Checking

**Release Date**: 2026-01-02
**Status**: Stable

## TL;DR

- **New flags**: `--new-issues-only` and `--new-issues-base` for incremental lint checking
- **Behavior change**: Hook mode no longer implicitly applies incremental checking
- **Tool support**: golangci-lint and biome integration for incremental mode

## What Changed

### Explicit Incremental Lint Checking

New flags for `goneat assess` enable opt-in incremental lint checking:

| Flag | Default | Description |
|------|---------|-------------|
| `--new-issues-only` | `false` | Only report issues since base reference |
| `--new-issues-base` | `HEAD~` | Git reference for baseline comparison |

```bash
# Report only NEW lint issues since previous commit
goneat assess --categories lint --new-issues-only

# Report only NEW issues since main branch
goneat assess --categories lint --new-issues-only --new-issues-base main
```

### Tool Support

| Tool | Language | Native Flag |
|------|----------|-------------|
| golangci-lint | Go | `--new-from-rev REF` |
| biome | JS/TS | `--changed --since=REF` |

### Hook Mode Behavior Change

**Before v0.4.1**: Hook mode implicitly applied `--lint-new-from-rev HEAD~`, reporting only new issues.

**After v0.4.1**: Hook mode reports ALL lint issues by default (consistent with direct assess).

**To restore previous behavior**, add `--new-issues-only` to your `.goneat/hooks.yaml`:

```yaml
hooks:
  pre-commit:
    - command: assess
      args: ["--categories", "format,lint", "--fail-on", "high", "--new-issues-only"]
```

## Upgrade Notes

### From v0.4.0

**Behavioral change**: If your hooks relied on implicit incremental checking, you may see more lint issues after upgrading. This is intentional—incremental checking is now opt-in.

**Migration**: Add `--new-issues-only` to hooks.yaml if you want incremental behavior.

## Documentation

- **Appnote**: `docs/appnotes/assess/incremental-lint-checking.md`
- **Assess flags**: `docs/user-guide/commands/assess.md`
- **Hooks config**: `docs/user-guide/commands/hooks.md`

---

# Goneat v0.4.0 — Language-Aware Assessment for Python & JavaScript/TypeScript

**Release Date**: 2025-12-31
**Status**: Stable

## TL;DR

- **Python support**: lint and format via [ruff](https://docs.astral.sh/ruff/)
- **JavaScript/TypeScript support**: lint and format via [biome](https://biomejs.dev/)
- **Tool-present gating**: gracefully skip tools that aren't installed
- **Role-based agentic attribution**: simplified AI collaboration model

## What Changed

### Language-Aware Assessment

Goneat now provides **polyglot assessment** with automatic tool detection:

| Language               | Lint | Format | Tool   | Install              |
| ---------------------- | ---- | ------ | ------ | -------------------- |
| **Python**             | ✅   | ✅     | ruff   | `brew install ruff`  |
| **JavaScript/TypeScript** | ✅   | ✅     | biome  | `brew install biome` |

Tool-present gating: goneat gracefully skips tools that aren't installed.

```bash
goneat assess --categories lint,format
```

### Agentic Attribution v2

Migrated to role-based attribution (devlead, secrev, releng) per [3leaps crucible](https://crucible.3leaps.dev/).

---

# Goneat v0.3.25 — Checkmake Makefile Discovery Fix

**Release Date**: 2025-12-27
**Status**: Stable

## TL;DR

- **Makefile linting works by default**: checkmake now reliably runs on root-level `Makefile` targets
- **Release upload homedir**: `make release-upload` honors `GONEAT_GPG_HOMEDIR`

---

**Previous Releases**: See `docs/releases/` for older release notes.
