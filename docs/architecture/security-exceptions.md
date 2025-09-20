---
title: "Security Exception Handling Architecture"
description: "Strategy for handling security tool exceptions across languages in goneat"
author: "@arch-eagle"
date: "2025-09-01"
last_updated: "2025-09-01"
status: "proposal"
tags: ["architecture", "security", "exceptions", "multi-language"]
---

# Security Exception Handling Architecture

This document proposes a strategy for handling security tool exceptions (suppressions) in goneat, considering both immediate Go-focused needs and future multi-language support.

## Current State Analysis

### 1. Why Exceptions Aren't Working

Based on code inspection, `#nosec` comments SHOULD work because:

- gosec is called without the `-nosec` flag ([security_runner.go:228](../../internal/assess/security_runner.go#L228))
- gosec respects `#nosec` comments by default
- The JSON output from gosec excludes suppressed issues

**Hypothesis**: The issue count includes all files in the repository, and existing `#nosec` comments may not be properly formatted or placed.

### 2. Current Tool Support

| Language   | Security Tool | Exception Syntax               | Notes                     |
| ---------- | ------------- | ------------------------------ | ------------------------- |
| Go         | gosec         | `#nosec` or `#nosec G104`      | Rule-specific suppression |
| Go         | govulncheck   | N/A                            | No inline suppression     |
| Go         | golangci-lint | `//nolint` or `//nolint:gosec` | Tool-specific             |
| TypeScript | ESLint        | `// eslint-disable-next-line`  | Rule-specific             |
| TypeScript | Biome         | `// biome-ignore`              | Their own syntax          |
| Python     | Bandit        | `# nosec` or `# nosec B101`    | Similar to gosec          |
| Python     | Ruff          | `# noqa` or `# noqa: S101`     | PEP 8 style               |

## Proposed Architecture

### Phase 1: Native Tool Support (Immediate)

Support each tool's native exception syntax without modification:

```go
// pkg/security/exceptions.go
type SecurityException struct {
    Tool     string   `json:"tool"`
    RuleID   string   `json:"rule_id,omitempty"`
    Line     int      `json:"line"`
    Reason   string   `json:"reason,omitempty"`
    Syntax   string   `json:"syntax"` // The actual comment used
}

// Example Go usage:
func dangerousOperation() {
    // #nosec G304 - Path is validated upstream
    content, err := os.ReadFile(userPath)
}

// Example TypeScript usage:
// biome-ignore lint/security/noGlobalEval: Required for dynamic config
eval(configExpression);
```

**Benefits**:

- Works immediately with existing tools
- No learning curve for developers
- Preserves tool ecosystem compatibility
- Can be implemented quickly

### Phase 2: Unified Reporting (Short-term)

Track and report on exceptions across all tools:

```json
{
  "security_exceptions": {
    "total": 15,
    "by_tool": {
      "gosec": 10,
      "biome": 3,
      "bandit": 2
    },
    "by_reason": {
      "validated_input": 8,
      "legacy_code": 4,
      "false_positive": 3
    }
  }
}
```

### Phase 3: Language-Neutral Metadata (Medium-term)

Add optional structured metadata while preserving native syntax:

```go
// Go example
// #nosec G304
// @goneat-exception reason="Path validated in middleware" reviewer="@3leapsdave" expires="2025-09-20"
content, err := os.ReadFile(userPath)

// TypeScript example
// biome-ignore lint/security/noGlobalEval
// @goneat-exception reason="Required for plugin system" risk-accepted="true"
eval(pluginCode);

// Python example
# nosec B104
# @goneat-exception reason="Hardcoded password for test fixture" scope="test-only"
TEST_PASSWORD = "admin123"
```

**Metadata Schema**:

```yaml
exception_metadata:
  reason: string # Required: Why this exception exists
  reviewer: string # Who approved this exception
  expires: date # When to re-review
  risk-accepted: boolean # Explicit risk acceptance
  scope: string # test-only, development, etc.
  jira: string # Issue tracking reference
```

## Implementation Strategy

### 1. Immediate Actions (Phase 1)

```go
// internal/assess/security_runner.go modifications
func (r *SecurityAssessmentRunner) runGosec(ctx context.Context, moduleRoot string, config AssessmentConfig) ([]Issue, error) {
    // Existing code...

    // Add support for tracking suppressions
    args := []string{"-quiet", "-fmt=json"}
    if config.TrackSuppressions {
        args = append(args, "-track-suppressions")
    }

    // Continue with existing implementation...
}
```

### 2. Exception Validation Rules

```go
// pkg/security/validation.go
type ExceptionValidator struct {
    rules []ValidationRule
}

type ValidationRule interface {
    Validate(exc SecurityException) error
}

// Example rules:
// - RequireReasonRule: Exceptions must have a reason
// - ExpirationRule: Exceptions must be reviewed periodically
// - ApprovalRule: High/Critical exceptions need approval
```

### 3. Reporting Integration

Extend the JSON output schema:

```json
{
  "categories": {
    "security": {
      "issues": [...],
      "exceptions": [
        {
          "file": "cmd/security.go",
          "line": 200,
          "tool": "gosec",
          "rule": "G204",
          "syntax": "#nosec G204",
          "metadata": {
            "reason": "Command arguments are sanitized",
            "reviewer": "@arch-eagle"
          }
        }
      ]
    }
  }
}
```

## Enterprise Considerations

### 1. Policy Enforcement

```yaml
# .goneat/security-policy.yaml
security:
  exceptions:
    require_reason: true
    require_reviewer: true
    max_age_days: 90

    rules:
      - severity: critical
        requires_approval: true
        approvers: ["@security-team"]

      - severity: high
        max_exceptions: 10
        requires_jira: true
```

### 2. Audit Trail

All exceptions should be:

- Tracked in version control
- Included in assessment reports
- Exportable for compliance audits
- Reviewable through dashboards

### 3. Migration Path

For teams with existing suppressions:

1. Scanner discovers existing native suppressions
2. Report on suppressions without metadata
3. Gradually add metadata during code reviews
4. Enforce metadata for new exceptions

## Decision Points

### 1. Should we create a unified syntax?

**Pros**:

- Consistent across languages
- Easier to teach and document
- Can enforce consistent metadata

**Cons**:

- Breaks tool ecosystem compatibility
- Requires custom parsers
- May not be recognized by IDEs/editors

**Recommendation**: No. Stay with native tool syntax but add optional metadata.

### 2. Should exceptions require justification?

**Recommendation**: Yes, but make it configurable:

- Development: Optional reasons
- Production: Required reasons with review
- Enterprise: Full metadata with approvals

### 3. How to handle tool-specific differences?

**Recommendation**: Abstract at the reporting layer:

```go
type ExceptionReport struct {
    Tool       string
    Syntax     string // Original syntax
    Normalized NormalizedRule // Mapped to common categories
}
```

## Next Steps

1. **Immediate**: Fix gosec integration to properly respect `#nosec` comments
2. **Week 1**: Implement exception tracking in security assessment
3. **Week 2**: Add exception reporting to JSON output
4. **Month 1**: Design metadata schema and validation rules
5. **Month 2**: Implement policy enforcement for enterprises

## Code Examples

### Testing Exception Support

```go
// test/fixtures/security/exceptions_test.go
package main

import "os"

func testExceptions() {
    // Should be reported - no exception
    os.ReadFile("/etc/passwd") // Line 10

    // Should be suppressed - inline exception
    // #nosec G304
    os.ReadFile("/tmp/user.txt") // Line 14

    // Should be suppressed - specific rule
    // #nosec G304 - User input validated in middleware
    os.ReadFile(validatedPath) // Line 18
}
```

### Configuration

```yaml
# .goneat.yaml
security:
  tools:
    gosec:
      respect_suppressions: true # Default
      track_suppressions: true # Include in reports

    custom_rules:
      - id: "EX001"
        description: "Exception without reason"
        severity: "low"
        pattern: "#nosec(?!.*-)" # Regex for #nosec without dash
```

## Conclusion

The recommended approach is to:

1. Respect native tool suppression syntax
2. Add optional structured metadata for enterprise needs
3. Provide comprehensive reporting on all exceptions
4. Enable policy-based validation for different environments

This balances immediate functionality with long-term enterprise requirements while maintaining ecosystem compatibility.

---

Generated by @arch-eagle under supervision of @3leapsdave
