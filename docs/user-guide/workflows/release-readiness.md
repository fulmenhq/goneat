# Release Readiness Workflow

## Overview

This workflow guides you through preparing a repository for release using goneat's repository management and maturity validation tools. It ensures consistency across phases (dev → rc → release), git state, versions, documentation, and schemas. The process integrates with git hooks, assess, and CI/CD for automated enforcement.

**Key Tools**:

- `repository phase set/show` - Manage RELEASE_PHASE/LIFECYCLE_PHASE.
- `maturity validate/release-check` - Validate health/readiness.
- `assess --categories maturity` - Full scan integration.
- Hooks (pre-commit/pre-push) - Gate bad states.

**Phases Covered**:

- **Dev**: Feature development (lenient: allow dirty git, 50% coverage).
- **RC**: Release candidate (strict: clean git, 75% coverage, docs required).
- **Release**: Production (90% coverage, no suffixes, full validation).
- **Hotfix**: Urgent fixes (80% coverage, clean git).

Configure policies in `.goneat/phases.yaml` (see [Phases Schema](../standards/phases-schema.md)).

## Workflow Steps

### 1. Development Phase (Dev/Alpha)

**Goal**: Build features without strict gates.

**Commands**:

```
# Set initial phase
goneat repository phase set --release dev --lifecycle alpha

# Daily validation (warnings only)
goneat maturity validate --level warn

# Full assess with maturity
goneat assess --categories maturity,lint,format

# Run tests (includes Tier 1 integration)
make test
```

**Hooks**:

- Pre-commit: `goneat maturity validate --level warn` (catches basic drifts).
- Expected Issues: Low coverage warnings (alpha:50%); uncommitted OK.

**Best Practices**:

- Commit frequently; use `--dry-run` for phase previews.
- Monitor: `goneat policy show` to review dev rules (e.g., "-dev" suffix).

### 2. Testing Phase (RC/Beta)

**Goal**: Prepare for release candidate; enforce cleanliness.

**Commands**:

```
# Transition to RC
goneat repository phase set --release rc --lifecycle beta

# Release readiness check
goneat maturity release-check --phase rc --strict

# Validate policies/docs
goneat repository policy validate --level error

# Assess full repo
goneat assess --categories maturity,security,tools --json > readiness.json

# Integration testing (quick validation)
export GONEAT_COOLING_TEST_ROOT=$HOME/dev/playground
make test-integration-cooling-quick  # Tier 2: ~8s, Hugo baseline
```

**Integration Testing**:

Quick validation recommended for RC:

```bash
# Set test repo location
export GONEAT_COOLING_TEST_ROOT=$HOME/dev/playground

# Run Tier 2 quick test (Hugo baseline)
make test-integration-cooling-quick

# Expected: < 15s, < 10% violations
```

**Expected Results**:

- Hugo baseline: ~8s (warm cache)
- Violations: < 10% (1-2 expected)
- All registry calls cached on second run

````

**Hooks**:

- Pre-push: `goneat maturity release-check --phase rc --level error` (fails on dirty git/mismatches).

**Expected Checks**:

- Git: Clean (no uncommitted); main branch.
- Version: Suffix "-rc.1" in VERSION; CHANGELOG entry.
- Docs: RELEASE_NOTES.md present.
- Coverage: 75% min (exceptions for node_modules=0%).

**CI/CD Integration** (GitHub Actions Example):

```yaml
name: RC Validation
on: [push]
jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: curl -sSfL https://goneat.dev/install.sh | sh
      - run: goneat repository phase set --release rc --lifecycle beta
      - run: goneat maturity release-check --phase rc --strict --json | jq '.ready'
      - run: if [ "$(goneat assess --categories maturity --json | jq '.issues | length')" -gt 0 ]; then exit 1; fi
````

### 3. Production Release (Release/GA)

**Goal**: Final validation for production.

**Commands**:

```
# Set to release
goneat repository phase set --release release --lifecycle ga

# Strict readiness
goneat maturity release-check --phase release --strict

# Policy validation
goneat repository policy validate --level error

# Full release assess
goneat assess --categories all --output json > release-report.json

# Integration testing (comprehensive for major releases)
export GONEAT_COOLING_TEST_ROOT=$HOME/dev/playground
make test-integration-extended  # All 3 tiers, ~2 minutes
```

**Integration Testing**:

Comprehensive validation for major releases (v0.3.0+):

```bash
# Set test repo location
export GONEAT_COOLING_TEST_ROOT=$HOME/dev/playground

# Run all 3 tiers
make test-integration-extended

# Document results
cat /tmp/goneat-phase4-full-suite.log > dist/release/integration-test-results.log
```

**Expected Results**:

- Tier 1: PASS (< 10s)
- Tier 2: PASS (~8s warm)
- Tier 3: 6/8 PASS (2 known non-blocking failures)

```

**Hooks**:

- Pre-push to main: Full `release-check --strict` + assess.

**Expected Checks**:

- Git: Clean + tagged (vX.Y.Z, no suffix).
- Version: Matches CHANGELOG/RELEASE_NOTES; 90% coverage.
- Schema: All configs valid.

**Best Practices**:

- Run in CI before tagging: Fail if not ready.
- Post-release: Set to maintenance for hotfixes.

### 4. Hotfix/Maintenance

**Goal**: Quick fixes post-release.

**Commands**:

```

# Hotfix mode

goneat repository phase set --release hotfix --lifecycle maintenance

# Check

goneat maturity release-check --phase hotfix

# Assess focused

goneat assess --categories maturity,security

```

**Integration**: Similar to RC but 80% coverage; allow limited dirty git.

## Troubleshooting

- **Phase Mismatch**: If suffix doesn't match (e.g., no "-rc.1"), update VERSION and validate.
- **Dirty Git**: Run `git status`; commit/add before phase set.
- **Missing Docs**: Add required files per policy show.
- **Coverage Low**: Use coverage_exceptions in phases.yaml for overrides (e.g., tests=100%).

## Advanced Configuration

- **Custom Phases**: Edit `.goneat/phases.yaml` (validated on load).
- **Exceptions**: For multi-lang: `coverage_exceptions: {"src/js/**": 70, "tests/**": 100}`.
- **Error Levels**: Set "skip" for non-critical in dev.

For command details, see [Repository Commands](../commands/repository.md) and [Maturity Commands](../commands/maturity.md). For standards, [Validation Policies](../standards/validation-policies.md).
```
