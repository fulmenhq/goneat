---
title: "OPA v1 and Rego v1 Syntax Migration"
description: "Architectural decision to adopt OPA v1 imports and Rego v1 syntax for policy evaluation"
author: "@arch-eagle"
date: "2025-10-10"
last_updated: "2025-10-10"
status: "approved"
tags:
  - "architecture"
  - "policy"
  - "opa"
  - "rego"
  - "dependencies"
  - "security"
category: "architecture"
---

# OPA v1 and Rego v1 Syntax Migration

## Status

**APPROVED** - Implemented in v0.3.0 Wave 2 Phase 1

## Context

The goneat dependencies package uses the Open Policy Agent (OPA) for policy-based dependency validation. During the Wave 2 implementation, we discovered that despite having OPA v1.9.0 installed, the codebase was using:

1. **Deprecated v0.x import path**: `github.com/open-policy-agent/opa/rego`
2. **Rego v0 syntax**: Rules without `if` and `contains` keywords

This created several issues:

- Deprecation warnings in static analysis (staticcheck SA1019)
- Missing access to OPA v1 features and optimizations
- Technical debt accumulating in policy engine code
- Potential breaking changes when v0.x compatibility layer is removed

## Decision

**Migrate to OPA v1 imports and generate Rego v1 syntax for all policy rules.**

### Import Path Change

```go
// Before (deprecated)
import "github.com/open-policy-agent/opa/rego"

// After (modern)
import "github.com/open-policy-agent/opa/v1/rego"
```

### Rego Syntax Upgrade

```rego
# Before (Rego v0 syntax)
deny[msg] {
  dep := input.dependencies[_]
  forbidden[_] == dep.license.type
  msg := sprintf("Package %s uses forbidden license: %s",
    [dep.module.name, dep.license.type])
}

# After (Rego v1 syntax)
deny contains msg if {
  dep := input.dependencies[_]
  forbidden[_] == dep.license.type
  msg := sprintf("Package %s uses forbidden license: %s",
    [dep.module.name, dep.license.type])
}
```

**Key Changes:**

1. Partial set rules require `contains` keyword
2. All rules require explicit `if` keyword before body
3. Function definitions require `if` keyword

## Rationale

### Why OPA v1?

1. **Active Development**
   - OPA v1 is the actively maintained branch
   - v0.x compatibility layer is for legacy code only
   - Future features will be v1-only

2. **Better Performance**
   - v1 includes optimizations not backported to v0.x
   - Improved compilation and evaluation performance
   - Better memory management

3. **Cleaner Errors**
   - v1 provides more detailed error messages
   - Better debugging experience with Rego v1 syntax
   - Improved type checking

4. **Future-Proofing**
   - v0.x compatibility layer will eventually be removed
   - Migrating now prevents forced migration later
   - Access to new OPA features as they're released

### Why Rego v1 Syntax?

1. **Required by OPA v1**
   - OPA v1 defaults to Rego v1 syntax parser
   - v0 syntax requires explicit compatibility flag
   - Going forward with modern syntax aligns with OPA direction

2. **More Explicit**

   ```rego
   # v0: Ambiguous partial set rule
   deny[msg] { ... }

   # v1: Clearly states this is a partial set
   deny contains msg if { ... }
   ```

3. **Better Tooling Support**
   - IDE plugins understand v1 syntax better
   - Rego playground defaults to v1
   - OPA documentation examples use v1

4. **Consistency**
   - All new OPA policies will use v1 syntax
   - Mixing v0 and v1 syntax creates confusion
   - Team learns one syntax, not two

## Implementation

### Changes Made

#### 1. Import Path Update

**File**: `pkg/dependencies/policy/engine.go:11`

```go
// Changed from:
"github.com/open-policy-agent/opa/rego"

// To:
"github.com/open-policy-agent/opa/v1/rego"
```

#### 2. Rego Transpiler Updates

**File**: `pkg/dependencies/policy/engine.go:transpileYAMLToRego()`

Updated transpiler to generate Rego v1 syntax:

```go
// License policy rules (v1 syntax)
buf.WriteString("deny contains msg if {\n")
buf.WriteString("  dep := input.dependencies[_]\n")
buf.WriteString("  forbidden := " + formatRegoArray(forbidden) + "\n")
buf.WriteString("  forbidden[_] == dep.license.type\n")
buf.WriteString("  msg := sprintf(...)\n")
buf.WriteString("}\n\n")

// Cooling policy rules (v1 syntax)
buf.WriteString("deny contains msg if {\n")
buf.WriteString("  dep := input.dependencies[_]\n")
buf.WriteString("  dep.metadata.age_days < " + minAgeDays + "\n")
buf.WriteString("  not is_cooling_exception(dep.module.name)\n")
buf.WriteString("  msg := sprintf(...)\n")
buf.WriteString("}\n\n")

// Helper functions (v1 syntax)
buf.WriteString("is_cooling_exception(name) if {\n")
buf.WriteString("  false  # TODO: implement exception logic\n")
buf.WriteString("}\n\n")
```

### Testing

All existing tests pass with v1 syntax:

