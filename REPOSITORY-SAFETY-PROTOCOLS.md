# 🛡️ REPOSITORY SAFETY PROTOCOLS - MANDATORY COMPLIANCE

## 🚨 CRITICAL WARNING: THESE RULES ARE NON-NEGOTIABLE

**Purpose**: Critical safety rules and protocols for all contributors (human and AI)
**Enforcement**: Zero-tolerance policies to prevent data loss and maintain repository integrity
**Scope**: All development work in goneat (format/lint CLI tool)

---

## 🔥 OPERATIONAL DANGER CLASSIFICATION

### **Level 1: CATASTROPHIC (Never Execute Without User Confirmation)**

- File operations without existence checks (can overwrite existing work)
- Git force operations without `--force-with-lease`
- Bulk file creation or deletion
- History rewriting operations (`rebase`, `reset --hard`)
- Quality gate bypasses (`--no-verify`) outside emergency procedures
- **Format operations on production code** without dry-run validation
- **Test infrastructure modifications** affecting CI/CD pipelines

### **Level 2: HIGH RISK (Validate Before Execution)**

- Committing without pre-commit validation
- Creating new files when Edit would suffice
- Dependency modifications (`go.mod`, package management)
- Configuration changes affecting build pipeline
- Large refactoring operations
- **Format tool modifications** that could break existing workflows
- **Test framework changes** affecting coverage reporting

### **Level 3: MEDIUM RISK (Proceed with Caution)**

- Single file edits with proper validation
- Test additions with coverage verification
- Documentation updates
- Code formatting operations
- **Format rule modifications** for supported file types

---

## 🚨 MANDATORY: POST-COMPACTION RECOVERY PROTOCOL

**CRITICAL**: Context compaction recovery creates high-risk situations where agents may resume work without proper context.

### **MANDATORY RULE OF ENGAGEMENT**

**ALL team members (AI and human) MUST seek explicit authorization before resuming ANY work after:**

- Session restarts or conversation compaction boundaries
- Context recovery situations
- Tool malfunction recovery
- Extended work interruptions

### **Required Post-Compaction Process**

1. **🛑 STOP**: Do NOT begin any work automatically
2. **📖 READ**: Review AGENTS.md and current project state
3. **🔍 ASSESS**: Check recent commits and active work status
4. **🤝 SEEK APPROVAL**: Request explicit authorization from supervisor
5. **✅ CONFIRM SCOPE**: Verify specific tasks and boundaries before proceeding
6. **🚀 PROCEED**: Only after explicit go-ahead received

### **Why This Matters**

- **Context loss**: Compaction can lose critical nuances of active work
- **Direction drift**: Agents may start down incorrect paths
- **Duplicate work**: Multiple agents may tackle same issues
- **Quality degradation**: Lost context leads to lower quality decisions

**Emergency Exception**: Only for Level 1 system failures affecting production

---

## 🚨 CRITICAL: File Operation Rules

**MANDATORY FILE OPERATION PROTOCOL**: These rules are NON-NEGOTIABLE and must be followed for ALL file operations:

### Before Writing ANY File

1. **ALWAYS CHECK IF FILE EXISTS FIRST** — Use `Read` or `Glob` to verify presence; never assume
2. **READ BEFORE WRITE** — If it exists, read to understand structure and context
3. **VERIFY PARENT DIRECTORY** — For new files, confirm the parent directory exists and is correct
4. **MINIMAL-DIFF POLICY** — Prefer surgical edits; avoid wholesale rewrites unless explicitly authorized
5. **PRESERVE EXISTING WORK** — Never overwrite without explicit permission

### File Operation Workflow

```bash
# WRONG - Never do this:
Write file without checking → Overwrites existing work

# CORRECT - Always do this:
1. Check if file exists (Read/Glob)
2. If exists: Read contents, understand structure
3. If modifying: Use Edit tool to preserve existing content
4. If creating new: Confirm with user if similar file exists
```

### Common Violations to Avoid

- **Writing test files without checking**: Test files often already exist with high coverage
- **Creating new files when Edit would suffice**: Prefer editing over recreating
- **Assuming file structure**: Always verify actual structure before modifications
- **Batch file creation**: Check each file individually before writing

### Recovery from Mistakes

If you accidentally overwrite a file:

