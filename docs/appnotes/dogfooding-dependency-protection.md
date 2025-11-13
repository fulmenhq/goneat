# Dogfooding Dependency Protection in Goneat

**Application Note**: How goneat itself uses its dependency protection features as a living reference implementation.

**Last Updated**: 2025-10-28  
**Version**: v0.3.0  
**Status**: Production

---

## Overview

This document shows exactly how the goneat project configures and uses its own dependency protection features (license compliance, package cooling, SBOM generation). Use this as a concrete reference when setting up your own projects.

**Why this document exists**: Abstract documentation is useful, but seeing real-world configuration with actual file paths, specific license choices, and operational patterns is more valuable for implementation.

---

## Configuration Files

goneat uses a two-level cooling policy configuration:

1. **Global Policy** (`.goneat/dependencies.yaml`) - Applies to all tools and dependencies
2. **Tool-Specific Overrides** (`.goneat/tools.yaml`) - Per-tool customization

### Primary Configuration: `.goneat/dependencies.yaml`

**Location**: `./.goneat/dependencies.yaml` (repository root)

```yaml
version: v1

# License Compliance Policy
licenses:
  forbidden:
    - GPL-3.0 # Strong copyleft - would require goneat to be GPL-3.0
    - AGPL-3.0 # Network copyleft - even stricter than GPL-3.0
    - MPL-2.0 # Mozilla Public License - weak copyleft with patent concerns

  # Explicitly allow only safe, permissive licenses
  allowed:
    - MIT
    - Apache-2.0
    - BSD-3-Clause
    - BSD-2-Clause
    - ISC
    - 0BSD
    - Unlicense

# Package Cooling Policy (Supply Chain Security)
cooling:
  enabled: true
  min_age_days: 7 # Packages must be ≥7 days old
  min_downloads: 100 # Must have ≥100 total downloads
  min_downloads_recent: 10 # Must have ≥10 recent downloads
  alert_only: false # FAIL build on violations (strict mode)
  grace_period_days: 3 # 3-day grace for initial publication

  # Trust our own organization's packages
  exceptions:
    - pattern: "github.com/fulmenhq/*"
      reason: "Internal FulmenHQ packages are pre-vetted by maintainers"
      approved_by: "@3leapsdave"
      approved_date: "2025-10-28"

# Policy Engine Configuration
policy_engine:
  type: embedded # Use embedded OPA engine (fast, offline)
```

**Key Decisions**:

1. **Forbidden Licenses**: We forbid GPL-3.0, AGPL-3.0, and MPL-2.0 because:
   - GPL/AGPL would force goneat to be GPL (copyleft contamination)
   - MPL has weak copyleft that complicates distribution
   - Patent grant clauses in MPL are ambiguous for our use case

2. **Allowlist Approach**: We use an explicit allowlist (stricter than just forbidding) to ensure only battle-tested, permissive licenses are used

3. **Cooling Thresholds**: 7 days minimum age balances security with development velocity:
   - 80% of supply chain attacks are detected within 7 days
   - Conservative teams should use 14 days
   - Critical infrastructure should use 30 days

4. **FulmenHQ Exception**: We trust our own organization's packages because:
   - Pre-vetted by maintainers during code review
   - Hosted on private/controlled infrastructure
   - Subject to internal security policies

### Tool-Specific Overrides: `.goneat/tools.yaml`

**Location**: `./.goneat/tools.yaml` (repository root)

**NEW in v0.3.6**: goneat now supports tool-specific cooling policy overrides.

#### Real-World Example: Stricter Policy for Syft

```yaml
tools:
  syft:
    name: "syft"
    description: "SBOM generation tool for software supply chain security"
    kind: "system"
    detect_command: "syft version"
    # ... platform configs, artifacts ...

    # Tool-specific cooling override for critical SBOM tool
    cooling:
      min_age_days: 14        # More conservative than global 7 days
      min_downloads: 5000     # Higher threshold than global 100
      min_downloads_recent: 100  # Ensure active maintenance
```