```bash
$ go test ./pkg/dependencies/policy/... -v
=== RUN   TestOPADenyPath
--- PASS: TestOPADenyPath (0.00s)
=== RUN   TestOPACoolingPolicyDeny
--- PASS: TestOPACoolingPolicyDeny (0.00s)
=== RUN   TestOPAPolicyTranspilation
--- PASS: TestOPAPolicyTranspilation (0.00s)
=== RUN   TestOPAInvalidPolicyPath
--- PASS: TestOPAInvalidPolicyPath (0.00s)
=== RUN   TestOPAPathTraversalProtection
--- PASS: TestOPAPathTraversalProtection (0.00s)
PASS
ok  	github.com/fulmenhq/goneat/pkg/dependencies/policy	0.412s
```

### Verification

```bash
# No deprecation warnings
$ staticcheck ./pkg/dependencies/...
✓ Clean
```

## Alternatives Considered

### Alternative 1: Keep v0.x Imports with Compatibility Flag

**Approach**: Use v0.x imports and set `rego.v0Compatible()` flag

**Rejected because:**

- Still using deprecated import path
- Requires maintenance of compatibility mode
- Doesn't solve underlying technical debt
- Just delays inevitable migration

### Alternative 2: Use v1 Imports but v0 Syntax

**Approach**: Import `github.com/open-policy-agent/opa/v1/rego` but keep v0 syntax

**Rejected because:**

- Requires explicit v0 compatibility flag in every evaluation
- OPA v1 defaults to v1 syntax for good reason
- Mixing import versions and syntax versions is confusing
- Doesn't align with OPA project direction

### Alternative 3: External OPA Server

**Approach**: Use external OPA server instead of embedded evaluation

**Rejected because:**

- Adds infrastructure dependency (OPA server)
- Increases latency (network calls)
- Complicates deployment (need to manage OPA server)
- Embedded OPA is sufficient for our use case
- **Note**: Server mode is still supported via config if needed

## Consequences

### Positive

1. **No Deprecation Warnings**
   - Clean static analysis (staticcheck)
   - No technical debt markers

2. **Access to v1 Features**
   - Can use new OPA v1 capabilities as released
   - Better performance and error messages

3. **Future-Proof**
   - Won't break when v0.x compatibility removed
   - Aligned with OPA project direction

4. **Better Documentation Alignment**
   - OPA docs use v1 syntax
   - Team can copy examples directly

### Negative

1. **Learning Curve**
   - Team needs to learn `contains` and `if` keywords
   - **Mitigation**: Small syntax change, easy to understand

2. **Transpiler Complexity**
   - YAML-to-Rego transpiler needs v1 syntax generation
   - **Mitigation**: One-time update, well-tested

3. **Existing Policies**
   - If users have custom Rego policies in v0 syntax, they'll break
   - **Mitigation**: No custom policies exist yet (Wave 1 only has YAML)

## Migration Path for Users

### Wave 1 Users

**No action required** - Wave 1 only used YAML policies transpiled to Rego. The transpiler now generates v1 syntax automatically.

### Future Custom Rego Policies

When we support custom Rego files (Wave 3+), users will need to:

1. Use Rego v1 syntax for new policies
2. Migrate existing v0 policies using OPA's migration guide

**Example migration:**

```rego
# Old v0 syntax
deny[msg] {
  input.x > 10
  msg := "x too large"
}

# New v1 syntax
deny contains msg if {
  input.x > 10
  msg := "x too large"
}
```

## Rollback Plan

If critical issues arise, rollback involves:

1. Revert import path: `"github.com/open-policy-agent/opa/rego"`
2. Revert transpiler syntax changes
3. Accept deprecation warnings temporarily

**Likelihood**: Very low - v1 has been stable for years and is well-tested.

## Monitoring

### Success Metrics

- ✅ All policy tests passing
- ✅ Zero deprecation warnings in static analysis
- ✅ No performance regression in policy evaluation

### Post-Implementation

- Monitor policy evaluation performance
- Track any OPA-related errors in production
- Document any issues with v1 syntax for team knowledge base

## References

### OPA Documentation

- [OPA v1 Release Notes](https://github.com/open-policy-agent/opa/releases/tag/v1.0.0)
- [Rego v1 Syntax Guide](https://www.openpolicyagent.org/docs/latest/policy-language/#rego-v1)
- [v0 Compatibility Mode](https://www.openpolicyagent.org/docs/latest/v0-compatibility/)

### Internal Documentation

- [Dependencies Package](../appnotes/lib/dependencies.md)
- [Policy Engine Implementation](../../pkg/dependencies/policy/engine.go)
- [Wave 2 Specification](.plans/active/v0.3.0/wave-2-detailed-spec.md)

### Code Changes

- **PR**: Wave 2 Phase 1 Implementation
- **Commit**: "feat: migrate to OPA v1 and Rego v1 syntax"
- **Files Modified**:
  - `pkg/dependencies/policy/engine.go` (import + transpiler)
  - `go.mod` (already had OPA v1.9.0)

## Related Decisions

- **Wave 1**: Chose embedded OPA over external server
- **Wave 2 Phase 1**: Mockable HTTP transport for registry clients
- **Future**: May support custom Rego policies (will require v1 syntax)

---

**Decision made by**: @arch-eagle
**Approved by**: Project maintainers
**Implementation**: v0.3.0 Wave 2 Phase 1
**Status**: Complete and deployed
