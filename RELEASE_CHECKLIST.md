# Release Checklist

This document provides standard release procedures and best practices for goneat releases. Use this as a reference guide for preparing, validating, and executing releases.

## Release Workflow Philosophy

**Always use `make` targets** instead of standalone `go` commands. The Makefile orchestrates complex workflows, ensures proper sequencing, and maintains consistency across development and CI/CD environments.

**Git hooks delegate to `make`**: Our pre-commit and pre-push hooks invoke make targets (not direct tool invocations), ensuring developer workflows match CI validation.

## Release Target Chain

goneat implements a three-stage release validation chain:

```
make prepush
  ↓
make release-check
  ↓
make release-prepare → build + sync-crucible + embed-assets
  ↓
test + lint + verify-crucible + license-audit
```

**Key Targets:**

- `make release-prepare`: Synchronizes SSOT, embeds assets, builds binary (no validation)
- `make release-check`: Full validation suite (tests, lint, crucible, license audit)
- `make prepush`: Comprehensive pre-push validation (includes release-check + crucible-clean + build-all + assess)

**Why this matters**: Running `make prepush` before pushing ensures all validation gates pass. This target automatically chains through release-check → release-prepare, providing full release readiness validation.

## Pre-Release Preparation

### Code Quality Gates

**Always run through make targets:**

```bash
# Full validation (recommended before any push)
make prepush

# Individual validation targets (if needed)
make test                    # Unit + Tier 1 integration tests
make lint                    # Go linting via goneat assess
make verify-crucible         # Verify SSOT sync is current
make verify-crucible-clean   # Verify no uncommitted changes in crucible sources
make license-audit           # Forbidden license detection (GPL/LGPL/AGPL/MPL/CDDL)
make build-all               # Cross-platform builds (6 targets)
```

**Never use standalone commands** like `go test ./...` or `golangci-lint run` directly. Always use make targets to ensure proper environment setup and configuration.

### Integration Testing Strategy (Three-Tier)

**Tier 1 (Mandatory - Always Run)**:
- Included in `make test` automatically
- Target: `make test-integration-cooling-synthetic`
- Time: < 10s, no external dependencies (CI-friendly)
- When: Every commit, pre-commit, pre-push

**Tier 2 (Recommended - Pre-Release)**:
- Target: `make test-integration-cooling-quick` (Hugo baseline)
- Time: ~8s (warm cache), ~38s (cold cache)
- Dependencies: Requires Hugo repository (set `GONEAT_COOLING_TEST_ROOT`)
- When: Before tagging any release

**Tier 3 (Optional - Major Releases)**:
- Target: `make test-integration-cooling` (all 8 scenarios)
- Time: ~113s (1.9 minutes)
- Dependencies: Hugo, OPA, Traefik, Mattermost repos in `GONEAT_COOLING_TEST_ROOT`
- When: Major version releases (v0.3.0, v1.0.0, etc.)

**Extended Testing** (Comprehensive):
- Target: `make test-integration-extended`
- Runs all three tiers sequentially
- When: Final validation before major releases

**Setup**:

```bash
# For Tier 2/3 testing
export GONEAT_COOLING_TEST_ROOT=$HOME/dev/playground
# Or clone test repos to ~/dev/playground/
```

### Version Management

Version updates should be handled through make targets or the goneat binary:

```bash
# Using goneat (dogfooding - recommended)
./dist/goneat version bump patch   # 0.3.5 → 0.3.6
./dist/goneat version bump minor   # 0.3.5 → 0.4.0
./dist/goneat version bump major   # 0.3.5 → 1.0.0

# Using make (alternative)
make version-set VERSION=v0.3.6
make version-set-prerelease VERSION_SET=v0.3.6-rc.1
```

**Always update these files when bumping versions:**
- `VERSION` - Single source of truth
- `CHANGELOG.md` - User-facing changes
- `RELEASE_NOTES.md` - Detailed release context (for RELEASE.md artifact)
- `docs/releases/v<version>.md` - Complete release documentation

### Cross-Platform Build Validation

```bash
make build-all  # Builds 6 platform targets

# Platforms:
# - Linux AMD64/ARM64
# - macOS AMD64/ARM64 (Darwin)
# - Windows AMD64
# - (Windows ARM64 planned for future)
```

Binary testing is automatic for compatible platforms. Non-compatible platforms (e.g., Windows on macOS) will show test warnings but still produce binaries.

### Documentation Requirements

