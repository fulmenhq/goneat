# Goneat v0.5.11 — Hook Timeout Enforcement and Dates Runner Single-Pass Fix

**Release Date**: 2026-05-17
**Status**: Stable

## TL;DR

- **Pre-commit hook no longer hangs** when `goneat assess --hook pre-commit --staged-only --package-mode` is configured with `dates` in the assess categories — the dates runner now does a single scoped scan instead of one full-repo scan per staged file
- **Hook manifest timeouts actually fire**: per-command `timeout` values in `.goneat/hooks.yaml` now reach the internal `assess`/`format`/`dependencies` handlers and preempt the underlying work, instead of being silently dropped
- **CI hardening**: container jobs pin the goneat tools runner image to `:v0.4.1` and use the runner-bundled `golangci-lint` instead of `golangci/golangci-lint-action@v7`, removing CI's dependency on a network fetch from `golangci-lint.run`
- **No config migration required** — drop-in replacement for v0.5.10

## What Changed

### Dates Runner: Single-Pass Scan Under `--staged-only`

Sumpter (`fulmenhq/sumpter`) surfaced a real pre-commit hang on
`goneat assess --hook pre-commit --staged-only --package-mode` with
`format,lint,dates` categories. The hook would reach
`Running dates assessment...` and then go silent for minutes.

Root cause: when the assess wrapper received an `IncludeFiles` list (populated
by `--staged-only` and expanded by `--package-mode`), the dates assess runner
looped over the include set and ran a **full-repository dates scan once per
file** — re-walking the tree, re-reading every changelog, re-running monotonic
order analysis for each entry in the staged set. For a package containing
dozens of files this multiplied a 3-second scan into multi-minute work and
duplicated every finding N times in the report.

v0.5.11 deletes the per-file loop in `internal/assess/dates_runner.go` and
instead passes the filtered include set directly to the internal
`dates.DatesRunner.Assess`, which now honors an optional explicit file list
via its existing `extra interface{}` parameter. One scan, one worker pool,
one set of issues — no walks, no duplicates.

Additional cleanups:

- Removed a stray `fmt.Printf("DEBUG: …")` from the assess wrapper that was
  violating STDOUT hygiene during dates assessment failures.
- An explicit empty include set (e.g. when every staged file is filtered by
  `.goneatignore`) is now correctly honored as "scan zero files" instead of
  falling back to a full-repo discovery.

### Hook Manifest Timeouts Now Reach Internal Commands

The same sumpter report revealed a second, independent bug: the `timeout: 2m`
configured per-command in `.goneat/hooks.yaml` was being silently dropped for
internal commands (`assess`, `format`, `dependencies`).

`HookExecutor` correctly wrapped each command in `context.WithTimeout`, but
`cmd/assess.go::runInternalAssess` ignored the supplied context and passed
`cmd.Context()` (the parent cobra context) to the assessment engine. The
manifest timeout therefore never reached the runners — a stuck assessment
ran until the user killed the hook.

v0.5.11 threads the handler context through `runInternalAssess`,
`runInternalFormat`, and `runInternalDependencies` so the per-command timeout
reaches the engine. As a layered defense, the dates runner now also observes
`ctx.Done()` at the directory-discovery loop, in the `filepath.WalkDir`
callback, at each worker iteration, and after `wg.Wait()` — so a stuck scan
preempts within one in-flight file when the deadline fires and surfaces
`context.DeadlineExceeded` to the executor.

Net result: a hook manifest like

```yaml
pre-commit:
  - command: assess
    args: [--categories, format,lint,dates, --fail-on, high]
    timeout: 2m
```

now actually terminates after 2 minutes instead of hanging indefinitely.

## Regression Coverage

Three new tests guard against regression:

- `TestDatesRunner_IncludeFilesSingleScan` — fixture with four `.md` files
  where only three are in `IncludeFiles`; asserts the excluded file is not
  scanned and that each included file produces exactly one issue (no
  N×duplication).
- `TestDatesRunner_Assess_HonorsCtxCancellation` — seeds 200 files, cancels
  the context before invoking the runner, asserts the call returns a
  ctx-derived error in <2s with `Success=false`.
- `TestDatesRunner_Assess_ExplicitEmptyFiles` — confirms an explicit empty
  `[]string{}` is honored as "scan zero files" rather than falling back to
  full discovery.

The existing `TestExecuteHookCommands_InternalHandlerCtxCarriesTimeout`
covers the executor-to-handler ctx contract as a layered defense.

### CI Workflow Hardening

All container jobs in `.github/workflows/ci.yml` now pin the
`ghcr.io/fulmenhq/goneat-tools-runner-glibc` image to `:v0.4.1` instead of
`:latest`. This eliminates floating-tag drift between PRs and `main`.

The `build-test-lint` job no longer uses `golangci/golangci-lint-action@v7`
(which performed a network fetch from `golangci-lint.run` at setup time) and
instead invokes the `golangci-lint` binary bundled in the pinned runner image.
The lint version is now anchored to the runner-image tag rather than to the
action's default — a future tooling bump becomes a deliberate image-tag flip
rather than a silent action-default change. CI builds log the active
`go version` and `golangci-lint --version` for auditability.

The lint step remains advisory (`continue-on-error: true`), matching prior
behavior — no gate change.

### Dependency Follow-Up Deferred to v0.5.12

