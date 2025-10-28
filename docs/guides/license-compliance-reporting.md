# License Compliance Reporting Guide

**How to prove your dependencies are compliant to legal, security, and business stakeholders**

## Why This Guide Exists

Your CTO asks: *"Are you sure we don't have any GPL packages?"*

Your legal team needs: *"Provide a complete license audit for Q4 compliance review."*

Your security team requires: *"Document all open source dependencies before deployment."*

**This guide shows you how to generate professional compliance reports using goneat.**

---

## Quick Answer: Proving GPL-Free Status

The fastest way to prove compliance:

```bash
# 1. Run license check
goneat dependencies --licenses

# 2. Check for forbidden licenses
goneat dependencies --licenses --format json | jq '.Dependencies[] | .License.Type' | grep -i "gpl\|agpl\|mpl"

# If output is empty → you're GPL-free ✅
```

**For business leaders, read on for comprehensive reporting.**

---

## Three-Step Compliance Reporting Process

### Step 1: Run Comprehensive Analysis

```bash
# Analyze all dependencies
goneat dependencies --licenses --cooling --sbom

# For JSON output (parseable by other tools)
goneat dependencies --licenses --format json --output compliance-report.json
```

**What this does:**
- Scans all dependencies for license information
- Checks package cooling policy (supply chain security)
- Generates SBOM (Software Bill of Materials)

### Step 2: Generate License Summary

```bash
# Get license breakdown
goneat dependencies --licenses --format json | \
  jq -r '.Dependencies[] | .License.Type' | \
  sort | uniq -c | sort -rn

# Output example:
#   38 Apache-2.0
#   30 MIT
#   24 ISC
#    1 Unlicense
```

### Step 3: Verify Zero Forbidden Licenses

```bash
# Explicitly check for GPL/AGPL/MPL
goneat dependencies --licenses --format json | \
  jq -r '.Dependencies[] | .License.Type' | \
  grep -i "gpl\|agpl\|mpl" || echo "✅ No forbidden licenses"
```

---

## Complete Compliance Report Template

Use this template to create a professional compliance report:

```bash
# Save to compliance-report.md
cat > compliance-report.md << 'EOF'
# License Compliance Report

**Project**: [Your Project Name]
**Generated**: $(date +%Y-%m-%d)
**Tool**: goneat dependencies v$(goneat version --short)
**Status**: [COMPLIANT/NON-COMPLIANT]

---

## Executive Summary

**Total Dependencies**: [COUNT]
**Forbidden Licenses Found**: [0 or COUNT]
**Compliance Status**: [✅ PASS / ❌ FAIL]

---

## License Distribution

| License | Count | Compliance |
|---------|-------|------------|
| Apache-2.0 | X | ✅ Allowed |
| MIT | X | ✅ Allowed |
| ISC | X | ✅ Allowed |

---

## Forbidden License Check

**GPL-3.0**: 0 found ✅
**AGPL-3.0**: 0 found ✅
**MPL-2.0**: 0 found ✅

---

## Policy Configuration

[Paste your .goneat/dependencies.yaml policy]

---

## Verification Commands

Run these commands to reproduce this report:

\`\`\`bash
# License check
goneat dependencies --licenses

# Forbidden license search
goneat dependencies --licenses --format json | \\
  jq -r '.Dependencies[] | .License.Type' | \\
  grep -i "gpl"
\`\`\`

---

## Auditor Certification

I certify that all dependencies have been reviewed and comply with
our organization's license policy.

**Auditor**: [Your Name]
**Date**: [Date]
**Tool Version**: goneat v[VERSION]
EOF
```

---

## Automated Compliance Report (Coming in v0.3.1)

**Planned feature**: Single command to generate complete compliance report

```bash
# Coming soon
goneat dependencies --licenses --report compliance-report.md

# Will generate:
# - License breakdown table
# - Forbidden license check
# - Dependency list with licenses
# - Policy configuration
# - Reproducibility commands
# - Auditor sign-off section
```

**Track progress**: https://github.com/fulmenhq/goneat/issues (search "compliance report")

---

