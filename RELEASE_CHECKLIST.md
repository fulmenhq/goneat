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

## Prerequisites

### Repository Structure

goneat's release automation requires specific sibling repositories for package manager formula updates:

```
parent/
  ├── goneat/              # This repository
  ├── homebrew-tap/        # Required for `make update-homebrew-formula`
  └── homebrew-tap-tools/  # Optional (improves local dev workflow)
```

**Setup:**

```bash
cd ..  # Navigate to parent directory
git clone https://github.com/fulmenhq/homebrew-tap.git
git clone https://github.com/fulmenhq/homebrew-tap-tools.git  # Optional
cd goneat
```

**Why this matters**: The `make release-upload` target automatically updates the Homebrew formula after uploading release artifacts. If `../homebrew-tap` is not present, the release process will skip formula updates and provide manual instructions.

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
make build-all  # Builds 6 platform targets (for inspection - CI builds actual release artifacts)

# Platforms:
# - Linux AMD64/ARM64
# - macOS AMD64/ARM64 (Darwin)
# - Windows AMD64
# - (Windows ARM64 planned for future)
```

Binary testing is automatic for compatible platforms. Non-compatible platforms (e.g., Windows on macOS) will show test warnings but still produce binaries.

**Note**: Release artifacts are built by CI, not local `make build-all`. Use `make build-all` for pre-release inspection and validation only.

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
make release-clean  # Optional but recommended: wipe dist/release before packaging
make build-all      # Cross-platform binaries
make package        # Create distribution archives (dist/release/*.tar.gz, *.zip, SHA256SUMS)
# Note: make release-notes is automatically called by make release-upload
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

**Current Status**: Manual signing workflow operational using CI-built artifacts. Automated signing planned for future releases.

**Artifact Strategy**: Sign CI-built artifacts (not local builds) to ensure signatures match what users download. Use `make build-all` for pre-release inspection only.

**Prerequisites:**

- YubiKey connected with GPG signing subkey
- `gpg --card-status` shows signing subkey available
- `gpg --list-secret-keys security@fulmenhq.dev` accessible

## Signing Workflow

> **CRITICAL: One-Way Sequence**
>
> The signing workflow is a ONE-WAY sequence. Once you sign (step 5), you MUST NOT
> regenerate checksums (step 3). Doing so invalidates all signatures and requires
> re-signing. The Makefile includes guards to prevent accidental checksum regeneration
> after signing.

### 1. Wait for CI to complete and build artifacts

After tagging, wait for GitHub Actions to build and upload artifacts to draft release.

```bash
RELEASE_TAG=v0.3.15  # Set to current release version
echo "Waiting for CI completion for $RELEASE_TAG..."
gh run list --workflow=ci.yml --limit=1 --json status,conclusion | jq -r '.[0] | select(.status == "completed" and .conclusion == "success") | "CI completed successfully"'
```

Or monitor: https://github.com/fulmenhq/goneat/actions

### 2. Download CI-built artifacts (sign what users actually get)

```bash
make release-clean     # Clean any local artifacts
RELEASE_TAG=$RELEASE_TAG make release-download  # Download CI-built artifacts (requires gh CLI)
```

### 3. Generate checksums from downloaded artifacts

> **WARNING**: Do NOT run this step again after signing! Regenerating checksums
> invalidates signatures. The Makefile will block this if signatures exist.

```bash
RELEASE_TAG=$RELEASE_TAG make release-checksums # Generate SHA256SUMS and SHA512SUMS
```

### 3a. (Optional) Verify checksums match artifacts

Use this to verify checksum integrity without regenerating (safe to run anytime):

```bash
RELEASE_TAG=$RELEASE_TAG make release-verify-checksums  # Non-destructive verification
```

### 4. Set signing environment variables

Set these environment variables (temporary - do not export to avoid shell pollution):

```bash
PGP_KEY_ID=$(gpg --list-secret-keys --keyid-format=long security@fulmenhq.dev | grep '^sec' | head -1 | awk '{print $2}' | cut -d'/' -f2)
MINISIGN_KEY=~/.minisign/fulmenhq-release.key
MINISIGN_PUB=~/.minisign/fulmenhq-release.pub
GPG_HOMEDIR=${GNUPGHOME:-~/.gnupg}  # Use GNUPGHOME if set, fallback to default
```

### 5. Sign checksum manifests

```bash
RELEASE_TAG=$RELEASE_TAG make release-sign  # Sign with GPG and minisign
```

This target automatically:

- Uses sign-checksums.sh helper if available
- Falls back to manual GPG + minisign signing
- Copies minisign public key for distribution
- Extracts GPG public key for distribution

### 6. Verify signatures and key safety

```bash
RELEASE_TAG=$RELEASE_TAG make release-verify-signatures  # Verify GPG + minisign signatures
RELEASE_TAG=$RELEASE_TAG make release-verify-key         # Verify GPG key is public-only
```

#### Manual verification (fallback)

GPG signatures:

```bash
for asc in SHA256SUMS.asc SHA512SUMS.asc; do
  gpg --homedir "$GPG_HOMEDIR" --verify "$asc" "${asc%.asc}"
