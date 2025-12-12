#!/usr/bin/env bash
set -euo pipefail

usage() {
	cat <<'EOF' >&2
Usage: scripts/verify-release-assets.sh vX.Y.Z

Downloads the published artifacts for the given release tag, recomputes
SHA256 sums, and compares them against the locally generated dist/release/SHA256SUMS.
Requires: gh CLI, shasum, network access.
EOF
}

if [[ $# -ne 1 ]]; then
	usage
	exit 1
fi

VERSION="$1"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_RELEASE="$REPO_ROOT/dist/release"
LOCAL_SHA="$DIST_RELEASE/SHA256SUMS"

if [[ ! -f "$LOCAL_SHA" ]]; then
	echo "Local SHA256SUMS not found at $LOCAL_SHA" >&2
	exit 2
fi

for cmd in gh shasum; do
	if ! command -v "$cmd" >/dev/null 2>&1; then
		echo "Missing required command: $cmd" >&2
		exit 3
	fi
done

TMP_DIR="$(mktemp -d)"
cleanup() {
	rm -rf "$TMP_DIR"
}
trap cleanup EXIT

# Download the published archives served by GitHub for this release tag.
download_pattern() {
	local pattern="$1"
	gh release download "$VERSION" \
		--dir "$TMP_DIR" \
		--pattern "$pattern" \
		--clobber >/dev/null
}

download_pattern "goneat_${VERSION}_*.tar.gz"
download_pattern "goneat_${VERSION}_*.zip"

declare -a remote_files
while IFS= read -r -d '' file; do
	remote_files+=("$file")
done < <(cd "$TMP_DIR" && find . -maxdepth 1 -type f \( -name "goneat_${VERSION}_*.tar.gz" -o -name "goneat_${VERSION}_*.zip" \) -print0)

if [[ ${#remote_files[@]} -eq 0 ]]; then
	echo "No remote artifacts were downloaded for $VERSION" >&2
	exit 4
fi

compute_hash_line() {
	local algo=$1
	local file=$2
	local hash
	case "$algo" in
	256)
		if command -v shasum >/dev/null 2>&1; then
			hash=$(shasum -a 256 "$file" | awk '{print $1}')
		elif command -v sha256sum >/dev/null 2>&1; then
			hash=$(sha256sum "$file" | awk '{print $1}')
		elif command -v openssl >/dev/null 2>&1; then
			hash=$(openssl dgst -sha256 "$file" | sed 's/^.*= //')
		else
			echo "No sha256-capable tool available" >&2
			exit 5
		fi
		;;
	512)
		if command -v shasum >/dev/null 2>&1; then
			hash=$(shasum -a 512 "$file" | awk '{print $1}')
		elif command -v sha512sum >/dev/null 2>&1; then
			hash=$(sha512sum "$file" | awk '{print $1}')
		elif command -v openssl >/dev/null 2>&1; then
			hash=$(openssl dgst -sha512 "$file" | sed 's/^.*= //')
		else
			echo "No sha512-capable tool available" >&2
			exit 5
		fi
		;;
	*)
		echo "Unsupported hash algorithm: $algo" >&2
		exit 5
		;;
	esac
	printf "%s  %s\n" "$hash" "$(basename "$file")"
}

REMOTE_SHA256_SORTED="$TMP_DIR/SHA256SUMS.github"
REMOTE_SHA512_SORTED="$TMP_DIR/SHA512SUMS.github"
LOCAL_SHA256_SORTED="$TMP_DIR/SHA256SUMS.local"
LOCAL_SHA512_SORTED="$TMP_DIR/SHA512SUMS.local"

>"$REMOTE_SHA256_SORTED"
>"$REMOTE_SHA512_SORTED"
for file in "${remote_files[@]}"; do
	compute_hash_line 256 "$file" >>"$REMOTE_SHA256_SORTED"
	compute_hash_line 512 "$file" >>"$REMOTE_SHA512_SORTED"
done

sort "$LOCAL_SHA" >"$LOCAL_SHA256_SORTED"
LOCAL_SHA512="$DIST_RELEASE/SHA512SUMS"
if [[ ! -f "$LOCAL_SHA512" ]]; then
	echo "Local SHA512SUMS not found at $LOCAL_SHA512" >&2
	exit 6
fi
sort "$LOCAL_SHA512" >"$LOCAL_SHA512_SORTED"

diff -u "$LOCAL_SHA256_SORTED" "$REMOTE_SHA256_SORTED" >"$TMP_DIR/diff256" || {
	echo "❌ Remote artifacts do not match local SHA256SUMS" >&2
	cat "$TMP_DIR/diff256" >&2
	exit 7
}

diff -u "$LOCAL_SHA512_SORTED" "$REMOTE_SHA512_SORTED" >"$TMP_DIR/diff512" || {
	echo "❌ Remote artifacts do not match local SHA512SUMS" >&2
	cat "$TMP_DIR/diff512" >&2
	exit 8
}

for sums in SHA256SUMS SHA512SUMS; do
	gh release download "$VERSION" --dir "$TMP_DIR" --pattern "$sums" --clobber >/dev/null
	if [[ -f "$TMP_DIR/$sums" ]]; then
		sort "$TMP_DIR/$sums" >"$TMP_DIR/${sums}.remote"
		local_sorted="$TMP_DIR/${sums}.local"
		if [[ "$sums" == "SHA256SUMS" ]]; then
			cp "$LOCAL_SHA256_SORTED" "$local_sorted"
		else
			cp "$LOCAL_SHA512_SORTED" "$local_sorted"
		fi
		diff -u "$local_sorted" "$TMP_DIR/${sums}.remote" >"$TMP_DIR/${sums}.diff" || {
			echo "❌ Uploaded $sums asset differs from local copy" >&2
			cat "$TMP_DIR/${sums}.diff" >&2
			exit 9
		}
	fi
done

echo "✅ Remote release assets for $VERSION match local SHA256SUMS"