**Rationale**: Syft generates SBOMs that document our entire supply chain. A compromised SBOM tool could:
- Inject malicious packages into SBOMs
- Hide vulnerable dependencies
- Provide false security assurances

Therefore, we apply a **stricter cooling policy** (14 days vs 7 days global) to ensure Syft releases have extra time for community vetting.

**Other tools** (ripgrep, jq, golangci-lint, etc.) still use the global 7-day policy since they have lower supply chain risk.

#### Configuration Hierarchy in Action

| Tool | Cooling Policy Source | Min Age | Min Downloads |
|------|----------------------|---------|---------------|
| syft | Tool-specific override | 14 days | 5000 |
| ripgrep | Global default | 7 days | 100 |
| jq | Global default | 7 days | 100 |
| golangci-lint | Global default | 7 days | 100 |
| (with --no-cooling) | CLI flag | Disabled | N/A |

**Key Insight**: This demonstrates goneat's "defense in depth" approach - different security postures for different risk profiles.

---

## Git Hooks Integration

### Hook Configuration: `.goneat/hooks.yaml`

**Location**: `./.goneat/hooks.yaml` (repository root)

```yaml
version: "1.0.0"
hooks:
  pre-commit:
    - command: "make"
      args: ["format-all"]
      priority: 5
      timeout: "60s"

    # Fast license-only check (offline, <1 second)
    - command: "dependencies"
      args: ["--licenses", "--fail-on", "high"]
      priority: 8
      timeout: "30s"

    - command: "assess"
      args: ["--categories", "format,lint,dates,tools", "--fail-on", "high"]
      priority: 10
      timeout: "90s"

  pre-push:
    - command: "make"
      args: ["format-all"]
      priority: 5
      timeout: "60s"

    # Comprehensive check: licenses + cooling (online, 3-5 seconds)
    - command: "dependencies"
      args: ["--licenses", "--cooling", "--fail-on", "high"]
      priority: 7
      timeout: "45s"

    - command: "make"
      args: ["verify-embeds"]
      priority: 8
      timeout: "30s"

    - command: "assess"
      args:
        [
          "--categories",
          "format,lint,security,dependencies,dates,tools,maturity,repo-status",
          "--fail-on",
          "high",
        ]
      priority: 10
      timeout: "2m"
```

**Key Decisions**:

1. **Pre-Commit Hook** (fast, offline):
   - ✅ License check only (`--licenses`)
   - ❌ NO cooling check (too slow for commit workflow)
   - Typical execution: <1 second
   - Network: NOT required

2. **Pre-Push Hook** (comprehensive, online):
   - ✅ License + cooling check (`--licenses --cooling`)
   - ✅ Full security scan
   - Typical execution: 3-5 seconds
   - Network: REQUIRED (queries npm, PyPI, etc.)

3. **Why This Split?**:
   - Developers commit frequently (5-20x per day)
   - Developers push infrequently (1-3x per day)
   - Fast commits = good developer experience
   - Comprehensive push = security without friction

---

## Operational Patterns

### Daily Development Workflow

**Developer makes a commit**:

```bash
$ git commit -m "feat: add new feature"
# Pre-commit hook runs automatically:
# ✅ Format check (instant)
# ✅ License check (instant)  <-- License policy enforced here
# ✅ Lint check (instant)
# Total: <2 seconds
```

**Developer pushes changes**:

```bash
$ git push
# Pre-push hook runs automatically:
# ✅ Format check (instant)
# ✅ License + cooling check (3-5s)  <-- Full dependency gate
# ✅ Security scan (5-10s)
# ✅ Comprehensive assessment (10-15s)
# Total: 20-30 seconds
```

### Adding a New Dependency

**Scenario**: Add a new Go module

