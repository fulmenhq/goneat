# Goneat Makefile
# Dogfooding: This Makefile uses goneat itself for formatting (when available)
# SSOT Version: Use our own version command for version management (dogfooding!)
# Quick Start Commands:
#   make help           - Show all available commands
#   make build          - Build the binary
#   make test           - Run tests (when added)
#   make dev            - Set up development environment
#   make version-get    - Show current version
#   make version-bump-patch - Bump patch version

# Variables
BINARY_NAME := goneat
# Detect Windows platform and add .exe extension
ifeq ($(OS),Windows_NT)
	BINARY_NAME := goneat.exe
	BINARY_NAME_NOEXT := goneat
else
	BINARY_NAME_NOEXT := goneat
endif
VERSION := $(shell [ -f dist/$(BINARY_NAME_NOEXT) ] && ./dist/$(BINARY_NAME_NOEXT) version --project --json 2>/dev/null | jq -r '.project.version' 2>/dev/null || cat VERSION 2>/dev/null || echo "0.1.0")
BUILD_DIR := dist
SRC_DIR := .

# Go related variables
GOCMD := go
GOBUILD := $(GOCMD) build
GOCLEAN := $(GOCMD) clean
GOTEST := $(GOCMD) test
GOMOD := $(GOCMD) mod
GOFMT := $(GOCMD) fmt

# Test parallelization (default 1 for CI, override locally)
# Supports both: export GONEAT_TEST_PARALLEL=3 AND make test GONEAT_TEST_PARALLEL=3
ifndef GONEAT_TEST_PARALLEL
GONEAT_TEST_PARALLEL := 1
endif

# Build flags
# Embed binary version, build time, and git commit for `go install` builds as well
LDFLAGS := -ldflags "\
	-X 'github.com/fulmenhq/goneat/pkg/buildinfo.BinaryVersion=$(VERSION)' \
	-X 'github.com/fulmenhq/goneat/pkg/buildinfo.BuildTime=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")' \
	-X 'github.com/fulmenhq/goneat/pkg/buildinfo.GitCommit=$(shell git rev-parse HEAD 2>/dev/null || echo "unknown")'"
BUILD_FLAGS := $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)

.PHONY: help build clean clean-all test fmt format-docs format-config format-root format-all version version-bump-patch version-bump-minor version-bump-major version-set version-set-prerelease license-inventory license-save license-audit update-licenses embed-assets verify-embeds prerequisites sync-crucible sync-ssot verify-crucible verify-crucible-clean bootstrap tools lint release-check release-prepare release-build check-all prepush precommit update-homebrew-formula

# Default target
all: clean build format-all

# Help target
help: ## Show this help message
	@echo "Goneat - Available Make Targets"
	@echo ""
	@echo "License targets:"
	@echo "  license-inventory   - Generate CSV inventory of dependency licenses"
	@echo "  license-save        - Save third-party license texts to docs/licenses/third-party"
	@echo "  license-audit       - Fail if forbidden licenses (GPL/LGPL/AGPL/MPL/CDDL) detected"
	@echo "  update-licenses     - Alias: inventory + save"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'
	@echo ""

# Build targets
build: embed-assets ## Build the binary
	@echo "Building $(BINARY_NAME) $(VERSION)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(BUILD_FLAGS) ./$(SRC_DIR)
	@echo "✅ Build completed: $(BUILD_DIR)/$(BINARY_NAME)"

embed-assets: ## Sync templates/ and schemas/ into embedded assets (SSOT -> internal/assets)
	@echo "Embedding assets (templates/, schemas/)..."
	@chmod +x scripts/embed-assets.sh
	@./scripts/embed-assets.sh
	@echo "✅ Assets embedded"
	@echo "ℹ️  Note: Uses 'go run' to invoke content embed without requiring prebuilt binary"

verify-embeds: ## Verify embedded mirrors match SSOT (fails on drift)
	@chmod +x scripts/verify-embeds.sh
	@./scripts/verify-embeds.sh
	@echo "ℹ️  Note: Uses 'go run' to verify content without chicken-and-egg dependency"

# Cross-platform build targets
build-all: ## Build for all supported platforms
	@echo "Building $(BINARY_NAME) v$(VERSION) for all platforms..."
	@./scripts/build-all.sh
	@echo "✅ Cross-platform builds completed"

