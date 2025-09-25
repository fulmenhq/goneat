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
VERSION := $(shell [ -f dist/goneat ] && ./dist/goneat version --project --json 2>/dev/null | jq -r '.project.version' 2>/dev/null || cat VERSION 2>/dev/null || echo "0.1.0")
BUILD_DIR := dist
SRC_DIR := .

# Go related variables
GOCMD := go
GOBUILD := $(GOCMD) build
GOCLEAN := $(GOCMD) clean
GOTEST := $(GOCMD) test
GOMOD := $(GOCMD) mod
GOFMT := $(GOCMD) fmt

# Build flags
# Embed binary version for `go install` builds as well
LDFLAGS := -ldflags "-X 'github.com/fulmenhq/goneat/pkg/buildinfo.BinaryVersion=$(VERSION)'"
BUILD_FLAGS := $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)

.PHONY: help build clean test fmt format-docs version-get version-bump-patch version-bump-minor version-bump-major version-set version-set-prerelease \
	license-inventory license-save license-audit update-licenses embed-assets verify-embeds prerequisites

# Default target
all: clean build fmt

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
	@echo "Building $(BINARY_NAME) v$(VERSION)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(BUILD_FLAGS) ./$(SRC_DIR)
	@echo "✅ Build completed: $(BUILD_DIR)/$(BINARY_NAME)"

embed-assets: ## Sync templates/ and schemas/ into embedded assets (SSOT -> internal/assets)
	@echo "Embedding assets (templates/, schemas/)..."
	@chmod +x scripts/embed-assets.sh
	@./scripts/embed-assets.sh
	@echo "✅ Assets embedded"

verify-embeds: ## Verify embedded mirrors match SSOT (fails on drift)
	@chmod +x scripts/verify-embeds.sh
	@./scripts/verify-embeds.sh

# Cross-platform build targets
build-all: ## Build for all supported platforms
	@echo "Building $(BINARY_NAME) v$(VERSION) for all platforms..."
	@./scripts/build-all.sh
	@echo "✅ Cross-platform builds completed"

sync-schemas: ## Fetch curated JSON Schema meta-schemas (network required)
	@chmod +x scripts/sync-schemas.sh
	@./scripts/sync-schemas.sh

package: ## Package built binaries into archives + checksums
	@echo "Packaging artifacts for v$(VERSION)..."
	@chmod +x scripts/package-artifacts.sh
	@./scripts/package-artifacts.sh
	@echo "✅ Packaging completed (dist/release)"

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
clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	@echo "✅ Clean completed"

# Test targets
test: ## Run all tests
	@echo "Running test suite..."
	GONEAT_OFFLINE_SCHEMA_VALIDATION=true $(GOTEST) ./... -v

test-unit: ## Run unit tests only
	@echo "Running unit tests..."
	$(GOTEST) ./cmd/... ./internal/... -v

test-integration: ## Run integration tests only
	@echo "Running integration tests..."
	$(GOTEST) ./tests/integration/... -v

test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	@mkdir -p $(HOME)/.goneat/coverage
	$(GOTEST) ./... -coverprofile=$(HOME)/.goneat/coverage/coverage.out
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
pre-commit: build test ## Run pre-commit checks using goneat (format + lint)
	@echo "Running pre-commit checks with goneat..."
	@if [ -f "$(BUILD_DIR)/$(BINARY_NAME)" ]; then \
		$(BUILD_DIR)/$(BINARY_NAME) assess --hook pre-commit; \
		echo "✅ Pre-commit checks passed"; \
	else \
		echo "❌ goneat binary not found, cannot run pre-commit checks"; \
		exit 1; \
	fi

pre-push: build-all license-audit ## Run pre-push checks using goneat (format + lint + security + license audit)
	@echo "Running pre-push checks with goneat..."
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
	$(MAKE) fmt
	@echo "✅ Development environment ready"

# Version management targets
version-get: ## Show current version
	@echo "Current version: $(VERSION)"

version-bump-patch: ## Bump patch version (x.y.Z -> x.y.Z+1)
	@echo "Bumping patch version using goneat version command..."
	@./dist/goneat version bump patch
	@echo "✅ Patch version bumped"

version-bump-minor: ## Bump minor version (x.Y.z -> x.Y+1.0)
	@echo "Bumping minor version using goneat version command..."
	@./dist/goneat version bump minor
	@echo "✅ Minor version bumped"

version-bump-major: ## Bump major version (X.y.z -> X+1.0.0)
	@echo "Bumping major version using goneat version command..."
	@./dist/goneat version bump major
	@echo "✅ Major version bumped"

version-set: ## Set specific version (usage: make version-set VERSION=x.y.z)
	@if [ -z "$(VERSION_SET)" ]; then \
		echo "❌ Usage: make version-set VERSION_SET=x.y.z"; \
		exit 1; \
	fi
	@echo "$(VERSION_SET)" > VERSION
	@echo "✅ Version set to: $(VERSION_SET)"

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
	$(MAKE) fmt
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

release: release-prep release-tag release-push ## Complete release process
	@echo "🎉 Release v$(VERSION) completed!"
	@echo ""
	@echo "📋 Next steps:"
	@echo "   1. Create GitHub release: https://github.com/fulmenhq/goneat/releases"
	@echo "   2. Upload artifacts from dist/release/ (binaries + release-notes-v$(VERSION).md)"
	@echo "   3. Paste release notes from dist/release/release-notes-v$(VERSION).md"
	@echo "   4. Announce release in relevant channels"

# Future: goneat-based version management
version-manage: build ## Use goneat for version management (future feature)
	@echo "Version management with goneat (coming soon)..."
	@echo "Current version: $(VERSION)"
	# TODO: Implement goneat version command
	# $(BUILD_DIR)/$(BINARY_NAME) version bump patch
	# $(BUILD_DIR)/$(BINARY_NAME) version set x.y.z
