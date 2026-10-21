# Goneat v0.6.6 — Go 1.26.10

**Release Date**: 2026-10-21
**Status**: Stable

## TL;DR

- **Release and CI toolchains are Go 1.26.10.** The module language line stays `go 1.26.0`.
- **Recommended Go is 1.26.10.** Push and release checks use that toolchain on `PATH`.
- **Standard library.** Go 1.26.10 includes the `crypto/x509` fix for CVE-2026-111943.
- **Direct modules.** etree v1.8.1, doublestar/v4 v4.10.3, go-runewidth v0.0.31, golang.org/x/mod v0.42.0, golang.org/x/sync v0.24.0, golang.org/x/sys v0.49.0, and golang.org/x/text v0.43.0.

## What Changed

- Workflow pins in CI and native release jobs are Go 1.26.10.
- The recommended Go version in the repository tool policy is 1.26.10.
- Published release binaries are built with `go1.26.10`.
- CI runner images stay fulmen-toolbox v0.5.10. Those images still carry Go 1.26.9. CI selects Go 1.26.10 with `GOTOOLCHAIN` and `actions/setup-go`.
- Direct modules are etree v1.8.1, doublestar/v4 v4.10.3, go-runewidth v0.0.31, golang.org/x/mod v0.42.0, golang.org/x/sync v0.24.0, golang.org/x/sys v0.49.0, and golang.org/x/text v0.43.0. golang.org/x/mod v0.42.0 selects golang.org/x/tools v0.51.0. go-runewidth v0.0.31 measures regional-indicator flag emoji wider in `ascii` boxes.

## Go 1.27

gosec v2.29.0 still cannot read Go 1.27 package data. The workarounds in the [v0.6.3 notes](https://github.com/fulmenhq/goneat/blob/v0.6.6/docs/releases/v0.6.3.md#go-127) still apply. This release does not move the reference toolchain to Go 1.27.

## Upgrading

Building from source or with `go install` requires **Go 1.26 or later**. Release workflow pins are Go 1.26.10.

v0.6.6 release archives are Darwin ARM64, Linux amd64, Linux ARM64, Windows amd64, and Windows ARM64. Darwin amd64 downloads end at v0.6.1. Verify downloads as described in the [v0.6.4 notes](https://github.com/fulmenhq/goneat/blob/v0.6.6/docs/releases/v0.6.4.md#verifying-this-release), using `RELEASE_VERSION="v0.6.6"`.
