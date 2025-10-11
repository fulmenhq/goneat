# Dependencies Package Testing Guide

**Status**: Wave 2 Phase 4 Complete
**Last Updated**: October 10, 2025

## Overview

This document describes the comprehensive testing strategy for the Dependencies package, including unit tests, integration tests with real repositories, cache performance validation, and benchmarking.

## Testing Philosophy

The Dependencies package testing follows these principles:

1. **Real-World Validation** - Test with actual Go repositories from `~/dev/playground` instead of artificial fixtures
2. **Cache Performance Verification** - Measure warm vs cold cache behavior to ensure 24-hour TTL works correctly
3. **Scenario Coverage** - Test baseline, strict policies, exceptions, time-limits, failures, and performance
4. **Repository Guardrails** - Skip tests gracefully when test repositories aren't available

## Test Categories

### Unit Tests (Fast, No Network)

```bash
# Run all unit tests with no external dependencies
go test ./pkg/dependencies/... -short -v
```

**Characteristics:**
- No network calls (use mocked HTTP via `MockHTTPFetcher`)
- JSON fixtures in `testdata/` directories
- Fast execution (< 50ms per test)
- Run on every commit in CI/CD

**Example:**
```go
func TestGoAnalyzer_Basic(t *testing.T) {
    analyzer := NewGoAnalyzer()
    // Test with minimal setup
}
```

### Integration Tests (With Real Repositories)

```bash
# Run all integration tests
go test ./pkg/dependencies/... -tags=integration -v

# Run specific scenario
go test ./pkg/dependencies/... -tags=integration -run TestCoolingPolicy_Hugo_Baseline

# Run with timeout for large repos
go test ./pkg/dependencies/... -tags=integration -timeout=10m
```

**Characteristics:**
- Real network calls to registry APIs
- Real Go projects from `~/dev/playground`
- Slower execution (2-30s per test depending on repo size)
- Run nightly or pre-release in CI/CD

**Example:**
```go
//go:build integration
// +build integration

func TestCoolingPolicy_Hugo_Baseline(t *testing.T) {
    hugoPath := os.ExpandEnv("$HOME/dev/playground/hugo")
    checkRepoExists(t, hugoPath)
    // Test with real repository
}
```

### Benchmark Tests (Performance Baselines)

```bash
# Run all benchmarks
go test ./pkg/dependencies/... -tags=integration -bench=. -benchmem

# Run specific benchmark
go test ./pkg/dependencies/... -tags=integration -bench=BenchmarkCoolingPolicy_Hugo

# Save baseline for comparison
go test ./pkg/dependencies/... -tags=integration -bench=. -benchmem | tee baseline.txt
```

**Example Output:**
```
BenchmarkCoolingPolicy_Hugo-8           1    8234567890 ns/op    5242880 B/op    98765 allocs/op
BenchmarkCoolingPolicy_Mattermost-8     1   25678901234 ns/op   15728640 B/op   234567 allocs/op
```

## Test Scenarios (Phase 4)

### Scenario 1: Baseline Validation (Happy Path)

**File**: `pkg/dependencies/integration_test.go::TestCoolingPolicy_Hugo_Baseline`

**Purpose**: Verify basic cooling policy works with real dependencies

**Test Repository**: `~/dev/playground/hugo`

**Policy**: `testdata/policies/baseline.yaml` (7 days, 100 downloads, 10 recent)

**Expected Results:**
- Analysis completes successfully
- Most dependencies pass (Hugo uses stable, mature packages)
- < 10% violation rate
- Clear violation messages for any young/unpopular packages
- Performance: < 15s for ~80 dependencies

**Run:**
```bash
go test ./pkg/dependencies/... -tags=integration -run TestCoolingPolicy_Hugo_Baseline -v
```

---

### Scenario 2: Strict Policy (High Thresholds)

**File**: `pkg/dependencies/integration_test.go::TestCoolingPolicy_Mattermost_Strict`

