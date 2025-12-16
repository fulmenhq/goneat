# Goneat v0.3.21 — Dependencies Reliability + Tool Cooling Metadata

**Release Date**: 2025-12-15
**Status**: Draft

## TL;DR

- **Go 1.25 compatibility**: `goneat dependencies` no longer fails on stdlib “module info” errors
- **Better cooling checks**: `goneat doctor tools` can evaluate cooling for more tools (fewer "metadata unavailable" failures)
- **Shell lint compatibility**: shfmt lint can match repo style via `lint.shell.shfmt.args`
- **Repo lint debt cleanup**: checkmake backlog cleared without raising thresholds

## What Changed

### Dependencies: Go 1.25 stdlib module-info failures fixed

Some Go 1.25.x environments hit repeated stdlib errors originating from go-licenses:

- `Package <stdlib> does not have module info. Non go modules projects are no longer supported.`

v0.3.21 decouples dependency discovery from go-licenses:

- Cooling/policy module discovery uses `go list -deps -json` and skips stdlib packages.
- go-licenses runs only when `--licenses` is requested.
- If license extraction is degraded, goneat falls back to best-effort license file detection from module directories.

### Doctor tools: cooling metadata for more install types

`goneat doctor tools` now resolves upstream metadata for more tools:

- GitHub repo inference for `kind: go` tools via `install_package`
- PyPI metadata for tools installed via uv/pip (e.g. yamllint)

### Lint: shfmt style args override

Forge repos commonly standardize `shfmt` flags (indentation, continuation indentation). v0.3.21 adds an opt-in override so `goneat assess --categories lint` can apply the same style:

- `.goneat/assess.yaml`: `lint.shell.shfmt.args: ["-i", "4", "-ci"]`
- goneat still controls `-d`/`-w` (check vs fix mode)

### Lint: Makefile checkmake backlog cleared

- Repo config sets `lint.make.checkmake.config.max_body_length: 15`
- Refactored large Make targets into helper targets to stay within limit

---

# Goneat v0.3.20 — NOTICE for Distribution

**Release Date**: 2025-12-14
**Status**: Release

## TL;DR

- **NOTICE shipped in archives**: Release archives include `LICENSE` and `NOTICE`
- **Documented libc policy**: ADR added for the Linux `CGO_ENABLED=0` musl/glibc compatibility decision
- **Docs polish**: `assess` docs explicitly call out checkmake override keys

## What Changed

### NOTICE + packaging

- Added a top-level `NOTICE` file.
- Release archives include `LICENSE` and `NOTICE` alongside the `goneat` binary.

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

**Previous Releases**: See `docs/releases/` for older release notes.
