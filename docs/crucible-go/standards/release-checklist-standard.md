---
title: "Release Checklist Standard"
description: "How Fulmen repositories use the shared release checklist template"
author: "Codex Assistant"
date: "2025-10-02"
last_updated: "2025-10-02"
status: "draft"
tags: ["standards", "release", "process", "quality"]
---

# Release Checklist Standard

## Purpose

Provide a consistent, lightweight playbook that every Fulmen repository can use to validate releases. The checklist lives at the repository root as `RELEASE_CHECKLIST.md` and acts as a **template** – copy or duplicate it for each release (PR description, GitHub issue, etc.) rather than editing the template in place.

## Why a Template?

- Keeps the repo free from stale release metadata.
- Allows teammates to copy/paste into a release issue while the root document remains evergreen.
- Ensures newcomers see every gate that must pass before tagging.

## Required Sections

Release checklists **must** include the following blocks:

1. **Metadata** – version, target date, release captain (fill in when copy is made).
2. **Pre-Release Validation** – formatting, tests, schema sync, docs, changelog, version alignment.
3. **Integration Testing** (if applicable) – multi-tier test strategy:
   - Tier 1: Mandatory synthetic tests (CI-friendly, < 10s)
   - Tier 2: Quick validation with real dependencies (pre-release, ~8s)
   - Tier 3: Full suite (major releases only, ~2 min)
4. **Packaging & Distribution** – language-specific publish steps, package builds, CI verification.
5. **Tagging & Announcement** – git tag/push, release notes, documentation updates.
6. **Post-Release Validation** – module install checks, downstream sync confirmations, monitoring follow-up.
7. **Rollback Plan** – commands and comms channel to revert quickly if needed.

Projects can append repository-specific gates (e.g., cross-platform binary builds for `goneat`).

## Usage Pattern

1. **Copy the template** into a new GitHub issue or PR at release time.
2. **Fill in metadata** (version/date/captain) and check boxes as tasks complete.
3. **Link back** to the issue/PR from release notes for audit trail.
4. **Close or archive** the filled-in checklist after the release.

### Integration Test Tiers

For projects with integration tests:
- **Always run Tier 1** (included in `make test`, < 10s, no dependencies)
- **Run Tier 2 before releases** (quick validation, ~8s with test repos)
- **Run Tier 3 for major releases** (comprehensive, ~2 min, all scenarios)

Environment setup:
```bash
export GONEAT_COOLING_TEST_ROOT=$HOME/dev/playground
# Or clone test repos to ~/dev/playground/
```

See project-specific integration test protocol for details.

## Crucible-Specific Notes

- Always run `bun run sync:to-lang` and `bun run version:update` before tagging.
- Ensure both Go and TypeScript tests pass (`bun run test:go`, `bun run test:ts`).
- Confirm documentation updates in `docs/` and language READMEs.
- Verify published packages embed the same `VERSION` string (CalVer).

## Related Resources

- [Schema Normalization Standard](schema-normalization.md)
- [Repository Versioning Standard](repository-versioning.md)
- [Crucible Sync Model Architecture](../architecture/sync-model.md)
