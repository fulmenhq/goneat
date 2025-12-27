# Goneat v0.3.25 — Checkmake Makefile Discovery Fix

**Release Date**: 2025-12-27
**Status**: Draft

## TL;DR

- **Makefile linting works by default**: checkmake now reliably runs on root-level `Makefile` targets
- **Release upload homedir**: `make release-upload` honors `GONEAT_GPG_HOMEDIR`
- **Release upload homedir**: `make release-upload` honors `GONEAT_GPG_HOMEDIR`

## What Changed

### Lint: checkmake now discovers root `Makefile`

Previously, patterns like `**/Makefile` could fail to match a root-level `Makefile`, causing checkmake to silently skip.

v0.3.25 fixes Makefile discovery so:

- default Makefile lint paths include both `Makefile` and `**/Makefile`
- `**/Makefile` patterns now work reliably for root-level Makefiles

---

# Goneat v0.3.24 — Offline Canonical ID Lookup + Spec-Host CI Guidance

**Release Date**: 2025-12-23
**Status**: Draft

## TL;DR

- **Canonical ID mode (offline-first)**: resolve URL `schema_id` values from `--ref-dir` with `--schema-resolution id-strict`
- **Scalable schema validation**: `validate suite` supports canonical URL IDs (registry-like manifests) without network
- **CI guidance**: dual-run strategy (offline strict pre-deploy + post-deploy spec-host probe)
- **Crucible SSOT sync**: updated embedded Crucible docs/schemas/config to v0.2.27

---

# Goneat v0.3.23 — Bulk Validate Suite + Local Schema DX + Offline Refs

**Release Date**: 2025-12-21
**Status**: Draft

## TL;DR

- **Bulk validation**: `goneat validate suite` validates many YAML/JSON files in one run (parallel workers)
- **CI/AI friendly output**: `--format json` includes per-file results + summary
- **Local schema DX**: `schema_path` in `.goneat/schema-mappings.yaml` maps patterns directly to local schema files
- **Offline-first**: `--ref-dir` resolves absolute `$ref` URLs without a live schema registry
- **Release docs in CLI**: `goneat docs show release-notes` and `goneat docs show releases/latest`

---

**Previous Releases**: See `docs/releases/` for older release notes.
