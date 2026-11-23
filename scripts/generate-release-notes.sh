#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="$ROOT_DIR/dist/release"
RELEASE_TAG=${GONEAT_RELEASE_TAG:-}

# The tag is an explicit input and is used exactly as given; it is never
# derived or normalized.
if ! [[ "$RELEASE_TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-(rc|beta|alpha)\.[0-9]+)?$ ]]; then
	echo "GONEAT_RELEASE_TAG=vX.Y.Z is required" >&2
	exit 1
fi

NOTES_FILE="$ROOT_DIR/docs/releases/${RELEASE_TAG}.md"
if [[ ! -f "$NOTES_FILE" ]]; then
	echo "Release notes file not found: $NOTES_FILE" >&2
	echo "Expected docs/releases/${RELEASE_TAG}.md" >&2
	exit 1
fi

mkdir -p "$OUT_DIR"

# Keep both a stable name and a versioned name for convenience
cp "$NOTES_FILE" "$OUT_DIR/release-notes.md"
cp "$NOTES_FILE" "$OUT_DIR/release-notes-${RELEASE_TAG}.md"

echo "Release notes written to: $OUT_DIR/release-notes.md"
echo "Release notes written to: $OUT_DIR/release-notes-${RELEASE_TAG}.md"
