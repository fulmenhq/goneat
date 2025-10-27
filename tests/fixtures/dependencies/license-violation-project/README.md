# License Violation Test Fixture (Deprecated)

⚠️ **This fixture is deprecated.** The integration tests now use the `synthetic-go-project` fixture with dynamic policy configuration for more flexible testing.

This fixture was originally designed to demonstrate license policy enforcement, but has been replaced by a more maintainable approach that uses existing fixtures with runtime policy modification.

## Legacy Information

This project was designed to include `github.com/apache/thrift v0.21.0` (Apache-2.0 licensed) with a policy that forbids Apache-2.0 licenses. However, dependency resolution issues made this unreliable for CI/CD environments.

## Current Approach

Integration tests now use `synthetic-go-project` with dynamically created policies that forbid existing licenses (like MIT) to create violations. This approach:

- ✅ **Reliable**: Uses existing, well-tested dependencies
- ✅ **Maintainable**: No external dependency resolution required
- ✅ **Flexible**: Can test different violation scenarios by changing policies
- ✅ **Fast**: Dependencies are already cached locally

## Migration

If you need to test license violations, use the synthetic project approach:

```go
// Create policy that forbids MIT licenses
policyContent := `version: v1
licenses:
  forbidden:
    - MIT  # Causes testify and yaml.v3 to fail
  allowed:
    - BSD-3-Clause
cooling:
  enabled: false
`
```

---

**Status**: Deprecated - Use synthetic-go-project with dynamic policies instead
**Last Updated**: October 23, 2025