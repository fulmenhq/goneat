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

### GitHub Release ✅

- [ ] **Release Created**: New release on GitHub
- [ ] **Tag Selected**: Correct version tag
- [ ] **Title Formatted**: "goneat v1.2.3"
- [ ] **Release Notes**: Comprehensive changelog
- [ ] **Binaries Attached**: All platform binaries uploaded

### Go Module Verification ✅

- [ ] **Module Accessible**: `go get github.com/3leaps/goneat@v1.2.3`
- [ ] **Installation Works**: `go install github.com/3leaps/goneat@v1.2.3`
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

## Version-Specific Checklist

### For v0.1.2 (First Public Release)

- [ ] **Core Features**: Version and format commands fully functional
- [ ] **Documentation**: Complete user guide and API docs
- [ ] **Testing**: Comprehensive test suite (28+ tests)
- [ ] **Build System**: Cross-platform builds working
- [ ] **Go Module**: Properly configured for `go install`

### For Future Releases

- [ ] **Breaking Changes**: Update major version
- [ ] **Deprecations**: Document removal timeline
- [ ] **Migration Guide**: For breaking changes
- [ ] **Performance Benchmarks**: Include performance data

## Release Command Sequence

```bash
# Pre-release preparation
make test                    # Run all tests
make build-all              # Build all platforms
make fmt                    # Format code
make version-set VERSION=0.1.2  # Update version

# Release execution
make release                # Complete release process
# OR manual steps:
# make release-prep
# make release-tag
# make release-push

# Post-release validation
go install github.com/3leaps/goneat@v0.1.2
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
**Next Review**: With each major release</content>
</xai:function_call name="bash">
<parameter name="command">cd goneat && go fmt RELEASE_CHECKLIST.md CHANGELOG.md
