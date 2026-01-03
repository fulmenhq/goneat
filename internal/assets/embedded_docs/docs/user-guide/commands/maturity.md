# Maturity Validation Commands

## Overview

The `maturity` commands provide read-only validation of your repository's health and release readiness. They check git state, version consistency, documentation synchronization, and schema validity based on your current phases (RELEASE_PHASE and LIFECYCLE_PHASE). These tools help prevent common issues like uncommitted changes during releases or version drifts in documentation.

The `maturity` group is part of the **neat** category under validation. It integrates with the `assess` command for automated checks and git hooks for pre-commit/pre-push gates. Use these in CI/CD to enforce standards without mutating your repo.

## Phase Values (SSOT: Crucible Schemas)

Phase validation follows **crucible schemas** as the single source of truth:

| Phase File | Schema Path | Valid Values |
|------------|-------------|--------------|
| `LIFECYCLE_PHASE` | `schemas/crucible-go/config/repository/v1.0.0/lifecycle-phase.json` | `experimental`, `alpha`, `beta`, `rc`, `ga`, `lts` |
| `RELEASE_PHASE` | `schemas/crucible-go/config/goneat/v1.0.0/release-phase.json` | `dev`, `rc`, `ga`, `release` |

**Schema distinction:**
- **lifecycle-phase**: Repo-level concept (product maturity) - shared across all FulmenHQ tools
- **release-phase**: Tool-specific (goneat deployment gates) - `release` is equivalent to `ga`

These schemas are synced from [fulmenhq/crucible](https://github.com/fulmenhq/crucible) and embedded in goneat. When crucible schemas are updated, goneat will automatically reflect the changes upon rebuild.

## Synopsis

```
goneat maturity [command] [flags]
```

### Available Subcommands

- `validate` - Full repository maturity validation
- `release-check` - Phase-specific release readiness check

## Commands

### validate

**Description**: Runs comprehensive maturity validation across git state, version consistency, documentation, and schemas. Outputs issues with levels (warn/error) based on phase policies. Use `--json` for structured output in scripts.

**Synopsis**:

```
goneat maturity validate [flags]
```

**Flags**:

- `--level string` (default "warn"): Set error level (warn, error) for issues.
- `--json` (bool): Output in JSON format for CI/parsing.

**Examples**:

```
# Basic validation (human-readable warnings)
goneat maturity validate

# Strict validation (fail on any issue)
goneat maturity validate --level error

# JSON for CI
goneat maturity validate --json

# Output example (human-readable):
✅ Maturity validation passed
(or)
⚠️  maturity: Dirty git state (uncommitted changes) not allowed in rc phase (level: error, fix: git add . && git commit)
⚠️  maturity: CHANGELOG.md missing entry for v0.2.5-rc.1 (level: warn, fix: Add ## [v0.2.5-rc.1] section)

# JSON example:
{
  "category": "maturity",
  "issues": [
    {"message": "Dirty git state...", "level": "error", "fix": "git add ."}
  ]
}
```

**Integration**:

- Assess: `goneat assess --categories maturity` (includes in full scans).
- Hooks: Pre-commit: `goneat maturity validate --level warn`; Pre-push: `--level error`.
- CI: Parse JSON for gates (e.g., exit 1 if errors >0).

### release-check

**Description**: Validates release readiness for a specific RELEASE_PHASE (e.g., rc). Checks phase-specific rules like git cleanliness, version suffixes, and required docs. `--strict` upgrades warnings to errors.

**Synopsis**:

```
goneat maturity release-check --phase [dev|rc|ga|release] [flags]
```

**Flags**:

- `--phase string` (Required): The RELEASE_PHASE to check against.
- `--strict` (bool): Treat warnings as errors.
- `--json` (bool): JSON output.

**Examples**:

```
# Check for RC readiness
goneat maturity release-check --phase rc

# Strict check for release
goneat maturity release-check --phase release --strict

# JSON output
goneat maturity release-check --phase ga --json

# Output example:
Release check for rc: ❌ Not ready (2 issues)
- Dirty git: 3 uncommitted files (fix: git commit)
- Version suffix mismatch (fix: append -rc.1 to VERSION)

# JSON example:
{
  "category": "maturity",
  "phase": "rc",
  "ready": false,
  "issues": [...]
}
```

**Integration**:

- Release Scripts: `goneat maturity release-check --phase ga --strict || echo "Fix issues before release"`
- Hooks: Pre-push for RC: `goneat maturity release-check --phase rc --level error`
- With Phases: Automatically uses current phase from `repository phase show` if --phase omitted in future.

## Configuration

Maturity checks use phases defined in `.goneat/phases.yaml` (see [Phases Configuration](../commands/repository.md#configuration)). Defaults enforce basic rules (e.g., clean git for rc+). Customize error levels and exceptions (e.g., lower coverage for tests).

## Best Practices

- Run `validate` daily or in pre-commit hooks to catch drifts early.
- Use `release-check` as a gate before tagging releases.
- Combine with `assess`: `goneat assess --categories maturity,lint,security` for full health check.
- For teams: Set policies in phases.yaml (e.g., ga requires 90% coverage); validate in CI.

For workflows using these commands, see [Release Readiness Workflow](../workflows/release-readiness.md). For advanced configuration, refer to [Standards](../standards/validation-policies.md).
