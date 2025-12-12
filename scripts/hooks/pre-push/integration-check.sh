#!/bin/bash
# Pre-push hook: Integration validation
# Runs comprehensive tests before pushing to remote repositories

set -e

echo "🔗 Running integration tests..."

# Set longer timeout for pre-push (can be more comprehensive)
TIMEOUT="5m"

# Run integration tests (slower, more comprehensive)
if command -v go &>/dev/null; then
	# Run all tests including integration
	if ! go test ./... -timeout "$TIMEOUT" -v; then
		echo "❌ Integration tests failed"
		echo "💡 Fix: go test ./..."
		echo "💡 For faster iteration: go test ./cmd/... ./internal/... -short"
		exit 1
	fi
	echo "✅ Integration tests passed"
else
	echo "⚠️  Go not found, skipping integration tests"
	exit 0
fi

# Run goneat assessment if available (comprehensive check)
if command -v goneat &>/dev/null && [ -f "./goneat" ]; then
	if ./goneat assess --help >/dev/null 2>&1; then
		if ! ./goneat assess --fail-on critical >/dev/null 2>&1; then
			echo "❌ Assessment failed (critical issues only)"
			echo "💡 Fix critical severity issues before pushing"
			echo "💡 Note: High/medium/low severity issues are acceptable for alpha"
			exit 1
		fi
		echo "✅ Assessment passed (goneat)"
	fi
fi

# Optional: Check test coverage
if [ "${CHECK_COVERAGE:-false}" = "true" ] && command -v go &>/dev/null; then
	echo "📊 Checking test coverage..."
	go test ./... -coverprofile=coverage.out >/dev/null 2>&1
	coverage=$(go tool cover -func=coverage.out | grep "total:" | awk '{print $3}' | sed 's/%//')

	# Check if coverage meets minimum threshold (adjust as needed)
	if (($(echo "$coverage < 70" | bc -l))); then
		echo "⚠️  Test coverage below threshold: ${coverage}% (minimum: 70%)"
		echo "💡 Consider adding more tests before pushing"
		# Warning only, not blocking
	else
		echo "✅ Test coverage: ${coverage}%"
	fi

	# Clean up coverage file
	rm -f coverage.out
fi

echo "✅ Integration validation completed"
