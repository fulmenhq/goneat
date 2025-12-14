# Architecture Decision Records (ADRs)

This directory contains Architecture Decision Records (ADRs) for significant technical decisions in the goneat project.

## What is an ADR?

An Architecture Decision Record (ADR) is a document that captures an important architectural decision made along with its context and consequences. ADRs help teams:

- Document the reasoning behind architectural choices
- Provide historical context for future maintainers
- Enable informed discussion about changes
- Create a searchable decision log

## Naming Convention

ADRs follow this naming pattern:

```
adr-XXXX-title-as-slug.md
```

Where:

- `XXXX` is a sequential 4-digit number (0001, 0002, etc.)
- `title-as-slug` is a brief, hyphenated description
- `.md` is the markdown file extension

**Examples:**

- `adr-0001-opa-v1-rego-v1-migration.md`
- `adr-0002-mockable-http-transport.md`
- `adr-0003-embedded-vs-external-policy-engine.md`

## ADR Template

See [`adr-template.md`](adr-template.md) for the standard template.

## Active ADRs

| Number                                                         | Title                                             | Status   | Date       | Tags                                   |
| -------------------------------------------------------------- | ------------------------------------------------- | -------- | ---------- | -------------------------------------- |
| [0001](adr-0001-opa-v1-rego-v1-migration.md)                   | OPA v1 and Rego v1 Syntax Migration               | Approved | 2025-10-10 | policy, opa, rego, dependencies        |
| [0002](adr-0002-ssot-dirty-detection.md)                       | SSOT Dirty Detection: Repo vs Global Gitignore    | Approved | 2025-10-28 | architecture, ssot, git, provenance    |
| [0003](adr-0003-linux-release-artifacts-cgo-disabled-for-musl-compat.md) | Linux Release Artifacts: CGO Disabled for musl/glibc Compatibility | Approved | 2025-12-14 | infrastructure, release, linux, musl, glibc, cgo |

## ADR Statuses

- **Proposed**: Under discussion, not yet decided
- **Approved**: Decision made and accepted
- **Superseded**: Replaced by a newer decision (link to new ADR)
- **Deprecated**: No longer recommended but not replaced
- **Rejected**: Considered but not adopted

## Creating a New ADR

1. **Copy the template:**

   ```bash
   cp docs/architecture/decisions/adr-template.md \
      docs/architecture/decisions/adr-XXXX-your-title.md
   ```

2. **Assign the next sequential number** (check this README for current max)

3. **Fill in the template sections:**
   - Context: What forces are at play?
   - Decision: What did we decide?
   - Rationale: Why this decision?
   - Consequences: What are the impacts?
   - Alternatives: What else did we consider?

4. **Update this README** to add your ADR to the Active ADRs table

5. **Submit for review** via pull request

## ADR Lifecycle

```
Proposed → Approved → [Implemented]
   ↓
Rejected

Approved → Superseded (link to new ADR)
        → Deprecated
```

## Categories

ADRs are tagged by category for easy filtering:

- **architecture**: System design and structure
- **policy**: Policy engine and rule evaluation
- **security**: Security-related decisions
- **testing**: Testing strategy and tooling
- **performance**: Performance optimizations
- **dependencies**: Dependency management
- **infrastructure**: Deployment and operations

## Best Practices

### When to Write an ADR

Write an ADR when:

- Making a significant architectural choice
- Choosing between multiple viable alternatives
- Making a decision that's hard to reverse
- Solving a problem that may recur
- Making a decision that affects multiple teams/components

### When NOT to Write an ADR

Don't write an ADR for:

- Routine implementation details
- Obvious choices with no alternatives
- Temporary or experimental decisions
- Decisions easily changed later

### ADR Writing Tips

1. **Be specific**: Provide concrete examples and code snippets
2. **Show alternatives**: Explain what you considered and why you rejected it
3. **Be honest about tradeoffs**: No decision is perfect
4. **Link to related docs**: Reference implementations, specs, and other ADRs
5. **Keep it concise**: 2-3 pages is ideal
6. **Update when superseded**: Link to new ADRs that replace old ones

## References

- [Architecture Decision Records](https://adr.github.io/) - ADR methodology
- [Documenting Architecture Decisions](https://cognitect.com/blog/2011/11/15/documenting-architecture-decisions) - Original article by Michael Nygard
- [ADR Tools](https://github.com/npryce/adr-tools) - Command-line tools for ADRs

## Related Documentation

- [Architecture Overview](../README.md) - High-level architecture documentation
- [Technical Design Docs](../../technical/) - Detailed technical specifications
- [API Documentation](../../api/) - API contracts and interfaces

---

**Last Updated**: 2025-12-14
**ADR Count**: 3
**Next ADR Number**: 0004