done
```

Minisign signatures:

```bash
for sig in SHA256SUMS.minisig SHA512SUMS.minisig; do
  minisign -Vm "${sig%.minisig}" -p fulmenhq-release-minisign.pub
done
```

Key safety:

```bash
./scripts/verify-public-key.sh fulmenhq-release-signing-key.asc
```

### 7. Prepare for upload

All signature and key files are now ready in `dist/release/`:

- Checksums: `SHA256SUMS`, `SHA512SUMS`
- GPG signatures: `SHA256SUMS.asc`, `SHA512SUMS.asc`
- Minisign signatures: `SHA256SUMS.minisig`, `SHA512SUMS.minisig`
- Public keys: `fulmenhq-release-signing-key.asc`, `fulmenhq-release-minisign.pub`

## Upload to GitHub Release

**IMPORTANT:** Upload BOTH binaries and signatures, not just signatures!

### Option A: Automated Upload (Recommended)

Includes automatic Homebrew formula updates.

```bash
cd ../..  # Return to repo root
# CRITICAL: Ensure GPG_HOMEDIR matches what was used during signing
export GPG_HOMEDIR=${GNUPGHOME:-~/.gnupg}  # Use same value as signing step
make release-upload  # Uploads artifacts AND updates ../homebrew-tap formula
```

### Option B: Manual Upload

```bash
# Upload binaries and checksums
gh release upload $RELEASE_TAG \
  goneat_${RELEASE_TAG}_*.tar.gz \
  goneat_${RELEASE_TAG}_*.zip \
  SHA256SUMS \
  SHA512SUMS \
  --clobber

# Upload signatures and keys
gh release upload $RELEASE_TAG \
  SHA256SUMS.asc \
  SHA512SUMS.asc \
  SHA256SUMS.minisig \
  SHA512SUMS.minisig \
  fulmenhq-release-signing-key.asc \
  fulmenhq-release-minisign.pub \
  --clobber

# Update release notes
gh release edit $RELEASE_TAG --notes-file release-notes-${RELEASE_TAG}.md

# CRITICAL: If using Option B, you must manually verify signatures before upload
# The automated target does this verification automatically
```

### Verify Upload Success

```bash
# Should show 13 assets total
gh release view $RELEASE_TAG --json assets --jq '.assets | length'
gh release view $RELEASE_TAG --json assets --jq '.assets[].name'
```

## Post-Upload Verification

### Automated Verification (Recommended)

```bash
scripts/verify-release-assets.sh $RELEASE_TAG
```

### Manual Verification (Fallback)

```bash
TMPDIR=$(mktemp -d)
gh release download $RELEASE_TAG --dir "$TMPDIR" --pattern "goneat_${RELEASE_TAG}_*.tar.gz" --clobber
gh release download $RELEASE_TAG --dir "$TMPDIR" --pattern "goneat_${RELEASE_TAG}_*.zip" --clobber
(cd "$TMPDIR" && shasum -a 256 goneat_${RELEASE_TAG}_*.tar.gz goneat_${RELEASE_TAG}_*.zip | sort > SHA256SUMS.github)
sort dist/release/SHA256SUMS > "$TMPDIR"/SHA256SUMS.local
diff "$TMPDIR"/SHA256SUMS.local "$TMPDIR"/SHA256SUMS.github  # Must be empty before release is declared healthy
gh release download $RELEASE_TAG --dir "$TMPDIR" --pattern SHA256SUMS --clobber
sort "$TMPDIR"/SHA256SUMS > "$TMPDIR"/SHA256SUMS.remote
diff "$TMPDIR"/SHA256SUMS.local "$TMPDIR"/SHA256SUMS.remote  # Validates uploaded checksum matches local copy
```

> ⚠️ Since we sign CI-built artifacts, any checksum mismatches indicate CI build problems, not local packaging issues. Always verify CI builds are consistent before signing.

### Update Package Manager Formulas

If using Option B (manual upload), update Homebrew formula separately:

```bash
make update-homebrew-formula  # Requires ../homebrew-tap
```

**Note**: `make release-upload` (Option A) automatically generates release notes (`make release-notes`) and calls `make update-homebrew-formula` after uploading artifacts. If using Option B (manual upload), run these targets separately.

**See**: [`docs/security/release-signing.md`](docs/security/release-signing.md) for detailed signing procedures.

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

# Set version variable for convenience

VERSION=v0.3.15

# Download artifacts

curl -LO "https://github.com/fulmenhq/goneat/releases/download/${VERSION}/fulmenhq-release-signing-key.asc"
curl -LO "https://github.com/fulmenhq/goneat/releases/download/${VERSION}/SHA256SUMS"
curl -LO "https://github.com/fulmenhq/goneat/releases/download/${VERSION}/SHA256SUMS.asc"
curl -LO "https://github.com/fulmenhq/goneat/releases/download/${VERSION}/fulmenhq-release-minisign.pub"
curl -LO "https://github.com/fulmenhq/goneat/releases/download/${VERSION}/SHA256SUMS.minisig"

# Import and verify GPG signature

gpg --import fulmenhq-release-signing-key.asc
gpg --verify SHA256SUMS.asc SHA256SUMS

# Verify minisign signature

minisign -Vm SHA256SUMS -p fulmenhq-release-minisign.pub

# Verify checksums

shasum -a 256 --check SHA256SUMS
\`\`\`
```

