# Goneat v0.5.12 — Format-Runner Unification, Offline Lint UX, and Supply-Chain Hygiene

**Release Date**: 2026-05-27
**Status**: Stable

## TL;DR

- **`goneat format` is now byte-compatible with `goneat assess --categories format`** — the sequential `goneat format` path previously omitted `pad_line_comments` when invoking `yamlfmt`, so it was a silent no-op on YAML files the parallel and assess paths would correctly flag. All three call sites now share a single arg builder.
- **Direct `yamlfmt -lint` callers** (CI jobs, pre-commit hooks, IDE integrations) need to mirror goneat's canonical setting in their `.yamlfmt` — see the [appnote](docs/appnotes/yaml-format-lint-alignment.md) for the load-bearing one-liner that prevents "goneat says clean, CI says broken" false reds.
- **`Lint: error` no longer surfaces in offline/sandboxed dev** — `golangci-lint config verify` network failures are now classified and demoted to a warn-skip, while structural schema errors still propagate.
- **CI license-audit silent false-green is fixed** — `Makefile` `rg → grep -E` plus a narrow allowlist for the MPL-2.0 `cyphar/filepath-securejoin` dependency (transitive of `go-git/go-billy`, entarch-reviewed).
- **`scripts/verify-release-assets.sh` now actually works** — sorts by filename column (`sort -k 2`) for all three SHA256/SHA512 compares, plus a pre-existing `cp src dest`-where-src=dest fix.
- **Dependency hygiene**: `go-git/v5` 5.16.5 → 5.19.0, `go-billy/v5` 5.7.0 → 5.9.0, `grpc` 1.78.0 → 1.81.1, `otel/sdk` 1.40.0 → 1.43.0, `x/crypto` 0.47.0 → 0.51.0 (+ resolver-permitted `x/*` group coherence).
- **CI Go runtime bumped to 1.26.x (security-driven)** — CI containers now run `goneat-tools-runner-glibc:v0.4.2` (Go 1.26.2 + golangci-lint v2.12.1) instead of `:v0.4.1`. The Go 1.26.x line includes fixes for CVE-2026-33810 and other security advisories that landed after v0.5.11. `go.mod`'s minimum Go directive stays at `1.25.0` for downstream compatibility.
- **No config migration required** — drop-in replacement for v0.5.11.

## What Changed

### `goneat format` vs `yamlfmt -lint` Divergence (limensafe)

Limensafe surfaced a real CI-blocking divergence: bare `yamlfmt -lint .`
flagged YAML files that `make fmt` (which dispatches `goneat format`) said
were clean. Worse, `goneat format` applied directly to those files was a
no-op — yet `goneat assess --categories format` on the same files reported
them as needing formatting. Three goneat code paths disagreeing among
themselves was the actual bug; the CI-vs-goneat split was a downstream
symptom.

Root cause: each YAML-handling path built its own `-formatter` argument list
for `yamlfmt`.

- `cmd/format.go::formatYAMLFile` (sequential `goneat format`) **omitted
  `pad_line_comments` entirely**. yamlfmt's built-in default is `1`, so
  goneat's canonical `pad_line_comments: 2` was silently dropped — the file
  matched yamlfmt's view of "clean" but did not match goneat's policy.
- `pkg/work/format_processor.go::formatYAMLFile` (parallel `goneat format`
  and `goneat assess --categories format --fix`) **did** pass
  `pad_line_comments=2`.
- `pkg/work/format_processor.go::checkYAMLFile` (assess check path) also
  passed `pad_line_comments=2`, but did so via its own hand-built copy of
  the arg-construction logic.

Three copies of the same logic, only two of them correct, drifting silently.

v0.5.12 extracts the canonical builder as
`pkg/config.YAMLFormatConfig.YamlfmtFormatterArgs` and routes all three
call sites through it. A flag is emitted only when the configured value
diverges from yamlfmt's own default (yamlfmt defaults: `indent=2`,
`line_length=80`, `pad_line_comments=1`); goneat's canonical
`pad_line_comments=2` therefore always materializes as an explicit
`-formatter pad_line_comments=2` flag under default configuration.

Regression coverage:

- `TestYAMLFormatConfig_YamlfmtFormatterArgs` (5 cases) exercises the
  builder directly.
- `TestDefaultConfig_YAMLFormat_EmitsPadLineCommentsTwo` guards the
  load-bearing default — if goneat ever stops emitting
  `pad_line_comments=2` under default config, this test fails.