## Real-World Example: Goneat's Own Compliance

Here's how goneat proves its own compliance:

```bash
# 1. Run full analysis
$ goneat dependencies --licenses --format json

# Results:
# - Dependencies: 93
# - Passed: true
# - License violations: 0

# 2. Get license breakdown
$ goneat dependencies --licenses --format json | \
    jq -r '.Dependencies[] | .License.Type' | \
    sort | uniq -c | sort -rn

# Results:
#   38 Apache-2.0
#   30 MIT
#   24 ISC
#    1 Unlicense

# 3. Check for GPL/AGPL/MPL
$ goneat dependencies --licenses --format json | \
    jq -r '.Dependencies[] | .License.Type' | \
    grep -i "gpl\|agpl\|mpl"

# Results: (empty) → ✅ No forbidden licenses
```

**Status**: ✅ **100% Compliant** - Zero GPL/AGPL/MPL in all 93 dependencies

See [`docs/appnotes/dogfooding-dependency-protection.md`](../appnotes/dogfooding-dependency-protection.md) for complete details.

---

## Compliance Scenarios

### Scenario 1: Quarterly Legal Audit

**Request**: "Legal needs a full dependency audit for Q4."

**Response**:

```bash
# 1. Generate SBOM (Software Bill of Materials)
goneat dependencies --sbom --sbom-output sbom/q4-2025-audit.cdx.json

# 2. Run license analysis
goneat dependencies --licenses --format json --output licenses-q4-2025.json

# 3. Create summary report
goneat dependencies --licenses > licenses-q4-2025.txt

# 4. Package for legal
tar -czf q4-2025-compliance.tar.gz \
  sbom/q4-2025-audit.cdx.json \
  licenses-q4-2025.json \
  licenses-q4-2025.txt \
  .goneat/dependencies.yaml

# 5. Send to legal with cover memo
```

**Files to provide**:
- SBOM in CycloneDX format (industry standard)
- License analysis JSON (machine-readable)
- Human-readable summary
- Policy configuration

### Scenario 2: Security Review for Deployment

**Request**: "Security team needs proof we're GPL-free before production deployment."

**Response**:

```bash
# 1. Run compliance check
goneat dependencies --licenses --fail-on high

# 2. Generate proof-of-compliance
cat > security-compliance-proof.md << 'EOF'
# Security Compliance Certificate

**Deployment**: Production v1.0.0
**Date**: $(date)
**Auditor**: $(whoami)

## GPL/AGPL/MPL Status

\`\`\`bash
$ goneat dependencies --licenses --format json | \
    jq '.Dependencies[] | .License.Type' | grep -i "gpl"

# Result: (empty)
\`\`\`

**Status**: ✅ ZERO GPL/AGPL/MPL licenses detected

## License Distribution

\`\`\`bash
$ goneat dependencies --licenses --format json | \
    jq -r '.Dependencies[] | .License.Type' | sort | uniq -c

  38 Apache-2.0
  30 MIT
  24 ISC
   1 Unlicense
\`\`\`

**All licenses**: Permissive and approved for commercial use.

**Approved for deployment**: ✅ YES

---

Signed: [Name]
Date: [Date]
EOF
```

### Scenario 3: Customer Due Diligence

**Request**: "Enterprise customer requires license audit before purchasing."

**Response**:

```bash
# 1. Generate customer-facing report
goneat dependencies --sbom --sbom-output customer-sbom.json

# 2. Create license summary
cat > customer-license-summary.md << 'EOF'
# Third-Party License Summary

**Product**: [Your Product]
**Version**: [Version]
**Provided**: [Date]

## Open Source Dependencies

Your product uses the following open source components:

| Component Count | License Type | Commercial Use |
|----------------|--------------|----------------|
| 38 | Apache-2.0 | ✅ Permitted |
| 30 | MIT | ✅ Permitted |
| 24 | ISC | ✅ Permitted |
| 1 | Unlicense | ✅ Permitted |

## GPL/AGPL/Copyleft Status

**GPL-3.0**: Not present ✅
**AGPL-3.0**: Not present ✅
**MPL-2.0**: Not present ✅

All dependencies use permissive licenses that allow commercial use,
modification, and distribution without source code disclosure requirements.

## SBOM Provided

A complete Software Bill of Materials (SBOM) in CycloneDX format is
attached for your security and compliance review.

**File**: customer-sbom.json
**Format**: CycloneDX 1.5 JSON
**Components**: [COUNT]

---

For questions: [contact@yourcompany.com]
EOF
```

