#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

SSOT_TEMPLATES="$ROOT_DIR/templates"
SSOT_SCHEMAS="$ROOT_DIR/schemas"
EMBED_TEMPLATES="$ROOT_DIR/internal/assets/embedded_templates/templates"
EMBED_SCHEMAS="$ROOT_DIR/internal/assets/embedded_schemas/schemas"

echo "🔎 Verifying embedded mirrors are in sync with SSOT..."

fail=0
check_dir() {
  local src="$1" dst="$2" name="$3"
  if [ ! -d "$src" ]; then
    echo "❌ Missing SSOT directory: $src" >&2
    fail=1; return
  fi
  if [ ! -d "$dst" ]; then
    echo "❌ Missing embedded mirror: $dst" >&2
    fail=1; return
  fi
  # Use rsync dry-run to detect differences (files added/updated/deleted)
  if ! rsync -anic --delete "$src"/ "$dst"/ | grep -qE '^[*<>]f|^deleting '; then
    echo "✅ $name: in sync"
  else
    echo "❌ $name: drift detected between SSOT and embedded mirror" >&2
    rsync -anic --delete "$src"/ "$dst"/ >&2 || true
    fail=1
  fi
}

check_dir "$SSOT_TEMPLATES" "$EMBED_TEMPLATES" templates
check_dir "$SSOT_SCHEMAS"   "$EMBED_SCHEMAS"   schemas

# Verify curated docs mirror using go run (avoids chicken-and-egg dependency)
SSOT_DOCS="$ROOT_DIR/docs"
EMBED_DOCS="$ROOT_DIR/internal/assets/embedded_docs/docs"
if [ -d "$SSOT_DOCS" ]; then
  echo "🔎 Verifying curated docs mirror via content verify..."
  if ! (cd "$ROOT_DIR" && go run . content verify --manifest "$SSOT_DOCS/embed-manifest.yaml" --root "$SSOT_DOCS" --target "$EMBED_DOCS" --json >/dev/null); then
    fail=1
  fi
fi

if [ "$fail" -ne 0 ]; then
  echo "\nHint: run 'make embed-assets' to re-sync mirrors from SSOT, then commit changes." >&2
  exit 1
fi

echo "✅ All embedded mirrors are up to date"
