---
title: "Assess Command Reference"
description: "Complete reference for the goneat assess command - comprehensive codebase assessment and workflow planning"
author: "@forge-neat"
date: "2025-08-28"
last_updated: "2025-10-22"
status: "approved"
tags:
  ["cli", "assessment", "validation", "reporting", "commands", "dependencies"]
category: "user-guide"
---

# Assess Command Reference

The `goneat assess` command provides comprehensive codebase assessment with intelligent validation, workflow planning, and unified reporting across all supported tools and categories.

## Overview

Goneat assess is the core intelligence engine that:

- **Orchestrates multiple validation tools** (format, lint, security, performance)
- **Applies intelligent prioritization** based on issue severity and impact
- **Enables parallel execution** for faster validation cycles
- **Generates unified reports** with actionable remediation workflows
- **Supports multiple output formats** (JSON-first, HTML/Markdown from JSON)
- **Integrates with git hooks** for automated validation

## Concurrency

- Runtime concurrency across categories uses a worker-pool.
- Defaults to 50% of CPU cores (min 1). Override via flags:
  - `--concurrency <int>`
  - `--concurrency-percent <int>` (1-100)
- Category failures do not stop other categories; final exit still honors `--fail-on`.
- Logs include: workers used, per-category runtimes, totals.

Example log summary:

```
workers=6, categories=3
Runtime: format           115ms
Runtime: static-analysis  812ms
Runtime: lint             1.067s
Total: 1.067s; issues: 4
```

## Command Structure

```bash
goneat assess [target] [flags]
```

## Core Use Cases

### Manual Code Assessment

Run comprehensive validation on your codebase:

```bash
# Assess current directory
goneat assess

# Assess specific directory
goneat assess /path/to/project

# Assess with custom priorities
goneat assess --priority "security=1,format=2,lint=3"
```

### Git Hook Integration

Used automatically by git hooks for pre-commit and pre-push validation:

```bash
# Pre-commit validation (runs automatically)
git commit -m "Add feature"
# → Triggers: goneat assess --hook pre-commit

# Pre-push validation (runs automatically)
git push origin main
# → Triggers: goneat assess --hook pre-push

# Manual hook testing
goneat assess --hook pre-commit
```

### CI/CD Integration

Generate reports for automated pipelines:

```bash
# JSON output for CI/CD systems
goneat assess --format json --output assessment.json

# Fail on specific severity levels
goneat assess --fail-on high

# Include/exclude specific files
goneat assess --include "*.go" --exclude "vendor/**"
```

## Command Flags

### Core Assessment Flags

| Flag         | Type     | Description                                | Example                            |
| ------------ | -------- | ------------------------------------------ | ---------------------------------- |
| `--format`   | string   | Output format (markdown, json, html, both) | `--format json`                    |
| `--mode`     | string   | Operation mode (check, fix, no-op)         | `--mode fix`                       |
| `--no-op`    | boolean  | Assessment mode only (no changes)          | `--no-op`                          |
| `--check`    | boolean  | Check mode (report issues, no changes)     | `--check`                          |
| `--fix`      | boolean  | Fix mode (apply fixes automatically)       | `--fix`                            |
| `--priority` | string   | Custom priority string                     | `--priority "security=1,format=2"` |
| `--fail-on`  | string   | Fail on severity level                     | `--fail-on high`                   |
| `--timeout`  | duration | Assessment timeout                         | `--timeout 5m`                     |
| `--output`   | string   | Output file path                           | `--output report.md`               |

### Concurrency Flags

| Flag                    | Type | Description                  | Example                    |
| ----------------------- | ---- | ---------------------------- | -------------------------- |
| `--concurrency`         | int  | Explicit worker count        | `--concurrency 4`          |
| `--concurrency-percent` | int  | Percent of CPU cores (1-100) | `--concurrency-percent 75` |

### Filtering Flags

| Flag                   | Type    | Description                              | Example                               |
| ---------------------- | ------- | ---------------------------------------- | ------------------------------------- |
| `--include`            | strings | Include file patterns                    | `--include "*.go"`                    |
| `--exclude`            | strings | Exclude file patterns                    | `--exclude "vendor/**"`               |
| `--categories`         | string  | Specific categories to assess            | `--categories "format,lint"`          |
| `--no-ignore`          | boolean | Disable ignore files                     | `--no-ignore`                         |
| `--force-include`      | strings | Force-include ignored paths (repeatable) | `--force-include 'tests/fixtures/**'` |
| `--scope`              | boolean | Limit traversal to include/force anchors | `--scope`                             |
| `--schema-enable-meta` | boolean | Attempt schema meta validation           | `--schema-enable-meta`                |

