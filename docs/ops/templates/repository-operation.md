# Repository Operation: [Brief Description]

**Operation ID**: `YYYY-MM-DD-repository-[brief-description]`
**Date**: YYYY-MM-DD
**Time**: HH:MM:SS TZ
**Operator**: [Agent/Person Name]
**Supervisor**: [Supervisor Handle]
**Category**: Repository Maintenance
**Risk Level**: [Low/Medium/High/Critical]

## Operation Summary

[Brief description of what operation was performed and why]

### Rationale

- **Primary Reason**: [Main driver for the operation]
- **Business Impact**: [Effect on users/project]
- **Timing**: [Why now, urgency factors]
- **Alternatives Considered**: [Other options evaluated]

## Pre-Operation State

### Repository Condition

- **Branch**: [Current branch]
- **Remote**: [Remote repository URL]
- **Commit Count**: [Number of commits]
- **Working Directory**: [Clean/Modified/Staged changes]
- **Tags**: [Relevant tags if applicable]

### Baseline Measurements

```
[Key metrics before operation]
- File count:
- Repository size:
- Branch count:
- Other relevant metrics:
```

### Issues Identified

- **Issue 1**: [Description and impact]
- **Issue 2**: [Description and impact]
- **Risk Factors**: [Potential complications]

### Backup Verification

- **Backup Method**: [How backup was created]
- **Backup Location**: [Where backup is stored]
- **Backup Verification**: [How backup was tested]
- **Recovery Time**: [Estimated time to restore]

## Operation Steps

### Phase 1: Preparation (HH:MM-HH:MM)

1. **Step Description**

   ```bash
   # Commands executed
   command --with-flags
   ```

   - **Result**: [Outcome of step]
   - **Verification**: [How success was confirmed]

2. **Additional Steps**
   - [Detailed step-by-step process]

### Phase 2: Execution (HH:MM-HH:MM)

[Continue with detailed phases as needed]

### Phase 3: Verification (HH:MM-HH:MM)

[Post-execution verification steps]

## Post-Operation Verification

### ✅ Success Criteria

- [ ] [Criterion 1 - specific, measurable outcome]
- [ ] [Criterion 2 - specific, measurable outcome]
- [ ] [Criterion 3 - specific, measurable outcome]

### Validation Tests

```bash
# Commands used to verify success
git fsck --full
git status
[other verification commands]
```

### Measurements Comparison

```
Before → After
Metric 1: [before] → [after]
Metric 2: [before] → [after]
```

## Impact Assessment

### ✅ Positive Impacts

- **Impact 1**: [Description of benefit]
- **Impact 2**: [Description of benefit]

### ⚠️ Breaking Changes

- **Change 1**: [What changed and potential impact]
- **Migration Required**: [If users need to take action]

### 🔄 Side Effects

- **Effect 1**: [Unintended but harmless consequences]
- **Monitoring Required**: [What to watch for]

## Risk Assessment

### 🔴 High Risks (Mitigated)

1. **Risk Name**: Description
   - **Mitigation**: How risk was addressed
   - **Detection**: How to identify if risk materializes

### 🟡 Medium Risks (Monitored)

1. **Risk Name**: Description
   - **Monitoring**: What to watch for
   - **Response Plan**: What to do if detected

### 🟢 Low Risks (Accepted)

1. **Risk Name**: Description
   - **Justification**: Why risk is acceptable

## Rollback Procedure

### When to Rollback

- **Trigger Conditions**: [Specific failure scenarios]
- **Decision Authority**: [Who can authorize rollback]
- **Time Limit**: [How long rollback remains viable]

### Rollback Steps

1. **Immediate Actions**

   ```bash
   # Emergency rollback commands
   ```

2. **Complete Restoration**

   ```bash
   # Full rollback procedure
   ```

3. **Verification**
   - [How to confirm rollback success]

### Recovery Time Objective

- **RTO**: [Maximum acceptable downtime]
- **RPO**: [Maximum acceptable data loss]

## Lessons Learned

### ✅ Successful Practices

- **Practice 1**: [What worked well and why]
- **Practice 2**: [What worked well and why]

### 🔧 Process Improvements

- **Improvement 1**: [What could be done better]
- **Improvement 2**: [What could be done better]

### 📚 Knowledge Gained

- **Learning 1**: [New understanding or technique]
- **Documentation Updates**: [What docs need updating]

## Related Operations

### Dependencies

- **Prerequisite Operations**: [Operations that had to complete first]
- **Concurrent Operations**: [Operations running simultaneously]

### Follow-up Required

- **Immediate**: [Actions needed within 24 hours]
- **Short-term**: [Actions needed within 1 week]
- **Long-term**: [Actions needed within 1 month]

## Stakeholder Communication

### Internal Team

- **Notification Method**: [How team was informed]
- **Timing**: [When notifications were sent]
- **Feedback Received**: [Team responses or concerns]

### External Impact

- **User Communication**: [How users were notified]
- **Service Status**: [Any status page updates]
- **Documentation Updates**: [Public docs changed]

### Communication Timeline

- **Pre-operation**: [Advance notice given]
- **During operation**: [Real-time updates]
- **Post-operation**: [Completion notification]

---

**Operation Status**: [🔄 IN PROGRESS / ✅ COMPLETED SUCCESSFULLY / ❌ FAILED / 🔁 ROLLED BACK]
**Completion Time**: YYYY-MM-DD HH:MM:SS TZ
**Total Duration**: [Duration]
**Next Review**: [When to review this operation]

**Generated by**: [Operator Name]
**Supervised by**: [Supervisor]
**Documentation Standard**: Repository Operations Template v1.0.0

---

## Template Usage Notes

### Required Fields

- All fields marked with brackets `[]` must be filled
- Dates should use ISO 8601 format (YYYY-MM-DD)
- Times should include timezone
- Risk levels: Low/Medium/High/Critical

### Optional Sections

- Remove sections that don't apply
- Add custom sections as needed for specific operations
- Maintain consistent formatting

### Best Practices

- Document as you execute, not after
- Include exact commands and outputs
- Be specific about verification steps
- Link to related documentation