Before any release, ensure:
- `README.md` - Installation and quick start current
- `docs/` - All feature documentation updated
- `docs/releases/v<version>.md` - Complete release documentation created
- API reference docs - All commands documented
- Breaking changes - Clearly documented with migration paths

### Licensing Compliance (Required)

**Always run license audit before release:**

```bash
make license-audit          # Fail on forbidden licenses
make license-inventory      # Generate CSV inventory (docs/licenses/inventory.csv)
make license-save           # Save third-party license texts (docs/licenses/third-party/)
make update-licenses        # Alias: inventory + save
```

**Forbidden licenses**: GPL, LGPL, AGPL, MPL, CDDL

License audit is included in `make release-check` and `make prepush`.

### Dependency Protection Dogfooding (v0.3.0+)

goneat uses its own dependency protection features:

```bash
# Validate dependency configuration
./dist/goneat dependencies --licenses   # License compliance check
./dist/goneat dependencies --cooling    # Cooling policy check
./dist/goneat dependencies --sbom       # SBOM generation

# Verify zero violations
./dist/goneat assess --categories=dependencies
```

Configuration: `.goneat/dependencies.yaml`

### SSOT Provenance Verification

```bash
make sync-crucible        # Sync from crucible SSOT
make verify-crucible      # Verify sync is current
make verify-crucible-clean # Verify no uncommitted changes

# Provenance files (committed to repo):
# - .goneat/ssot/provenance.json
# - .crucible/metadata/metadata.yaml
```

## Release Execution

### Standard Release Flow (Patch/Minor)

**1. Pre-Release Validation**

```bash
# Full validation (includes all checks below)
make prepush

# This internally runs:
#   make release-check
#     → make release-prepare (build, sync, embed)
#     → make test
#     → make lint
#     → make verify-crucible
#     → make license-audit
#   make verify-crucible-clean
#   make build-all
#   goneat assess --hook pre-push
```

**2. Tier 2 Integration Testing (Recommended)**

```bash
export GONEAT_COOLING_TEST_ROOT=$HOME/dev/playground
make test-integration-cooling-quick  # Hugo baseline (~8s)
```

**3. Tag and Push** (only after validation passes)

```bash
# Using make target
make release-tag   # Creates annotated tag from VERSION file

# Or manually
git tag -a v0.3.6 -m "Release v0.3.6"
git push origin v0.3.6
git push origin main  # Push commits
```

**4. Build Release Artifacts**

```bash
make build-all    # Cross-platform binaries
make package      # Create distribution archives (dist/release/*.tar.gz, *.zip, SHA256SUMS)
make release-notes # Generate release notes artifact (dist/release/release-notes-v<version>.md)
```

### Major Release Flow (v0.X.0, v1.0.0)

**All standard steps above, PLUS:**

```bash
# Comprehensive integration testing (before tagging)
export GONEAT_COOLING_TEST_ROOT=$HOME/dev/playground
make test-integration-extended  # All 3 tiers (~2 minutes)

# Document results for release notes
# Expected: 6/8 passing (2 known non-blocking failures in Tier 3)
```

### Cryptographic Signing (v0.3.4+)

**Current Status**: Manual signing workflow operational. Automated signing planned for future releases.

**Prerequisites:**
- YubiKey connected with GPG signing subkey
- `gpg --card-status` shows signing subkey available
- `gpg --list-secret-keys security@fulmenhq.dev` accessible

**Signing Workflow:**

