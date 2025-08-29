---
title: "Assess Code Workflow Guide"
description: "Comprehensive workflow guide for using goneat assess in development, CI/CD, and team collaboration scenarios"
author: "@forge-neat"
date: "2025-08-28"
last_updated: "2025-08-28"
status: "approved"
tags: ["workflow", "assessment", "development", "ci-cd", "collaboration", "best-practices"]
category: "user-guide"
---

# Assess Code Workflow Guide

This comprehensive guide demonstrates practical workflows for using `goneat assess` across different development scenarios. The three-mode system (no-op, check, fix) enables flexible, safe, and efficient code assessment workflows.

## Overview

Goneat assess provides three operational modes designed for different use cases:

- **🔍 No-Op Mode** (`--no-op`): Safe validation without execution
- **📋 Check Mode** (`--check`): Issue detection and reporting
- **🔧 Fix Mode** (`--fix`): Automatic issue resolution where possible

## Quick Start Workflows

### New Project Setup

```bash
# Step 1: Validate environment setup
goneat assess --no-op --verbose

# Step 2: Get baseline assessment
goneat assess --check --format markdown --output baseline.md

# Step 3: Clean up auto-fixable issues
goneat assess --fix --categories format,lint

# Step 4: Review remaining issues
goneat assess --check --fail-on medium
```

### Daily Development Workflow

```bash
# Morning: Check current state
goneat assess --check --timeout 2m

# During development: Quick validation
goneat assess --no-op

# Before commit: Auto-fix and validate
goneat assess --fix --categories format
goneat assess --check --categories lint --fail-on high

# End of day: Comprehensive assessment
goneat assess --check --format json --output daily-report.json
```

## Development Scenarios

### Scenario 1: Code Cleanup Sprint

**Goal:** Dedicate time to improve code quality across the entire codebase.

```bash
# Phase 1: Assessment (1-2 days)
# Get comprehensive baseline
goneat assess --check --format both --output baseline/

# Analyze by priority
goneat assess --check --priority "format=1" --output format-issues.md
goneat assess --check --priority "lint=1" --output lint-issues.md

# Phase 2: Systematic Fixes (3-5 days)
# Auto-fix format issues
goneat assess --fix --categories format

# Review and fix critical lint issues
goneat assess --check --categories lint --fail-on critical --output critical-lint.md

# Phase 3: Validation (1 day)
# Final assessment
goneat assess --check --format json --output final-assessment.json

# Generate improvement report
./scripts/compare-assessments.sh baseline/assessment.json final-assessment.json
```

### Scenario 2: Feature Development

**Goal:** Maintain code quality during active feature development.

```bash
# Before starting feature
goneat assess --check --output pre-feature.md

# During development - frequent checks
goneat assess --no-op  # Quick validation
goneat assess --check --categories format  # Format-only check

# Before feature completion
goneat assess --fix --categories format  # Auto-fix formatting
goneat assess --check --fail-on high  # Ensure no high-severity issues

# Feature completion
goneat assess --check --format markdown --output feature-assessment.md
```

### Scenario 3: Code Review Preparation

**Goal:** Prepare code for peer review with consistent formatting and quality.

```bash
# Individual developer workflow
git checkout feature-branch

# Auto-fix formatting issues
goneat assess --fix --categories format

# Check for quality issues
goneat assess --check --categories lint --output review-issues.md

# Generate review report
goneat assess --check --format markdown --output code-review-report.md

# Commit cleaned code
git add .
git commit -m "Clean: Auto-fix formatting and address lint issues"

# Push for review
git push origin feature-branch
```

### Scenario 4: Legacy Code Migration

**Goal:** Gradually improve quality of existing legacy code.

```bash
# Initial assessment
goneat assess --check --format json --output legacy-baseline.json

# Phase 1: Safe improvements (auto-fixable)
goneat assess --fix --categories format --include "pkg/**/*.go"

# Phase 2: Gradual quality improvements
# Start with most critical files
goneat assess --check --fail-on critical --include "cmd/**/*.go"

# Phase 3: Expand coverage
goneat assess --check --fail-on high --include "internal/**/*.go"

# Track progress
goneat assess --check --format json --output current-state.json
./scripts/track-improvement.sh legacy-baseline.json current-state.json
```

