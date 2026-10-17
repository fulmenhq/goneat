# Goneat v0.6.5 — Rust License Inventory Matches cargo-deny

**Release Date**: 2026-10-17
**Status**: Stable

## TL;DR

- **Rust license checks no longer fail on clean workspaces.** `goneat dependencies --licenses` (and `--sbom`) reported "incomplete Rust license inventory" for workspaces that cargo-deny inventories without errors. goneat now builds the expected crate set the same way cargo-deny does.
- **Older crate license fields are accepted.** Slash-separated licenses such as `MIT/Apache-2.0` are compared as OR, as cargo-deny parses them.
- **Accurate omission messages.** The `include-dev` upgrade advice appears only for cargo-deny versions below 0.20 when every missing crate is dev-only.
- **Releases stay drafts until signed.** A release is published only after its signatures are uploaded and verified.
- **Scanner versions.** Syft 1.54.1 and Grype 0.120.1 are the minimum and recommended versions.

## What Changed

### Rust license inventory

goneat checks that every crate in the workspace's resolved dependency graph has license evidence from `cargo deny list`. It previously walked every resolved dependency edge in `cargo metadata`, including edges that cargo-deny correctly leaves out, and failed on the crates cargo-deny did not list. Workspaces depending on crates such as `uuid`, `time`, `zerocopy` or `rustls-pki-types` were affected.

The expected crate set is now built from the same metadata snapshot cargo-deny reads, following its rules:

- Features are resolved from the workspace members. An optional dependency counts only when an enabled feature turns it on. A weak `dep?/feature` reference does not turn it on.
- Dependencies declared only under a target expression that can never be true, such as `cfg(any())`, are skipped. Target expressions that name a platform are kept, as cargo-deny keeps them when no `[graph] targets` are configured.
- Each resolved dependency is matched to its manifest declaration by kind, target, name, version requirement and source. Version requirements follow Cargo's semver rules, including pre-release versions.
- `licenses.include-dev` and `licenses.include-build` remove crates that are reachable only through dev or build dependencies, without changing which features are enabled.

Configured `[graph] targets` and `[graph] exclude` remain unsupported and are reported as such. A `deny.toml` that leaves out dev-dependency crates (`include-dev = false`) is still rejected, because the license inventory would not cover the whole dependency graph. Set `licenses.include-dev = true` to use the check.

### License expressions

- When comparing a crate's `license` field with cargo-deny's license list, `/` is read as OR (`MIT/Apache-2.0`, `Apache-2.0 / MIT`). The reported license is the crate's declared expression.

### Omission messages

- A crate goneat expected but cargo-deny did not list is reported with the installed cargo-deny version and a request to report the issue.
- The `include-dev` upgrade advice appears only for cargo-deny versions below 0.20 when every missing crate is a dev-only dependency.

### Releases

- The release workflow creates the GitHub release as a draft. `release-assets.py upload` publishes it only after every asset and signature has been verified in place. A failed check leaves the release as a draft.

### Tools

- Syft 1.54.1 and Grype 0.120.1 are both the minimum and recommended versions in the repository tool policy and the embedded defaults.
- CI runner images are fulmen-toolbox v0.5.10.

## Go 1.27

gosec v2.29.0 still cannot read Go 1.27 package data. The workarounds in the [v0.6.3 notes](https://github.com/fulmenhq/goneat/blob/v0.6.5/docs/releases/v0.6.3.md#go-127) still apply.

## Upgrading

Building from source or with `go install` requires **Go 1.26 or later**. Release workflow pins are Go 1.26.9.

v0.6.5 release archives are Darwin ARM64, Linux amd64, Linux ARM64, Windows amd64, and Windows ARM64. Darwin amd64 downloads end at v0.6.1. Verify downloads as described in the [v0.6.4 notes](https://github.com/fulmenhq/goneat/blob/v0.6.5/docs/releases/v0.6.4.md#verifying-this-release), using `RELEASE_VERSION="v0.6.5"`.
