---
title: "Push Gate Is One Check-Only Assess"
description: "The git hook, make prepush, and CI run the same goneat assess --mode check invocation and do not rewrite tracked files"
author: "goneat contributors"
date: "2026-10-10"
last_updated: "2026-10-10"
status: "approved"
tags:
  - "architecture"
  - "ci"
  - "hooks"
category: "architecture"
---

# ADR-0004: Push Gate Is One Check-Only Assess

## Status

**APPROVED** - The pre-push git hook, `make prepush`, and CI use one check-only assess.

## Context

A push can be gated in three places: the git pre-push hook, `make prepush` / `make pr-final`, and CI. Those callers drift when one of them grows extra steps.

Goneat already has the check-only forms:

- `goneat assess --mode check` reports issues. Hook execution forces this mode, including when `--fix` or `--lint-shell-fix` is also present, and does not apply `shfmt.fix`.
- `goneat format --check` reports format drift. A hook `format` command is invoked with `--check` last.
- A hook `dependencies` command does not receive `--output`, `--sbom-output`, `--sbom`, or `--vuln`.

`make release-prepare` is a different class of work. It builds a binary, syncs Crucible, and embeds assets. Those steps write tracked files. `make release-check` runs that prepare step and then tests, lint, schema checks, Crucible verification, and the license audit.

Putting `make release-check` or `make prepush` inside the git hook mixes that write step into the push. A hook that rewrites the tree is not a stable gate. Replacing the hook with a smaller, different command list creates the other failure: the hook and `make prepush` no longer check the same things, and CI checks a third set.

### Goals

- One push gate for the git hook, `make prepush`, and CI.
- That gate flags errors and does not rewrite tracked files.
- Release preparation stays an explicit, separate target.

### Constraints

- Categories, fail level, and timeout for the push gate live in `.goneat/hooks.yaml`.
- The hook template calls `goneat` directly. It does not call `make`, so a later edit to a Makefile target cannot add a write step to the hook.

## Decision

The push gate is:

```bash
goneat assess --mode check --hook pre-push --hook-manifest .goneat/hooks.yaml --staged-only --package-mode
```

The pre-commit gate is the same command with `--hook pre-commit`.

`make prepush` and `make pr-final` run that pre-push command. `make precommit` runs the pre-commit command. CI runs `make prepush` after the binary exists. The git hooks run the same `goneat assess --mode check` arguments and do not call `make`.

`make release-check` remains the release target. It is not the push gate.

The dates category in this assess compares content dates with the earliest commit. A shallow repository does not contain that commit. The check reports a high issue, `repository is shallow`, and does not use the newest fetched commit as repository creation. A job that runs the push gate checks out the full history (`fetch-depth: 0`). `scripts/push-gate-preflight.py` checks for a shallow repository before the assess.

The tools category checks the foundation tools in `.goneat/tools.yaml`. `scripts/install-push-gate-tools.sh` installs shellcheck and yamllint at the recommended versions when the copies on `PATH` are missing or older. CI runs that script, then the preflight, then `make prepush`. The assess command is the same one the git hook runs. The security category runs the scanners it finds on `PATH`. When none of those scanners are on `PATH`, the category reports that it was skipped.

Hook assessment reports only. `--fix`, `--lint-shell-fix`, and `shfmt.fix` do not rewrite files during a hook assess. The dependency category still writes a vulnerability report under `sbom/`. That directory is gitignored. The push gate does not modify tracked files.

## Rationale

1. **One class of operation.** The hook, the Makefile target, and CI either accept or reject the same assess result.
2. **Check-only calls.** The mode and the hook executor select report-only behavior. A flag on the same command cannot turn the gate into a rewrite.
3. **Writes stay named.** Embedding, sync, and release builds stay on `make release-prepare` and `make release-check`.

## Alternatives Considered

### Hook runs `make prepush`

**Approach**: The generated hook delegates to the Makefile target.

**Rejected because**: The target that people name `prepush` had included `release-prepare`. That writes tracked files. The hook would then see the files it wrote.

### A thinner hook than `make prepush`

**Approach**: Leave `make prepush` on the release-check chain and keep the hook on a smaller assess.

**Rejected because**: The two push checks diverge. A failure shows up in one place and not the other.

### Fix mode in the hook, then re-stage

**Approach**: Let the hook format files and require the caller to stage the result.

**Rejected because**: The push gate's job is to flag errors. Rewriting the tree is a separate command (`goneat assess --fix`, `goneat format`).

## Consequences

### Positive

- ✅ The git hook, `make prepush`, and CI run one assess.
- ✅ That assess does not rewrite tracked files.
- ✅ Release preparation remains available as `make release-check`.

### Negative

- ⚠️ `make prepush` no longer runs tests, embed, or cross-platform builds.
  - **Mitigation**: `make test` and `make release-check` remain the targets for those steps. CI runs `make test` and `make prepush` as separate steps.
- ⚠️ The dependency scan writes an ignored report under `sbom/`.
  - **Mitigation**: The report is not a tracked file. The gate result is the assess status.

## Implementation

1. `.goneat/hooks.yaml` lists the pre-push and pre-commit assess arguments.
2. `templates/hooks/` call `goneat assess --mode check`.
3. `cmd/assess.go` forces check mode for hook execution.
4. `Makefile` `prepush`, `pr-final`, and `precommit` run those assess commands and do not depend on `release-check`.
5. `.github/workflows/ci.yml` checks out full history, runs `scripts/install-push-gate-tools.sh` and `scripts/push-gate-preflight.py`, then runs `make prepush`.

## References

- [Hook execution models](../hook-execution-models.md)
- [Hooks command guide](../../user-guide/commands/hooks.md)
- `.goneat/hooks.yaml`
- `cmd/assess.go` (`forceHookReportOnly`)

## Timeline

- **Approved**: 2026-10-10
- **Implemented**: 2026-10-10
