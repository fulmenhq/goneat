#!/bin/bash
# Pre-commit hook: Lint validation
# Ensures code quality standards are met

set -e

echo "🔍 Checking code linting..."

# Check if goneat is available and has lint command
if command -v goneat &> /dev/null && [ -f "./goneat" ]; then
    # Check if lint command exists (when Code Scout completes it)
    if ./goneat lint --help >/dev/null 2>&1; then
        # Use goneat lint (dogfooding - preferred)
        if ! ./goneat lint --check --quiet; then
            echo "❌ Linting issues found"
            echo "💡 Fix: ./goneat lint --fix"
            exit 1
        fi
        echo "✅ Code linting OK (goneat)"
    else
        echo "⚠️  goneat lint not yet available, using fallback tools"
        # Fallback to available tools
        fallback_lint_check
    fi
else
    echo "⚠️  goneat not available, using fallback tools"
    # Fallback to available tools
    fallback_lint_check
fi

fallback_lint_check() {
    local has_issues=false

    # Try golangci-lint first (most comprehensive)
    if command -v golangci-lint &> /dev/null; then
        if ! golangci-lint run --quiet --timeout 5m; then
            echo "❌ golangci-lint issues found"
            has_issues=true
        fi
    # Fallback to go vet
    elif command -v go &> /dev/null; then
        if ! go vet ./... >/dev/null 2>&1; then
            echo "❌ go vet issues found"
            has_issues=true
        fi
    else
        echo "⚠️  No linting tools available, skipping lint check"
        return 0
    fi

    if [ "$has_issues" = true ]; then
        echo "💡 Fix linting issues before committing"
        exit 1
    else
        echo "✅ Code linting OK (fallback tools)"
    fi
}