# Goneat Makefile
# Dogfooding: This Makefile uses goneat itself for formatting (when available)
# SSOT Version: VERSION file is the single source of truth for version management
# Quick Start Commands:
#   make help           - Show all available commands
#   make build          - Build the binary
#   make test           - Run tests (when added)
#   make dev            - Set up development environment
#   make version-get    - Show current version
#   make version-bump-patch - Bump patch version

# Variables
BINARY_NAME := goneat
VERSION := $(shell cat VERSION 2>/dev/null || echo "0.1.0")
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
LDFLAGS := -ldflags "-X 'main.Version=$(VERSION)'"
BUILD_FLAGS := $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)

.PHONY: help build clean test fmt format-docs version-get version-bump-patch version-bump-minor version-bump-major version-set

# Default target
all: clean build fmt

# Help target
help: ## Show this help message
	@echo "Goneat - Available Make Targets"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'
	@echo ""

# Build targets
build: ## Build the binary
	@echo "Building $(BINARY_NAME) v$(VERSION)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(BUILD_FLAGS) ./$(SRC_DIR)
	@echo "✅ Build completed: $(BUILD_DIR)/$(BINARY_NAME)"

# Cross-platform build targets
build-all: ## Build for all supported platforms
	@echo "Building $(BINARY_NAME) v$(VERSION) for all platforms..."
	@./scripts/build-all.sh
	@echo "✅ Cross-platform builds completed"

package: ## Package built binaries into archives + checksums
	@echo "Packaging artifacts for v$(VERSION)..."
	@chmod +x scripts/package-artifacts.sh
	@./scripts/package-artifacts.sh
	@echo "✅ Packaging completed (dist/release)"

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
	$(GOTEST) ./... -v

test-unit: ## Run unit tests only
	@echo "Running unit tests..."
	$(GOTEST) ./cmd/... ./internal/... -v

test-integration: ## Run integration tests only
	@echo "Running integration tests..."
	$(GOTEST) ./tests/integration/... -v

test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	$(GOTEST) ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Coverage gating based on lifecycle phase
coverage-check: test-coverage ## Enforce coverage threshold based on lifecycle phase
	@phase=$$(tr '[:upper:]' '[:lower:]' < LIFECYCLE_PHASE 2>/dev/null || echo alpha); \
	case $$phase in \
	  experimental) threshold=0;; \
	  alpha) threshold=30;; \
	  beta) threshold=60;; \
	  rc) threshold=70;; \
	  ga) threshold=75;; \
	  lts) threshold=80;; \
	  *) threshold=30;; \
	esac; \
	if [ ! -f coverage.out ]; then echo "❌ coverage.out not found. Run make test-coverage first"; exit 1; fi; \
	total=$$(go tool cover -func=coverage.out | awk '/^total:/ {gsub(/%/,"",$$3); print $$3}'); \
	awk -v cov="$$total" -v thr="$$threshold" -v ph="$$phase" 'BEGIN { \
	  cov+=0; thr+=0; \
	  if (cov >= thr) { printf "✅ Coverage %.1f%% meets threshold %.1f%% (phase=%s)\n", cov, thr, ph; exit 0 } \
	  else { printf "❌ Coverage %.1f%% below threshold %.1f%% (phase=%s)\n", cov, thr, ph; exit 1 } \
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

# Hook targets (dogfooding)
pre-commit: build ## Run pre-commit checks using goneat (format + lint)
	@echo "Running pre-commit checks with goneat..."
	@if [ -f "$(BUILD_DIR)/$(BINARY_NAME)" ]; then \
		$(BUILD_DIR)/$(BINARY_NAME) assess --hook pre-commit; \
		echo "✅ Pre-commit checks passed"; \
	else \
		echo "❌ goneat binary not found, cannot run pre-commit checks"; \
		exit 1; \
	fi