**Purpose**: Trigger violations to verify detection works

**Test Repository**: `~/dev/playground/mattermost-server`

**Policy**: `testdata/policies/strict.yaml` (365 days, 1M downloads, 100K recent)

**Expected Results:**
- Many violations triggered (>25% of packages)
- Each violation has clear message with actual vs. expected values
- Dependencies with real registry data show age/download violations
- Dependencies without registry data (local, vendored) handled gracefully

**Run:**
```bash
go test ./pkg/dependencies/... -tags=integration -run TestCoolingPolicy_Mattermost_Strict -v
```

---

### Scenario 3: Exception Pattern Matching

**File**: `pkg/dependencies/integration_test.go::TestCoolingPolicy_Traefik_Exceptions`

**Purpose**: Verify glob patterns and prefix matching work with real package names

**Test Repository**: `~/dev/playground/traefik` or `~/dev/playground/traefik-assessment`

**Policy**: `testdata/policies/exceptions.yaml` (extremely strict with exceptions)

**Exception Patterns Tested:**
- `github.com/traefik/*` - Organization packages
- `github.com/containous/*` - Old organization name
- `golang.org/x/*` - Go extended standard library
- `github.com/spf13/*` - Trusted maintainer

**Expected Results:**
- Zero false positives on exempted patterns
- Traefik's own packages exempted
- Go extended libs exempted
- Trusted maintainer packages exempted
- Other packages trigger violations

**Run:**
```bash
go test ./pkg/dependencies/... -tags=integration -run TestCoolingPolicy_Traefik_Exceptions -v
```

---

### Scenario 4: Time-Limited Exceptions

**File**: `pkg/dependencies/integration_test.go::TestCoolingPolicy_OPA_TimeLimited`

**Purpose**: Verify time-based exceptions expire correctly

**Test Repository**: `~/dev/playground/opa`

**Policy**: `testdata/policies/time-limited.yaml` (14 days with time-limited exceptions)

**Time-Limited Exceptions Tested:**
- Expired: `until: "2020-01-01"` (should NOT apply)
- Valid: `until: "2030-12-31"` (should apply)
- Edge case: `until: "2024-01-01"` (recently expired)

**Expected Results:**
- OPA org packages exempted (valid until 2030)
- spf13 packages exempted (valid until 2027)
- Expired exceptions don't prevent violations
- Zero violations for currently-valid exception patterns

**Run:**
```bash
go test ./pkg/dependencies/... -tags=integration -run TestCoolingPolicy_OPA_TimeLimited -v
```

---

### Scenario 5: Registry Failure Handling

**File**: `pkg/dependencies/integration_test.go::TestCoolingPolicy_RegistryFailure_Graceful`

**Purpose**: Verify graceful degradation when registry APIs fail

**Test Repository**: `~/dev/playground/hugo`

**Policy**: `testdata/policies/baseline.yaml`

**Expected Results:**
- Analysis doesn't crash on registry errors
- Conservative fallback: `age_days: 365` for failed lookups
- `age_unknown: true` flag set
- `registry_error` metadata contains error message
- Packages with registry errors pass cooling policy (conservative)

**Run:**
```bash
go test ./pkg/dependencies/... -tags=integration -run TestCoolingPolicy_RegistryFailure_Graceful -v
```

---

### Scenario 6: Cache Performance Validation

**File**: `pkg/dependencies/integration_test.go::TestCoolingPolicy_CachePerformance`

**Purpose**: Verify 24-hour TTL behavior and cache speedup

**Test Repository**: `~/dev/playground/hugo`

**Policy**: `testdata/policies/baseline.yaml`

**Cache Timing Instrumentation:**
```go
type CacheTiming struct {
    ColdHits int           // Dependencies found on first run
    WarmHits int           // Dependencies found on second run
    ColdTime time.Duration // Time for cold cache run
    WarmTime time.Duration // Time for warm cache run
}
```

