# Goneat v0.6.7 — Bootstrap Resilience and Tool Pins

**Release Date**: 2026-10-29
**Status**: Stable

## TL;DR

- **`make bootstrap` keeps going when Homebrew auto-install fails.** Non-brew installers still run, brew-only tools are reported as unavailable with a Homebrew hint, and report-only tools do not fail the run.
- **yq is pinned to mikefarah/yq.** A distro `yq` from a different project no longer satisfies detection.
- **Linux minisign is report-only.** goneat prints the Ubuntu/Debian and Alpine install commands instead of running sudo.
- **CI runner images are fulmen-toolbox v0.5.14.**
- **Security.** `golang.org/x/net` v0.62.0 fixes GO-2026-6860 and GO-2026-6866. It is adopted as a cooling-period exception. GO-2026-5932 has no fix and stays allowlisted. govulncheck v1.9.0 reports all three as unreachable.

## What Changed

### Doctor and bootstrap

- When user-local Homebrew auto-install fails, `make bootstrap` continues. Installers other than brew that were already available, such as `go-install`, still run. A required tool that cannot be installed another way still fails with an actionable message.
- The install prompt shows `using: <command>` before `Install X now? [y/N]`.
- A policy-compliant `$GOBIN` binary does not count as a pass unless it is first on `PATH`.
- After a successful `go-install`, doctor reports the version of the managed binary and prints a `PATH`-order hint.
- This release does not add uv, pip or npm installer kinds.

### Tool pins

- yq is pinned to `github.com/mikefarah/yq/v4@v4.53.3` through `go-install`. brew, scoop and winget remain fallbacks.
- On Linux, minisign is report-only. goneat prints the Ubuntu/Debian (`apt-get`) and Alpine (`apk`) commands. Darwin (brew) and Windows (scoop) installs are unchanged.
- Offline tests fail if the yq pin or the minisign instructions are reverted.

### CI

- Hosted CI and native-contract smoke jobs use fulmen-toolbox `goneat-tools-runner` v0.5.14, glibc and musl. A test requires every runner image tag in the workflows to match one expected tag.

## Security

govulncheck v1.9.0, run with Go 1.26.10, reports no vulnerabilities in code goneat calls or packages it imports. Three advisories in required modules are unreachable:

| Advisory | Module | Status |
| --- | --- | --- |
| GO-2026-6860 | `golang.org/x/net` | Fixed by the `golang.org/x/net` v0.62.0 bump in this release |
| GO-2026-6866 | `golang.org/x/net` | Fixed by the `golang.org/x/net` v0.62.0 bump in this release |
| GO-2026-5932 | `golang.org/x/crypto` | No fix; allowlisted |

`golang.org/x/net` moves from v0.60.0 to v0.62.0. v0.62.0 is still inside the cooling period, which ends 2026-11-03, so `.goneat/dependencies.yaml` carries a cooling exception for `golang.org/x/net` that expires on 2026-11-03. The bump also selects `golang.org/x/crypto` v0.58.0 and `golang.org/x/term` v0.47.0.

GO-2026-5932 covers `golang.org/x/crypto/openpgp`, which is unmaintained. goneat uses the ProtonMail fork, and no fixed version exists. It is allowlisted in `.goneat/dependencies.yaml`.

## Go 1.27

gosec installed from a package manager reports processing errors under the Go 1.27 toolchain. The fixes are on gosec main and are not in a release yet. Run with `GOTOOLCHAIN=go1.26.10` until a gosec release includes them:

```bash
GOTOOLCHAIN=go1.26.10 goneat assess
```

This release does not move the reference toolchain to Go 1.27.

## Upgrading

Building from source or with `go install` requires **Go 1.26.10 or later**.

v0.6.7 release archives are Darwin ARM64, Linux amd64, Linux ARM64, Windows amd64, and Windows ARM64. Darwin amd64 downloads end at v0.6.1. Verify downloads as described in the [v0.6.4 notes](https://github.com/fulmenhq/goneat/blob/v0.6.7/docs/releases/v0.6.4.md#verifying-this-release), using `RELEASE_VERSION="v0.6.7"`.
