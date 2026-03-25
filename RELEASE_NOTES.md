# Goneat v0.5.9 — YAML Comment Parity, Repo-Scoped gosec, and PR Workflow Refresh

**Release Date**: 2026-03-25
**Status**: Stable

## TL;DR

- **YAML comment parity**: `goneat format`, `goneat assess --categories format`, and strict `yamllint` now agree on inline comment spacing by default
- **Repo-scoped gosec**: security assessment no longer lets `GOCACHE` or other out-of-repo Go build artifacts block releases
- **Maintainer workflow refresh**: goneat now uses a protected `main` + pull request flow, with `make pr-final` as the final merge-readiness check
- **golangci-lint tool alignment**: recommended local tool guidance now matches CI at `2.11.2`

## What Changed

### YAML Format/Lint Alignment

`3leaps/sysprims` exposed a real dogfood gap in YAML handling.

Before this fix, `goneat format` and `goneat assess --categories format --fix`
could rewrite inline comments into the one-space form:

```yaml
enabled: true # inline comment
```

while strict `yamllint` still expected:

```yaml
enabled: true  # inline comment
```

The underlying issue is that `yamllint` defaults to `2` spaces before inline
comments, while `yamlfmt` defaults to `1` via `pad_line_comments`.

v0.5.9 fixes this in three ways:

- `goneat assess --categories format` now routes YAML files through the same formatter path as `goneat format`
- goneat pins YAML inline comment padding to a lint-compatible default of `2`
- goneat passes that same setting into both YAML format and YAML check flows so
  `format`, `assess --categories format --check`, and `assess --categories format --fix`
  stay aligned

This restores the expected contract that developers can format first and then
trust hook and CI checks.

### Evergreen YAML Guidance

Because many teams use both `.yamlfmt` and `.yamllint`, goneat now documents the
precedence model explicitly:

- `.yamllint` defines lint policy
- goneat config defines formatter behavior for settings that goneat pins
- `.yamlfmt` defines the remaining formatter-native behavior

That guidance is now available in the binary docs via:

```bash
goneat docs show user-guide/commands/format
goneat docs show appnotes/yaml-format-lint-alignment
```

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

Teams that intentionally use non-default YAML inline comment spacing should keep
goneat formatter settings and `.yamllint` policy aligned explicitly.

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
