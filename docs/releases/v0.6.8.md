# Goneat v0.6.8 — JSON Schema Engine, Canonical JSON and Advisory Policy

**Release Date**: 2026-11-10
**Status**: Stable

## TL;DR

- **New JSON Schema engine.** Schema validation uses [santhosh-tekuri/jsonschema](https://github.com/santhosh-tekuri/jsonschema) v6.0.3 behind one adapter, replacing `gojsonschema`. `$schema` is honoured, `format` is asserted, references are closed, and JSON data is read exactly. Some data and schemas that passed before now fail; see [Upgrading](#upgrading).
- **`goneat canon`.** RFC 8785 canonical JSON, digests and test vectors, with NDJSON arrays and sorted sets.
- **govulncheck allowlist is repository policy.** Findings are downgraded only by advisory IDs listed in `.goneat/dependencies.yaml`.
- **Advisory tools.** Tools can be marked advisory per platform; a missing advisory tool does not fail doctor or the tools assessment.
- **Cooling exceptions fail closed** when `until` is malformed.

## What Changed

### JSON Schema validation

Every path that compiles a schema to validate data uses the new engine: `schema validate-data`, `validate suite`, the config-to-schema checks of the `schema` assessment, the `pkg/schema` library, and goneat's own config, hooks manifest, dependency policy and source SBOM checks. `schema validate-schema` checks schemas against the engine's built-in metaschemas.

- **Dialects.** `$schema` is honoured for Draft-04, -06, -07, 2019-09 and 2020-12. A schema without `$schema` validates as Draft-07. The unversioned `https://json-schema.org/schema` alias and Draft-03 are refused.
- **Formats.** `format` is asserted in every dialect. Unknown format names are schema errors rather than being skipped. `date`, `time`, `date-time`, `email` and `idn-email` use goneat's own syntax checkers; `time` accepts second 60 only at UTC-equivalent 23:59:60, and `date-time` also requires the last day of the UTC month.
- **References.** `$ref` resolves within the schema, to `--ref-dir` schemas by `$id`, and to the built-in metaschemas. Nothing is fetched, `file:` URLs are refused, and user schemas may not claim `json-schema.org` identifiers.
- **Data.** JSON data and configuration files are decoded exactly: numbers keep their written value, and invalid UTF-8 or unpaired surrogate escapes are refused. YAML data is unchanged.
- **Reports.** `validate-data` JSON output and `pkg/schema.Result` add `engine`, `dialect`, `dialect_source`, `format_mode` and `leap_policy`. Error paths keep their dotted form; messages use the engine's wording. Schemas the engine refuses are reported as `AdmissionError`, distinct from data that fails validation.
- **Hooks manifest.** The hooks manifest schema declares Draft-07 and carries its URL as `$id`.

Details are in [Validation behaviour](../user-guide/commands/schema.md#validation-behaviour) and [Owned format checkers](../user-guide/commands/schema.md#owned-format-checkers).

### Canonical JSON

`goneat canon jcs [file|-]` writes RFC 8785 (JCS) canonical bytes using the pure-Go `github.com/3leaps/canonprims` library; no external command or CGo is needed.

- `--check` succeeds only for byte-identical canonical input; `--digest` prints `sha256:<hex>`; `--vector` prints a JSON object with the canonical text, byte length, digest and scheme.
- `--ndjson-array` canonicalizes LF-terminated lines into an array, optionally limited by `--head N`. `--sort-set` sorts a top-level array by canonical bytes and refuses duplicates.
- Input size, depth, object members and string bytes are bounded by flags. Refusals have stable identifiers such as `duplicate_member` and `lone_surrogate`.
- `goneat canon info` describes the scheme and the library's conformance corpus.

See [canon](../user-guide/commands/canon.md).

### Security policy

- govulncheck findings are downgraded only by advisory IDs listed in `.goneat/dependencies.yaml`; goneat no longer classifies any advisory itself. An allowlisted finding is reported as info only after a complete, successful run with consistent report evidence. Unlisted, expired, malformed or duplicate entries leave the finding high. When an allowlisted advisory names a tagged fix release, goneat adds an info hint to upgrade.
- Cooling exceptions fail closed when `until` is malformed. `until` is read as literal text and is exclusive at 00:00 UTC. A present but null, empty, malformed, wrong-typed or time-of-day value is a config error: the exception is dropped and a high `exception_config_invalid` violation fails the gate, even under `alert_only` or the grace period.

### Tools

- `advisory` is a per-platform map (`all`, `darwin`, `linux`, `windows`); a platform value, including `false`, overrides `all`. A missing advisory tool does not fail `doctor tools` or the tools assessment, and doctor prints install instructions instead of installing. Installed advisory tools still get version-policy checks. Linux minisign is advisory.

## Security

govulncheck v1.9.0, run with Go 1.26.10, reports no vulnerabilities in code goneat calls or packages it imports. One advisory in a required module is unreachable:

| Advisory     | Module                | Status                                                   |
| ------------ | --------------------- | -------------------------------------------------------- |
| GO-2026-5932 | `golang.org/x/crypto` | Allowlisted; goneat imports no package from the advisory |

GO-2026-5932 covers `golang.org/x/crypto/openpgp`. goneat uses the ProtonMail fork and does not import `golang.org/x/crypto/openpgp`.

`golang.org/x/net` v0.62.0 has left its cooling period, and its cooling exception is removed. New direct modules are `github.com/santhosh-tekuri/jsonschema/v6` v6.0.3 (Apache-2.0) and `github.com/3leaps/canonprims` v0.1.4. `github.com/xeipuuv/gojsonschema` is removed.

## Known Issues

- The vendored Crucible schema `schemas/crucible-go/config/standards/v1.0.0/adr-lifecycle-status.json` is not a valid Draft 2020-12 schema, and `schema validate-schema` now reports it. The correction belongs upstream in Crucible and will be synced in a later release.

## Upgrading

Building from source or with `go install` requires **Go 1.26.10 or later**.

Check your schemas and data before upgrading:

- Repositories that relied on goneat's built-in GO-2026-5932 classification must add an entry for it to `.goneat/dependencies.yaml`.
- Cooling exceptions with a malformed `until` no longer apply.
- Schemas can now be refused: unknown `format` names, references outside the schema and `--ref-dir`, the unversioned `$schema` alias, `json-schema.org` identifiers in user schemas, and invalid schemas.
- Data can now fail: malformed dates and times, out-of-range offsets, misplaced leap seconds, invalid mailboxes, keywords next to `$ref` in 2020-12 schemas, inexact JSON numbers, and JSON with invalid UTF-8 or unpaired surrogate escapes.
- Data can now pass: valid leap-second spellings, lowercase `t` and `z` in dates and times, valid mailboxes that were rejected before, and non-string values under a format-only schema.
- Tooling that matches validation error text should match on path and verdict instead.
- `GONEAT_OFFLINE_SCHEMA_VALIDATION` no longer has any effect.

v0.6.8 release archives are Darwin ARM64, Linux amd64, Linux ARM64, Windows amd64, and Windows ARM64. Darwin amd64 downloads end at v0.6.1. Verify downloads as described in the [v0.6.4 notes](https://github.com/fulmenhq/goneat/blob/v0.6.8/docs/releases/v0.6.4.md#verifying-this-release), using `RELEASE_VERSION="v0.6.8"`.
