---
title: "[ADR Title - Clear and Descriptive]"
description: "[Brief one-line description of the decision]"
author: "@[github-username]"
date: "YYYY-MM-DD"
last_updated: "YYYY-MM-DD"
status: "proposed|approved|superseded|deprecated|rejected"
supersedes: "[Link to ADR this replaces, if applicable]"
superseded_by: "[Link to ADR that replaces this, if applicable]"
tags:
  - "category1"
  - "category2"
  - "category3"
category: "architecture|policy|security|testing|performance|dependencies|infrastructure"
---

# [ADR XXXX]: [Title]

## Status

**[PROPOSED|APPROVED|SUPERSEDED|DEPRECATED|REJECTED]** - [Brief status note]

## Context

What is the issue we're addressing? What forces are at play? What are the requirements or constraints?

- **Background**: Explain the technical context
- **Problem Statement**: What specific problem are we solving?
- **Goals**: What do we want to achieve?
- **Constraints**: What limitations exist?

## Decision

We will [describe the decision in 1-2 sentences].

### Detailed Description

[Provide a detailed explanation of what was decided. Include diagrams, code examples, or configuration samples as needed.]

```go
// Example code showing the decision
```

## Rationale

Why did we make this decision?

### Key Factors

1. **Factor 1**: Explanation
2. **Factor 2**: Explanation
3. **Factor 3**: Explanation

### Supporting Evidence

- Benchmark results
- Security analysis
- Community best practices
- Team experience

## Alternatives Considered

### Alternative 1: [Name]

**Approach**: [Brief description]

**Pros**:
- Advantage 1
- Advantage 2

**Cons**:
- Disadvantage 1
- Disadvantage 2

**Rejected because**: [Explanation]

### Alternative 2: [Name]

**Approach**: [Brief description]

**Pros**:
- Advantage 1
- Advantage 2

**Cons**:
- Disadvantage 1
- Disadvantage 2

**Rejected because**: [Explanation]

## Consequences

### Positive

- ✅ Benefit 1
- ✅ Benefit 2
- ✅ Benefit 3

### Negative

- ⚠️ Tradeoff 1
  - **Mitigation**: How we address this
- ⚠️ Tradeoff 2
  - **Mitigation**: How we address this

### Neutral

- Changes required in existing code
- Documentation updates needed
- Team training requirements

## Implementation

### Changes Required

1. **Code Changes**:
   - File 1: Description
   - File 2: Description

2. **Configuration Changes**:
   - Config 1: Description

3. **Documentation Updates**:
   - Doc 1: Description

### Migration Path

If this changes existing behavior:

1. Step 1
2. Step 2
3. Step 3

### Testing Strategy

- Unit tests
- Integration tests
- Performance tests
- Validation approach

## Monitoring and Success Criteria

How will we know if this decision was successful?

### Metrics

- Metric 1: Target value
- Metric 2: Target value

### Success Indicators

- ✅ Indicator 1
- ✅ Indicator 2

### Failure Indicators

- ⚠️ What would indicate this isn't working

## Rollback Plan

If this decision needs to be reversed:

1. Rollback step 1
2. Rollback step 2
3. Rollback step 3

**Likelihood of rollback**: [Low/Medium/High]

## References

### Internal Documentation

- [Link to related ADRs]
- [Link to technical specs]
- [Link to implementation files]

### External References

- [Industry standards]
- [Academic papers]
- [Open source projects]
- [Blog posts or articles]

### Code References

- **Files Modified**: List of files
- **PR**: Link to pull request
- **Commit**: Link to commit

## Related Decisions

- **[ADR-XXXX](adr-xxxx-title.md)**: How it relates
- **[ADR-YYYY](adr-yyyy-title.md)**: How it relates

## Timeline

- **Proposed**: YYYY-MM-DD
- **Discussed**: YYYY-MM-DD
- **Approved**: YYYY-MM-DD
- **Implemented**: YYYY-MM-DD
- **Reviewed**: YYYY-MM-DD (if applicable)

---

**Decision made by**: @username
**Approved by**: @username, @username
**Implementation**: [Version/milestone]
**Review date**: [When to review this decision]
