# Security Documentation

This directory contains security-related documentation for goneat.

## Structure

```
docs/security/
├── README.md              # This file
├── decisions/             # Security Decision Records (SDRs)
│   └── SDR-NNN-title.md   # Individual decisions
└── bulletins/             # Security bulletins (user-facing announcements)
    └── YYYY-MM-SBXXX.md   # Security Bulletin format
```

## Security Decision Records (SDRs)

SDRs document significant security-related decisions, including:

- Vulnerability assessments and remediation strategies
- False positive analysis and justification
- Security architecture decisions
- Dependency security evaluations

**Format:** `SDR-NNN-short-title.md`

**Template:** See `decisions/TEMPLATE.md`

## Security Bulletins

Bulletins are user-facing announcements for security-relevant releases:

- CVE fixes
- Security improvements
- Breaking changes with security implications

**Format:** `YYYY-MM-SBXXX.md` (e.g., `2026-01-SB001.md`)

## Machine-Readable Configuration

Vulnerability allowlists and policy are maintained in `.goneat/dependencies.yaml`:

```yaml
vulnerabilities:
  allow:
    - id: GHSA-xxxx-xxxx-xxxx
      status: false_positive|accepted_risk|mitigated
      reason: "Brief one-line explanation"
      sdr: docs/security/decisions/SDR-NNN-title.md  # Full repo-relative path
      analysis: |
        2-3 line summary so readers can understand the rationale without
        opening the SDR. Include key facts: why it's safe, what was verified.
      verified_by: "@handle"
      verified_date: YYYY-MM-DD
      expires: YYYY-MM-DD  # Optional expiry for accepted risks
```

**Best practices:**
- Use full repo-relative path in `sdr:` for easy navigation
- Include `analysis:` summary for quick understanding without opening the SDR
- Keep `reason:` as a one-liner; use `analysis:` for detail

## Process

1. **New vulnerability found:** Triage severity and exploitability
2. **Remediation or suppression:** Fix if possible, document if not
3. **SDR created:** For non-trivial decisions (false positives, accepted risks)
4. **Config updated:** Add to allowlist with SDR reference
5. **Bulletin published:** If user-facing impact (releases with fixes)

## References

- [SECURITY.md](/SECURITY.md) - Vulnerability reporting policy
- [.goneat/dependencies.yaml](/.goneat/dependencies.yaml) - Vulnerability policy config
