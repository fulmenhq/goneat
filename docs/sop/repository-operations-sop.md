# Goneat Repository Operations SOP

_Standard Operating Procedures for repository management, commits, and deployments_

**Version:** 1.0 **Last Updated:** August 27, 2025
**Maintainers:** DevOps/CI-CD Team

## Overview

This document establishes standard operating procedures for Goneat repository operations, ensuring consistent quality gates, security compliance, and professional development practices. Goneat focuses on work planning and formatting orchestration for "making it neat".

## Quality Gates Summary

| Gate           | Target           | Coverage | Security | Time  |
| -------------- | ---------------- | -------- | -------- | ----- |
| **check-all**  | Development      | N/A      | Basic    | ~5s   |
| **pre-commit** | Commit Ready     | 10%+     | Basic    | ~30s  |
| **pre-push**   | Production Ready | 70%+     | Full     | ~2min |

## Repository Phase Management

**Phase Files (Single Source of Truth):**

- `LIFECYCLE_PHASE`: Product maturity phase
  - **Allowed values**: `experimental`, `alpha`, `beta`, `rc`, `ga`, `lts`
  - **Current project phase**: `alpha` (30% coverage threshold)
- `RELEASE_PHASE`: Distribution cadence phase
  - **Allowed values**: `dev`, `rc`, `ga`
  - **Current project phase**: `rc` (70% coverage threshold)

**Coverage Thresholds by Phase:**

- **Lifecycle**: experimental=0%, alpha=30%, beta=60%, rc=70%, ga=75%, lts=80%
- **Release**: dev=50%, rc=70%, ga=75%
- **Policy**: Coverage gates use LIFECYCLE_PHASE threshold (authoritative). RELEASE_PHASE is for distribution cadence only.

**Documentation**: See [docs/standards/lifecycle-release-phase-standard.md](../standards/lifecycle-release-phase-standard.md) for complete definitions, validation rules, and change control procedures.

## Commit Operations

### Standard Commit Workflow

#### 1. Pre-Commit Quality Check

```bash
# MANDATORY: Run quality checks before any commit work
make check-all
```

**Requirements:**

- All formatting checks pass (`fmt-strict`)
- Static analysis clean (`vet`)
- Linting passes with 0 issues (`lint`)

#### 2. File Staging Strategy

##### Full Repository Staging

```bash
# For feature completion, refactoring, or comprehensive changes
git add .
```

**Use Cases:**

- Initial commits
- Version releases
- Major feature completion
- Documentation updates

##### Selective File Staging

```bash
# For targeted fixes or incremental development
git add specific/file/path.go
git add specific/directory/
```

**Use Cases:**

- Bug fixes
- Single feature development
- Security patches
- Dependency updates

#### 3. Staged File Inspection & Cleanup

```bash
# MANDATORY: Review staged files before commit
git status
git diff --cached --name-only

# Remove extraneous files if needed
git reset HEAD unwanted/file.go

# Update .gitignore for new patterns
# Edit .gitignore, then:
git add .gitignore
```

**Critical Checks:**

- No temporary files (`.tmp`, `.cache`, etc.)
- No build artifacts in `dist/`, `coverage/`
- No IDE files (`.vscode/`, `.idea/`)
- No sensitive data or credentials
- No AI tool artifacts (`.claude/`, `.cursor/`)

#### 4. Pre-Commit Validation

```bash
# MANDATORY: Full pre-commit validation
make pre-commit
```

**Validation Includes:**

- Code quality checks (`check-all`)
- Fast test suite (`test-short`)
- Coverage threshold validation (10%+ minimum)
- Documentation formatting (`fmt-docs`)

#### 5. Commit Execution

```bash
# Standard commit with descriptive message
git commit -m "feat: enhance format command with work planning

- Add work planning integration (branch, priority, dependencies)
- Implement environment detection (development/production)
- Add extended output mode with build metadata
- Support JSON output format for automation
- Enhance error handling for processing operations

Coverage: 70%"
```

**Commit Message Standards:**

- **Type:** `feat:`, `fix:`, `docs:`, `test:`, `refactor:`, `perf:`, `chore:`
- **Description:** Clear, actionable summary
- **Body:** Detailed changes and impact
- **Footer:** Issue references, coverage metrics
- **Attribution:** Follow [Agentic Attribution Standard](../standards/agentic-attribution.md) for AI agent contributions

### Emergency Bypass Procedures

#### --no-verify Override

**RESTRICTED OPERATION - Requires Supervisor Approval**

##### Minimum Requirements (Even in Emergency)

```bash
# ALWAYS run formatting - no excuse for unformatted code
make fmt          # Go code formatting
make fmt-docs     # Documentation formatting (best effort)
```

**Rationale:** Formatting is trivial to fix and prevents unnecessary diff noise in emergency commits.

##### Emergency Bypass Execution

