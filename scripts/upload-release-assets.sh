#!/usr/bin/env bash
set -euo pipefail
# Upload only verified signatures, public keys and notes; never replace CI assets.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec python3 "$SCRIPT_DIR/release-assets.py" upload "${1:?'usage: upload-release-assets.sh <tag> [directory]'}" "${2:-dist/release}"
