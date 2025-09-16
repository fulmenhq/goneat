# Repository Management Commands

## Overview

The `repository` commands manage repository phases and policies for release readiness. These are administrative tools that help maintain consistency in your project's lifecycle and release preparation states. They integrate with goneat's validation system to ensure your repository is in a healthy state for development, testing, or production releases.

The `repository` group falls under the **workflow** category, focusing on management tasks like setting phases and validating policies. Use these commands in CI/CD pipelines or pre-release checklists to enforce standards.

## Synopsis

```
goneat repository [command] [flags]
```

### Available Subcommands

- `phase show` - Display current repository phases
- `phase set` - Set RELEASE_PHASE and LIFECYCLE_PHASE
- `policy show` - Show phase policy rules
- `policy validate` - Validate policies against current state

## Commands

### phase show

**Description**: Loads and displays the current RELEASE_PHASE (e.g., dev, rc, release, hotfix) and LIFECYCLE_PHASE (e.g., alpha, beta, ga, maintenance) from configuration files or inferred from literal files like `RELEASE_PHASE` and `LIFECYCLE_PHASE`. This helps you verify the repository's current state.

**Synopsis**:

```
goneat repository phase show
```

**Examples**:

```
# Show current phases
goneat repository phase show

# Output example:
Current RELEASE_PHASE: rc (rules: suffixes=["-rc.1", "-rc.2"], min_coverage=75%, dirty_git=false)
Current LIFECYCLE_PHASE: beta (coverage min: 75%)
```

**Integration**:

- Use in scripts to check phase before running assess: `if [ "$(goneat repository phase show | grep RELEASE_PHASE)" != "rc" ]; then echo "Not ready for RC"; exit 1; fi`
- No flags; always reads from `.goneat/phases.yaml` or literal files.

### phase set

**Description**: Sets the RELEASE_PHASE and LIFECYCLE_PHASE, updating `.goneat/phases.yaml` or creating literal files (`RELEASE_PHASE`, `LIFECYCLE_PHASE`). Validates the phases against the schema and applies default rules (e.g., allowed suffixes). Use `--dry-run` to preview changes.

**Synopsis**:

```
goneat repository phase set --release [dev|rc|release|hotfix] --lifecycle [alpha|beta|ga|maintenance] [flags]
```

**Flags**:

- `--release string` (Required): The RELEASE_PHASE to set (dev, rc, release, hotfix).
- `--lifecycle string` (Required): The LIFECYCLE_PHASE to set (alpha, beta, ga, maintenance).
- `--dry-run` (bool): Preview changes without writing files.

**Examples**:

```
# Set to RC for release candidate preparation
goneat repository phase set --release rc --lifecycle beta

# Preview without changes
goneat repository phase set --release release --lifecycle ga --dry-run

# Output example:
Set RELEASE_PHASE=rc, LIFECYCLE_PHASE=beta
Updated .goneat/phases.yaml with rules (min_coverage=75%, suffixes=["-rc.1"])
```

**Integration**:

- In release scripts: `goneat repository phase set --release rc --lifecycle beta && goneat assess --categories maturity`
- Hooks: Not mutating in pre-commit; use in CI for phase transitions.

### policy show

**Description**: Displays the policy rules for all phases from the configuration, including allowed suffixes, minimum coverage thresholds, git cleanliness requirements, and documentation needs. Useful for understanding what each phase enforces.

**Synopsis**:

```
goneat repository policy show
```

**Examples**:

```
goneat repository policy show

# Output example:
dev: suffixes=["-dev", "-alpha"], min_coverage=50%, dirty_git=true, docs=["CHANGELOG.md"]
rc: suffixes=["-rc.1", "-rc.2"], min_coverage=75%, dirty_git=false, docs=["CHANGELOG.md", "RELEASE_NOTES.md"]
release: suffixes=[], min_coverage=90%, dirty_git=false
hotfix: suffixes=["-hotfix.1"], min_coverage=80%, dirty_git=false
alpha: min_coverage=50%
beta: min_coverage=75%
ga: min_coverage=90%
maintenance: min_coverage=80%, support=P1Y
```

**Integration**:

- Document team policies: Pipe to Markdown for README.
- No flags; reads from `.goneat/phases.yaml` (falls back to defaults).

### policy validate

**Description**: Validates the current policies against the repository state, checking for consistency (e.g., current phase matches version suffix). Use `--level` to control strictness.

**Synopsis**:

```
goneat repository policy validate [flags]
```

**Flags**:

- `--level string` (default "warn"): Error level (warn, error) for violations.

**Examples**:

```
# Validate with warnings
goneat repository policy validate --level warn

# Strict validation (fail on issues)
goneat repository policy validate --level error

# Output example:
Policy validation passed (level: warn)
- Warning: Version suffix doesn't match dev phase (fix: update VERSION)
```

**Integration**:

- CI gate: `goneat repository policy validate --level error || exit 1`
- With assess: `goneat assess --categories maturity` (includes policy checks).

## Configuration

Policies are defined in `.goneat/phases.yaml` (validated against embedded schema). See [Phases Configuration Schema](../standards/phases-schema.md) for details. Defaults ensure basic functionality without config.

## Best Practices

- Set phases early in the release cycle (e.g., dev for features, rc for testing).
- Use `policy show` to review rules before commits.
- Integrate `phase set` into release workflows; validate in pre-push hooks.
- For multi-language repos, use coverage_exceptions in phases.yaml for language-specific thresholds.

For more on integration with assess and hooks, see [Workflows](../workflows/release-readiness.md).