### Hook Integration Flags

| Flag              | Type   | Description        | Example                              |
| ----------------- | ------ | ------------------ | ------------------------------------ |
| `--hook`          | string | Run in hook mode   | `--hook pre-commit`                  |
| `--hook-manifest` | string | Hook manifest path | `--hook-manifest .goneat/hooks.yaml` |

### Display Flags

| Flag           | Type    | Description                             | Example        |
| -------------- | ------- | --------------------------------------- | -------------- |
| `--verbose`    | boolean | Verbose output                          | `--verbose`    |
| `--quiet`      | boolean | Minimal output                          | `--quiet`      |
| `--ci-summary` | boolean | One-line CI status (PASS/FAIL + counts) | `--ci-summary` |

### Security Flags

| Flag                   | Type    | Description                                          | Example                |
| ---------------------- | ------- | ---------------------------------------------------- | ---------------------- |
| `--track-suppressions` | boolean | Track and report suppressions (e.g., gosec `#nosec`) | `--track-suppressions` |

### Profiles

Use preset profiles to apply sensible defaults without overriding explicitly set flags:

| Profile | Defaults                                                       |
| ------- | -------------------------------------------------------------- |
| `ci`    | `--categories format,lint,security` and `--fail-on critical`   |
| `dev`   | `--categories format,lint,security,schema` and `--fail-on low` |

Example:

```bash
goneat assess --profile ci --format concise --ci-summary
```

## Assessment Categories

Goneat assess supports multiple validation categories:

### Format (`format`)

- **Purpose:** Code formatting and style consistency
- **Tools:** gofmt, goimports
- **Typical Issues:** Indentation, import organization, whitespace
- **Auto-fixable:** Yes (most issues)

### Lint (`lint`)

- **Purpose:** Code quality and best practices (Go, shell, Make, GitHub Actions)
- **Tools:** golangci-lint, govet, shfmt (check/fix), shellcheck (verify-only, opt-in GPL), actionlint, checkmake
- **Typical Issues:** Unused variables, style violations, shell hygiene, workflow bugs, Makefile hygiene
- **Auto-fixable:** Partial (golangci limited; shfmt fix optional; others verify-only)

#### Lint extensions (shell / Make / GitHub Actions)
- Defaults: shfmt/actionlint/checkmake enabled; shellcheck disabled unless explicitly enabled and available.
- Config: see `docs/assess/lint-shell-make-gha.md` for `.goneat/assess.yaml` examples (paths/excludes, shellcheck sidecar) and CI notes.
- CLI toggles: `--lint-shell`, `--lint-shell-fix`, `--lint-shellcheck`, `--shellcheck-path`, `--lint-gha`, `--lint-make`, plus include/exclude overrides (`--lint-shell-paths`, `--lint-shell-exclude`, `--lint-gha-paths`, `--lint-gha-exclude`, `--lint-make-paths`, `--lint-make-exclude`).

### Security (`security`)

- **Purpose:** Security vulnerability detection
- **Tools:** gosec, custom security scanners
- **Typical Issues:** SQL injection, hardcoded secrets, unsafe operations
- **Auto-fixable:** No (requires manual review)

### Dependencies (`dependencies`)

- **Purpose:** Supply-chain security and license compliance
- **Tools:** License analyzer, cooling policy validator, SBOM generator
- **Typical Issues:** Forbidden licenses, newly published packages (cooling violations), missing SBOM
- **Auto-fixable:** No (requires policy review and dependency updates)
- **Network:** May require network access for cooling policy (package registry queries)
- **Priority:** 2 (aligned with security)

Run dependencies assessment:

```bash
# Full dependencies assessment
goneat assess --categories dependencies

# Dependencies with other security checks
goneat assess --categories security,dependencies --fail-on high

# Offline mode (license-only, no cooling checks)
goneat dependencies --licenses
```

