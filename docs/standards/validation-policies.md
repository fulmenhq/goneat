# Validation Policies Standards

## Overview

Goneat's validation policies define rules for repository health, release phases, and compliance. These are configurable via `.goneat/phases.yaml` and enforced in maturity commands, assess categories, and hooks. Policies ensure consistency (e.g., clean git in release) while allowing customization (e.g., lower coverage for prototypes).

**Key Principles**:

- **Phased Enforcement**: Rules vary by RELEASE_PHASE (dev lenient, release strict).
- **Extensible**: Schema-validated; metadata for features like coverage thresholds.
- **Lang-Agnostic**: Globs/exceptions work for Go/JS/Python (future parsers).
- **Audit-Ready**: Violations logged to Pathfinder audit for compliance (SOC2/HIPAA).

## Policy Structure

Policies are defined in `.goneat/phases.yaml` (see [Phases Schema](../schemas/repository/v1.0.0/phases.yaml)). Core elements:

### RELEASE_PHASE Policies

- **dev**: Lenient (dirty git OK, 50% coverage, suffixes: -dev/-alpha).
- **rc**: Medium (clean git, 75% coverage, suffixes: -rc.1, required: CHANGELOG+RELEASE_NOTES).
- **release**: Strict (90% coverage, no suffixes, all docs).
- **hotfix**: Balanced (80% coverage, clean git, -hotfix.1).

**Config Example**:

```yaml
release_phases:
  rc:
    allowed_suffixes: ["-rc.1"]
    min_coverage: 75
    allow_dirty_git: false
    coverage_exceptions: { "tests/**": 100, "node_modules/**": 0 }
```

### LIFECYCLE_PHASE Policies

- **alpha**: Early (50% coverage).
- **beta**: Testing (75%).
- **ga**: Production (90%).
- **maintenance**: Sustained (80%, 1Y support).

**Coverage Keying**: Use `min_coverage` + exceptions for thresholds (e.g., GetAdjustedCoverage("beta", "src/js") → 75 or override).

## Enforcement Levels

- **warn**: Log issue but continue (dev/alpha default).
- **error**: Fail command/hook (rc+ default).
- **skip**: Ignore for phase (custom).

Set per-phase or global in config.

## Integration Standards

### Assess & Hooks

- **Assess**: `--categories maturity` runs policy validation; JSON for CI.
- **Hooks**: Pre-commit: warn level; Pre-push: error for release phases.
- **Registry**: Policies registered under GroupNeat/CategoryValidation.

### Multi-Language Support

- Exceptions use globs (doublestar): "src/python/\*\*":70 for Python repos.
- Future: Lang parsers (e.g., JS coverage via npm) query adjusted thresholds.

### Compliance

- **Audit**: Violations → Pathfinder log (e.g., "maturity:dirty_git" with fix).
- **SOC2/HIPAA**: Enforce clean git/docs for ga; retention via phases support_duration.

## Best Practices

- **Team Policies**: Customize phases.yaml; commit to repo.
- **CI Gates**: `goneat maturity release-check --phase release --strict --json | jq '.ready' == "true"`
- **Migration**: From literal files: `goneat repository phase set --release rc` auto-generates yaml.
- **Troubleshooting**: `goneat policy show` for rules; `goneat repository policy validate` for checks.

For workflows, see [Release Readiness](../workflows/release-readiness.md). For schema details, [Phases Schema](../schemas/repository/v1.0.0/phases.yaml).