sync-schemas: ## Fetch curated JSON Schema meta-schemas (network required)
	@chmod +x scripts/sync-schemas.sh
	@./scripts/sync-schemas.sh

# Crucible SSOT sync (dogfooding goneat ssot command, following fuldx pattern)
sync-crucible: build ## Sync documentation and schemas from crucible repository (SSOT)
	@echo "🔄 Syncing Crucible Go assets..."
	@$(BUILD_DIR)/$(BINARY_NAME) ssot sync

sync-ssot: sync-crucible ## Alias for sync-crucible (clarity)

verify-crucible: build ## Verify that crucible content is up-to-date
	@echo "🔍 Verifying Crucible sync..."
	@$(BUILD_DIR)/$(BINARY_NAME) ssot sync --dry-run >/dev/null 2>&1
	@if git diff --exit-code docs/crucible-go schemas/crucible-go >/dev/null 2>&1; then \
		echo "✅ Crucible content is up-to-date"; \
	else \
		echo "❌ Crucible content is stale - run 'make sync-crucible'"; \
		exit 1; \
	fi

verify-crucible-clean: ## Verify crucible sources are clean (no uncommitted changes)
	@chmod +x scripts/verify-crucible-clean.sh
	@./scripts/verify-crucible-clean.sh

bootstrap: build ## Install foundation scope (auto-installs user-local brew/scoop as needed)
	@echo "🥾 Installing foundation tools via goneat doctor tools..."
	@if [ -f "$(BUILD_DIR)/$(BINARY_NAME)" ]; then \
		$(BUILD_DIR)/$(BINARY_NAME) doctor tools --scope foundation --install --yes --no-cooling && \
		echo "✅ Foundation tools installed"; \
	else \
		echo "❌ goneat binary not found, cannot install tools"; \
		exit 1; \
	fi

tools: build ## Verify external tools are present; may be a no-op if none are required
	@echo "🔧 Verifying external tools..."
	@if [ -f "$(BUILD_DIR)/$(BINARY_NAME)" ]; then \
		$(BUILD_DIR)/$(BINARY_NAME) doctor tools --scope all; \
		echo "✅ Tools verification completed"; \
	else \
		echo "❌ goneat binary not found, cannot verify tools"; \
		exit 1; \
	fi

lint: build ## Run lint/format/style checks
	@echo "🔍 Running lint checks..."
	@if [ -f "$(BUILD_DIR)/$(BINARY_NAME)" ]; then \
		$(BUILD_DIR)/$(BINARY_NAME) assess --categories=lint; \
		echo "✅ Lint checks completed"; \
	else \
		echo "❌ goneat binary not found, cannot run lint checks"; \
		exit 1; \
	fi

install-probe: ## Opt-in package-manager/tool availability probe (requires network; uses build tag installprobe)
	GONEAT_INSTALL_PROBE=1 go test -tags=installprobe ./internal/doctor

release-prepare: build sync-crucible embed-assets ## Prepare release (sync SSOT, embed assets, build binary)
	@echo "🚀 Preparing release environment..."
	@echo "✅ Release preparation complete (SSOT synced, assets embedded, binary built)"

release-check: release-prepare ## Validate release readiness (tests, lint, crucible, licenses)
	@echo "🔍 Running release checklist validation..."
	$(MAKE) test
	$(MAKE) lint
	$(MAKE) verify-crucible
	$(MAKE) license-audit
	@echo "✅ Release checklist validation passed"

package: ## Package binaries into distribution archives (dist/release/*.tar.gz, *.zip, SHA256SUMS)
	@echo "📦 Packaging release artifacts..."
	@./scripts/package-artifacts.sh
	@echo "✅ Release artifacts packaged in dist/release/"

release-build: build-all package ## Build release artifacts (binaries + checksums) for distribution
	@echo "📦 Release build completed"

check-all: build ## Run all checks (lint, test, typecheck)
	@echo "🔍 Running all checks..."
	@if [ -f "$(BUILD_DIR)/$(BINARY_NAME)" ]; then \
		$(BUILD_DIR)/$(BINARY_NAME) assess; \
		echo "✅ All checks completed"; \
	else \
		echo "❌ goneat binary not found, cannot run checks"; \
		exit 1; \
	fi



