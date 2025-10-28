---
title: "SSOT Dirty Detection: Repository vs Global Gitignore"
description: "Architectural decision to use repository .gitignore only for SSOT dirty state detection"
author: "@code-scout"
date: "2025-10-28"
last_updated: "2025-10-28"
status: "approved"
tags:
  - "architecture"
  - "ssot"
  - "git"
  - "provenance"
  - "quality"
category: "architecture"
---

# ADR-0002: SSOT Dirty Detection - Repository vs Global Gitignore

## Status

**APPROVED** - Implemented in v0.3.2

## Context

Goneat's Single Source of Truth (SSOT) system tracks provenance metadata that includes whether the crucible repository is in a "dirty" state (has uncommitted changes or untracked files). This dirty detection is critical because:

1. **Release Quality**: Clean provenance indicates the documentation and schemas embedded in goneat match committed crucible sources
2. **Documentation Consistency**: Ensures embedded docs reflect the actual state of crucible
3. **Build Integrity**: CI/CD relies on clean state for reliable builds
4. **Developer Experience**: Prepush validation blocks pushes when crucible sources are dirty

### The Bug Discovered

After implementing prepush validation to block dirty crucible sources, we discovered the dirty detection had a false positive bug:

**Symptom**: Crucible repository reported as "dirty" even when `git status` showed "working tree clean"

**Root Cause**: The go-git library's `Status().IsClean()` method includes ALL untracked files in its dirty check, even those matched by gitignore patterns. This differs from git's behavior.

**Specific Test Case**:
- File: `crucible/.claude/settings.local.json`
- Status: Untracked, but matched by global gitignore (`~/.config/git/ignore`)
- Not in crucible's repository `.gitignore`
- Git command: `git status --porcelain` showed nothing (clean)
- Git command: `git status --porcelain --ignored` showed `!! .claude/settings.local.json`
- go-git Status: Included file with `Worktree='?'` (untracked), marking repo as dirty

**Investigation**:
```bash
# Crucible showed clean via git CLI
$ cd crucible && git status --porcelain
# (empty output)

# But showed ignored files with --ignored flag
$ git status --porcelain --ignored
!! .claude/settings.local.json

# File was in global gitignore, not repo gitignore
$ git check-ignore -v .claude/settings.local.json
/Users/davethompson/.config/git/ignore:1:**/.claude/settings.local.json
```

## Decision

**We will check repository `.gitignore` only (and `.git/info/exclude`) for dirty state detection, ignoring global gitignore patterns.**

### Implementation Approach

**File**: `pkg/ssot/metadata.go:introspectRepository()`

Instead of using go-git's `Status().IsClean()` which includes all untracked files, we:

1. Load repository `.gitignore` patterns using `gitignore.ReadPatterns(worktree.Filesystem, nil)`
2. Include repository-local excludes from `.git/info/exclude` via `worktree.Excludes`
3. Create a matcher from these patterns: `gitignore.NewMatcher(patterns)`
4. Iterate through status, filtering untracked files through the matcher
5. Mark repository dirty only for:
   - Modified tracked files
   - Untracked files NOT matched by repository gitignore patterns

**Code Example**:
```go
// Load repository .gitignore patterns
patterns, err := gitignore.ReadPatterns(worktree.Filesystem, nil)
if err == nil {
    // Include .git/info/exclude patterns (repository-local)
    patterns = append(patterns, worktree.Excludes...)
    matcher := gitignore.NewMatcher(patterns)

    // Check each file status
    for path, fileStatus := range status {
        if fileStatus.Worktree == git.Untracked {
            // Check if matched by repo .gitignore
            pathParts := strings.Split(path, "/")
            isIgnored := matcher.Match(pathParts, false)
            if isIgnored {
                continue // Skip - properly ignored by repo
            }
            dirty = true // Untracked file NOT in repo .gitignore
            break
        }
        // Modified tracked files also mark as dirty
        if fileStatus.Worktree != git.Unmodified {
            dirty = true
            break
        }
    }
}
```

## Rationale

### Why Repository `.gitignore` Only?

1. **Team Source of Truth**
   - Repository `.gitignore` is committed and shared across the team
   - All developers and CI/CD see the same ignore rules
   - Explicit, documented, and version-controlled

2. **CI/CD Alignment**
   - CI/CD environments don't have global gitignore
   - Behavior is consistent between local dev and CI/CD
   - Prevents "works on my machine" scenarios

