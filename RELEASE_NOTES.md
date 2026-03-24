# Goneat v0.5.9 — YAML Parity, Repo-Scoped gosec, and PR Workflow Refresh

**Release Date**: 2026-03-24
**Status**: Stable

## TL;DR

- **YAML format parity**: `goneat assess --categories format` now sees and fixes the same `yamlfmt` rewrites as `goneat format`
- **Repo-scoped gosec**: security assessment no longer lets `GOCACHE` or other out-of-repo Go build artifacts block releases
- **Maintainer workflow refresh**: goneat now uses a protected `main` + pull request flow, with `make pr-final` as the final merge-readiness check

## What Changed

### YAML Format / Assess Parity

`3leaps/sysprims` exposed a correctness gap in the format pipeline: `goneat format`
could rewrite YAML inline comments into a form that later failed strict
`yamllint`, while `goneat assess --categories format --check` still reported the
tree as format clean.

v0.5.9 closes that gap by routing YAML files through the shared format processor
during format assessment. In practice:

- `goneat format` and `goneat assess --categories format --fix` now apply the
  same `yamlfmt`-driven rewrites
- `goneat assess --categories format --check` now fails when the fix path would
  rewrite the file
- invalid YAML syntax still surfaces as a format error instead of being hidden

This restores the expected contract that developers can format first and then
trust the assess/check path used by hooks and CI.

### Repo-Scoped gosec Findings

The second `sysprims` blocker came from `gosec` findings emitted for files under
the Go build cache rather than the repository under assessment. The findings
followed `GOCACHE` when it was redirected, which showed the issue was scope
leakage rather than repo-local code.

v0.5.9 now filters security issues and suppressions back to the assessment root
before severity aggregation and `--fail-on` gating. That means:

- findings under `GOCACHE`, `go-build`, and similar external paths are dropped
- relative path escapes are rejected
- only repo-owned source files can block security gates

### Maintainer Workflow Refresh

goneat now operates with a protected `main` branch and pull-request-based merge
flow. This is a maintainer/workflow change rather than an end-user CLI feature,
but it is part of the v0.5.9 hardening work:

- `main` is PR-only with squash/rebase merges enabled
- `make pr-final` is available as the standard final merge-readiness target
- generated local hooks default away from guardian browser interception for the
  normal feature-branch workflow

### golangci-lint Tool Alignment

The advisory CI lint step and the recommended tool defaults are now aligned on
`golangci-lint` `2.11.2`, reducing surprise when contributors compare local tool
guidance with CI behavior.

## Upgrade Notes

Drop-in replacement for v0.5.8. No config migration required.

## Contributors

- GPT-5.4 (devlead)
- @3leapsdave (supervision)

---

# Goneat v0.5.8 — Windows Portability, Dependency Remediation & Non-Go Lint Fix

**Release Date**: 2026-03-13
**Status**: Stable

## TL;DR

- **Windows `--output /dev/null`**: All output commands now handle both `/dev/null` (Unix) and `NUL` (Windows) transparently
- **Dependency vulnerabilities**: 3 Go module bumps + 2 GitHub Actions pins eliminate critical/high CVEs
- **Non-Go project lint**: golangci-lint no longer fails in Rust (or other non-Go) projects that have Go bindings in subdirectories

## What Changed

### Cross-Platform Null Device Handling

Running `goneat assess --output /dev/null` on Windows native cmd/PowerShell produced a file named `dev\null` instead of discarding output. All four `--output`-bearing commands (`assess`, `validate`, `security`, `dependencies`) now detect null device paths via `pkg/safeio.IsNullDevice()` and substitute an in-memory `NullWriter` — no file system touch at all.

Recognized null device paths:

| Platform | Path |
| -------- | ---- |
| Unix/macOS | `/dev/null` |
| Windows | `NUL` (case-insensitive) |

mingw/Git Bash users are unaffected (POSIX emulation translates `/dev/null` already).

### Dependency Vulnerability Remediation

Three Go module bumps address published CVEs:

| Module | Old | New | CVE / Advisory |
| ------ | --- | --- | -------------- |
| `go.opentelemetry.io/otel/sdk` | v1.39.0 | v1.40.0 | GHSA-qhcg-phj2-fjhh |
| `github.com/go-git/go-git/v5` | v5.16.4 | v5.16.5 | GHSA-898f-h2v3-q986 |
| `github.com/cloudflare/circl` | v1.6.2 | v1.6.3 | GHSA-vjc3-whcr-jvjj |

Two GitHub Actions pins harden the CI pipeline against tag-hijack supply chain attacks:

| Action | Before | After | Advisory |
| ------ | ------ | ----- | -------- |
| `actions/download-artifact` | `v4` | `v4.1.8` | GHSA-cxww-7g56-2vh6 |
| `actions/upload-artifact` | `v4` | `v4.6.2` | Preventive pin |

### golangci-lint Non-Go Project Gate

In polyglot repositories (e.g., a Rust workspace with Go bindings in `bindings/go/`), `findGoFiles()` discovered `.go` files in subdirectories and dispatched golangci-lint against the project root — where no `go.mod` exists. golangci-lint returned exit code 7 (typechecking error), which surfaced as a medium-severity lint finding and broke `--fail-on medium` gates.

