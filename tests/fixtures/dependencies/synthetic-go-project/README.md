# Synthetic Go Project for Cooling Policy Testing

This is a **controlled test fixture** for validating the cooling policy implementation in CI/CD without requiring external repository clones.

## Purpose

- **CI-friendly**: No need to clone external repos (hugo, opa, etc.)
- **Deterministic**: Dependencies are pinned to specific versions
- **Comprehensive**: Covers common cooling policy scenarios
- **Fast**: Small dependency tree for quick test execution

## Dependencies

This project includes a mix of package types to test various scenarios:

### Mature Packages (Should Pass)
- **github.com/google/uuid** - Widely used, stable
- **github.com/stretchr/testify** - Popular testing library
- **gopkg.in/yaml.v3** - Mature YAML parser

### Standard Library Extensions (Should Pass with Exceptions)
- **golang.org/x/sync** - Go extended library
- **golang.org/x/time** - Go extended library

## Usage

### In Integration Tests

```go
func TestCoolingPolicy_Synthetic(t *testing.T) {
    syntheticPath := "../../tests/fixtures/dependencies/synthetic-go-project"
    analyzer := NewGoAnalyzer()

    cfg := AnalysisConfig{
        Target:     syntheticPath,
        PolicyPath: "testdata/policies/baseline.yaml",
    }

    result, err := analyzer.Analyze(context.Background(), cfg.Target, cfg)
    // ... assertions
}
```

### In CI/CD

This fixture can run in GitHub Actions or other CI environments without cloning external repositories:

```yaml
- name: Run synthetic integration tests
  run: go test ./pkg/dependencies/... -tags=integration -run Synthetic
```

## Maintenance

When updating dependencies:
1. Keep at least one recent package for violation testing
2. Keep mature packages for baseline testing
3. Test with exception patterns (golang.org/x/*)
4. Document any intentional violations

## `.goneatignore`

This fixture is already excluded from goneat coverage analysis via the global `.goneatignore` pattern:
```
fixtures/
```

This prevents the intentionally "bad" dependency configurations from being flagged in goneat's own analysis.
