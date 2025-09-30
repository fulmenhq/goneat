# Session Initialization Protocol

**MANDATORY**: All AI agents MUST follow this protocol at the start of every work session.

## Protocol Steps

### 1. Identity Confirmation

- [ ] Confirm agentic identity (Forge Neat, Code Scout, Arch Eagle)
- [ ] Verify supervision assignment (@3leapsdave)
- [ ] Check current session context for any active work

### 2. Documentation Review

- [ ] Read [AGENTS.md](AGENTS.md) - Complete agent guidelines
- [ ] Read [MAINTAINERS.md](MAINTAINERS.md) - Supervision and responsibilities
- [ ] Read [REPOSITORY_SAFETY_PROTOCOLS.md](REPOSITORY_SAFETY_PROTOCOLS.md) - Critical safety rules
- [ ] Read [docs/standards/agentic-attribution.md](docs/standards/agentic-attribution.md) - Attribution standards
- [ ] Read [docs/standards/go-coding-standards.md](docs/standards/go-coding-standards.md) - Coding standards

### 3. Context Assessment

- [ ] Review recent commits: `git --no-pager log --oneline -5`
- [ ] Check active work in `.plans/active/`
- [ ] Assess current repository state
- [ ] Identify any ongoing work or blockers

### 4. Safety Acknowledgment

- [ ] Acknowledge push authorization requirements
- [ ] Confirm understanding of file operation protocols
- [ ] Verify quality gate requirements
- [ ] Document any context compaction recovery needs

### 5. Work Authorization

- [ ] Request explicit work scope approval from supervisor
- [ ] Document approved tasks and boundaries
- [ ] Confirm no unauthorized autonomous actions

## Session Documentation

**Session Start**: 2025-09-19 07:45
**Agent**: [Forge Neat/Code Scout/Arch Eagle]
**Supervisor**: [@3leapsdave]
**Approved Scope**: [Brief description of authorized work]

**Documentation Review Completed**: [Date/Time]
**Safety Protocols Acknowledged**: [Date/Time]
**Work Authorization Obtained**: [Date/Time]

---

**MANDATORY**: No work may begin until all steps are completed and documented.

**Violation**: Any work started without following this protocol may be considered unauthorized.