**Expected Results:**
- Warm cache faster than cold (speedup >= 1.5x)
- Same dependency count on both runs
- Registry calls cached (HTTP requests avoided on second run)
- Cache hit rate near 100% on second run

**Run:**
```bash
go test ./pkg/dependencies/... -tags=integration -run TestCoolingPolicy_CachePerformance -v
```

**Example Output:**
```
Cache speedup: 3.24x (cold=8.5s, warm=2.6s)
Dependencies: 82
PASS
```

---

### Bonus: Disabled Cooling (Control Test)

**File**: `pkg/dependencies/integration_test.go::TestCoolingPolicy_Disabled`

**Purpose**: Verify disabled cooling passes all packages

**Policy**: `testdata/policies/disabled.yaml` (cooling enabled: false)

**Expected Results:**
- Zero cooling violations
- All packages pass regardless of age/downloads

**Run:**
```bash
go test ./pkg/dependencies/... -tags=integration -run TestCoolingPolicy_Disabled -v
```

---

## Performance Benchmarks

### Repository Size Benchmarks

**File**: `pkg/dependencies/integration_bench_test.go`

```bash
# Benchmark all repository sizes
go test ./pkg/dependencies/... -tags=integration -bench=BenchmarkCoolingPolicy_ -benchmem

# Results:
# BenchmarkCoolingPolicy_Hugo         (small-medium, ~80 deps)
# BenchmarkCoolingPolicy_OPA          (medium, ~60 deps)
# BenchmarkCoolingPolicy_Traefik      (large, ~100 deps)
# BenchmarkCoolingPolicy_Mattermost   (very large, ~200 deps)
```

**Expected Performance Targets:**
- Small repo (< 50 deps): < 5s
- Medium repo (50-100 deps): < 10s
- Large repo (100-200 deps): < 20s

### Policy Overhead Benchmarks

```bash
# Compare policy overhead
go test ./pkg/dependencies/... -tags=integration -bench=BenchmarkCoolingPolicy_StrictPolicy
go test ./pkg/dependencies/... -tags=integration -bench=BenchmarkCoolingPolicy_Exceptions
go test ./pkg/dependencies/... -tags=integration -bench=BenchmarkCoolingPolicy_Disabled
```

**Purpose**: Measure overhead of different policy configurations

## Helper Functions

### checkRepoExists

```go
func checkRepoExists(t *testing.T, path string) {
    t.Helper()
    if _, err := os.Stat(path); os.IsNotExist(err) {
        t.Skipf("Test repository not found: %s (please clone it to ~/dev/playground)",
                filepath.Base(path))
    }
}
```

**Purpose**: Gracefully skip tests when repositories aren't available

### countCoolingViolations

```go
func countCoolingViolations(issues []Issue) int {
    count := 0
    for _, issue := range issues {
        if issue.Type == "age_violation" || issue.Type == "download_violation" {
            count++
        }
    }
    return count
}
```

**Purpose**: Count cooling-specific violations (age + downloads)

### validateViolationStructure

```go
func validateViolationStructure(t *testing.T, issues []Issue) {
    t.Helper()
    for i, issue := range issues {
        if issue.Type == "age_violation" || issue.Type == "download_violation" {
            if issue.Message == "" {
                t.Errorf("Issue %d: missing message", i)
            }
            if issue.Severity == "" {
                t.Errorf("Issue %d: missing severity", i)
            }
            if issue.Dependency == nil {
                t.Errorf("Issue %d: missing dependency reference", i)
            }
        }
    }
}
```

**Purpose**: Ensure all violations have required fields

### measureCachePerformance

```go
func measureCachePerformance(t *testing.T, analyzer Analyzer, cfg AnalysisConfig) *CacheTiming {
    // Run analysis twice (cold then warm cache)
    // Measure timing and dependency counts
    // Log speedup metrics
    return &CacheTiming{...}
}
```

