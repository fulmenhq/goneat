# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

> **Note**: This changelog keeps the latest 10 releases for readability. For older releases, see `docs/releases/` archive.

## [v0.6.2] - 2026-10-09

### Changed

- **Release toolchain**: workflow toolchain pins are Go 1.26.9. The module language line stays `go 1.26.0`.
- **Selected Go modules**: `golang.org/x/net` v0.60.0, `golang.org/x/crypto` v0.57.0, `golang.org/x/sys` v0.48.0, and `golang.org/x/text` v0.42.0. Selected `golang.org/x/term` changes from v0.45.0 to v0.46.0. `go.sum` retains the v0.45.0 checksums and includes the v0.46.0 go.mod checksum. `go.mod` does not require `golang.org/x/term` directly.
- **Indirect requirements added**: `cloud.google.com/go` v0.26.0, `cloud.google.com/go/compute/metadata` v0.3.0, `github.com/golang/glog` v1.2.4, `github.com/yuin/goldmark` v1.7.17, and `golang.org/x/oauth2` v0.27.0.
- **`golang.org/x/mod`**: v0.41.0 moves from indirect to direct.
- **Cooling selection**: The cooling policy exception for golang.org/x/net is in effect through 2026-10-16T00:00:00Z.
- **Cooling selection check**: While the cooling window is active, `scripts/check-cooling-selection.py` requires the selected `golang.org/x/net` module to be v0.60.0 and indirect.
- **Release archives**: v0.6.2 archives are Darwin ARM64, Linux amd64, Linux ARM64, Windows amd64, and Windows ARM64. Windows ARM64 is a native executable. There is no Darwin amd64 archive. Darwin amd64 downloads end at v0.6.1.

### Fixed

- **Scanner execution**: assessments retain scanner failures, release-asset results, and hook diagnostics. Native command output is captured as bytes.

## [v0.6.1] - 2026-09-28

### Added

- **Rust formatting**: `goneat format` and `goneat assess --categories format` run `cargo fmt --all` for the containing Cargo workspace (check mode: `cargo fmt --all -- --check -l`, one issue per unformatted file). Configure with `format.rust.enabled` / `format.rust.toolchain` in `.goneat.yaml`. When Rust is in scope, a missing cargo or rustfmt fails both commands; opt out with `format.rust.enabled: false` (or `--ignore-missing-tools` on `goneat format`, except for a configured toolchain). `assess <target>` reads the target's `.goneat.yaml`; when Rust is in scope, an invalid project config fails the check with the file path instead of falling back to defaults.
- **Clippy configuration**: `lint.rust.clippy` in `.goneat/assess.yaml` sets toolchain, `all_targets`, `all_features`, `features`, `no_default_features`, `locked`, `packages` and `targets` (one run per target triple, duplicate findings merged). Values are passed as separate arguments and option-shaped values are rejected. Defaults are unchanged.

### Changed

- **Minimum Go is now 1.26**: `go.mod` declares `go 1.26.0` (was `1.25.0`), required by `golang.org/x/crypto` v0.56.0. Building from source or via `go install` needs Go 1.26 or later; release binaries are built with Go 1.26.6+. `.goneat/tools.yaml` Go minimum raised to 1.26.0, and the cloud bootstrap `GOTOOLCHAIN` default is now `go1.26.6`.
- **Dependency refresh**: `open-policy-agent/opa` 1.18.2 → 1.20.2 (selects `grpc` 1.83.2, `oras-go` 2.6.2, OpenTelemetry 1.46.0, `klauspost/compress` 1.19.1), `golang.org/x/mod` 0.38.0 → 0.41.0, `golang.org/x/sync` 0.22.0 → 0.23.0, `golang.org/x/text` 0.40.0 → 0.42.0, `stretchr/testify` 1.11.1 → 1.12.1.
- **Recommended tool pins**: Go 1.26.5 → 1.26.6, `cargo-deny` 0.16.0 → 0.20.2, `cargo-audit` 0.21.0 → 0.22.2 (shared defaults, repository tools config, and the Rust example).

