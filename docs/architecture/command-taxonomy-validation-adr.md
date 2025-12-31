---
title: "Command Taxonomy Validation Architecture"
description: "Architectural decision record for implementing hybrid taxonomy validation system"
author: "goneat contributors"
date: "2025-08-29"
last_updated: "2025-08-29"
status: "approved"
tags:
  - "architecture"
  - "taxonomy"
  - "validation"
  - "testing"
  - "command-structure"
category: "architecture"
---

# Command Taxonomy Validation Architecture

## Status

**APPROVED** - Implementation in progress

## Context

The goneat CLI tool uses a three-level taxonomy system to classify commands:

1. **Groups**: Operational classification (Support, Workflow, Neat)
2. **Categories**: Functional classification within groups
3. **Capabilities**: Behavioral metadata for automation

As the command set grows, we need a validation system to ensure:

- Core commands are properly registered with correct classifications
- Taxonomy consistency is maintained
- Future extensions don't break expected behavior
- Configuration mistakes are caught early

## Decision

Implement a **hybrid taxonomy validation system** that combines:

1. **Strict validation** for core commands (must exist with correct classification)
2. **Flexible validation** for extensions (allowed but monitored)
3. **Consistency checks** for taxonomy structure integrity

## Rationale

### Why Not Completely Dynamic?

- Silent failures when commands are misclassified
- No way to catch registration mistakes
- Difficult to maintain expected behavior contracts
- Hard to document system capabilities for automation

### Why Not Strict Expected Configuration?

- Too rigid for future extensibility
- Requires test updates for every new command
- Doesn't support plugin/extension scenarios
- Maintenance overhead for legitimate additions

### Why Hybrid Approach?

- **Core Stability**: Critical commands have guaranteed classifications
- **Extension Flexibility**: Room for plugins and third-party commands
- **Fail-Fast Detection**: Catches configuration mistakes early
- **Documentation**: Clear separation of core vs. extension functionality

## Implementation

### Core Components

#### 1. Taxonomy Validator

```go
// internal/ops/taxonomy_validation.go
type TaxonomyValidator struct {
    coreCommands     map[string]CommandClassification
    allowedGroups    []CommandGroup
    allowedCategories map[CommandGroup][]CommandCategory
}

type ValidationError struct {
    Type        ErrorType
    Command     string
    Message     string
    Severity    ErrorSeverity
}
```

#### 2. Validation Rules

**Core Command Validation:**

- Commands in `coreCommands` map must exist
- Must have exact group/category classification
- Failures are test errors

**Extension Validation:**

- Additional commands allowed but logged
- Warnings for unexpected classifications
- No test failures for extensions

**Consistency Validation:**

- All commands use valid group/category combinations
- No orphaned categories or invalid relationships
- Taxonomy structure integrity

#### 3. Test Integration

```go
// internal/ops/registry_test.go
func TestTaxonomyValidation(t *testing.T) {
    registry := GetRegistry()
    validator := NewTaxonomyValidator()

    errors := validator.Validate(registry)

    // Core errors fail tests
    coreErrors := filterErrors(errors, ErrorTypeCoreCommand)
    assert.Empty(t, coreErrors, "Core command validation failed")

    // Extension warnings are logged
    extensionWarnings := filterErrors(errors, ErrorTypeExtensionWarning)
    for _, warning := range extensionWarnings {
        t.Logf("Taxonomy warning: %v", warning)
    }
}
```

### Expected Core Commands

| Command   | Group         | Category              | Rationale                       |
| --------- | ------------- | --------------------- | ------------------------------- |
| `assess`  | GroupNeat     | CategoryAssessment    | Core multi-operation assessment |
| `format`  | GroupNeat     | CategoryFormatting    | Core formatting functionality   |
| `lint`    | GroupNeat     | CategoryAnalysis      | Core linting functionality      |
| `envinfo` | GroupSupport  | CategoryEnvironment   | Core environment diagnostics    |
| `version` | GroupSupport  | CategoryInformation   | Core version information        |
| `home`    | GroupSupport  | CategoryConfiguration | Core user configuration         |
| `hooks`   | GroupWorkflow | CategoryOrchestration | Core hook orchestration         |

## Consequences

### Positive

1. **Early Error Detection**: Misclassifications caught in CI/CD
2. **Documentation**: Expected taxonomy clearly defined
3. **Maintainability**: Clear core vs. extension separation
4. **Extensibility**: Framework ready for plugins
5. **Automation Safety**: External tools can rely on core classifications

### Negative

1. **Maintenance Overhead**: Core command list requires updates
2. **Test Complexity**: More sophisticated test structure
3. **Documentation Burden**: Need to maintain expected classifications

### Mitigation

1. **Clear Process**: Document how to add new core commands
2. **Tooling**: Provide helpers for taxonomy validation
3. **Automation**: CI/CD integration for validation
4. **Migration Path**: Clear upgrade process for taxonomy changes

## Alternatives Considered

### Alternative 1: Completely Dynamic

**Rejected** - No validation of core functionality

### Alternative 2: Strict Expected Configuration

**Rejected** - Too rigid for extensibility

### Alternative 3: Runtime Validation Only

**Rejected** - No build-time error detection

## Implementation Plan

### Phase 1: Core Implementation

1. Create `TaxonomyValidator` struct
2. Implement validation logic
3. Add comprehensive tests
4. Integrate with existing test suite

### Phase 2: CI/CD Integration

1. Add validation to build pipeline
2. Create reporting for taxonomy status
3. Set up monitoring for taxonomy drift

### Phase 3: Documentation & Tooling

1. Document taxonomy maintenance process
2. Create helper tools for taxonomy management
3. Add examples for extension developers

## Success Metrics

- [ ] All core commands pass validation in CI/CD
- [ ] Taxonomy validation runs in <5 seconds
- [ ] Clear error messages for validation failures
- [ ] Extension mechanism documented and tested
- [ ] Zero core command misclassifications in production

## Related Documents

- [Command Taxonomy Design](../standards/command-taxonomy-standard.md) - Taxonomy structure definition
- [Extension Development Guide](../user-guide/extension-development.md) - How to add new commands
- [CI/CD Pipeline](../../.github/workflows/validation.yml) - Validation integration

---

**Decision Made By**: Code Scout (Task Execution & Assessment Expert)
**Approved By**: @3leapsdave
**Date**: 2025-08-29
**Review Date**: 2025-09-05
