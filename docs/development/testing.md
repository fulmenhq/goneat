# Testing Guide

This guide covers testing practices, parallel test execution, and performance optimization for the goneat test suite.

## Table of Contents

- [Running Tests](#running-tests)
- [Parallel Test Execution](#parallel-test-execution)
- [Writing Parallel-Safe Tests](#writing-parallel-safe-tests)
- [Performance Benchmarks](#performance-benchmarks)
- [Platform-Specific Considerations](#platform-specific-considerations)

---

## Running Tests

### Basic Commands

```bash
# Run all tests
make test

# Run only unit tests
make test-unit

# Run integration tests
make test-integration

# Run precommit (includes tests, linting, formatting)
make precommit
```

### Test Timeouts

Tests have a 15-minute timeout by default to accommodate slower platforms (especially Windows with anti-malware scanning):

```bash
# Default timeout (15m)
go test ./... -v

# Custom timeout
go test ./... -v -timeout 30m
```

---

## Parallel Test Execution

### Overview

Go tests run **sequentially by default** within a package. To enable parallel execution, tests must explicitly call `t.Parallel()`.

The `-parallel N` flag sets the maximum number of tests that can run concurrently, but only affects tests that opt-in via `t.Parallel()`.

### Using GONEAT_TEST_PARALLEL

The test suite supports configurable parallelization via the `GONEAT_TEST_PARALLEL` variable:

```bash
# Default (sequential)
make test-unit

# Run with 3 parallel tests (environment variable)
export GONEAT_TEST_PARALLEL=3
make test-unit

# Run with 3 parallel tests (command line)
make test-unit GONEAT_TEST_PARALLEL=3
```

**Default:** `1` (sequential execution for CI stability)

**Recommended for local development:**
- **macOS/Linux:** `4-8` (depending on CPU cores)
- **Windows:** `3-4` (anti-malware scanning creates I/O bottlenecks)

---

## Writing Parallel-Safe Tests

### The t.Parallel() Pattern

To make a test run in parallel, add `t.Parallel()` as the **first statement** in the test function:

```go
func TestMyFeature(t *testing.T) {
	t.Parallel()  // ← Add this line

	// Rest of test code...
}
```

### Example: Before and After

**Before (Sequential):**
```go
func TestPackageManagerDetection(t *testing.T) {
	config, err := LoadPackageManagersConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	// ... rest of test
}
```

**After (Parallel):**
```go
func TestPackageManagerDetection(t *testing.T) {
	t.Parallel()  // ← Added

	config, err := LoadPackageManagersConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	// ... rest of test
}
```

### Requirements for Parallel Tests

Tests can safely use `t.Parallel()` if they:

1. ✅ **Do not modify global state** (package-level variables, environment, etc.)
2. ✅ **Do not rely on execution order** (each test is independent)
3. ✅ **Use unique resources** (different temp directories, ports, files)
4. ✅ **Are deterministic** (same inputs → same outputs)

### Anti-Patterns to Avoid

❌ **Shared State:**
```go
var globalCounter int  // ← Bad: shared across tests

func TestIncrement(t *testing.T) {
	t.Parallel()
	globalCounter++  // ← Race condition!
}
```

❌ **Fixed Ports/Paths:**
```go
func TestServer(t *testing.T) {
	t.Parallel()
	server := startServer(8080)  // ← Bad: port conflicts!
}
```

✅ **Correct Approach:**
```go
func TestServer(t *testing.T) {
	t.Parallel()
	port := getAvailablePort(t)  // ← Good: dynamic port
	server := startServer(port)
}
```

### Subtests and Parallelization

Subtests can also run in parallel:

```go
func TestFeatures(t *testing.T) {
	t.Parallel()  // Parent test runs in parallel

	tests := []struct {
		name string
		input string
		want string
	}{
		{"case1", "input1", "output1"},
		{"case2", "input2", "output2"},
	}

	for _, tt := range tests {
		tt := tt  // Capture range variable
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()  // Subtest also runs in parallel
			// ... test code
		})
	}
}
```

---

## Performance Benchmarks

### Proof-of-Concept: internal/doctor Package

We measured the impact of parallelization on the `internal/doctor` package (69 tests):

| Configuration | Time | Speedup |
|--------------|------|---------|
| Sequential (`-parallel 1`) | 69.8s | baseline |
| Parallel (`-parallel 3`) | 39.0s | **1.79x (44% faster)** |

**Platform:** Windows with anti-malware scanning (worst-case scenario)

**Expected speedup on macOS/Linux:** 2-3x with higher parallelization settings.

### Scaling with Parallelization

| `-parallel` | Expected Speedup | Best For |
|-------------|------------------|----------|
| 1 | 1x (baseline) | CI environments, debugging |
| 3 | 1.5-2x | Windows with I/O constraints |
| 4 | 2-2.5x | 4-core machines |
| 8 | 2.5-3x | 8+ core machines |

**Note:** Speedup plateaus due to:
- I/O bottlenecks (file system, network)
- Test dependencies (some tests can't parallelize)
- Package-level serialization (Go runs packages in parallel, but not all tests within)

---

## Platform-Specific Considerations

### Windows

**Challenges:**
- Anti-malware scanning introduces significant I/O overhead
- Process creation is slower than Unix-like systems
- File locking can cause test flakiness

**Recommendations:**
- Use `-parallel 3` for local development
- Increase test timeouts to 15m+ via Makefile
- Exclude test directories from anti-malware scanning if possible

**Performance:**
- Full test suite: ~400-500 seconds (sequential)
- With parallelization: ~200-250 seconds (`-parallel 3`)

### macOS/Linux

**Advantages:**
- Faster process creation
- Better file system performance
- No anti-malware overhead (typically)

**Recommendations:**
- Use `-parallel 4-8` for local development
- Standard 10m timeout is usually sufficient

---

## Test Infrastructure

### Helper Timeouts

All test helpers include timeouts to prevent hanging:

**Test Commands** (`testenv.go`):
- `RunVersionCommand()`: 30-second timeout
- `runCommand()`: 30-second timeout

**Git Commands** (`fixtures.go`):
- `runGitCommand()`: 30-second timeout

**Package Manager Detection** (`pkg/tools/package_managers.go`):
- `brew --version`: 5-second timeout
- `scoop --version`: 5-second timeout

These timeouts prevent tests from hanging indefinitely on slow or unresponsive operations.

### Cross-Platform Binary Paths

Test helpers automatically detect the correct binary extension:

```go
// tests/integration/testenv.go
func (env *TestEnv) findGoneatBinary() string {
	binaryName := "goneat"
	if runtime.GOOS == "windows" {
		binaryName = "goneat.exe"
	}
	// ... search logic
}
```

---

## Adding Parallel Tests to New Packages

When writing new tests or refactoring existing ones:

1. **Assess parallelizability:** Check for shared state, global variables, fixed resources
2. **Add `t.Parallel()`:** Place as first line in test function
3. **Test locally:** Run with `GONEAT_TEST_PARALLEL=3` to verify no race conditions
4. **Benchmark:** Compare sequential vs parallel execution time
5. **Document:** Note any tests that cannot be parallelized and why

### Automated Conversion Script

To bulk-add `t.Parallel()` to a package:

```bash
# Add t.Parallel() to all test functions in a directory
for file in pkg/mypackage/*_test.go; do
    awk '
    /^func Test.*\(t \*testing\.T\) {$/ {
        print $0
        print "\tt.Parallel()"
        next
    }
    { print }
    ' "$file" > "$file.tmp" && mv "$file.tmp" "$file"
done
```

**⚠️ Warning:** Only use this script after verifying tests are parallel-safe!

---

## References

- [Go Testing Documentation](https://pkg.go.dev/testing)
- [Parallel Test Execution](https://go.dev/blog/subtests#parallel-tests)
- [Crucible Testing Standards](../crucible-go/standards/testing/README.md)
- [Integration Testing Overview](../user-guide/integration-testing-overview.md)
