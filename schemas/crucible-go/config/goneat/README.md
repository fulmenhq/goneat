# Goneat Configuration Schemas

This directory contains JSON Schema definitions that codify configuration and policy
files consumed by the goneat toolchain. Each schema is published at
`https://schemas.fulmenhq.dev/config/goneat/<name>-<version>.schema.json` and should
be referenced from the corresponding YAML/JSON policy files via the `$schema` field
whenever possible.

## Available Schemas

| Schema                                        | Purpose                                                          |
| --------------------------------------------- | ---------------------------------------------------------------- |
| `goneat-config.yaml`                          | Core goneat CLI configuration (toolchain sources, telemetry opt) |
| `dates.yaml`                                  | Date formatting conventions for release automation               |
| `security-policy.yaml`                        | Security scanning and suppression policy                         |
| `lifecycle-phase.json` / `release-phase.json` | Lifecycle metadata used by release automation                    |
| `version-policy.schema.yaml`                  | Version SSOT propagation policy (new in v0.3.0)                  |

## Version Policy (`version-policy.schema.yaml`)

The version policy schema enables repositories to describe how the authoritative
`VERSION` file propagates into downstream package manifests using goneat’s
`version propagate` command. Policies live at `.goneat/version-policy.yaml` in consumer
repositories and should reference the schema via:

```yaml
$schema: https://schemas.fulmenhq.dev/config/goneat/version-policy-v1.0.0.schema.json
```

Key capabilities:

- **Schemes & Channels**: declare whether the repo uses SemVer or CalVer and optionally
  constrain prerelease channels.
- **Propagation Rules**: control default include/exclude patterns, handler-specific
  overrides, workspace behaviour, and backup retention.
- **Content Rules vs Guards**: `rules` describes version content constraints (channels,
  tags, prerelease allowances) while `guards` enforce execution preconditions (branch
  allow-lists, dirty worktree checks). Both sections are optional—planned for Phase 3a of
  goneat’s roadmap.
- **Extensibility**: `metadata` allows teams to attach internal annotations that goneat
  will ignore while keeping the schema forward-compatible.

The schema keys align with manifest filenames (e.g., `package.json`, `pyproject.toml`,
`go.mod`). The workspace strategy supports three patterns:

- `single-version`: all manifests inherit the root VERSION.
- `opt-in`: only globs listed in `allowlist` receive independent version control.
- `opt-out`: everything propagates except paths explicitly listed in `blocklist`.

See the schema `examples` block for a reference configuration that covers common
use cases (JS + Python monorepo with Go validation only). Additional cookbook
samples will ship with goneat v0.3.0 documentation.

## Contributing

- Update the `$id` to point at the published schema URL when introducing new versions.
- Keep the schema version number in lock-step with goneat releases that introduce
  breaking changes to the configuration surface.
- Whenever a schema is added or updated, ensure `make check-all` passes so that
  downstream language wrappers receive the new definition during sync.
