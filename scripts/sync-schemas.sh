#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
META_DIR="$ROOT_DIR/schemas/meta"
DEST_DIR="$ROOT_DIR/internal/assets/jsonschema"

mkdir -p "$META_DIR/draft-07" "$META_DIR/draft-2020-12/meta"
mkdir -p "$DEST_DIR/draft-07" "$DEST_DIR/draft-2020-12"

echo "Fetching JSON Schema meta-schemas..."
curl -fsSL https://json-schema.org/draft-07/schema -o "$META_DIR/draft-07/schema.json"
curl -fsSL https://json-schema.org/draft/2020-12/schema -o "$META_DIR/draft-2020-12/schema.json"

# Fetch supporting vocabularies for draft 2020-12
PARTS=(core applicator unevaluated validation meta-data format-annotation content)
for part in "${PARTS[@]}"; do
	url="https://json-schema.org/draft/2020-12/meta/${part}"
	out="$META_DIR/draft-2020-12/meta/${part}.json"
	if curl -fsSL "$url" -o "$out"; then
		echo "Downloaded ${part} vocabulary"
	else
		echo "Warning: failed to download ${url}" >&2
	fi
done

# Mirror downloads into internal/assets for direct embeds
cp "$META_DIR/draft-07/schema.json" "$DEST_DIR/draft-07/schema.json"
cp "$META_DIR/draft-2020-12/schema.json" "$DEST_DIR/draft-2020-12/schema.json"

echo "✅ Synced meta-schemas to $META_DIR and $DEST_DIR"
