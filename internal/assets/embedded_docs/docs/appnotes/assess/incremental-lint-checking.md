---
title: "Incremental Lint Checking"
description: "Guide to using --new-issues-only for incremental lint checking in goneat assess"
author: "goneat contributors"
date: "2026-01-02"
last_updated: "2026-01-02"
status: "approved"
tags: ["assess", "lint", "incremental", "hooks", "golangci-lint", "biome"]
category: "appnotes"
---

# Incremental Lint Checking

This guide explains how to use goneat's incremental lint checking feature to report only issues introduced since a specified git reference.

## Overview

By default, `goneat assess --categories lint` reports **all** lint issues in the codebase. For projects with existing lint debt, this can be noisy and slow in git hooks.

The `--new-issues-only` flag enables **incremental lint checking**, which reports only issues introduced since a baseline git reference (default: `HEAD~`).

## When to Use Incremental Checking

**Use incremental checking when:**

- You have existing lint debt that cannot be fixed immediately
- You want faster feedback in pre-commit/pre-push hooks
- You want to prevent new issues without blocking on existing ones
- You're gradually improving code quality over time

**Use full checking (default) when:**

- You want to see all issues in the codebase
- Running comprehensive CI/CD quality gates
- Performing code audits or compliance checks
- Your codebase has minimal or zero lint debt

## Basic Usage

### Direct Assessment

```bash
# Report only NEW lint issues since previous commit
goneat assess --categories lint --new-issues-only

# Report only NEW issues since main branch
goneat assess --categories lint --new-issues-only --new-issues-base main

# Report only NEW issues since a specific commit
goneat assess --categories lint --new-issues-only --new-issues-base abc123
```

### Git Hook Configuration

To enable incremental checking in git hooks, add `--new-issues-only` to your `.goneat/hooks.yaml`:

```yaml
version: "1.0.0"
hooks:
  pre-commit:
    - command: "assess"
      args: ["--categories", "format,lint", "--fail-on", "high", "--new-issues-only"]
      priority: 10
      timeout: "2m"
  pre-push:
    - command: "assess"
      args: ["--categories", "lint,security", "--fail-on", "high", "--new-issues-only", "--new-issues-base", "main"]
      priority: 10
      timeout: "3m"
```

After modifying hooks.yaml, regenerate and reinstall hooks:

```bash
goneat hooks generate && goneat hooks install
```

## Flag Reference

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--new-issues-only` | bool | `false` | Only report issues introduced since base reference |
| `--new-issues-base` | string | `HEAD~` | Git reference for baseline comparison |

**Note:** `--new-issues-base` has no effect without `--new-issues-only`. If used alone, a warning is emitted.

## Tool Support

Incremental checking is supported by tools that have native git-aware diffing:

| Tool | Language | Native Flag | Support |
|------|----------|-------------|---------|
| golangci-lint | Go | `--new-from-rev REF` | Full |
| biome | JS/TS | `--changed --since=REF` | Full |
| ruff | Python | N/A | None (file-scoped only) |
| gosec | Go | N/A | None (full scan always) |
| staticcheck | Go | N/A | None (full scan always) |

Tools without incremental support run full scans regardless of flag setting. This means:

- Go projects: golangci-lint respects the flag
- JS/TS projects: biome respects the flag
- Python projects: ruff always runs full scans
- Security scans: always run full scans

## Behavior Matrix

| Context | `--new-issues-only` | Behavior |
|---------|---------------------|----------|
| Direct assess (default) | `false` | Report ALL issues |
| Direct assess (explicit) | `true` | Report issues since `--new-issues-base` |
| Hook mode (default) | `false` | Report ALL issues |
| Hook mode (explicit in hooks.yaml) | `true` | Report issues since base |

**Important:** Hook mode does NOT implicitly enable incremental checking. You must explicitly add `--new-issues-only` to your hooks.yaml args.

## Migration from v0.4.0

In v0.4.0 and earlier, hook mode implicitly applied incremental lint checking (`LintNewFromRev = HEAD~`). This caused:

- Inconsistent results between `goneat assess` and `goneat assess --hook pre-commit`
- Silent accumulation of lint debt when commits bypassed hooks with `--no-verify`

**v0.4.1 changes:**

- Hook mode now reports ALL issues by default (consistent with direct assess)
- Incremental checking is opt-in via explicit `--new-issues-only` flag
- This is a behavioral change - existing hooks may report more issues

**To restore previous behavior**, add `--new-issues-only` to your hooks.yaml:

```yaml
# Before (v0.4.0 - implicit incremental)
hooks:
  pre-commit:
    - command: assess
      args: ["--categories", "format,lint"]

# After (v0.4.1 - explicit opt-in)
hooks:
  pre-commit:
    - command: assess
      args: ["--categories", "format,lint", "--new-issues-only"]
```

## Enterprise Considerations

### Lint Debt Management

Incremental checking can mask existing lint debt. Consider:

1. **Baseline tracking**: Periodically run full scans to track total debt
2. **Debt reduction sprints**: Schedule time to address accumulated issues
3. **CI full scans**: Run full scans in CI even if hooks use incremental

### Bypass Detection

Issues introduced via `--no-verify` bypass are caught on the next hook run when:

- Using incremental mode: Only if the bypass commit is within the base range
- Using full mode: Always caught

### Recommended Workflow

```yaml
# Pre-commit: fast feedback, incremental
hooks:
  pre-commit:
    - command: assess
      args: ["--categories", "format,lint", "--new-issues-only", "--fail-on", "high"]

# Pre-push: more thorough, still incremental against main
  pre-push:
    - command: assess
      args: ["--categories", "lint,security", "--new-issues-only", "--new-issues-base", "main", "--fail-on", "high"]
```

And in CI:

```bash
# Full scan for comprehensive quality gate
goneat assess --categories lint,security --fail-on medium
```

## Troubleshooting

### "No issues found" when issues exist

If `--new-issues-only` reports no issues but you know issues exist:

1. Check if issues pre-date the base reference
2. Run without `--new-issues-only` to see all issues
3. Verify the base reference is correct: `git log --oneline -5`

### Base reference not found

If the base reference doesn't exist:

```
fatal: bad revision 'main'
```

The tool will fall back to reporting all issues. Ensure the base branch/commit exists locally.

### Warning: "--new-issues-base has no effect"

This warning appears when you specify `--new-issues-base` without `--new-issues-only`:

```
WARN: --new-issues-base has no effect without --new-issues-only; base reference will be ignored
```

Add `--new-issues-only` to enable incremental checking.

## See Also

- [Assess Command Reference](../../user-guide/commands/assess.md) - Full flag reference
- [Hooks Command Reference](../../user-guide/commands/hooks.md) - Hook configuration
- [Git Hooks Operation Workflow](../../user-guide/workflows/git-hooks-operation.md) - Setup guide