```bash
# 1. Add dependency
$ go get github.com/example/new-package

# 2. Try to commit (pre-commit license check)
$ git commit -m "deps: add new-package"
# ✅ License check passes (MIT license is allowed)

# 3. Try to push (pre-push includes cooling check)
$ git push
# ⚠️  BLOCKED: Package too new (published 2 days ago, need 7 days)
# Exit code: 1 (build fails)

# 4. Options:
#    a) Wait 5 more days (recommended)
#    b) Add temporary exception to .goneat/dependencies.yaml
#    c) Ask tech lead for approval to bypass

# If approved exception added:
$ git push
# ✅ Passes with documented exception
```

### Pre-Release Validation

**Before pushing v0.3.0 tag**:

```bash
# 1. Full dependency audit (licenses + cooling)
$ ./dist/goneat dependencies --licenses --cooling --fail-on high
Dependencies: 93
Passed: true
License violations: 0
Cooling violations: 0
✅ CLEAN BILL OF HEALTH

# 2. Generate SBOM for release artifacts
$ ./dist/goneat dependencies --sbom --sbom-output sbom/goneat-v0.3.0.cdx.json
Packages cataloged: 723
Format: CycloneDX 1.6 JSON
✅ SBOM generated: sbom/goneat-v0.3.0.cdx.json

# 3. Full security assessment
$ ./dist/goneat assess --categories format,lint,security,dependencies --fail-on high
Overall Health: 🟢 Excellent (100%)
Critical Issues: 0
Total Issues: 0
✅ READY FOR RELEASE

# 4. Push with confidence
$ git push origin v0.3.0
```

---

## Current Dependency Health Status

**As of 2025-10-28** (immediately before v0.3.0 release):

```bash
$ goneat dependencies --licenses --cooling --format markdown
```

**Results**:

- **Total Dependencies**: 93
- **License Violations**: 0
- **Cooling Violations**: 0
- **Overall Status**: ✅ **CLEAN BILL OF HEALTH**

**License Breakdown**:

- MIT: 67 packages (72%)
- BSD-3-Clause: 15 packages (16%)
- Apache-2.0: 8 packages (9%)
- ISC: 3 packages (3%)

**Cooling Policy Compliance**:

- All 93 packages meet minimum age threshold (≥7 days)
- All 93 packages meet download thresholds (≥100 total, ≥10 recent)
- Zero packages required exceptions

**SBOM Generation**:

- Format: CycloneDX 1.6 JSON
- Total packages cataloged: 723 (including transitive dependencies)
- Includes: Go modules, GitHub Actions, binary dependencies
- Output size: ~450 KB

---

## Lessons Learned & Best Practices

### What Works Well

1. **License check at pre-commit** is fast enough (<1s) to not annoy developers
2. **Cooling check at pre-push** provides security without blocking commits
3. **Explicit allowlist** prevents accidental adoption of problematic licenses
4. **Organization exception** (`github.com/fulmenhq/*`) reduces false positives without compromising security
5. **7-day cooling period** catches most threats while allowing reasonable development velocity

### What We Considered But Rejected

1. ❌ **Cooling check at pre-commit**: Too slow (3-5s), would frustrate developers
2. ❌ **30-day cooling period**: Too conservative, would block legitimate development
3. ❌ **No allowlist (only forbid list)**: Too permissive, missed some weak-copyleft licenses
4. ❌ **Vulnerability scanning in pre-push**: Too slow (30-60s), moving to v0.3.1 with `goneat security --grype`

### Common Pitfalls & Solutions

**Problem**: "All packages fail cooling policy after fresh install"
**Solution**: System clock was wrong. Fix clock and clear cache: `rm -rf ~/.goneat/cache/registry/`

**Problem**: "Pre-push is too slow (60+ seconds)"
**Solution**: Proxy configuration was causing timeouts. Set `HTTPS_PROXY` environment variable.

**Problem**: "Emergency fix blocked by 7-day cooling"
**Solution**: Added temporary exception with expiration date and approval ticket:

```yaml
exceptions:
  - module: "github.com/example/urgent-fix"
    until: "2025-12-31"
    reason: "Emergency security fix required immediately"
    approved_by: "@cto"
    ticket: "SEC-1234"
```

---

## Implementation Checklist

