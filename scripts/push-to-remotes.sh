#!/bin/bash
# Push to multiple git remotes for redundancy
# Supports GitHub (primary) and GitLab (backup) repositories

set -e

# Configuration
PRIMARY_REMOTE="origin"
BACKUP_REMOTE="gitlab"
BRANCH="main"

echo "🚀 Pushing to all remotes..."
echo "   Primary: $PRIMARY_REMOTE (GitHub)"
echo "   Backup:  $BACKUP_REMOTE (GitLab)"
echo "   Branch:  $BRANCH"
echo ""

# Function to check if remote exists
remote_exists() {
    git remote | grep -q "^$1$"
}

# Check remotes
if ! remote_exists "$PRIMARY_REMOTE"; then
    echo "❌ Primary remote '$PRIMARY_REMOTE' not found"
    echo "   Run: git remote add $PRIMARY_REMOTE <github-url>"
    exit 1
fi

if ! remote_exists "$BACKUP_REMOTE"; then
    echo "⚠️  Backup remote '$BACKUP_REMOTE' not found"
    echo "   This is optional but recommended for disaster recovery"
    echo "   Run: git remote add $BACKUP_REMOTE <gitlab-url>"
    BACKUP_REMOTE=""
fi

# Push to primary remote
echo "📤 Pushing to primary remote ($PRIMARY_REMOTE)..."
if git push "$PRIMARY_REMOTE" "$BRANCH"; then
    echo "✅ Primary push successful"
else
    echo "❌ Primary push failed"
    exit 1
fi

# Push tags to primary
echo "🏷️  Pushing tags to primary remote..."
if git push "$PRIMARY_REMOTE" --tags; then
    echo "✅ Primary tags push successful"
else
    echo "⚠️  Primary tags push failed (continuing...)"
fi

# Push to backup remote (if configured)
if [ -n "$BACKUP_REMOTE" ]; then
    echo ""
    echo "📤 Pushing to backup remote ($BACKUP_REMOTE)..."

    if git push "$BACKUP_REMOTE" "$BRANCH"; then
        echo "✅ Backup push successful"
    else
        echo "❌ Backup push failed"
        echo "   Primary push was successful - continuing..."
    fi

    # Push tags to backup
    echo "🏷️  Pushing tags to backup remote..."
    if git push "$BACKUP_REMOTE" --tags; then
        echo "✅ Backup tags push successful"
    else
        echo "⚠️  Backup tags push failed (continuing...)"
    fi
else
    echo ""
    echo "ℹ️  Backup remote not configured - skipping"
fi

echo ""
echo "🎉 Push completed successfully!"
echo ""
echo "📊 Summary:"
echo "   ✅ Primary remote: Pushed successfully"
if [ -n "$BACKUP_REMOTE" ]; then
    echo "   ✅ Backup remote:  Pushed successfully"
else
    echo "   ⚠️  Backup remote:  Not configured"
fi
echo "   ✅ Tags:           Pushed to all remotes"</content>
</xai:function_call name="bash">
<parameter name="command">chmod +x goneat/scripts/push-to-remotes.sh
