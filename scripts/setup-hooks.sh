#!/bin/bash
# Setup script for goneat git hooks
# Installs and configures Lefthook for automated quality assurance

set -e

echo "🔧 Setting up goneat git hooks..."

# Check if lefthook is installed
if ! command -v lefthook &>/dev/null; then
	echo "❌ lefthook not found"
	echo ""
	echo "📦 Install lefthook:"
	echo "   Go: go install github.com/evilmartians/lefthook@latest"
	echo "   Homebrew: brew install lefthook"
	echo "   Or download from: https://github.com/evilmartians/lefthook/releases"
	echo ""
	exit 1
fi

# Check if we're in a git repository
if ! git rev-parse --git-dir >/dev/null 2>&1; then
	echo "❌ Not in a git repository"
	echo "   Run this script from the root of your goneat git repository"
	exit 1
fi

# Make hook scripts executable
echo "📝 Making hook scripts executable..."
chmod +x scripts/hooks/pre-commit/*.sh
chmod +x scripts/hooks/pre-push/*.sh
chmod +x scripts/hooks/post-commit/*.sh 2>/dev/null || true

# Install lefthook hooks
echo "🔗 Installing git hooks..."
lefthook install

# Verify installation
echo "✅ Verifying hook installation..."
if [ -L ".git/hooks/pre-commit" ] && [ -L ".git/hooks/pre-push" ]; then
	echo "✅ Git hooks installed successfully"
else
	echo "⚠️  Hook installation may have issues"
	echo "   Check: ls -la .git/hooks/"
fi

# Test hooks (optional)
read -p "🧪 Test hooks now? (y/N): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
	echo "🧪 Testing pre-commit hooks..."
	lefthook run pre-commit

	echo "🧪 Testing pre-push hooks..."
	lefthook run pre-push
fi

echo ""
echo "🎉 Git hooks setup complete!"
echo ""
echo "📋 What's configured:"
echo "   • Pre-commit: format, lint, test, standards checks"
echo "   • Pre-push: security, integration validation"
echo "   • Dogfooding: Uses goneat commands when available"
echo ""
echo "💡 Next steps:"
echo "   1. Make your first commit to test the hooks"
echo "   2. Use 'lefthook run <hook-name>' to test individual hooks"
echo "   3. Check .plans/active/v0.1.2/ for hook documentation"
echo ""
echo "🔍 Useful commands:"
echo "   lefthook run pre-commit    # Test all pre-commit hooks"
echo "   lefthook run pre-push      # Test all pre-push hooks"
echo "   lefthook --help           # See all lefthook options"
