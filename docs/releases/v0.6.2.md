# Goneat v0.6.2 — Toolchain, Module Selection, and Release Archives

**Release Date**: 2026-10-09
**Status**: Stable

## TL;DR

- **Workflow toolchain pins are Go 1.26.9.** The module language line stays `go 1.26.0`.
- **Selected modules** are `golang.org/x/net` v0.60.0, `golang.org/x/crypto` v0.57.0, `golang.org/x/sys` v0.48.0, and `golang.org/x/text` v0.42.0. Selected `golang.org/x/term` changes from v0.45.0 to v0.46.0. `go.sum` retains the v0.45.0 checksums and includes the v0.46.0 go.mod checksum.
- **Indirect requirements added**: `cloud.google.com/go` v0.26.0, `cloud.google.com/go/compute/metadata` v0.3.0, `github.com/golang/glog` v1.2.4, `github.com/yuin/goldmark` v1.7.17, and `golang.org/x/oauth2` v0.27.0. `golang.org/x/mod` v0.41.0 moves from indirect to direct.
- The cooling policy exception for golang.org/x/net is in effect through 2026-10-16T00:00:00Z.
- While the cooling window is active, `scripts/check-cooling-selection.py` requires the selected `golang.org/x/net` module to be v0.60.0 and indirect.
- **Assessments retain scanner failures**, release-asset results, and hook diagnostics. Native command output is captured as bytes.
- **v0.6.2 archives** are Darwin ARM64, Linux amd64, Linux ARM64, Windows amd64, and Windows ARM64. There is no Darwin amd64 archive.

## What Changed

### Build and toolchain

- Workflow toolchain pins are Go 1.26.9.
- `go.mod` stays at `go 1.26.0`.

### Selected Go modules

- `golang.org/x/net` v0.60.0
- `golang.org/x/crypto` v0.57.0
- `golang.org/x/sys` v0.48.0
- `golang.org/x/text` v0.42.0
- Selected `golang.org/x/term` changes from v0.45.0 to v0.46.0. `go.sum` retains the v0.45.0 checksums and includes the v0.46.0 go.mod checksum. `go.mod` does not require `golang.org/x/term` directly.
- Indirect requirements added: `cloud.google.com/go` v0.26.0, `cloud.google.com/go/compute/metadata` v0.3.0, `github.com/golang/glog` v1.2.4, `github.com/yuin/goldmark` v1.7.17, and `golang.org/x/oauth2` v0.27.0.
- `golang.org/x/mod` v0.41.0 moves from indirect to direct.

### Cooling selection

- The cooling policy exception for golang.org/x/net is in effect through 2026-10-16T00:00:00Z.

### Cooling selection check

- While the cooling window is active, `scripts/check-cooling-selection.py` requires the selected `golang.org/x/net` module to be v0.60.0 and indirect.

### Scanner execution

- Assessments retain scanner failures, release-asset results, and hook diagnostics.
- Native command output is captured as bytes.

### Release archives

v0.6.2 release archives:

- Darwin ARM64
- Linux amd64
- Linux ARM64
- Windows amd64
- Windows ARM64, as a native executable

Darwin amd64 downloads end at v0.6.1. v0.6.2 does not provide a Darwin amd64 archive.

## Upgrading

Building from source or with `go install` requires **Go 1.26 or later**. Release workflow pins are Go 1.26.9.

Install the archive that matches the machine. Intel macOS keeps the v0.6.1 Darwin amd64 archive. v0.6.2 does not publish a replacement for it.