# Release notes artifact from RELEASE_NOTES.md
release-notes: ## Generate release notes artifact (dist/release/release-notes-v<version>.md)
	@echo "📝 Generating release notes for $(VERSION)..."
	@chmod +x scripts/generate-release-notes.sh
	@./scripts/generate-release-notes.sh
	@echo "✅ Release notes generated (dist/release)"

build-linux-amd64: ## Build for Linux AMD64
	@echo "Building for Linux AMD64..."
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./$(SRC_DIR)

build-linux-arm64: ## Build for Linux ARM64
	@echo "Building for Linux ARM64..."
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./$(SRC_DIR)

build-darwin-amd64: ## Build for macOS AMD64
	@echo "Building for macOS AMD64..."
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./$(SRC_DIR)

build-darwin-arm64: ## Build for macOS ARM64
	@echo "Building for macOS ARM64..."
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./$(SRC_DIR)

build-windows-amd64: ## Build for Windows AMD64
	@echo "Building for Windows AMD64..."
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(BUILD_FLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./$(SRC_DIR)

# Clean targets
# NOTE: internal/assets/embedded_* directories are NOT cleaned - they contain embedded assets
# that allow `goneat docs list/show` to display docs/schemas/config without requiring the repo.
# These directories are committed to git and synced via `make embed-assets` from SSOT sources.
clean: ## Clean build artifacts, test cache, and generated files
	@echo "Cleaning build artifacts..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	@echo "Cleaning test cache..."
	go clean -testcache
	@echo "Cleaning vendor directory..."
	rm -rf vendor/
	@echo "Cleaning coverage files..."
	@find . -name "coverage.out" -type f -delete 2>/dev/null || true
	@find . -name "*.test" -type f -delete 2>/dev/null || true
	@rm -f coverage.out
	@echo "Cleaning OS metadata files..."
	@find . -name ".DS_Store" -type f -delete 2>/dev/null || true
	@echo "Cleaning backup files..."
	@find . -name "*.backup" -type f -delete 2>/dev/null || true
	@echo "✅ Clean completed"

clean-all: clean ## Deep clean including Go build cache (slow - use before major updates)
	@echo "🧹 Deep cleaning Go build cache..."
	go clean -cache
	@echo "🧹 Cleaning user coverage directory..."
	@rm -rf $(HOME)/.goneat/coverage/
	@echo "✅ Deep clean completed (next build will be slower)"

# Test targets
test: test-unit test-integration-cooling-synthetic ## Run all tests (unit + Tier 1 integration)
	@echo "✅ Test suite completed"

test-unit: ## Run unit tests only
	@echo "Running unit tests..."
	GONEAT_OFFLINE_SCHEMA_VALIDATION=true GONEAT_GUARDIAN_TEST_MODE=true GONEAT_GUARDIAN_AUTO_DENY=true $(GOTEST) ./... -v -timeout 15m -parallel $(GONEAT_TEST_PARALLEL)

test-integration: ## Run integration tests only
	@echo "Running integration tests..."
	GONEAT_OFFLINE_SCHEMA_VALIDATION=true GONEAT_GUARDIAN_TEST_MODE=true GONEAT_GUARDIAN_AUTO_DENY=true $(GOTEST) ./tests/integration/... -v -timeout 15m

test-integration-cooling-synthetic: ## Run cooling policy integration test (synthetic fixture only, CI-friendly)
	@echo "Running cooling policy integration test (synthetic fixture)..."
	$(GOTEST) ./pkg/dependencies/... -tags=integration -run TestCoolingPolicy_Synthetic -v -timeout=5m

test-integration-cooling: ## Run cooling policy integration tests (requires GONEAT_COOLING_TEST_ROOT or repos in ~/dev/playground)
	@echo "Running cooling policy integration tests..."
	@echo "⚠️  This requires test repositories. Set GONEAT_COOLING_TEST_ROOT or clone repos to ~/dev/playground"
	@echo "📚 See docs/appnotes/lib/dependencies/TEST_EXECUTION_GUIDE.md for setup instructions"
	$(GOTEST) ./pkg/dependencies/... -tags=integration -v -timeout=15m

test-integration-cooling-quick: ## Quick cooling policy test (Hugo baseline only)
	@echo "Running quick cooling policy test (Hugo baseline)..."
	@echo "⚠️  This requires Hugo repository. Set GONEAT_COOLING_TEST_ROOT or clone to ~/dev/playground"
	$(GOTEST) ./pkg/dependencies/... -tags=integration -run TestCoolingPolicy_Hugo_Baseline -v -timeout=5m

test-integration-extended: ## Run extended integration tests (Tier 1 + Tier 2 + Tier 3, comprehensive)
	@echo "Running extended integration tests (all tiers)..."
	@echo "⚠️  This requires test repositories. Set GONEAT_COOLING_TEST_ROOT or clone repos to ~/dev/playground"
	@$(MAKE) test-integration-cooling-synthetic
	@$(MAKE) test-integration-cooling-quick
	@$(MAKE) test-integration-cooling

test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	@mkdir -p $(HOME)/.goneat/coverage
	GONEAT_OFFLINE_SCHEMA_VALIDATION=true GONEAT_GUARDIAN_TEST_MODE=true GONEAT_GUARDIAN_AUTO_DENY=true $(GOTEST) ./... -coverprofile=$(HOME)/.goneat/coverage/coverage.out
	go tool cover -html=$(HOME)/.goneat/coverage/coverage.out -o $(HOME)/.goneat/coverage/coverage.html
	@echo "Coverage report: $(HOME)/.goneat/coverage/coverage.html"
	@echo "Coverage data: $(HOME)/.goneat/coverage/coverage.out"

# Coverage gating based on lifecycle phase
coverage-check: test-coverage ## Enforce coverage threshold based on lifecycle phase
	@lifecycle=$$(tr '[:upper:]' '[:lower:]' < LIFECYCLE_PHASE 2>/dev/null || echo alpha); \
	release=$$(tr '[:upper:]' '[:lower:]' < RELEASE_PHASE 2>/dev/null || echo ""); \
	case $$lifecycle in \
	  experimental) threshold=0;; \
	  alpha) threshold=30;; \
	  beta) threshold=60;; \
	  rc) threshold=70;; \
	  ga) threshold=75;; \
	  lts) threshold=80;; \
	  *) threshold=30;; \
	 esac; \
	if [ -n "$$release" ]; then \
	  phase_label="lifecycle=$$lifecycle,release=$$release"; \
	else \
	  phase_label="lifecycle=$$lifecycle"; \
	fi; \
	if [ ! -f $(HOME)/.goneat/coverage/coverage.out ]; then echo "❌ coverage.out not found. Run make test-coverage first"; exit 1; fi; \
	total=$$(go tool cover -func=$(HOME)/.goneat/coverage/coverage.out | awk '/^total:/ {gsub(/%/,"",$$3); print $$3}'); \
	awk -v cov="$$total" -v thr="$$threshold" -v ph="$$phase_label" 'BEGIN { \
	  cov+=0; thr+=0; \
	  if (cov >= thr) { printf "✅ Coverage %.1f%% meets threshold %.1f%% (%s)\n", cov, thr, ph; exit 0 } \
	  else { printf "❌ Coverage %.1f%% below threshold %.1f%% (%s)\n", cov, thr, ph; exit 1 } \
	}'