See [Dependency Gating Workflow](../workflows/dependency-gating.md) for complete integration patterns.

**Assessment Output:** Dependency assessment emits structured JSON that conforms to
`schemas/dependencies/v1.0.0/dependency-analysis.schema.json`. Metrics include package
counts, cooling violations, license policy results, and SBOM metadata (path, tool
version, generation timestamp) when a recent SBOM is present. Use these fields to
power dashboards or CI quality gates.

### Static Analysis (`static-analysis`)

- **Purpose:** Advanced code analysis
- **Tools:** staticcheck, ineffassign
- **Typical Issues:** Dead code, inefficient assignments, type issues
- **Auto-fixable:** Limited

### Performance (`performance`)

- **Purpose:** Performance optimization opportunities
- **Tools:** Custom performance analyzers
- **Typical Issues:** Memory leaks, inefficient algorithms
- **Auto-fixable:** No (requires architectural changes)

### Schema (`schema`) [Preview]

- **Purpose:** Schema-aware validation (syntax + meta-schema checks)
- **Tools:** Embedded JSON Schema meta-schemas (Draft-07, 2020-12)
- **Typical Issues:** YAML/JSON syntax errors, schema structure violations
- **Auto-fixable:** No (preview)

Run only schema validation:

```bash
goneat assess --categories schema --format json --output schemas-report.json
```

Alternatively use the convenience command:

```bash
goneat validate --include schemas/ --format json --output validate.json
```

#### Schema Assessment in Git Hooks

**Use Case 1: Meta-Schema Validation (Schema Files Themselves)**
Validate that your schema files are well-formed JSON Schema documents:

```bash
# Hook for validating schema files against meta-schemas
goneat assess --categories schema --schema-enable-meta \
  --schema-discovery-mode all \
  --hook pre-commit

# Target specific schema directories
goneat assess schemas/ --categories schema --schema-enable-meta
```

**Use Case 2: Data Validation (Config Files Against Schemas)**
Validate that configuration files match their schemas:

```bash
# Validate config files against schemas (using goneat validate command)
goneat validate --schema-file schemas/config.yaml config/app.yaml

# Or use pathfinder + schema workflow for batch validation
goneat pathfinder find --schemas --schema-id json-schema-2020-12 --output text \
  | cut -d ' ' -f1 \
  | xargs -r goneat schema validate-schema --schema-id json-schema-2020-12
```

#### Enhanced Schema Discovery

With v0.2.10 enhancements, schema assessment now supports:

```bash
# Enhanced discovery - find any file with $schema field
goneat assess --categories schema --schema-discovery-mode all

# Filter by specific drafts
goneat assess --categories schema --schema-drafts "draft-07,2020-12"

# Custom patterns for schema files
goneat assess --categories schema --schema-patterns "*.schema.yaml"
```

#### Intelligent Config Mapping (Preview)

Enable automatic config-to-schema mapping for YAML/JSON configuration files:

```bash
goneat assess config/ \
  --categories schema \
  --schema-mapping \
  --schema-mapping-strict \
  --schema-mapping-min-confidence 0.8
```

- `--schema-mapping`: turn on intelligent mapping using built-in heuristics and `<repo>/.goneat/schema-mappings.yaml`.
- `--schema-mapping-manifest <path>`: override the manifest location when working in mono-repos.
- `--schema-mapping-min-confidence <0-1>`: require a minimum confidence score before validating a mapping.
- `--schema-mapping-strict`: surface issues when mappings are missing or fall below the configured confidence threshold.

When mapping is enabled, schema assessment metrics include detection rate, mapped/unmapped counts, exclusion counts, validation successes/failures, and the active confidence threshold so CI dashboards can trend accuracy over time.

## Force-Include Override

The `--force-include` flag allows you to assess files that would normally be ignored by `.goneatignore` or `.gitignore`. This is especially useful for testing, fixtures, and validating normally-excluded content.

### Purpose

- **Test Fixtures**: Validate test data and schema fixtures that are typically ignored
- **Documentation Files**: Include vendor docs or generated files for specific assessments
- **Configuration Templates**: Assess template files that might be in ignored directories
- **Security Audits**: Include vendor dependencies for security scanning
- **Schema Validation**: Validate schema files in test directories

### Usage Patterns

#### Single File Override