**Purpose**: Measure cold vs warm cache performance (Arch Eagle suggestion)

## Test Repository Setup

### Environment Variable Configuration

Integration tests use the **`GONEAT_COOLING_TEST_ROOT`** environment variable to locate test repositories:

```bash
# Set custom location
export GONEAT_COOLING_TEST_ROOT=/path/to/test/repos

# Or use default (~/dev/playground)
# No export needed - tests will check this location automatically
```

**Behavior**:
- If `GONEAT_COOLING_TEST_ROOT` is set, tests look for repos there
- If not set, tests default to `~/dev/playground`
- If repos are not found, tests **skip gracefully** (no failures)

### Required Repositories

Clone these to `$GONEAT_COOLING_TEST_ROOT` (or `~/dev/playground`) for full test coverage:

```bash
cd ~/dev/playground

# Currently used in Phase 4
git clone https://github.com/gohugoio/hugo.git
git clone https://github.com/mattermost/mattermost-server.git
git clone https://github.com/traefik/traefik.git
git clone https://github.com/open-policy-agent/opa.git

# Optional for additional coverage
git clone https://github.com/envoyproxy/envoy.git
```

### Future Wave 3 (Multi-Language)

**TypeScript/JavaScript:**
```bash
git clone https://github.com/microsoft/vscode.git
git clone https://github.com/grafana/grafana.git
git clone https://github.com/vercel/next.js.git
```

**Python:**
```bash
git clone https://github.com/pallets/flask.git
git clone https://github.com/django/django.git
git clone https://github.com/psf/requests.git
```

**Rust:**
```bash
git clone https://github.com/rust-lang/cargo.git
git clone https://github.com/BurntSushi/ripgrep.git
git clone https://github.com/tokio-rs/tokio.git
```

**C#/.NET:**
```bash
git clone https://github.com/dotnet/runtime.git
git clone https://github.com/dotnet/aspnetcore.git
git clone https://github.com/JamesNK/Newtonsoft.Json.git
```

## CI/CD Integration

### Recommended CI Strategy

**For CI/CD**: Use the **synthetic fixture test** (fast, deterministic, no external repos):

```yaml
# .github/workflows/test.yml
name: Tests

on: [push, pull_request]

jobs:
  unit-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
      - name: Run unit tests
        run: go test ./pkg/dependencies/... -short -v

  integration-synthetic:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
      - name: Run synthetic integration test (CI-friendly)
        run: make test-integration-cooling-synthetic
```

### Optional: Full Integration Tests in CI

**For nightly/release testing** with real repositories:

```yaml
  integration-full:
    runs-on: ubuntu-latest
    if: github.event_name == 'schedule' || github.ref == 'refs/heads/main'
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4

      - name: Clone test repositories
        run: |
          mkdir -p /tmp/test-repos
          cd /tmp/test-repos
          git clone --depth=1 https://github.com/gohugoio/hugo.git
          git clone --depth=1 https://github.com/open-policy-agent/opa.git

      - name: Run full integration tests
        env:
          GONEAT_COOLING_TEST_ROOT: /tmp/test-repos
        run: make test-integration-cooling
```

### Makefile Targets

```makefile
# Fast unit tests (run on every commit)
.PHONY: test-unit
test-unit:
	go test ./pkg/dependencies/... -short -v

# Synthetic fixture test (CI-friendly, no external repos)
.PHONY: test-integration-cooling-synthetic
test-integration-cooling-synthetic:
	go test ./pkg/dependencies/... -tags=integration -run TestCoolingPolicy_Synthetic -v

# Full integration tests (requires GONEAT_COOLING_TEST_ROOT)
.PHONY: test-integration-cooling
test-integration-cooling:
	go test ./pkg/dependencies/... -tags=integration -v -timeout=15m

# Quick integration test (Hugo only)
.PHONY: test-integration-cooling-quick
test-integration-cooling-quick:
	go test ./pkg/dependencies/... -tags=integration -run TestCoolingPolicy_Hugo_Baseline -v

# Benchmarks
.PHONY: bench
bench:
	go test ./pkg/dependencies/... -tags=integration -bench=. -benchmem
```

