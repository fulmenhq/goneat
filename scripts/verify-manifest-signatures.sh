#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec python3 "$SCRIPT_DIR/release-assets.py" verify-signatures "${1:?'usage: verify-manifest-signatures.sh <tag> [directory]'}" "${2:-dist/release}"