```bash
# Validate a specific ignored file
goneat assess --categories schema \
  --force-include 'tests/fixtures/schemas/bad/invalid-schema.json'
```

#### Directory Pattern Override

```bash
# Include all files in an ignored directory
goneat assess --categories schema \
  --force-include 'tests/fixtures/**' \
  --schema-enable-meta
```

#### Multiple Pattern Override

```bash
# Include multiple ignored patterns
goneat assess --categories schema \
  --force-include 'tests/fixtures/**' \
  --force-include 'vendor/schemas/**' \
  --force-include 'docs/examples/**'
```

### Combined with Other Flags

#### Scoped Assessment

```bash
# Targeted assessment with scope limiting
goneat assess --scope --categories schema \
  --include tests/fixtures/schemas/bad/ \
  --force-include 'tests/fixtures/schemas/bad/**' \
  --format json --output fixtures-report.json
```

#### Schema-Specific Patterns

```bash
# Force-include with schema filtering
goneat assess --categories schema \
  --force-include 'tests/fixtures/**' \
  --schema-patterns "*.schema.yaml" \
  --schema-discovery-mode all
```

### Best Practices

1. **Quote Glob Patterns**: Always quote patterns to prevent shell expansion

   ```bash
   # Good
   goneat assess --force-include 'tests/**/*.yaml'

   # Bad (shell expands before goneat sees it)
   goneat assess --force-include tests/**/*.yaml
   ```

2. **Use with Target Scope**: Combine with `--include` or target directories for efficiency

   ```bash
   # More efficient
   goneat assess tests/fixtures --force-include 'tests/fixtures/**'

   # Less efficient (scans entire repo)
   goneat assess --force-include 'tests/fixtures/**'
   ```

3. **Multiple Patterns**: Use multiple `--force-include` flags for different patterns
   ```bash
   goneat assess \
     --force-include 'tests/fixtures/**' \
     --force-include 'docs/examples/**' \
     --force-include 'vendor/important/**'
   ```

### Ignore Overrides (DX)

Run assess on paths normally ignored by `.goneatignore`:

```bash
# Bring back ignored fixtures only (recommended; targeted directory)
goneat assess --scope --categories schema \
  --include tests/fixtures/schemas/bad/ \
  --force-include 'tests/fixtures/schemas/bad/**' \
  --format json -o schemas-report.json

# Validate a single ignored file
goneat assess --categories schema --include . \
  --force-include 'tests/fixtures/schemas/bad/bad-additionalprops-wrong.json'

# Scan everything in scope (may be noisy/slow)
goneat assess --no-ignore --include . --format concise
```

### Quoting Globs

Quote glob patterns to prevent your shell from expanding them before goneat receives them:

```bash
goneat assess --force-include '**/*.yaml' --categories schema
```

## Output Formats

### Markdown Format (Default)

Human-readable reports with structured sections:

```markdown
# Codebase Assessment Report

**Generated:** 2025-08-28T10:30:00Z
**Tool:** goneat assess
**Target:** /path/to/project

## Executive Summary

- **Overall Health:** 🟢 Good (85% compliant)
- **Critical Issues:** 0
- **Estimated Fix Time:** 2-3 hours
- **Parallelizable Tasks:** 3 groups identified

## Assessment Results

### 🔧 Format Issues (Priority: 1)

**Status:** ⚠️ 3 issues found
**Estimated Time:** 15 minutes
**Parallelizable:** Yes

| File         | Issues | Severity | Auto-fixable |
| ------------ | ------ | -------- | ------------ |
| src/main.go  | 2      | Low      | Yes          |
| pkg/utils.go | 1      | Low      | Yes          |

### 🛡️ Security Issues (Priority: 2)

**Status:** ✅ No issues found

## Recommended Workflow

1. **Phase 1 (15 min)**: Auto-fix all format issues
2. **Phase 2 (30 min)**: Address critical lint issues
3. **Phase 3 (45 min)**: Review remaining items
```

### JSON Format

Machine-readable format for automation and integration:

