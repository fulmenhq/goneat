#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEST_DIR="$ROOT_DIR/internal/assets/jsonschema"

mkdir -p "$DEST_DIR/draft-07" "$DEST_DIR/draft-2020-12"

echo "Fetching JSON Schema meta-schemas..."
curl -fsSL https://json-schema.org/draft-07/schema -o "$DEST_DIR/draft-07/schema.json"
curl -fsSL https://json-schema.org/draft/2020-12/schema -o "$DEST_DIR/draft-2020-12/schema.json"

echo "✅ Synced meta-schemas to $DEST_DIR"