```bash
# 1. Build and package artifacts
make build-all
make package  # Creates dist/release/*.tar.gz, *.zip, SHA256SUMS

# 2. Sign artifacts (manual - performed by release manager)
cd dist/release
for file in *.tar.gz *.zip SHA256SUMS; do
  gpg --detach-sign --armor --output "${file}.asc" "${file}"
done

# 3. Extract public key for distribution
gpg --armor --export security@fulmenhq.dev > fulmenhq-release-signing-key.asc

# 4. CRITICAL: Verify PUBLIC key only (never upload private keys!)
# Before first use: Inspect scripts/verify-public-key.sh to understand checks performed
# The script performs three independent verifications:
#   - Negative check: grep entire file for "PRIVATE KEY" blocks (must be absent)
#   - Positive check: grep entire file for "PUBLIC KEY" blocks (must be present)
#   - GPG verification: gpg --show-keys must show "pub" entries only (never "sec")
# All three checks must pass or script exits with error and blocks upload

./scripts/verify-public-key.sh fulmenhq-release-signing-key.asc

# Manual verification (optional - script automates these checks):
# grep -i "PRIVATE KEY" fulmenhq-release-signing-key.asc  # Must return nothing
# grep -i "PUBLIC KEY" fulmenhq-release-signing-key.asc   # Must find blocks
# gpg --show-keys fulmenhq-release-signing-key.asc       # Must show "pub" not "sec"

# 5. Verify signatures locally
for asc in *.asc; do
  gpg --verify "$asc" "${asc%.asc}"
done
# Should show "Good signature" for all

# 6. Upload to GitHub release
# IMPORTANT: Upload BOTH binaries and signatures, not just signatures!
# Option A: Use make target (recommended)
cd ../..  # Return to repo root
make release-upload

# Option B: Manual upload
gh release upload v<version> \
  goneat_v<version>_*.tar.gz \
  goneat_v<version>_*.zip \
  SHA256SUMS \
  --clobber
gh release upload v<version> \
  goneat_v<version>_*.asc \
  SHA256SUMS.asc \
  fulmenhq-release-signing-key.asc \
  --clobber
gh release edit v<version> --notes-file release-notes-v<version>.md

# Verify upload succeeded (should show 13 assets)
gh release view v<version> --json assets --jq '.assets | length'
gh release view v<version> --json assets --jq '.assets[].name'
```

**See**: `docs/security/release-signing.md` for detailed signing procedures.

### GitHub Release Creation

**1. Create Release on GitHub**
- Navigate to: https://github.com/fulmenhq/goneat/releases
- Click "Draft a new release"
- Select tag: `v0.3.6`
- Title: `goneat v0.3.6`

**2. Release Notes**
- Use generated artifact: `dist/release/release-notes-v0.3.6.md`
- Include signature verification instructions:

```markdown
## Verifying Signatures

Download the FulmenHQ public key and verify artifacts:

\`\`\`bash
curl -LO https://github.com/fulmenhq/goneat/releases/download/v0.3.6/fulmenhq-release-signing-key.asc
gpg --import fulmenhq-release-signing-key.asc
gpg --verify goneat-linux-amd64.tar.gz.asc goneat-linux-amd64.tar.gz
\`\`\`
```

**3. Upload Artifacts**
- All platform binaries (`.tar.gz`, `.zip`)
- All signature files (`.asc`)
- Checksums: `SHA256SUMS`, `SHA256SUMS.asc`
- Public key: `fulmenhq-release-signing-key.asc` (first release or key rotation)

### Go Module Verification

After GitHub release is created and tag is pushed:

```bash
# Wait 5-10 minutes for pkg.go.dev indexing

# Test module resolution
go get github.com/fulmenhq/goneat@v0.3.6

# Test installation
go install github.com/fulmenhq/goneat@v0.3.6

# Verify binary works
goneat version
goneat doctor tools --scope foundation
```

## Post-Release Validation

### Distribution Verification

**GitHub Release:**
- All binaries downloadable
- Signatures verify correctly
- SHA256SUMS matches all artifacts

**Go Module:**
- `go get` resolves correctly
- `go install` produces working binary
- pkg.go.dev documentation generated

**Cross-Platform:**
- Binaries functional on target platforms
- No runtime errors on supported OS/architectures

### Communication

- Announce release in relevant channels
- Update installation documentation if needed
- Monitor GitHub issues for critical problems

## Emergency Procedures

### Rollback Plan

**If critical issue discovered immediately after release:**

```bash
# 1. Delete tag (local and remote)
git tag -d v0.3.6
git push origin :refs/tags/v0.3.6

# 2. Delete GitHub release
# (via GitHub web UI)

# 3. Revert VERSION file
echo "v0.3.5" > VERSION
git add VERSION
git commit -m "revert: rollback to v0.3.5 due to critical issue"

# 4. Notify users
# - GitHub issue explaining rollback
# - Update release notes
```

### Recovery Checklist

**After rollback:**
- Verify local and remote repos in sync
- Check GitLab backup has correct state
- Inform all stakeholders
- Create hotfix branch if needed
- Re-run full validation before re-release

## Git Hooks and Automation

### Hook Delegation Pattern

goneat git hooks **always delegate to make targets**:

```bash
# .git/hooks/pre-commit (simplified)
#!/bin/bash
make precommit

# .git/hooks/pre-push (simplified)
#!/bin/bash
make prepush
```

