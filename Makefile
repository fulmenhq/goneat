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
GOTEST := $(GOTEST) test
GOMOD := $(GOCMD) mod
GOFMT := $(GOCMD) fmt

# Build flags
LDFLAGS := -ldflags "-X 'main.Version=$(VERSION)'"
BUILD_FLAGS := $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)

.PHONY: help build clean test fmt version-get version-bump-patch version-bump-minor version-bump-major version-set

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

# Clean targets
clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	@echo "✅ Clean completed"

# Test targets (placeholder for now)
test: ## Run tests (placeholder)
	@echo "No tests yet - coming soon!"

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

# Future: goneat-based version management
version-manage: build ## Use goneat for version management (future feature)
	@echo "Version management with goneat (coming soon)..."
	@echo "Current version: $(VERSION)"
	# TODO: Implement goneat version command
	# $(BUILD_DIR)/$(BINARY_NAME) version bump patch
	# $(BUILD_DIR)/$(BINARY_NAME) version set x.y.z