```bash
# Emergency bypass (SUPERVISOR APPROVAL REQUIRED)
git commit --no-verify -m "hotfix: critical formatting patch

EMERGENCY BYPASS: Pre-commit checks skipped
Supervisor: @3leapsdave
Ticket: URGENT-001
Reason: Production formatting vulnerability
Formatting: Applied (make fmt + make fmt-docs)

Will address quality gates in follow-up commit"
```

##### In-Development Override

```bash
# Development bypass (SUPERVISOR APPROVAL REQUIRED)
git commit --no-verify -m "wip: partial work planning implementation

IN-DEVELOPMENT BYPASS: Incomplete implementation
Supervisor: @3leapsdave
Ticket: DEV-456
Reason: End-of-day checkpoint, tests incomplete
Formatting: Applied (make fmt + make fmt-docs)

Will complete implementation and tests in next commit"
```

**Authorization Required:**

- **Supervisor/Maintainer approval** via Slack/email
- **Documented justification** in commit message
- **Formatting applied** (`make fmt` + `make fmt-docs`)
- **Follow-up commitment** to address quality gates
- **Post-deployment review** scheduled (emergency) or **next-day review** (development)

**Valid Emergency Scenarios:**

- Critical security vulnerabilities
- Production outages
- Data corruption fixes
- Regulatory compliance deadlines

**Valid In-Development Scenarios:**

- End-of-day work-in-progress checkpoints
- Incomplete feature implementations requiring backup
- Experimental code requiring version control
- Collaborative development handoffs

## Push Operations

### Standard Push Workflow

#### 1. Pre-Push Preparation

##### Version Confirmation

```bash
# Confirm current version
make version-get
cat VERSION

# Update version if needed (semantic versioning)
make version-bump-patch  # or minor/major
git add VERSION
```

##### Pre-Push Quality Validation

```bash
# MANDATORY: Full quality validation BEFORE commit consolidation
make pre-push
```

**Critical:** Run `pre-push` validation BEFORE any commit consolidation operations to minimize rework.

#### 2. Commit Consolidation to Last Pushed Version

**📋 For comprehensive commit consolidation procedures, see: [Git Commit Consolidation SOP](git-commit-consolidation-sop.md)**

##### Quick Reference

```bash
# 1. Create backup (ALWAYS!)
git branch backup/pre-consolidation-$(date +%Y%m%d-%H%M%S)

# 2. Run quality validation BEFORE consolidation
make pre-push

# 3. Soft reset to target commit
git reset --soft <commit-hash>

# 4. Create clean commit
git add -A
git commit -m "feat: comprehensive feature implementation"
```

##### Post-Consolidation Validation

```bash
# Verify consolidation success
git log --oneline -5
git status

# MANDATORY: Final pre-push check after consolidation
make pre-push
```

**⚠️ Important**: Always follow the [Git Commit Consolidation SOP](git-commit-consolidation-sop.md) for complete procedures including backup strategies, commit message standards, and recovery options.

#### 3. Version Tagging (For Release Commits)

For version release commits, create and push annotated tags:

```bash
# Create annotated tag with release notes
git tag -a v0.1.0 -m "v0.1.0: [Brief Release Description]

✨ Features:
- [New features added]

🐛 Fixes:
- [Bug fixes included]

📊 Quality Metrics:
- Coverage: [X.X]% (meets requirements)
- Security: Zero vulnerabilities
- Quality: Zero linting issues

🏗️ Architecture:
- [Any architectural improvements]

Ready for development."

# Verify tag created
git tag -l -n9 v0.1.0
```

#### 4. Push Execution

```bash
# Standard push (set upstream on first push)
git push -u origin main

# Push tag immediately after successful branch push
git push origin v0.1.0

# Force push after rebase (if needed)
git push --force-with-lease origin main
```

**IMPORTANT**: Always push tags immediately after successful branch push to maintain version consistency.

## Git Hooks Integration

### Automated Quality Validation

Goneat includes git hooks for automated quality validation:

#### Pre-Commit Hook

- **Location**: `.git/hooks/pre-commit`
- **Purpose**: Runs `make pre-commit` before allowing commits
- **Validation**: Code quality, fast tests, dynamic coverage, documentation formatting
- **Bypass**: Use `git commit --no-verify` (requires supervisor approval per SOP)

#### Pre-Push Hook

- **Location**: `.git/hooks/pre-push`
- **Purpose**: Runs `make pre-push` before allowing pushes
- **Validation**: Full test suite, security scans, production-ready coverage
- **Bypass**: Use `git push --no-verify` (requires supervisor approval per SOP)

### Hook Setup Verification

```bash
# Verify hooks are executable
ls -la .git/hooks/pre-*

# Test pre-commit hook manually
./.git/hooks/pre-commit

# Test pre-push hook manually
./.git/hooks/pre-push
```

### Single Commit/Push Cycle

