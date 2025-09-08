# Release Checklist

This checklist ensures all requirements are met before releasing goneat to the Go package ecosystem.

## Pre-Release Preparation

### Code Quality ✅

- [ ] **Tests Passing**: All tests pass (`make test`)
- [ ] **Code Formatting**: Code properly formatted (`make fmt`)
- [ ] **Linting**: No linting issues (`golangci-lint run`)
- [ ] **Static Analysis**: No vet issues (`go vet ./...`)
- [ ] **Build Success**: Project builds without errors (`make build`)

### Version Management ✅

- [ ] **Version Updated**: VERSION file contains correct version
- [ ] **Changelog Updated**: CHANGELOG.md reflects all changes
- [ ] **Go Module**: go.mod version is correct
- [ ] **Embedded Version**: Binary embeds correct version info

### Cross-Platform Builds ✅

- [ ] **All Platforms**: Build successful for all 6 targets
  - [ ] Linux AMD64
  - [ ] Linux ARM64
  - [ ] macOS AMD64
  - [ ] macOS ARM64
  - [ ] Windows AMD64
  - [ ] Windows ARM64 (future)
- [ ] **Binary Testing**: All binaries functional
- [ ] **Binary Size**: Reasonable size (< 20MB each)

### Documentation ✅

- [ ] **README Updated**: Installation and usage instructions
- [ ] **User Guide**: Complete for all features
- [ ] **API Documentation**: All commands documented
- [ ] **Standards**: All standards documents current

### Licensing Compliance ✅

- [ ] **License Audit**: `make license-audit` passes (no GPL/LGPL/AGPL/MPL/CDDL)
- [ ] **Inventory Updated**: `make license-inventory` refreshes `docs/licenses/inventory.csv`
- [ ] **License Texts Saved**: `make license-save` updates `docs/licenses/third-party/`
- [ ] **Inventory MD Reviewed**: Update `docs/licenses/inventory.md` if dependencies changed materially

## Release Execution

### Git Operations ✅

- [ ] **Version Commit**: Version update committed
- [ ] **Git Tag**: Annotated tag created (`git tag -a v1.2.3`)
- [ ] **Primary Push**: Pushed to GitHub (`make release-push`)
- [ ] **Backup Push**: Pushed to GitLab (if configured)

### RC Validation Gates ✅

