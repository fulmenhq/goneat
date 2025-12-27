title: "Linux Release Artifacts: CGO Disabled for musl/glibc Compatibility"
description: "Ship Linux release binaries with CGO_ENABLED=0 to run on both musl (Alpine) and glibc (Debian/Ubuntu)"
author: "@forge-neat"
date: "2025-12-14"
last_updated: "2025-12-14"
status: "approved"
tags:

- "infrastructure"
- "release"
- "linux"
- "musl"
- "glibc"
- "cgo"
  category: "infrastructure"

---

# ADR-0003: Linux Release Artifacts - CGO Disabled for musl/glibc Compatibility

## Status

**APPROVED** - Implemented in v0.3.19

## Context

Goneat is distributed as signed release artifacts and is commonly installed into CI containers.

A user attempted to install a Linux release binary inside an Alpine-based container (musl libc) and hit runtime loader/relocation failures typical of glibc-linked binaries:

- `__vfprintf_chk: symbol not found`
- `__fprintf_chk: symbol not found`

This occurs when a Linux binary is dynamically linked against glibc (or expects the glibc loader), and is then executed in a musl-based environment.

### Goals

- Provide a single Linux artifact that runs in both musl and glibc environments.
- Avoid requiring consumers to choose an image base (Alpine vs Debian) to run goneat.
- Prove the compatibility claim continuously in CI.

### Constraints

- Goneat should remain a portable DX tool with minimal runtime dependencies.
- Release pipeline must remain deterministic and verifiable (checksums + signatures).

## Decision

We will build Linux release artifacts with `CGO_ENABLED=0`.

### Detailed Description

- Linux targets (`linux/amd64`, `linux/arm64`) are built with `CGO_ENABLED=0`.
- The release build script asserts the Linux binary is not dynamically linked.
- The release workflow runs a smoke test executing the Linux binary inside:
  - `alpine:3.21` (musl)
  - `debian:bookworm-slim` (glibc)

## Rationale

### Key Factors

1. **Portability**: CGO-disabled Go binaries do not depend on system libc at runtime.
2. **DX Consistency**: Users should not need to know which libc their container uses.
3. **Low operational burden**: This removes the need to publish separate musl/glibc variants for typical Go CLIs.
4. **Proven-by-CI**: Executing in both Alpine and Debian makes the guarantee concrete.

## Alternatives Considered

### Alternative 1: Switch the tooling container to Debian/Ubuntu (glibc)

**Pros**:

- Simple for the specific container image

**Cons**:

- Does not help users running goneat in other musl environments
- Increases base image size

**Rejected because**: it moves the burden to container selection instead of fixing artifact portability.

### Alternative 2: Publish separate `linux_musl_*` release assets

**Pros**:

- Works even if CGO is required

**Cons**:

- Doubles Linux artifact surface area (packaging, checksums, signatures)
- Bootstrappers must detect libc and select correct asset

**Rejected because**: CGO is not required for goneat’s current feature set.

### Alternative 3: Bundle/Install glibc into Alpine containers

**Pros**:

- Can run existing glibc-linked binaries

**Cons**:

- Operationally brittle and varies by environment

**Rejected because**: it increases complexity and weakens the portability story.

## Consequences

### Positive

- Linux release binaries run in both Alpine and Debian-based containers.
- Reduced installation friction for users and CI pipelines.
- Fewer moving parts than maintaining dual libc variants.

### Negative

- If a future dependency requires CGO, this decision may need revisiting.
  - **Mitigation**: Keep smoke tests (Alpine + Debian) and fail fast if linkage regresses.

## Implementation

### Changes Required

- `scripts/build-all.sh` builds Linux targets with `CGO_ENABLED=0` and enforces non-dynamic linkage.
- `.github/workflows/release.yml` runs musl+glibc smoke tests for the Linux artifact.

### Testing Strategy

- Linkage assertion during build (fail if dynamically linked).
- Runtime smoke tests inside Alpine and Debian containers.

## References

### Internal

- `scripts/build-all.sh`
- `.github/workflows/release.yml`
- `docs/releases/v0.3.19.md`

---

**Decision made by**: @3leapsdave
**Documented by**: @forge-neat
**Implementation**: v0.3.19
