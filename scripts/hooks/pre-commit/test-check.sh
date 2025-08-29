#!/bin/bash
# Pre-commit hook: Test validation
# Runs quick unit tests to ensure code changes don't break existing functionality

set -e

echo "🧪 Running unit tests..."

# Set timeout for pre-commit (should be fast)
TIMEOUT="60s"

# Run unit tests only (skip integration tests for speed)
if command -v go &> /dev/null; then
    # Run tests with short flag to skip long-running tests
    if ! go test ./cmd/... ./internal/... -short -timeout "$TIMEOUT" -v; then
        echo "❌ Unit tests failed"
        echo "💡 Fix: go test ./cmd/... ./internal/..."
        echo "💡 For faster iteration: go test ./cmd/... ./internal/... -short"
        exit 1
    fi
    echo "✅ Unit tests passed"
else
    echo "⚠️  Go not found, skipping test check"
    exit 0
fi

# Optional: Check test coverage if desired
# Uncomment the following lines if you want coverage checks in pre-commit
# echo "📊 Checking test coverage..."
# if ! go test ./cmd/... ./internal/... -short -cover -covermode=count; then
#     echo "⚠️  Test coverage check failed (non-blocking)"
# fi