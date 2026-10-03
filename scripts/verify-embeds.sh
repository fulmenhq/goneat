#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

SSOT_TEMPLATES="$ROOT_DIR/templates"
SSOT_SCHEMAS="$ROOT_DIR/schemas"
SSOT_CONFIG="$ROOT_DIR/config"
EMBED_TEMPLATES="$ROOT_DIR/internal/assets/embedded_templates/templates"
EMBED_SCHEMAS="$ROOT_DIR/internal/assets/embedded_schemas/schemas"
EMBED_CONFIG="$ROOT_DIR/internal/assets/embedded_config/config"

echo "🔎 Verifying embedded mirrors are in sync with SSOT..."

fail=0
check_dir() {
	local src="$1" dst="$2" name="$3"
	if [ ! -d "$src" ]; then
		echo "❌ Missing SSOT directory: $src" >&2
		fail=1
		return
	fi
	if [ ! -d "$dst" ]; then
		echo "❌ Missing embedded mirror: $dst" >&2
		fail=1
		return
	fi
	# Capture first: pipefail plus grep -q can mask drift on a SIGPIPE, and a
	# failed rsync must not be interpreted as an empty/clean comparison.
	local changes
	if ! changes=$(rsync -anic --delete "$src"/ "$dst"/); then
		echo "❌ $name: mirror comparison failed" >&2
		fail=1
		return
	fi
	if ! grep -qE '^[<>]f|^\*deleting ' <<<"$changes"; then
		echo "✅ $name: in sync"
	else
		echo "❌ $name: drift detected between SSOT and embedded mirror" >&2
		printf '%s\n' "$changes" >&2
		fail=1
	fi
}

check_dir "$SSOT_TEMPLATES" "$EMBED_TEMPLATES" templates
check_dir "$SSOT_SCHEMAS" "$EMBED_SCHEMAS" schemas
check_dir "$SSOT_CONFIG" "$EMBED_CONFIG" config

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
	printf "\nHint: run 'make embed-assets' to re-sync mirrors from SSOT, then commit changes.\n" >&2
	exit 1
fi

echo "✅ All embedded mirrors are up to date"
