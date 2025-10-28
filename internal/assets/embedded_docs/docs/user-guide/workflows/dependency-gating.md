---
title: "Dependency Gating Workflow Guide"
description: "Workflow strategies for dependency license and cooling policy validation in pre-commit, pre-push, and CI/CD environments"
author: "@code-scout"
date: "2025-10-17"
last_updated: "2025-10-22"
status: "approved"
tags:
  [
    "workflow",
    "dependencies",
    "security",
    "supply-chain",
    "hooks",
    "ci-cd",
    "licensing",
    "cooling-policy",
  ]
category: "user-guide"
---

# Dependency Gating Workflow Guide

This guide demonstrates practical strategies for integrating dependency license and cooling policy validation into development workflows using `goneat assess --categories dependencies`.

## Overview

The dependencies assessment category validates:

- **License Compliance**: Enforces allowed/forbidden license policies
- **Cooling Policy**: Ensures dependencies meet minimum maturity requirements
- **SBOM Metadata**: Tracks software bill of materials for supply-chain transparency

### Network Considerations

Dependency analysis has different network requirements depending on the validation type:

- **License validation**: Offline (uses local go.mod/package manifests)
- **Cooling policy**: Online (queries package registries for publish dates)
- **SBOM generation**: Offline (uses local dependency graphs)

This guide provides strategies for both **offline** (pre-commit) and **online** (pre-push/CI) workflows.

**SBOM Output Directory:** By default, SBOMs are generated in the `sbom/` directory at the project root. Make sure to add `sbom/` to your `.gitignore` to prevent committing generated artifacts.

## Quick Start

### Try It Yourself (goneat Repository)

If you've cloned the goneat repository, try these commands to see dependencies assessment in action:

```bash
# Clone goneat (if you haven't already)
git clone https://github.com/fulmenhq/goneat.git
cd goneat

# 1. Generate an SBOM for the goneat project
goneat dependencies sbom
# Output: sbom/goneat-<timestamp>.cdx.json

# 2. Run dependencies assessment (includes license + cooling checks)
goneat assess --categories dependencies --verbose

# 3. View SBOM metadata in assessment output
goneat assess --categories dependencies --format json | jq '.categories.dependencies.metrics.sbom_metadata'

# 4. Examine the generated SBOM file
cat sbom/goneat-latest.cdx.json | jq '.metadata, .components | length'
```

**What you'll see:**

- ~150+ Go dependencies analyzed
- License compliance checked (MIT, Apache-2.0, BSD licenses are allowed)
- Cooling policy validated (packages must be >7 days old)
- SBOM metadata included in assessment report

### Basic Dependency Assessment

```bash
# Run dependencies assessment on current project
goneat assess --categories dependencies

# Run with verbose output to see policy evaluation details
goneat assess --categories dependencies --verbose

# Fail on high-severity issues (critical/high)
goneat assess --categories dependencies --fail-on high

# Generate JSON report for automation
goneat assess --categories dependencies --format json --output deps-report.json
```

### Check Assessment Output

The dependencies assessment provides structured output:

```markdown
### ✅ Dependencies Issues (Priority: 2)

**Status:** 0 issues found
**Estimated Time:** 0 seconds
**Parallelizable:** No

**Metrics:**

- Dependency count: 142
- License violations: 0
- Cooling violations: 0
- Analysis passed: true
- SBOM metadata: available (sbom/goneat-latest.cdx.json)
```

## Hook-Based Workflows

### Strategy 1: Pre-Push Only (Recommended)

**Best for:** Most teams balancing speed and supply-chain hygiene

This strategy runs dependency checks during `pre-push` when network is available:

```yaml
# .goneat/hooks.yaml
version: v1
hooks:
  pre-commit:
    - command: assess
      args:
        - --categories
        - format,lint,dates,tools
        - --fail-on
        - high
      fallback: warn

  pre-push:
    - command: assess
      args:
        - --categories
        - format,lint,security,dependencies,dates,tools,maturity,repo-status
        - --fail-on
        - high
      fallback: fail

optimization:
  only_changed_files: true
  cache_results: true
  parallel: auto
```

**Benefits:**

- ✅ Fast local commits (no network delay)
- ✅ Comprehensive pre-push gating
- ✅ Cooling policy validated before push
- ✅ Catches issues before CI