**3. Upload Artifacts**

- All platform binaries (`.tar.gz`, `.zip`)
- Checksum signatures (`SHA256SUMS.asc`, `SHA512SUMS.asc`, `.minisig` companions)
- Checksums: `SHA256SUMS`, `SHA512SUMS`
- Public keys: `fulmenhq-release-signing-key.asc`, `fulmenhq-release-minisign.pub` (first release or key rotation)

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

**Homebrew Formula (if updated):**

- Formula version matches release version
- All platform checksums updated correctly
- Formula passes audit: `cd ../homebrew-tap && make audit APP=goneat`
- Test installation works: `cd ../homebrew-tap && make test APP=goneat`

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

### Invalid Signature Recovery

**Symptom**: `make release-upload` fails with "Invalid GPG signature for SHA256SUMS"

**Cause**: Checksums were regenerated AFTER signing, invalidating signatures. This can happen if:

- `make release-checksums` was run after `make release-sign`
- Artifacts were modified after signing
- Workflow steps were run out of order

**Diagnosis**: Check timestamps - signatures should be NEWER than checksums:

```bash
ls -la dist/release/SHA256SUMS dist/release/SHA256SUMS.asc
# .asc file MUST have a timestamp >= SHA256SUMS timestamp

# Verify checksums match artifacts (non-destructive)
RELEASE_TAG=vX.Y.Z make release-verify-checksums
```

**Recovery**:

```bash
# Option 1: Re-sign existing checksums (if checksums are correct)
cd dist/release
rm -f *.asc *.minisig  # Remove invalid signatures
cd ../..
RELEASE_TAG=vX.Y.Z make release-sign  # Re-sign

# Option 2: Full reset (if unsure about checksum integrity)
make release-clean
RELEASE_TAG=vX.Y.Z make release-download
RELEASE_TAG=vX.Y.Z make release-checksums
RELEASE_TAG=vX.Y.Z make release-sign
RELEASE_TAG=vX.Y.Z make release-verify-signatures
```

**Prevention**: The Makefile now guards against running `release-checksums` when signatures exist.

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
- `make release-upload` - Upload artifacts, generate release notes, and update Homebrew formula (v0.3.9+, enhanced v0.3.11)
- `make update-homebrew-formula` - Update Homebrew tap formula (v0.3.10+)

**Scripts:**

- `scripts/build-all.sh` - Multi-platform build orchestration
- `scripts/package-artifacts.sh` - Archive creation and checksums
- `scripts/push-to-remotes.sh` - Push to all configured remotes
- `scripts/generate-release-notes.sh` - Release notes generation

### Future Automation (Planned)

- GitHub Actions: Automated builds on tag push
- Automated release creation
- ✅ Binary upload automation (v0.3.9: `make release-upload`)
- ✅ Homebrew formula updates (v0.3.10: `make update-homebrew-formula`)
- Native `goneat formula` command (v0.3.11+: multi-package-manager support)
- Scoop and Winget manifest updates (v0.4.x+)
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
- ✅ Clone ../homebrew-tap for automated formula updates (v0.3.10+)
- ✅ Use `make release-upload` for complete release process (v0.3.9+)
- ✅ Verify Homebrew formula updates after release (v0.3.10+)
- ✅ Wait for pkg.go.dev indexing before announcing

**DON'T:**

- ❌ Use standalone `go test`, `golangci-lint`, etc.
- ❌ Skip `make prepush` validation
- ❌ Tag before validation passes
- ❌ Push without running full test suite
- ❌ Release with failing license audit
- ❌ Skip documentation updates
- ❌ Manually update Homebrew formulas (use `make update-homebrew-formula`)

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

**Document Version**: 2.2 (Best Practice Reference Guide)
**Last Updated**: 2025-12-02 (v0.3.11 - release-upload now generates release notes automatically)
**Next Review**: With each major release or significant process change
**Format**: General reference (not version-specific checklist)
