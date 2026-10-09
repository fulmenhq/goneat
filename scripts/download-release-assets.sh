#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec python3 "$SCRIPT_DIR/release-assets.py" download "${1:?'usage: download-release-assets.sh <tag> [directory]'}" "${2:-dist/release}"