# Format targets
fmt: build ## Format code using goneat (dogfooding)
	@echo "Formatting code with goneat..."
	@if [ -f "$(BUILD_DIR)/$(BINARY_NAME)" ]; then \
		$(BUILD_DIR)/$(BINARY_NAME) format cmd/ pkg/ main.go; \
		echo "✅ Formatting completed with goneat"; \
	else \
		echo "❌ goneat binary not found, falling back to go fmt"; \
		go fmt ./cmd/... ./pkg/... ./main.go; \
	fi

format-docs: build ## Format documentation files using goneat (dogfooding)
	@echo "Formatting documentation with goneat..."
	@if [ -f "$(BUILD_DIR)/$(BINARY_NAME)" ]; then \
		$(BUILD_DIR)/$(BINARY_NAME) format --types yaml,json,markdown --folders docs/; \
		echo "✅ Documentation formatting completed with goneat"; \
	else \
		echo "❌ goneat binary not found, falling back to manual formatting"; \
		echo "Please install yamlfmt, jq, and prettier for documentation formatting"; \
	fi

format-config: build ## Format configuration and schema files using goneat (dogfooding)
	@echo "Formatting configuration and schema files with goneat..."
	@if [ -f "$(BUILD_DIR)/$(BINARY_NAME)" ]; then \
		$(BUILD_DIR)/$(BINARY_NAME) format --types yaml,json --folders config/ schemas/; \
		echo "✅ Configuration formatting completed with goneat"; \
	else \
		echo "❌ goneat binary not found, cannot format config files"; \
		echo "Please install yamlfmt and jq for configuration formatting"; \
	fi

