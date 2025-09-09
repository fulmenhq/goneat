# License Compliance SOP

_Standard Operating Procedures for license compliance and dependency management_

**Version:** 1.0
**Last Updated:** January 25, 2025
**Maintainers:** Architecture Team (@arch-eagle supervised by @3leapsdave)

## Overview

This SOP establishes procedures for maintaining license compliance in the Goneat project, ensuring all dependencies and tools respect our Apache 2.0 license while safely utilizing GPL/AGPL tools during development.

## License Policy Summary

| License Family                    | Go Dependencies | Dev Tools   | Production   | Distribution |
| --------------------------------- | --------------- | ----------- | ------------ | ------------ |
| **Permissive** (MIT, BSD, Apache) | ✅ Allowed      | ✅ Allowed  | ✅ Allowed   | ✅ Allowed   |
| **Weak Copyleft** (LGPL, MPL)     | ⚠️ Review       | ✅ Allowed  | ⚠️ Review    | ⚠️ Review    |
| **Strong Copyleft** (GPL, AGPL)   | ❌ Forbidden    | ✅ CLI Only | ❌ Forbidden | ❌ Forbidden |
| **Commercial/Proprietary**        | ❌ Forbidden    | ⚠️ Review   | ❌ Forbidden | ❌ Forbidden |

## Compliance Procedures

### 1. Dependency Addition Process

#### Go Module Dependencies

```bash
# Before adding any new dependency
go get github.com/new/dependency

# Immediately run license audit
make license-audit

# Review the report
cat docs/licenses/inventory.csv
```

**Approval Requirements:**

- **Permissive licenses**: Auto-approved (MIT, BSD, Apache 2.0)
- **Weak copyleft**: Architecture review required
- **Strong copyleft**: Rejected automatically
- **Unknown licenses**: Legal review required

#### External Tool Dependencies

```bash
# Document in GPL boundary memo
vim docs/ops/compliance/2025-01-25-gpl-boundary-memo.md

# Verify CLI-only usage pattern
grep -n "exec.Command.*toolname" $(find . -name "*.go")
```

### 2. License Audit Procedures

#### Automated Audit (Go Dependencies)

```bash
# Full license audit with inventory update
make license-audit

# Quick check without updating inventory
go-licenses check ./...
```

#### Manual Audit (External Tools)

1. **Identify all external tool usage:**

   ```bash
   # Find all exec.Command calls
   grep -r "exec\.Command" --include="*.go" .

   # Check Makefile for tool invocations
   grep -E "(golangci-lint|gosec|go-licenses)" Makefile
   ```

2. **Verify tool licenses:**
   - Check tool documentation/repository
   - Update GPL boundary memo if needed
   - Ensure no distribution in releases

#### CI/CD Integration

```yaml
# GitHub Actions example (already implemented)
- name: License Audit
  run: |
    make license-audit
    # Fail if forbidden licenses detected
```

### 3. GPL/AGPL Tool Usage

#### Safe Usage Pattern

```go
// ✅ CORRECT: External process execution
cmd := exec.CommandContext(ctx, "golangci-lint", args...)
output, err := cmd.CombinedOutput()

// ❌ WRONG: Would require importing GPL code
import "github.com/golangci/golangci-lint/pkg/lint"
linter := lint.NewLinter()
```

#### Documentation Requirements

For each GPL/AGPL tool:

1. Add entry to GPL boundary memo
2. Document usage pattern
3. Verify process isolation
4. Confirm no distribution

### 4. License Inventory Management

#### Update Triggers

- New dependency added
- Dependency version updated
- Monthly scheduled audit
- Pre-release verification

#### Update Process

```bash
# 1. Generate new inventory
make license-audit

# 2. Review changes
git diff docs/licenses/inventory.csv

# 3. Update markdown summary
vim docs/licenses/inventory.md

# 4. Commit with clear message
git add docs/licenses/
git commit -m "chore: update license inventory

- Added: package-name (MIT)
- Updated: other-package v1.2.3 (BSD-3)
- No forbidden licenses detected"
```

### 5. Pre-Release License Verification

Before any release:

```bash
# 1. Full dependency audit
make license-audit

# 2. Verify no GPL in distribution
ls -la dist/
# Should contain ONLY goneat binaries

# 3. Check release artifacts
tar -tzf dist/release/goneat_*.tar.gz
# Should NOT contain any GPL tools

# 4. Document in release notes
echo "License Compliance: ✅ Apache 2.0 compatible" >> RELEASE_NOTES.md
```

## Compliance Monitoring

### Automated Checks

1. **Pre-commit**: Quick license check for changed dependencies
2. **Pre-push**: Full license audit
3. **CI/CD**: License audit on every PR
4. **Release**: Comprehensive compliance verification

### Manual Reviews

1. **Weekly**: Review new dependencies in PRs
2. **Monthly**: Full inventory audit
3. **Quarterly**: GPL boundary memo update
4. **Annually**: Policy review and updates

## Exception Process

### Requesting an Exception

1. **Document the need**: Technical justification required
2. **Identify alternatives**: Why can't permissive options work?
3. **Risk assessment**: Legal and technical implications
4. **Mitigation plan**: How to isolate the dependency
5. **Architecture review**: (@arch-eagle)
6. **Approval**: Lead maintainer (@3leapsdave)

### Exception Documentation

Approved exceptions must be documented in:

- GPL boundary memo (for tools)
- License inventory (for dependencies)
- Exception rationale file

## Quick Reference

### Common License Compatibility

| Our License | Compatible With  | Incompatible With       |
| ----------- | ---------------- | ----------------------- |
| Apache 2.0  | MIT, BSD, Apache | GPL\*, AGPL, Commercial |

\*Note: GPL tools safe for development use via CLI

### License Detection Commands

```bash
# Check single package
go-licenses check github.com/pkg/name

# Scan all dependencies
go-licenses csv ./...

# Find GPL mentions
grep -i "GPL" go.mod

# Audit external tools
which golangci-lint && golangci-lint version
```

### Red Flags in Code Review

- ❌ New import with "gpl" in package name
- ❌ Vendored GPL code
- ❌ Binary distribution of GPL tools
- ❌ Network services using AGPL code
- ❌ Static linking of GPL libraries

## Escalation Path

1. **Developer**: Detects potential license issue
2. **Code Review**: Automated checks flag concern
3. **Architecture Team**: Technical assessment
4. **Lead Maintainer**: Business decision
5. **Legal Counsel**: If significant risk

## Related Documentation

- [GPL License Boundary Memo](../ops/compliance/2025-01-25-gpl-boundary-memo.md)
- [License Inventory](../licenses/inventory.md)
- [Repository Operations SOP](repository-operations-sop.md)

---

Generated by Arch Eagle ([Cursor](https://cursor.sh/)) under supervision of [@3leapsdave](https://github.com/3leapsdave)
