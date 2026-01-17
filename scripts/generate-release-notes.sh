#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="$ROOT_DIR/dist/release"
RELEASE_TAG=${GONEAT_RELEASE_TAG:-}

if [[ -z "$RELEASE_TAG" ]]; then
	echo "GONEAT_RELEASE_TAG is required (e.g. v0.4.5 or 0.4.5)" >&2
	exit 1
fi

# Normalize tag to include leading v
if [[ "$RELEASE_TAG" != v* ]]; then
	RELEASE_TAG="v${RELEASE_TAG}"
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
