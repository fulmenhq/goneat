# License Policy Enforcement with Git Hooks

This appnote demonstrates how to add license compliance checking to your git hooks using goneat's hooks system, ensuring that commits and pushes are automatically validated against your license policy.

## Overview

Goneat provides a comprehensive license policy system that can be integrated into git hooks to prevent commits and pushes that introduce forbidden licenses. This ensures license compliance is enforced at the earliest possible point in your development workflow.

## Prerequisites

- goneat v0.3.0 or later with dependency analysis features
- A configured `.goneat/dependencies.yaml` policy file
- Git hooks enabled in your repository

## Quick Setup

### 1. Configure License Policy

First, ensure your `.goneat/dependencies.yaml` contains license policy rules:

```yaml
version: v1

licenses:
  forbidden:
    - GPL-3.0 # Strong copyleft
    - AGPL-3.0 # Network copyleft
  # Optional: explicit allowlist
  # allowed:
  #   - MIT
  #   - Apache-2.0
  #   - BSD-3-Clause
```

### 2. Add License Checks to Hooks

Use the `goneat hooks` command to add license policy enforcement:

```bash
# Add license checking to pre-commit hook
goneat hooks add pre-commit dependencies --licenses --fail-on high

# Add comprehensive license + cooling checks to pre-push hook
goneat hooks add pre-push dependencies --licenses --cooling --fail-on high
```

### 3. Install Updated Hooks

```bash
# Regenerate and install the hooks
goneat hooks generate
goneat hooks install
```

## Detailed Configuration

### Hook Manifest Structure

The hooks are configured in `.goneat/hooks.yaml`. Here's how license checks are added:

```yaml
version: "1.0.0"
hooks:
  pre-commit:
    - command: "dependencies"
      args: ["--licenses", "--fail-on", "high"]
      priority: 8
      timeout: "30s"
    - command: "assess"
      args: ["--categories", "format,lint,dates,tools", "--fail-on", "high"]
      priority: 10
      timeout: "90s"
  pre-push:
    - command: "dependencies"
      args: ["--licenses", "--cooling", "--fail-on", "high"]
      priority: 7
      timeout: "45s"
    - command: "assess"
      args:
        [
          "--categories",
          "format,lint,security,dependencies,dates,tools,maturity,repo-status",
          "--fail-on",
          "high",
        ]
      priority: 10
      timeout: "2m"
```

### Command Options

The `dependencies` command supports these options for hooks:

- `--licenses`: Run license compliance checks
- `--cooling`: Check cooling policy (requires network access)
- `--fail-on <level>`: Fail on issues at or above this severity (critical, high, medium, low)
- `--policy <path>`: Path to policy file (default: `.goneat/dependencies.yaml`)

### Priority and Timing

- **Priority**: Lower numbers run first (5 = highest priority)
- **Timeout**: Maximum execution time before hook fails
- **Pre-commit**: Fast checks only (no network required)
- **Pre-push**: Comprehensive checks (may require network for cooling policy)

## Testing Hook Enforcement

### Manual Testing

Test license enforcement manually:

```bash
# Test license compliance check
goneat dependencies --licenses --fail-on high

# Test with cooling policy (requires network)
goneat dependencies --licenses --cooling --fail-on high

# Test hook execution
goneat assess --hook pre-commit
```

### Integration Testing

Test that hooks actually block violations:

```bash
# Add a forbidden license to go.mod (for testing)
echo 'require github.com/forbidden/package v1.0.0' >> go.mod

# Try to commit - should fail
git add go.mod
git commit -m "test forbidden license"

# Clean up
git reset HEAD go.mod
```

## Troubleshooting

### Common Issues

**Hook fails with "goneat not found"**

```bash
# Ensure goneat is in PATH or use full path
export PATH="$HOME/go/bin:$PATH"
goneat hooks install
```

**License check fails unexpectedly**

```bash
# Check your policy file
goneat dependencies --licenses --format json | jq .

# Validate policy syntax
goneat validate .goneat/dependencies.yaml
```

**Network access required for cooling policy**

```bash
# Pre-push hooks may fail in CI without network
# Consider separate CI-only license checks
```

### Hook Debugging

Enable verbose logging for hook debugging:

```bash
# Test with verbose output
GONEAT_LOG_LEVEL=debug goneat assess --hook pre-commit

# Check hook execution in isolation
./.git/hooks/pre-commit
```

## Advanced Configuration

### Custom Policy Files

Use different policies for different contexts:

```bash
# Development policy (more permissive)
goneat hooks add pre-commit dependencies --licenses --policy .goneat/dev-dependencies.yaml

# Production policy (strict)
goneat hooks add pre-push dependencies --licenses --policy .goneat/prod-dependencies.yaml
```

### Conditional Execution

Hooks can be conditional based on environment:

```yaml
hooks:
  pre-push:
    - command: "dependencies"
      args: ["--licenses", "--cooling"]
      priority: 7
      timeout: "45s"
      # Only run in CI or when network available
      condition: "CI=true || NETWORK_AVAILABLE=true"
```

### Integration with CI/CD

For CI/CD pipelines, consider:

```yaml
# CI workflow (GitHub Actions example)
- name: License Compliance Check
  run: |
    goneat dependencies --licenses --cooling --fail-on high

# SBOM Generation
- name: Generate SBOM
  run: |
    goneat dependencies --sbom --output sbom/
```

## Policy Examples

### Permissive Open Source Policy

```yaml
version: v1
licenses:
  forbidden:
    - GPL-3.0
    - AGPL-3.0
    - LGPL-3.0
  allowed:
    - MIT
    - Apache-2.0
    - BSD-3-Clause
    - ISC
```

### Enterprise Policy

```yaml
version: v1
licenses:
  forbidden:
    - GPL-3.0
    - AGPL-3.0
    - LGPL-3.0
    - MS-PL # Microsoft Permissive License
  allowed:
    - MIT
    - Apache-2.0
    - BSD-3-Clause
    - ISC
    - EPL-2.0 # Eclipse Public License 2.0
```

## Performance Considerations

- **Pre-commit**: Keep fast (< 30s) - license checks only
- **Pre-push**: Can be slower (< 2m) - include cooling policy
- **Caching**: goneat automatically caches results where possible
- **Parallelization**: Multiple checks run concurrently when possible

## Integration with Other Tools

### With Pre-commit Framework

If using pre-commit framework alongside goneat:

```yaml
# .pre-commit-config.yaml
repos:
  - repo: local
    hooks:
      - id: goneat-license-check
        name: goneat license compliance
        entry: goneat dependencies --licenses --fail-on high
        language: system
        pass_filenames: false
```

### With GitHub Actions

```yaml
# .github/workflows/license-check.yml
name: License Compliance
on: [push, pull_request]

jobs:
  license-check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v4
        with:
          go-version: "1.21"
      - run: go install github.com/fulmenhq/goneat@latest
      - run: goneat dependencies --licenses --cooling --fail-on high
```

## See Also

- [Dependencies Command](../../user-guide/commands/dependencies.md) - CLI usage reference
- [Dependency Policy Configuration](../../configuration/dependency-policy.md) - Policy syntax guide
- [Git Hooks Guide](../../user-guide/workflows/git-hooks.md) - General hooks configuration
- [License Compliance Workflow](../../user-guide/workflows/dependency-protection.md) - Complete workflow guide

## References

- goneat Dependencies Package: `pkg/dependencies/`
- Hook System: `pkg/tools/hooks/`
- Policy Engine: `pkg/dependencies/policy/`

---

**Last Updated**: October 23, 2025
**Status**: Active