3. **Explicit Intent**
   - If files should be ignored for dirty detection, add them to repo `.gitignore`
   - No hidden assumptions based on developer's personal config
   - Clear documentation of what's intentionally ignored

4. **Simpler Implementation**
   - Repository `.gitignore` is accessible via go-git's worktree filesystem
   - No need to access OS filesystem or parse global config
   - Standard go-git patterns work directly

### Supporting Evidence

- **Git Behavior**: `git status` only shows untracked files not matched by repository `.gitignore`
- **Team Consistency**: Repository `.gitignore` is the team's agreed-upon ignore patterns
- **CI/CD Standard**: GitHub Actions, GitLab CI, and other CI systems use repository `.gitignore` only

## Alternatives Considered

### Alternative 1: Skip All Untracked Files

**Approach**: Ignore all untracked files in dirty detection, checking only tracked file modifications

**Pros**:
- Simplest implementation
- No gitignore parsing needed
- Never false positives from ignored files

**Cons**:
- Too permissive - misses legitimate untracked files that should trigger dirty state
- Developer could have important uncommitted new files
- Defeats purpose of comprehensive dirty detection

**Rejected because**: Would miss legitimate untracked files that indicate work in progress. The goal of dirty detection is to ensure ALL changes are committed, not just modifications to tracked files.

### Alternative 2: Load Global Gitignore

**Approach**: Parse and apply both repository `.gitignore` and global gitignore (`~/.config/git/ignore`, `~/.gitignore`, etc.)

**Pros**:
- Matches individual developer's local git behavior exactly
- No "false positives" for developers with global ignore rules

**Cons**:
- **Personal Configuration**: Global gitignore varies per developer
- **CI/CD Mismatch**: CI doesn't use global gitignore
- **Team Inconsistency**: Different developers see different dirty states
- **Complex Implementation**: Requires OS filesystem access (not worktree), parsing multiple locations
- **Platform-Dependent**: Different paths on Windows, macOS, Linux

**Rejected because**: Global gitignore is personal preference, not team policy. CI/CD is the source of truth for what should be committed. Repository `.gitignore` is the explicit, shared team contract.

### Alternative 3: Match Git CLI Behavior Exactly

**Approach**: Shell out to `git status --porcelain` instead of using go-git

**Pros**:
- Matches git CLI behavior exactly
- No library discrepancies

**Cons**:
- Requires git CLI to be installed and available
- Platform-dependent (Windows vs Unix paths)
- Slower than in-memory go-git operations
- Additional subprocess management complexity

**Rejected because**: go-git provides the necessary functionality without external dependencies. The issue wasn't go-git's capability, but how we were using it. Proper gitignore pattern matching resolves the discrepancy.

## Consequences

### Positive

1. ✅ **Team Consistency**
   - All team members see the same dirty state
   - No surprises from personal gitignore configurations
   - Clear, shared understanding of ignored files

2. ✅ **CI/CD Alignment**
   - Local dirty detection matches CI/CD behavior
   - Prepush validation prevents CI failures
   - Reliable provenance metadata

3. ✅ **Explicit and Documented**
   - Repository `.gitignore` documents what's intentionally ignored
   - Version-controlled ignore patterns
   - Easy to audit and review

4. ✅ **Simpler Implementation**
   - No OS filesystem access required
   - Standard go-git patterns
   - Cross-platform compatible

### Negative

1. ⚠️ **Developer-Specific Ignored Files May Show Dirty**
   - Developers with global gitignore patterns may see "dirty" for their personally-ignored files
   - **Mitigation**: Add common patterns to repository `.gitignore` (e.g., `.claude/`, `.vscode/`, editor configs)
   - **Example**: `.claude/settings.local.json` triggered this - now added to crucible's `.gitignore`

