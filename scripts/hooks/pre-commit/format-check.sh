#!/bin/bash
# Pre-commit hook: Format validation
# Ensures code is properly formatted before commit

set -e

echo "🔍 Checking code formatting..."

# Check if goneat is available and built
if command -v goneat &> /dev/null && [ -f "./goneat" ]; then
    # Use goneat format (dogfooding - preferred)
    if ! ./goneat format --check --quiet; then
        echo "❌ Code formatting issues found"
        echo "💡 Fix: ./goneat format"
        echo "💡 Auto-fix: ./goneat format --fix"
        exit 1
    fi
    echo "✅ Code formatting OK (goneat)"
elif command -v go &> /dev/null; then
    # Fallback to go fmt
    if ! go fmt ./... >/dev/null 2>&1; then
        echo "❌ Code formatting issues found"
        echo "💡 Fix: go fmt ./..."
        exit 1
    fi
    echo "✅ Code formatting OK (go fmt)"
else
    echo "⚠️  Neither goneat nor go found, skipping format check"
fi
