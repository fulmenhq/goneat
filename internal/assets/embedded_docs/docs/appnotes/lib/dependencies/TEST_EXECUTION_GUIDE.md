# Phase 4 Test Execution Guide

**Quick Start Guide for Running Wave 2 Phase 4 Integration Tests**

## Prerequisites

### Quick Start (CI-Friendly)

**No setup required!** Use the synthetic fixture test:

```bash
make test-integration-cooling-synthetic
```

This test uses a controlled fixture in `tests/fixtures/dependencies/` and works in any environment without external repository clones.

---

### Full Test Suite (Optional)

For comprehensive testing with real repositories:

#### 1. Set Environment Variable (Recommended)

```bash
# Option A: Custom location
export GONEAT_COOLING_TEST_ROOT=/path/to/test/repos

# Option B: Use default (~/ dev/playground)
# No export needed - tests will check this location automatically
```

#### 2. Clone Test Repositories

```bash
# If using custom location
mkdir -p $GONEAT_COOLING_TEST_ROOT
cd $GONEAT_COOLING_TEST_ROOT

# If using default
mkdir -p ~/dev/playground
cd ~/dev/playground

# Required for Phase 4 tests
git clone https://github.com/gohugoio/hugo.git
git clone https://github.com/mattermost/mattermost-server.git
git clone https://github.com/traefik/traefik.git
git clone https://github.com/open-policy-agent/opa.git
```

#### 3. Verify Go Environment

```bash
go version  # Should be Go 1.21+
cd /path/to/goneat
go mod download
```

## Running Tests

### Synthetic Fixture Test (CI-Friendly) ⚡

**Recommended for CI/CD** - No external repos required:

```bash
# Using Makefile
make test-integration-cooling-synthetic

# Or directly with go test
go test ./pkg/dependencies/... -tags=integration -run TestCoolingPolicy_Synthetic -v
```

**Expected Output:**
```
=== RUN   TestCoolingPolicy_Synthetic_Baseline
    integration_test.go:534: Synthetic fixture: 7 dependencies, 0 cooling violations
--- PASS: TestCoolingPolicy_Synthetic_Baseline (2.15s)
PASS
```

**Why use synthetic?**
- ✅ No repository setup required
- ✅ Fast (< 5 seconds)
- ✅ Deterministic results
- ✅ Works in any CI/CD environment

---

### Quick Test (Single Real Repository)

```bash
# Using Makefile (with helpful warnings)
make test-integration-cooling-quick

# Or directly with go test
go test ./pkg/dependencies/... -tags=integration -run TestCoolingPolicy_Hugo_Baseline -v
```

**Expected Output:**
```
=== RUN   TestCoolingPolicy_Hugo_Baseline
    integration_test.go:168: Analyzed 82 dependencies in 8.234s
    integration_test.go:169: Found 3 cooling violations
--- PASS: TestCoolingPolicy_Hugo_Baseline (8.23s)
PASS
```

**If repos not configured:**
```
--- SKIP: TestCoolingPolicy_Hugo_Baseline (0.00s)
    integration_test.go:63: Test repository not configured. Set GONEAT_COOLING_TEST_ROOT or clone repos to ~/dev/playground
```

### All Scenarios (Full Test Suite)

```bash
# Using Makefile
make test-integration-cooling

# Or directly with go test
go test ./pkg/dependencies/... -tags=integration -v -timeout=10m
```

**Tests Run:**
1. `TestCoolingPolicy_Hugo_Baseline` - Happy path
2. `TestCoolingPolicy_Mattermost_Strict` - Violation detection
3. `TestCoolingPolicy_Traefik_Exceptions` - Pattern matching
4. `TestCoolingPolicy_OPA_TimeLimited` - Time-limited exceptions
5. `TestCoolingPolicy_RegistryFailure_Graceful` - Error handling
6. `TestCoolingPolicy_CachePerformance` - Cache validation
7. `TestCoolingPolicy_Disabled` - Control test

### Individual Scenarios