1. Immediately inform the user
2. Check git status to see if file can be recovered
3. Use `git checkout -- <file>` if uncommitted changes can be reverted
4. Learn from the mistake and always check first in the future

**REMEMBER**: It takes seconds to check if a file exists, but hours to recreate lost work. ALWAYS CHECK FIRST.

---

## 🔒 Git Operations Safety

### No Chained Critical Operations

- Never chain `git add`, `git commit`, and/or `git push` in a single command or script line.
- Run each as a separate step with validation between steps.

Correct:
```bash
git add <files>
make pre-commit
git commit -m "..."
# approval required before push
git push
```

Incorrect:
```bash
git add . && git commit -m "..." && git push
git commit && git push
```

### Use `--no-pager` for Non-Interactive Commands

When using git commands in scripts or automated contexts, always use the `--no-pager` flag to prevent output from opening in less:

```bash
# Correct - prevents less pager
git --no-pager log --oneline -5
git --no-pager diff HEAD~1
git --no-pager show --stat

# Incorrect - may open in less and freeze terminal
git log --oneline -5  # Avoid in scripts/automation
```

### Commit Process Requirements

**MANDATORY PRE-COMMIT WORKFLOW**: All commits must pass quality checks before submission.

```bash
# 1. ALWAYS run make pre-commit first (validates everything)
make pre-commit

# 2. Only commit if pre-commit passes completely
git add <files>
git commit -m "feat: your commit message"

# 3. Exception: Integration checkpoints only
git commit --no-verify -m "checkpoint: work in progress"  # Use sparingly
```

**Why This Matters**:

- `make pre-commit` includes ALL quality gates: formatting, linting, testing, documentation
- Matches git pre-commit hooks exactly (prevents hook failures)
- Catches formatting issues before they cause commit failures
- Ensures consistent code quality across all contributors

### Push Operations Require Explicit Approval

- AI agents MUST obtain explicit human maintainer approval before pushing to any remote branch.
- Force pushes are prohibited unless explicitly authorized; when authorized, use `--force-with-lease`.
- Automated pipelines may push only within designated release workflows with prior, documented approval.
- Document the approval context in the PR or commit description when applicable.

### Safe History Management

- **Clean History**: Follow the [Git Commit Consolidation SOP](docs/sop/git-commit-consolidation-sop.md) for maintaining clean commit history
- **Safety First**: Always create backup branches before rewriting history
- **Commit Messages**: Remove internal task IDs and phase markers from final commits
- **Force Push**: Use `--force-with-lease` instead of `--force` to prevent overwriting others' work

### 🛡️ Pre-Push Validation Before Consolidation

**CRITICAL**: Always run `make pre-push` BEFORE any commit consolidation operations to minimize rework.

```bash
# MANDATORY: Run pre-push validation BEFORE consolidation
make pre-push

# Only proceed with consolidation if pre-push passes
# If issues found, fix them before consolidation
```

**Why This Matters:**

- Prevents discovering quality gate failures after rewriting history
- Minimizes backup branch restoration complexity
- Ensures consolidated commits meet production readiness standards
- Maintains professional development workflow integrity

---

## ⚖️ Quality Gate Enforcement

### Pre-Commit Requirements (Non-Negotiable)

All of these must pass before ANY commit:

- ✅ **Code formatting**: `gofmt` and `goimports` compliance
- ✅ **Document formatting**: YAML, JSON, Markdown standards
- ✅ **Dependency management**: Clean `go.mod` and `go.sum`
- ✅ **Linting**: `golangci-lint` comprehensive rules
- ✅ **Static analysis**: `go vet` checks
- ✅ **Test coverage**: Minimum 80% overall, 75% per package
- ✅ **Build success**: Code compiles without errors
- ✅ **Format tool validation**: `goneat format` works on its own code

### Quality Commands Reference

```bash
# Complete quality validation (required before commit)
make pre-commit

# Individual quality checks (for troubleshooting)
make fmt                    # Format code and documents
make lint                   # Run comprehensive linting
make test-coverage-check    # Run tests with coverage validation
make vet                    # Static analysis
make tidy-check            # Dependency validation
```

### Coverage Requirements

**Lifecycle-Driven Coverage Standards** (See [Coverage Governance SOP](docs/sop/lifecycle-driven-coverage-governance.md)):