```json
{
  "metadata": {
    "generated_at": "2025-08-28T10:30:00Z",
    "tool": "goneat",
    "version": "1.0.0",
    "target": "/path/to/project",
    "execution_time": "45s",
    "commands_run": ["gofmt", "golangci-lint", "gosec"],
    "change_context": {
      "modified_files": [
        "pkg/work/planner.go",
        "schemas/config/goneat-config-v1.0.0.yaml"
      ],
      "total_changes": 42,
      "change_scope": "small",
      "git_sha": "abc123def",
      "branch": "feature/diff-aware"
    }
  },
  "summary": {
    "overall_health": 0.85,
    "critical_issues": 0,
    "total_issues": 3,
    "estimated_time": "2h30m",
    "parallel_groups": 3,
    "categories_with_issues": 2
  },
  "categories": {
    "format": {
      "priority": 1,
      "issues_count": 3,
      "estimated_time": "15m",
      "parallelizable": true,
      "status": "success",
      "issues": [
        {
          "file": "src/main.go",
          "line": 42,
          "column": 5,
          "severity": "low",
          "message": "Incorrect indentation",
          "category": "format",
          "auto_fixable": true,
          "estimated_time": "5m",
          "change_related": true,
          "lines_modified": [40, 41, 42, 43]
        }
      ]
    }
  },
  "workflow": {
    "phases": [
      {
        "name": "Phase 1",
        "description": "Address format issues",
        "estimated_time": "15m",
        "categories": ["format"],
        "parallel_groups": ["group_1", "group_2"]
      }
    ],
    "parallel_groups": [
      {
        "name": "group_1",
        "description": "Format issues in main package",
        "files": ["src/main.go"],
        "categories": ["format"],
        "estimated_time": "5m",
        "issue_count": 2
      }
    ],
    "total_time": "2h30m"
  }
}
```

## Usage Examples

### Basic Assessment

```bash
# Quick assessment of current directory
goneat assess

# Assess with verbose output
goneat assess --verbose

# Assess specific directory
goneat assess ./src

# Save report to file
goneat assess --output assessment.md
```

### Category-Specific Assessment

```bash
# Only check formatting
goneat assess --categories format

# Check multiple categories
goneat assess --categories "format,lint"

# Lint new issues only against the previous commit
goneat assess --categories lint --lint-new-from-rev HEAD~ --format concise

# Security-focused assessment
goneat assess --categories security --fail-on high

# Dependencies assessment (supply-chain security)
goneat assess --categories dependencies --verbose

# Comprehensive security (vulnerabilities + dependencies)
goneat assess --categories security,dependencies --fail-on high
```

### CI/CD Integration

```bash
# JSON output for automated processing
goneat assess --format json --output results.json

# Fail build on any issues
goneat assess --fail-on low

# Include only source files
goneat assess --include "*.go" --exclude "vendor/**"

# Quick check for pre-commit hooks
goneat assess --categories format,lint --timeout 30s

# Track security suppressions in output
goneat assess --categories security --track-suppressions --format json
```

### Custom Priorities

```bash
# Security first, then format, then lint
goneat assess --priority "security=1,format=2,lint=3"

# Focus on performance issues
goneat assess --priority "performance=1" --categories performance
```

### Hook Testing

```bash
# Test what pre-commit hook would do
goneat assess --hook pre-commit

# Test pre-push validation
goneat assess --hook pre-push

# Debug hook execution
goneat assess --hook pre-commit --verbose
```

## Assessment Modes

Goneat assess supports three distinct operational modes with different behaviors:

### Check Mode (Default)

- **Purpose:** Report issues without making any changes to files
- **Use Case:** Regular assessment, CI/CD validation, compliance checking
- **Flags:** `--mode check`, `--check`, or default behavior
- **Behavior:**
  - Runs all assessment tools in read-only mode
  - Reports issues found with detailed information
  - Provides time estimates for fixes
  - Generates comprehensive reports
  - Safe for production environments

### Fix Mode

- **Purpose:** Report issues and automatically apply fixes where possible
- **Use Case:** Development workflow, pre-commit auto-fixing, code cleanup
- **Flags:** `--mode fix` or `--fix`
- **Behavior:**
  - Runs assessment tools and reports all issues found
  - Automatically applies fixes for auto-fixable issues
  - golangci-lint `--fix` flag for supported linters
  - gofmt/goimports for formatting issues
  - Reports remaining issues that require manual attention
  - **⚠️ Warning:** Modifies files - use with caution in production

### No-Op Mode