```bash
# Scenario 1: Baseline (Hugo)
go test ./pkg/dependencies/... -tags=integration -run TestCoolingPolicy_Hugo_Baseline -v

# Scenario 2: Strict Policy (Mattermost)
go test ./pkg/dependencies/... -tags=integration -run TestCoolingPolicy_Mattermost_Strict -v

# Scenario 3: Exceptions (Traefik)
go test ./pkg/dependencies/... -tags=integration -run TestCoolingPolicy_Traefik_Exceptions -v

# Scenario 4: Time-Limited (OPA)
go test ./pkg/dependencies/... -tags=integration -run TestCoolingPolicy_OPA_TimeLimited -v

# Scenario 5: Registry Failure
go test ./pkg/dependencies/... -tags=integration -run TestCoolingPolicy_RegistryFailure_Graceful -v

# Scenario 6: Cache Performance
go test ./pkg/dependencies/... -tags=integration -run TestCoolingPolicy_CachePerformance -v
```

### Performance Benchmarks

```bash
# Run all benchmarks
go test ./pkg/dependencies/... -tags=integration -bench=. -benchmem

# Save baseline
go test ./pkg/dependencies/... -tags=integration -bench=. -benchmem > benchmark-baseline.txt

# Compare with previous baseline (if available)
benchstat benchmark-baseline.txt benchmark-new.txt
```

**Expected Benchmark Results:**
```
BenchmarkCoolingPolicy_Hugo-8                1    8234567890 ns/op  (~8s)
BenchmarkCoolingPolicy_OPA-8                 1    6123456789 ns/op  (~6s)
BenchmarkCoolingPolicy_Traefik-8             1   12345678901 ns/op  (~12s)
BenchmarkCoolingPolicy_Mattermost-8          1   25678901234 ns/op  (~25s)
```

## Interpreting Results

### Success Indicators ✅

**Hugo Baseline Test:**
- Analyzed 80-100 dependencies
- < 10% violation rate (< 10 violations)
- Completed in < 15 seconds
- All violations have clear messages

**Mattermost Strict Test:**
- Many violations (> 25% of packages)
- Each violation contains "minimum:" threshold
- No missing violation fields

**Traefik Exceptions Test:**
- Zero false positives on exempted patterns
- Patterns checked:
  - `github.com/traefik/*`
  - `golang.org/x/*`
  - `github.com/spf13/*`
  - `github.com/stretchr/*`

**OPA Time-Limited Test:**
- Zero violations for `github.com/open-policy-agent/*` (valid until 2030)
- Zero violations for `github.com/spf13/*` (valid until 2027)

**Registry Failure Test:**
- No crashes on registry errors
- Conservative fallback: `age_days: 365`
- `age_unknown: true` flag set

**Cache Performance Test:**
- Warm cache >= 1.5x faster than cold
- Same dependency count on both runs
- Log output shows speedup (e.g., "3.24x")

### Failure Scenarios ❌

**Test Skipped:**
```
--- SKIP: TestCoolingPolicy_Hugo_Baseline (0.00s)
    integration_test.go:146: Test repository not found: hugo (please clone it to ~/dev/playground)
```
**Solution**: Clone missing repository

**Timeout:**
```
panic: test timed out after 10m0s
```
**Solution**: Increase timeout: `-timeout=20m`

**High Violation Rate:**
```
    integration_test.go:174: High violation rate: 25.00% (expected < 10%)
```
**Possible Causes**:
- Network issues causing registry failures
- Recent package updates (packages are now < 7 days old)
- Check violation messages for actual causes

**Cache Not Working:**
```
    integration_test.go:411: Warm cache should be faster than cold (cold=8.5s, warm=8.7s)
```
**Debug**: Check registry client caching implementation

## Common Issues

### Issue: go: build constraints exclude all Go files

**Error:**
```
go: no buildable Go source files in pkg/dependencies
```

**Solution**: Add `-tags=integration` flag:
```bash
go test ./pkg/dependencies/... -tags=integration -v
```

### Issue: Repository not found

**Error:**
```
--- SKIP: TestCoolingPolicy_Hugo_Baseline (0.00s)
    integration_test.go:63: Test repository not configured. Set GONEAT_COOLING_TEST_ROOT or clone repos to ~/dev/playground
```

**Solution A**: Use synthetic fixture instead (no repos needed):
```bash
make test-integration-cooling-synthetic
```