## CI/CD Integration Workflows

### GitHub Actions Workflow

```yaml
name: Code Quality
on: [push, pull_request]

jobs:
  assess:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Install goneat
        run: go install ./...

      - name: Validate Setup
        run: goneat assess --no-op

      - name: Run Assessment
        run: goneat assess --check --format json --output assessment.json

      - name: Auto-fix (development branches only)
        if: github.ref != 'refs/heads/main'
        run: goneat assess --fix --categories format

      - name: Upload Results
        uses: actions/upload-artifact@v3
        with:
          name: assessment-results
          path: assessment.json

      - name: Quality Gate
        run: |
          if [ "$(jq '.summary.critical_issues' assessment.json)" -gt 0 ]; then
            echo "❌ Critical issues found - failing build"
            exit 1
          fi
```

### GitLab CI Pipeline

```yaml
stages:
  - validate
  - assess
  - deploy

validate:
  stage: validate
  script:
    - goneat assess --no-op --verbose
  only:
    - merge_requests

assess:
  stage: assess
  script:
    - goneat assess --check --format json --output assessment.json
    - goneat assess --fix --categories format
    - goneat assess --check --fail-on high
  artifacts:
    reports:
      junit: assessment.json
  only:
    - merge_requests
    - main

deploy:
  stage: deploy
  script:
    - echo "Deployment logic here"
  only:
    - main
```

## Team Collaboration Workflows

### Code Review Workflow

```bash
# Reviewer workflow
# Check out the branch
git checkout feature/awesome-feature

# Run comprehensive assessment
goneat assess --check --format markdown --output review-assessment.md

# Focus on critical issues
goneat assess --check --fail-on high --output critical-issues.md

# Generate review checklist
goneat assess --check --format json | jq '.workflow.parallel_groups[] | .description'
```

### Pair Programming Workflow

```bash
# Pair programming session
# Driver: Focus on development
# Navigator: Monitor code quality

# Quick checks during development
goneat assess --no-op  # Fast validation
goneat assess --check --categories format  # Format-only check

# Before switching roles
goneat assess --fix --categories format  # Auto-fix formatting
goneat assess --check --timeout 30s  # Quick quality check

# End of session
goneat assess --check --format markdown --output pair-session-report.md
```

### Team Standards Enforcement

```bash
# Team lead workflow
# Weekly quality assessment
goneat assess --check --format json --output weekly-assessment.json

# Department-wide standards
goneat assess --check --priority "security=1,lint=2" --output team-standards.md

# Generate team dashboard
./scripts/generate-team-dashboard.sh weekly-assessment.json

# Individual developer reports
for dev in alice bob charlie; do
  goneat assess --check --include "$dev/**/*.go" --output "$dev-report.md"
done
```

## Advanced Workflows

### Custom Assessment Pipelines

```bash
# Security audit pipeline
goneat assess --categories security --format json --output security-audit.json

# Performance analysis
goneat assess --categories performance --verbose --output performance-report.md

# Compliance checking
goneat assess --check --categories format,lint --fail-on medium --output compliance.md

# Documentation validation
goneat assess --categories docs --output docs-assessment.md
```

### Integration with External Tools

```bash
# Combined with golangci-lint (direct)
golangci-lint run --fix
goneat assess --check --categories format,lint

# Combined with SonarQube
goneat assess --check --format json --output goneat-results.json
sonar-scanner
./scripts/merge-reports.sh goneat-results.json sonar-results/

# Combined with custom tools
goneat assess --check --output goneat-report.md
./custom-security-scanner --input . --output security-report.md
./scripts/generate-combined-report.sh goneat-report.md security-report.md
```

### Automated Remediation Workflows

