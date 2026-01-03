#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

SRC_TEMPLATES="$ROOT_DIR/templates"
SRC_SCHEMAS="$ROOT_DIR/schemas"
SRC_CONFIG="$ROOT_DIR/config"

DST_TEMPLATES="$ROOT_DIR/internal/assets/embedded_templates/templates"
DST_SCHEMAS="$ROOT_DIR/internal/assets/embedded_schemas/schemas"
DST_CONFIG="$ROOT_DIR/internal/assets/embedded_config/config"

echo "📦 Embedding assets from SSOT (templates/, schemas/, config/)..."

mkdir -p "$DST_TEMPLATES" "$DST_SCHEMAS" "$DST_CONFIG"

sync_dir() {
	local src="$1"
	local dst="$2"
	if [ -d "$src" ]; then
		if command -v rsync >/dev/null 2>&1; then
			rsync -a --delete "$src"/ "$dst"/
		else
			rm -rf "$dst"/*
			(cd "$src" && find . -type d -print0 | xargs -0 -I{} mkdir -p "$dst/{}")
			(cd "$src" && find . -type f -print0 | xargs -0 -I{} cp -f "$src/{}" "$dst/{}")
		fi
		echo "✅ Synced $(basename "$src") -> $dst"
	else
		echo "ℹ️  Source not found: $src (skipping)"
	fi
}

sync_dir "$SRC_TEMPLATES" "$DST_TEMPLATES"
sync_dir "$SRC_SCHEMAS" "$DST_SCHEMAS"
sync_dir "$SRC_CONFIG" "$DST_CONFIG"

# Copy only if content would change (idempotent - avoids noisy diffs on every build)
copy_if_changed() {
	local src="$1"
	local dst="$2"
	local label="${3:-$(basename "$dst")}"
	if [ ! -f "$src" ]; then
		return 1
	fi
	if [ -f "$dst" ]; then
		if diff -q "$src" "$dst" >/dev/null 2>&1; then
			echo "ℹ️  $label unchanged, skipping"
			return 0
		fi
	fi
	cp -f "$src" "$dst"
	echo "✅ Updated $label"
}

generate_release_doc_aliases() {
	local version=""
	if [ -f "$ROOT_DIR/VERSION" ]; then
		version=$(tr -d ' \n\t' <"$ROOT_DIR/VERSION")
	fi
	if [ -z "$version" ]; then
		echo "ℹ️  VERSION not found; skipping release docs aliases"
		return 0
	fi

	# Expose curated recent release notes via embedded docs.
	copy_if_changed "$ROOT_DIR/RELEASE_NOTES.md" "$ROOT_DIR/docs/release-notes.md" "docs/release-notes.md"

	# Stable slug for the current release notes.
	# Only update latest.md when a version-specific doc exists for the current VERSION.
	# This prevents regenerating latest.md on every build during development when
	# VERSION has been bumped but release notes haven't been written yet.
	mkdir -p "$ROOT_DIR/docs/releases"
	local versionDoc="$ROOT_DIR/docs/releases/${version}.md"
	local latestDoc="$ROOT_DIR/docs/releases/latest.md"
	if [ -f "$versionDoc" ]; then
		copy_if_changed "$versionDoc" "$latestDoc" "docs/releases/latest.md"
	elif [ ! -f "$latestDoc" ]; then
		# Only create latest.md from RELEASE_NOTES.md if it doesn't exist at all.
		# This handles initial bootstrap; otherwise keep existing latest.md.
		copy_if_changed "$ROOT_DIR/docs/release-notes.md" "$latestDoc" "docs/releases/latest.md (bootstrap)"
	else
		echo "ℹ️  docs/releases/latest.md preserved (no $version.md yet)"
	fi
}

generate_release_doc_aliases

echo "📦 Embedding curated docs (docs/ -> internal/assets/embedded_docs/docs via content embed)..."
# Use go run to invoke content embed without requiring prebuilt binary
# This avoids chicken-and-egg problem where build depends on embed-assets
DOCS_TARGET="$ROOT_DIR/internal/assets/embedded_docs/docs"
mkdir -p "$DOCS_TARGET"
(cd "$ROOT_DIR" && go run . content embed --manifest docs/embed-manifest.yaml --root docs --target "$DOCS_TARGET" --json >/dev/null) || {
	echo "⚠️  Content embedding failed; leaving docs mirror unchanged" >&2
}

echo "✅ Embed assets sync complete"