### Fixed

- **Linked Git worktrees**: repository state is now read correctly in worktrees created with `git worktree add` (attached or detached HEAD), where previously every tracked file could appear uncommitted or status failed with "object not found". This affects `assess` repo-status and release-phase maturity, `dates`, and `goneat version propagate` guards. A failure to read git state is a distinct high `git-status-error` finding in repo-status and maturity; maturity previously skipped its release-phase clean-tree check in that case. `dates` now runs its impossible-chronology check in linked worktrees, where it was silently skipped, and reports when the check cannot run. `version propagate` no longer blocks a clean linked worktree as dirty.
- **Clippy fails closed**: a non-zero Cargo exit now fails the lint category (manifest or dependency errors, build-script failures, missing toolchain or target), with Cargo's stderr tail. Diagnostics parsed before the failure are kept. goneat disables rustup auto-install for these runs.
- **biome fails closed; cargo-audit failures are diagnosed**: a timeout, a killed process, or a non-zero exit that the report does not explain is now an error instead of a clean result. For biome (lint, format check, config check), any `internalError` diagnostic (such as an unreadable or missing file) fails the run even at exit 0. A non-zero exit is accepted only with ordinary diagnostics, or when biome reports that every selected path is ignored by its own configuration. `format --write` failures are reported. For cargo-audit, the JSON must contain a vulnerabilities section, and a non-zero exit must come with advisories; otherwise the run is reported as a scanner error, which, like other security scanner errors, is logged and does not fail the category.
- **assess config root**: `.goneat/assess.yaml` schema now rejects unknown top-level keys (the root `additionalProperties: false` was nested under `properties`). The loader warns once per run about each unknown top-level key and ignores it, keeping the other sections. The warning names where the key belongs: a legacy `rust:` block moves to `format.rust` (`.goneat.yaml`) and `lint.rust.clippy`. A top-level `format:` block in `assess.yaml` has never been read; formatter options belong in the project `.goneat.yaml` and exclusions in `.goneatignore`.
- **Invalid assess config fails closed**: if `.goneat/assess.yaml` exists but cannot be read, parsed or validated (for example a wrong value type under `lint` or `typecheck`), the lint and typecheck categories now fail with the file path and the offending keys, in human and JSON output, instead of warning and running every check with defaults. A missing file still means defaults (a dangling symlink counts as present and fails), and unknown top-level keys are still warned about and ignored. Categories that do not read `assess.yaml` are unaffected.
- **Hooks fail on category errors**: `goneat assess --hook` now fails when a category ends in `error` status (it could not complete, for example invalid assess config, a missing required Rust tool, or a Cargo failure), as a direct `goneat assess` already did. Previously a hook passed unless a completed finding reached the `--fail-on` threshold. Thresholds for completed findings are unchanged.
- **yamllint strict warnings**: in strict mode (the default), yamllint exits 2 when it reports only warnings. The lint category now reports those warnings as findings instead of failing with "yamllint failed: exit status 2". Other non-zero exits still fail the category, and so does a findings exit (1, or 2 under `--strict`) unless stderr is empty and every output line is a parsable finding.
- **Missing shfmt skips**: when shfmt is not installed and shell files are in scope, shell format lint is now skipped, as it already is for shellcheck, actionlint and checkmake. Previously it failed the lint category with "shfmt execution failed".
- **Lint runs without golangci-lint**: the lint category no longer disappears when `golangci-lint` is not installed. Only Go lint is skipped, with a note in the report; clippy, ruff, biome, shell, YAML, GitHub Actions and Makefile linting still run. Previously `assess --categories lint` and hooks ran none of them on such machines.
- **Security runs any applicable scanner**: the security category no longer requires gosec or govulncheck to be installed; a Cargo project with only cargo-audit or cargo-deny is now assessed by them.
- **Requested categories are always reported**: a category named in `--categories` or a hook that cannot run here (for example `security` when no applicable scanner is installed) now appears as `skipped` with a `reason` in human and JSON output, instead of being left out silently.
- **Unknown category names fail**: an unrecognized name in `--categories` or a hook's category list (for example a misspelling) now fails the run before anything is assessed, and the error lists the valid names. Previously it was ignored, and a list of only unknown names ran nothing and passed. `goneat hooks validate` warns about unknown names too.
- **License audit fails closed**: `make license-audit` and `make license-inventory` now fail when `go-licenses` exits with an error or returns an empty inventory, instead of reporting success. A `go-licenses` binary built with a different Go toolchain than the active one is the usual cause; the error message names the reinstall command.

