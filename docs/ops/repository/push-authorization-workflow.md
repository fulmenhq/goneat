# Push Authorization Workflow

**MANDATORY**: All push operations require explicit human maintainer approval.

> 🛡️ **Guardian policy**: Protected branches require guardian approval (`goneat guardian approve git push -- git push ...`) before the push executes. Remote enforcement (pre-receive/CI) will land in v0.2.9 and reject pushes that skip guardian.

## Workflow Overview

```
Agent Work → Pre-Push Validation → Guardian Approval → Human Review → Authorized Push
```

## Step-by-Step Process

### Step 1: Agent Pre-Push Validation

**Agent Responsibility**: Complete all validations before requesting approval.

```bash
# Quality Gates (MANDATORY)
make pre-commit          # All quality checks pass
make pre-push           # Additional validation
make test-coverage-check # Coverage requirements met

# Content Validation
git --no-pager log --oneline -5  # Review recent commits
# Verify attribution standards followed
# Check for sensitive data
```

### Step 2: Approval Request

**Agent Responsibility**: Submit formal approval request.

**Request Format**:

```
🚨 PUSH APPROVAL REQUEST

Agent: [Forge Neat]
Branch: [feature-branch]
Target: [origin/main]
Changes: [Brief summary of what will be pushed]

Pre-Push Validation Complete:
✅ make pre-commit passed
✅ make pre-push passed
✅ Coverage requirements met
✅ Attribution standards followed
✅ No sensitive data detected

Requesting approval from @3leapsdave to execute push.
```

### Step 2.5: Launch Guardian Approval

Before the supervisor confirms the push, start the guardian approval session so the final command runs under guardian control:

```bash
goneat guardian approve git push -- git push origin main
```

- Complete the browser approval (expires in ~15 minutes).
- Document the approval timestamp/URL in the push request for audit purposes.
- If the approval expires before the push executes, rerun the command to obtain a new grant.

### Step 3: Human Review

**Supervisor Responsibility**: Review and approve/reject request.

**Review Checklist**:

- [ ] Changes align with approved work scope
- [ ] Quality gates actually passed (verify if needed)
- [ ] No unauthorized commits or changes
- [ ] Attribution and documentation correct
- [ ] Push serves legitimate development purpose

**Approval Format**:

```
✅ PUSH APPROVED

Approved by: @3leapsdave
Date: 2025-09-19 07:45
Rationale: [Brief approval reason]
Conditions: [Any special conditions, e.g., "Use --force-with-lease"]
```

**Rejection Format**:

```
❌ PUSH DENIED

Reason: [Specific reason for denial]
Required Actions: [What agent must do before re-requesting]
```

### Step 4: Authorized Push

**Agent Responsibility**: Execute push only after approval.

```bash
# (Optional) Document approval in commit message
git commit --amend -m "feat: [description]

[Original message]

Guardian approval: [token/id] granted on [date]"

# Execute push under guardian supervision (rerun if approval expired)
goneat guardian approve git push -- git push origin [branch]

# Document completion
# Update push checklist with results
```

## Emergency Procedures

### Level 1 Emergency (System Down)

- **Trigger**: Production system failure requiring immediate fix
- **Process**: Supervisor can grant blanket approval for emergency pushes
- **Documentation**: Must be logged in incident report
- **Guardian Reminder**: Attempt guardian approval first. If `--no-verify` is unavoidable, document the bypass and notify @3leapsdave; upcoming remote enforcement (v0.2.9) will reject unapproved pushes automatically.

### Level 2 Emergency (Build Breaking)

- **Trigger**: CI/CD pipeline broken, blocking all development
- **Process**: Require supervisor approval but allow faster review
- **Documentation**: Must be justified in post-mortem
- **Guardian Reminder**: Preferred path is `goneat guardian approve`; log any bypass with the reason so the security audit trail stays intact.

## Audit Trail

All push operations must maintain audit trail:

- Pre-push checklist completion
- Approval request documentation
- Supervisor approval record
- Push execution confirmation
- Post-push validation results

## Violation Consequences

- **First Violation**: Warning and mandatory retraining
- **Second Violation**: Temporary suspension of push privileges
- **Third Violation**: Permanent revocation of autonomous push capability

---

**MANDATORY**: This workflow ensures safety while enabling efficient development. All agents must follow it without exception.
