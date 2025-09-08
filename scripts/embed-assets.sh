#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

SRC_TEMPLATES="$ROOT_DIR/templates"
SRC_SCHEMAS="$ROOT_DIR/schemas"

DST_TEMPLATES="$ROOT_DIR/internal/assets/embedded_templates/templates"
DST_SCHEMAS="$ROOT_DIR/internal/assets/embedded_schemas/schemas"

echo "📦 Embedding assets from SSOT (templates/, schemas/)..."

mkdir -p "$DST_TEMPLATES" "$DST_SCHEMAS"

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

echo "📦 Embedding curated docs (docs/ -> internal/assets/embedded_docs/docs via CLI if available)..."
# Prefer CLI-driven embedding when built binary is present
CLI_BIN="$ROOT_DIR/dist/goneat"
DOCS_TARGET="$ROOT_DIR/internal/assets/embedded_docs/docs"
if [ -x "$CLI_BIN" ]; then
  mkdir -p "$DOCS_TARGET"
  "$CLI_BIN" content embed --manifest "$ROOT_DIR/docs/embed-manifest.yaml" --root "$ROOT_DIR/docs" --target "$DOCS_TARGET" --json >/dev/null || {
    echo "⚠️  CLI-driven docs embedding failed; leaving docs mirror unchanged" >&2
  }
else
  echo "ℹ️  dist/goneat not found; skipping CLI-driven docs embedding (mirrors must be tracked)"
fi

echo "✅ Embed assets sync complete"