A dependency scan during this cycle produced a punch list of direct and
indirect bumps (notably `go-git`/`go-billy` direct deps, plus indirect lifts
in `grpc`, `otel/sdk`, and `x/crypto`). Those were intentionally deferred to
v0.5.12 to keep v0.5.11 focused on the critical hook hang and timeout fix
and to avoid introducing dependency-behavior risk late in the cycle.

Release artifacts continue to be built by `release.yml` via `setup-go`
on the current patched 1.25.x line (≥ go1.25.10).

## Known Issues

### `golangci-lint` Config Verification Fails in Offline/Sandboxed Environments

In local dev environments without network egress to `golangci-lint.run`
(e.g., sandboxed CI agents, offline laptops), the `goneat assess --categories
lint` path may log:

```
Lint: error - golangci-lint config validation failed: ...
```

This happens because goneat shells out to `golangci-lint config verify`,
which internally fetches the JSON schema for `.golangci.yml` from
`golangci-lint.run`. When that network fetch fails, the subprocess returns
non-zero and goneat surfaces the failure as a lint error — even when the
actual lint run succeeds and the hook reports `Hook validation passed`.

**Impact**: cosmetic only. The hook exit code is correct and gating is not
affected. Safe to ignore when the surrounding hook reports success.

**Workaround**: none required locally. CI is unaffected because
`.github/workflows/ci.yml` uses the runner-bundled `golangci-lint` against a
pinned runner image, bypassing the schema fetch.

**Planned fix**: v0.5.12 will distinguish network-fetch failure from genuine
schema validation failure so the offline path degrades to a warning rather
than surfacing as a lint error.

## Workaround for Pre-v0.5.11 Users

If upgrade is blocked, the same outcome can be reached by adjusting hook
policy in `.goneat/hooks.yaml`:

- Drop `dates` from the pre-commit `assess` categories
- Keep `dates` in pre-push, or invoke `goneat dates check` as a standalone
  hook command

This matches the pattern in most fulmenhq repositories and avoids the
problematic `dates` + `--staged-only` + `--package-mode` combination.

## Upgrade Notes

Drop-in replacement for v0.5.10. No config migration required.

If you have hooks configured with categories `format,lint,dates` and use
`--staged-only --package-mode`, the dates assessment will now complete in
seconds instead of minutes (or hanging).

## Contributors

- Claude Opus 4.7 (devlead, devrev)
- @3leapsdave (supervision)
- @agent-india-devlead (bug report and cross-repo audit)

---

# Goneat v0.5.10 — Dependency License Exceptions and Cleaner Policy Validation

**Release Date**: 2026-03-30
**Status**: Stable

## TL;DR

- **License exceptions now work in assess**: `goneat assess --categories dependencies` now honors `licenses.exceptions` instead of treating them as config-only documentation
- **Cleaner dogfood dependency output**: the recurring `dependencies: policy failed schema validation` warning is gone for goneat's own repo policy
- **No upstream dependency churn required**: the HashiCorp license false-positive workaround is fixed in goneat's evaluation path while keeping `go-licenses` pinned at `v2.0.1`

## What Changed

### License Exceptions Now Apply During Dependency Assessment

`GNT-008` closed a real assess-path gap in dependency policy enforcement.

Before v0.5.10, goneat could parse forbidden licenses from
`.goneat/dependencies.yaml`, but the assess path ignored `licenses.exceptions`
even though the policy shape already documented them. That meant maintainers
could record a reviewed false positive in config and still see the build fail on
the same package during `goneat assess --categories dependencies`.

v0.5.10 fixes that by routing Go dependency license evaluation through an
exception-aware helper that:

- matches exact package/license exception entries before raising forbidden-license findings
- supports both `package` + `license` and `name` + `licenses` schema forms
- respects `approved_date` and optional `until` windows for temporary overrides

This was verified against the real `enacthq/enact` dependency graph, where
`github.com/hashicorp/go-cleanhttp` and
`github.com/hashicorp/go-retryablehttp` were previously surfacing as forbidden
`GPL-3.0` findings despite reviewed exceptions.

### Dependency Policy Schema Warning Cleanup

`GNT-004` cleaned up a separate but noisy release-time issue.

goneat's checked-in `.goneat/dependencies.yaml` already used richer
vulnerability allowlist metadata for traceability:

- `status`
- `sdr`
- `analysis`
- `verified_by`
- `verified_date`

The runtime suppression behavior was already fine, but the
`dependencies-policy-v1.0.0` schema did not allow those fields, so dogfood runs
emitted:

```text
dependencies: policy failed schema validation
```

v0.5.10 aligns the schema with the real supported config shape and adds a
regression test that validates the repo's actual `.goneat/dependencies.yaml`
against the embedded schema. The result is calmer dependency assessment output
without weakening real validation for actual policy drift.

### Policy Examples and Troubleshooting Updated

The dependency policy examples and troubleshooting docs now better reflect the
actual supported license exception flow, including optional `until` dates for
temporary overrides.

## Upgrade Notes

Drop-in replacement for v0.5.9. No config migration required.

If you already maintain `licenses.exceptions` in `.goneat/dependencies.yaml`,
those entries now affect dependency assessment directly instead of serving only
as documentation.

## Contributors

- GPT-5.4 (devlead)
- @3leapsdave (supervision)

---

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
