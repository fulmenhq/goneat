# Goneat v0.3.19 — Linux Release Artifacts (musl/glibc compatible)

**Release Date**: 2025-12-14
**Status**: Draft

## TL;DR

- **Alpine-compatible Linux binaries**: Linux release artifacts are built with `CGO_ENABLED=0` to avoid glibc-only linkage
- **Stronger CI proof**: Release workflow smoke tests run the Linux binary in both musl (Alpine) and glibc (Debian) containers

## What Changed

### Linux builds: CGO disabled

Linux release artifacts are now built with `CGO_ENABLED=0` to produce a binary that runs in both:

- glibc-based environments (Ubuntu/Debian)
- musl-based environments (Alpine)

This prevents runtime failures like:

- `Error relocating ...: __vfprintf_chk: symbol not found`

### Release workflow smoke test

The GitHub release workflow now executes `goneat version` inside:

- `alpine:3.21`
- `debian:bookworm-slim`

This makes the musl/glibc compatibility guarantee concrete.

---

# Goneat v0.3.18 — Checkmake Config Overrides (MVP)

**Release Date**: 2025-12-13
**Status**: Release

## TL;DR

- **Less Noise**: Configure checkmake rules via `.goneat/assess.yaml`
- **MVP Scope**: Supports `max_body_length` and `min_phony_targets`

## What Changed

### checkmake configuration

`.goneat/assess.yaml` supports generating a temporary checkmake config file and passing it via `checkmake --config`.

Supported settings:

- `lint.make.checkmake.config.max_body_length`
- `lint.make.checkmake.config.min_phony_targets`

---

# Goneat v0.3.17 — Unified Ignore Scope for Lint Sidecars

**Release Date**: 2025-12-13
**Status**: Release

## TL;DR

- **Less DRY + More Predictable**: Lint sidecars now respect `.gitignore` + `.goneatignore` by default
- **Force Include Works Everywhere**: `--force-include` applies consistently across lint integrations
- **Better Foundations for Future Sidecars**: Establishes scope contract for upcoming biome/ruff integrations

## What Changed

### Unified Ignore Scope

The glob-based lint integrations (shell, Makefile, GitHub Actions, YAML) previously relied on `.goneat/assess.yaml` exclude globs and could require duplicating ignore patterns that already exist in `.gitignore`/`.goneatignore`.

v0.3.17 makes path scope consistent:

- Files ignored by `.gitignore`/`.goneatignore` are not scanned by sidecars
- `goneat assess --force-include <glob>` can re-include ignored paths when needed
- `.goneat/assess.yaml` `ignore:` remains available as a tool-specific, additive exclusion layer

Affected integrations:

- shfmt / shellcheck
- actionlint
- checkmake
- yamllint target resolution

---

**Previous Releases**: See `docs/releases/` for older release notes.
