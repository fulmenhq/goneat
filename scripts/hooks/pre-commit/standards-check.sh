#!/bin/bash
# Pre-commit hook: Standards compliance validation
# Ensures code follows repository standards and conventions

set -e

echo "📋 Checking standards compliance..."

# Define basic function first
basic_standards_check() {
	local has_issues=false

	# Check for required files
	local required_files=("README.md" "LICENSE" "go.mod")
	for file in "${required_files[@]}"; do
		if [ ! -f "$file" ]; then
			echo "❌ Required file missing: $file"
			has_issues=true
		fi
	done

	# Check for large files (>500KB)
	if command -v find &>/dev/null; then
		large_files=$(find . -type f -size +500k -not -path "./.git/*" -not -path "./bin/*" 2>/dev/null)
		if [ -n "$large_files" ]; then
			echo "⚠️  Large files detected (consider git-lfs):"
			echo "$large_files"
			# Not a blocking issue, just a warning
		fi
	fi

	# Check for TODO/FIXME comments (warning only)
	if command -v grep &>/dev/null; then
		todo_count=$(grep -r "TODO\|FIXME\|XXX" --include="*.go" --exclude-dir=.git --exclude-dir=bin . 2>/dev/null | wc -l)
		if [ "$todo_count" -gt 0 ]; then
			echo "ℹ️  Found $todo_count TODO/FIXME comments (non-blocking)"
		fi
	fi

	if [ "$has_issues" = true ]; then
		echo "💡 Address standards issues before committing"
		exit 1
	else
		echo "✅ Basic standards compliance OK"
	fi
}

# Check if goneat is available and has forge command
if command -v goneat &>/dev/null && [ -f "./goneat" ]; then
	# Check if forge command exists (when implemented)
	if ./goneat forge --help >/dev/null 2>&1; then
		# Use goneat forge (dogfooding - preferred)
		if ! ./goneat forge --check --quiet; then
			echo "❌ Standards compliance issues found"
			echo "💡 Fix: ./goneat forge --fix"
			exit 1
		fi
		echo "✅ Standards compliance OK (goneat)"
	else
		echo "⚠️  goneat forge not yet available, using basic checks"
		# Fallback to basic standards checks
		basic_standards_check
	fi
else
	echo "⚠️  goneat not available, using basic standards checks"
	# Fallback to basic standards checks
	basic_standards_check
fi
