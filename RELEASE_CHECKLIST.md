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

This signing and supplement-upload flow operates in the goneat repository.
It does not update Homebrew, Scoop or any other package manager and does not
require sibling package-manager repositories.

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
make build-all  # Builds 5 platform targets (for inspection - CI builds actual release artifacts)

# Platforms:
# - Linux AMD64/ARM64
# - macOS ARM64 (Darwin)
# - Windows AMD64/ARM64
```

The host-native binary is tested during cross-building; a failure stops the build.
Foreign targets are not executed locally. Required native CI cells execute the
same packaged candidate bytes on all five targets before release publication.
Both manifests must contain the exact five archive names; missing binaries or
stale retired archives fail packaging. Darwin amd64 downloads end at v0.6.1.

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

**3. Tag and Push** (only after validation passes and the release commit is merged)

Release tags are GPG-signed annotated tags with a declared tagger identity.
Run these on the operator machine, never in CI. Every step needs:

| Variable              | Meaning                                                                                          |
| --------------------- | ------------------------------------------------------------------------------------------------ |
| `GONEAT_RELEASE_TAG`  | tag to create, verify or push; must equal the `VERSION` file                                     |
| `GONEAT_PGP_KEY_ID`   | signing key: 40-hex fingerprint or 16-hex long key id; a trailing `!` forces that exact (sub)key |
| `GONEAT_GPG_HOMEDIR`  | isolated gpg homedir holding the signing key; the default keyring is never used                  |
| `GONEAT_TAGGER_NAME`  | tagger name recorded on the tag                                                                  |
| `GONEAT_TAGGER_EMAIL` | tagger email; must be a uid email on the signing key                                             |

```bash
# 1. Create the signed tag on HEAD and verify it. Refuses unless on a clean main
#    equal to a freshly fetched origin/main, and if the tag exists locally or on
#    origin. A tag that fails verification is deleted. Does not push.
make release-tag

# 2. Check it before anything leaves the machine: annotated, named for VERSION,
#    at HEAD, tagger identity as declared, one good signature by the selected key.
make release-tag-verify
git show --no-patch "$GONEAT_RELEASE_TAG"

# 3. Push only refs/tags/<tag> to origin. Repeats the checks from steps 1 and 2,
#    refuses if origin already has the tag, and confirms that origin's tag
#    object and commit match the local ones. Never forced; does not push main,
#    other tags or other remotes.
make release-tag-push
```

To check the published tag from any clone:

```bash
TAG="$(cat VERSION)"
git fetch origin tag "$TAG"
git cat-file -t "$TAG"   # tag
git tag -v "$TAG"        # with the release public key imported
```

`make test-release-tag` runs the script's tests against a disposable repository
and throwaway keys.

`make release-push` (main plus the verified tag, to origin and the optional
`gitlab` backup remote) and the aggregate `make release` are not part of this
procedure. Sync a backup remote as its own step.

**4. Build Release Artifacts**

```bash
make release-clean  # Optional but recommended: wipe dist/release before packaging
make build-all      # Cross-platform binaries
make package        # Create distribution archives (dist/release/*.tar.gz, *.zip, SHA256SUMS)
# Local packages are candidate inspection only, not replacements for CI assets.
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
> Preserve the original five CI archives and both CI checksum manifests from
> download through upload. Never regenerate the downloaded manifests, even
> before signing. `release-checksums` refuses existing manifests; use the
> non-destructive `release-verify-checksums` target instead.

### 1. Wait for CI to complete and build artifacts

After tagging, wait for GitHub Actions to build and upload artifacts to draft release.

```bash
GONEAT_RELEASE_TAG=v0.3.15  # Set to current release version
echo "Waiting for CI completion for $GONEAT_RELEASE_TAG..."
gh run list --workflow=ci.yml --limit=1 --json status,conclusion | jq -r '.[0] | select(.status == "completed" and .conclusion == "success") | "CI completed successfully"'
```

Or monitor: https://github.com/fulmenhq/goneat/actions

### 2. Download CI-built artifacts (sign what users actually get)

```bash
# Use a new or empty dist/release directory; preserve any existing evidence elsewhere.
GONEAT_RELEASE_TAG=$GONEAT_RELEASE_TAG make release-download  # Download CI-built artifacts (requires gh CLI)
```

Download retrieves exactly the five release archives plus original `SHA256SUMS`
and `SHA512SUMS`. Both checksum algorithms must verify before local files are
promoted. Existing destination files are never overwritten.

### 3. Verify both original checksum manifests

Verify integrity without rewriting either CI manifest:

```bash
GONEAT_RELEASE_TAG=$GONEAT_RELEASE_TAG make release-verify-checksums  # Non-destructive verification
```

### 4. Set signing environment variables

Set signing environment variables.

Preferred convention is `GONEAT_*` (Fulmen standard). Pass them inline to `make` to avoid shell pollution.

```bash
# Prefer absolute paths (avoid ~ which does not always expand in Make/env)
export GONEAT_PGP_KEY_ID="<independently approved signing identity or fingerprint>"
export GONEAT_GPG_HOMEDIR="<approved GPG homedir outside dist/release>"
export GONEAT_MINISIGN_KEY="<approved minisign secret key path>"
export GONEAT_MINISIGN_PUB="<independently approved minisign public key path outside dist/release>"
```

Notes:

- Minisign signing is required.
- PGP signing is required for goneat releases (the Makefile enforces this).
- Verification and upload require the approved GPG homedir/identity and minisign
  public key even when the release contains copies of those keys. Downloaded
  public keys are comparison inputs, never independent trust roots.

### 5. Sign checksum manifests

```bash
GONEAT_RELEASE_TAG="$GONEAT_RELEASE_TAG" \
make release-sign
```

This target uses `scripts/sign-release-manifests.sh` (preferred) which:

- Signs `SHA256SUMS` and `SHA512SUMS` with minisign
- Signs the same manifests with PGP
- Copies the independently approved minisign public key into `dist/release/`
- Exports the PGP public key into `dist/release/fulmenhq-release-signing-key.asc`
- Verifies all four staged signatures before publishing local signing outputs
- Refuses existing outputs rather than overwriting signatures or keys
- Preserves the original seven downloaded files byte for byte

### 6. Verify signatures and key safety

```bash
GONEAT_RELEASE_TAG=$GONEAT_RELEASE_TAG make release-verify-signatures  # Verify GPG + minisign signatures
GONEAT_RELEASE_TAG=$GONEAT_RELEASE_TAG make release-verify-key         # Verify GPG key is public-only
```

Keep the same independently approved trust variables available for verification
and upload. Missing manifests, signatures, tools or approved trust inputs, bad
signatures, private key material and mismatched bundled public keys all fail
nonzero. All four checks are mandatory; none is a warning-only skip.

### 7. Prepare for upload

All signature and key files are now ready in `dist/release/`:

- Checksums: `SHA256SUMS`, `SHA512SUMS`
- GPG signatures: `SHA256SUMS.asc`, `SHA512SUMS.asc`
- Minisign signatures: `SHA256SUMS.minisig`, `SHA512SUMS.minisig`
- Public keys: `fulmenhq-release-signing-key.asc`, `fulmenhq-release-minisign.pub`

## Upload to GitHub Release

**IMPORTANT:** CI already published the original five archives and both
checksum manifests. Do not upload them again. Upload supplements only.

```bash
make release-notes   # Explicitly prepare both matching release-note files
make release-upload  # Verified supplements only; no package-manager updates
```

The uploader verifies both manifests and all four signatures before remote
writes. It compares actual remote bytes for all seven originals and any existing
supplements. A byte-identical supplement is a verified no-op; different existing
bytes halt. It uploads only missing signatures, public keys and versioned notes,
updates the body from matching notes, and checks original asset IDs and bytes
again afterward. No upload uses `--clobber` or replaces an original asset.
Publication still requires separate maintainer authorization.

### Verify Upload Success

```bash
# Five archives, two original manifests, four signatures, two keys and versioned notes
gh release view $GONEAT_RELEASE_TAG --json assets --jq '.assets | length'
gh release view $GONEAT_RELEASE_TAG --json assets --jq '.assets[].name'
```

## Post-Upload Verification

### Automated Verification (Recommended)

```bash
scripts/verify-release-assets.sh $GONEAT_RELEASE_TAG
```

### Manual Verification (Fallback)

```bash
VERIFY_DIR=$(mktemp -d)
scripts/download-release-assets.sh "$GONEAT_RELEASE_TAG" "$VERIFY_DIR"
cmp dist/release/SHA256SUMS "$VERIFY_DIR/SHA256SUMS"
cmp dist/release/SHA512SUMS "$VERIFY_DIR/SHA512SUMS"
```

> ⚠️ Since we sign CI-built artifacts, any checksum mismatches indicate CI build problems, not local packaging issues. Always verify CI builds are consistent before signing.

### Update Package Manager Formulas

This flow does not update Homebrew, Scoop or any other package manager. No
package-manager procedure or repository change is included in signature upload.

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