For simple changes that don't require consolidation:

```bash
# 1. Pre-push validation FIRST
make pre-push

# 2. Stage and commit
git add .
git commit -m "fix: resolve work planning timeout issue"

# 3. Push immediately
git push origin main
```

## Quality Gate Details

### make check-all (Development)

```bash
# Components:
make fmt-strict  # Code formatting compliance
make vet        # Static analysis
make lint       # Comprehensive linting (0 issues required)
```

**Purpose:** Fast feedback during development cycle **Time:** ~5 seconds **Coverage:** Not enforced

### make pre-commit (Commit Ready)

```bash
# Components:
make check-all                    # Quality checks
make test-short                   # Fast test suite
make coverage-check-pre-commit    # 10% minimum coverage
make fmt-docs                     # Documentation formatting
```

**Purpose:** Commit readiness validation **Time:** ~30 seconds **Coverage:** 10% minimum

### make pre-push (Production Ready)

```bash
# Components:
make check-all         # Quality checks
make test             # Full test suite with race detection
make coverage-check   # 70% minimum coverage
make security-scan    # gosec + govulncheck
```

**Purpose:** Production deployment readiness **Time:** ~2 minutes **Coverage:** 70% minimum

## Troubleshooting

### Common Issues

#### Pre-commit Failures

```bash
# Formatting issues
make fmt
git add .

# Test failures
go test -v ./...
# Fix failing tests, then retry

# Coverage below threshold
# Add tests to increase coverage
# Or document exception with supervisor approval
```

#### Pre-push Failures

```bash
# Security vulnerabilities
make security-scan
# Review and fix vulnerabilities

# Coverage insufficient
make test
# Add comprehensive tests
```

#### Rebase Conflicts

```bash
# Resolve conflicts manually
git status
# Edit conflicted files
git add resolved-file.go
git rebase --continue

# If rebase becomes complex, abort and seek guidance
git rebase --abort
```

## Compliance & Audit

### Required Documentation

- All emergency bypasses logged in commit messages
- Supervisor approvals documented
- Post-deployment reviews scheduled
- Quality gate exceptions justified
- License compliance verified per [License Compliance SOP](license-compliance-sop.md)

### Audit Trail

- Pre-commit/pre-push logs retained
- Coverage reports archived
- Security scan results stored
- Version bump history maintained

## Contacts

**Emergency Approvals:**

- **Lead Maintainer:** @3leapsdave
- **DevOps Lead:** Forge Neat (supervised by @3leapsdave)

**Escalation Path:**

1. Team Lead → Senior Engineer → Engineering Manager
2. Security Issues → Security Team → CISO
3. Production Issues → On-Call → Engineering Manager

---

## Appendix: Make Targets Reference

| Target                    | Purpose                       | Time  | Coverage | Security |
| ------------------------- | ----------------------------- | ----- | -------- | -------- |
| `make help`               | Show available commands       | <1s   | -        | -        |
| `make check-all`          | Development quality checks    | ~5s   | -        | Basic    |
| `make pre-commit`         | Commit validation             | ~30s  | 10%+     | Basic    |
| `make pre-push`           | Push validation               | ~2min | 70%+     | Full     |
| `make test`               | Full test suite with coverage | ~45s  | 70%      | -        |
| `make build`              | Build binary                  | ~5s   | -        | -        |
| `make security-scan`      | Full security analysis        | ~30s  | -        | Full     |
| `make fmt-docs`           | Format documentation          | ~3s   | -        | -        |
| `make version-bump-patch` | Semantic version bump         | <1s   | -        | -        |

## Goneat v0.1.0 First Commit Preparation

### Current Status

- ✅ **70% test coverage** target (bootstrap phase)
- ✅ **Zero security vulnerabilities** (gosec + govulncheck clean)
- ✅ **Zero linting issues** (golangci-lint clean)
- ✅ **Professional documentation** (README, overview, user guides)
- ✅ **Complete build system** with multi-platform support
- ✅ **Work planning and formatting architecture**

### First Commit Checklist

- [x] All quality gates passing (`make check-all`)
- [x] Pre-commit validation successful (`make pre-commit`)
- [x] Documentation formatted (`make fmt-docs`)
- [x] Build system verified (`make build`)
- [x] Version file current (`cat VERSION` → 0.1.0)
- [x] Git repository configured and clean

### First Commit Message Template

```bash
git commit -m "Initial commit: Goneat v0.1.0 - Work planning and formatting tool

- CLI tool for work planning and formatting orchestration
- 70% test coverage with comprehensive testing
- Zero security vulnerabilities verified
- Professional tooling (linting, formatting, security scanning)
- Complete documentation and user guides
- Multi-platform build support
- Quality-first implementation for 'making it neat'

Ready for development with clear path to v0.1.1"
```

**Last Updated:** August 27, 2025 **Next Review:** December 2025