- `TestFormatProcessor_YAMLDivergenceFixture_FixesAndAgrees` exercises a
  fixture in the broken-input state (1-space comments + mid-block blanks)
  and verifies four agreement points: `checkYAMLFile` flags it,
  `formatYAMLFile` fixes it, `checkYAMLFile` is then clean, and bare
  `yamlfmt -lint -formatter pad_line_comments=2` agrees with the
  goneat-formatted output.

### Updated Guidance for Direct `yamlfmt` Callers

Goneat's internal alignment does **not** extend to tools you invoke
outside goneat. Direct `yamlfmt -lint` from a CI job, a pre-commit hook,
or an IDE integration reads only `.yamlfmt` (plus yamlfmt's built-in
defaults) — it has no path to goneat's canonical config. For those
callers to agree with goneat, `.yamlfmt` must set
`formatter.pad_line_comments: 2` explicitly.

`docs/appnotes/yaml-format-lint-alignment.md` now leads its Repository
Guidance section with this requirement as a load-bearing callout. A new
"Direct `yamlfmt` callouts" subsection lists three patterns
(`replace-with-goneat` / `pin-in-.yamlfmt` / `pass-on-cmdline`) with the
recommended ordering, and a new "Symptom of misalignment" subsection
names the exact diff users will see when the drift is active so future
debugging is fast. The matching guidance landed in
`docs/user-guide/commands/format.md`.

### `Lint: error` Offline UX

`internal/assess/lint_runner.go::verifyGolangciConfig` invokes
`golangci-lint config verify`, which fetches its JSON schema from
`golangci-lint.run`. In offline or sandboxed dev environments that fetch
fails, golangci-lint exits non-zero, and goneat surfaced the failure as
`Lint: error - golangci-lint config validation failed` even though hook
gating was correct and the surrounding hook reported `Hook validation
passed`. Cosmetic only, but confusing — reported by @agent-india-devlead
during v0.5.11 validation.

v0.5.12 classifies the stderr against a curated set of network-error
patterns:

```
dial tcp
no such host
i/o timeout
failed to get schema
connection refused
network is unreachable
tls handshake timeout
context deadline exceeded
temporary failure in name resolution
```

On a match, the failure is demoted to
`logger.Warn("golangci-lint schema verification skipped (offline): ...")`
and the function returns `nil`. Structural schema errors (yaml parse
errors, linter-name typos, etc.) still propagate as before. The
classifier is a pure function exercised by 10 unit tests including
negative cases.

### CI License-Audit Silent False-Green

@agent-kilo-devrev found that `make license-audit` was passing in CI even
when the goneat repo carries an MPL-2.0 transitive dependency that the
audit policy forbids. Root cause: the Makefile target shelled `rg
"$forbidden"` inline. The CI runner image does not install ripgrep, so
`rg: not found` returned non-zero, the `if echo "$out" | rg "$forbidden"
>/dev/null; then` branch fell through to the success arm, and CI happily
reported `✅ No forbidden licenses detected`.

v0.5.12:

- Replaces `rg "$forbidden"` with `grep -E "$forbidden"` — no new tool
  dependency, and the matcher cannot silently fail.
- Adds an explicit Makefile-level allowlist filter for the exact
  `(github.com/cyphar/filepath-securejoin, MPL-2.0)` exception before the
  forbidden-pattern check.
- Records the narrow exception in `.goneat/dependencies.yaml`:
  - **Reason**: transitive bounded-filesystem dependency of
    `go-git/go-billy` (`pkg/ignore` → `go-billy/v5/osfs` →
    `filepath-securejoin`). Required by the `go-billy` v5.9.0 /
    `go-git` v5.19.0 bumps in this release.
  - **Condition**: unmodified dependency only; no MPL-covered files
    vendored or modified.
  - **Reviewer**: @agent-entarch-fulmenhq (architecture review).
  - **Approver**: @3leapsdave.
  - **Revisit**: v0.5.13 / v0.6.0.

The schema in `.goneat/dependencies.yaml` doesn't currently gate the audit
(`make license-audit` shells `go-licenses csv` directly), so the exception
is a durable policy record today and becomes the audit SSOT in v0.5.13
when the Makefile target switches to `goneat dependencies --licenses`.

### `scripts/verify-release-assets.sh` Fix