**Scoop Manifest (if updated):**

- Manifest version matches release version
- Windows amd64 hash matches SHA256SUMS
- Manifest parses: `jq . ../scoop-bucket/bucket/goneat.json`

### Communication

- Announce release in relevant channels
- Update installation documentation if needed
- Monitor GitHub issues for critical problems

## Emergency Procedures

### Rollback Plan

**If a critical issue is found after release:**

A published release tag is never deleted, moved or reused, and `VERSION` is
never set back to an earlier release. Fix forward instead:

1. Mark the affected release: edit the GitHub release to state the issue and
   point users to the previous good version. Mark it as a pre-release if it
   should stop being shown as latest.
2. If needed, hold or revert the Homebrew formula and Scoop manifest to the
   previous good version. These are separate commits in those repositories.
3. Fix the issue on `main` through a normal pull request, bump to the next
   patch version, and release it with the full checklist, including a new
   signed tag.
4. Notify users: open a GitHub issue that explains the problem and names the
   fixed version, and note it in the release notes.

Deleting or replacing a published tag is outside this procedure and needs
separate maintainer authorization.

### Invalid Signature Recovery

**Symptom**: `make release-verify-signatures` or `make release-upload` fails.

Possible causes include changed manifests or archives, missing or invalid
signatures, unavailable verification tools, or incorrect independent trust
inputs. File timestamps alone do not establish validity.

**Diagnosis**: Preserve the failing set and verify the original manifests and
all four signatures with the independently approved trust inputs:

```bash
GONEAT_RELEASE_TAG=vX.Y.Z make release-verify-checksums
GONEAT_RELEASE_TAG=vX.Y.Z make release-verify-signatures
```

**Recovery**:

Retain the failed files and diagnostics. If a fresh local set is needed, use a
different new or empty directory with `scripts/download-release-assets.sh`,
verify both original manifests, and request approval before signing. Do not
regenerate CI manifests, discard failed evidence or overwrite existing outputs.
Different existing remote signatures remain a hard stop, not a clobber action.

**Prevention**: `release-checksums` refuses existing manifests; signature
verification and supplement upload fail closed on missing or invalid inputs.

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
- `make release-upload` - Verify and upload missing signatures, public keys and notes only; preserve original archives/manifests and do not update package managers

**Scripts:**

- `scripts/build-all.sh` - Multi-platform build orchestration
- `scripts/package-artifacts.sh` - Archive creation and checksums
- `scripts/release-tag.sh` - Create, verify and push the signed release tag (tests: `scripts/release_tag_test.go`)
- `scripts/push-to-remotes.sh` - Push main and the verified release tag to all configured remotes
- `scripts/generate-release-notes.sh` - Release notes generation

### Future Automation (Planned)

- GitHub Actions: Automated builds on tag push
- Automated release creation
- ✅ Verified supplement upload (`make release-upload`; CI publishes archives/manifests)
- ✅ Homebrew formula updates (v0.3.10: `make update-homebrew-formula`)
- ✅ Scoop manifest updates (v0.5.7: `make update-scoop-manifest`)
- Native `goneat formula` command (v0.3.11+: multi-package-manager support)
- Winget manifest updates (future)
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

# 4. Open a pull request and merge it to main, then tag the merged main
#    with the signed-tag steps in "3. Tag and Push" above:
#    make release-tag, make release-tag-verify, make release-tag-push
```

## Best Practices Summary

**DO:**

- ✅ Use `make` targets for all operations
- ✅ Run `make prepush` before pushing
- ✅ Test Tier 2 integration before any release
- ✅ Update all documentation before tagging
- ✅ Verify license audit passes
- ✅ Sign all release artifacts (v0.3.4+)
- ✅ Verify both original CI checksum manifests and all four signatures before supplement upload
- ✅ Use `make release-upload` only for verified supplements; preserve original asset IDs and bytes
- ✅ Wait for pkg.go.dev indexing before announcing

**DON'T:**

- ❌ Use standalone `go test`, `golangci-lint`, etc.
- ❌ Skip `make prepush` validation
- ❌ Tag before validation passes
- ❌ Push without running full test suite
- ❌ Release with failing license audit
- ❌ Skip documentation updates
- ❌ Regenerate downloaded CI manifests or clobber published archives/manifests
- ❌ Treat signature upload as authorization for package-manager changes

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

**Document Version**: 2.3 (Best Practice Reference Guide)
**Last Updated**: 2026-10-07 (original CI manifest verification and supplement-only signature upload)
**Next Review**: With each major release or significant process change
**Format**: General reference (not version-specific checklist)
