# Versioning Package Test Fixtures

This directory contains comprehensive test fixtures for the `pkg/versioning` package, designed to simulate real-world version comparison and policy evaluation scenarios.

## Directory Structure

```
tests/fixtures/versioning/
├── semver-full/              # Full SemVer test data (with prerelease & build metadata)
│   └── versions.txt          # Comprehensive semver formats for full mode
├── semver-compact/           # Compact SemVer test data (x.y.z only)
│   └── versions.txt          # Simple semver formats for compact mode
├── calver/                    # Calendar versioning test data
│   └── versions.txt          # Strict YYYY.MM/YYYY.MM.DD formats only
├── lexical/                   # Lexical versioning test data
│   └── versions.txt          # Various lexical formats for testing
├── policies/                  # Version policy configuration files
│   ├── golangci-policy.yaml  # golangci-lint version policy
│   ├── go-policy.yaml        # Go toolchain version policy
│   └── calver-policy.yaml    # Calendar versioning policy example
├── integration/               # Integration test scenarios
│   ├── tool-evaluation-scenarios.yaml  # Complete tool evaluation workflows
│   └── version-comparison-matrix.yaml  # Comprehensive comparison tests
└── README.md                 # This file
```

## Fixture Categories

### Version Format Fixtures

The `versions.txt` files in each scheme directory contain various version formats for comprehensive testing:

- **semver-full/versions.txt**: Complete SemVer 2.0.0 compliance with prerelease, build metadata, natural sorting
- **semver-compact/versions.txt**: Simple x.y.z format only, rejects prerelease and build metadata
- **calver/versions.txt**: Strict YYYY.MM/YYYY.MM.DD formats only, rejects invalid dates and mixed separators
- **lexical/versions.txt**: Case variations, whitespace, special characters, unicode

### Policy Configuration Fixtures

The `policies/` directory contains real-world policy configurations:

- **golangci-policy.yaml**: Based on actual golangci-lint version requirements
- **go-policy.yaml**: Go toolchain version constraints
- **calver-policy.yaml**: Calendar versioning example

### Integration Test Scenarios

The `integration/` directory contains comprehensive test scenarios:

- **tool-evaluation-scenarios.yaml**: Complete tool evaluation workflows with expected outcomes
- **version-comparison-matrix.yaml**: Cross-scheme comparison tests and edge cases

## Usage in Tests

### Basic Fixture Testing

```go
// Test using version fixtures
func TestVersionFixtures(t *testing.T) {
    fixtureDir := filepath.Join("..", "..", "tests", "fixtures", "versioning")

    t.Run("semver_fixtures", func(t *testing.T) {
        testVersionFile(t, filepath.Join(fixtureDir, "semver", "versions.txt"), SchemeSemver)
    })
}
```

### Policy Evaluation Testing

```go
// Test using policy fixtures
func TestPolicyFixtures(t *testing.T) {
    policyFile := filepath.Join("..", "..", "tests", "fixtures", "versioning", "policies", "golangci-policy.yaml")

    data, err := os.ReadFile(policyFile)
    // Parse and test policy evaluation
}
```

### Integration Scenario Testing

```go
// Test complete workflows
func TestToolEvaluationScenarios(t *testing.T) {
    scenariosFile := filepath.Join("..", "..", "tests", "fixtures", "versioning", "integration", "tool-evaluation-scenarios.yaml")

    // Load and execute comprehensive test scenarios
}
```

## Test Coverage

These fixtures ensure comprehensive test coverage for:

### Version Comparison
- ✅ Happy-path ordering for all schemes
- ✅ Invalid input handling and error reporting
- ✅ SemVer full mode: prerelease ordering (alpha < beta < rc), build metadata ignored
- ✅ SemVer compact mode: simple x.y.z progression only
- ✅ CalVer strict mode: YYYY.MM/YYYY.MM.DD only, rejects mixed separators
- ✅ Natural sorting: rc.2 < rc.11, beta.2 < beta.11
- ✅ Prefix handling (v prefix for semver)
- ✅ Large number support (>= v10.20.30)
- ✅ Build metadata and pre-release stripping (full mode)
- ✅ Separator variants (-, _, . for calver with consistency)
- ✅ Numeric overflow detection
- ✅ Case sensitivity in lexical comparisons
- ✅ Whitespace trimming
- ✅ Unicode character handling

### Policy Evaluation
- ✅ Minimum version only policies
- ✅ Recommended version only policies
- ✅ Both minimum and recommended policies
- ✅ Disallowed version lists
- ✅ Disallowed precedence over minimum
- ✅ Mixed scheme policies
- ✅ Zero-policy fast path
- ✅ Empty version handling
- ✅ Invalid scheme handling

### Tool Schema Integration
- ✅ VersionPolicy() method extraction
- ✅ Config merge with policy preservation
- ✅ Roundtrip parsing and validation
- ✅ Schema validation compliance
- ✅ Tool scope resolution

### Cross-Reference Validation
- ✅ Consistency with cmd/version.go bump logic
- ✅ Alignment with latestSemverTag patterns
- ✅ Compatibility with latestCalverTag detection
- ✅ Regression testing for existing behavior

## Real-World Scenarios

The fixtures simulate actual tool ecosystems:

- **golangci-lint**: Real version constraints and update patterns
- **Go toolchain**: Version compatibility requirements
- **Calendar versioning**: Date-based release patterns
- **Mixed schemes**: Projects using different versioning strategies

## Maintenance

When adding new fixtures:

1. **Version formats**: Add to appropriate `versions.txt` file
2. **Policy examples**: Create new YAML files in `policies/`
3. **Integration scenarios**: Update `tool-evaluation-scenarios.yaml`
4. **Comparison tests**: Update `version-comparison-matrix.yaml`

## Integration with CI/CD

These fixtures are designed to work with:

- **Unit tests**: Basic functionality validation
- **Integration tests**: Real-world scenario simulation
- **Regression tests**: Ensuring compatibility with cmd/version.go
- **Fuzz testing**: Edge case discovery
- **Performance testing**: Large version set handling

The fixtures provide a solid foundation for maintaining version governance reliability across the goneat ecosystem.