**Solution B**: Clone repository:
```bash
# Using custom location
export GONEAT_COOLING_TEST_ROOT=/path/to/repos
mkdir -p $GONEAT_COOLING_TEST_ROOT
cd $GONEAT_COOLING_TEST_ROOT
git clone https://github.com/gohugoio/hugo.git

# Or using default location
cd ~/dev/playground
git clone https://github.com/gohugoio/hugo.git
```

### Issue: Network timeout

**Error:**
```
Get "https://proxy.golang.org/...": context deadline exceeded
```

**Solutions:**
1. Check internet connection
2. Increase timeout: `-timeout=20m`
3. Run tests when network is stable
4. Some failures are expected (tests verify graceful handling)

### Issue: Rate limiting

**Error:**
```
registry returned 429 for package
```

**Solution**:
- Wait a few minutes and retry
- This is expected behavior (tests verify handling)
- Cache should reduce subsequent requests

## Quick Validation Checklist

After running full test suite, verify:

- [ ] All 7 tests passed (6 scenarios + 1 control)
- [ ] No test crashes or panics
- [ ] Hugo test: < 10% violations
- [ ] Mattermost test: > 25% violations (strict policy)
- [ ] Traefik test: 0 false positives
- [ ] OPA test: 0 violations for exempted packages
- [ ] Registry failure test: graceful fallback verified
- [ ] Cache test: speedup >= 1.5x
- [ ] Disabled test: 0 violations

## Performance Targets

### Test Execution Time

- **Hugo**: < 15s
- **OPA**: < 10s
- **Traefik**: < 20s
- **Mattermost**: < 30s
- **Full Suite**: < 2 minutes (first run)
- **Full Suite**: < 1 minute (cached)

### Repository Sizes

- **Hugo**: ~80 dependencies (small-medium)
- **OPA**: ~60 dependencies (medium)
- **Traefik**: ~100 dependencies (large)
- **Mattermost**: ~200 dependencies (very large)

## Makefile Integration

Already included in `Makefile`:

```makefile
.PHONY: test-integration-cooling-synthetic
test-integration-cooling-synthetic:
	@echo "Running cooling policy integration test (synthetic fixture)..."
	$(GOTEST) ./pkg/dependencies/... -tags=integration -run TestCoolingPolicy_Synthetic -v -timeout=5m

.PHONY: test-integration-cooling
test-integration-cooling:
	@echo "Running cooling policy integration tests..."
	@echo "⚠️  This requires test repositories. Set GONEAT_COOLING_TEST_ROOT or clone repos to ~/dev/playground"
	$(GOTEST) ./pkg/dependencies/... -tags=integration -v -timeout=15m

.PHONY: test-integration-cooling-quick
test-integration-cooling-quick:
	@echo "Running quick cooling policy test (Hugo baseline)..."
	@echo "⚠️  This requires Hugo repository. Set GONEAT_COOLING_TEST_ROOT or clone to ~/dev/playground"
	$(GOTEST) ./pkg/dependencies/... -tags=integration -run TestCoolingPolicy_Hugo_Baseline -v -timeout=5m
```

Usage:
```bash
make test-integration-cooling-synthetic  # CI-friendly (no repos needed)
make test-integration-cooling           # Full suite (requires repos)
make test-integration-cooling-quick     # Quick validation (Hugo only)
```

## CI/CD Integration

### Minimal CI Configuration

```yaml
# .github/workflows/integration.yml
name: Integration Tests

on:
  push:
    branches: [main]
  pull_request:

jobs:
  integration:
    runs-on: ubuntu-latest
    timeout-minutes: 20
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Setup test repositories
        run: |
          mkdir -p ~/dev/playground
          cd ~/dev/playground
          git clone --depth=1 https://github.com/gohugoio/hugo.git &
          git clone --depth=1 https://github.com/open-policy-agent/opa.git &
          wait

      - name: Run integration tests
        run: make test-integration-phase4
```

## Next Steps After Testing

Once all tests pass:

1. **Document Results**: Save test output for Phase 4 completion report
2. **Update Wave 2 Spec**: Mark Phase 4 as complete
3. **Performance Baseline**: Save benchmark results for future comparison
4. **Create Commit**: Commit Phase 4 implementation with test results

---

**Questions?** See [`testing.md`](testing.md) for detailed documentation.

**Generated by Code Scout ([Claude Code](https://claude.ai/code)) under supervision of @3leapsdave**
