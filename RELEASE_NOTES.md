# Goneat v0.6.9 — Go and Scanner Updates

**Release Date**: 2026-11-11
**Status**: Stable

## What Changed

- **Go 1.26.11** is the module minimum, repository toolchain minimum, CI pin and release build reference. Foundation tool defaults recommend 1.26.11 and retain their generic Go 1.21.0 minimum.
- **Syft 1.57.1 and Grype 0.121.3** are the minimum and recommended SBOM tool versions in both repository configuration and embedded defaults.
- **gosec 2.30.0** is the recommended version and cloud bootstrap pin. The interim unreleased Go 1.27 workaround is retired. The minimum gosec version remains 2.18.0.
- **CI runner images** use digest-pinned fulmen-toolbox v0.5.15 references.

## Security

The selected module graph requires `github.com/klauspost/compress v1.20.2`, the fixed version for [GO-2026-6865 / GHSA-fpmh-frq8-7rj8](https://github.com/klauspost/compress/security/advisories/GHSA-fpmh-frq8-7rj8). The requirement floor is recorded in [PDR-0001](https://github.com/fulmenhq/goneat/blob/v0.6.9/docs/architecture/decisions/pdr-0001-go-module-requirement-floors.md).

The existing GO-2026-5932 not-imported exception is unchanged. It covers `golang.org/x/crypto/openpgp`; goneat uses the ProtonMail fork and does not import that package. No vulnerability suppression is added in this release.

## Known Issues

- The vendored Crucible schema `schemas/crucible-go/config/standards/v1.0.0/adr-lifecycle-status.json` is not a valid Draft 2020-12 schema. The correction belongs upstream in Crucible and will be synced in a later release.
- A Go registry lookup can report `age_unknown` for `github.com/ProtonMail/go-crypto`. That flag means publication age has not been verified by the analyzer; a fallback age is not verified release metadata.

## Upgrading

Building from source or with `go install` requires **Go 1.26.11 or later**. Use `GOTOOLCHAIN=go1.26.11` for release builds and tests. Go 1.27 changes Unicode behavior and remains outside the qualified release toolchain.

Update the SBOM tools to meet the new minimums:

```bash
goneat doctor tools --scope sbom --upgrade --yes
```

gosec 2.30.0 supports Go 1.27 package data; that analyzer support does not change goneat's release compiler reference. See [Go toolchain guidance](https://github.com/fulmenhq/goneat/blob/v0.6.9/docs/user-guide/toolchains/go.md).

v0.6.9 release archives are Darwin ARM64, Linux amd64, Linux ARM64, Windows amd64, and Windows ARM64. Darwin amd64 downloads end at v0.6.1. Verify downloads as described in the [v0.6.4 notes](https://github.com/fulmenhq/goneat/blob/v0.6.9/docs/releases/v0.6.4.md#verifying-this-release), using `RELEASE_VERSION="v0.6.9"`.