---

## Cross-Verification with Other Tools

For maximum confidence, verify goneat's results with independent tools:

### Verification with go-licenses

```bash
# Install go-licenses
go install github.com/google/go-licenses@latest

# Run audit
go-licenses csv github.com/yourorg/yourproject | grep -i "gpl"

# Should return empty if GPL-free
```

### Verification with make license-audit

```bash
# If you have this in your Makefile
make license-audit

# Should output: ✅ No forbidden licenses detected
```

### Triple Verification Example

```bash
# Method 1: goneat
goneat dependencies --licenses | grep -i "violations: 0"

# Method 2: go-licenses
go-licenses csv . | grep -i "gpl" || echo "No GPL found"

# Method 3: make target
make license-audit

# All three agree → high confidence ✅
```

---

## Common Questions from Stakeholders

### "How do I know this is accurate?"

**Answer**: Run the verification yourself:

```bash
# 1. Clone the project
git clone [repo-url]

# 2. Run goneat analysis
goneat dependencies --licenses

# 3. Cross-check with go-licenses
go-licenses csv . | grep -i "gpl"

# 4. Review the policy
cat .goneat/dependencies.yaml
```

All commands are reproducible by anyone with the repository.

### "What if we add a new dependency?"

**Answer**: Automated enforcement prevents violations:

```yaml
# Git hook automatically runs on every push
pre-push:
  - goneat dependencies --licenses --fail-on high
```

Any GPL/AGPL/MPL dependency will:
1. Be detected automatically
2. Fail the build
3. Block the push
4. Require explicit approval to proceed

### "Can I trust goneat's detection?"

**Answer**: Yes, for multiple reasons:

1. **Battle-tested tools**: Uses `go-licenses` (Google) under the hood for Go
2. **Cross-verification**: Can verify with independent tools
3. **Explicit allowlist**: We use strictest approach (only allow specific licenses)
4. **Open source**: goneat itself is Apache-2.0 and auditable
5. **Dogfooding**: goneat uses itself for compliance (93 deps, 0 violations)

### "What about transitive dependencies?"

**Answer**: goneat analyzes the entire dependency tree:

```bash
# Includes both direct and transitive dependencies
goneat dependencies --licenses

# Also available in SBOM
goneat dependencies --sbom

# The SBOM includes relationship graph showing:
# - Direct dependencies
# - Transitive dependencies  
# - Dependency chains
```

All dependencies analyzed, not just direct imports.

---

## Compliance Report Checklist

Use this checklist when preparing compliance reports:

**Analysis Phase:**
- [ ] Run `goneat dependencies --licenses`
- [ ] Run `goneat dependencies --cooling` (if required)
- [ ] Generate SBOM: `goneat dependencies --sbom`
- [ ] Save JSON output: `--format json --output report.json`

**Verification Phase:**
- [ ] Check for forbidden licenses: `grep -i "gpl\|agpl\|mpl"`
- [ ] Verify zero violations: Check `"Passed": true` in JSON
- [ ] Cross-check with go-licenses (optional but recommended)
- [ ] Review policy file: `.goneat/dependencies.yaml`

**Documentation Phase:**
- [ ] Create executive summary (compliance status)
- [ ] Include license breakdown table
- [ ] Document forbidden license check results
- [ ] Attach SBOM file
- [ ] Include policy configuration
- [ ] Add reproducibility commands

**Delivery Phase:**
- [ ] Sign and date the report
- [ ] Include tool version: `goneat version`
- [ ] Provide verification instructions
- [ ] Archive for future audits

---

## Integration with CI/CD

Automate compliance reporting in your pipeline:

### GitHub Actions Example

```yaml
name: Compliance Report

on:
  schedule:
    - cron: '0 0 1 * *'  # Monthly on 1st
  workflow_dispatch:      # Manual trigger

jobs:
  compliance:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Install goneat
        run: go install github.com/fulmenhq/goneat@latest
      
      - name: Run compliance check
        run: |
          goneat dependencies --licenses --format json --output compliance.json
          goneat dependencies --sbom --sbom-output sbom.json
      
      - name: Check for violations
        run: |
          VIOLATIONS=$(jq '.Dependencies[] | select(.License.Type | test("GPL|AGPL|MPL"; "i"))' compliance.json | wc -l)
          if [ "$VIOLATIONS" -gt 0 ]; then
            echo "❌ Found $VIOLATIONS forbidden licenses"
            exit 1
          fi
          echo "✅ No forbidden licenses found"
      
      - name: Upload compliance artifacts
        uses: actions/upload-artifact@v4
        with:
          name: compliance-report-${{ github.run_number }}
          path: |
            compliance.json
            sbom.json
```

### Monthly Compliance Report

```bash
# Add to cron or CI/CD
#!/bin/bash
DATE=$(date +%Y-%m)
REPORT_DIR="compliance-reports/$DATE"

mkdir -p "$REPORT_DIR"

# Generate reports
goneat dependencies --licenses --format json --output "$REPORT_DIR/licenses.json"
goneat dependencies --sbom --sbom-output "$REPORT_DIR/sbom.json"

# Create summary
goneat dependencies --licenses > "$REPORT_DIR/summary.txt"

# Archive
tar -czf "compliance-$DATE.tar.gz" "$REPORT_DIR"

echo "✅ Monthly compliance report generated: compliance-$DATE.tar.gz"
```

---

## Advanced: Custom Compliance Policies

Different organizations have different policies:

### Example 1: Ultra-Permissive (Startup)

```yaml
# .goneat/dependencies.yaml
licenses:
  forbidden:
    - GPL-3.0
    - AGPL-3.0
  # Allow everything else except GPL/AGPL
```

### Example 2: Conservative (Enterprise)

```yaml
# .goneat/dependencies.yaml
licenses:
  allowed:  # Explicit allowlist
    - MIT
    - Apache-2.0
    - BSD-3-Clause
    - ISC
  # Everything else is forbidden by default
```

### Example 3: Custom Policy (Financial Services)

```yaml
# .goneat/dependencies.yaml
licenses:
  forbidden:
    - GPL-3.0
    - AGPL-3.0
    - MPL-2.0
    - LGPL-3.0  # Even LGPL forbidden
  allowed:
    - MIT
    - Apache-2.0
    - BSD-3-Clause
  # Require approval for anything else
```

Adjust your policy to match your organization's risk tolerance.

---

## Getting Help

### Documentation
- [Dependency Protection Overview](dependency-protection-overview.md) - Complete feature guide
- [Package Cooling Policy](package-cooling-policy.md) - Supply chain security
- [Dependencies Command Reference](../user-guide/commands/dependencies.md) - CLI documentation
- [Troubleshooting](../troubleshooting/dependencies.md) - Common issues

### Support
- **Issues**: https://github.com/fulmenhq/goneat/issues
- **Discussions**: https://github.com/fulmenhq/goneat/discussions
- **Enterprise**: support@3leaps.net

---

## Summary

**Key Takeaways**:

1. ✅ Use `goneat dependencies --licenses` to analyze all dependencies
2. ✅ Check for forbidden licenses with `grep -i "gpl\|agpl\|mpl"`
3. ✅ Generate SBOM for comprehensive documentation
4. ✅ Cross-verify with independent tools (go-licenses)
5. ✅ Automate in CI/CD for continuous monitoring
6. ✅ Use templates provided in this guide for professional reports

**Bottom Line**: Proving compliance is straightforward with goneat - run the analysis, verify the results, document with provided templates.

---

**Last Updated**: 2025-10-28  
**Status**: Active  
**Part of**: goneat v0.3.0 Dependency Protection Features
