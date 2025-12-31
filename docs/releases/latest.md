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

See [assess command reference](../user-guide/commands/assess.md) for detailed options.

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

# Goneat v0.3.24 — Offline Canonical ID Lookup + Spec-Host CI Guidance

**Release Date**: 2025-12-23
**Status**: Stable

## TL;DR

- **Canonical ID mode (offline-first)**: resolve URL `schema_id` values from `--ref-dir` with `--schema-resolution id-strict`
- **Scalable schema validation**: `validate suite` supports canonical URL IDs without network
- **CI guidance**: dual-run strategy (offline strict pre-deploy + post-deploy spec-host probe)

---

**Previous Releases**: See `docs/releases/` for older release notes.