format-root: build ## Format root-level markdown files (README, CHANGELOG, etc.)
	@echo "Formatting root markdown files with goneat..."
	@if [ -f "$(BUILD_DIR)/$(BINARY_NAME)" ]; then \
		$(BUILD_DIR)/$(BINARY_NAME) format --types markdown *.md .github/; \
		echo "✅ Root markdown formatting completed with goneat"; \
	else \
		echo "❌ goneat binary not found, cannot format root files"; \
	fi

format-all: fmt format-docs format-config format-root ## Format all code, documentation, and configuration files

# License compliance
license-inventory: ## Generate CSV inventory of dependency licenses
	@echo "🔎 Generating license inventory (CSV)..."
	@mkdir -p docs/licenses dist/reports
	@if ! command -v go-licenses >/dev/null 2>&1; then \
		echo "Installing go-licenses..."; \
		GOBIN=$$(go env GOPATH)/bin go install github.com/google/go-licenses@latest; \
	fi
	@go-licenses csv . | tee docs/licenses/inventory.csv >/dev/null
	@echo "✅ Wrote docs/licenses/inventory.csv"

license-save: ## Save third-party license texts (for distribution)
	@echo "📄 Saving third-party license texts..."
	@rm -rf docs/licenses/third-party
	@if ! command -v go-licenses >/dev/null 2>&1; then \
		echo "Installing go-licenses..."; \
		GOBIN=$$(go env GOPATH)/bin go install github.com/google/go-licenses@latest; \
	fi
	@go-licenses save . --save_path=docs/licenses/third-party
	@echo "✅ Saved third-party licenses to docs/licenses/third-party"

license-audit: ## Audit dependencies for forbidden licenses; fail on detection
	@echo "🧪 Auditing dependency licenses..."
	@if ! command -v go-licenses >/dev/null 2>&1; then \
		echo "Installing go-licenses..."; \
		GOBIN=$$(go env GOPATH)/bin go install github.com/google/go-licenses@latest; \
	fi
	@mkdir -p dist/reports; \
	forbidden='GPL|LGPL|AGPL|MPL|CDDL'; \
		out=$$(go-licenses csv .); \
		echo "$$out" > dist/reports/license-inventory.csv; \
		if echo "$$out" | rg "$$forbidden" >/dev/null; then \
			echo "❌ Forbidden license detected. See dist/reports/license-inventory.csv"; \
			echo "   Forbidden patterns: $$forbidden"; \
			exit 1; \
		else \
			echo "✅ No forbidden licenses detected"; \
		fi

update-licenses: license-inventory license-save ## Update license inventory and third-party texts

# Hook targets (dogfooding)
precommit: build test ## Run pre-commit hooks (stub for now)
	@echo "Running pre-commit checks with goneat..."
	@if [ -f "$(BUILD_DIR)/$(BINARY_NAME)" ]; then \
		$(BUILD_DIR)/$(BINARY_NAME) assess --hook pre-commit; \
		echo "✅ Pre-commit checks passed"; \
	else \
		echo "❌ goneat binary not found, cannot run pre-commit checks"; \
		exit 1; \
	fi

prepush: release-check ## Run comprehensive pre-push validation (prepare + check)
	@echo "Running pre-push checks with goneat..."
	$(MAKE) verify-crucible-clean
	$(MAKE) build-all
	@if [ -f "$(BUILD_DIR)/$(BINARY_NAME)" ]; then \
		GONEAT_OFFLINE_SCHEMA_VALIDATION=false $(BUILD_DIR)/$(BINARY_NAME) assess --hook pre-push; \
		echo "✅ Pre-push checks passed"; \
	else \
		echo "❌ goneat binary not found, cannot run pre-push checks"; \
		exit 1; \
	fi

