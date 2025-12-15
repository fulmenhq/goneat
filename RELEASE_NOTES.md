# Goneat v0.3.21 — Cooling Policy Metadata for Tools

**Release Date**: 2025-12-15
**Status**: Draft

## TL;DR

- **Better cooling checks**: `goneat doctor tools` can evaluate cooling for more tools (fewer "metadata unavailable" failures)
- **Go tool repo inference**: derive GitHub repo from `install_package` for GitHub-hosted Go tools
- **PyPI support**: fetch release publish dates from PyPI for tools installed via uv/pip (e.g. yamllint)

## What Changed

- Added a PyPI metadata fetcher for cooling policy evaluation.
- Added repo inference for Go-installed tools hosted on GitHub.

---

# Goneat v0.3.20 — NOTICE for Distribution

**Release Date**: 2025-12-14
**Status**: Draft

## TL;DR

- **NOTICE shipped in archives**: Release archives now include `LICENSE` and `NOTICE`
- **Documented libc policy**: ADR added for the Linux `CGO_ENABLED=0` musl/glibc compatibility decision
- **Docs polish**: `assess` docs now explicitly call out checkmake override keys

## What Changed

### NOTICE + packaging

- Added a top-level `NOTICE` file.
- Release archives now include `LICENSE` and `NOTICE` alongside the `goneat` binary.

### Documentation

- Documented checkmake config override keys supported via `.goneat/assess.yaml`:
  - `lint.make.checkmake.config.max_body_length`
  - `lint.make.checkmake.config.min_phony_targets`

---

# Goneat v0.3.19 — Linux Release Artifacts (musl/glibc compatible)

**Release Date**: 2025-12-14
**Status**: Release

## TL;DR

- **Alpine-compatible Linux binaries**: Linux release artifacts are built with `CGO_ENABLED=0` to avoid glibc-only linkage
- **Stronger CI proof**: Release workflow smoke tests run the Linux binary in both musl (Alpine) and glibc (Debian) containers

## What Changed

### Linux builds: CGO disabled

Linux release artifacts are built with `CGO_ENABLED=0` to produce a binary that runs in both:

- glibc-based environments (Ubuntu/Debian)
- musl-based environments (Alpine)

This prevents runtime failures like:

- `Error relocating ...: __vfprintf_chk: symbol not found`

### Release workflow smoke test

The GitHub release workflow executes `goneat version` inside:

- `alpine:3.21`
- `debian:bookworm-slim`

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

**Previous Releases**: See `docs/releases/` for older release notes.
