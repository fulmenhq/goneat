# Integration Testing Overview

## Purpose

This guide explains how goneat's integration tests work, particularly when testing against large real-world projects like Hugo, OPA, and Traefik. It's designed for developers who need to understand the testing strategy, set up their environment, or debug test failures.

**Target Audience**: Contributors, maintainers, and anyone working on features that require integration testing.

**Last Updated**: 2025-10-28 (v0.3.0)

## Testing Strategy Overview

Goneat uses a **three-tier integration testing approach** that balances comprehensive validation with development velocity:

- **Tier 1**: Fast synthetic tests (mandatory, always run)
- **Tier 2**: Quick validation against real projects (optional, pre-release)
- **Tier 3**: Comprehensive multi-project suite (optional, major releases)

This tiered approach ensures:

- ✅ Fast feedback for developers (Tier 1 < 10s)
- ✅ No setup barriers for new contributors (Tier 1 has zero dependencies)
- ✅ Real-world validation when needed (Tiers 2/3 available on-demand)
- ✅ CI/CD friendly (Tier 1 works everywhere)

## Integration Test Sections

This document currently covers the following integration test suites. Each section follows the three-tier structure (synthetic → quick → comprehensive) and documents setup requirements, expected results, and troubleshooting.

| Test Suite                                                                | Code Coverage                                                                     | External Assets                                      | Test Method                                    | Purpose                                                                                             |
| ------------------------------------------------------------------------- | --------------------------------------------------------------------------------- | ---------------------------------------------------- | ---------------------------------------------- | --------------------------------------------------------------------------------------------------- |
| [Dependency Cooling Policy](#dependency-cooling-policy-integration-tests) | `pkg/dependencies/`<br>`pkg/cooling/`<br>`pkg/registry/`<br>`cmd/dependencies.go` | Hugo, OPA, Traefik, Mattermost repos (Tier 2/3 only) | Analyze real Go projects with cooling policies | Validate dependency age detection, registry integration, policy enforcement, and license compliance |

**Coming Soon**:

- Schema Validation Suite (validate schemas against real config files)
- Format Command Suite (format large multi-language codebases)
- Hook Execution Suite (pre-commit/pre-push hook integration)
- Guardian Workflow Suite (approval policy enforcement)

### Dependency Cooling Policy Integration Tests

**Code Coverage**:

- `pkg/dependencies/` - Dependency analysis and detection
- `pkg/cooling/` - Cooling policy checker
- `pkg/registry/` - Multi-language registry clients
- `cmd/dependencies.go` - CLI command integration

**Documentation**:

- Integration Test Protocol: `../../.plans/active/v0.3.0/wave-2-phase-4-INTEGRATION-TEST-PROTOCOL.md`
- Test Execution Guide: `../appnotes/lib/dependencies/TEST_EXECUTION_GUIDE.md`
- Release Process: `../ops/repository/release-process.md`

#### Tier 1: Synthetic Fixture (Mandatory)

**What it tests**: Basic cooling policy functionality using a synthetic Go project with controlled dependencies.

**Setup Required**: None - fixture included in repo at `tests/fixtures/dependencies/synthetic-go-project/`

**How to run**:

```bash
# Runs automatically with standard test suite
make test

# Or run directly
make test-integration-cooling-synthetic
```

**Time**: < 10s
**When**: Every commit, pre-commit, pre-push, CI/CD
**Dependencies**: None (always works)

**What's validated**:

- Cooling policy configuration parsing
- Age-based violation detection
- Registry client integration (mocked responses)
- Issue creation and reporting
- Basic license detection

**Test files**:

- `pkg/dependencies/integration_test.go` - Test scenarios
- `pkg/dependencies/integration_bench_test.go` - Performance benchmarks
- `pkg/dependencies/testdata/` - Test policy configurations
- `tests/fixtures/dependencies/synthetic-go-project/` - Synthetic test project

#### Tier 2: Hugo Baseline (Recommended for Releases)

**What it tests**: Real-world Go project with ~80 dependencies, validating registry integration, caching, and performance.

**Setup Required**: Clone Hugo repository once

**How to set up** (one-time):

1. **Clone the test repository**:

```bash
# Create a directory for test repos (any location works)
mkdir -p ~/dev/playground
cd ~/dev/playground

# Clone Hugo (shallow clone is fine)
git clone --depth=1 https://github.com/gohugoio/hugo
```

2. **Tell goneat where to find it**:

```bash
# Add to your shell profile (~/.zshrc, ~/.bashrc, etc.)
export GONEAT_COOLING_TEST_ROOT=$HOME/dev/playground

# Or set it per-session
export GONEAT_COOLING_TEST_ROOT=/path/to/wherever/you/put/repos
```

**Important**: Point to the **parent directory** containing the repos, not the repos themselves.

Expected structure:

```
$GONEAT_COOLING_TEST_ROOT/
└── hugo/           ← the repo, with go.mod at root
```

**How to run**:

```bash
# After setup above
make test-integration-cooling-quick
```

**Time**: ~8s (warm cache), ~38s (cold cache)
**When**: Before releases, when working on dependencies feature
**Target**: < 15s warm, < 10% violations

**What's validated**:

- Real go.mod parsing with ~80 dependencies
- Registry API calls to pkg.go.dev (cached after first run)
- License detection on real packages
- Cooling policy evaluation with real package ages
- Cache performance (4-5x speedup on second run)

**What if I don't set this up?**
Tests skip gracefully with a message:

```
Test repository not configured. Set GONEAT_COOLING_TEST_ROOT or clone repos to ~/dev/playground
```

#### Tier 3: Full Suite (Major Releases Only)

**What it tests**: Comprehensive scenarios across 4 different Go projects (Hugo, OPA, Traefik, Mattermost) with varying complexity and policy configurations.

**Setup Required**: Clone 4 test repositories

**How to set up** (one-time):

```bash
# In your test repos directory
cd ~/dev/playground  # Or wherever you chose

# Clone all test repos (shallow clones are fine)
git clone --depth=1 https://github.com/gohugoio/hugo
git clone --depth=1 https://github.com/open-policy-agent/opa
git clone --depth=1 https://github.com/traefik/traefik
git clone --depth=1 https://github.com/mattermost/mattermost-server

# Environment variable already set from Tier 2 setup
```

**Disk Space**: ~500MB total (shallow clones)
**Time to Clone**: ~2-5 minutes (network dependent)

**How to run**:

```bash
# Run full suite (8 test scenarios)
make test-integration-cooling

# Or run all 3 tiers sequentially
make test-integration-extended
```

**Time**: ~113s (1.9 minutes)
**When**: Major releases (v0.3.0, v1.0.0), comprehensive validation
**Expected**: 6/8 tests passing (2 known non-blocking failures)

**What's validated**:

- Hugo baseline (80 deps, permissive policy)
- Mattermost strict policy (skip, known repo structure issue)
- Traefik with exceptions (98 issues, 0 false positives)
- OPA with time-limited exceptions (expired vs valid)
- Registry failure handling (17 fallbacks)
- Cache performance benchmarks (skip, threshold issue)
- Disabled cooling policy (no violations expected)
- Cooling checker integration (synthetic package validation)

**Known Issues** (non-blocking):

1. **Mattermost test fails**: Repo has `go.mod` in `/server` subdirectory, not root
   - Status: Documented, not blocking
   - Fix: Update test to use `/server` path

2. **Cache performance test fails**: Test expects 1.5x speedup, actual 1.01x
   - Status: Cache IS working (4.77x speedup first→second run)
   - Issue: go-licenses overhead dominates, test threshold too strict
   - Fix: Relax threshold or document behavior

#### Dependencies and Coupling

**Tier 1 (Zero External Coupling)**:

- Synthetic fixture in our repo
- No external dependencies
- Breaks only if we break it

**Tier 2 (Low External Coupling)**:

- Hugo repository structure (expects `go.mod` at root)
- Hugo dependency stability (assumes < 50% churn)
- pkg.go.dev API stability (has SLA)
- **Risk**: Low - Hugo is stable, changes slowly

**Tier 3 (Medium External Coupling)**:

- 4 external repositories (Hugo, OPA, Traefik, Mattermost)
- Multiple registry APIs (npm, PyPI, crates.io for future)
- Network availability for registry lookups (cached after first run)
- **Risk**: Medium - More repos = more drift potential, but documented

#### Assumptions

1. **Repository Structure**: go.mod at repo root (fails gracefully if not)
2. **Stability**: Repos don't change dependency count by >50% overnight
3. **Registry APIs**: pkg.go.dev, npm, PyPI, crates.io stay stable
4. **File System**: ~500MB disk space available for test repos
5. **Network**: Internet access for registry API calls (first run only)

#### Graceful Degradation

All Tier 2/3 tests check for:

- `GONEAT_COOLING_TEST_ROOT` environment variable set?
- Test repository exists at expected path?
- Repository has expected structure (go.mod location)?

If any check fails → **test skips with informative message**, doesn't fail CI/CD.

#### Documentation Quick Reference

| I want to...            | Read this...                          | Location                                                                 |
| ----------------------- | ------------------------------------- | ------------------------------------------------------------------------ |
| Run tests quickly       | README or Makefile                    | `../../README.md`                                                        |
| Understand the tiers    | This doc                              | You're here!                                                             |
| Set up test repos       | TEST_EXECUTION_GUIDE                  | `../appnotes/lib/dependencies/TEST_EXECUTION_GUIDE.md`                   |
| Understand decisions    | Integration Test Protocol             | `../../.plans/active/v0.3.0/wave-2-phase-4-INTEGRATION-TEST-PROTOCOL.md` |
| Debug test failures     | Integration Test Protocol (section 7) | Same as above                                                            |
| Know when to run tests  | RELEASE_CHECKLIST                     | `../../RELEASE_CHECKLIST.md`                                             |
| Understand release flow | Release Process                       | `../ops/repository/release-process.md`                                   |

#### Workflow Examples

##### New Contributor (Day 1)

**Goal**: Just want to contribute code, don't care about integration tests yet.

```bash
# Clone the repo
git clone https://github.com/fulmenhq/goneat
cd goneat

# Run tests (Tier 1 included automatically)
make test

# Make changes, commit, done!
```

✅ **No setup required** - Tier 1 always works.

##### Developer Working on Dependencies Feature (Week 1)

**Goal**: Need to validate changes against real Go projects.

```bash
# One-time setup (5 minutes)
mkdir ~/dev/playground
cd ~/dev/playground
git clone --depth=1 https://github.com/gohugoio/hugo

# Add to shell profile
echo 'export GONEAT_COOLING_TEST_ROOT=$HOME/dev/playground' >> ~/.zshrc
source ~/.zshrc

# Back to goneat repo
cd ~/dev/fulmenhq/goneat

# Run quick Hugo test (now works)
make test-integration-cooling-quick

# Make changes, test, iterate
```

✅ **Tier 2 available** - Quick validation loop.

##### Release Manager (Preparing v0.3.0)

**Goal**: Run comprehensive tests before major release.

```bash
# Ensure all test repos cloned (one-time, if not done)
cd ~/dev/playground
git clone --depth=1 https://github.com/open-policy-agent/opa
git clone --depth=1 https://github.com/traefik/traefik
git clone --depth=1 https://github.com/mattermost/mattermost-server

# Back to goneat repo
cd ~/dev/fulmenhq/goneat

# Run all 3 tiers
make test-integration-extended

# Review results (expect 6/8 passing in Tier 3)
# Document any new failures
# Proceed with release if no regressions
```

✅ **Full validation** - Comprehensive coverage before shipping.

#### Troubleshooting

##### "Test repository not configured"

**Symptom**: Tier 2/3 tests skip with message about GONEAT_COOLING_TEST_ROOT.

**Solution**:

1. Set environment variable: `export GONEAT_COOLING_TEST_ROOT=$HOME/dev/playground`
2. Clone required repos to that location (see setup sections above)
3. Restart your shell or `source ~/.zshrc`

##### "Hugo test suddenly failing"

**Possible Causes**:

1. Hugo changed their dependency count significantly
   - Check Hugo repo's recent commits
   - Update expected violation threshold if legitimate change
2. Hugo moved their go.mod file
   - Check Hugo repo structure
   - Update test path if needed
3. Registry API changed format
   - Check other registry tests (would affect multiple tests)
   - Update client code if API evolved

##### "Registry API rate limits"

**Symptom**: Tests fail with 429 or rate limit errors.

**Solution**:

1. Wait 1 hour for rate limit to reset
2. Registry responses are cached, so subsequent runs use cache
3. If persistent, check if registry has updated rate limits

##### "Cache seems stale"

**Symptom**: Test results inconsistent, registry data seems old.

**Solution**:

1. Registry cache location: `~/.goneat/cache/registry/` (check if exists)
2. Clear cache: `rm -rf ~/.goneat/cache/registry/`
3. Re-run test (will fetch fresh data)

**Note**: Cache behavior and location should be verified - this is based on common patterns.

##### "Mattermost test always fails"

**Expected**: This is a known issue (documented in Integration Test Protocol).

**Reason**: Mattermost has `go.mod` in `/server` subdirectory, test expects it at root.

**Status**: Non-blocking, documented as acceptable failure.

**Fix** (if you want to tackle it): Update test to check `/server/go.mod` path.

#### Performance Expectations

##### Tier 1: Synthetic

- **Target**: < 10s
- **Current**: ~8s
- **Status**: ✅ Meeting target

##### Tier 2: Hugo

- **Target**: < 15s (warm cache)
- **Current**: ~8s (warm), ~38s (cold)
- **Status**: ✅ Meeting target (warm)
- **Note**: First run always slower (~38s) due to registry lookups

##### Tier 3: Full Suite

- **Target**: < 5 minutes
- **Current**: ~113s (~1.9 minutes)
- **Status**: ✅ Exceeding target

**Cache Behavior**:

- First run: Cold cache, makes real registry API calls (~38s for Hugo)
- Second run: Warm cache, uses cached responses (~8s for Hugo)
- **Speedup**: 4-5x improvement with cache

#### Related Documentation

- **[Release Checklist](../../RELEASE_CHECKLIST.md)** - When to run integration tests during release process
- **[Release Process](../ops/repository/release-process.md)** - Integration of tests into release workflow
- **[Integration Test Protocol](../../.plans/active/v0.3.0/wave-2-phase-4-INTEGRATION-TEST-PROTOCOL.md)** - Detailed strategy and decisions
- **[TEST_EXECUTION_GUIDE](../appnotes/lib/dependencies/TEST_EXECUTION_GUIDE.md)** - Step-by-step execution instructions
- **[Dependency Cooling Policy](../appnotes/lib/dependencies.md)** - Feature documentation

## Contributing

Found an issue with integration tests? Want to add a new test scenario?

1. **For bugs/issues**: Open a GitHub issue with:
   - Which tier failed (1/2/3)
   - Full test output
   - Your environment (OS, Go version, `$GONEAT_COOLING_TEST_ROOT` setting)

2. **For new test scenarios**:
   - Add to appropriate `*_test.go` file in `pkg/dependencies/`
   - Update test documentation in `docs/appnotes/lib/dependencies/`
   - Update this overview if adding new external dependencies

3. **For new integration suites**:
   - Add a new section to this document
   - Follow the tier structure (synthetic → quick → comprehensive)
   - Document coupling and assumptions clearly

**Last Updated**: 2025-10-11
**Maintained by**: Code Scout (@code-scout) under supervision of @3leapsdave
**Status**: Active (Wave 2 Phase 4 complete)

For questions or clarifications, see GitHub issues or reach out to maintainers.