# Development setup
prerequisites: ## Check and install required development tools using goneat
	@echo "🔧 Checking development prerequisites..."
	@if [ ! -f "$(BUILD_DIR)/$(BINARY_NAME)" ]; then \
		echo "⚠️  goneat binary not found, building first..."; \
		$(MAKE) embed-assets; \
		$(GOBUILD) $(BUILD_FLAGS) ./$(SRC_DIR); \
	fi
	@echo "📋 Checking Go toolchain..."
	@if ! command -v go >/dev/null 2>&1; then \
		echo "❌ Go toolchain not found in PATH"; \
		echo "🔧 Please install Go: https://golang.org/dl/"; \
		echo "💡 For macOS: brew install go"; \
		echo "💡 Add to PATH: export PATH=\"$$PATH:$$(go env GOPATH)/bin\""; \
		echo "🔄 After installing Go, restart your shell and re-run this command"; \
		exit 1; \
	fi
	@echo "📦 Installing development tools using goneat..."
	@$(BUILD_DIR)/$(BINARY_NAME) doctor tools --scope all --install --yes || true
	@echo "✅ Prerequisites check complete"
	@echo "💡 If tools were installed but not found, you may need to:"
	@echo "   export PATH=\"$$PATH:$$(go env GOPATH)/bin\""
	@echo "   source ~/.zshrc  # or ~/.bashrc on Linux"

dev: prerequisites ## Set up development environment
	@echo "Setting up development environment..."
	$(MAKE) build
	$(MAKE) format-all
	@echo "✅ Development environment ready"

# Version management targets
version: ## Print current repository version from VERSION
	@echo "Current version: $(VERSION)"

version-bump-patch: ## Bump version according to strategy (SemVer or CalVer) and regenerate derived files
	@echo "Bumping patch version using goneat version command..."
	@./dist/goneat version bump patch
	@echo "✅ Patch version bumped"

version-bump-minor: ## Bump version according to strategy (SemVer or CalVer) and regenerate derived files
	@echo "Bumping minor version using goneat version command..."
	@./dist/goneat version bump minor
	@echo "✅ Minor version bumped"

version-bump-major: ## Bump version according to strategy (SemVer or CalVer) and regenerate derived files
	@echo "Bumping major version using goneat version command..."
	@./dist/goneat version bump major
	@echo "✅ Major version bumped"

version-set: ## Update VERSION and any derived metadata (usage: make version-set VERSION=x.y.z)
	@if [ -z "$(VERSION)" ]; then \
		echo "❌ Usage: make version-set VERSION=x.y.z"; \
		exit 1; \
	fi
	@echo "$(VERSION)" > VERSION
	@echo "✅ Version set to: $(VERSION)"

version-set-prerelease: ## Set prerelease version (usage: make version-set-prerelease VERSION_SET=x.y.z-rc.N)
	@if [ -z "$(VERSION_SET)" ]; then \
		echo "❌ Usage: make version-set-prerelease VERSION_SET=x.y.z-rc.N"; \
		exit 1; \
	fi
	@echo "$(VERSION_SET)" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+-(rc\.[0-9]+|beta\.[0-9]+|alpha\.[0-9]+)$$' || (echo "❌ Invalid prerelease format. Expected x.y.z-(rc|beta|alpha).N" && exit 1)
	@echo "$(VERSION_SET)" > VERSION
	@echo "✅ Prerelease version set: $(VERSION_SET)"

# Release management targets
release-prep: ## Prepare for release (run tests, coverage gate, build, etc.)
	@echo "🚀 Preparing for release v$(VERSION)..."
	$(MAKE) test-coverage
	$(MAKE) coverage-check
	$(MAKE) build-all
	$(MAKE) format-all
	$(MAKE) release-notes
	@echo "✅ Release preparation complete"

release-tag: ## Create git tag for release
	@echo "🏷️  Creating release tag $(VERSION)..."
	git tag -a $(VERSION) -m "Release $(VERSION)"
	@echo "✅ Tag created: $(VERSION)"

release-push: ## Push release to all remotes
	@echo "📤 Pushing release to all remotes..."
	./scripts/push-to-remotes.sh
	@echo "✅ Release pushed to all remotes"

