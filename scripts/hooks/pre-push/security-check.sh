#!/bin/bash
# Pre-push hook: Security validation
# Performs security checks before pushing to remote repositories

set -e

echo "🔒 Performing security checks..."

# Check for sensitive files that shouldn't be committed
check_sensitive_files() {
    local sensitive_patterns=(
        "*.key"
        "*.pem"
        "*.p12"
        "*.pfx"
        ".env*"
        "*secret*"
        "*password*"
        "*token*"
        "id_rsa*"
        "id_dsa*"
        "id_ecdsa*"
        "id_ed25519*"
    )

    local found_sensitive=false

    for pattern in "${sensitive_patterns[@]}"; do
        # Check staged files for sensitive patterns
        if git diff --cached --name-only | grep -q "$pattern" 2>/dev/null; then
            echo "❌ Potential sensitive file detected: $pattern"
            found_sensitive=true
        fi
    done

    if [ "$found_sensitive" = true ]; then
        echo "💡 Remove sensitive files from staging before pushing"
        echo "💡 Use: git rm --cached <file>"
        exit 1
    fi
}

# Check for hardcoded secrets in code
check_hardcoded_secrets() {
    local secret_patterns=(
        "password.*=.*[\"'][^\"']*[\"']"
        "secret.*=.*[\"'][^\"']*[\"']"
        "token.*=.*[\"'][^\"']*[\"']"
        "key.*=.*[\"'][^\"']*[\"']"
        "api.*key.*=.*[\"'][^\"']*[\"']"
    )

    local found_secrets=false

    for pattern in "${secret_patterns[@]}"; do
        # Check staged Go files for potential hardcoded secrets
        if git diff --cached -- "*.go" | grep -q "$pattern" 2>/dev/null; then
            echo "⚠️  Potential hardcoded secret detected: $pattern"
            found_secrets=true
        fi
    done

    if [ "$found_secrets" = true ]; then
        echo "💡 Review code for hardcoded secrets before pushing"
        echo "💡 Consider using environment variables or secure credential storage"
        # Warning only, not blocking
    fi
}

# Run security checks
check_sensitive_files
check_hardcoded_secrets

# Check if goneat has security features (future)
if command -v goneat &> /dev/null && [ -f "./goneat" ]; then
    if ./goneat security --help >/dev/null 2>&1; then
        if ! ./goneat security --scan --quiet; then
            echo "❌ Security scan failed"
            echo "💡 Fix security issues before pushing"
            exit 1
        fi
        echo "✅ Security scan passed (goneat)"
    fi
fi

echo "✅ Security checks completed"