2. ⚠️ **Requires Repository Gitignore Maintenance**
   - Team must add common IDE and tool patterns to repo `.gitignore`
   - **Mitigation**: Template `.gitignore` with common patterns (GitHub's Go template is good baseline)

### Neutral

- `.git/info/exclude` (repository-local excludes) is still respected - good middle ground for truly local excludes

## Implementation

### Changes Made

**File**: `pkg/ssot/metadata.go:95-136` - Function `introspectRepository()`

**Before** (using `Status().IsClean()`):
```go
status, err := worktree.Status()
if err != nil {
    return err
}
if !status.IsClean() {
    dirty = true
}
```

**After** (filtering by repository gitignore):
```go
status, err := worktree.Status()
if err != nil {
    return err
}

// Load repository .gitignore patterns
patterns, err := gitignore.ReadPatterns(worktree.Filesystem, nil)
if err == nil {
    patterns = append(patterns, worktree.Excludes...)
    matcher := gitignore.NewMatcher(patterns)

    for path, fileStatus := range status {
        if fileStatus.Worktree == git.Untracked {
            pathParts := strings.Split(path, "/")
            isIgnored := matcher.Match(pathParts, false)
            if isIgnored {
                continue
            }
            dirty = true
            break
        }
        if fileStatus.Worktree != git.Unmodified {
            dirty = true
            break
        }
    }
} else {
    // Fallback to IsClean if pattern loading fails
    dirty = !status.IsClean()
}
```

### Testing Strategy

**Manual Testing**:
1. Add `.claude/` to crucible's `.gitignore`
2. Run `make sync-ssot` to update provenance
3. Verify crucible shows clean in provenance metadata
4. Verify prepush validation passes

**Integration Testing**:
- Existing tests in `pkg/ssot/` validate metadata introspection
- Prepush validation script: `scripts/verify-crucible-clean.sh`

### Validation Commands

```bash
# Check crucible status (CLI)
cd ../crucible && git status

# Sync goneat provenance
make sync-ssot

# Verify clean state
./scripts/verify-crucible-clean.sh

# Check what's gitignored
git check-ignore -v .claude/settings.local.json
```

## Monitoring and Success Criteria

### Success Indicators

- ✅ Crucible shows clean when `git status` shows clean
- ✅ Prepush validation passes for clean crucible
- ✅ Prepush validation blocks when crucible has legitimate untracked files
- ✅ CI/CD and local dev have consistent dirty detection

### Failure Indicators

- ⚠️ Developers consistently seeing false "dirty" states
- ⚠️ CI/CD passes but local prepush validation fails
- ⚠️ Legitimate untracked files not triggering dirty state

### Verification Results

The fix was verified with a 3-pass test demonstrating correct behavior:

| Pass | Crucible State | Provenance `dirty` | Expected | Result |
|------|----------------|-------------------|----------|---------|
| 1 | `.claude/settings.local.json` (global ignore only) | `true` | False positive bug | ❌ Bug present |
| 2 | `.gitignore` modified (uncommitted) | `true` | Correct (real change) | ✅ Working |
| 3 | `.gitignore` committed (clean) | *absent* | Correct (clean) | ✅ **Fixed!** |

**Pass 3 Result**: After adding `.claude/` to repository `.gitignore` and committing:
- Provenance shows no `dirty` field (indicates clean)
- Crucible commit updated: `00ab9d81ce2ed1e8906c6c78de817464db48abcf`
- `git status --porcelain` confirms clean state
- Fix verified: 2025-10-28

## Rollback Plan

If this decision causes issues:

1. Revert to `Status().IsClean()` check
2. Document known false positive from global gitignore
3. Add override flag to skip crucible dirty check

**Likelihood of rollback**: Very low - the implementation correctly aligns with git's behavior and CI/CD expectations.

## References

### Internal Documentation

- **Implementation**: `pkg/ssot/metadata.go:95-136`
- **Validation Script**: `scripts/verify-crucible-clean.sh`
- **Prepush Hook Template**: `internal/assets/embedded_templates/templates/hooks/bash/pre-push.sh.tmpl`
- **Related Feature**: Prepush validation (commit `fa63091`)

### Code References

- **Commit**: (pending) - "fix: correct dirty detection to respect repository .gitignore"
- **Related Commit**: `fa63091` - "feat: add prepush validation to block dirty crucible sources"
- **Related Commit**: `bd6f63d` - "feat: add version conflict detection and management to doctor command"

### External References

- [go-git Status documentation](https://pkg.go.dev/github.com/go-git/go-git/v5#Status)
- [go-git gitignore package](https://pkg.go.dev/github.com/go-git/go-git/v5/plumbing/format/gitignore)
- [Git Documentation: gitignore](https://git-scm.com/docs/gitignore)
- [Git Configuration: core.excludesFile](https://git-scm.com/docs/git-config#Documentation/git-config.txt-coreexcludesFile)

## Related Decisions

- **Prepush Validation**: Decision to block pushes when crucible is dirty
- **SSOT Provenance**: Decision to track crucible metadata in goneat provenance
- **Doctor Command**: Version conflict detection relies on clean provenance

---

**Decision made by**: @code-scout
**Approved by**: @3leapsdave
**Implementation**: v0.3.2
**Status**: Implemented and verified
