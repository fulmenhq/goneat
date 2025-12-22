# Goneat v0.3.23 — Bulk Validate Suite + Local Schema DX + Offline Refs

**Release Date**: 2025-12-21
**Status**: Draft

## TL;DR

- **Bulk validation**: `goneat validate suite` validates many YAML/JSON files in one run (parallel workers)
- **CI/AI friendly output**: `--format json` includes per-file results + summary
- **Local schema DX**: `schema_path` in `.goneat/schema-mappings.yaml` maps patterns directly to local schema files
- **Offline-first**: `--ref-dir` resolves absolute `$ref` URLs without a live schema registry
- **Release docs in CLI**: `goneat docs show release-notes` and `goneat docs show releases/latest`

## What Changed

### Validate: new `suite` subcommand (bulk)

`goneat validate suite` validates many files in one run, with:

- `--workers` for parallelism
- `--expect-fail` and `--skip` globs for invalid/taxonomy fixtures
- stable JSON output for CI + AI parsing

### Validate suite: local schema workflow

Two supported ways to route files to local schemas:

- **Recommended**: `schema_path` shorthand in `mappings`
- **Optional**: `overrides.path` + `schema_id` for stable, canonical identifiers

### Offline `$ref` resolution (friction-free)

- `validate data --ref-dir` handles root-vs-ref-dir duplicate `$id` collisions correctly
- `validate suite --ref-dir` supports offline `$ref` resolution for schema ecosystems before a registry exists

### Docs: release notes available via `goneat docs`

- `goneat docs show release-notes` shows the curated recent release notes.
- `goneat docs show releases/latest` shows the current release note.

---

# Goneat v0.3.22 — Assess Scaffolding + Hooks UX + Offline Schema Refs

**Release Date**: 2025-12-20
**Status**: Draft

## TL;DR

- **Scaffold assess config**: `goneat doctor assess init` generates a starter `.goneat/assess.yaml`
- **Hooks transparency**: `goneat hooks validate/inspect` now show effective behavior + warn on mutators
- **Machine-readable output**: `--format json` for `hooks validate` and `hooks inspect`
- **Safer hook scripts**: bash hooks disable glob expansion (`set -f`)
- **Offline schema refs**: `goneat validate data --ref-dir` resolves remote `$ref` URLs from local schema directories

## What Changed

### Doctor: `.goneat/assess.yaml` scaffolding

`goneat doctor assess init` seeds a starter `.goneat/assess.yaml` tailored to your repo type.

### Hooks: inspection + warnings

Hooks commands now help answer: “What will my hooks actually do?”

- effective wrapper invocation
- internal vs external command classification
- mutator detection (e.g., `format`, `assess --fix`, `stage_fixed`, `make precommit`)

### Hooks: JSON output

Use JSON output for automation and CI policy checks:

- `goneat hooks inspect --format json`
- `goneat hooks validate --format json`

### Validate: offline `$ref` resolution

`goneat validate data --ref-dir` can preload local schema directories so absolute `$ref` URLs resolve without a live schema registry.

### Hooks generation: glob safety

Generated bash hooks now include `set -f` to prevent glob patterns from expanding into many arguments.

---

# Goneat v0.3.21 — Dependencies Reliability + Tool Cooling Metadata

**Release Date**: 2025-12-15
**Status**: Draft

## TL;DR

- **Go 1.25 compatibility**: `goneat dependencies` no longer fails on stdlib “module info” errors
- **Better cooling checks**: `goneat doctor tools` can evaluate cooling for more tools (fewer "metadata unavailable" failures)
- **Shell lint compatibility**: shfmt lint can match repo style via `lint.shell.shfmt.args`
- **Repo lint debt cleanup**: checkmake backlog cleared without raising thresholds

## What Changed

### Dependencies: Go 1.25 stdlib module-info failures fixed

Some Go 1.25.x environments hit repeated stdlib errors originating from go-licenses:

- `Package <stdlib> does not have module info. Non go modules projects are no longer supported.`

v0.3.21 decouples dependency discovery from go-licenses:

- Cooling/policy module discovery uses `go list -deps -json` and skips stdlib packages.
- go-licenses runs only when `--licenses` is requested.
- If license extraction is degraded, goneat falls back to best-effort license file detection from module directories.

### Doctor tools: cooling metadata for more install types

`goneat doctor tools` now resolves upstream metadata for more tools:

- GitHub repo inference for `kind: go` tools via `install_package`
- PyPI metadata for tools installed via uv/pip (e.g. yamllint)

### Lint: shfmt style args override

Forge repos commonly standardize `shfmt` flags (indentation, continuation indentation). v0.3.21 adds an opt-in override so `goneat assess --categories lint` can apply the same style:

- `.goneat/assess.yaml`: `lint.shell.shfmt.args: ["-i", "4", "-ci"]`
- goneat still controls `-d`/`-w` (check vs fix mode)

### Lint: Makefile checkmake backlog cleared

- Repo config sets `lint.make.checkmake.config.max_body_length: 15`
- Refactored large Make targets into helper targets to stay within limit

---

**Previous Releases**: See `docs/releases/` for older release notes.