Use this checklist when setting up dependency protection in your own project:

### Initial Setup (5 minutes)

- [ ] Copy `.goneat/dependencies.yaml` from this repo as template
- [ ] Customize `licenses.forbidden` for your organization's policy
- [ ] Decide: explicit `allowed` list (strict) or just `forbidden` list (permissive)
- [ ] Set `cooling.min_age_days` based on risk tolerance (7/14/30 days)
- [ ] Add exceptions for your organization: `github.com/yourorg/*`
- [ ] Test: `goneat dependencies --licenses --cooling`

### Hook Integration (10 minutes)

- [ ] Add license check to pre-commit hook (fast, offline)
- [ ] Add license + cooling to pre-push hook (comprehensive, online)
- [ ] Set appropriate timeouts (30s pre-commit, 45s pre-push)
- [ ] Test commit: `git commit --allow-empty -m "test"`
- [ ] Test push: `git push --dry-run`
- [ ] Document network requirement for pre-push in team wiki

### CI/CD Integration (15 minutes)

- [ ] Add `goneat dependencies --licenses --cooling --fail-on high` to CI pipeline
- [ ] Generate SBOM on release: `goneat dependencies --sbom`
- [ ] Archive SBOM as CI artifact
- [ ] Add dependency check as required status check on main branch
- [ ] Document how to bypass in emergency (with approval process)

### Team Rollout (1 day)

- [ ] Present to team: why we're doing this (supply chain security)
- [ ] Show example: how to handle cooling violations
- [ ] Document exception approval process
- [ ] Set grace period: `grace_period_days: 7` for first week
- [ ] Monitor Slack/chat for questions/issues
- [ ] After 1 week: reduce grace period to 3 days
- [ ] After 1 month: review exceptions, tighten policies if needed

---

## Future Enhancements (v0.3.1+)

**Planned for v0.3.1**:

- 🔒 Vulnerability scanning with Grype integration (`goneat security --grype`)
- 📊 Vulnerability scoring and risk assessment
- 🛡️ Auto-remediation suggestions (upgrade to fixed version)
- 🔍 CVE database integration (offline mode with periodic sync)

**Under consideration**:

- 📈 Trending malicious packages detection (PyPI/npm threat feeds)
- 🎯 SLSA provenance verification for critical dependencies
- 🔐 Signature verification for downloaded packages (sigstore integration)
- 📝 Policy-as-code with custom Rego policies

---

## References

**Internal Documentation**:

- [Dependency Protection Overview](../guides/dependency-protection-overview.md) - User-facing guide
- [Package Cooling Policy](../guides/package-cooling-policy.md) - Threat model and design
- [Dependency Troubleshooting](../troubleshooting/dependencies.md) - Common issues and solutions
- [Dependency Gating Workflow](../user-guide/workflows/dependency-gating.md) - Integration patterns

**Configuration Files**:

- `.goneat/dependencies.yaml` - Dependency policy (this document's primary reference)
- `.goneat/hooks.yaml` - Git hook configuration
- `go.mod` - Go dependencies
- `CHANGELOG.md` - Version history and policy changes

**External Resources**:

- [NIST Supply Chain Security](https://www.nist.gov/itl/executive-order-improving-nations-cybersecurity/software-supply-chain-security-guidance)
- [SLSA Framework](https://slsa.dev/)
- [Open Policy Agent (OPA)](https://www.openpolicyagent.org/)
- [CycloneDX SBOM Standard](https://cyclonedx.org/)

---

## Questions & Support

**For goneat users**:

- Documentation: `goneat docs dependency-protection-overview`
- Command help: `goneat dependencies --help`
- Issues: https://github.com/fulmenhq/goneat/issues

**For goneat contributors**:

- Update this document when changing `.goneat/dependencies.yaml`
- Document policy changes in `CHANGELOG.md`
- Test all examples before committing (this is a living document)

---

**Maintained by**: @3leapsdave  
**Contributors**: Code Scout, Forge Neat  
**License**: Same as goneat (Apache-2.0)