**Usage**:
```bash
# CI/CD (always works)
make test-integration-cooling-synthetic

# Local development (requires repos)
export GONEAT_COOLING_TEST_ROOT=~/dev/playground
make test-integration-cooling

# Quick check
make test-integration-cooling-quick
```

## Acceptance Criteria (Phase 4)

### Must Pass (Blocking) ✅

1. ✅ All 6 test scenarios pass with real repositories
2. ✅ Hugo baseline test completes in < 15s with < 10% violations
3. ✅ Mattermost strict test triggers violations correctly
4. ✅ Traefik exception test has zero false positives on exempted patterns
5. ✅ OPA time-limited test correctly applies/expires exceptions
6. ✅ Registry failure test doesn't crash analyzer
7. ✅ Cache performance test shows >= 1.5x speedup
8. ✅ All tests compile cleanly
9. ✅ Helper functions well-tested

### Should Pass (Non-Blocking) ⚠️

1. ⚠️ Cache hit rate > 90% on second run (measured via cache timing)
2. ⚠️ Graceful handling of rate limits (429 responses)
3. ⚠️ Clear error messages for network timeouts
4. ⚠️ Benchmark performance meets targets

## Troubleshooting

### Test Repository Not Found

**Error**: `Test repository not found: hugo (please clone it to ~/dev/playground)`

**Solution**:
```bash
cd ~/dev/playground
git clone https://github.com/gohugoio/hugo.git
```

### Registry Timeout

**Error**: `context deadline exceeded` or `network timeout`

**Solution**: Increase timeout or skip integration tests:
```bash
# Increase timeout
go test ./pkg/dependencies/... -tags=integration -timeout=30m

# Skip integration tests
go test ./pkg/dependencies/... -short
```

### Cache Not Working

**Symptom**: Warm cache not faster than cold cache

**Debug**:
```go
// Add logging to registry client GetMetadata
log.Printf("Cache key: %s, cached: %t", key, ok && time.Now().Before(entry.expiry))
```

### False Positives on Exceptions

**Symptom**: Exempted packages still flagged

**Debug**: Check pattern matching:
```go
// In cooling/checker.go
log.Printf("Testing pattern '%s' against package '%s': %t", pattern, pkgName, matched)
```

## Best Practices

### 1. Always Use checkRepoExists

```go
func TestMyIntegrationTest(t *testing.T) {
    repoPath := os.ExpandEnv("$HOME/dev/playground/myrepo")
    checkRepoExists(t, repoPath)  // Skip if not found
    // ... test code
}
```

### 2. Use testing.Short() for Fast/Slow Split

```go
func TestCoolingPolicy_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test in short mode")
    }
    // ... slow test code
}
```

### 3. Set Reasonable Timeouts

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
result, err := analyzer.Analyze(ctx, ...)
```

### 4. Validate Violation Structure

```go
// Don't just count violations - validate their structure
validateViolationStructure(t, result.Issues)
```

### 5. Log Performance Metrics

```go
t.Logf("Analyzed %d dependencies in %v", len(result.Dependencies), result.Duration)
t.Logf("Cache speedup: %.2fx", speedup)
```

## References

- Phase 4 Plan: `.plans/active/v0.3.0/wave-2-phase-4-plan.md`
- Wave 2 Spec: `.plans/active/v0.3.0/wave-2-detailed-spec.md`
- Integration Test File: `pkg/dependencies/integration_test.go`
- Benchmark File: `pkg/dependencies/integration_bench_test.go`
- Test Policies: `pkg/dependencies/testdata/policies/`

---

**Generated by Code Scout ([Claude Code](https://claude.ai/code)) under supervision of @3leapsdave**

🔍 Task Execution & Assessment Expert