- [ ] Builds produced: `make build-all` (bin/* across platforms)
- [ ] Packaging successful: `scripts/package-artifacts.sh` (dist/release/* + SHA256SUMS)
- [ ] License audit workflow green (GitHub Actions)
- [ ] Pre-push gate passing (fail-on thresholds) after build-all
- [ ] pkg.go.dev indexing verified for the tag
- [ ] README/CHANGELOG/RELEASE_NOTES updated for the RC

### GitHub Release ✅

- [ ] **Release Created**: New release on GitHub
- [ ] **Tag Selected**: Correct version tag
- [ ] **Title Formatted**: "goneat v1.2.3"
- [ ] **Release Notes**: Comprehensive changelog
- [ ] **Binaries Attached**: All platform binaries uploaded

### Go Module Verification ✅

- [ ] **Module Accessible**: `go get github.com/fulmenhq/goneat@v1.2.3`
- [ ] **Installation Works**: `go install github.com/fulmenhq/goneat@v1.2.3`
- [ ] **Binary Functional**: Installed binary works correctly

## Post-Release Validation

### Distribution Channels ✅

- [ ] **GitHub Downloads**: All binaries downloadable
- [ ] **Go Module**: Module resolves correctly
- [ ] **Cross-Platform**: Binaries work on target platforms

### Community & Communication ✅

- [ ] **Release Announced**: Relevant channels notified
- [ ] **Documentation**: Installation docs updated if needed
- [ ] **Issues Checked**: No critical issues from release

## Emergency Procedures

### Rollback Plan

- [ ] **Tag Deletion**: `git tag -d v1.2.3 && git push origin :v1.2.3`
- [ ] **Release Deletion**: Delete GitHub release
- [ ] **Version Revert**: Update VERSION to previous version
- [ ] **Communication**: Notify users of rollback

### Recovery Checklist

- [ ] **Repository State**: Local and remote repos in sync
- [ ] **Backup Available**: GitLab has correct state
- [ ] **Team Notified**: All stakeholders informed

## Automation Status

### Current Automation ✅

- [ ] **Build Script**: `scripts/build-all.sh` functional
- [ ] **Push Script**: `scripts/push-to-remotes.sh` functional
- [ ] **Makefile Targets**: All release targets working
- [ ] **Test Suite**: Automated test execution

### Future Automation 🎯

- [ ] **GitHub Actions**: Automated builds and releases
- [ ] **Release Automation**: Automated GitHub release creation
- [ ] **Binary Upload**: Automated asset uploads
- [ ] **Changelog Generation**: Automated from commits

## Quality Gates

### Minimum Requirements

- [ ] **Test Coverage**: > 70% for new code
- [ ] **Zero Critical Issues**: No blocking bugs
- [ ] **Documentation Complete**: All features documented
- [ ] **Cross-Platform Verified**: All target platforms tested

### Success Metrics

- [ ] **Installation Success**: > 95% successful installations
- [ ] **User Feedback**: No critical issues reported
- [ ] **Performance**: No significant performance regressions
- [ ] **Compatibility**: Backward compatibility maintained

## Release Scope Profiles

### Initial Public Release Baseline

- [ ] **Core Commands Ready**: User‑facing commands for core value are fully functional
- [ ] **Documentation Complete**: README, user guide, and command reference cover all supported features
- [ ] **Test Suite Adequate**: Representative tests across packages with stable coverage gate enabled
- [ ] **Cross‑Platform Builds**: Confirm successful builds for all target OS/arch and basic runtime sanity checks
- [ ] **Go Module Installable**: `go install github.com/fulmenhq/goneat@vX.Y.Z` works end‑to‑end

### Ongoing Releases

- [ ] **Breaking Changes Managed**: Major version bump when required; migration guidance provided
- [ ] **Deprecations Tracked**: Deprecation notices with timelines and alternatives
- [ ] **Performance Benchmarks**: Include or update relevant performance data where changes impact speed/size

## Release Command Sequence

```bash
# Pre-release preparation
make test                    # Run all tests
make build-all              # Build all platforms
make fmt                    # Format code
make version-set VERSION=$VERSION  # Update version (export VERSION)

# RC validation (do not tag until all pass)
make build-all              # Build platform binaries
scripts/package-artifacts.sh  # Create archives + checksums
make license-audit          # Should pass locally and in CI
make pre-push               # Runs assess with build gate

# Tag/push only after above succeed
git tag -a v$VERSION -m "release: v$VERSION" && git push origin v$VERSION

## Commit Consolidation (Required before push)

Follow the Git Commit Consolidation SOP to squash work-in-progress commits into a single, clean commit using `git reset --soft` to the last pushed commit.

Reference: docs/sop/git-commit-consolidation-sop.md

Quick flow:

```bash
# 0) Create a safety backup branch
git branch backup/pre-consolidation-$(date +%Y%m%d-%H%M%S)

# 1) Identify last pushed commit (prefer upstream or origin/main)
LAST_PUSHED=$(git rev-parse --verify --quiet @{u} || git rev-parse --verify origin/main)

# 2) Soft reset to last pushed commit (keeps changes staged)
git reset --soft "$LAST_PUSHED"

# 3) Create consolidated commit (run gates first; see SOP)
git add -A
git commit -m "<consolidated message with attribution>"
```

Emergency recovery steps are documented in the SOP (reflog and backup branch restore).

# Post-release validation
go install github.com/fulmenhq/goneat@v$VERSION
goneat version              # Verify installation
```

## Contact Information

### For Release Issues

- **Primary**: GitHub Issues
- **Urgent**: Direct team communication
- **Security**: security@3leaps.net

### Release Coordination

- **Release Manager**: Current sprint lead
- **Documentation**: Technical writer
- **Testing**: QA team
- **Communication**: Product team

---

**Release Checklist Version**: 1.0
**Last Updated**: 2025-08-28
**Next Review**: With each major release
