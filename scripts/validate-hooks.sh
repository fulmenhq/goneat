#!/bin/bash
# Validation script for goneat git hooks
# Tests hook functionality and provides diagnostic information

set -e

echo "🔍 Validating goneat git hooks setup..."

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print status
print_status() {
    local status=$1
    local message=$2
    case $status in
        "success")
            echo -e "${GREEN}✅${NC} $message"
            ;;
        "warning")
            echo -e "${YELLOW}⚠️${NC} $message"
            ;;
        "error")
            echo -e "${RED}❌${NC} $message"
            ;;
        "info")
            echo -e "${BLUE}ℹ️${NC} $message"
            ;;
    esac
}

# Check if lefthook is installed
if ! command -v lefthook &> /dev/null; then
    print_status "error" "lefthook not found"
    print_status "info" "Install with: go install github.com/evilmartians/lefthook@latest"
    exit 1
else
    LEFTHOOK_VERSION=$(lefthook version 2>/dev/null || echo "unknown")
    print_status "success" "lefthook installed (version: $LEFTHOOK_VERSION)"
fi

# Check if we're in a git repository
if ! git rev-parse --git-dir > /dev/null 2>&1; then
    print_status "error" "Not in a git repository"
    exit 1
else
    print_status "success" "Git repository detected"
fi

# Check if lefthook.yml exists
if [ ! -f "lefthook.yml" ]; then
    print_status "error" "lefthook.yml not found"
    exit 1
else
    print_status "success" "lefthook.yml configuration found"
fi

# Check hook script permissions
HOOK_SCRIPTS=(
    "scripts/hooks/pre-commit/format-check.sh"
    "scripts/hooks/pre-commit/lint-check.sh"
    "scripts/hooks/pre-commit/test-check.sh"
    "scripts/hooks/pre-commit/standards-check.sh"
    "scripts/hooks/pre-push/security-check.sh"
    "scripts/hooks/pre-push/integration-check.sh"
)

HOOK_SCRIPTS_MISSING=()
HOOK_SCRIPTS_NOT_EXECUTABLE=()

for script in "${HOOK_SCRIPTS[@]}"; do
    if [ ! -f "$script" ]; then
        HOOK_SCRIPTS_MISSING+=("$script")
    elif [ ! -x "$script" ]; then
        HOOK_SCRIPTS_NOT_EXECUTABLE+=("$script")
    fi
done

if [ ${#HOOK_SCRIPTS_MISSING[@]} -gt 0 ]; then
    print_status "error" "Missing hook scripts:"
    for script in "${HOOK_SCRIPTS_MISSING[@]}"; do
        echo "   - $script"
    done
    exit 1
else
    print_status "success" "All hook scripts present"
fi

if [ ${#HOOK_SCRIPTS_NOT_EXECUTABLE[@]} -gt 0 ]; then
    print_status "warning" "Hook scripts not executable:"
    for script in "${HOOK_SCRIPTS_NOT_EXECUTABLE[@]}"; do
        echo "   - $script"
    done
    print_status "info" "Fix with: chmod +x scripts/hooks/**/*.sh"
else
    print_status "success" "All hook scripts executable"
fi

# Check if hooks are installed
HOOKS_INSTALLED=true
if [ ! -L ".git/hooks/pre-commit" ] || [ ! -L ".git/hooks/pre-push" ]; then
    HOOKS_INSTALLED=false
fi

if [ "$HOOKS_INSTALLED" = false ]; then
    print_status "warning" "Git hooks not installed"
    print_status "info" "Install with: lefthook install"
else
    print_status "success" "Git hooks installed"
fi

# Test individual hooks (optional)
if [ "${TEST_HOOKS:-false}" = "true" ]; then
    echo ""
    echo "🧪 Testing individual hooks..."

    # Test pre-commit hooks
    echo "Testing pre-commit hooks..."
    if lefthook run pre-commit >/dev/null 2>&1; then
        print_status "success" "Pre-commit hooks working"
    else
        print_status "error" "Pre-commit hooks failed"
    fi

    # Test pre-push hooks
    echo "Testing pre-push hooks..."
    if lefthook run pre-push >/dev/null 2>&1; then
        print_status "success" "Pre-push hooks working"
    else
        print_status "warning" "Pre-push hooks failed (may be expected)"
    fi
fi

# Check goneat availability
if command -v goneat &> /dev/null && [ -f "./goneat" ]; then
    print_status "success" "goneat binary available (dogfooding enabled)"
else
    print_status "warning" "goneat binary not available (using fallback tools)"
fi

# Check for required tools
REQUIRED_TOOLS=("go")
MISSING_TOOLS=()

for tool in "${REQUIRED_TOOLS[@]}"; do
    if ! command -v "$tool" &> /dev/null; then
        MISSING_TOOLS+=("$tool")
    fi
done

if [ ${#MISSING_TOOLS[@]} -gt 0 ]; then
    print_status "warning" "Missing required tools:"
    for tool in "${MISSING_TOOLS[@]}"; do
        echo "   - $tool"
    done
else
    print_status "success" "All required tools available"
fi

# Summary
echo ""
echo "📊 Validation Summary:"
echo "   • lefthook: $(if command -v lefthook &> /dev/null; then echo "✅ Installed"; else echo "❌ Missing"; fi)"
echo "   • Git repository: $(if git rev-parse --git-dir >/dev/null 2>&1; then echo "✅ Yes"; else echo "❌ No"; fi)"
echo "   • lefthook.yml: $(if [ -f "lefthook.yml" ]; then echo "✅ Found"; else echo "❌ Missing"; fi)"
echo "   • Hook scripts: $(if [ ${#HOOK_SCRIPTS_MISSING[@]} -eq 0 ]; then echo "✅ All present"; else echo "❌ Missing some"; fi)"
echo "   • Executable: $(if [ ${#HOOK_SCRIPTS_NOT_EXECUTABLE[@]} -eq 0 ]; then echo "✅ All executable"; else echo "⚠️ Some not executable"; fi)"
echo "   • Hooks installed: $(if [ "$HOOKS_INSTALLED" = true ]; then echo "✅ Yes"; else echo "⚠️ No"; fi)"
echo "   • goneat available: $(if command -v goneat &>/dev/null && [ -f "./goneat" ]; then echo "✅ Yes (dogfooding)"; else echo "⚠️ No (fallback mode)"; fi)"

echo ""
if [ "$HOOKS_INSTALLED" = true ] && [ ${#HOOK_SCRIPTS_MISSING[@]} -eq 0 ] && [ ${#HOOK_SCRIPTS_NOT_EXECUTABLE[@]} -eq 0 ]; then
    print_status "success" "Git hooks validation PASSED"
    echo ""
    echo "🎉 Your goneat git hooks are ready!"
    echo ""
    echo "💡 Next steps:"
    echo "   1. Make a test commit: git commit -m 'test: validate hooks'"
    echo "   2. Push to test pre-push hooks: git push"
    echo "   3. Check hook performance: lefthook --verbose run pre-commit"
else
    print_status "warning" "Git hooks validation has issues"
    echo ""
    echo "🔧 To fix:"
    if [ "$HOOKS_INSTALLED" = false ]; then
        echo "   • Install hooks: lefthook install"
    fi
    if [ ${#HOOK_SCRIPTS_NOT_EXECUTABLE[@]} -gt 0 ]; then
        echo "   • Make scripts executable: chmod +x scripts/hooks/**/*.sh"
    fi
    echo "   • Re-run validation: ./scripts/validate-hooks.sh"
fi

echo ""
echo "📖 For more information, see:"
echo "   .plans/active/v0.1.2/git-hooks-development-plan.md"