Hit during v0.5.11 release verification: the script reported a
false-positive checksum mismatch because it sorted `SHA256SUMS` /
`SHA512SUMS` with the default `sort` (lexical full-line, hash-first) and
the local and uploaded files had identical `(hash, filename)` pairs in
different orders. Manual `diff <(sort -k 2 A) <(sort -k 2 B)` was clean.

v0.5.12:

- Changes all three compares to `sort -k 2` (sort by filename, the second
  whitespace-delimited field) so identical content with different line
  ordering compares clean.
- Fixes a pre-existing logic bug where `LOCAL_SHA256_SORTED` and
  `local_sorted` pointed at the same file, causing BSD `cp` (macOS) to
  exit 1 with "are identical".

Verified end-to-end against the live v0.5.11 GitHub release: exit 0.

### `.git/**` Excludes Are Now Worktree-Safe

The default `.goneat/assess.yaml` templates excluded `.git/**` from
`yamllint`, `actionlint`, `shellcheck`, and `checkmake` targets. In a
linked git worktree (`git worktree add ...`) `.git` is a file
(gitfile), not a directory, so the `**` glob trips on `stat .git/**` and
the entire exclude entry can fail to apply. Limensafe's PR #1 hit this
during a worktree-based refit.

v0.5.12 sweeps `.git/**` → `.git` in all five
`templates/assess/*.yaml` SSOTs (`go`, `rust`, `python`, `typescript`,
`unknown`). The bare-name form excludes both files and directories
named `.git` safely. Existing repos that ran `goneat doctor assess init`
under an earlier version should apply the same edit to their
`.goneat/assess.yaml` manually; a future `goneat doctor assess
upgrade` could automate this.

### Dependency Bumps

Per @agent-kilo-devrev's punch list, re-validated against `go list -m -u`
at branch time. Scope intentionally narrow:

| Module                            | From    | To      | Type     |
| --------------------------------- | ------- | ------- | -------- |
| `github.com/go-git/go-git/v5`     | 5.16.5  | 5.19.0  | direct   |
| `github.com/go-git/go-billy/v5`   | 5.7.0   | 5.9.0   | direct   |
| `google.golang.org/grpc`          | 1.78.0  | 1.81.1  | indirect |
| `go.opentelemetry.io/otel/sdk`    | 1.40.0  | 1.43.0  | indirect |
| `golang.org/x/crypto`             | 0.47.0  | 0.51.0  | indirect |

Resolver-permitted `golang.org/x/*` group coherence lifted as a
side-effect: `sys` 0.40.0 → 0.44.0, `text` 0.33.0 → 0.37.0, `net`
0.49.0 → 0.53.0, `sync` 0.19.0 → 0.20.0, `tools` 0.41.0 → 0.44.0,
`mod` 0.32.0 → 0.35.0, `exp` updated to its current snapshot. No
broadening to other indirect deps (OPA et al. left untouched).

`github.com/cyphar/filepath-securejoin` is pulled to v0.6.1 as a
required upgrade for `go-billy` v5.9.0 / `go-git` v5.19.0; covered by
the MPL-2.0 allowlist landed in this same release.

### CI Go Runtime Bumped to 1.26.x (Security-Driven)

The original v0.5.12 punchlist deferred the Go runtime pin strategy to
v0.5.13 — the intent was a deliberate research pass against the
`fulmen-toolbox` CVE-driven pin patterns. That research surfaced
`fulmen-toolbox v0.3.5` ("Go 1.26.2 toolchain bump (CVE-2026-33810)"),
which means the existing `:v0.4.1` runner pin in `.github/workflows/ci.yml`
is on a Go 1.25.x line that lacks the v0.3.5 fix and other 1.26.x
security advisories. With local dev already moved to Go 1.26.3 and
fulmen-toolbox already publishing a `:v0.4.2` image that bundles Go
1.26.2 + golangci-lint v2.12.1, the right move is to pull the bump
into v0.5.12 rather than ship a release on the older runtime.

Changes in this release:

- `.github/workflows/ci.yml` — three container references bumped from
  `ghcr.io/fulmenhq/goneat-tools-runner-glibc:v0.4.1` to `:v0.4.2`
  (build-test-lint, container-probe, bootstrap-probe).
- `.github/workflows/release.yml` — `go-version: '1.25.x'` →
  `'1.26.x'` for the release-artifact build.
- `.github/workflows/license-audit.yml` — same `'1.25.x'` →
  `'1.26.x'` bump.