**Why this matters:**
- Hooks use same validation as CI/CD
- Changes to validation logic only need Makefile updates
- Developers get same feedback locally as in pipeline
- `make precommit` and `make prepush` can be run manually

### Current Automation

**Makefile targets:**
- `make precommit` - Format checks, quick validation
- `make prepush` - Full validation (release-check + build-all + assess)
- `make build-all` - Cross-platform binary builds
- `make package` - Release artifact packaging
- `make release-notes` - Generate release notes artifact

**Scripts:**
- `scripts/build-all.sh` - Multi-platform build orchestration
- `scripts/package-artifacts.sh` - Archive creation and checksums
- `scripts/push-to-remotes.sh` - Push to all configured remotes
- `scripts/generate-release-notes.sh` - Release notes generation

### Future Automation (Planned)

- GitHub Actions: Automated builds on tag push
- Automated release creation
- Binary upload automation
- Changelog generation from commits
- Automated signing integration (requires CI infrastructure)

## Quality Gates

### Minimum Release Requirements

**Must pass before any release:**
- `make test` - All unit + Tier 1 integration tests
- `make lint` - No linting issues
- `make license-audit` - No forbidden licenses
- `make verify-crucible` - SSOT sync current
- `make build-all` - All platform builds succeed
- `make prepush` - Full validation passes

**Coverage gates:**
- Enforced via `make coverage-check`
- Thresholds based on `LIFECYCLE_PHASE` file
- Alpha: 30%, Beta: 60%, RC: 70%, GA: 75%, LTS: 80%

### Success Metrics

**Installation success:** > 95% successful installations (monitor GitHub issues)
**User feedback:** No critical issues reported within 48 hours
**Performance:** No significant regressions (benchmark before major releases)
**Compatibility:** Backward compatibility maintained (semver compliance)

## Release Scope Profiles

### Initial Public Release Baseline

**Required for first public release:**
- Core commands fully functional
- Documentation complete (README, user guide, API reference)
- Test suite with stable coverage gate
- Cross-platform builds verified
- `go install github.com/fulmenhq/goneat@vX.Y.Z` works end-to-end

### Ongoing Releases

**For all subsequent releases:**
- Breaking changes require major version bump (semver)
- Deprecation notices with timelines and alternatives
- Migration guides for breaking changes
- Performance benchmarks for significant changes

## Development Workflows

### Daily Development

```bash
# Before committing
make fmt           # Format code
make test          # Quick validation

# Before pushing
make prepush       # Full validation (recommended)
```

### Pre-Release Development

```bash
# Continuous validation during feature development
make test                              # Unit + Tier 1 integration
make test-integration-cooling-quick    # Tier 2 validation (with repos)

# Before creating release branch
make prepush                           # Full validation
make test-integration-extended         # Comprehensive (major releases)
```

### Release Branch Workflow

```bash
# 1. Create release branch
git checkout -b release/v0.3.6

# 2. Update version and docs
./dist/goneat version set v0.3.6
# Update CHANGELOG.md, RELEASE_NOTES.md, docs/releases/v0.3.6.md

# 3. Full validation
make prepush

# 4. Merge to main and tag
git checkout main
git merge release/v0.3.6
git tag -a v0.3.6 -m "Release v0.3.6"
git push origin main v0.3.6
```

## Best Practices Summary

**DO:**
- ✅ Use `make` targets for all operations
- ✅ Run `make prepush` before pushing
- ✅ Test Tier 2 integration before any release
- ✅ Update all documentation before tagging
- ✅ Verify license audit passes
- ✅ Sign all release artifacts (v0.3.4+)
- ✅ Wait for pkg.go.dev indexing before announcing

**DON'T:**
- ❌ Use standalone `go test`, `golangci-lint`, etc.
- ❌ Skip `make prepush` validation
- ❌ Tag before validation passes
- ❌ Push without running full test suite
- ❌ Release with failing license audit
- ❌ Skip documentation updates

## Contact Information

### For Release Issues

- **Primary**: GitHub Issues (https://github.com/fulmenhq/goneat/issues)
- **Security**: security@fulmenhq.dev
- **Urgent**: Direct team communication

### Release Coordination

- **Release Manager**: Current sprint lead
- **Documentation**: Technical writer
- **Testing**: QA team
- **Communication**: Product team

---

**Document Version**: 2.0 (Best Practice Reference Guide)
**Last Updated**: 2025-11-14 (v0.3.6 release cycle)
**Next Review**: With each major release or significant process change
**Format**: General reference (not version-specific checklist)