Two fixes:

1. **go.mod existence gate**: Before dispatching golangci-lint, `Assess()` now checks for `go.mod` at the target root. If absent, the tool is skipped with an info log.
2. **Include-filter fallback**: When `--include` filters result in zero Go files, `runGolangCILintWithMode()` now returns empty instead of falling back to the `./...` glob (which would scan the entire project).

## Upgrade Notes

Drop-in replacement for v0.5.7. No config migration required.

## Contributors

- Claude Opus 4.6 (devlead)
- @3leapsdave (supervision)

---

# Goneat v0.5.7 — Scoop Distribution & v0.5.6 Signing Fix

**Release Date**: 2026-03-09
**Status**: Stable

## TL;DR

- **Scoop distribution**: goneat is now installable on Windows via `scoop install goneat` from the `fulmenhq/scoop-bucket` bucket
- **Release pipeline**: `make release-upload` automatically updates both Homebrew and Scoop package metadata
- **Signing fix**: v0.5.6 shipped without minisign/PGP signature assets — v0.5.7 restores full signed releases

## What Changed

### Scoop Bucket & Manifest Automation

goneat Windows users can now install via Scoop:

```powershell
scoop bucket add fulmenhq https://github.com/fulmenhq/scoop-bucket
scoop install goneat
```

The `fulmenhq/scoop-bucket` repository includes:

- `bucket/goneat.json` manifest with SHA256 hash verification
- `scripts/update-manifest.sh` for automated version bumps via `jq`
- Makefile targets (`update-goneat`, `update`, `release`)

### Release Pipeline Integration

`make release-upload` now calls `make update-scoop-manifest` after uploading artifacts, matching the existing Homebrew formula automation. If `../scoop-bucket` is missing, the Scoop step is skipped with a warning (non-blocking).

RELEASE_CHECKLIST.md and binary distribution docs updated to cover Scoop alongside Homebrew.

### Stabilized ASCII Art Tests

Terminal-width-dependent specs for ASCII art rendering are now stable across environments with varying terminal column counts.

### gosec 2.24 Remediation

gosec upgraded from 2.23.0 to 2.24.7, introducing expanded taint analysis and new rules. Rather than blanket suppression, findings were resolved in three ways:

- **G122 (filepath.Walk TOCTOU)**: `cmd/content.go`, `pkg/schema/validator.go`, `pkg/schema/id_index.go` — switched to `os.Root`-scoped APIs to eliminate symlink race conditions
- **G118 (context.Background in goroutine)**: `internal/guardian/browser.go` — introduced a derived shutdown context via `context.WithoutCancel`
- **G703 (path traversal via taint)**: 19 local file-write sites routed through `pkg/safeio/write.go`, documenting the trust boundary once rather than repeating per-line suppressions
- **G118 (cancel not called)**: `cmd/validate_suite.go`, `internal/assess/security_runner.go` — suppressed with rationale (helpers intentionally return cancel func for callers to defer)

### v0.5.6 Signing Gap

v0.5.6 was released with binaries and checksums but without PGP signatures (`.asc`) or minisign signatures (`.minisig`). v0.5.7 restores the full signing workflow. Users who need verified artifacts should upgrade to v0.5.7.

## Upgrade Notes

Drop-in replacement for v0.5.6. No config migration required.

## Contributors

- Claude Opus 4.6 (devlead)
- @3leapsdave (supervision)

---

# Goneat v0.5.6 — Glob-to-Regex Conversion for gosec Excludes

**Release Date**: 2026-02-26
**Status**: Stable

## TL;DR

- **No more gosec panics**: ignore-file glob patterns like `*.egg-info/` are now converted to valid regex before being passed to gosec
- **Better exclude matching**: `**/dist/` and similar patterns are properly converted to regex equivalents
- **Detailed diagnostics**: per-pattern debug logging shows what was converted or skipped

## What Changed

### Glob-to-Regex Conversion

gosec's `-exclude-dir` flag expects regex patterns, but goneat was passing glob patterns from `.gitignore` files directly. Patterns like `*.egg-info/` caused gosec to panic on invalid regexp syntax (`?*` — nested repetition operator).

goneat now converts glob metacharacters to regex-safe equivalents:

| Glob Pattern | Regex Output |
| ------------ | ------------ |
| `*.egg-info` | `[^/]*\.egg-info` |
| `**/dist` | `(.*/)?dist` |
| `test?` | `test[^/]` |

### Pattern Validation

Every generated regex is validated with `regexp.Compile` before being passed to gosec. Unconvertible patterns are skipped safely with documented reason codes:

| Reason | Description |
| ------ | ----------- |
| `empty_pattern` | Whitespace-only or empty line |
| `negation_not_supported` | Patterns starting with `!` |
| `duplicate_pattern` | Already seen (deduped) |

### Optimization

Exclude pattern parsing now happens once outside the worker pool, avoiding redundant reads of `.gitignore`/`.goneatignore` files.

## Upgrade Notes

Drop-in replacement for v0.5.5. No config migration required.

## Contributors

- opencode/gpt-5.2 (devlead)
- @3leapsdave (supervision)