`go.mod`'s `go 1.25.0` directive is intentionally **unchanged** — the
floor for downstream consumers and `go install` stays at 1.25.0. This
release simply ships its own binaries built on the newer runtime; it
does not require downstreams to upgrade their build environment.

This is the "circular" goneat ↔ fulmen-toolbox relationship in normal
operation: the toolbox release that picked up the Go 1.26.x line (and
the matching golangci-lint v2.12.1 line) became available, so goneat
v0.5.12 consumes it. A future goneat release will trigger another
fulmen-toolbox cut, and so on.

## Known Issues Deferred to v0.5.13+

- **Defect C — hook-level vs per-command timeout placement.** Fix B in
  v0.5.11 handles per-command timeouts; the hook-level placement could
  not be verified during v0.5.12 because no consumer `.goneat/hooks.yaml`
  with hook-level `timeout:` surfaced. Low priority — most existing
  manifests are per-command.
- **`TestFormatter_HTML_UsesMetricsFilesForGitState` flake.** Pre-existing;
  ~1/3 failure under `-count=10`. Likely `GONEAT_TEMPLATE_PATH`
  env-pollution. Filed for v0.5.13 spike.
- **`TestEngine_ConcurrencyMappingAffectsWallTime` flake.** Pre-existing
  timing-sensitive test; passes 5/5 in isolation; flakes only under
  full-suite load contention. Sibling of the formatter flake.
- **Node 20 GitHub Actions deprecation.** Workflow-maintenance sweep;
  not a release blocker.
- **Dogfood `goneat dependencies --licenses` for `make license-audit`.**
  Changes bootstrap/build ordering and deserves its own review. Once
  landed, `.goneat/dependencies.yaml` becomes the audit SSOT and the
  Makefile allowlist filter from this release can be retired.
- **Deterministic Go patch pin for release.yml.** This release moves the
  CI workflows from `'1.25.x'` to `'1.26.x'` (`check-latest` semantics
  via the `.x` shorthand). A future release may pin a specific patch
  (e.g. `'1.26.3'`) in `release.yml` for deterministic supply-chain
  reproducibility, per @agent-kilo-devrev's original v0.5.12 proposal.
  Keeping `.x` for v0.5.12 to minimize churn in this fast-follow.
- **Pre-existing format issues in synced `config/crucible-go/` and
  `schemas/crucible-go/` content.** Medium severity, do not block
  `--fail-on high`. The content is SSOT-mirrored from
  `fulmenhq/crucible`; cleanup belongs upstream there, not in this repo.

## Out of Scope (Intentionally)

`fulmen-toolbox` runner image rebuild is **not** bundled with this
release. The toolbox is decoupled: a separate `v0.4.2` cut in
`~/dev/fulmenhq/fulmen-toolbox/` will pick up the new goneat binary
post-release. No change to `.github/workflows/ci.yml` runner pin in
this release.

## Upgrade Notes

Drop-in replacement for v0.5.11. No config migration required.

If your repository or its CI invokes `yamlfmt` directly (bare
`yamlfmt -lint .`, a pre-commit hook calling `yamlfmt`, an IDE
integration, etc.), apply the one-line fix from the appnote to your
repo-local `.yamlfmt`:

```yaml
formatter:
  type: basic
  indent: 2
  line_ending: lf
  pad_line_comments: 2  # MUST match goneat's canonical default
```

Without that line, you can hit a "goneat format says clean, direct
yamlfmt -lint says broken" false-red. See
[YAML Format/Lint Alignment](docs/appnotes/yaml-format-lint-alignment.md)
for the full guidance and a debug-aid section that names the exact
diff you'll see when the drift is active.

If you maintain your own `.goneat/assess.yaml` (rather than the one
generated by `goneat doctor assess init`) and your project uses git
worktrees, change `.git/**` exclude entries to `.git`. The `**` form
trips on linked worktrees where `.git` is a gitfile.

## Contributors

- Claude Opus 4.7 (devlead, devrev)
- @3leapsdave (supervision, scope decisions, MPL-2.0 disposition)
- @agent-kilo-devrev (devrev): dep-bump punch list, license-audit
  false-green root cause, Go pin observation, refactor guidance for
  the YAML arg-builder unification, two-round review on the
  fix branch
- @agent-entarch-fulmenhq (entarch): risk/reward review of the
  MPL-2.0 exception vs replacement decision
- @agent-india-devlead: original reporter of the `Lint: error` offline
  UX
- kilo-devlead (limensafe session): original reporter of the
  `goneat format` vs `yamlfmt -lint` divergence
