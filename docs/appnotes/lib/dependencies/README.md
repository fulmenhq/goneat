# Dependencies Package Documentation

This directory contains documentation for the Dependencies library implementation and related tooling.

## Document Map

- **Core Library**
  - [`overview.md`](overview.md) – Package overview, architecture, and key features
  - [`api-reference.md`](api-reference.md) – Complete API reference for core interfaces and types
  - [`cooling-policy.md`](cooling-policy.md) – Supply chain security and cooling policy implementation

- **Integration & Testing**
  - [`testing.md`](testing.md) – Testing strategies, integration tests, and benchmarks
  - [`registry-integration.md`](registry-integration.md) – Registry client integration patterns

- **Guides**
  - [`best-practices.md`](best-practices.md) – Best practices, anti-patterns, and error handling
  - `multi-language-support.md` _(planned Wave 3)_ – Multi-language analyzer patterns

Looking for the CLI instead of the Go library? See the user-facing command reference in [`docs/user-guide/commands/dependencies.md`](../../../user-guide/commands/dependencies.md).

## Overview

The Dependencies package provides comprehensive dependency analysis for multi-language projects, including:

- **License detection and compliance** - Identify and validate software licenses
- **Cooling policy enforcement** - Supply chain security for newly published packages
- **Multi-language support** - Go (Wave 2), npm/PyPI/crates/NuGet (planned)
- **Policy engine integration** - OPA-based policy evaluation with Rego v1
- **SBOM generation** _(planned Wave 2 Phase 5)_ - Software Bill of Materials

### What lives where?

- The **core `Analyzer` interface** (see `pkg/dependencies/analyzer.go`) exposes dependency analysis with policy enforcement
- The **Cooling Policy Checker** (see `pkg/cooling/checker.go`) validates packages against supply chain security rules
- The **Registry Clients** (see `pkg/registry/`) fetch package metadata from multiple ecosystems
- The **`goneat dependencies` CLI** wraps the analyzer for command-line usage

## Current Status

**Wave 2 Implementation Status:**

- ✅ **Phase 1 Complete** (Oct 10, 2025): Registry clients with mockable HTTP transport
- ✅ **Phase 2 Complete**: Cooling policy checker implementation
- ✅ **Phase 3 Complete**: Integration with Go analyzer
- ✅ **Phase 4 Complete**: End-to-end testing with real repositories
- 🚧 **Phase 5 Pending**: Wire-up verification and documentation

## Quick Start

```go
import (
    "context"
    "github.com/fulmenhq/goneat/pkg/dependencies"
)

// Create analyzer
analyzer := dependencies.NewGoAnalyzer()

// Configure analysis
cfg := dependencies.AnalysisConfig{
    Target:     "./myproject",
    PolicyPath: ".goneat/dependencies.yaml",
}

// Analyze dependencies
result, err := analyzer.Analyze(context.Background(), cfg.Target, cfg)
if err != nil {
    log.Fatal(err)
}

// Check results
if !result.Passed {
    for _, issue := range result.Issues {
        log.Printf("[%s] %s: %s", issue.Severity, issue.Type, issue.Message)
    }
}
```

## Policy Configuration Example

`.goneat/dependencies.yaml`:

```yaml
version: v1

licenses:
  forbidden:
    - GPL-3.0
    - AGPL-3.0

cooling:
  enabled: true
  min_age_days: 7 # 1 week minimum
  min_downloads: 100 # Minimal popularity
  min_downloads_recent: 10 # Recent activity

  exceptions:
    - pattern: "golang.org/x/*"
      reason: "Go extended standard library"
    - pattern: "github.com/myorg/*"
      reason: "Internal packages"
      until: "2026-12-31"
```

## Testing

### Unit Tests

```bash
# Fast unit tests (no network)
go test ./pkg/dependencies/... -short
```

### Integration Tests

```bash
# Full integration tests with real repositories
go test ./pkg/dependencies/... -tags=integration -v

# Specific scenario
go test ./pkg/dependencies/... -tags=integration -run TestCoolingPolicy_Hugo_Baseline
```

### Benchmarks

```bash
# Performance benchmarks
go test ./pkg/dependencies/... -tags=integration -bench=. -benchmem
```

See [`testing.md`](testing.md) for detailed testing documentation.

## Wave 2 Roadmap

- **Phase 1 (✅ Complete)**: Registry clients with mockable HTTP
- **Phase 2 (✅ Complete)**: Cooling policy checker implementation
- **Phase 3 (✅ Complete)**: Multi-language analyzer integration
- **Phase 4 (✅ Complete)**: End-to-end testing with real repositories
- **Phase 5 (Pending)**: Wire-up verification and final documentation

## See Also

- [Registry Package Documentation](../registry.md) - Registry client architecture
- [OPA v1 Migration ADR](../../architecture/decisions/adr-0001-opa-v1-rego-v1-migration.md) - Policy engine decision
- [Dependencies Command](../../../user-guide/commands/dependencies.md) - CLI usage
- [Dependency Policy Configuration](../../../configuration/dependency-policy.md) - Policy syntax

## References

- Wave 2 Spec: `.plans/active/v0.3.0/wave-2-detailed-spec.md`
- Phase 4 Plan: `.plans/active/v0.3.0/wave-2-phase-4-plan.md`
- OPA Documentation: https://www.openpolicyagent.org/docs/latest/
- go-licenses: https://github.com/google/go-licenses

---

**Last Updated**: October 10, 2025
**Status**: Wave 2 Phase 4 Complete
