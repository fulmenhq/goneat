#!/bin/bash
# Push main and one release tag to multiple git remotes for redundancy
# Supports GitHub (primary) and GitLab (backup) repositories
#
# Usage: scripts/push-to-remotes.sh <tag>
#
# Only the named tag is pushed; other local tags are never pushed. The tag is
# verified first with scripts/release-tag.sh verify (needs the GONEAT_PGP_KEY_ID,
# GONEAT_GPG_HOMEDIR and GONEAT_TAGGER_* settings). Each git push runs the
# pre-push hook. For a release ceremony use make release-tag-push, which pushes
# only the release tag and runs that same hook.

set -e

TAG="${1:?usage: scripts/push-to-remotes.sh <tag>}"

# Configuration
PRIMARY_REMOTE="origin"
BACKUP_REMOTE="gitlab"
BRANCH="main"

echo "🚀 Pushing to all remotes..."
echo "   Primary: $PRIMARY_REMOTE (GitHub)"
echo "   Backup:  $BACKUP_REMOTE (GitLab)"
echo "   Branch:  $BRANCH"
echo "   Tag:     $TAG"
echo ""

# The tag must be the signed VERSION tag at HEAD of a clean main
if [ "$TAG" != "$(tr -d '[:space:]' <VERSION)" ]; then
	echo "❌ $TAG does not match VERSION"
	exit 1
fi
GONEAT_RELEASE_TAG="$TAG" ./scripts/release-tag.sh verify

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

# Push the release tag to primary
echo "🏷️  Pushing $TAG to primary remote..."
if git push "$PRIMARY_REMOTE" "refs/tags/$TAG:refs/tags/$TAG"; then
	echo "✅ Primary tag push successful"
else
	echo "❌ Primary tag push failed"
	exit 1
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

	# Push the release tag to backup
	echo "🏷️  Pushing $TAG to backup remote..."
	if git push "$BACKUP_REMOTE" "refs/tags/$TAG:refs/tags/$TAG"; then
		echo "✅ Backup tag push successful"
	else
		echo "⚠️  Backup tag push failed (continuing...)"
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
echo "   ✅ Tag:            $TAG"
