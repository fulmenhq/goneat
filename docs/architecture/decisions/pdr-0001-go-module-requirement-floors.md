---
title: "Go Module Requirement Floors Survive go mod tidy"
description: "Indirect requirements kept only to raise a module's minimum version are recorded here and restored after any go mod tidy"
author: "goneat contributors"
date: "2026-10-12"
last_updated: "2026-10-12"
status: "approved"
tags:
  - "dependencies"
  - "go-modules"
  - "process"
category: "dependencies"
---

# PDR-0001: Go Module Requirement Floors Survive `go mod tidy`

## Status

**APPROVED** - Requirement floors in `go.mod` are deliberate and are kept when the module graph is tidied.

## Context

Go selects the highest version of each module that any `go.mod` in the module graph requires. A dependency's `go.mod` can require an old version of a module that goneat never compiles. That old version still appears in the module graph.

Module-graph scanners report advisories against the versions selected in that graph. This includes `goneat dependencies --vuln` with a `go-module-graph` source. `govulncheck` does not report them, because it only analyzes code in the build.

A **requirement floor** is an `// indirect` requirement in goneat's `go.mod` that raises such a module to a version without the advisory. Nothing in the build imports a floor module, so `go mod tidy` removes it. The old version is then selected again, and the module-graph findings return.

## Decision

1. A requirement floor is allowed when a module-graph scan reports an advisory against a version that is not in the build list. The floor is the lowest version that clears the advisory and passes the cooling policy.
2. Every floor is listed in the table below. Adding or removing a floor updates this table in the same change.
3. After `go mod tidy`, restore any floor that it removed. Do not commit a tidy that drops a listed floor.
4. A floor is removed when the module graph no longer selects the vulnerable version without it. To check, delete the floor, run `go mod tidy`, and run `goneat dependencies --vuln`. The findings count must not increase.

### Current floors

| Requirement                        | Old version otherwise selected       | Required by              | Advisory            |
| ---------------------------------- | ------------------------------------ | ------------------------ | ------------------- |
| `golang.org/x/oauth2 v0.27.0`      | `v0.0.0-20180821212333-d2e6202438be` | `google.golang.org/grpc` | GHSA-6v2p-p543-phr9 |
| `github.com/golang/glog v1.2.4`    | `v0.0.0-20160126235308-23def4e6c14b` | `google.golang.org/grpc` | GHSA-6wxm-mpqj-6jpf |
| `github.com/yuin/goldmark v1.7.17` | `v1.4.13`                            | `golang.org/x/tools`     | GO-2026-5320        |

`go.mod` also lists `cloud.google.com/go` and `cloud.google.com/go/compute/metadata` as `// indirect`. They are not floors. They are requirements of the floor modules that Go records because of module graph pruning. They stay while the floors stay and leave with them.

Tidy also promotes modules that goneat imports directly, such as `golang.org/x/sys`, to the direct `require` block. Keep those changes.

### Checking a tidy

```bash
GOTOOLCHAIN=go1.26.10 go mod tidy -diff
```

A `-` line for a requirement in the table, or for one of its recorded requirements, is expected. Every other line should be applied.

## Consequences

- `go mod tidy -diff` is not empty while floors exist. A tidy check in CI would have to allow the listed floors.
- Floors are dependency changes. They go through the cooling policy and dependency review like any other version bump.
- `go mod tidy` drops comments attached to removed requirements. Explanations live in this record, not in `go.mod`.

## References

- [Go Modules Reference: Minimal version selection](https://go.dev/ref/mod#minimal-version-selection)
- [Go Modules Reference: go mod tidy](https://go.dev/ref/mod#go-mod-tidy)
- [Dependency protection architecture](../dependency-protection-architecture.md)
