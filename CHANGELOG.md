# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Initial release preparation

### Changed
- Repository structure and documentation

### Fixed
- Build and test infrastructure

## [0.1.2] - 2025-08-30

### Added
- 🛠️ Hooks Dogfooding & Template Engine
  - Schema-driven hook templates under `templates/hooks/bash/` rendered via `goneat hooks generate`
  - Templates consume `.goneat/hooks.yaml` for args, fallback, and optimization (`only_changed_files`)
  - Dev-mode fallback and robust binary discovery (PATH + repo `dist/` + common locations)
  - Docs updated with setup, output modes, and JSON piping
    - `docs/user-guide/workflows/git-hooks-operation.md`
    - `docs/user-guide/commands/hooks.md`

- 🔎 Concise Hook Output + Pretty Renderer (prototype)
  - New `concise` output format for short, colorized summaries in hooks (top-N files listed)
  - `goneat pretty` (stub) renders JSON to console (concise) or HTML using existing formatter
  - Env override: `GONEAT_HOOK_OUTPUT=concise|markdown|json|html|both`

- ✍️ Format Command Improvements
  - `--staged-only` to operate on staged files (ACMR)
  - `--ignore-missing-tools` to skip YAML/JSON/MD formatting if external tools absent
  - Plan/dry-run works with staged-only (synthesized plan)

- 📚 Environment Variables (SSOT)
  - Added `docs/environment-variables.md` covering `GONEAT_HOOK_OUTPUT`, `NO_COLOR`, `GONEAT_TEMPLATE_PATH`, and future vars

### Changed
- Hook mode output selection:
  - Honors explicit `--format`; otherwise `GONEAT_HOOK_OUTPUT`, else `--verbose` → markdown, else concise
- Reduced runner “failed without error” log noise to debug in hook mode context
- Concise output: fallback to first issue message when no file path is available

### Fixed
- Robust JSON parsing in `goneat pretty` (tolerates log preambles)
- Hook templates prefer repo-local `dist/goneat`; improved fail-fast guidance when missing

### Technical Details
- Taxonomy docs: `docs/architecture/command-taxonomy-validation-adr.md`
- Hook docs: `docs/user-guide/workflows/git-hooks-operation.md`, `docs/user-guide/commands/hooks.md`
- Structured fixtures: `tests/fixtures/` for ongoing lint/format testing

## [0.1.1] - 2025-08-28

### Added
- **Assessment System Enhancement**: Concurrency support for parallel processing
  - Configurable worker count and CPU percentage utilization
  - Improved performance for large codebase assessments
  - JSON-first reporting with HTML fallback

### Changed
- **Report Format**: Enhanced HTML template with better styling and information architecture
- **Assessment Engine**: Format run summaries and improved error handling
- **Git Integration**: Better semver/calver tag detection and validation

### Fixed
- Lint issues across assessment engine and formatter modules
- Static analysis warnings in runner and engine components

## [0.1.0] - 2025-08-28

### Added
- **Version Command**: Complete version management system
  - Multi-source version detection (VERSION files, git tags, Go constants)
  - Version bumping (patch, minor, major)
  - Version setting with validation
  - First-run detection and intelligent setup guidance
  - Git integration with tag creation
  - JSON and extended output formats
  - Assessment mode (`--no-op`) for safe testing

- **Format Command**: Code formatting with Go support
  - Go file formatting using `gofmt`
  - Dry-run and plan-only modes
  - Sequential and parallel execution strategies
  - File discovery and filtering
  - Comprehensive error handling

- **Test Infrastructure**: Enterprise-grade testing framework
  - Integration test suite (28+ tests)
  - Test environment framework (`TestEnv`)
  - Fixture helpers for various scenarios
  - Cross-platform testing support

- **Standards & Documentation**: Comprehensive project standards
  - Document frontmatter standard
  - Copyright template for code files
  - Authoring guidelines and templates
  - Repository safety protocols
  - User guides and API documentation

- **Internal Architecture**: Robust internal systems
  - Operations registry for command management
  - Assessment engine foundation
  - Configuration management system
  - Logger infrastructure

### Changed
- Repository structure optimized for Fulmen ecosystem
- Build system enhanced with cross-platform support
- Error handling improved throughout codebase

### Fixed
- Errcheck issues resolved in test files
- Code formatting consistency improved
- Static analysis warnings addressed

### Technical Details
- **Go Version**: 1.21+
- **Dependencies**: Cobra CLI, Viper config, Testify testing
- **Platforms**: Linux, macOS, Windows (AMD64/ARM64)
- **Test Coverage**: 75%+ of testable code
- **Build System**: Makefile with cross-platform targets

---

## Release Notes Template

When creating a new release, copy this template and fill in the details:

```markdown
## [x.y.z] - YYYY-MM-DD

### Added
- New features and functionality

### Changed
- Modifications to existing functionality

### Deprecated
- Features scheduled for removal

### Removed
- Removed features

### Fixed
- Bug fixes and patches

### Security
- Security-related changes
```

### Version Numbering
- **MAJOR**: Breaking changes (1.0.0 → 2.0.0)
- **MINOR**: New features, backward compatible (1.0.0 → 1.1.0)
- **PATCH**: Bug fixes, backward compatible (1.0.0 → 1.0.1)

### Pre-release Versions
- **Alpha**: `1.1.0-alpha.1` - Early testing
- **Beta**: `1.1.0-beta.1` - Feature complete, testing
- **RC**: `1.1.0-rc.1` - Release candidate

---

## Guidelines

### Contributing to the Changelog

1. **Keep entries brief but descriptive**
2. **Group changes by type** (Added, Changed, Fixed, etc.)
3. **Use present tense** for changes ("Add feature" not "Added feature")
4. **Reference issues/PRs** when applicable
5. **Update on release** - Move unreleased changes to version section

### Release Process

1. Update VERSION file with new version
2. Move unreleased changes to new version section
3. Add release date
4. Commit changes
5. Create git tag
6. Push to all remotes
7. Create GitHub release

---

**Legend:**
- 🎉 Major features and milestones
- 🔧 Technical improvements
- 🐛 Bug fixes
- 📚 Documentation updates
- 🏗️ Infrastructure changes</content>
</xai:function_call name="write">
<parameter name="filePath">goneat/RELEASE_CHECKLIST.md