- **Purpose:** Validate configuration and tool availability without executing assessments
- **Use Case:** Setup testing, configuration validation, dry-run planning
- **Flags:** `--mode no-op` or `--no-op`
- **Behavior:**
  - Validates tool installation and configuration
  - Reports which assessment categories are available
  - Shows execution plan without running tools
  - Perfect for CI/CD pipeline validation
  - Zero risk - no file modifications

### Mode Selection Priority

When multiple mode flags are provided, goneat follows this priority order:

1. Explicit mode flag: `--mode check/fix/no-op`
2. Shorthand flags: `--no-op`, `--check`, `--fix`
3. Default: Check mode (safest option)

**Examples:**

```bash
# These are equivalent (check mode)
goneat assess
goneat assess --mode check
goneat assess --check

# These are equivalent (fix mode)
goneat assess --mode fix
goneat assess --fix

# These are equivalent (no-op mode)
goneat assess --mode no-op
goneat assess --no-op

# Error: Multiple modes specified
goneat assess --check --fix  # ❌ Invalid combination
```

### Mode-Specific Behavior by Category

| Category            | Check Mode                        | Fix Mode                    | No-Op Mode                          |
| ------------------- | --------------------------------- | --------------------------- | ----------------------------------- |
| **Format**          | Report formatting issues          | Apply gofmt/goimports fixes | Validate gofmt availability         |
| **Lint**            | Report lint violations            | Apply golangci-lint --fix   | Validate golangci-lint installation |
| **Static Analysis** | Report analysis issues            | Report only (no auto-fix)   | Validate go vet availability        |
| **Security**        | Report security issues            | Report only (manual fixes)  | Validate security tools             |
| **Performance**     | Report optimization opportunities | Report only (architectural) | Validate performance tools          |

## Priority System

Goneat uses an intelligent priority system to optimize assessment order:

### Default Priorities

1. **Format** (Priority 1) - Quick wins, often auto-fixable
2. **Security** (Priority 2) - Critical issues requiring immediate attention
3. **Static Analysis** (Priority 3) - Code correctness and potential bugs
4. **Lint** (Priority 4) - Code quality and style issues
5. **Performance** (Priority 5) - Optimization opportunities

### Custom Priorities

Override defaults for specific use cases:

```bash
# Security-first assessment
goneat assess --priority "security=1,format=2,lint=3"

# Development workflow (format first)
goneat assess --priority "format=1,lint=2,security=3"

# Performance audit
goneat assess --priority "performance=1"
```

## Three-Mode Workflow Examples

### Development Workflow

```bash
# Step 1: No-op mode - Validate setup before making changes
goneat assess --no-op --verbose

# Step 2: Check mode - See what issues exist
goneat assess --check --format markdown --output current-issues.md

# Step 3: Fix mode - Automatically fix what can be fixed
goneat assess --fix --categories format,lint

# Step 4: Final check - Verify remaining issues
goneat assess --check --fail-on high
```

### CI/CD Pipeline

```bash
# Early validation (fast feedback)
goneat assess --no-op --timeout 30s

# Comprehensive check (quality gate)
goneat assess --check --format json --output assessment.json --fail-on high

# Optional: Auto-fix for development branches
if [ "$BRANCH" != "main" ]; then
  goneat assess --fix --categories format
fi
```

### Pre-commit Hook

```bash
#!/bin/bash
# .git/hooks/pre-commit

# Quick validation
goneat assess --no-op --quiet

# Auto-fix formatting issues
goneat assess --fix --categories format

# Check remaining issues (fail if critical)
goneat assess --check --categories lint,security --fail-on high
```

### Code Review Preparation

```bash
# Assess only changed files
git diff --name-only | xargs goneat assess --check

# Auto-fix formatting before review
goneat assess --fix --categories format

# Generate review report
goneat assess --check --format markdown --output review-report.md
```

### Troubleshooting and Debugging

```bash
# Debug mode issues
goneat assess --check --verbose --timeout 10m

# Isolate category issues
goneat assess --check --categories lint --verbose

# Test configuration without execution
goneat assess --no-op --hook pre-commit
```

## Parallel Execution

Goneat automatically identifies and executes independent tasks in parallel:

### Automatic Parallelization