**Install hooks:**

```bash
goneat hooks install
```

### Strategy 2: License-Only Pre-Commit + Full Pre-Push

**Best for:** Teams requiring license compliance on every commit

This strategy validates licenses offline during pre-commit, adds cooling policy during pre-push:

```yaml
# .goneat/hooks.yaml
version: v1
hooks:
  pre-commit:
    - command: dependencies
      args:
        - check
        - --licenses # Offline license validation
        - --fail-on
        - high
      fallback: fail

  pre-push:
    - command: assess
      args:
        - --categories
        - dependencies # Full validation including cooling
        - --fail-on
        - high
      fallback: fail
```

**Benefits:**

- ✅ Early license violation detection
- ✅ No network requirement for commits
- ✅ Full policy enforcement before push

### Strategy 3: CI-Only (No Local Hooks)

**Best for:** Teams preferring centralized validation

Skip local hooks and validate in CI pipelines:

```yaml
# .github/workflows/validate.yml
name: Dependency Validation

on: [pull_request, push]

jobs:
  dependencies:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Install goneat
        run: |
          curl -sSL https://github.com/fulmenhq/goneat/releases/latest/download/install.sh | bash
          echo "$HOME/.local/bin" >> $GITHUB_PATH

      - name: Run dependency assessment
        run: |
          goneat assess --categories dependencies --fail-on high --format json --output deps-report.json

      - name: Upload assessment report
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: dependency-assessment
          path: deps-report.json
```

## CI/CD Integration Patterns

### GitHub Actions (Full Assessment)

```yaml
# .github/workflows/assess.yml
name: Code Assessment

on:
  pull_request:
    branches: [main]
  push:
    branches: [main]

jobs:
  assess:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: "1.23"

      - name: Install goneat
        run: |
          curl -sSL https://github.com/fulmenhq/goneat/releases/latest/download/install.sh | bash
          echo "$HOME/.local/bin" >> $GITHUB_PATH

      - name: Run assessment with dependencies
        run: |
          goneat assess \
            --categories format,lint,security,dependencies \
            --fail-on high \
            --format json \
            --output assessment.json

      - name: Print CI summary
        run: |
          goneat assess \
            --categories dependencies \
            --ci-summary

      - name: Upload assessment artifacts
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: assessment-reports
          path: |
            assessment.json
            sbom/
```

### GitLab CI

```yaml
# .gitlab-ci.yml
stages:
  - validate

dependency-check:
  stage: validate
  image: golang:1.23
  script:
    - curl -sSL https://github.com/fulmenhq/goneat/releases/latest/download/install.sh | bash
    - export PATH="$HOME/.local/bin:$PATH"
    - goneat assess --categories dependencies --fail-on high --ci-summary
  artifacts:
    reports:
      junit: assessment.json
    paths:
      - assessment.json
      - sbom/
    when: always
```

### Jenkins Pipeline

```groovy
// Jenkinsfile
pipeline {
    agent any

    stages {
        stage('Dependency Assessment') {
            steps {
                sh '''
                    curl -sSL https://github.com/fulmenhq/goneat/releases/latest/download/install.sh | bash
                    export PATH="$HOME/.local/bin:$PATH"
                    goneat assess --categories dependencies --fail-on high --format json --output deps.json
                '''
            }
        }
    }

    post {
        always {
            archiveArtifacts artifacts: 'deps.json,sbom/**', allowEmptyArchive: true
        }
    }
}
```

## Understanding SBOM Output

### What is an SBOM?

A Software Bill of Materials (SBOM) is a comprehensive inventory of all components, libraries, and dependencies in your software. Goneat generates SBOMs in CycloneDX JSON format, an industry-standard format for supply-chain security.

### SBOM Structure Example

Here's what a goneat-generated SBOM looks like (truncated for readability):

