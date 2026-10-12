# Goneat v0.6.3 — Security Scanner Fixes, Faster Startup, and Go 1.27

**Release Date**: 2026-10-12
**Status**: Stable

## TL;DR

- **Non-Go repositories pass the security category again.** gosec and govulncheck run only when the project contains a Go module. In Rust, TypeScript and Python repositories, v0.6.2 ran them and failed the category.
- **Stamped govulncheck builds are accepted.** Released govulncheck builds from v1.1.4 up to, but not including, v1.10.0 classify GO-2026-5932 as inventory. v0.6.2 accepted only unstamped `v0.0.0` builds, so Homebrew and `go install` builds failed the gate.
- **Pre-commit hooks pass for mixed changes.** A staged change to Go code together with documentation in other directories no longer fails govulncheck.
- **Faster startup.** Embedded schemas compile on first use. `validate data --schema-file` drops from about 0.9 s to about 0.2 s per call.
- **Go 1.27.** goneat builds, tests and runs govulncheck with Go 1.27. The gosec v2.29.0 release cannot read Go 1.27 package data; goneat names the cause and the remedies. See the Go 1.27 section below.

## What Changed

### Security category

- gosec and govulncheck run only when the project contains a `go.mod` outside ignored paths. Otherwise they are reported as not applicable, as cargo-audit and cargo-deny already are when there is no `Cargo.toml`.
- Running `assess` from a subdirectory of a Go module finds the module root. govulncheck previously failed with no module found. govulncheck findings report the absolute path of `go.mod`.
- Scoped govulncheck runs, including the pre-commit hook, omit selected directories without Go files. When no selected directory contains Go files, govulncheck scans the module.
- When the security category is skipped, the reason no longer says tools were missing from `PATH` when they did not apply.

### govulncheck inventory classification

- GO-2026-5932 (`golang.org/x/crypto/openpgp`, no fix available, module-level) is classified as inventory for govulncheck `v0.0.0` or a canonical release from v1.1.4 up to, but not including, v1.10.0. Later releases fail closed until they are checked.
- When the classification does not apply, the error names the first condition that did not hold.

### gosec and newer Go toolchains

- When gosec cannot read package data from the active Go toolchain, the security category names the gosec version, the `golang.org/x/tools` version it was built with, and the remedies. The category still fails.

### Startup

- Embedded schemas compile on first use instead of at process start. Every command previously compiled all bundled schemas during package initialization, about 0.7 s.

### Dependencies

- `github.com/go-git/go-git/v5` v5.19.3, `github.com/go-git/go-billy/v5` v5.9.2, `github.com/ProtonMail/go-crypto` v1.5.2, and `github.com/pjbgf/sha1cd` v0.7.0.
- `github.com/klauspost/cpuid/v2` is no longer required.
- `golang.org/x/sys` moves from indirect to direct.
- [PDR-0001](https://github.com/fulmenhq/goneat/blob/v0.6.3/docs/architecture/decisions/pdr-0001-go-module-requirement-floors.md) records the indirect requirements kept as version floors and how to keep them through `go mod tidy`.

## Go 1.27

goneat builds, tests and runs govulncheck with Go 1.27. The gosec v2.29.0 release, including the Homebrew build, uses `golang.org/x/tools` v0.49.0 and cannot read Go 1.27 package data. Until a gosec release supports Go 1.27, use one of these:

```bash
# Keep the gosec release and run under Go 1.26
GOTOOLCHAIN=go1.26.9 goneat assess

# Or install an unreleased gosec commit built with golang.org/x/tools v0.50.0
go install github.com/securego/gosec/v2/cmd/gosec@v2.29.1-0.20260930122349-9b318063d35c
```

The [Go toolchain guide](https://github.com/fulmenhq/goneat/blob/v0.6.3/docs/user-guide/toolchains/go.md#newer-go-toolchains) has the details.

## Upgrading

Building from source or with `go install` requires **Go 1.26 or later**. Release workflow pins are Go 1.26.9.

The pre-push assess uses `go` from `PATH` and does not set `GOTOOLCHAIN`. To match CI, push with Go 1.26.9 on `PATH` and `GOTOOLCHAIN=go1.26.9`.

v0.6.3 release archives are Darwin ARM64, Linux amd64, Linux ARM64, Windows amd64, and Windows ARM64. Darwin amd64 downloads end at v0.6.1.
