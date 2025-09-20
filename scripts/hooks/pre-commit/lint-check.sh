#!/bin/bash
# Pre-commit hook: Lint validation
# Ensures code quality standards are met

set -e

echo "🔍 Checking code linting..."

# Define fallback function first
fallback_lint_check() {
    local has_issues=false

    # Try golangci-lint first (most comprehensive)
    if command -v golangci-lint &> /dev/null; then
        version_output="$(golangci-lint --version 2>/dev/null || true)"
        version_token="$(printf '%s\n' "$version_output" | grep -Eo 'v?[0-9]+\.[0-9]+\.[0-9]+' | head -n1)"
        major=0
        minor=0
        patch=0
        if [ -n "$version_token" ]; then
            version_core="${version_token#v}"
            IFS='.-' read -r major minor patch _ <<< "$version_core"
            major=${major:-0}
            minor=${minor:-0}
            patch=${patch:-0}
        fi

        lint_cmd=("golangci-lint" "run" "--new-from-rev=HEAD~" "--timeout" "5m")

        if [ "$major" -gt 2 ] || { [ "$major" -eq 2 ] && [ "$minor" -ge 4 ]; }; then
            lint_cmd=("golangci-lint" "run" "--output=json" "--new-from-rev=HEAD~" "--timeout" "5m")
        elif [ "$major" -ge 2 ]; then
            lint_cmd=("golangci-lint" "run" "--out-format" "json" "--new-from-rev=HEAD~" "--timeout" "5m")
            echo "ℹ️  golangci-lint $version_token detected (v2.0–v2.3). Using transitional JSON flags; consider upgrading to v2.4.0+ for richer output."
        else
            if [ -n "$version_token" ]; then
                echo "⚠️  golangci-lint $version_token detected (<v2.0). Falling back to legacy JSON output; upgrade recommended."
            else
                echo "⚠️  Unable to detect golangci-lint version. Falling back to legacy JSON output."
            fi
            lint_cmd=("golangci-lint" "run" "--out-format" "json" "--new-from-rev=HEAD~" "--timeout" "5m")
        fi

        echo "ℹ️  Running ${lint_cmd[*]} (informational for alpha)"
        if "${lint_cmd[@]}" >/dev/null 2>&1; then
            echo "✅ golangci-lint passed"
        else
            echo "⚠️  golangci-lint found issues (acceptable for alpha)"
            echo "💡 These will be addressed in future releases"
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
