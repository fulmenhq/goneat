#!/usr/bin/env bash
set -euo pipefail

# upload-release-assets.sh - Upload signed release artifacts to GitHub
#
# Intended use:
# - Called by `make release-upload`
# - Can also be run manually
#
# Security notes:
# - Verifies checksum manifest signatures before uploading
# - Uses GONEAT_GPG_HOMEDIR (preferred) or GPG_HOMEDIR if provided (does not require using the user's default keyring)
#
# Usage:
#   scripts/upload-release-assets.sh <version> [release_dir]
#
# Examples:
#   scripts/upload-release-assets.sh v0.3.15
#   GPG_HOMEDIR=/tmp/gpg scripts/upload-release-assets.sh v0.3.15 dist/release

VERSION=${1:?'usage: scripts/upload-release-assets.sh <version> [release_dir]'}
RELEASE_DIR=${2:-dist/release}

if ! command -v gh >/dev/null 2>&1; then
	echo "❌ gh CLI not found. Install GitHub CLI: https://cli.github.com/" >&2
	exit 1
fi

if [ ! -d "$RELEASE_DIR" ]; then
	echo "❌ Release directory not found: $RELEASE_DIR" >&2
	exit 1
fi

cd "$RELEASE_DIR"

echo "📤 Uploading release artifacts for $VERSION..."

echo "   Checking required files..."
REQUIRED_FILES=(
	"SHA256SUMS"
	"SHA512SUMS"
	"SHA256SUMS.asc"
	"SHA512SUMS.asc"
	"SHA256SUMS.minisig"
	"SHA512SUMS.minisig"
	"fulmenhq-release-signing-key.asc"
	"fulmenhq-release-minisign.pub"
	"release-notes.md"
	"release-notes-${VERSION}.md"
)

for file in "${REQUIRED_FILES[@]}"; do
	if [ ! -f "$file" ]; then
		echo "❌ Required file not found: $RELEASE_DIR/$file" >&2
		exit 1
	fi
done

GPG_HOMEDIR_EFF="${GONEAT_GPG_HOMEDIR:-${GPG_HOMEDIR:-}}"

# shellcheck disable=SC2034
GPG_HOMEDIR_FLAG=()
if [ -n "${GPG_HOMEDIR_EFF}" ]; then
	GPG_HOMEDIR_FLAG=(--homedir "$GPG_HOMEDIR_EFF")
fi

echo "   🔏 Verifying GPG checksum signatures..."
for sums in SHA256SUMS SHA512SUMS; do
	if ! gpg "${GPG_HOMEDIR_FLAG[@]:-}" --verify "${sums}.asc" "$sums" >/dev/null 2>&1; then
		echo "❌ Error: Invalid GPG signature for $sums" >&2
		echo "   Make sure GONEAT_GPG_HOMEDIR (preferred) or GPG_HOMEDIR matches what was used during signing" >&2
		exit 1
	fi
done

if command -v minisign >/dev/null 2>&1; then
	echo "   🔐 Verifying minisign checksum signatures..."
	for sums in SHA256SUMS SHA512SUMS; do
		if ! minisign -Vm "$sums" -p fulmenhq-release-minisign.pub >/dev/null 2>&1; then
			echo "❌ Error: Invalid minisign signature for $sums" >&2
			exit 1
		fi
	done
else
	echo "   ⚠️  minisign not available; skipping local minisign verification"
fi

echo "   ✅ Signatures verified"

echo "   Uploading binaries and checksums..."
# shellcheck disable=SC2086
# Version is expected to be a single token (e.g. v0.3.15)
gh release upload "$VERSION" \
	goneat_${VERSION}_*.tar.gz \
	goneat_${VERSION}_*.zip \
	SHA256SUMS \
	SHA512SUMS \
	--clobber

echo "   Uploading signatures, keys, and release notes asset..."
gh release upload "$VERSION" \
	SHA256SUMS.asc \
	SHA512SUMS.asc \
	SHA256SUMS.minisig \
	SHA512SUMS.minisig \
	fulmenhq-release-signing-key.asc \
	fulmenhq-release-minisign.pub \
	"release-notes-${VERSION}.md" \
	--clobber

echo "   Setting release body from notes file..."
gh release edit "$VERSION" --notes-file "release-notes.md"

echo "✅ Release artifacts uploaded for $VERSION"