- **Format checks** run in parallel across files
- **Independent lint rules** execute simultaneously
- **Security scans** of different file types run concurrently

### Parallel Groups

The assessment report identifies parallelizable work:

```json
{
  "workflow": {
    "parallel_groups": [
      {
        "name": "format_group_1",
        "description": "Format issues in main package",
        "files": ["src/main.go", "src/utils.go"],
        "estimated_time": "10m"
      },
      {
        "name": "lint_group_1",
        "description": "Lint issues in handlers",
        "files": ["api/handlers.go"],
        "estimated_time": "15m"
      }
    ]
  }
}
```

## Integration Examples

### Git Hook Integration

```bash
# .git/hooks/pre-commit
#!/bin/bash
goneat assess --hook pre-commit --hook-manifest .goneat/hooks.yaml
```

### GitHub Actions

```yaml
- name: Code Assessment
  run: |
    goneat assess --format json --output assessment.json
    # Upload assessment.json as artifact
```

### Pre-commit Framework

```yaml
repos:
  - repo: local
    hooks:
      - id: goneat-assess
        name: goneat assessment
        entry: goneat assess --categories format,lint
        language: system
        files: \.(go)$
```

### VS Code Integration

```json
{
  "go.formatTool": "goneat",
  "go.lintTool": "goneat",
  "go.vetOnSave": "package",
  "go.lintOnSave": "package"
}
```

## Performance Optimization

### Caching

- **Result caching:** Skip unchanged files between runs
- **Tool availability:** Cache which tools are installed
- **Configuration:** Cache parsed manifest files

### Smart Filtering

- **File type detection:** Only run relevant tools on appropriate files
- **Change detection:** Use git status to identify modified files
- **Dependency analysis:** Skip files that haven't changed

### Resource Management

- **Timeout handling:** Prevent runaway tool execution
- **Memory limits:** Control resource usage for large codebases
- **Parallel limits:** Respect system capabilities

## Troubleshooting

### Common Issues

**Assessment fails with timeout:**

```bash
# Increase timeout for large codebases
goneat assess --timeout 10m

# Run specific categories to isolate issues
goneat assess --categories format
```

**Tools not found:**

```bash
# Check tool availability
goneat assess --verbose

# Install missing tools
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

**Configuration errors:**

```bash
# Validate configuration
cat .goneat/hooks.yaml

# Test with minimal config
goneat assess --categories format
```

### Performance Issues

**Slow execution:**

```bash
# Use parallel execution
goneat assess --parallel

# Focus on changed files only
goneat assess --only-changed

# Exclude vendor directories
goneat assess --exclude "vendor/**"
```

**Memory issues:**

```bash
# Limit concurrent operations
goneat assess --max-workers 2

# Process in batches
goneat assess --batch-size 10
```

## Advanced Usage

### Custom Assessment Pipelines

Create specialized assessment workflows:

```bash
# Security audit pipeline
goneat assess --categories security --fail-on high --format json

# Performance analysis
goneat assess --categories performance --verbose

# Documentation validation
goneat assess --categories docs --output docs-report.md
```

### Integration with External Tools

Combine goneat with other validation tools:

```bash
# Run goneat assessment
goneat assess --format json --output goneat-results.json

# Run additional tools
sonar-scanner
goreportcard-cli

# Combine results
./scripts/combine-reports.sh goneat-results.json sonar-results.json
```

### Custom Reporting

Generate specialized reports:

```bash
# Team-specific report
goneat assess --format markdown --template team-template.md

# Compliance report
goneat assess --categories security --format json --output compliance.json