```json
{
  "bomFormat": "CycloneDX",
  "specVersion": "1.4",
  "version": 1,
  "metadata": {
    "timestamp": "2025-10-17T14:30:00Z",
    "tools": [
      {
        "vendor": "anchore",
        "name": "syft",
        "version": "0.100.0"
      }
    ],
    "component": {
      "type": "application",
      "name": "goneat",
      "version": "v0.3.0-dev"
    }
  },
  "components": [
    {
      "type": "library",
      "name": "github.com/spf13/cobra",
      "version": "v1.8.0",
      "purl": "pkg:golang/github.com/spf13/cobra@v1.8.0",
      "licenses": [
        {
          "license": {
            "id": "Apache-2.0"
          }
        }
      ]
    },
    {
      "type": "library",
      "name": "gopkg.in/yaml.v3",
      "version": "v3.0.1",
      "purl": "pkg:golang/gopkg.in/yaml.v3@v3.0.1",
      "licenses": [
        {
          "license": {
            "id": "MIT"
          }
        }
      ]
    }
    // ... 150+ more components
  ]
}
```

### SBOM Use Cases

1. **Supply-Chain Security**: Track all dependencies for vulnerability management
2. **License Compliance**: Audit licenses across your entire dependency tree
3. **Regulatory Compliance**: Meet SBOM requirements (e.g., Executive Order 14028)
4. **Dependency Analysis**: Understand your software composition
5. **CI/CD Integration**: Automate dependency tracking in pipelines

### SBOM + Assessment Integration

When you run `goneat assess --categories dependencies`, the assessment automatically:

- Looks for existing SBOM files (in `sbom/goneat-latest.cdx.json`)
- Includes SBOM metadata in the assessment report
- Reports if SBOM is missing or needs regeneration

**Example assessment output with SBOM:**

```json
{
  "categories": {
    "dependencies": {
      "metrics": {
        "dependency_count": 142,
        "license_violations": 0,
        "cooling_violations": 0,
        "analysis_passed": true,
        "sbom_metadata": {
          "path": "sbom/goneat-latest.cdx.json",
          "status": "available",
          "tool_version": "syft-0.100.0",
          "generated_at": "2025-10-17T14:30:00Z",
          "component_count": 142
        }
      }
    }
  }
}
```

## Configuration

### Dependencies Policy Configuration

Create `.goneat/dependencies.yaml` to customize policy:

```yaml
# License policy
licenses:
  allowed:
    - MIT
    - Apache-2.0
    - BSD-3-Clause
    - BSD-2-Clause
    - ISC
  forbidden:
    - GPL-3.0
    - AGPL-3.0
  warn:
    - LGPL-2.1
    - LGPL-3.0

# Cooling policy (prevent newly published packages)
cooling:
  enabled: true
  min_age_days: 14 # Package must be at least 2 weeks old
  exceptions:
    - github.com/your-org/* # Trust your organization

# SBOM settings
sbom:
  output_dir: sbom
  format: cyclonedx-json
  include_dev_dependencies: false
```

### Hook Configuration with Dependencies

Example `.goneat/hooks.yaml` with dependency validation:

```yaml
version: v1

hooks:
  pre-commit:
    - command: assess
      args:
        - --categories
        - format,lint,dates,tools
        - --fail-on
        - high
      fallback: warn

  pre-push:
    - command: assess
      args:
        - --categories
        - format,lint,security,dependencies,dates,tools,maturity,repo-status
        - --fail-on
        - high
      fallback: fail

optimization:
  only_changed_files: true
  cache_results: true
  parallel: auto

policies:
  dependencies:
    fail_on: high
    check_licenses: true
    check_cooling: true
```

## Caching Strategies

### Local Development Caching

Goneat caches registry queries to improve performance:

```bash
# Cache location
~/.goneat/cache/registry/

# Clear cache if needed
rm -rf ~/.goneat/cache/registry/
```

### CI Caching (GitHub Actions)

```yaml
- name: Cache goneat dependencies
  uses: actions/cache@v4
  with:
    path: |
      ~/.goneat/cache/registry
      sbom/
    key: goneat-deps-${{ runner.os }}-${{ hashFiles('go.sum') }}
    restore-keys: |
      goneat-deps-${{ runner.os }}-
```

### CI Caching (GitLab)

```yaml
dependency-check:
  cache:
    key: ${CI_COMMIT_REF_SLUG}
    paths:
      - .goneat/cache/
      - sbom/
```

## Troubleshooting

### Issue: Cooling Policy Fails Offline

**Problem:** Pre-commit hook fails with cooling policy errors when offline.

**Solution:** Move cooling checks to pre-push or CI:

```yaml
# .goneat/hooks.yaml - Disable cooling in pre-commit
hooks:
  pre-commit:
    - command: dependencies
      args:
        - check
        - --licenses # Skip cooling (network-dependent)
        - --fail-on
        - high
```

### Issue: Slow Dependency Assessment

**Problem:** `goneat assess --categories dependencies` takes too long.

**Solutions:**

1. **Enable caching:**

   ```bash
   # Caching is enabled by default
   # Check cache location:
   ls ~/.goneat/cache/registry/
   ```

2. **Skip unchanged dependencies:**

   ```bash
   # Use assessment caching in hooks.yaml
   optimization:
     cache_results: true
   ```

3. **Run in parallel with other categories:**
   ```bash
   # Dependencies run concurrently with other assessments
   goneat assess --categories format,lint,dependencies --concurrency 4
   ```

### Issue: False Positive License Violations

**Problem:** License detected incorrectly or dependency has dual license.

**Solution:** Add exception to `.goneat/dependencies.yaml`:

```yaml
licenses:
  exceptions:
    - package: github.com/example/pkg
      reason: "Dual licensed MIT/Apache-2.0, using MIT"
      approved_by: "@tech-lead"
      approved_date: "2025-10-17"
```

### Issue: SBOM Not Found

**Problem:** Assessment reports "SBOM metadata: not_generated".

**Solution:** Generate SBOM before assessment:

```bash
# Generate SBOM
goneat dependencies sbom

# Then run assessment (will find existing SBOM)
goneat assess --categories dependencies
```

## Best Practices

### 1. Layer Your Validation

- **Pre-commit**: Fast, offline checks (format, lint)
- **Pre-push**: Comprehensive checks including dependencies
- **CI**: Full validation + reporting + artifacts

### 2. Configure Fail Thresholds Appropriately

```bash
# Development: Warn on issues
goneat assess --categories dependencies --fail-on critical

# CI: Strict enforcement
goneat assess --categories dependencies --fail-on high
```

### 3. Document Exceptions

Always document license and cooling policy exceptions:

```yaml
# .goneat/dependencies.yaml
licenses:
  exceptions:
    - package: github.com/special/pkg
      reason: "Vendor approved license after legal review"
      approved_by: "@legal-team"
      approved_date: "2025-10-15"
      ticket: "SEC-1234"
```

### 4. Generate SBOMs Regularly

```bash
# Add to your release process
goneat dependencies sbom --output sbom/release-v1.0.0.cdx.json

# Archive SBOMs with releases
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0
```

### 5. Monitor Dependency Trends

```bash
# Track dependency counts over time
goneat assess --categories dependencies --format json | \
  jq '.categories.dependencies.metrics.dependency_count'

# Detect new violations
goneat assess --categories dependencies --ci-summary
```

## Advanced Patterns

### Monorepo: Per-Package Validation

```bash
# Validate specific package
cd packages/api
goneat assess --categories dependencies --fail-on high

# Validate all packages
for pkg in packages/*; do
  echo "Validating $pkg..."
  (cd "$pkg" && goneat assess --categories dependencies)
done
```

### Custom Policy Enforcement

```bash
# Fail if any LGPL licenses detected
if goneat assess --categories dependencies --format json | \
   jq -e '.categories.dependencies.issues[] | select(.sub_category == "license") | select(.message | contains("LGPL"))'; then
  echo "❌ LGPL license detected - requires review"
  exit 1
fi
```

### Integration with OPA

```bash
# Generate assessment data for OPA policy evaluation
goneat assess --categories dependencies --format json > deps.json

# Evaluate with OPA
opa eval --data policy.rego --input deps.json "data.dependencies.allow"
```

## Related Documentation

- [Assess Command Reference](../commands/assess.md)
- [Dependencies Command Reference](../commands/dependencies.md)
- [Hooks Architecture](../../architecture/hooks-command-architecture.md)
- [License Compliance SOP](../../sop/license-compliance-sop.md)

## Next Steps

1. **Install hooks**: `goneat hooks install`
2. **Configure policy**: Edit `.goneat/dependencies.yaml`
3. **Test workflow**: `goneat assess --categories dependencies`
4. **Integrate CI**: Add to your CI pipeline
5. **Monitor compliance**: Review reports regularly

---

**Generated by Code Scout via Claude Code**