release-upload: ## Upload signed release artifacts to GitHub (requires dist/release/*.asc signatures)
	@echo "📤 Uploading release artifacts to GitHub $(VERSION)..."
	@if [ ! -f "dist/release/goneat_$(VERSION)_darwin_arm64.tar.gz.asc" ]; then \
		echo "❌ Error: Signature files not found in dist/release/"; \
		echo "   Run signing workflow first (see RELEASE_CHECKLIST.md)"; \
		exit 1; \
	fi
	@echo "   Uploading binaries and checksums..."
	cd dist/release && gh release upload $(VERSION) \
		goneat_$(VERSION)_*.tar.gz \
		goneat_$(VERSION)_*.zip \
		SHA256SUMS \
		--clobber
	@echo "   Uploading signatures..."
	cd dist/release && gh release upload $(VERSION) \
		goneat_$(VERSION)_*.asc \
		SHA256SUMS.asc \
		fulmenhq-release-signing-key.asc \
		--clobber
	@echo "   Updating release notes..."
	cd dist/release && gh release edit $(VERSION) --notes-file release-notes-$(VERSION).md
	@echo "✅ Release artifacts uploaded to $(VERSION)"
	@echo ""
	@echo "🔍 Verify upload:"
	@echo "   gh release view $(VERSION)"
	@echo ""
	@echo "📝 Updating Homebrew formula..."
	@$(MAKE) update-homebrew-formula

update-homebrew-formula: ## Update Homebrew formula with new version and checksums (requires ../homebrew-tap)
	@echo "Updating Homebrew formula for $(BINARY_NAME) $(VERSION)..."
	@echo ""
	@echo "ℹ️  Note: This target requires ../homebrew-tap to be cloned as a sibling directory"
	@echo "   Repository: https://github.com/fulmenhq/homebrew-tap"
	@echo "   Expected path: ../homebrew-tap"
	@echo ""
	@if [ ! -d "../homebrew-tap" ]; then \
		echo "❌ Error: ../homebrew-tap directory not found"; \
		echo ""; \
		echo "Please clone the homebrew-tap repository:"; \
		echo "  cd .. && git clone https://github.com/fulmenhq/homebrew-tap.git"; \
		echo ""; \
		echo "Directory structure should be:"; \
		echo "  parent/"; \
		echo "    ├── goneat/           (this repository)"; \
		echo "    └── homebrew-tap/     (sibling repository)"; \
		exit 1; \
	fi
	@if [ ! -f "../homebrew-tap/Formula/$(BINARY_NAME).rb" ]; then \
		echo "❌ Error: Formula file not found: ../homebrew-tap/Formula/$(BINARY_NAME).rb"; \
		exit 1; \
	fi
	@echo "✅ Sibling repository found: ../homebrew-tap"
	@echo "   Calling homebrew-tap update target..."
	@cd ../homebrew-tap && $(MAKE) update-goneat VERSION=$(VERSION)
	@echo ""
	@echo "✅ Homebrew formula updated successfully!"
	@echo ""
	@echo "📋 Next steps:"
	@echo "   1. Review changes:  cd ../homebrew-tap && git diff Formula/$(BINARY_NAME).rb"
	@echo "   2. Test formula:    cd ../homebrew-tap && make test APP=$(BINARY_NAME)"
	@echo "   3. Commit formula:  cd ../homebrew-tap && git add Formula/$(BINARY_NAME).rb && git commit"
	@echo "   4. Push to tap:     cd ../homebrew-tap && git push"

release: release-prep release-tag release-push ## Complete release process
	@echo "🎉 Release v$(VERSION) completed!"
	@echo ""
	@echo "📋 Next steps:"
	@echo "   1. Sign artifacts: cd dist/release && gpg --detach-sign --armor *.tar.gz *.zip SHA256SUMS"
	@echo "   2. Extract public key: gpg --armor --export security@fulmenhq.dev > fulmenhq-release-signing-key.asc"
	@echo "   3. Verify public key: ../../scripts/verify-public-key.sh fulmenhq-release-signing-key.asc"
	@echo "   4. Upload to GitHub: make release-upload"
	@echo "   5. Update Homebrew tap (if applicable)"
	@echo "   6. Announce release in relevant channels"

# Future: goneat-based version management
version-manage: build ## Use goneat for version management (future feature)
	@echo "Version management with goneat (coming soon)..."
	@echo "Current version: $(VERSION)"
	# TODO: Implement goneat version command
	# $(BUILD_DIR)/$(BINARY_NAME) version bump patch
	# $(BUILD_DIR)/$(BINARY_NAME) version set x.y.z