# Executive summary
goneat assess --format executive-summary --output summary.md
```

## Future Enhancements

The assess command is designed for extensibility:

- **Extended Output (`--extended`):** Enhanced output format with detailed workplan information including file discovery details, category planning, and execution transparency for debugging and automation ✅ **Available**
- **Dependencies Category:** Supply-chain security and license compliance validation ✅ **Available** (see [Dependency Gating Workflow](../workflows/dependency-gating.md))
- **Additional categories:** Testing, documentation coverage
- **Custom tools:** Plugin system for proprietary validators
- **Machine learning:** Intelligent prioritization based on codebase patterns
- **Distributed execution:** Cluster support for large monorepos
- **Real-time feedback:** IDE integration with incremental assessment

### Extended Output Format (`--extended`)

The `--extended` flag enhances output with comprehensive workplan details:

```json
{
  "metadata": { ... },
  "summary": { ... },
  "workplan": {
    "files_discovered": 45,
    "files_included": 12,
    "files_excluded": 33,
    "exclusion_reasons": {
      "gitignore": 20,
      "goneatignore": 8,
      "pattern_mismatch": 5
    },
    "categories_planned": ["dates", "format", "lint"],
    "categories_skipped": ["security"],
    "skip_reasons": { "security": "no matching files" },
    "estimated_duration": "15s",
    "file_list": ["CHANGELOG.md", "README.md", "internal/dates/dates.go"],
    "discovery_patterns": {
      "include": ["**/*.md", "**/*.go"],
      "exclude": ["**/vendor/**", "**/.git/**"]
    }
  },
  "categories": { ... }
}
```

This enhanced output provides:

- **Complete file discovery transparency** - see exactly what files were found, included, and excluded
- **Category planning details** - understand why categories were run or skipped
- **Debugging information** - pattern matching results and configuration resolution
- **Automation support** - structured data for CI/CD pipeline decisions
- **Performance insights** - detailed execution times per category and worker utilization

### Usage Examples

```bash
# Enhanced JSON output for automation
goneat assess --extended --format=json --output=detailed-report.json

# Enhanced Markdown output for human review
goneat assess --extended --format=markdown --output=detailed-report.md

# Debug single file processing
goneat assess --categories=dates --extended CHANGELOG.md

# CI/CD pipeline with detailed workplan
goneat assess --extended --format=json | jq '.workplan.execution_summary'
```

## Related Commands

- [`goneat hooks`](hooks.md) - Git hook management
- [`goneat format`](format.md) - Code formatting (planned)
- [`goneat lint`](lint.md) - Code linting (planned)
- [`goneat version`](version.md) - Version information

## See Also

- [Git Hooks Operation Workflow](../workflows/git-hooks-operation.md) - Complete setup guide
- [Assessment Architecture](../../architecture/assess-workflow.md) - Technical implementation
- [Hooks Architecture](../../architecture/hooks-command-architecture.md) - Hook integration design

## Change-Aware Assessment

Goneat includes diff-aware intelligence to prioritize and contextualize results in large repositories:

- Diff-aware metadata: `metadata.change_context` summarizes modified files, total changes, scope (small/medium/large), branch and SHA.
- Issue focus: Each issue includes `change_related` when it targets a modified file and may include `lines_modified` for precise correlation.
- Benefits: Reduces noise by highlighting what changed, speeds review, and enables smarter CI gates (e.g., block only when the change set triggers violations).

This works offline and gracefully degrades outside a git repository.

## Suppression Tracking (Security)

Expose intentional suppressions for security tools that support them (e.g., gosec):

- Enable with `--track-suppressions` on `assess` or via config.
- Output includes `categories.security.suppression_report` with:
  - `suppressions`: detailed list with file, line, rule, reason
  - `summary.by_rule`: counts per rule
  - `summary.by_rule_files`: files per rule (deduplicated)
  - `summary.by_file`: counts per file
  - `summary.top_rules`: top rules by count
  - `summary.top_files`: top files by count
  - `summary.by_tool`, `summary.by_severity`, `with_reason/without_reason`

Example JSON excerpt:

```json
{
  "categories": {
    "security": {
      "suppression_report": {
        "suppressions": [
          {
            "tool": "gosec",
            "rule_id": "G404",
            "file": "internal/db/store.go",
            "line": 120,
            "reason": "input validated",
            "syntax": "#nosec G404"
          }
        ],
        "summary": {
          "total": 3,
          "by_tool": { "gosec": 3 },
          "by_rule": { "G404": 2, "G304": 1 },
          "by_rule_files": {
            "G404": ["internal/db/store.go"],
            "G304": ["cmd/main.go"]
          },
          "by_file": { "internal/db/store.go": 2, "cmd/main.go": 1 },
          "top_rules": [
            { "name": "G404", "count": 2 },
            { "name": "G304", "count": 1 }
          ],
          "top_files": [{ "name": "internal/db/store.go", "count": 2 }],
          "with_reason": 2,
          "without_reason": 1
        }
      }
    }
  }
}
```