- **Current Phase**: Alpha (50% minimum - defined in `LIFECYCLE_PHASE` file)
- **Stage-Specific**: Pre-commit (50%), Pre-push (50%), CI/CD (55%)
- **Progressive**: Coverage requirements increase with lifecycle maturity
- **Emergency Override**: Available with proper authorization and expiration
- **Package-Specific**: Overrides supported with documented rationale

**Legacy Standards** (for reference):

- ~~Minimum: 80% overall coverage~~ (Now lifecycle-driven)
- ~~Per-package: 75% minimum~~ (Now stage-specific)
- **New code**: Should not decrease overall coverage (still applies)
- **Critical packages**: Enhanced thresholds via package overrides

---

## 🛡️ Format Tool Safety Protocols

### Dogfooding Requirements

**MANDATORY DOGFOODING**: The format tool must work on its own codebase.

```bash
# MANDATORY: Test format tool on itself before committing
./dist/goneat format cmd/ pkg/ main.go tests/

# Verify no errors and proper formatting
make fmt  # Should use goneat internally
```

**Why This Matters**:

- Ensures the format tool works correctly on real codebases
- Catches format tool bugs before they affect users
- Maintains consistency between tool and codebase
- Validates format rules work as intended

### Format Operation Safety

**MANDATORY FORMAT WORKFLOW**: All format operations must follow safety protocols.

```bash
# 1. ALWAYS dry-run first for new or complex operations
./dist/goneat format --dry-run --folders src/

# 2. Review changes before applying
./dist/goneat format --plan-only --folders src/

# 3. Apply changes only after validation
./dist/goneat format --folders src/

# 4. Verify no breaking changes
make test  # Ensure tests still pass
```

**High-Risk Format Operations** (require explicit approval):

- Formatting entire monorepos without dry-run validation
- Applying new format rules to established codebases
- Format operations affecting CI/CD pipelines
- Bulk format operations on unfamiliar codebases

### Format Tool Development Safety

**MANDATORY TESTING**: Format tool changes must be thoroughly tested.

```bash
# Test format tool changes comprehensively
make test                    # Unit tests
make integration-test       # Integration tests
make format-test           # Format-specific tests

# Dogfood the changes
./dist/goneat format --dry-run cmd/ pkg/
```

---

## 🛡️ Security Protocols

### Files to Never Commit

- Coverage reports (`*_coverage.html`, `cover.out`)
- Binary artifacts (unless specifically required)
- Local environment files (`.env` with secrets)
- IDE-specific configurations (beyond agreed team standards)
- Temporary or cache files
- **Format tool artifacts** that could interfere with user workflows

### Secret Management

- **No hardcoded secrets**: All secrets must be properly managed
- **API Key Protection**: Use environment variables and mask in logs
- **Configuration Security**: Secure configuration hierarchy
- **Dependency Scanning**: Regular security updates and vulnerability checks

### Pre-commit Security Validation

The pre-commit hooks automatically run:

- `detect-secrets` security vulnerability scanning
- Dependency vulnerability checks
- Large file detection (>500KB)
- Merge conflict detection
- **Format tool integrity checks**

---

## 🚨 Emergency Procedures

### Broken Build Recovery

1. Check recent commits: `git --no-pager log --oneline -5`
2. Run tests locally: `make test`
3. Check CI logs for specific failures
4. Rollback if needed: Create fix branch from last good commit

### Lost Work Recovery

1. Check reflog: `git --no-pager reflog -20`
2. Look for backup branches: `git branch -a | grep backup`
3. Check stash: `git stash list`
4. Recover from reflog: `git checkout <reflog-sha>`

### Hook Bypass (Emergency Only)

When hooks fail but code is correct:

1. **Verify manually**: `make pre-commit`
2. **Check specific issue**: `make lint` or `make test`
3. **Bypass if needed**: `git commit --no-verify`
4. **Document in commit**: Explain why bypass was needed

---

## 📋 Safety Checklist

### Before Starting Any Work

- [ ] Read current AGENTS.md for role and responsibilities
- [ ] Check MAINTAINERS.md for supervision structure
- [ ] Review `LIFECYCLE_PHASE` file to understand current coverage requirements
- [ ] Scan `.plans/active/` for current work
- [ ] Review recent commits: `git --no-pager log --oneline -10`

### Before Committing Any Changes