```bash
# Nightly cleanup job
#!/bin/bash
# Run every night at 2 AM

# Auto-fix formatting issues
goneat assess --fix --categories format --output nightly-format.log

# Auto-fix simple lint issues (if configured)
goneat assess --fix --categories lint --include "auto-fixable-patterns" --output nightly-lint.log

# Report remaining issues
goneat assess --check --format json --output nightly-assessment.json

# Send summary email
./scripts/send-nightly-summary.sh nightly-assessment.json
```

## Troubleshooting Workflows

### Common Issues Resolution

```bash
# Issue: Assessment is slow
goneat assess --check --timeout 5m --exclude "vendor/**" --verbose

# Issue: Too many false positives
goneat assess --check --categories format,lint --exclude "generated/**"

# Issue: Tools not found
goneat assess --no-op --verbose  # Check tool availability
./scripts/install-tools.sh      # Install missing tools

# Issue: Configuration problems
goneat assess --no-op --hook pre-commit  # Test hook configuration
cat .goneat/hooks.yaml                   # Validate configuration
```

### Debug and Investigation

```bash
# Debug specific categories
goneat assess --check --categories lint --verbose --timeout 10m

# Isolate file-specific issues
goneat assess --check --include "problematic-file.go" --verbose

# Test different priorities
goneat assess --check --priority "security=1" --output security-focus.md
goneat assess --check --priority "performance=1" --output perf-focus.md

# Generate detailed logs
goneat assess --check --verbose 2>&1 | tee assessment-debug.log
```

## Performance Optimization

### Large Codebase Strategies

```bash
# Parallel assessment for large repos
goneat assess --check --parallel --max-workers 4

# Incremental assessment
git diff --name-only HEAD~1 | xargs goneat assess --check

# Cached assessments
goneat assess --check --cache-dir .goneat/cache

# Batched processing
goneat assess --check --batch-size 50 --include "pkg/**/*.go"
```

### CI/CD Performance

```bash
# Fast feedback (early stages)
goneat assess --no-op --timeout 30s

# Quick checks (commit hooks)
goneat assess --check --categories format --timeout 2m

# Comprehensive (main branch)
goneat assess --check --timeout 10m --fail-on high
```

## Best Practices

### Development Best Practices

1. **Always start with no-op mode** for new setups
2. **Use check mode** for regular development validation
3. **Apply fix mode** strategically (not blindly)
4. **Set appropriate timeouts** for your environment
5. **Use categories** to focus on specific concerns

### Team Best Practices

1. **Establish team standards** for assessment usage
2. **Define quality gates** appropriate for your project
3. **Automate assessments** in CI/CD pipelines
4. **Monitor trends** in code quality over time
5. **Provide training** on assessment workflows

### CI/CD Best Practices

1. **Use no-op mode** for early validation (fast feedback)
2. **Fail fast** on critical issues
3. **Auto-fix** on development branches only
4. **Archive results** for trend analysis
5. **Integrate with** existing quality tools

## Metrics and Monitoring

### Quality Metrics Tracking

```bash
# Generate quality metrics
goneat assess --check --format json | jq '.summary' > quality-metrics.json

# Track improvement over time
./scripts/track-quality-trends.sh quality-metrics.json

# Generate quality dashboard
./scripts/generate-quality-dashboard.sh assessments/weekly/
```

### Team Performance Metrics

```bash
# Individual developer metrics
for dev in $(ls developers/); do
  goneat assess --check --include "authors/$dev/**/*.go" --output "metrics/$dev.json"
done

# Team-wide metrics
goneat assess --check --format json --output team-metrics.json

# Generate performance report
./scripts/generate-team-report.sh metrics/ team-metrics.json
```

## Conclusion

The three-mode assessment system enables flexible, safe, and efficient code quality workflows. By combining no-op validation, comprehensive checking, and intelligent fixing, goneat assess supports development teams throughout the entire software lifecycle.

Key takeaways:
- **Start safe** with no-op mode for validation
- **Check comprehensively** using check mode for regular assessment
- **Fix strategically** using fix mode for automated improvements
- **Integrate early** in CI/CD pipelines for continuous quality
- **Monitor trends** to track improvement over time

This workflow guide provides the foundation for implementing goneat assess effectively across different development scenarios and team sizes.
