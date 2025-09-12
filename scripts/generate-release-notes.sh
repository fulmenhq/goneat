#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION_FILE="$ROOT_DIR/VERSION"
NOTES_FILE="$ROOT_DIR/RELEASE_NOTES.md"
OUT_DIR="$ROOT_DIR/dist/release"

if [[ ! -f "$VERSION_FILE" ]]; then
  echo "VERSION file not found" >&2
  exit 1
fi
VERSION=$(cat "$VERSION_FILE")

if [[ ! -f "$NOTES_FILE" ]]; then
  echo "RELEASE_NOTES.md not found" >&2
  exit 1
fi

mkdir -p "$OUT_DIR"
# Strip 'v' prefix if present for filename
VERSION_CLEAN=${VERSION#v}
OUT_FILE="$OUT_DIR/release-notes-v${VERSION_CLEAN}.md"

# Simple sanity: ensure notes mention the version
if ! grep -qi "${VERSION}" "$NOTES_FILE"; then
  echo "Warning: RELEASE_NOTES.md does not mention version ${VERSION}" >&2
fi

cp "$NOTES_FILE" "$OUT_FILE"
echo "Release notes written to: $OUT_FILE"