- [ ] Existence checked and file read before any write/edit
- [ ] Parent directory verified for any new files
- [ ] Minimal-diff policy followed (no wholesale rewrites without approval)
- [ ] Pre-commit quality gates passed: `make pre-commit`
- [ ] Test coverage maintained or improved
- [ ] No secrets or sensitive data included
- [ ] Proper commit attribution format used
- [ ] **Format tool tested on its own codebase**

### Before Pushing to Remote

- [ ] Pre-push tests passed: `make test-coverage-check-full`
- [ ] Integration tests verified
- [ ] No force-push without `--force-with-lease`
- [ ] Team notified of significant changes
- [ ] **Format tool dogfooding validated**
 - [ ] Explicit human approval to push was obtained

---

## 🎯 Success Metrics

### Individual Developer Success

- [ ] **Consistency**: >95% of commits pass pre-commit on first try
- [ ] **Safety**: Zero accidental file overwrites or data loss
- [ ] **Quality**: Maintain or improve coverage with each commit
- [ ] **Security**: No committed secrets or vulnerabilities
- [ ] **Dogfooding**: Format tool works perfectly on its own codebase

### Team Success

- [ ] **CI stability**: <5% of builds fail due to preventable issues
- [ ] **Recovery time**: <1 hour average for any broken build recovery
- [ ] **Knowledge retention**: Safety protocols followed consistently
- [ ] **Zero data loss**: No work lost due to preventable file operations
- [ ] **Format reliability**: Tool works consistently across all supported formats

---

## 📚 Related Documentation

- **[AGENTS.md](AGENTS.md)** - Complete AI agent standards and operational guidelines
- **[MAINTAINERS.md](MAINTAINERS.md)** - Human and AI maintainer responsibilities
- **[Lifecycle-Driven Coverage Governance SOP](docs/sop/lifecycle-driven-coverage-governance.md)** - Process-based coverage management
- **[Coverage Threshold Resolution Algorithm](docs/standards/coverage-threshold-resolution-algorithm.md)** - Technical threshold specification
- **[Git Commit Consolidation SOP](docs/sop/git-commit-consolidation-sop.md)** - Safe history management
- **[Pre-Commit Quality Workflow SOP](docs/development/sop/pre-commit-quality-workflow.md)** - Daily quality workflow
- **[Test Categorization SOP](docs/development/sop/test-categorization-decision-process.md)** - Test safety and organization

---

---

## 📋 MANDATORY COMPLIANCE ACKNOWLEDGMENT

**All team members and AI agents must explicitly confirm:**

✅ I have read and understand the Repository Safety Protocols
✅ I will follow file operation protocols and ALWAYS check existence before writing
✅ I will seek explicit authorization before resuming work after context compaction
✅ I will never bypass quality gates without emergency justification
✅ I will follow the post-compaction recovery process without exception
✅ I will validate all operations and request confirmation for high-risk changes
✅ I understand that safety protocols are mandatory and non-negotiable
✅ I will test format tool changes on the tool's own codebase (dogfooding)
✅ I will always dry-run format operations before applying to user codebases

**Digital Signature**: **\*\***\_\_\_\_**\*\*** **Date**: **\*\***\_\_\_\_**\*\***

---

## 🛡️ REPOSITORY SAFETY = TEAM PRODUCTIVITY

**Remember**: Every safety protocol violation can either:

- 🚀 **Be caught by automated systems and quickly resolved**
- 💥 **Cause data loss, broken builds, and hours of recovery work**

**The difference is FOLLOWING THE PROTOCOLS.**

### **Latest Safety Learnings:**

- **Context Compaction**: Always seek authorization before resuming work
- **File Operations**: Check existence before ANY write operation
- **Quality Gates**: Never bypass without explicit emergency justification
- **Format Tool Safety**: Always dogfood and dry-run before applying
- **Git Safety**: Use `--no-pager` and `--force-with-lease` patterns

---

**⚡ Remember**: These protocols exist to prevent 70% of common mistakes that cause data loss, broken builds, and wasted time. Following them is the fastest path to productivity, not an obstacle to it.

**Maintained by**: Repository Safety Committee
**Last Updated**: 2025-08-28
**Next Review**: Quarterly</content>
</xai:function_call name="bash">
<parameter name="command">cd goneat && make build