pre-push: build ## Run pre-push checks using goneat (format + lint + security)
	@echo "Running pre-push checks with goneat..."
	@if [ -f "$(BUILD_DIR)/$(BINARY_NAME)" ]; then \
		$(BUILD_DIR)/$(BINARY_NAME) assess --hook pre-push; \
		echo "✅ Pre-push checks passed"; \
	else \
		echo "❌ goneat binary not found, cannot run pre-push checks"; \
		exit 1; \
	fi

# Development setup
dev: ## Set up development environment
	@echo "Setting up development environment..."
	$(MAKE) build
	$(MAKE) fmt
	@echo "✅ Development environment ready"

# Version management targets
version-get: ## Show current version
	@echo "Current version: $(VERSION)"

version-bump-patch: ## Bump patch version (x.y.Z -> x.y.Z+1)
	@echo "Bumping patch version..."
	@current=$(shell cat VERSION); \
	major=$$(echo $$current | cut -d. -f1); \
	minor=$$(echo $$current | cut -d. -f2); \
	patch=$$(echo $$current | cut -d. -f3); \
	new_patch=$$((patch + 1)); \
	new_version="$$major.$$minor.$$new_patch"; \
	echo $$new_version > VERSION; \
	echo "✅ Version bumped: $$current -> $$new_version"

version-bump-minor: ## Bump minor version (x.Y.z -> x.Y+1.0)
	@echo "Bumping minor version..."
	@current=$(shell cat VERSION); \
	major=$$(echo $$current | cut -d. -f1); \
	minor=$$(echo $$current | cut -d. -f2); \
	new_minor=$$((minor + 1)); \
	new_version="$$major.$$new_minor.0"; \
	echo $$new_version > VERSION; \
	echo "✅ Version bumped: $$current -> $$new_version"

version-bump-major: ## Bump major version (X.y.z -> X+1.0.0)
	@echo "Bumping major version..."
	@current=$(shell cat VERSION); \
	major=$$(echo $$current | cut -d. -f1); \
	new_major=$$((major + 1)); \
	new_version="$$new_major.0.0"; \
	echo $$new_version > VERSION; \
	echo "✅ Version bumped: $$current -> $$new_version"

version-set: ## Set specific version (usage: make version-set VERSION=x.y.z)
	@if [ -z "$(VERSION_SET)" ]; then \
		echo "❌ Usage: make version-set VERSION_SET=x.y.z"; \
		exit 1; \
	fi
	@echo "$(VERSION_SET)" > VERSION
	@echo "✅ Version set to: $(VERSION_SET)"

# Release management targets
release-prep: ## Prepare for release (run tests, coverage gate, build, etc.)
	@echo "🚀 Preparing for release v$(VERSION)..."
	$(MAKE) test-coverage
	$(MAKE) coverage-check
	$(MAKE) build-all
	$(MAKE) fmt
	@echo "✅ Release preparation complete"

release-tag: ## Create git tag for release
	@echo "🏷️  Creating release tag v$(VERSION)..."
	git tag -a v$(VERSION) -m "Release v$(VERSION)"
	@echo "✅ Tag created: v$(VERSION)"

release-push: ## Push release to all remotes
	@echo "📤 Pushing release to all remotes..."
	./scripts/push-to-remotes.sh
	@echo "✅ Release pushed to all remotes"

release: release-prep release-tag release-push ## Complete release process
	@echo "🎉 Release v$(VERSION) completed!"
	@echo ""
	@echo "📋 Next steps:"
	@echo "   1. Create GitHub release: https://github.com/3leaps/goneat/releases"
	@echo "   2. Upload binaries from bin/ directory"
	@echo "   3. Update CHANGELOG.md if needed"
	@echo "   4. Announce release in relevant channels"

# Future: goneat-based version management
version-manage: build ## Use goneat for version management (future feature)
	@echo "Version management with goneat (coming soon)..."
	@echo "Current version: $(VERSION)"
	# TODO: Implement goneat version command
	# $(BUILD_DIR)/$(BINARY_NAME) version bump patch
	# $(BUILD_DIR)/$(BINARY_NAME) version set x.y.z