### Security

- **go-git** 5.19.1 → 5.19.2 and **go-billy** 5.9.0 → 5.9.1: fixes GO-2026-6213 (worktree symlink following) and GO-2026-6214 (ref-name path traversal).
- **golang.org/x/crypto** 0.54.0 → 0.56.0: fixes SSH denial-of-service advisories GO-2026-6354 and GO-2026-6355.
- Module-graph advisories in `grpc`, `oras-go`, `x/mod`, OpenTelemetry and `klauspost/compress` are resolved by the updates above.

## [v0.6.0] - 2026-09-01

### Added

- **Rust crates.io cooling**: `goneat dependencies` now wires crates.io registry cooling for Rust dependencies — crates.io publish age is attached as cooling metadata for registry-sourced crates only, with `age_unknown`/`registry_error` metadata for non-registry sources (#28).

### Changed

- **CI tools runner pin (Go 1.26.6)**: `.github/workflows/ci.yml` container jobs now use `ghcr.io/fulmenhq/goneat-tools-runner-glibc:v0.5.4` (was `:v0.5.1`) at all three sites. That image is the fulmen-toolbox v0.5.4 content cut: Go 1.26.6 (clears the Go-stdlib HIGH advisory cluster in in-runner-built binaries), node 22.23.2, glibc jq 1.8.2, and the musl openssl 3.5.8-r0 security pins. The `go 1.25.0` module floor is unchanged — runner toolchain and language compatibility stay decoupled.
- **Cloud bootstrap pins**: repository-managed Cursor Cloud Agent environment added (#29); bootstrap tool versions pinned to recommended pins (#31); `GOTOOLCHAIN` pin raised to `go1.25.8` for gosec (#32).
- **Cloud agent install hardening**: `scripts/cloud-agent-install.sh` now verifies the shellcheck release tarball against arch-specific SHA256 pins (x86_64/aarch64) before install, and `ensure_go` performs a best-effort version compare that WARNs (never fails the host) when an already-installed tool does not match its pin. Tool pins remain install-time; version comparison at run time is advisory.
- **MPL-2.0 exception renewed**: the `filepath-securejoin` exception (transitive via go-git/go-billy) is renewed through 2027-03-31 with unchanged conditions (unmodified dependency only); removal/refactor of the go-git path is tracked for a future v0.6.x release.

### Fixed

- **YAML format precedence**: format targeting now honors explicit `--files` over YAML precedence fixtures (#30).
- **Lint**: removed an unnecessary guard around `delete` in `pkg/dependencies/rust_cooling.go`; `make lint` is clean.

## [v0.5.16] - 2026-08-03

### Changed

- **CI tools runner pin**: `.github/workflows/ci.yml` container jobs now use
  `ghcr.io/fulmenhq/goneat-tools-runner-glibc:v0.5.1` (was `:v0.4.2`). That image
  is the fulmen-toolbox v0.5.0 content cut (republished as v0.5.1 for release-
  pipeline hygiene) and includes `ruff` for Python format/lint parity with
  goneat v0.5.15 fail-closed formatter preflight, plus Go 1.26.5 and the v0.5.0
  CVE/tool sweep. Container-probe smoke now checks `ruff --version`.

## [v0.5.15] - 2026-07-28

### Fixed

- **Standalone formatter coverage**: `goneat format` now fails when a required external formatter is unavailable, with `--ignore-missing-tools` as the explicit degradation path.
- **Doctor upgrade verification**: Go-tool upgrades retain versions for binaries discovered outside PATH, re-resolve the active binary after installation, detect PATH shadowing, and honor configured CLI version probes.

### Changed

- **go-licenses v2 alignment**: maintained Make, CI, doctor, policy, embedded-default, and documentation surfaces now pin `github.com/google/go-licenses/v2@v2.0.1`.
- **Tool configuration cleanup**: removed the deprecated hidden doctor defaults path; embedded foundation defaults initialize policy, while `.goneat/tools.yaml` remains the runtime source of truth.
- **Dependency maintenance**: refreshed direct, security-sensitive, resolver, and Unicode dependencies while retaining the `go 1.25.0` module floor and existing MPL-2.0 exception for unmodified `filepath-securejoin`.
- **Foundation recommendations**: refreshed recommended tool versions without raising minimum versions or changing the v0.4.2 CI runner; Syft 1.50.0 is a narrow named-High remediation exception, and Homebrew reached that version before release closeout.

## [v0.5.14] - 2026-07-07

### Added

- **File-selection and ignore semantics app note**: documented how `.gitignore`, `.goneatignore`, built-in generated defaults, `--no-ignore`, and `--force-include` apply across assessment categories and external tools.
- **Starter `.goneatignore` templates**: `goneat init` now writes language-specific starter ignore files from source templates, including shared generated/tooling defaults for `.cache/`, `bin/`, `dist/`, `sbom/`, and `vendor/`.
- **Vulnerability source provenance**: dependency vulnerability reports and JSON issues now include source type and source path metadata for Go module graph, SBOM-file, and fallback file-walk scans.

### Fixed

- **Security scan scope**: gosec package discovery now honors goneat's unified ignore matcher before nested module and package inputs are passed to gosec, reducing false findings from ignored/generated paths.
- **Dependency vulnerability scope**: Go vulnerability scans now use a module-graph SBOM from `go list -m -json all`, avoiding recursive workspace scans of caches, generated SBOMs, release artifacts, and vendored examples.
- **Install replacement behavior**: `make install` removes an existing installed binary before copying the replacement, avoiding platform-specific overwrite failures.

### Changed

- **Dependency updates**: refreshed key Go modules including `golang.org/x/crypto` v0.53.0, `golang.org/x/net` v0.56.0, `github.com/go-git/go-git/v5` v5.19.1, `github.com/open-policy-agent/opa` v1.18.2, and the OpenTelemetry OTLP trace HTTP exporter v1.44.0 while keeping `go.mod` at `go 1.25.0`.

## [v0.5.13] - 2026-06-05

### Added

- **Markdown AI transcript artifact linting**: `goneat assess --categories lint` now detects common AI transcript fragments in authored Markdown, including orphan content closing tags, xai function-call closing fragments, and parameter-name blocks. The scanner is fence-aware, so examples inside fenced code blocks do not suppress later real findings.

### Fixed

- **YAML format check/apply agreement**: `goneat format --check` now compares YAML after the same finalizer normalization used by apply mode, eliminating false positives where files were already stable after formatting.
- **SSOT metadata indentation**: `goneat ssot sync` now writes metadata YAML with the repository's canonical 2-space indentation instead of regenerating `.crucible/metadata/metadata.yaml` at 4 spaces.

### Changed

- **YAML format noise cleanup**: pre-existing goneat-owned YAML comment-padding and indentation drift has been normalized so the repository starts v0.5.13 clean.
- **Crucible-synced mirror handling**: Crucible-synced config and schema mirrors are now excluded from local format drift checks so upstream-owned content is not churned by goneat-local formatting.
- **README language matrix**: Markdown now advertises both built-in artifact linting and prettier-backed formatting, while JSON remains format-only.

## [v0.5.12] - 2026-05-27

### Fixed

- **`goneat format` vs `yamlfmt -lint` divergence**: the sequential `cmd/format.go::formatYAMLFile` path silently omitted `pad_line_comments` when invoking `yamlfmt`, so `goneat format` was a no-op on YAML files that the parallel/assess paths would correctly flag. All three call sites — sequential format, parallel format, and assess check (`pkg/work/format_processor.go::checkYAMLFile`) — now share a single arg builder, `pkg/config.YAMLFormatConfig.YamlfmtFormatterArgs`. Reported by limensafe via kilo-devlead.
- **`Lint: error` in offline/sandboxed dev**: `internal/assess/lint_runner.go::verifyGolangciConfig` now classifies golangci-lint stderr against 9 network-error patterns (`dial tcp`, `no such host`, `i/o timeout`, `failed to get schema`, `connection refused`, `network is unreachable`, `tls handshake timeout`, `context deadline exceeded`, `temporary failure in name resolution`) and demotes those to a warn-skip. Structural schema errors still propagate. Reported by @agent-india-devlead during v0.5.11 validation.
- **CI license-audit silent false-green**: `make license-audit` invoked `rg "$forbidden"` inline; CI runners without ripgrep produced `rg: not found` (non-zero) which the `if` branch treated as "no forbidden licenses." Replaced with `grep -E` so the matcher cannot silently fail. Added an explicit Makefile allowlist filter for `(github.com/cyphar/filepath-securejoin, MPL-2.0)` — a transitive bounded-filesystem dependency of `go-git`/`go-billy` required by the v5.9.0 / v5.19.0 bumps in this release. Narrow exception recorded in `.goneat/dependencies.yaml` per @agent-entarch-fulmenhq's review, approved by @3leapsdave, revisit at v0.5.13/v0.6.0.
- **`scripts/verify-release-assets.sh` false-positive checksum mismatch**: all three SHA256/SHA512 compares now sort by filename column (`sort -k 2`) so identical content with different line ordering compares clean. Also fixed a pre-existing logic bug where `LOCAL_SHA256_SORTED` and `local_sorted` pointed at the same file, causing BSD `cp` to exit 1.
- **`.git/**` excludes in assess templates fail in linked worktrees**: swept `.git/**` → `.git` in all five `templates/assess/*.yaml` SSOTs. The `**` form trips when `.git` is a gitfile (worktree) instead of a directory.

### Changed

- **Dependency bumps**: `github.com/go-git/go-git/v5` 5.16.5 → 5.19.0, `github.com/go-git/go-billy/v5` 5.7.0 → 5.9.0, `google.golang.org/grpc` 1.78.0 → 1.81.1, `go.opentelemetry.io/otel/sdk` 1.40.0 → 1.43.0, `golang.org/x/crypto` 0.47.0 → 0.51.0. Resolver-permitted `golang.org/x/*` group coherence lifted (`sys`, `text`, `net`, `sync`, `tools`, `mod`, `exp`). `github.com/cyphar/filepath-securejoin` pulled to v0.6.1 by the go-git/go-billy upgrade (covered by the MPL-2.0 allowlist). Scope re-validated against `go list -m -u` by @agent-kilo-devrev at branch time.
- **YAML format/lint alignment guidance**: `docs/appnotes/yaml-format-lint-alignment.md` and `docs/user-guide/commands/format.md` now lead with a ⚠️ callout that `.yamlfmt` MUST set `formatter.pad_line_comments: 2` whenever direct `yamlfmt` is invoked outside goneat (CI, hooks, IDE integrations). Added a "Direct `yamlfmt` callouts" three-pattern subsection and a "Symptom of misalignment" debug aid that names the exact diff users will see.
- **CI Go runtime alignment (security-driven)**: `.github/workflows/ci.yml` now uses `ghcr.io/fulmenhq/goneat-tools-runner-glibc:v0.4.2` (bumped from `:v0.4.1`), which bundles Go 1.26.2 and golangci-lint v2.12.1 — picks up the Go 1.26.x CVE fixes (notably CVE-2026-33810) that landed in fulmen-toolbox v0.3.5. `.github/workflows/release.yml` and `.github/workflows/license-audit.yml` move from `go-version: '1.25.x'` to `'1.26.x'` for matching coverage in the non-containerized jobs. `go.mod`'s `go 1.25.0` directive is intentionally unchanged — the floor for downstream consumers stays where it was.

## [v0.5.11] - 2026-05-17

### Fixed

- **Pre-commit hang in dates assessment under `--staged-only`**: `goneat assess --hook pre-commit --staged-only --package-mode` with `dates` in the categories no longer hangs. The dates assess runner previously looped a full-repository dates scan once per file in `IncludeFiles`, multiplying a 3-second scan into multi-minute work and duplicating every finding N times. The runner now does a single scoped scan over the include set via `internal/dates/dates.DatesRunner.Assess`'s explicit-files path.
- **Hook manifest timeouts now enforced for internal commands**: per-command `timeout` values in `.goneat/hooks.yaml` now reach the internal `assess`/`format`/`dependencies` handlers. Previously `cmd/assess.go::runInternalAssess` passed `cmd.Context()` to the engine instead of the `context.WithTimeout`-wrapped ctx supplied by `HookExecutor`, so manifest timeouts were silently dropped.
- **Dates runner observes context cancellation**: the runner now checks `ctx.Done()` in the directory-discovery loop, in the `filepath.WalkDir` callback, at each worker iteration, and after `wg.Wait()`, surfacing `context.DeadlineExceeded` instead of running a stuck scan to completion as a "successful" partial result.
- **Explicit empty include set is honored**: when staged-file filtering drops every file (e.g. all matched `.goneatignore` patterns), the dates runner now scans zero files instead of falling back to a full-repo discovery.
- **STDOUT hygiene in dates assess wrapper**: removed a stray `fmt.Printf("DEBUG: …")` that printed to STDOUT on per-file processing errors.

### Changed

- **CI runner image pinned**: `.github/workflows/ci.yml` container jobs now use `ghcr.io/fulmenhq/goneat-tools-runner-glibc:v0.4.1` instead of `:latest`, eliminating floating-tag drift across PRs and `main`.
- **CI lint anchors to runner image**: `build-test-lint` invokes the runner-bundled `golangci-lint` binary instead of `golangci/golangci-lint-action@v7`, removing CI's dependency on a network fetch from `golangci-lint.run`. Lint remains advisory (`continue-on-error: true`); the active `go version` and `golangci-lint --version` are logged for auditability.

## [v0.5.10] - 2026-03-30

### Fixed

- **License exception evaluation in assess**: `goneat assess --categories dependencies` now honors `licenses.exceptions` from `.goneat/dependencies.yaml` before emitting forbidden-license findings, including exact package/license matching and date-based activation with `approved_date` and `until`.
- **Dependency policy schema drift**: the embedded `dependencies-policy-v1.0.0` schema now accepts the vulnerability allowlist metadata already used by goneat's own repo policy (`status`, `sdr`, `analysis`, `verified_by`, `verified_date`), removing the recurring `dependencies: policy failed schema validation` warning from dogfood dependency assessment.

### Changed

- **Dependency policy docs/schema alignment**: license exception examples, troubleshooting guidance, and the dependency policy schema now document temporary license overrides more accurately, including optional expiry dates.

---

**Note**: Older releases (v0.5.9 and earlier) are archived in [`docs/releases/`](docs/releases/).
